# DPlaneOS HA: Failure Modes and Recovery Procedures

## Overview

This guide covers real failure scenarios in DPlaneOS HA clusters and step-by-step recovery procedures. Use this when something goes wrong, not as architecture documentation. For design background, see [HIGH-AVAILABILITY.md](HIGH-AVAILABILITY.md).

**Before any manual intervention:** Always check the cluster state first using the commands in "Inspecting Cluster State" below. Wrong diagnoses lead to split-brain or data loss.

---

## Inspecting Cluster State

Keep these commands in a terminal and run them frequently:

```bash
# From either node or operator workstation (requires network access to node)

# HA cluster status (summary of peer state, quorum, last failover)
curl -s http://NODE_IP:5000/api/ha/status | jq '.'

# Is this node the Patroni leader?
curl -s http://localhost:8008/primary
# Returns 200 (primary/leader) or 503 (replica/follower)

# PostgreSQL replication lag (on replica)
psql -h localhost -d dplaneos -c "SELECT NOW() - pg_last_xact_replay_timestamp() AS replication_lag;"

# Patroni cluster state
patronictl -c /etc/dplaneos/patroni.yaml list

# etcd member status
etcdctl --endpoints=http://127.0.0.1:2379 member list

# ZFS pools on this node
zpool list

# Are pools imported? (standby should have NO pools imported)
zpool status

# Recent HA logs
journalctl -u dplaned -n 50 --no-pager
journalctl -u patroni -n 50 --no-pager

# SCSI-3 PR reservation status (Path A topology only)
curl -s http://NODE_IP:5000/api/ha/scsi/status | jq '.'

# Network witness (if configured)
curl -s http://NODE_IP:5000/api/ha/network-witness/test | jq '.'
```

---

## Scenario 1: One Node Unreachable (Network Partition)

### Symptoms
- One peer shows as "unreachable" in `/api/ha/status`
- Missed heartbeats steadily increase (3+)
- Last seen time stopped updating
- The cluster may still be functional (other node is primary)

### Root Causes
- Network link down (cable unplugged, NIC failed)
- Firewall blocking HA heartbeat port (5000)
- Peer dplaned process crashed/hung
- Peer OS hung (not responding to pings)
- Network packet loss on core switch

### Recovery Procedure

**Step 1: Check if the peer is actually down**
```bash
# From the healthy node, try to reach the unreachable peer
ping UNREACHABLE_NODE_IP
ssh UNREACHABLE_NODE_USER@UNREACHABLE_NODE_IP "curl -s http://localhost:8008/primary"
```

**Case A: Peer is reachable (network partition false alarm)**
- Network transient recovery; cluster will auto-heal
- Peer should rejoin within 30 seconds (next heartbeat)
- No action needed

**Case B: Peer is unreachable**
- Go to Step 2

**Step 2: Determine if you are the primary**
```bash
curl -s http://localhost:8008/primary
```
- Status 200: You are primary. Go to Step 3.
- Status 503: You are replica. Go to Step 4.

**Step 3: YOU ARE PRIMARY, PEER IS UNREACHABLE**

**Option A: Wait for automatic failover (recommended)**
- Do nothing for 30-60 seconds
- Watchdog timeout timer started (if configured)
- If witness is configured and reachable: secondary will promote automatically
- If NO witness: STONITH (fencing) must succeed to promote secondary; if fencing is disabled, primary continues running alone

```bash
# Monitor promotion progress
watch -n 5 'curl -s http://localhost:8008/primary && echo "Primary OK" || echo "Primary down"'
```

**Option B: Manual acknowledgment (if automatic failover is blocked)**

Check why failover didn't trigger:
```bash
# Read HA status
curl -s http://NODE_IP:5000/api/ha/status | jq '.{hysteresis_active, maintenance_active, subordinate_mode}'
```

**If hysteresis_active=true**: Failover is rate-limited after a recent promotion
```bash
# Clear the fault (resets hysteresis and subordinate mode)
curl -X POST http://NODE_IP:5000/api/ha/clear_fault
# Then manually trigger failover:
# 1. Isolate primary: power down or network isolate it
# 2. Wait 60 seconds for fence timeout
# 3. Secondary should promote automatically
```

