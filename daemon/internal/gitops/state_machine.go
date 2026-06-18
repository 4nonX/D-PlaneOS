// Package gitops provides declarative state reconciliation.
// state_machine.go: Phase 3.1 - Formalized state transitions with crash recovery.
package gitops

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"
)

// OperationState represents the state of a long-running operation.
type OperationState string

const (
	StateDecl     OperationState = "declared"     // User declared the desired state in Git
	StateValidating OperationState = "validating"   // Validating preconditions
	StateInProgress OperationState = "in_progress"  // Operation actively running
	StateCompleted  OperationState = "completed"    // Operation succeeded
	StateFailed     OperationState = "failed"       // Operation failed
	StateRolledBack OperationState = "rolled_back"  // Rolled back after failure
)

// Operation represents a unit of work that can be retried/resumed.
type Operation struct {
	ID          string                 `json:"id"`           // Unique operation ID
	Type        string                 `json:"type"`         // "dataset_create", "pool_vdev_replace", etc.
	State       OperationState         `json:"state"`        // Current state
	StartedAt   time.Time              `json:"started_at"`
	CompletedAt *time.Time             `json:"completed_at,omitempty"`
	Details     map[string]interface{} `json:"details"`      // Operation-specific data
	Error       string                 `json:"error,omitempty"`
	Idempotent  bool                   `json:"idempotent"`   // Can be safely retried
}

// StateMachine manages operation state transitions.
type StateMachine struct {
	db  *sql.DB
	log func(string, ...interface{})
}

// NewStateMachine creates a new state machine.
func NewStateMachine(db *sql.DB) *StateMachine {
	return &StateMachine{
		db: db,
		log: func(msg string, args ...interface{}) {
			log.Printf("[STATE-MACHINE] "+msg, args...)
		},
	}
}

// StartOperation begins a new operation and writes initial state.
func (sm *StateMachine) StartOperation(ctx context.Context, op Operation) error {
	if op.ID == "" {
		return fmt.Errorf("operation ID required")
	}

	op.StartedAt = time.Now()
	op.State = StateDecl

	// Write to operation_journal atomically
	data, err := json.Marshal(op)
	if err != nil {
		return fmt.Errorf("marshal operation failed: %w", err)
	}

	_, err = sm.db.ExecContext(ctx, `
		INSERT INTO operation_journal (id, operation_type, state, details, started_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, op.ID, op.Type, op.State, string(data), op.StartedAt, time.Now())

	if err != nil {
		return fmt.Errorf("insert operation failed: %w", err)
	}

	sm.log("Operation started: %s (type=%s)", op.ID, op.Type)
	return nil
}

// Transition moves operation to a new state.
// Only allowed transitions: Declared → Validating → InProgress → Completed/Failed/RolledBack
func (sm *StateMachine) Transition(ctx context.Context, opID string, newState OperationState, details map[string]interface{}) error {
	// Get current state
	var currentState string
	err := sm.db.QueryRowContext(ctx, `SELECT state FROM operation_journal WHERE id = $1`, opID).Scan(&currentState)
	if err != nil {
		return fmt.Errorf("operation not found: %s", opID)
	}

	// Validate transition is allowed
	if !isValidTransition(OperationState(currentState), newState) {
		return fmt.Errorf("invalid state transition: %s → %s", currentState, newState)
	}

	completedAt := (*time.Time)(nil)
	if newState == StateCompleted || newState == StateFailed || newState == StateRolledBack {
		now := time.Now()
		completedAt = &now
	}

	data, _ := json.Marshal(details)
	_, err = sm.db.ExecContext(ctx, `
		UPDATE operation_journal
		SET state = $1, details = $2, completed_at = $3, updated_at = $4
		WHERE id = $5
	`, newState, string(data), completedAt, time.Now(), opID)

	if err != nil {
		return fmt.Errorf("transition failed: %w", err)
	}

	sm.log("Operation transitioned: %s (%s → %s)", opID, currentState, newState)
	return nil
}

// Resume resumes an incomplete operation from its last known state.
func (sm *StateMachine) Resume(ctx context.Context, opID string) (*Operation, error) {
	var op Operation
	var detailsJSON string

	err := sm.db.QueryRowContext(ctx, `
		SELECT id, operation_type, state, details, started_at, completed_at, error_msg
		FROM operation_journal
		WHERE id = $1
	`, opID).Scan(&op.ID, &op.Type, &op.State, &detailsJSON, &op.StartedAt, &op.CompletedAt, &op.Error)

	if err != nil {
		return nil, fmt.Errorf("operation not found: %w", err)
	}

	json.Unmarshal([]byte(detailsJSON), &op.Details)

	// Only resume incomplete operations
	if op.State == StateCompleted || op.State == StateFailed {
		return nil, fmt.Errorf("cannot resume operation in %s state", op.State)
	}

	sm.log("Resuming operation: %s at state %s", opID, op.State)
	return &op, nil
}

// GetIncompleteOperations returns all operations not in final state.
func (sm *StateMachine) GetIncompleteOperations(ctx context.Context) ([]Operation, error) {
	rows, err := sm.db.QueryContext(ctx, `
		SELECT id, operation_type, state, details, started_at, completed_at, error_msg
		FROM operation_journal
		WHERE state NOT IN ('completed', 'failed', 'rolled_back')
		ORDER BY started_at DESC
		LIMIT 100
	`)
	if err != nil {
		return nil, fmt.Errorf("query incomplete operations failed: %w", err)
	}
	defer rows.Close()

	var ops []Operation
	for rows.Next() {
		var op Operation
		var detailsJSON string

		if err := rows.Scan(&op.ID, &op.Type, &op.State, &detailsJSON, &op.StartedAt, &op.CompletedAt, &op.Error); err != nil {
			continue
		}

		json.Unmarshal([]byte(detailsJSON), &op.Details)
		ops = append(ops, op)
	}

	return ops, nil
}

// isValidTransition checks if a state transition is allowed.
func isValidTransition(from, to OperationState) bool {
	allowedTransitions := map[OperationState][]OperationState{
		StateDecl: {StateValidating},
		StateValidating: {StateInProgress, StateFailed},
		StateInProgress: {StateCompleted, StateFailed},
		StateCompleted: {}, // Terminal state
		StateFailed: {StateRolledBack},
		StateRolledBack: {}, // Terminal state
	}

	allowed, exists := allowedTransitions[from]
	if !exists {
		return false
	}

	for _, allowedTo := range allowed {
		if allowedTo == to {
			return true
		}
	}
	return false
}
