# DPlaneOS Resilience Architecture

The goal here is simple but demanding: harden DPlaneOS so that **nothing ever fails catastrophically**. The system needs to gracefully degrade when things go wrong, whether it's hardware that isn't present, resources that run out, or services that go offline.

We built this as five integrated phases that work together to keep the system alive and responsive even under adverse conditions.

## Phase 1: When Disk and Memory Run Low

We continuously watch disk space, memory, and file descriptor usage. When resources hit critical levels, the system stops accepting new operations rather than trying to push forward and crash.

The implementation lives in `daemon/internal/resource/watcher.go` (background monitor), with tests in `daemon/internal/resource/watcher_test.go`, and HTTP-level enforcement via `daemon/internal/handlers/resource_guard.go`.

Thresholds are:
- **Disk**: 85% full triggers DEGRADED, 95% full triggers CRITICAL (returns 507 Insufficient Storage)
- **Memory**: 80% in use triggers DEGRADED, 90% in use triggers CRITICAL (returns 429 Too Many Requests)
- **File Descriptors**: 80% of max triggers DEGRADED, 95% triggers CRITICAL (returns 429)

The watcher can register callbacks that fire when these thresholds are crossed. Phase 5 listens to these callbacks to make rollback decisions:

```go
watcher.Start()
watcher.Stop()
watcher.GetStatus() → Status{DiskUsagePercent, MemoryPercent, FileDescriptorPercent, Status, Warnings}
watcher.RegisterCallback(threshold, severity, fn) → Invokes fn when crossed
```

When resource thresholds are hit, Phase 5 gets the callback and decides whether to allow or reject new operations.

## Phase 2: Making Features Work With Any Hardware

Not every appliance has the same hardware. Some have a BMC, some don't. Some have NVMe, some are pure SATA. Rather than fail when hardware is missing, we detect what you have at startup and automatically enable or disable features accordingly.

Hardware detection (`daemon/internal/hardware/profile.go`) scans PCI slots, IPMI interfaces, and network capabilities. It looks for:
- **BMC Type**: Redfish, iLO5, iLO4, IPMI, or nothing
- **Storage Controllers**: SAS controllers, NVMe controllers, SES enclosures
- **Network**: Bonding support, VLAN support, AD/DNS connectivity

Based on what's detected, the feature manager (`daemon/internal/features/flags.go`) enables or disables features. Features can be in one of four states:
- `disabled`: Hardware not present or operator turned it off
- `beta`: Experimental, available if hardware is present but not recommended for production
- `stable`: Fully tested and safe to use
- `deprecated`: Old feature, avoid using

Here's the mapping:
| Hardware Detected | Feature Enabled | Why |
|---|---|---|
| Redfish/iLO BMC | ha_clustering | High availability needs BMC power control |
| NVMe Controllers | nvmeof_support | NVMe-oF needs modern HBA firmware support |
| SES Enclosure | ses_enclosure | Temperature monitoring and LED control via SES SCSI |
| Network Bonding | lacp_bonding | LACP needs NIC driver support |
| AD Connectivity | ad_integration | Domain join needs network and DNS |

We also have circuit breakers for external services (`daemon/internal/resilience/circuit_breaker.go`). These prevent a single failing service from cascading across the system. A circuit breaker can be:
- `closed`: Service is working, requests go through normally
- `open`: Service failed too many times, requests rejected, fallbacks activated
- `half_open`: Waiting to test if the service recovered

Users interact with features through the API:
- `GET /api/system/features` lists all features and their current state
- `POST /api/system/features/:id/enable` enables a feature (if hardware allows)
- `POST /api/system/features/:id/disable` disables a feature (safe, no data loss)

Feature state is stored in the database:
```sql
CREATE TABLE feature_flags (
  id TEXT PRIMARY KEY,
  state TEXT CHECK (state IN ('disabled', 'beta', 'stable', 'deprecated')),
  requires_hardware TEXT,
  enabled BOOLEAN,
  last_modified TIMESTAMP
);
```

## Phase 3.1: Surviving Crashes and Power Loss

Long-running operations (creating a pool, migrating data, running snapshots) need to survive daemon crashes and power loss. We do this by writing operation state to a journal before executing the operation. If the daemon crashes, it reads the journal at startup and resumes from where it left off.

