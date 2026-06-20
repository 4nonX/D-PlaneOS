# DPlaneOS Resilience & Recovery

## What is This?

DPlaneOS is designed to keep running even when things break. This document explains how the system recovers from failures and what you can expect.

## Key Behaviors

### 1. Hardware Compatibility Check
When the appliance starts, it detects available hardware (BMC, NVMe controllers, SES enclosures, network bonds). Features that depend on missing hardware are automatically disabled.

**What you'll see**: Some features may show as "Not Available" in Settings even though you haven't disabled them. This means your hardware doesn't support them, which is fine.

**What to do**: Nothing. The system adapts automatically.

---

### 2. Resource Limits
The system monitors disk space, memory, and open file limits. When any reach critical levels, new operations are rejected to prevent crashes.

**Disk Critical (> 95% full)**:
- New operations return: `507 Insufficient Storage`
- Running operations continue
- System logs a warning event

**Memory Critical (> 90% in use)**:
- New operations return: `429 Too Many Requests`
- Running operations continue
- System attempts to free memory

**File Descriptor Limit (> 95% of max)**:
- New operations return: `429 Too Many Requests`
- Indicates too many open file handles
- Usually resolved by restarting affected services

**What to do**: 
- Disk: Delete old snapshots, replicas, or logs to free space
- Memory: Reduce number of concurrent operations
- File descriptors: Restart Docker containers or the appliance

---

### 3. External Service Failures
When external services (LDAP, OIDC, email, NFS) become unavailable, the system doesn't crash, it degrades gracefully.

**What you'll see**:
- Active operations continue
- New operations using that service are rejected
- Health dashboard shows service as "Degraded"
- Operations that don't depend on the service work normally

**Example**: LDAP server offline
- User authentication fails → login rejected
- File serving continues normally
- Snapshots can still run
- Replication can still operate (unless it needs LDAP)

**What to do**:
- Restore the external service
- Or, disable features that depend on it (see Settings → Features)
- System auto-recovers when service comes back online

---

### 4. Interrupted Operations
If the appliance crashes or loses power mid-operation (creating a pool, migrating data, etc.), the operation resumes automatically when it boots back up.

**What happens**:
1. Appliance starts
2. Checks for incomplete operations in the operation journal
3. Resumes each operation from its last checkpoint
4. Completes the operation
5. Logs the completion event

**What you'll see**:
- Operations that were "in progress" complete normally
- No data loss (state was persisted before the crash)
- Audit log shows when the operation was resumed

**What to do**: Nothing. The system handles recovery automatically. Check the audit log afterward to verify completion.

---

### 5. Hardware Hotplug
If hardware is removed or fails during operation (e.g., BMC goes offline, SES enclosure loses connection), the system detects this and adapts.

**What happens**:
1. Health check detects hardware is no longer available
2. Features depending on that hardware are disabled
3. New operations using those features are rejected
4. Running operations continue
5. Event is logged to audit trail

**What you'll see**:
- Feature shows "Not Available" (was previously available)
- Operations that used the hardware fail with descriptive error
- Health status shows "Degraded"

**What to do**:
- Restore the hardware
- System auto-detects it on next startup
- Features automatically re-enable if hardware returns

---

### 6. Health Status
Check `/api/health` endpoint (or Dashboard → System Health) anytime to see overall status.

**Health Levels**:
- **Green (OK)**: All systems operational, all features available
- **Yellow (Degraded)**: Some systems degraded or features unavailable, but core functionality works
- **Red (Unavailable)**: Critical systems down, core functionality at risk

**Per-System Status**:
- `zfs`: ZFS pools and datasets online
- `docker`: Container runtime responsive
- `postgres`: Database responsive
- `network`: Network interfaces up, routing working
- `resources`: Disk/memory/FD within safe limits
- `external_services`: LDAP, OIDC, NFS connectivity
- `bmc` (if hardware present): BMC accessible via IPMI/Redfish
- `storage_monitoring`: Drive health (S.M.A.R.T or SES)
- `bonding` (if configured): Network bond status
- `vlan` (if configured): VLAN configuration valid

**What to do**: Check each failed subsystem and fix (e.g., if ZFS is down, check pool status; if LDAP is down, restore the server).

---

### 7. Audit Trail
Every significant event is logged: operations, feature changes, resource warnings, hardware detection, rollbacks, auth failures.

**Where to find it**: Admin → Audit Log (or query `/api/audit/events`)

**Entries include**:
- What happened (operation created, feature enabled, resource critical, etc.)
- When it happened (timestamp)
- Who did it (user ID)
- Result (success/failure)
- Details (what resource was affected, how much was used, etc.)

**Integrity**: Audit entries are cryptographically chained. If any entry is modified or deleted, the chain breaks and the tampering is detectable.

**What to do**: Review periodically for unusual activity. Export for compliance audits.

---

## Common Scenarios

### Scenario: Disk Almost Full

**Symptoms**: 
- Operations rejected with 507 Insufficient Storage
- Audit log shows "Resource Critical: disk 98%"

**Recovery**:
1. Identify what's using disk space (Pools → Used/Available)
2. Delete old snapshots or replicas
3. Or, add a new vdev to expand pool capacity
4. Watch dashboard, status returns to Green once disk < 95%

**Prevention**: Set up automated snapshot cleanup policies.

---

### Scenario: LDAP Server Offline (but NAS is running)