**If maintenance_active=true**: Failover is suspended for maintenance
```bash
# Disable maintenance mode
curl -X POST http://NODE_IP:5000/api/ha/maintenance/disable
```

**If fencing=disabled AND secondary is not promoting**: Cluster runs as 1-node
- No data loss, but redundancy is gone
- Enable fencing (IPMI, PDU, or watchdog) to get automatic failover

**Step 4: YOU ARE REPLICA, PEER IS UNREACHABLE**

You are NOT primary. Sit tight - primary holds all writes.

**Wait for automatic failover:**
```bash
# Monitor cluster state
for i in {1..20}; do
  echo "=== Attempt $i ==="
  curl -s http://localhost:8008/primary | grep -q "200" && echo "Promoted to primary!" && break
  sleep 3
done
```

If promotion doesn't happen after 60 seconds:
```bash
# Primary is still running (quorum maintained or no witness)
# Check primary status
curl -s http://PRIMARY_IP:5000/api/ha/status | jq '.{local_node, quorum, last_failover_at}'

# Primary is responsible for continuing writes
# This is safe - you are not isolated, primary is authoritative
```

**If you need to become primary NOW (emergency):**
```bash
# Only do this if you've confirmed primary node is physically dead

# Clear fault and force promotion
curl -X POST http://NODE_IP:5000/api/ha/clear_fault

# This removes hysteresis lock; next heartbeat miss triggers promotion
# BUT: ensure primary is actually unreachable first!
```

---

## Scenario 2: Failover Stalled (Secondary Doesn't Promote)

### Symptoms
- Primary node is unreachable for >60 seconds
- Secondary is still reporting as "standby" role
- Status shows `last_failover_at` unchanged
- No new primary elected (both nodes show 503 on `/primary`)

### Root Causes
- Witness unreachable (split-brain protection blocking promotion)
- Maintenance mode enabled (failover suspended)
- Hysteresis window active (rate-limiting after recent failover)
- Subordinate mode active (node was demoted mid-sync)
- Fencing/PDU disabled and watchdog self-fence didn't fire
- Patroni communication down (etcd not quorum)

### Recovery Procedure

**Step 1: Check HA status**
```bash
curl -s http://SECONDARY_IP:5000/api/ha/status | jq '.'
```

**Step 2: Identify blocking condition**
```bash
# All checks must pass for promotion:
# 1. Secondary must be standby (check: role)
# 2. Primary must be unreachable (check: peers[0].state)
# 3. No maintenance mode (check: maintenance_active)
# 4. Hysteresis window expired (check: hysteresis_active)
# 5. Subordinate mode off (check: subordinate_mode)
# 6. Fencing enabled OR witness reachable (check: ha_enabled)
```

**Step 3: Clear blocking conditions**

**If maintenance_active=true:**
```bash
curl -X POST http://SECONDARY_IP:5000/api/ha/maintenance/disable
```

**If hysteresis_active=true:**
```bash
# Failover was suppressed by rate limit (too recent). Options:
# Option A: Wait for hysteresis to expire (default 60 min)
# Option B: Emergency override (careful - can cause flapping)
curl -X POST http://SECONDARY_IP:5000/api/ha/clear_fault
```

**If subordinate_mode=true:**
```bash
# Node was demoted mid-data-sync. It's still catching up.
# Do NOT promote while catching up (data loss risk).
# Options:
# Option A: Wait for catch-up (may take minutes to hours)
#   watch 'curl -s http://SECONDARY_IP:5000/api/ha/replication/status'
# Option B: Emergency override (if you've confirmed primary is permanently dead)
curl -X POST http://SECONDARY_IP:5000/api/ha/clear_fault
```

**If witness unreachable:**
```bash
# Check witness status
curl -s http://SECONDARY_IP:5000/api/ha/status | jq '.peers[] | select(.id=="witness-1")'

# Witness availability is split-brain protection:
# If witness is down and primary is down, secondary cannot safely decide
# who is authoritative. Options:
# Option A: Restore witness node to network
# Option B: If primary is CONFIRMED permanently dead:
curl -X POST http://SECONDARY_IP:5000/api/ha/clear_fault
# Then wait for next heartbeat miss to trigger promotion
```