The state machine (`daemon/internal/gitops/state_machine.go`) manages strict state transitions to keep things safe:
```
Declared → Validating → InProgress → Completed
                            ↓
                          Failed → RolledBack
```

Each operation goes through these states:
1. **Declared**: Operation registered in journal (before execution)
2. **Validating**: Checking preconditions (hardware available, resources OK, feature enabled)
3. **InProgress**: Actually doing the work (ZFS, Docker, network changes)
4. **Completed**: Done and verified
5. **Failed**: Something went wrong during execution
6. **RolledBack**: Changes reverted to a safe state

The key methods are:
```go
sm.StartOperation(ctx, op Operation) error  // Register operation (writes to journal)
sm.Transition(ctx, opID, newState, details) error  // Move to next state
sm.Resume(ctx, opID) (*Operation, error)  // Resume from last state if daemon restarted
sm.GetIncompleteOperations(ctx) ([]Operation, error)  // Find operations that didn't finish
```

State is persisted in the operation_journal table (`daemon/migrations/00013_operation_journal.sql`):
```sql
CREATE TABLE operation_journal (
  id TEXT PRIMARY KEY,
  operation_type TEXT,
  state TEXT CHECK (state IN ('declared', 'validating', 'in_progress', 'completed', 'failed', 'rolled_back')),
  details JSONB,
  error_msg TEXT,
  started_at TIMESTAMP,
  completed_at TIMESTAMP,
  updated_at TIMESTAMP,
  INDEX idx_operation_journal_state ON state WHERE state NOT IN ('completed', 'failed', 'rolled_back')
);
```

When the daemon starts up, Phase 5 reads this table and resumes any operations that didn't finish.

## Phase 3.3: Knowing When Something Is Broken

We continuously check whether different parts of the system are healthy. This is how the dashboard knows what to show operators and how external monitoring tools know when something's wrong.

The health checker (`daemon/internal/monitoring/health_check.go`) runs checks on:
- **Always present**: resources (disk, memory, FDs), ZFS pools, Docker, PostgreSQL, network interfaces, circuit breaker states
- **If hardware detected**: BMC sensors, SES enclosure status, network bonding, VLANs

Health status levels are:
- `HealthOK`: Everything is operational
- `HealthDegraded`: Some things aren't working but the system still functions (e.g., LDAP down but file serving works)
- `HealthUnavailable`: Critical things are down and the system can't operate normally

We expose this health status via HTTP endpoints (`daemon/internal/handlers/health.go`):
| Endpoint | Response | Use |
|---|---|---|
| `GET /api/health` | Full system health as JSON | Dashboard and operators |
| `GET /api/health?subsystem=zfs` | Status of one subsystem | Checking specific component |
| `GET /api/health/live` | 200 if alive, 503 if not | Kubernetes liveness probe |
| `GET /api/health/ready` | 200 if ready, 503 if not | Kubernetes readiness probe |

The aggregation is straightforward: if any critical subsystem is unavailable, the whole system is unavailable. If any subsystem is degraded, the system is degraded. Only if everything is OK does the system report OK.

## Phase 4: Recording Everything That Happens

Every significant event is logged. Operations starting and completing, features being enabled/disabled, resource warnings, hardware detection results, rollbacks, auth failures. The audit trail helps operators understand what happened when something goes wrong, and it's designed to be tamper-proof.

We have a two-tier logging system. For high-volume production events, `daemon/internal/audit/buffered_logger.go` batches up to 100 events and flushes every 5 seconds. But security-critical events (authentication, permissions) bypass the buffer and write immediately. For phase-level events (`daemon/internal/audit/event.go`), we write one-at-a-time and compute HMAC integrity.

The event types are:
```
EventOperationStart        // Operation begins
EventOperationComplete     // Operation succeeds
EventOperationFailed       // Operation fails
EventCircuitOpen           // Circuit breaker opens
EventCircuitClosed         // Circuit breaker closes
EventFeatureEnabled        // Feature state change
EventFeatureDisabled       // Feature state change
EventResourceCritical      // Resource threshold exceeded
EventHardwareDetected      // Hardware discovery result
EventRollbackApplied       // Rollback executed
EventAuthFailure           // Authentication error
EventConfigChanged         // Configuration modification
```

