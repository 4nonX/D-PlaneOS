// Package handlers provides HTTP request handlers.
// resource_guard.go: Middleware to enforce resource limits and prevent writes when resources exhausted.
package handlers

import (
	"encoding/json"
	"net/http"
	"dplaned/internal/resource"
)

// ResourceGuardMiddleware returns HTTP 507 (Insufficient Storage) or 429 (Too Many Requests)
// when system resources are exhausted, preventing cascading failures from disk full or FD exhaustion.
func ResourceGuardMiddleware(watcher *resource.Watcher) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			status := watcher.GetStatus()

			// CRITICAL: Reject all writes
			if status.Status == "CRITICAL" {
				// Disk full: reject write operations
				if status.DiskUsagePercent >= 98 {
					http.Error(w, "Insufficient storage: disk full", http.StatusInsufficientStorage)
					return
				}
				// FD exhaustion: reject new connections
				if status.FileDescriptorPercent >= 90 {
					http.Error(w, "Too many open files", http.StatusTooManyRequests)
					return
				}
				// Memory pressure: reject complex operations
				if status.MemoryPercent >= 95 {
					http.Error(w, "Memory exhausted", http.StatusInsufficientStorage)
					return
				}
			}

			// DEGRADED: Warn but allow reads; reject writes for data operations
			if status.Status == "DEGRADED" {
				// Allow reads always; block writes to storage operations
				if isWriteOperation(r) && isDiskSensitiveOperation(r) {
					w.Header().Set("X-Resource-Warning", "System degraded; write operations may fail")
					if status.DiskUsagePercent >= 95 {
						http.Error(w, "Disk space low; operation rejected", http.StatusInsufficientStorage)
						return
					}
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

// isWriteOperation checks if request is a write (POST, PUT, PATCH, DELETE).
func isWriteOperation(r *http.Request) bool {
	return r.Method == http.MethodPost || r.Method == http.MethodPut ||
		r.Method == http.MethodPatch || r.Method == http.MethodDelete
}

// isDiskSensitiveOperation checks if the operation affects persistent storage.
// These operations should fail cleanly if disk is full.
func isDiskSensitiveOperation(r *http.Request) bool {
	path := r.URL.Path
	// Operations that write data: ZFS, Docker, Shares, GitOps, Backups
	diskSensitivePatterns := []string{
		"/api/zfs/",
		"/api/pools/",
		"/api/datasets/",
		"/api/snapshots/",
		"/api/docker/",
		"/api/shares/",
		"/api/files/",
		"/api/gitops/apply",
		"/api/backup/",
		"/api/replication/",
	}

	for _, pattern := range diskSensitivePatterns {
		if len(path) >= len(pattern) && path[:len(pattern)] == pattern {
			return true
		}
	}
	return false
}

// ResourceStatusHandler returns current resource status as JSON.
// Usage: router.HandleFunc("/api/system/resource-status", resourceStatusHandler(watcher)).Methods("GET")
func ResourceStatusHandler(watcher *resource.Watcher) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if watcher == nil {
			http.Error(w, "Resource monitoring not enabled", http.StatusServiceUnavailable)
			return
		}

		status := watcher.GetStatus()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(status)
	}
}
