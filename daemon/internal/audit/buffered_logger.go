package audit

import (
	"database/sql"
	"fmt"
	"log"
	"sync"
	"time"
)

// AuditEvent represents a single audit log entry
type AuditEvent struct {
	Timestamp int64
	User      string
	Action    string
	Resource  string
	Details   string
	IPAddress string
	Success   bool
}

// BufferedLogger implements batched audit logging for high-performance SQLite
type BufferedLogger struct {
	db          *sql.DB
	buffer      []AuditEvent
	bufferMutex sync.Mutex
	flushTicker *time.Ticker
	stopChan    chan struct{}
	maxBuffer   int
	flushInterval time.Duration
}

// NewBufferedLogger creates a new buffered audit logger
//
// CRITICAL for 52TB systems:
// - Batches audit logs to reduce SQLite I/O
// - Flushes every 5 seconds OR when buffer reaches maxBuffer
// - Prevents I/O stalls during mass file operations
//
// Example: Moving 10,000 files generates 10,000 audit events
// Without buffering: 10,000 individual SQLite INSERTs → slow!
// With buffering: 1-2 batch INSERTs → fast!
func NewBufferedLogger(db *sql.DB, maxBuffer int, flushInterval time.Duration) *BufferedLogger {
	if maxBuffer <= 0 {
		maxBuffer = 100
	}
	if flushInterval <= 0 {
		flushInterval = 5 * time.Second
	}

	bl := &BufferedLogger{
		db:            db,
		buffer:        make([]AuditEvent, 0, maxBuffer),
		maxBuffer:     maxBuffer,
		flushInterval: flushInterval,
		stopChan:      make(chan struct{}),
	}

	return bl
}

// Start begins the background flushing goroutine
func (bl *BufferedLogger) Start() {
	bl.flushTicker = time.NewTicker(bl.flushInterval)
	
	go func() {
		for {
			select {
			case <-bl.flushTicker.C:
				// Periodic flush
				if err := bl.Flush(); err != nil {
					log.Printf("Error flushing audit logs: %v", err)
				}
			case <-bl.stopChan:
				// Final flush before shutdown
				bl.flushTicker.Stop()
				if err := bl.Flush(); err != nil {
					log.Printf("Error in final audit flush: %v", err)
				}
				return
			}
		}
	}()
}

// Stop gracefully stops the buffered logger
func (bl *BufferedLogger) Stop() {
	close(bl.stopChan)
}

// Log adds an event to the buffer
//
// Thread-safe: Can be called from multiple goroutines
func (bl *BufferedLogger) Log(event AuditEvent) error {
	bl.bufferMutex.Lock()
	defer bl.bufferMutex.Unlock()

	// Add to buffer
	bl.buffer = append(bl.buffer, event)

	// Check if buffer is full
	if len(bl.buffer) >= bl.maxBuffer {
		// Flush immediately if buffer is full
		// Unlock first to avoid deadlock
		bl.bufferMutex.Unlock()
		err := bl.Flush()
		bl.bufferMutex.Lock()
		return err
	}

	return nil
}

// Flush writes all buffered events to SQLite in a single transaction
//
// CRITICAL: Uses BEGIN TRANSACTION for batch insert
// This is 100x faster than individual INSERTs
func (bl *BufferedLogger) Flush() error {
	bl.bufferMutex.Lock()
	
	// Quick exit if buffer is empty
	if len(bl.buffer) == 0 {
		bl.bufferMutex.Unlock()
		return nil
	}

	// Copy buffer and clear it
	events := make([]AuditEvent, len(bl.buffer))
	copy(events, bl.buffer)
	bl.buffer = bl.buffer[:0]
	
	bl.bufferMutex.Unlock()

	// Write to database in single transaction
	tx, err := bl.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Prepare statement (reused for all inserts)
	stmt, err := tx.Prepare(`
		INSERT INTO audit_logs (
			timestamp, user, action, resource, details, ip_address, success
		) VALUES (?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	// Insert all events in batch
	for _, event := range events {
		_, err := stmt.Exec(
			event.Timestamp,
			event.User,
			event.Action,
			event.Resource,
			event.Details,
			event.IPAddress,
			event.Success,
		)
		if err != nil {
			// Log error but continue with other events
			log.Printf("Failed to insert audit event: %v", err)
		}
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	log.Printf("Flushed %d audit events to database", len(events))
	return nil
}

// GetStats returns buffer statistics
func (bl *BufferedLogger) GetStats() map[string]interface{} {
	bl.bufferMutex.Lock()
	defer bl.bufferMutex.Unlock()

	return map[string]interface{}{
		"buffer_size":     len(bl.buffer),
		"max_buffer":      bl.maxBuffer,
		"flush_interval":  bl.flushInterval.String(),
		"buffer_capacity": cap(bl.buffer),
	}
}

// Example usage in main.go:
//
// var auditLogger *audit.BufferedLogger
//
// func main() {
//     db, _ := sql.Open("sqlite3", "/var/lib/dplaneos/dplaneos.db")
//     
//     // Create buffered logger
//     // Buffer up to 100 events, flush every 5 seconds
//     auditLogger = audit.NewBufferedLogger(db, 100, 5*time.Second)
//     auditLogger.Start()
//     defer auditLogger.Stop()
//     
//     // Log events (non-blocking, fast!)
//     auditLogger.Log(audit.AuditEvent{
//         Timestamp: time.Now().Unix(),
//         User:      "admin",
//         Action:    "file_delete",
//         Resource:  "/tank/data/old.txt",
//         Success:   true,
//     })
// }