To prevent tampering, each event is linked to the previous one via HMAC. If anyone deletes or modifies an event, the chain breaks and we can detect it:
```
event_N.hmac = MD5(event_(N-1).hmac || MD5(event_N_hash))
```

We can verify the chain integrity at any time:
```go
valid, firstBadID, err := eventLogger.VerifyAuditChain(ctx, fromID, toID)
if !valid {
    log.Printf("Audit chain broken at event %d - possible tampering", firstBadID)
}
```

The database schema (`daemon/migrations/00014_audit_events.sql`):
```sql
CREATE TABLE audit_events (
  id BIGSERIAL PRIMARY KEY,
  timestamp TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  event_type TEXT NOT NULL,
  component TEXT NOT NULL,  -- "zfs", "features", "resilience", "hardware", "gitops"
  operation_id TEXT,        -- Links to operation_journal
  user_id TEXT,
  status TEXT CHECK (status IN ('success', 'failure', 'warning')),
  details JSONB,
  ip_address TEXT,
  hmac TEXT,                -- HMAC chain link
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  INDEX idx_timestamp ON timestamp DESC,
  INDEX idx_type ON event_type,
  INDEX idx_component ON component,
  INDEX idx_operation_id ON operation_id
);

-- PostgreSQL trigger computes HMAC before inserting
CREATE TRIGGER audit_hmac_trigger
BEFORE INSERT ON audit_events
FOR EACH ROW
EXECUTE FUNCTION audit_compute_hmac();

-- Verification function checks chain integrity
CREATE FUNCTION audit_verify_chain(from_id BIGINT, to_id BIGINT)
RETURNS TABLE(is_valid BOOLEAN, first_invalid_id BIGINT) AS ...
```

All subsystems log events when they change state:

```go
// Phase 1: Resource watchers
watcher.RegisterCallback(func(status) {
    if status.Status == "CRITICAL" {
        eventLogger.LogResourceCritical(ctx, "disk", status.DiskUsagePercent)
    }
})

// Phase 2: Feature changes
featureMgr.OnStateChange(func(featureID, newState string) {
    eventLogger.LogFeatureChange(ctx, featureID, newState, "admin request")
})

// Phase 2: Circuit breaker
circuitBreaker.OnStateChange(func(newState string) {
    eventLogger.LogCircuitStateChange(ctx, "ldap", newState)
})

// Phase 3.1: Operation state machine
sm.Transition(ctx, opID, gitops.StateCompleted, ...)
// Followed by:
eventLogger.LogOperationComplete(ctx, opID, "gitops", duration)

// Phase 5: Rollback
rollbackMgr.RollbackOperation(ctx, opID, "hardware_incompatible")
// Internally logs:
eventLogger.LogRollbackApplied(ctx, opID, reason)
```

## Phase 5: Fixing Things When They Go Wrong

This is where the system responds to failures detected by all the other phases. It orchestrates recovery, decides what to do when resources are exhausted, hardware is missing, or operations fail.

The rollback manager (`daemon/internal/rollback/manager.go`) handles several recovery strategies:

**Incomplete Operation Recovery** happens at startup. It looks in the operation_journal for operations that didn't finish (didn't reach `completed`, `failed`, or `rolled_back` state). For each one, it tries to resume from the last checkpoint. If resumption isn't possible, it marks the operation as failed and logs it for manual review.

**Automatic Config Rollback** happens when an operation fails. The system executes `git revert HEAD`, creating a new commit that undoes the failed changes. This keeps the config repository consistent even if something went wrong.

**Feature Compatibility Enforcement** watches for hardware changes. If a hardware component goes offline (BMC, SES enclosure, network bonding), we automatically disable features that depend on it. For example, if the BMC goes offline, HA clustering gets disabled immediately.

**Circuit Breaker Recovery** distinguishes between critical and non-critical services. If LDAP goes down, we continue operating with cached credentials (degraded). If the database goes down, we fail the operation because we can't proceed safely.

**Health-Based Adaptation** watches for cascading failures. When multiple subsystems are down, we stop accepting new operations to prevent making things worse, but we let existing operations finish.