**If no fencing configured AND witness unreachable:**
```bash
# This is the "both guards blocked" case. Cluster cannot auto-failover.
# Options:
# Option A: Enable IPMI/PDU fencing or watchdog
# Option B: Manually isolate primary (power off, network unplug)
#   and wait for watchdog timeout to fire (if configured)
# Option C: Manually trigger promotion (only if primary is physically gone)
#   1. Confirm primary is unreachable: ssh, ping, OOB console all fail
#   2. Clear fault: curl -X POST http://SECONDARY_IP:5000/api/ha/clear_fault
#   3. Monitor promotion: curl http://SECONDARY_IP:5000/api/ha/status
```

**Step 4: If promotion still doesn't fire**

Cluster has hit a split-brain safety wall. Don't force promotion (data corruption risk).

**Emergency manual promotion (DATA LOSS RISK - LAST RESORT ONLY):**
```bash
# STOP: Only proceed if you are 100% certain primary is physically dead
# (no power, no network, no IPMI console response, etc.)

# 1. On secondary, clear all safeguards
curl -X POST http://SECONDARY_IP:5000/api/ha/clear_fault

# 2. Manually promote in Patroni (bypasses HA layer)
patronictl -c /etc/dplaneos/patroni.yaml failover --candidate=SECONDARY_HOST

# 3. Verify exactly one primary
curl http://SECONDARY_IP:8008/primary  # should return 200
curl http://PRIMARY_IP:8008/primary 2>/dev/null || echo "Primary unreachable (expected)"

# 4. If primary node returns online later, it MUST rejoin as replica
# If it comes back as primary, you have split-brain - SHUT DOWN ONE NODE immediately
```

---

## Scenario 3: Split-Brain Detected (Both Nodes Think They're Primary)

### Symptoms
- Both nodes return 200 on `/primary` (both act as leaders)
- Data writes happening on both nodes
- Patroni logs show "I am the primary but there's another primary"
- Cluster status reports 2 active nodes

### Root Causes
- Network partition healed while both nodes were promoted (rare if fencing works)
- Manual promotion on secondary without ensuring primary was fenced
- Patroni/etcd network split allowing simultaneous elections
- Fencing mechanism failed silently

### Recovery Procedure

**IMMEDIATE ACTIONS - Data Integrity At Risk**

1. Stop all applications from writing immediately
2. Isolate one node (preferred: isolate the secondary that was promoted)

```bash
# Option A: Network isolate (fastest)
ssh node-to-isolate "ip link set eth0 down"  # Immediately break network

# Option B: Service stop (graceful)
ssh node-to-isolate "systemctl stop dplaned patroni postgresql etcd"
```

3. Wait 30 seconds for the isolated node to release any locks

**Step 2: Determine which node is authoritative**

The PRIMARY (original leader before failover) is always authoritative:
```bash
# Check which node was elected FIRST (earliest timestamp)
curl http://PRIMARY_IP:5000/api/ha/status | jq '.last_failover_at'
curl http://SECONDARY_IP:5000/api/ha/status | jq '.last_failover_at'

# Lower timestamp = was primary first = is authoritative
# This node keeps its data; the other node's data must be discarded
```

**Step 3: Promote the authoritative node**
```bash
# On the authoritative node, verify it's primary
curl http://AUTHORITATIVE_IP:8008/primary  # should return 200

# On the non-authoritative node (still isolated), verify data divergence
ssh node-to-isolate "psql -d dplaneos -c 'SELECT MAX(id) FROM pgbench_history;'"
# This will show fewer rows than authoritative node

# Do NOT rejoin this node to the cluster yet; its data is stale
```

**Step 4: Resync the secondary**

Option A: Full rebuild (safe, slowest)
```bash
ssh node-to-isolate "
  # Stop and reset PostgreSQL
  systemctl stop postgresql
  rm -rf /var/lib/postgresql/*
  systemctl start postgresql
  
  # Rejoin the cluster
  systemctl start patroni etcd dplaned
"

# Patroni will replicate data from primary (may take minutes to hours)
# Monitor: watch 'curl http://node-to-isolate:8008/replica'
```

