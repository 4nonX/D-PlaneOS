package main

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

const (
	ChunkSize       = 10 * 1024 * 1024 // 10MB chunks
	UploadTempDir   = "/var/lib/dplaneos/upload-chunks"
	MaxUploadSize   = 1024 * 1024 * 1024 * 1024 // 1TB theoretical max
	ChunkTimeout    = 5 * time.Minute
)

// ChunkedUploadManager handles chunked file uploads
type ChunkedUploadManager struct {
	uploads map[string]*UploadSession
	mu      sync.RWMutex
}

// UploadSession tracks a chunked upload in progress
type UploadSession struct {
	UploadID      string
	Filename      string
	DestPath      string
	TotalChunks   int
	TotalSize     int64
	ReceivedChunks map[int]ChunkInfo
	StartTime     time.Time
	LastActivity  time.Time
	mu            sync.Mutex
}

// ChunkInfo stores metadata about a received chunk
type ChunkInfo struct {
	Index        int
	Size         int64
	ReceivedAt   time.Time
	MD5Checksum  string
}

// Global upload manager
var uploadManager = &ChunkedUploadManager{
	uploads: make(map[string]*UploadSession),
}

// Initialize upload manager
func init() {
	// Create upload temp directory
	os.MkdirAll(UploadTempDir, 0755)
	
	// Start cleanup goroutine
	go uploadManager.cleanupExpiredSessions()
}

// HandleChunkedUpload handles chunked file upload requests
func HandleChunkedUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	// Parse multipart form
	err := r.ParseMultipartForm(ChunkSize + 1024) // Chunk size + 1KB overhead
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "Failed to parse form: " + err.Error(),
		})
		return
	}
	
	// Extract metadata
	filename := r.FormValue("filename")
	destPath := r.FormValue("path")
	chunkIndex, _ := strconv.Atoi(r.FormValue("chunkIndex"))
	totalChunks, _ := strconv.Atoi(r.FormValue("totalChunks"))
	totalSize, _ := strconv.ParseInt(r.FormValue("fileSize"), 10, 64)
	
	// Validate
	if filename == "" || destPath == "" {
		respondJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "Missing filename or destination path",
		})
		return
	}
	
	// Generate upload ID (consistent across chunks)
	uploadID := generateUploadID(filename, totalSize)
	
	// Get or create session
	session := uploadManager.getOrCreateSession(uploadID, filename, destPath, totalChunks, totalSize)
	
	// Get chunk data
	file, handler, err := r.FormFile("chunk")
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "Failed to get chunk data: " + err.Error(),
		})
		return
	}
	defer file.Close()
	
	// Save chunk
	chunkPath := filepath.Join(UploadTempDir, uploadID, fmt.Sprintf("chunk_%d", chunkIndex))
	os.MkdirAll(filepath.Dir(chunkPath), 0755)
	
	outFile, err := os.Create(chunkPath)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"error":   "Failed to create chunk file: " + err.Error(),
		})
		return
	}
	defer outFile.Close()
	
	// Copy chunk data with MD5 checksum
	hash := md5.New()
	writer := io.MultiWriter(outFile, hash)
	
	written, err := io.Copy(writer, file)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"error":   "Failed to save chunk: " + err.Error(),
		})
		return
	}
	
	checksum := hex.EncodeToString(hash.Sum(nil))
	
	// Update session
	session.mu.Lock()
	session.ReceivedChunks[chunkIndex] = ChunkInfo{
		Index:       chunkIndex,
		Size:        written,
		ReceivedAt:  time.Now(),
		MD5Checksum: checksum,
	}
	session.LastActivity = time.Now()
	receivedCount := len(session.ReceivedChunks)
	session.mu.Unlock()
	
	// Log progress
	logger.Info("Chunk received",
		"upload_id", uploadID,
		"chunk", chunkIndex,
		"total_chunks", totalChunks,
		"size", written,
		"progress", fmt.Sprintf("%.1f%%", float64(receivedCount)/float64(totalChunks)*100),
	)
	
	// If all chunks received, assemble file
	if receivedCount == totalChunks {
		go func() {
			err := assembleFile(session)
			if err != nil {
				logger.Error("Failed to assemble file", "error", err, "upload_id", uploadID)
			} else {
				logger.Info("File assembled successfully", "upload_id", uploadID, "file", filename)
			}
		}()
		
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"success":  true,
			"complete": true,
			"upload_id": uploadID,
			"message": "All chunks received, assembling file...",
		})
	} else {
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"success":  true,
			"complete": false,
			"upload_id": uploadID,
			"chunk_index": chunkIndex,
			"received_chunks": receivedCount,
			"total_chunks": totalChunks,
			"progress": float64(receivedCount) / float64(totalChunks) * 100,
		})
	}
}

