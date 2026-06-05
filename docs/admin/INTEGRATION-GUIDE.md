# DPlaneOS Integration Guide

This document covers integrating DPlaneOS with external systems. Each section is honest about what works, what has known limitations, and what to use instead when the native path is incomplete.

## Monitoring (Prometheus / Grafana / Datadog)

DPlaneOS exposes a Prometheus/OpenMetrics endpoint at `GET /metrics` (no authentication required). Scrape it with any standard collector.

### Metrics exposed

| Metric | Description |
|--------|-------------|
| `dplaneos_zfs_pool_size_bytes{pool}` | Total pool capacity |
| `dplaneos_zfs_pool_alloc_bytes{pool}` | Allocated bytes |
| `dplaneos_zfs_pool_free_bytes{pool}` | Free bytes |
| `dplaneos_zfs_pool_healthy{pool}` | Health: 1=ONLINE 0.75=DEGRADED 0.5=FAULTED 0=UNAVAIL |
| `dplaneos_zfs_pool_scan_percent{pool}` | Scrub/resilver progress (-1 if no active scan) |
| `dplaneos_zfs_dataset_used_bytes{dataset}` | Dataset used bytes (compressed) |
| `dplaneos_zfs_dataset_available_bytes{dataset}` | Dataset available bytes |
| `dplaneos_zfs_dataset_quota_bytes{dataset}` | Dataset quota (0 = no quota) |
| `dplaneos_zfs_arc_size_bytes` | ZFS ARC current size |
| `dplaneos_zfs_arc_max_bytes` | ZFS ARC maximum size |
| `dplaneos_ha_enabled` | 1 if HA is configured |
| `dplaneos_ha_peer_count` | Number of cluster peers |
| `dplaneos_ha_quorum` | 1 if cluster has quorum |
| `dplaneos_ha_last_failover_timestamp_seconds` | Unix timestamp of last failover |
| `dplaneos_ha_peer_health{peer,role}` | Per-peer health: 1=healthy 0.5=degraded 0=unreachable |
| `dplaneos_replication_enabled{schedule,dataset}` | 1 if schedule is enabled |
| `dplaneos_replication_last_run_timestamp_seconds{schedule,dataset}` | Last run timestamp |
| `dplaneos_replication_last_success{schedule,dataset}` | 1 if last run succeeded |
| `dplaneos_memory_total_bytes` | Total physical memory |
| `dplaneos_memory_used_bytes` | Used memory |
| `dplaneos_cpu_iowait_ratio` | CPU IO wait fraction (0-1) |
| `dplaneos_load_1` / `_5` / `_15` | Load averages |
| `dplaneos_build_info{version}` | Build metadata |

### Prometheus scrape config example

```yaml
scrape_configs:
  - job_name: dplaneos
    static_configs:
      - targets: ['nas.example.com:80']
    metrics_path: /metrics
    scheme: http  # or https if TLS is configured
```

### SNMP

There is no native SNMP MIB. For environments that require SNMP, use `snmp_exporter` with a custom module that scrapes `/metrics` and re-exposes via SNMP, or use Prometheus federation.

---

## Active Directory / Windows Domain

**Current state:** LDAP authentication works. Winbind, NSS integration, and CTDB do not exist.

### What works

- Users and groups can be sourced from an LDAP/Active Directory server
- Authentication uses LDAP bind (password verification against AD)
- Group memberships are cached locally with configurable TTL
- SCRAM-SHA-512 is used for all local auth; LDAP users authenticate via LDAP bind

### What does not work

- **NSS integration**: Linux system tools (`ls -l`, `stat`, `id`) will show numeric UIDs for AD users rather than display names. There is no `/etc/nss_ldap.conf` or `libnss_ldap.so` wired in.
- **SMB with AD-integrated ACLs**: Samba can be configured with LDAP backend but there is no winbind daemon, so Samba cannot resolve AD SIDs to local accounts for ACL enforcement.
- **Kerberos**: No Kerberos keytab, no `kinit`, no ticket-based authentication.
- **CTDB**: No distributed Samba state for HA SMB. SMB clients reconnect after failover; byte-range lock state is lost.

