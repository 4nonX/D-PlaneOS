// Package zfs provides ZFS operations and recovery logic.
// recovery.go: Safe state recovery after power loss or daemon crash.
package zfs

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// RecoveryCheck runs on daemon startup to detect and repair incomplete operations.
// Returns a report of what was recovered.
type RecoveryReport struct {
	Timestamp          time.Time
	IncompleteOps      []string // Operations left in progress
	FaultedPools       []string // Pools with FAULTED vdevs
	LockedDatasets     []string // Encrypted datasets with keys unloaded
	CorruptSnapshots   []string // Snapshots with checksum errors
	RecommendedActions []string // Actions operator should take
}

// RunRecovery performs startup consistency checks and attempts safe recovery.
func RunRecovery(ctx context.Context, db *sql.DB) (*RecoveryReport, error) {
	report := &RecoveryReport{
		Timestamp:          time.Now(),
		RecommendedActions: []string{},
	}

	log.Println("[RECOVERY] Starting consistency checks...")

	// 1. Check for incomplete operations in database
	if err := checkIncompleteOperations(ctx, db, report); err != nil {
		log.Printf("[RECOVERY] Error checking incomplete ops: %v", err)
	}

	// 2. Check ZFS pool health
	if err := checkPoolHealth(report); err != nil {
		log.Printf("[RECOVERY] Error checking pools: %v", err)
	}

	// 3. Check encrypted dataset status
	if err := checkEncryptedDatasets(report); err != nil {
		log.Printf("[RECOVERY] Error checking encrypted datasets: %v", err)
	}

	// 4. Check for snapshot corruption
	if err := checkSnapshotIntegrity(report); err != nil {
		log.Printf("[RECOVERY] Error checking snapshots: %v", err)
	}

	// Log findings
	if len(report.IncompleteOps) > 0 || len(report.FaultedPools) > 0 || len(report.LockedDatasets) > 0 {
		log.Printf("[RECOVERY] Found issues: %d incomplete ops, %d faulted pools, %d locked datasets",
			len(report.IncompleteOps), len(report.FaultedPools), len(report.LockedDatasets))
	} else {
		log.Println("[RECOVERY] System consistent; no recovery needed")
	}

	return report, nil
}

// checkIncompleteOperations detects operations left in PENDING state.
func checkIncompleteOperations(ctx context.Context, db *sql.DB, report *RecoveryReport) error {
	// Query operation_journal table for incomplete operations
	// (Assumes table exists from Phase 3.1; for now, this is a placeholder)
	log.Println("[RECOVERY] Checking for incomplete operations...")

	// Example query when table exists:
	// SELECT id, operation_type, state FROM operation_journal WHERE state != 'COMPLETED' AND state != 'FAILED' LIMIT 100

	report.RecommendedActions = append(report.RecommendedActions,
		"Monitor operation logs: check /var/log/dplaneos/operations.log for failures")

	return nil
}

// checkPoolHealth verifies ZFS pool status.
func checkPoolHealth(report *RecoveryReport) error {
	log.Println("[RECOVERY] Checking ZFS pool health...")

	cmd := exec.Command("zpool", "status", "-x")
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Non-zero exit usually means one or more pools have issues
		log.Printf("[RECOVERY] zpool status: %s", string(output))
	}

	// Parse output to find faulted pools
	for _, line := range strings.Split(string(output), "\n") {
		if strings.Contains(line, "FAULTED") {
			// Extract pool name (crude parser; improve for production)
			fields := strings.Fields(line)
			if len(fields) > 0 {
				poolName := fields[0]
				report.FaultedPools = append(report.FaultedPools, poolName)
				report.RecommendedActions = append(report.RecommendedActions,
					fmt.Sprintf("Pool %s is FAULTED: replace failed vdev via ZFS VDEV replacement workflow", poolName))
			}
		}
	}

	// Check for resilver/scrub in progress
	cmd = exec.Command("zpool", "status")
	output, err = cmd.Output()
	if err == nil {
		for _, line := range strings.Split(string(output), "\n") {
			if strings.Contains(line, "resilver") || strings.Contains(line, "scrub") {
				log.Printf("[RECOVERY] ZFS operation in progress: %s", line)
				report.RecommendedActions = append(report.RecommendedActions,
					"ZFS resilver/scrub in progress; system will continue automatically")
			}
		}
	}

	return nil
}

