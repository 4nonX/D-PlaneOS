# HA Stabilization Roadmap - DPlaneOS v14.6.0

## Executive Summary

HA in v14.6.0 is **architecturally sound but operationally experimental**. The cluster logic, fencing guards, and split-brain prevention are well-implemented and CI-tested. However:

- **PostgreSQL HA (Patroni + etcd)** has never been load-tested at scale; risk of failover timing issues or state inconsistency under sustained I/O
- **CTDB (Clustered SMB)** is not implemented; SMB clients disconnect on failover and lose byte-range locks
- **SCSI-3 PR fencing** is untested against diverse hardware; path-specific validation required
- **Replicated topology** (ZFS send/recv) has limited production deployment history
- **Documentation** focuses on architecture, not operational failure modes and recovery procedures

**Honest recommendation**: Use single-node with backups/replication to external storage until HA passes load testing.

---

## Critical Path to Production Readiness

### Tier 1: Must-Fix Before Production HA

#### 1.1 PostgreSQL HA Load Testing
**Risk**: Patroni failover under sustained I/O may expose timing bugs, transaction state inconsistency, or data corruption scenarios.

**Current State**:
- Unit tests cover split-brain guard logic (9 tests in `failover_integration_test.go`)
- NixOS VM test in CI (`ha-failover.nix`) spins up two VMs, verifies Patroni election and etcd quorum
- **Missing**: Sustained workload testing (concurrent connections, long-running transactions, failover under load)

**Minimal Viable Fix**:
1. Create a load-test scenario:
   - Two-node cluster with shared-SAS storage path
   - pgbench running against the primary (100 connections, 30 min run)
   - Trigger failover mid-load (kill primary node or pause network)
   - Validate:
     - Standby promotes within 60 seconds
     - No transaction loss or duplication
     - Secondary catches up and resync with no divergence
     - Repeat failover; confirm no state corruption

2. Automate in CI pipeline:
   - Add `ha-load-test.yml` job (60 min timeout, large runner)
   - Check for Patroni WAL lag, PostgreSQL error logs, ZFS TXG divergence
   - Mark as optional (non-blocking) for now, but required after 2-3 clean runs

**Effort**: 1-2 weeks (test harness + pgbench playbook + CI integration)

**Why critical**: Data corruption is the failure mode we cannot recover from. Load testing is the only way to expose timing-dependent bugs.

---

#### 1.2 Operator Runbooks for HA Failure Modes
**Risk**: When a failover happens, operators may not know which node is authoritative, how to recover without split-brain, or what to do if recovery stalls.

**Current State**:
- HIGH-AVAILABILITY.md explains architecture and setup
- INTEGRATION-GUIDE.md mentions CTDB limitation
- **Missing**: Operational runbooks for:
  - "My failover completed but node B has stale data"
  - "Failover never triggered - peer was unreachable for 5 minutes"
  - "Hysteresis is blocking promotion - what happened?"
  - "How to safely reboot the primary without losing quorum"
  - "Witness node is down - how long can we run?"
  - "SCSI-3 PR fencing failed - what's the fallback?"

**Minimal Viable Fix**:
1. Create `docs/admin/HA-FAILURE-MODES.md`:
   - Enumerate 8-10 real failure scenarios
   - For each: root cause, detection symptoms, recovery procedure (who to promote, how to validate)
   - Include commands to inspect cluster state (`ha/status` API, `etcdctl`, `patronictl`, `zpool status`)
   - Include "never do this" list (split-brain risks)

2. Add to HIGH-AVAILABILITY.md:
   - Link to failure modes doc in "Troubleshooting" section
   - Add "Post-Failover Validation Checklist" before resuming normal ops

**Effort**: 2-3 days (runbook writing, testing commands against real HA cluster)

**Why critical**: Operators need a decision tree when things go wrong. Without one, panic-driven manual actions (forcing promotion, reimporting pools) cause data loss.

---

### Tier 2: Must-Have Before Recommending HA

#### 2.1 CTDB Implementation (SMB Clustering)
**Risk**: HA clusters without CTDB experience SMB client downtime on failover. Byte-range locks are lost, forcing applications to reconnect.