// assembleFile combines all chunks into final file
func assembleFile(session *UploadSession) error {
	session.mu.Lock()
	uploadID := session.UploadID
	filename := session.Filename
	destPath := session.DestPath
	totalChunks := session.TotalChunks
	session.mu.Unlock()
	
	// Create final file
	finalPath := filepath.Join(destPath, filename)
	finalFile, err := os.Create(finalPath)
	if err != nil {
		return fmt.Errorf("failed to create final file: %w", err)
	}
	defer finalFile.Close()
	
	// Assemble chunks in order
	for i := 0; i < totalChunks; i++ {
		chunkPath := filepath.Join(UploadTempDir, uploadID, fmt.Sprintf("chunk_%d", i))
		
		chunkFile, err := os.Open(chunkPath)
		if err != nil {
			return fmt.Errorf("failed to open chunk %d: %w", i, err)
		}
		
		_, err = io.Copy(finalFile, chunkFile)
		chunkFile.Close()
		
		if err != nil {
			return fmt.Errorf("failed to copy chunk %d: %w", i, err)
		}
		
		// Remove chunk after copying
		os.Remove(chunkPath)
	}
	
	// Remove upload directory
	os.RemoveAll(filepath.Join(UploadTempDir, uploadID))
	
	// Remove session
	uploadManager.mu.Lock()
	delete(uploadManager.uploads, uploadID)
	uploadManager.mu.Unlock()
	
	// Set proper permissions
	os.Chmod(finalPath, 0644)
	
	// Audit log
	auditLog("file_uploaded", map[string]interface{}{
		"upload_id":    uploadID,
		"filename":     filename,
		"destination":  destPath,
		"size":         session.TotalSize,
		"chunks":       totalChunks,
		"duration_sec": time.Since(session.StartTime).Seconds(),
	})
	
	return nil
}

// getOrCreateSession retrieves or creates upload session
func (m *ChunkedUploadManager) getOrCreateSession(uploadID, filename, destPath string, totalChunks int, totalSize int64) *UploadSession {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	session, exists := m.uploads[uploadID]
	if !exists {
		session = &UploadSession{
			UploadID:       uploadID,
			Filename:       filename,
			DestPath:       destPath,
			TotalChunks:    totalChunks,
			TotalSize:      totalSize,
			ReceivedChunks: make(map[int]ChunkInfo),
			StartTime:      time.Now(),
			LastActivity:   time.Now(),
		}
		m.uploads[uploadID] = session
	}
	
	return session
}

// cleanupExpiredSessions removes stale upload sessions
func (m *ChunkedUploadManager) cleanupExpiredSessions() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	
	for range ticker.C {
		m.mu.Lock()
		
		now := time.Now()
		for uploadID, session := range m.uploads {
			session.mu.Lock()
			lastActivity := session.LastActivity
			session.mu.Unlock()
			
			// Remove sessions inactive for more than chunk timeout
			if now.Sub(lastActivity) > ChunkTimeout {
				logger.Info("Cleaning up expired upload session", "upload_id", uploadID)
				
				// Remove temp files
				os.RemoveAll(filepath.Join(UploadTempDir, uploadID))
				
				// Remove session
				delete(m.uploads, uploadID)
			}
		}
		
		m.mu.Unlock()
	}
}

// generateUploadID creates consistent upload ID
func generateUploadID(filename string, size int64) string {
	data := fmt.Sprintf("%s:%d", filename, size)
	hash := md5.Sum([]byte(data))
	return hex.EncodeToString(hash[:])
}

// HandleUploadStatus returns status of an upload session
func HandleUploadStatus(w http.ResponseWriter, r *http.Request) {
	uploadID := r.URL.Query().Get("upload_id")
	if uploadID == "" {
		respondJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "Missing upload_id",
		})
		return
	}
	
	uploadManager.mu.RLock()
	session, exists := uploadManager.uploads[uploadID]
	uploadManager.mu.RUnlock()
	
	if !exists {
		respondJSON(w, http.StatusNotFound, map[string]interface{}{
			"success": false,
			"error":   "Upload session not found",
		})
		return
	}
	
	session.mu.Lock()
	receivedCount := len(session.ReceivedChunks)
	totalChunks := session.TotalChunks
	receivedBytes := int64(0)
	for _, chunk := range session.ReceivedChunks {
		receivedBytes += chunk.Size
	}
	session.mu.Unlock()
	
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"upload_id": uploadID,
		"filename": session.Filename,
		"received_chunks": receivedCount,
		"total_chunks": totalChunks,
		"received_bytes": receivedBytes,
		"total_bytes": session.TotalSize,
		"progress": float64(receivedCount) / float64(totalChunks) * 100,
		"complete": receivedCount == totalChunks,
	})
}

// HandleCancelUpload cancels an in-progress upload
func HandleCancelUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	uploadID := r.URL.Query().Get("upload_id")
	if uploadID == "" {
		respondJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "Missing upload_id",
		})
		return
	}
	
	uploadManager.mu.Lock()
	session, exists := uploadManager.uploads[uploadID]
	if exists {
		delete(uploadManager.uploads, uploadID)
	}
	uploadManager.mu.Unlock()
	
	if !exists {
		respondJSON(w, http.StatusNotFound, map[string]interface{}{
			"success": false,
			"error":   "Upload session not found",
		})
		return
	}
	
	// Remove temp files
	os.RemoveAll(filepath.Join(UploadTempDir, uploadID))
	
	logger.Info("Upload cancelled", "upload_id", uploadID, "filename", session.Filename)
	
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Upload cancelled",
	})
}

// respondJSON sends JSON response
func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// Register routes
func RegisterChunkedUploadRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/upload/chunk", HandleChunkedUpload)
	mux.HandleFunc("/api/upload/status", HandleUploadStatus)
	mux.HandleFunc("/api/upload/cancel", HandleCancelUpload)
}