// checkEncryptedDatasets verifies encryption key status.
func checkEncryptedDatasets(report *RecoveryReport) error {
	log.Println("[RECOVERY] Checking encrypted dataset status...")

	// List all datasets and check encryption status
	cmd := exec.Command("zfs", "list", "-H", "-o", "name,encryption,keystatus")
	output, err := cmd.Output()
	if err != nil {
		log.Printf("[RECOVERY] zfs list failed: %v", err)
		return nil // Not all systems have encrypted datasets; graceful failure
	}

	for _, line := range strings.Split(string(output), "\n") {
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 3 {
			name := fields[0]
			encryption := fields[1]
			keystatus := fields[2]

			if encryption != "off" && keystatus == "unavailable" {
				report.LockedDatasets = append(report.LockedDatasets, name)
				report.RecommendedActions = append(report.RecommendedActions,
					fmt.Sprintf("Dataset %s is encrypted but key is unavailable; load key via UI", name))
			}
		}
	}

	return nil
}

// checkSnapshotIntegrity verifies snapshot validity.
func checkSnapshotIntegrity(report *RecoveryReport) error {
	log.Println("[RECOVERY] Checking snapshot integrity...")

	// Verify last snapshot of each dataset is intact
	cmd := exec.Command("zfs", "list", "-H", "-t", "snapshot", "-o", "name,creation")
	output, err := cmd.Output()
	if err != nil {
		// No snapshots or zfs command not available
		return nil
	}

	// Group by dataset; find most recent snapshot per dataset
	snapshotsByDataset := make(map[string]string)
	for _, line := range strings.Split(string(output), "\n") {
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			snapName := fields[0]
			// Extract dataset (everything before @)
			parts := strings.Split(snapName, "@")
			if len(parts) == 2 {
				dataset := parts[0]
				snapshotsByDataset[dataset] = snapName
			}
		}
	}

	// For each recent snapshot, verify it's accessible
	for dataset, snapName := range snapshotsByDataset {
		// Try to list snapshot contents as a basic health check
		cmd := exec.Command("zfs", "get", "-H", "-o", "value", "type", snapName)
		out, err := cmd.Output()
		if err != nil {
			report.CorruptSnapshots = append(report.CorruptSnapshots, snapName)
			report.RecommendedActions = append(report.RecommendedActions,
				fmt.Sprintf("Snapshot %s may be corrupted; investigate with 'zpool status -v %s'", snapName, dataset))
		} else {
			log.Printf("[RECOVERY] Snapshot %s OK: %s", snapName, strings.TrimSpace(string(out)))
		}
	}

	return nil
}

// WriteAtomicFile writes to a file atomically (write to temp, fsync, rename).
// Returns an error if the write fails, ensuring data is never partially written.
func WriteAtomicFile(path string, data []byte) error {
	// Create temp file in same directory (ensures same filesystem)
	dir := filepath.Dir(path)
	tmpFile, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return fmt.Errorf("failed to create temp file in %s: %w", dir, err)
	}
	tmpName := tmpFile.Name()
	defer os.Remove(tmpName) // Clean up on error

	// Write all data
	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to write to temp file: %w", err)
	}

	// Fsync to ensure data on disk
	if err := tmpFile.Sync(); err != nil {
		tmpFile.Close()
		return fmt.Errorf("fsync failed: %w", err)
	}

	tmpFile.Close()

	// Atomic rename
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("rename failed: %w", err)
	}

	return nil
}

// VerifyConsistency checks that DB state matches ZFS reality.
// Used to detect and alert on inconsistencies after power loss.
func VerifyConsistency(ctx context.Context, db *sql.DB) ([]string, error) {
	var inconsistencies []string

	// 1. Check that all pools in DB exist in ZFS
	rows, err := db.QueryContext(ctx, "SELECT name FROM pools WHERE deleted_at IS NULL")
	if err != nil {
		return nil, fmt.Errorf("query pools failed: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var poolName string
		if err := rows.Scan(&poolName); err != nil {
			continue
		}

		// Verify pool exists
		cmd := exec.Command("zpool", "list", "-H", "-o", "name", poolName)
		err := cmd.Run()
		if err != nil {
			inconsistencies = append(inconsistencies,
				fmt.Sprintf("Database has pool %s but ZFS does not; pool may be offline or destroyed", poolName))
		}
	}

	// 2. Check that all datasets in DB exist in ZFS
	rows, err = db.QueryContext(ctx, "SELECT pool, name FROM datasets WHERE deleted_at IS NULL")
	if err != nil {
		return nil, fmt.Errorf("query datasets failed: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var pool, name string
		if err := rows.Scan(&pool, &name); err != nil {
			continue
		}

		fullName := fmt.Sprintf("%s/%s", pool, name)
		cmd := exec.Command("zfs", "list", "-H", "-o", "name", fullName)
		err := cmd.Run()
		if err != nil {
			inconsistencies = append(inconsistencies,
				fmt.Sprintf("Database has dataset %s but ZFS does not; dataset may be destroyed", fullName))
		}
	}

	return inconsistencies, nil
}
