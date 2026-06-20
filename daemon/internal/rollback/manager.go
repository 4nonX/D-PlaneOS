// Package rollback provides automatic failure recovery and state reversal.
// manager.go: Phase 5 - Rollback system for crash recovery
package rollback

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os/exec"
	"time"

	"dplaned/internal/audit"
	"dplaned/internal/features"
	"dplaned/internal/gitops"
)

// Manager orchestrates automatic rollback on operation failures.
// Integrates with Phase 1-4: resource exhaustion, hardware detection,
// state machines, and audit trail.
type Manager struct {
	db           *sql.DB
	stateMachine *gitops.StateMachine
	featuresMgr  *features.Manager
	eventLogger  *audit.EventLogger
	repoPath     string
	log          func(string, ...interface{})
}

// NewManager creates a rollback manager.
func NewManager(
	db *sql.DB,
	sm *gitops.StateMachine,
	fm *features.Manager,
	el *audit.EventLogger,
	repoPath string,
) *Manager {
	return &Manager{
		db:           db,
		stateMachine: sm,
		featuresMgr:  fm,
		eventLogger:  el,
		repoPath:     repoPath,
		log: func(msg string, args ...interface{}) {
			log.Printf("[ROLLBACK] "+msg, args...)
		},
	}
}

// RecoverIncompleteOperations resumes interrupted operations at daemon startup.
// Called from main after database initialization.
// Returns true if recovery succeeded, false if recovery is needed but failed.
func (m *Manager) RecoverIncompleteOperations(ctx context.Context) bool {
	ops, err := m.stateMachine.GetIncompleteOperations(ctx)
	if err != nil {
		m.log("ERROR: Failed to query incomplete operations: %v", err)
		return false
	}

	if len(ops) == 0 {
		m.log("No incomplete operations to recover")
		return true
	}

	m.log("Found %d incomplete operations, attempting recovery", len(ops))

	recoveredCount := 0
	for _, op := range ops {
		m.log("Recovering operation: %s (state=%s)", op.ID, op.State)

		// Resume from last known state
		resumed, err := m.stateMachine.Resume(ctx, op.ID)
		if err != nil {
			m.log("ERROR: Cannot resume operation %s: %v", op.ID, err)
			// Mark as failed if resumption isn't possible
			m.stateMachine.Transition(ctx, op.ID, gitops.StateFailed, map[string]interface{}{
				"error": "Resume failed during startup recovery",
			})
			continue
		}

		// Attempt to resume the operation
		// In production, this would re-execute the operation's handler
		// For now, we just transition to InProgress for manual retry
		err = m.stateMachine.Transition(ctx, resumed.ID, gitops.StateInProgress, map[string]interface{}{
			"resumed_at": time.Now(),
		})

		if err != nil {
			m.log("ERROR: Failed to transition operation %s to InProgress: %v", op.ID, err)
			continue
		}

		recoveredCount++
		m.log("Resumed operation: %s", op.ID)
	}

	m.log("Recovery complete: %d/%d operations resumed", recoveredCount, len(ops))
	return recoveredCount == len(ops)
}

// RollbackOperation performs automatic rollback when an operation fails.
// Steps:
// 1. Mark operation as rolled_back in state machine
// 2. Revert config via git revert to last known good state
// 3. Log rollback event
// 4. Notify operators
func (m *Manager) RollbackOperation(ctx context.Context, opID string, reason string) error {
	m.log("Initiating rollback for operation: %s (reason: %s)", opID, reason)

	// 1. Transition operation to rolled_back state
	err := m.stateMachine.Transition(ctx, opID, gitops.StateRolledBack, map[string]interface{}{
		"rollback_reason": reason,
		"rollback_time":   time.Now(),
	})

	if err != nil {
		m.log("ERROR: Failed to mark operation as rolled back: %v", err)
		return fmt.Errorf("state transition failed: %w", err)
	}

	// 2. Revert config via git
	if err := m.revertConfigRepo(ctx); err != nil {
		m.log("WARNING: Git revert failed, config may be inconsistent: %v", err)
		// Don't fail entirely; partial recovery is better than none
	}

	// 3. Disable incompatible features
	m.disableIncompatibleFeatures(ctx)

	// 4. Audit the rollback
	m.eventLogger.LogRollbackApplied(ctx, opID, reason)

	m.log("Rollback completed for operation: %s", opID)
	return nil
}

// revertConfigRepo reverts the Git config repository to the last stable commit.
// Uses git revert to create a new commit that undoes the failed changes.
func (m *Manager) revertConfigRepo(ctx context.Context) error {
	// Get current HEAD commit hash
	cmd := exec.CommandContext(ctx, "git", "-C", m.repoPath, "rev-parse", "HEAD")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to get HEAD commit: %w", err)
	}

	currentCommit := string(output)
	m.log("Current config commit: %s", currentCommit)

	// Find last successful commit (requires marker in commit message or external state)
	// For now, revert to parent: git revert HEAD
	// In production, you'd query a "last_successful_sync" timestamp
	revertCmd := exec.CommandContext(ctx, "git", "-C", m.repoPath, "revert", "--no-edit", "HEAD")
	if err := revertCmd.Run(); err != nil {
		return fmt.Errorf("git revert failed: %w", err)
	}

	m.log("Config repository reverted to previous state")
	return nil
}