**Current State**:
- Samba is configured as a shared filesystem service (not clustered)
- On failover, active Samba process goes offline; secondary reacquires VIP but SMB daemon is not coordinated
- Winbind NSS integration exists (AD user/group resolution)
- **Missing**: Samba CTDB daemon coordination and NSS hooks

**Architectural Decision Needed**:
DPlaneOS has two options:

**Option A: Pacemaker/Corosync + CTDB** (TrueNAS approach)
- Add Pacemaker cluster management on top of existing HA layer
- Coordinate Samba CTDB startup/shutdown with node roles
- Pros: Production-proven, enterprise-grade
- Cons: Double cluster stack (HA + Pacemaker); complex recovery

**Option B: In-Process CTDB Lite** (simpler, custom)
- Lightweight CTDB reimplementation in Go
- Integrate directly into the HA cluster manager
- Coordinate Samba config (ctdbd, shared lock DB) on promotion
- Pros: Single cluster stack, simpler ops
- Cons: Unproven approach, requires careful testing

**Recommendation**: Option B for now (lower risk of introducing Pacemaker complexity). Implement as:
1. Samba with CTDB enabled (NixOS: `samba.ctdb.enable = true`)
2. HA manager triggers `systemctl restart ctdbd` + `systemctl restart samba` on promotion
3. Replicated CTDB lock database on shared storage (for Path A) or ZFS replicated (for Path B)

**Minimal Viable Fix**:
1. Add CTDB NixOS module integration (flake.nix, nixos/module.nix)
2. Configure CTDB socket location in daemon (`/var/lib/ctdb/ctdbd.socket`)
3. Add HA promotion hook: trigger CTDB node setup on new primary
4. Test: SMB client connects, hold byte-range lock, failover, verify lock survives
5. Document in HIGH-AVAILABILITY.md: "SMB reconnect behavior" section

**Effort**: 3-4 weeks (CTDB NixOS packaging, daemon integration, testing)

**Why necessary**: Without CTDB, SMB HA is theater. It technically works but creates downtime on failover (unacceptable for file sharing use cases).

---

#### 2.2 Improved Monitoring & Alerting for HA
**Risk**: Cluster degrades silently. Operators don't realize they're running single-node until the primary fails.

**Current State**:
- Prometheus metrics for cluster status (ClusterStatus API exported)
- Alerting thresholds not documented
- **Missing**: Dashboards and alerts for:
  - Peer unreachable for >5 min (escalates to failover risk)
  - Replication lag on secondary (for Path B)
  - Patroni WAL lag or streaming replication lag
  - Witness unreachable
  - SCSI-3 PR reservation lost (storage fencing broken)

**Minimal Viable Fix**:
1. Create Prometheus alerts (prometheus/alerts.yml):
   ```yaml
   - alert: HAPeerUnreachable
     expr: ha_peer_missed_beats > 3  # >45 seconds
     for: 1m
     annotations:
       summary: "HA peer {{ $labels.peer_id }} unreachable - failover may trigger"
   
   - alert: HAReplicationLagHigh
     expr: ha_replication_lag_seconds > 300  # >5 min
     for: 5m
     annotations:
       summary: "HA replication lag {{ $value }}s - data loss risk at failover"
   
   - alert: HAFencingDisabled
     expr: ha_fencing_enabled == 0
     annotations:
       summary: "HA fencing disabled - split-brain risk"
   ```

2. Add Grafana dashboard: `dashboards/ha-cluster.json`
   - Single-stat cards: peer state, witness count, last failover time
   - Timeseries: peer latency, replication lag, Patroni WAL position
   - Heatmap: cluster composition changes over time

3. Add to operator docs:
   - "Golden state" values (what good looks like)
   - "Degradation progression" (how to recognize early trouble)
   - "Alert escalation" (when to page vs. log)

**Effort**: 1 week (alert rules, Grafana dashboard, docs)

**Why necessary**: Operators need real-time visibility. Without it, they run blind and discover problems at failover time.

---

### Tier 3: Nice-to-Have Post-Production

#### 3.1 SCSI-3 PR Hardware Validation Matrix
**Risk**: PR support varies by disk controller, JBOD firmware, and SAN vendor. A cluster works in lab but fails in production on a different array.

**Current State**:
- `POST /api/ha/scsi/probe` tests PR capability per-device
- No registry of tested hardware combinations
- No automated re-validation on firmware updates