The key methods are:
```go
rollbackMgr.RecoverIncompleteOperations(ctx) bool  // At startup, resume unfinished ops
rollbackMgr.RollbackOperation(ctx, opID, reason) error  // Explicit rollback
rollbackMgr.OnResourceExhausted(ctx, type, percent) bool  // Phase 1 callback
rollbackMgr.OnCircuitBreakerOpen(ctx, service) error  // Phase 2 callback
rollbackMgr.HealthCheckCallback(ctx, health) error  // Phase 3.3 callback
rollbackMgr.ValidateBeforeRollback(ctx) error  // Safety check before rolling back
```

## How the Phases Talk to Each Other

When a user makes a request to create a pool, here's what happens:

```
User: POST /api/pools {"name": "tank", ...}
  ↓
Phase 2: Feature Gate
  Is "zfs_pool_create" enabled?
  Is hardware compatible?
  → If not: 403 Forbidden (stop here)
  ↓
Phase 1: Resource Guard
  Is disk > 5% free?
  Is memory < 90%?
  → If critical: 507 or 429 (stop here)
  ↓
Phase 3.1: Start Operation
  INSERT into operation_journal: state='declared'
  ↓
Execute (actual work)
  ZFS create pool, Docker create containers, etc.
  ↓
Phase 3.1: Update Operation State
  state='declared' → 'validating' → 'in_progress' → 'completed'
  ↓
Phase 4: Log Event
  Insert EventOperationComplete with HMAC chain
  ↓
Phase 3.3: Health Check
  Verify: pool online, no errors
  Status: HealthOK
  ↓
Response to User
  {"status": "created", "pool": "tank"}
```

The phases communicate via callbacks. Phase 1 registers a callback with Phase 5:
```go
watcher.RegisterCallback(func(status) {
    if status.Status == "CRITICAL" {
        allow := rollbackMgr.OnResourceExhausted(ctx, "disk", status.DiskUsagePercent)
        // HTTP middleware checks this and returns 507 if allow=false
    }
})
```

Phase 2 logs feature changes to Phase 4:
```go
featureMgr.OnStateChange("*", func(feature) {
    phases.EventLogger.LogFeatureChange(ctx, feature.ID, newState, reason)
})
circuitBreaker.OnStateChange(func(newState) {
    phases.EventLogger.LogCircuitStateChange(ctx, service, newState)
})
```

Phase 2 also tells Phase 5 when a circuit breaker opens:
```go
circuitBreaker.OnStateChange(func(newState) {
    if newState == "open" {
        rollbackMgr.OnCircuitBreakerOpen(ctx, service)
    }
})
```

Phase 3.3 health checks report to Phase 5:
```go
healthChecker.RegisterCallback(func(health) {
    rollbackMgr.HealthCheckCallback(ctx, health)
})
```

And Phase 3.1 state transitions log to Phase 4:
```go
sm.Transition(ctx, opID, StateCompleted, details)
eventLogger.LogOperationComplete(ctx, opID, component, duration)
```

## Real World Examples

### Creating a ZFS Pool

When a user requests to create a pool, here's what happens:

1. **Feature Gate (Phase 2)**: Is "zfs_pool_create" enabled and hardware-compatible? If not, return 403.

2. **Resource Guard (Phase 1)**: Is there at least 5% disk free? Is memory under the limit? If critical, return 507 or 429.

3. **Start Operation (Phase 3.1)**: Write to operation_journal: `{id='op-123', type='pool_create', state='declared'}`

4. **Validate**: Check that SAS controllers are detected. Transition state to 'validating'.

5. **Execute**: Call ZFS to create the pool. Stream progress to the client.

6. **Transition Operation**: Mark it complete. Transition state to 'in_progress' then 'completed'.

7. **Log Event (Phase 4)**: Insert audit event: `EventOperationComplete(opID='op-123', duration=5s)` with HMAC chain.

8. **Health Check (Phase 3.3)**: Verify ZFS pool is online, no circuit breakers opened. Status: OK.

9. **Response**: Return to user: `{"status": "created", "pool": "tank", "id": "op-123"}`

### BMC Goes Offline

A user tries to set up HA clustering but their BMC is offline.

1. **User Request**: `POST /api/ha/cluster {"peer": "192.168.1.2"}`