### Recommended approach for AD environments

Use NFSv4 with sec=krb5 for Linux clients (requires a separate Kerberos realm setup outside DPlaneOS). For Windows clients accessing file shares, use local users with SMB authentication; map AD group membership to DPlaneOS roles via the LDAP group mapping feature.

If AD-integrated SMB with proper ACLs is a hard requirement for your deployment, consider continuing on TrueNAS SCALE (which ships winbind and Samba with AD integration) until Winbind support is added to DPlaneOS.

---

## SMB / Samba

SMB shares work for local and LDAP-authenticated users. Limitations:

- **No CTDB**: In HA configurations, SMB clients disconnect and reconnect on failover. Byte-range locks are not preserved.
- **No Winbind**: AD SID-to-UID mapping is not available.
- **Reconnect time**: Typically 5-30 seconds depending on client retry settings.

Applications that depend on lock continuity (Outlook PST files, Access databases, certain CAD tools) will experience errors on failover. For these workloads, pre-schedule failovers during maintenance windows and notify connected users.

---

## NVMe-oF

NVMe-oF over TCP is fully supported. ANA (Asymmetric Namespace Access) is supported for multi-path configurations.

### Setup in state.yaml

```yaml
fabrics:
  nvme:
    - subsystem_nqn: nqn.2024-01.io.dplane:my-volume
      zvol: tank/nvme/vol0
      transport: tcp
      listen_addr: 0.0.0.0
      listen_port: 4420
      allow_any_host: false
      host_nqns:
        - nqn.2014-08.org.nvmexpress:uuid:YOUR-HOST-UUID
      ana_enabled: true
      ana_groups:
        - group_id: 1
          namespace_id: 1
          state: optimized
```

### Connecting from a Linux initiator

```bash
modprobe nvme-tcp
nvme discover -t tcp -a <nas-ip> -s 4420
nvme connect -t tcp -a <nas-ip> -s 4420 -n nqn.2024-01.io.dplane:my-volume
```

---

## iSCSI

iSCSI with CHAP and ACL-based access control is supported. ALUA (Asymmetric Logical Unit Access) is supported for HA path management.

### HA failover with ALUA

The Keepalived notify_backup script sets iSCSI targets to Standby before exporting pools. Initiators using multi-path (DM-Multipath or MPIO) will see a clean path state transition rather than an abrupt loss.

For single-path iSCSI, clients will disconnect and reconnect on failover. This is equivalent to SMB failover behavior.

---

## GitOps / Automation

DPlaneOS manages configuration through `state.yaml`. This can be committed to a Git repository and pushed to trigger reconciliation.

### Integrate with CI/CD

The daemon accepts `-apply` to apply a state file and exit non-zero on failure. This allows GitOps pipelines to drive configuration changes:

```bash
# In CI:
scp state.yaml nas:/var/lib/dplaneos/gitops/state.yaml
ssh nas 'dplaned -apply -gitops-state /var/lib/dplaneos/gitops/state.yaml'
```

### Direct API

All operations available in the UI are available via the REST API. The API token system supports resource-level allowlists so CI/CD tokens can be scoped to only the endpoints they need.

---

## HA Timing Tuning

The failover threshold and hysteresis window are now runtime-configurable (changed in v14.0.0):

```
GET  /api/ha/timing  - read current values
POST /api/ha/timing  - update values (requires AAL2)
```

Default values: 45s failover threshold, 15s heartbeat interval, 60m hysteresis. For high-latency WAN links, increase failover_after_seconds proportionally. The constraint `failover_after_seconds >= heartbeat_interval_seconds * 3` is enforced.

Changes to failover_after_seconds and heartbeat_interval_seconds take effect after daemon restart. hysteresis_window_minutes takes effect immediately.