Option B: Patroni reinitialize (faster)
```bash
patronictl -c /etc/dplaneos/patroni.yaml reinit --role=replica node-to-isolate
```

**Step 5: Verify cluster consistency**

```bash
# Both nodes should have same data
psql -h PRIMARY -c "SELECT COUNT(*) FROM pgbench_history;"
psql -h SECONDARY -c "SELECT COUNT(*) FROM pgbench_history;"
# Should match

# Exactly one primary
curl http://PRIMARY:8008/primary        # 200
curl http://SECONDARY:8008/primary      # 503
```

---

## Scenario 4: Witness Node Down (Quorum Witness Unreachable)

### Symptoms
- One of the witness nodes is offline
- `last_seen` timestamp for witness is stale
- Promotion still works if primary fails (as long as majority of witnesses reachable)

### Root Causes
- Witness node crashed or powered off
- Network partition between cluster and witness
- Witness node disk full or out of memory
- etcd process crashed on witness

### Recovery Procedure

**Step 1: Check how many witnesses are configured**
```bash
curl -s http://NODE_IP:5000/api/ha/status | jq '.peers[] | select(.id | startswith("witness"))'
```

**Step 2: Calculate quorum requirement**
- If 1 witness configured: must be reachable (1/1)
- If 3 witnesses configured: need 2/3 reachable
- Failover is blocked if quorum of witnesses is lost

**Step 3: Restore the witness**

If it's a temporary outage (waiting for recovery):
```bash
# Monitor witness status
watch 'curl -s http://NODE_IP:5000/api/ha/network-witness/test'
```

If the witness node has failed permanently:
```bash
# Remove from witness list
curl -X POST http://NODE_IP:5000/api/ha/witness/remove \
  -H "Content-Type: application/json" \
  -d '{"witness_id":"witness-1"}'

# Add replacement witness (if available)
curl -X POST http://NODE_IP:5000/api/ha/witness/add \
  -H "Content-Type: application/json" \
  -d '{
    "address": "http://NEW_WITNESS_IP:5000",
    "id": "witness-2"
  }'
```

**Step 4: Verify quorum is maintained**
```bash
curl -s http://NODE_IP:5000/api/ha/status | jq '.quorum'
# Should be true if majority of witnesses reachable
```

---

## Scenario 5: Replication Lag High (Secondary Far Behind Primary)

### Symptoms
- Secondary's replication lag is >300 seconds (5 minutes)
- Cluster status shows large bytes-behind value
- Recovery Point Objective (RPO) violated

### Root Causes
- Network link slow or congested between nodes
- Secondary disk is I/O bottleneck
- Primary is under heavy write load
- Replication was intentionally paused (maintenance)

### Recovery Procedure

**Step 1: Check replication status**
```bash
# On secondary
psql -h localhost -d dplaneos -c "SELECT NOW() - pg_last_xact_replay_timestamp() AS replication_lag;"

# On primary
psql -h localhost -d dplaneos -c "SELECT name, sent_lsn, write_lsn, flush_lsn, replay_lsn FROM pg_stat_replication;"
```

**Step 2: Identify bottleneck**

**If lag is increasing:** Secondary cannot keep up
```bash
# Check secondary disk I/O
iostat -x 2 5 | grep -E "Device|sda|nvme"  # Look for high util%

# Check secondary network
iftop -i eth1 -n  # Watch byte rate to primary

# Check Secondary load
uptime
free -h
```

**If lag is stable:** Replication is working, just larger than desired
- No action needed; continue monitoring
- If RPO is critical, add capacity to secondary

**If lag is decreasing:** Secondary is catching up (good sign)
- Monitor until lag drops to acceptable level

**Step 3: Accelerate catch-up (if urgent)**

Option A: Reduce primary write load (maintenance)
```bash
# Pause application writes
systemctl stop dplaned  # Stops API; pending operations drain

# Monitor lag
watch 'psql -d dplaneos -c "SELECT NOW() - pg_last_xact_replay_timestamp();"'

# Resume when lag < 1 second
systemctl start dplaned
```

Option B: Increase replication priority (Patroni)
```bash
# Tell Patroni secondary is more important
patronictl -c /etc/dplaneos/patroni.yaml set-synchronous-members SECONDARY_HOST
```