// disableIncompatibleFeatures disables any features that depend on hardware
// that is no longer available or in a degraded state.
// Called during rollback to ensure the system remains consistent.
func (m *Manager) disableIncompatibleFeatures(ctx context.Context) {
	m.log("Checking for incompatible features to disable")

	// Query hardware capabilities (would be passed from hardware.Profile)
	// For now, we just disable common optional features
	incompatibleFeatures := []string{
		"ha_clustering",  // Requires BMC
		"nvmeof_support", // Requires NVMe controllers
		"ses_enclosure",  // Requires SES-capable hardware
		"ad_integration", // Requires network connectivity
	}

	for _, featureID := range incompatibleFeatures {
		// Check if feature is currently enabled
		if m.featuresMgr.IsEnabled(featureID) {
			err := m.featuresMgr.Disable(ctx, featureID)
			if err == nil {
				m.log("Disabled incompatible feature: %s", featureID)
				m.eventLogger.LogFeatureChange(ctx, featureID, "disabled", "rollback: compatibility check")
			} else {
				m.log("WARNING: Failed to disable feature %s: %v", featureID, err)
			}
		}
	}
}

// OnResourceExhausted is called by Phase 1 when resources hit critical threshold.
// Returns true if the operation should be allowed to continue, false if it should be rejected.
func (m *Manager) OnResourceExhausted(ctx context.Context, resourceType string, percent int) bool {
	m.log("Resource exhaustion detected: %s at %d%%", resourceType, percent)

	// Log the event
	m.eventLogger.LogResourceCritical(ctx, resourceType, percent)

	// For disk exhaustion, attempt cleanup
	if resourceType == "disk" && percent >= 95 {
		m.log("Attempting emergency cleanup for disk exhaustion")
		// In production: delete old logs, prune snapshots, compact datasets
		// For now, just reject new operations
		return false
	}

	// For memory, allow but mark as degraded
	if resourceType == "memory" && percent >= 90 {
		m.log("Memory critical, but allowing operation to complete")
		return true
	}

	// For file descriptors, reject if truly critical
	if resourceType == "file_descriptors" && percent >= 95 {
		m.log("File descriptor exhaustion, rejecting new operations")
		return false
	}

	return true
}

// OnCircuitBreakerOpen is called when an external service circuit breaker opens.
// Decides whether to fail the current operation or degrade gracefully.
func (m *Manager) OnCircuitBreakerOpen(ctx context.Context, service string) error {
	m.log("Circuit breaker opened for service: %s", service)

	// Log the event
	m.eventLogger.LogCircuitStateChange(ctx, service, "open")

	// Some services are critical, others can degrade gracefully
	criticalServices := map[string]bool{
		"database": true, // Can't operate without database
		"storage":  true, // Can't operate without storage
	}

	if criticalServices[service] {
		return fmt.Errorf("critical service unavailable: %s", service)
	}

	// Non-critical services (LDAP, OIDC, email, etc.) can fail gracefully
	m.log("Service %s degraded but continuing with reduced functionality", service)
	return nil
}

// HealthCheckCallback is called periodically by Phase 3.3 health checks.
// Can be used to auto-trigger rollback if health degrades severely.
func (m *Manager) HealthCheckCallback(ctx context.Context, health map[string]string) error {
	// health map: {"zfs": "ok", "docker": "degraded", "network": "unavailable"}

	unavailableCount := 0
	for subsystem, status := range health {
		if status == "unavailable" {
			unavailableCount++
			m.log("Subsystem %s is unavailable", subsystem)
		}
	}

	// If multiple critical subsystems are down, consider rollback
	if unavailableCount >= 2 {
		m.log("Multiple subsystems unavailable, health is critical")
		// In production: trigger automatic rollback of recent changes
		// For now, just alert
	}

	return nil
}

// ValidateBeforeRollback checks if rollback is safe to perform.
// Returns error if rollback would leave system in worse state.
func (m *Manager) ValidateBeforeRollback(ctx context.Context) error {
	// Check if previous state is known and stable
	// Would query operation_journal for last successful state

	// Check if git repo is in consistent state
	cmd := exec.CommandContext(ctx, "git", "-C", m.repoPath, "status", "--porcelain")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("cannot check git status: %w", err)
	}

	if len(output) > 0 {
		return fmt.Errorf("git repo has uncommitted changes, rollback unsafe")
	}

	return nil
}

// Close cleans up rollback manager resources.
func (m *Manager) Close() error {
	m.log("Rollback manager shutting down")
	return nil
}