**Minimal Viable Fix**:
1. Create `docs/reference/HA-HARDWARE-MATRIX.md`:
   - Table: Controller / JBOD / SAN model + firmware version -> PR supported (Y/N)
   - Link to each vendor's PR support documentation
   - Crowdsource entries from deployments (add to TROUBLESHOOTING.md)

2. Add to setup wizard:
   - Auto-detect controller model (dmidecode, lsscsi)
   - Check against matrix; warn if hardware is untested
   - Require explicit "I understand the risks" ACK if untested

**Effort**: 2 days (table creation, setup wizard integration)

**Why useful**: De-risks hardware surprises for future deployments.

---

#### 3.2 Chaotic Failover Testing Framework
**Risk**: Failure modes aren't exercised in controlled scenarios; discovering them in production is expensive.

**Current State**:
- Static HA tests in CI (guard logic, witness reachability)
- No "kill the primary in the middle of a transaction" scenarios

**Minimal Viable Fix**:
1. Create test binary: `daemon/cmd/ha-chaos-test/main.go`
   - Sets up two-node HA cluster (shared storage or replicated)
   - Runs pgbench + SMB client workloads
   - Injects failures: network partition, primary crash, witness outage, fencing timeout
   - Validates: no data loss, no split-brain, failover completes in <30s

2. Run manually on nightly basis against staging cluster

**Effort**: 2 weeks (harness + 5-6 scenario tests)

**Why nice-to-have**: Builds confidence in edge cases; payoff is long-term incident prevention.

---

## Summary Scorecard

| Component | Current | Status | Risk Level | Stabilization Effort |
|-----------|---------|--------|------------|----------------------|
| **Cluster logic & fencing** | Proven | ✅ Stable | Low | None |
| **PostgreSQL HA (Patroni)** | Lab-tested | 🟡 Experimental | High | 1-2 wk (load test) |
| **CTDB / SMB clustering** | Not implemented | ❌ Gap | High | 3-4 wk (impl) |
| **Monitoring & alerting** | Basic | 🟡 Incomplete | Medium | 1 wk (dashboards) |
| **Operator docs & runbooks** | Missing | ❌ Gap | High | 2-3 days |
| **SCSI-3 PR validation** | Probing works | 🟡 Hardware-dependent | Medium | 2 days (matrix) |
| **Replicated topology** | Lab-tested | 🟡 Limited history | Medium | 1-2 wk (load test) |

---

## Minimum Viable HA (for production use)

**Complete all Tier 1 items:**
1. Load-test PostgreSQL failover (Patroni)
2. Write operator runbooks (failure modes, recovery)

**Strongly recommended before first production HA deployment:**
- Tier 2.2 (monitoring & alerts)

**Can defer to next release:**
- Tier 2.1 (CTDB) - if SMB HA downtime is acceptable
- Tier 3 (validation matrix, chaos testing)

---

## Recommendation: Near-Term Action Items (This Sprint)

1. **Start**: Load-testing framework setup (1-2 days)
   - Provision two-node test cluster (VMs or hardware)
   - Run pgbench baseline failover
   - Document in TEST-HARNESS.md

2. **Write**: HA Failure Modes document (2-3 days)
   - Enumerate 10 real scenarios
   - Recovery procedures for each
   - Link from HIGH-AVAILABILITY.md

3. **Add**: Prometheus alerts + Grafana dashboard (1 week)
   - Monitor cluster health in real-time
   - Catch degradation before failover

4. **Decide**: CTDB or accept SMB downtime?
   - Poll users: is byte-range lock continuity mandatory?
   - If yes: scope CTDB implementation for next release
   - If no: document SMB reconnect behavior clearly

5. **Tag as Experimental**: Update docs and CHANGELOG
   - HIGH-AVAILABILITY.md: clear warning at top
   - Docs index: link to failure modes and load-testing status
   - CHANGELOG: "HA is experimental; load testing in progress"

---

## Success Criteria for GA (v15.0.0?)

- [ ] Patroni failover passes 3+ consecutive load-test runs (no data loss)
- [ ] Operator runbooks cover 90%+ of support tickets
- [ ] Cluster monitoring alerts catch degradation 5+ min before failover
- [ ] First 5 production deployments (N > 1 site) report no data loss
- [ ] CTDB implemented OR explicitly documented that SMB downtime is acceptable