Option C: (Path B only) Reduce replication interval temporarily
```bash
# Increase replication frequency (more overlapping syncs)
curl -X POST http://NODE_IP:5000/api/ha/replication/config \
  -H "Content-Type: application/json" \
  -d '{"interval_secs": 10}'  # Default is 30
  
# Monitor: watch -n 2 'curl -s http://NODE_IP:5000/api/ha/replication/status | jq .lag_seconds'
```

---

## Scenario 6: Patroni Cannot Elect Primary (etcd Quorum Lost)

### Symptoms
- `/primary` returns connection refused on both nodes
- Patroni logs: "could not get the leader lock from DCS"
- Dplaned is running, but PostgreSQL is not accepting connections
- Status shows "unknown" or repeated election attempts

### Root Causes
- etcd cluster has no quorum (fewer than 50% of members reachable)
- etcd network partition
- etcd data corruption (rare)
- All three etcd nodes crashed

### Recovery Procedure

**Step 1: Check etcd health**
```bash
etcdctl --endpoints=http://127.0.0.1:2379 endpoint health
etcdctl --endpoints=http://127.0.0.1:2379 member list
```

**Step 2: Understand etcd quorum**
- 3-member cluster: need 2 alive
- 1-member cluster (single node): works alone
- 2-member cluster: either can partition (no quorum possible)

**Step 3: Restore quorum**

If 1 etcd member is down (3-member cluster):
```bash
# Member must be reachable; if not, remove it
DEAD_MEMBER_ID=$(etcdctl member list | grep UNREACHABLE | awk '{print $1}')
etcdctl member remove $DEAD_MEMBER_ID

# New member can be added later when available
```

If entire etcd cluster is down:
```bash
# On primary node, force etcd restart
systemctl restart etcd

# Patroni will detect etcd is back and elect primary
# Wait 30 seconds, then verify
curl http://localhost:8008/primary  # should return 200
```

If etcd data is corrupted (rare):
```bash
# Last resort: reset etcd (LOSES cluster config)
# THIS REQUIRES MANUAL INTERVENTION - consult with DPlaneOS support
systemctl stop patroni etcd
rm -rf /var/lib/etcd/*
systemctl start etcd
# Wait for cluster to reform
systemctl start patroni
```

---

## Scenario 7: SCSI-3 PR Fencing Failed (Path A Only)

### Symptoms
- Failover triggered but secondary cannot acquire SCSI-3 PR
- dplane-fenced logs: "PROUT failed: Operation not permitted"
- Primary was NOT evicted; secondary cannot access shared pool

### Root Causes
- Storage controller does not support SCSI-3 PR
- Disk firmware too old or missing PR support
- dplane-fenced process crashed or blocked
- Storage connection dropped during failover
- Incorrect reservation key configuration

### Recovery Procedure

**Step 1: Verify SCSI-3 PR support**
```bash
curl -s http://NODE_IP:5000/api/ha/scsi/probe | jq '.devices[] | {path, supported}'
# All devices should show "supported": true
```

**If any device shows false:** That disk does not support SCSI-3 PR
- Cannot use Path A topology with this hardware
- Migrate to Path B (replicated ZFS)
- Contact storage vendor; older firmware may be updateable

**Step 2: Check reservation state**
```bash
curl -s http://NODE_IP:5000/api/ha/scsi/status | jq '.'
# Should show "running": true and list reserved devices
```

**If not running:** dplane-fenced service is down
```bash
systemctl status dplane-fenced
systemctl restart dplane-fenced
```

**Step 3: Manually release old reservation (if stuck)**

This is rare; only if dplane-fenced crashed mid-failover:
```bash
# On secondary (new primary), release any lingering reservations
sg_persist --no-inquiry -k -C /dev/sg0  # Clear all registrations
sg_persist --no-inquiry -r -K 1 -S 1 /dev/sg0  # Re-register

# Verify reservation is held by secondary
sg_persist --no-inquiry --read-keys /dev/sg0
```