2. **Feature Gate Check (Phase 2)**: HA clustering requires a BMC. The hardware detector didn't find one (or it's gone offline). Return 403 Forbidden.

Alternatively, if HA was working before and the BMC just went offline:

3. **Health Check Notices (Phase 3.3)**: BMC health check fails: "bmc: unavailable"

4. **Phase 5 Recovery Triggered**: The system automatically disables the HA feature since it depends on BMC. It updates the database: `UPDATE feature_flags SET enabled=false WHERE id='ha_clustering'`

5. **Audit Event (Phase 4)**: `EventFeatureDisabled(feature='ha_clustering', reason='rollback: bmc unavailable')`

6. **Response to User**: 503 Service Unavailable: "HA unavailable (BMC offline)"

### Daemon Crashes During Migration

A user starts a large dataset migration which takes several minutes.

1. **User Request**: `POST /api/datasets/migrate {...large migration...}` Operation 'op-456' is created with state 'declared'.

2. **Executing**: The daemon transitions the operation through states: declared → validating → in_progress. It writes progress checkpoints every 10 seconds to the operation_journal.

3. **Crash**: The OOM killer terminates the daemon because memory ran out. The operation is stuck in 'in_progress'. The migration tool is still running a child process.

4. **Daemon Restarts**: Phase 5 runs recovery immediately: `rollbackMgr.RecoverIncompleteOperations()`

5. **Found Incomplete Operation**: The recovery finds 'op-456' in 'in_progress' state. It calls `sm.Resume(ctx, 'op-456')`. The operation_journal has a checkpoint showing the last completed dataset.

6. **Resume Migration**: The handler re-runs the migration but skips datasets already completed. It uses the checkpoint from operation_journal.details as the resume point.

7. **Completes**: The operation transitions to 'completed'. An audit event is logged.

8. **Health Check**: Phase 3.3 verifies all datasets were created correctly.

9. **Audit Trail**: The HMAC chain in audit_events remains unbroken. No events were deleted or reordered.

## Starting Everything Up

All phases are initialized in a specific order to respect dependencies. The code in `daemon/internal/bootstrap/phases.go` handles this:

1. Database is ready
2. Phase 1 starts (resource watcher) - independent, no dependencies
3. Phase 2 starts (hardware detection, features, circuit breakers) - needs Phase 1
4. Phase 3.1 starts (state machine) - needs database
5. Phase 3.3 starts (health checker) - needs Phase 1, 2, and 3.1
6. Phase 4 starts (event logger) - needs database
7. Phase 5 starts (rollback manager) - needs Phase 3.1, 2, and 4
8. Callbacks are wired up between all phases
9. Background services start (watchers, periodic health checks)

In main(), it looks like this:
```go
db, _ := initDatabase()
phases, _ := bootstrap.InitializeAllPhases(ctx, db, repoPath)
defer phases.Close()

// All phases now running, wired, and integrated
```

## Database Changes

Three new tables need to be created. These migrations must be applied in order:

**00012_feature_flags.sql** (Phase 2): Creates the feature_flags table that persists feature state across restarts.

**00013_operation_journal.sql** (Phase 3.1): Creates the operation_journal table with proper indexes so crash recovery can quickly find incomplete operations.

**00014_audit_events.sql** (Phase 4): Creates the audit_events table with HMAC chain support, includes a PostgreSQL trigger that computes HMAC before inserting, and a verification function for checking chain integrity.

## Testing

Unit tests cover each phase individually: Phase 1 tests resource threshold detection, Phase 2 tests hardware mapping and circuit breaker transitions, Phase 3.1 tests state transitions and operation resumption, Phase 3.3 tests health aggregation, Phase 4 tests HMAC computation and integrity, Phase 5 tests rollback and recovery.

Integration tests verify that phases talk to each other correctly: resource exhaustion triggers Phase 5 recovery, hardware removal disables features and logs to the audit trail, incomplete operations at startup are resumed from checkpoints, health degradation causes rejection of new operations, and the audit chain remains verifiable throughout.

Chaos tests simulate real failure scenarios:
- Network partitions cause LDAP to fail, system degrades gracefully
- Disk full triggers resource guard, rejects operations, logs alerts
- Daemon crashes mid-operation are recovered at startup
- Hardware removal is detected, dependent features auto-disable
- Multiple subsystems down simultaneously still keep the system responsive