**Symptoms**:
- Dashboard shows "degraded" for "external_services"
- Login fails (LDAP needed for auth)
- File serving still works (doesn't need LDAP)

**Recovery**:
1. Restore LDAP server, or
2. In Settings → Features, disable "Active Directory Integration"
3. Switch to local user authentication temporarily
4. Once LDAP is back, re-enable the feature

**What doesn't break**:
- Snapshots
- Replication
- Storage access (if local auth is configured)
- Data integrity

---

### Scenario: Daemon Crashes During Pool Creation

**Symptoms**:
- Appliance reboots or service restarts
- Pool creation appears to hang

**Recovery (Automatic)**:
1. Appliance boots
2. Finds "pool_create" operation in incomplete state
3. Resumes from last checkpoint
4. Completes the pool creation
5. Logs the resumption in audit trail

**What you see**:
- Operation completes successfully (may take a few extra minutes)
- No data loss
- Audit log shows operation was resumed

**What to do**: Check dashboard to verify operation completed. If it doesn't, check logs for errors.

---

### Scenario: BMC Goes Offline

**Symptoms**:
- Dashboard shows BMC health as "Degraded"
- HA feature becomes unavailable
- Other features unaffected

**Recovery**:
1. System detects BMC offline on next health check
2. HA feature is disabled (depends on BMC)
3. HA operations are rejected: "Feature not available"
4. Non-HA operations continue normally

**What to do**:
- Restore BMC (reboot BMC, check network)
- Restart appliance to re-detect hardware
- HA feature re-enables automatically
- Resume HA operations

**Meantime**: System is fully functional without HA. Single-node mode works normally.

---

## Feature Availability

Some features require specific hardware or external services. If hardware is missing, the feature is disabled automatically.

| Feature | Requires | Disabled If |
|---------|----------|-------------|
| HA Clustering | BMC (Redfish/iLO/IPMI) | BMC offline or not present |
| NVMe-oF | NVMe controllers | No NVMe controllers detected |
| SES Enclosure Monitoring | SES enclosure | Enclosure offline or not present |
| Active Directory | Network + DNS | Network down or LDAP unavailable |
| OIDC SSO | Network | Network down or OIDC provider unavailable |
| Network Bonding | Network interfaces | Bonds misconfigured |

**What to do**: Check Settings → Features to see what's available on your hardware. Unavailable features will show as "Not Available".

---

## Monitoring & Alerts

### Health Endpoints (for integration with monitoring tools)

- **GET /api/health** → Overall system status (JSON)
- **GET /api/health?subsystem=zfs** → Specific subsystem (JSON)
- **GET /api/health/live** → Liveness probe (200 = alive, 503 = dead)
- **GET /api/health/ready** → Readiness probe (200 = ready, 503 = not ready)

### Recommended Monitoring

Set up alerts for:
- Health status = Red for > 5 minutes
- Resource critical (disk > 95%) 
- Circuit breaker open for > 10 minutes
- Audit chain verification failure (indicates tampering)

---

## Troubleshooting

### "Feature not available"
**Cause**: Hardware requirement not met (e.g., no BMC for HA), or external service offline (e.g., LDAP for AD)

**Fix**: 
- Check hardware (BMC online? NVMe present?)
- Check external services (LDAP reachable? Network up?)
- Or, disable the feature in Settings if you don't need it

### "Insufficient Storage" on new operations
**Cause**: Disk > 95% full

**Fix**:
- Delete old snapshots
- Delete old replicas
- Expand pool with new vdev
- (Or compress datasets to reclaim space)

### "Too Many Requests" on new operations
**Cause**: Memory > 90% or file descriptors > 95% in use

**Fix**:
- Restart Docker containers (` docker restart <container>`)
- Reduce concurrent operations (wait for some to complete)
- Restart appliance if FD limit keeps hitting

### Operation stuck (shows as "in progress" for hours)
**Possible causes**:
- Normal (large operations like migration take time)
- Daemon crashed and recovery failed
- External service unavailable

**What to do**:
1. Check audit log, was operation resumed?
2. Check resource health, is system low on disk/memory?
3. Check external services, are LDAP/NFS online?
4. If truly stuck, restart the appliance (recovery will retry)

### Audit log shows "Rollback Applied"
**What happened**: An operation failed, and the system automatically reverted changes to keep the system consistent.

**What to do**:
- Check the Details field for why rollback occurred
- Address the root cause (hardware incompatibility, resource shortage, etc.)
- Retry the operation

---

## Best Practices

1. **Monitor health regularly**: Check `/api/health` weekly, especially after hardware changes.

2. **Keep external services healthy**: LDAP, NFS, and NTP unavailability will degrade functionality. Monitor these.

3. **Maintain disk headroom**: Keep disk < 80% full. Don't wait for 95% critical threshold.

4. **Review audit logs monthly**: Look for patterns (e.g., repeated feature disables suggest hardware issue).

5. **Test recovery**: Simulate a crash by hard-reboot during an operation. Verify operation resumes correctly.

6. **Document your hardware**: Keep a record of BMC firmware version, NVMe controller models, etc. Useful when debugging compatibility issues.

---

## Support

If something breaks:
1. Check `/api/health` to understand what's down
2. Check audit log for error details
3. Check system logs (Admin → Logs)
4. Contact support with:
   - Health status (output of `/api/health`)
   - Audit log entries from the failure time
   - System logs
   - Hardware details (BMC type, NVMe models, etc.)