**Step 4: If fencing keeps failing**
```bash
# Disable SCSI-3 fencing (fallback to watchdog)
curl -X POST http://NODE_IP:5000/api/ha/fencing/config \
  -H "Content-Type: application/json" \
  -d '{"enable": false}'

# Enable watchdog self-fence instead
curl -X POST http://NODE_IP:5000/api/ha/watchdog/config \
  -H "Content-Type: application/json" \
  -d '{"enable": true, "timeout_secs": 30}'

# Cluster will still failover (via watchdog), just more slowly
```

---

## Scenario 8: Node Stuck in Subordinate Mode (Cannot Auto-Failover)

### Symptoms
- `/api/ha/status` shows `subordinate_mode: true`
- Node will not promote even if primary disappears
- Status message: "node is in Subordinate (catch-up) Mode"
- Failover permanently blocked until mode cleared

### Root Causes
- Node was promoted from stale data (zombie boot)
- Replication was incomplete when primary failed
- Manual demotion during sync (rare)

### Recovery Procedure

**Step 1: Understand subordinate mode**

Subordinate mode means: "I was promoted before finishing catch-up from the old primary; my data is stale."

While in this mode:
- The node runs as primary (serving clients)
- But auto-failover is BLOCKED to avoid data loss if partition heals
- If original primary comes back online, it rejoins as secondary

**Step 2: Clear subordinate mode**

Only safe if you're confident the node's data is now current:

```bash
# Check replication lag (should be near-zero if this was replica)
psql -d dplaneos -c "SELECT NOW() - pg_last_xact_replay_timestamp() AS lag;"

# Check transaction ID (should match other node)
psql -d dplaneos -c "SELECT txid_current();"
```

**If lag is zero and data looks correct:**
```bash
curl -X POST http://NODE_IP:5000/api/ha/clear_fault
# Subordinate mode is cleared
# Auto-failover re-enabled on next peer failure
```

**If lag is high or data looks stale:**
- Do NOT clear subordinate mode
- Investigate data divergence first
- Contact support if you're unsure

---

## Post-Failover Validation Checklist

After any failover (automatic or manual), use this checklist before resuming normal operations:

```
[ ] Exactly one Patroni primary
    curl http://NODE_A/8008/primary  # 200 or 503?
    curl http://NODE_B/8008/primary  # 200 or 503?
    Only one should return 200

[ ] Primary and secondary roles match expected topology
    curl -s http://NODE_IP/api/ha/status | jq '.peers[] | {id, role}'

[ ] No split-brain (all nodes agree on primary)
    curl -s http://NODE_A/api/ha/status | jq '.active_node.id'
    curl -s http://NODE_B/api/ha/status | jq '.active_node.id'
    Should be identical

[ ] Replication is running (secondary must not be too far behind)
    psql -h SECONDARY -d dplaneos -c "SELECT NOW() - pg_last_xact_replay_timestamp();"
    Should be < 5 seconds

[ ] Witness still reachable (if configured)
    curl -s http://NODE_IP/api/ha/status | jq '.peers[] | select(.id=="witness") | .state'
    Should be "healthy"

[ ] No errors in recent logs
    journalctl -u dplaned -u patroni -u etcd -n 100 | grep -i error

[ ] Applications can connect
    psql -h VIRTUAL_IP -d dplaneos -c "SELECT 1;"

[ ] Ready to resume write load
    Monitor replication lag and failover readiness for 5 minutes before resuming
```

---

## Emergency Contacts and Escalation

**If you encounter a scenario not covered above:**

1. Preserve logs:
   ```bash
   journalctl -u dplaned -u patroni -u etcd -u postgresql > /tmp/ha-logs.txt
   curl -s http://NODE_IP/api/ha/status > /tmp/ha-status.json
   zpool status > /tmp/zpool-status.txt
   ```

2. Do NOT make manual changes without understanding implications

3. Contact support with logs and status files

---

## Never Do This

These actions cause data loss or split-brain:

- ❌ Promote secondary WITHOUT confirming primary is unreachable
- ❌ Restart both nodes simultaneously
- ❌ Import pools manually (bypassing HA guards)
- ❌ Ignore split-brain alarms; shut down one node immediately
- ❌ Use `zfs rollback` to "go back in time" on primary
- ❌ Bypass Patroni to make PostgreSQL changes directly
- ❌ Delete etcd data to "reset" cluster state
- ❌ Re-enable fencing after disabling it (timing issues)