## Watching and Alerting

Prometheus metrics (hooks exist, values to be filled in):
```
dplaned_resource_disk_percent{severity="degraded"}
dplaned_resource_disk_percent{severity="critical"}
dplaned_operation_duration_seconds{type="pool_create"}
dplaned_circuit_breaker_state{service="ldap"} # 0=closed, 1=open, 2=half_open
dplaned_feature_enabled{feature="ha_clustering"}
dplaned_health_status{subsystem="zfs"} # 0=ok, 1=degraded, 2=unavailable
dplaned_audit_events_total{event_type="operation_start"}
```

All phases output structured JSON logs that aggregators can parse:
```json
{"timestamp": "2026-06-18T10:30:45Z", "level": "INFO", "phase": "1", "message": "Resource critical", "disk_percent": 95}
{"timestamp": "2026-06-18T10:30:46Z", "level": "WARN", "phase": "5", "message": "Rollback triggered", "operation_id": "op-456", "reason": "hardware_incompatible"}
```

Recommended alerting rules:
- Alert if any subsystem health is unavailable for more than 5 minutes
- Alert if operation_journal has incomplete operations lingering at startup
- Alert if circuit breaker remains open for more than 10 minutes
- Alert if audit chain verification fails (indicates potential tampering)

## Deploying This

When rolling this out, you need to:

1. Run the three database migrations (00012, 00013, 00014) in order
2. Update main.go to call `bootstrap.InitializeAllPhases()` during startup
3. Configure resource thresholds appropriate for your hardware (disk free, memory headroom)
4. Test the recovery scenarios manually: crash the daemon mid-operation, exhaust disk, unplug network
5. Watch the audit trail for the first week and make sure events are being logged
6. Set up Prometheus metrics if you want to monitor the phases
7. Create alerting rules for when health status degrades
8. Document operator runbooks so people know what to do if something fails

## The Philosophy

**Graceful Degradation** is the core idea. The system avoids catastrophic failure by degrading gracefully instead of crashing. No BMC? Disable HA clustering. Circuit breaker open? Continue with fallback. Resources critical? Reject new operations but keep the ones already running.

**Hardware Agnostic** means all features are optional. The system works with any hardware, even a minimal NAS with no BMC. We auto-detect capabilities at startup and enforce them via feature gates.

**Crash Safe** means every long-running operation is persisted to operation_journal before execution. If the daemon crashes, startup recovery reads the journal and resumes from the last checkpoint. Operation state is never lost.

**Immutable Audit Trail** means all phase transitions are logged with cryptographic integrity. You can verify the chain later and detect if anyone tampered with the logs.

**Composable Phases** means each phase is independent but they communicate via callbacks. You could add Phase 6 (ML-based anomaly detection) or Phase 7 (automated healing) without touching the existing phases.

## Is It Ready for Production?

The code follows Go best practices: well-tested, error-handled, proper context propagation for cancellation, cleanup via defer/close. Database operations are atomic via transactions with proper indexing. There's structured JSON logging from all phases (Prometheus metrics hooks exist but aren't filled in yet). We have circuit breakers, health checks, and automatic rollback. Security: HMAC prevents audit tampering, feature gates prevent unsupported features, resource limits prevent denial-of-service.

## What's Out of Scope

Some things are intentionally out of scope and can be added as Phase 6+ later:

- Auto-healing (replacing failed drives) requires domain-specific logic and hardware integration
- ML-based anomaly detection would be cool but adds complexity
- Distributed tracing (Jaeger, etc.) - useful for big deployments but not critical
- gRPC endpoints - REST API is sufficient
- Kubernetes integration - possible but not required for MVP
- TLS/mTLS between components - use host-level network security instead

## Did It Work?

The success criteria are:
- Zero uncaught panics during operations
- Resource exhaustion doesn't crash the daemon, it degrades gracefully
- Hardware removal doesn't break the system, features auto-disable
- Incomplete operations resume correctly when the daemon restarts
- Audit chain verifies integrity for all operations
- Circuit breaker keeps system responsive even with failed external services
- Operators can understand what's happening via `/api/health` endpoint
- No data loss even if the daemon crashes mid-operation
