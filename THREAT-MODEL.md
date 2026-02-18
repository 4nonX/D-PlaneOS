# D-PlaneOS v2.2.0 — Threat Model

## System Context

D-PlaneOS is a NAS operating system managing storage (ZFS), containers (Docker), network, and identity on a single server. It runs as one Go binary (`dplaned`) and serves a web UI on localhost. External access is via reverse proxy.

**Trust boundary**: the reverse proxy. Everything behind it (dplaned, SQLite, ZFS commands) is trusted. Everything in front (browser, network) is untrusted.

```
┌─────────────────────────────────────────────────┐
│                    UNTRUSTED                     │
│  Browser ──── Internet ──── Reverse Proxy        │
└──────────────────────┬──────────────────────────┘
                       │ TLS terminated
┌──────────────────────┴──────────────────────────┐
│                     TRUSTED                      │
│  dplaned (Go) ── SQLite ── exec.Command ── ZFS  │
│                              │                   │
│                     /dev/sd* │ /mnt/*            │
└─────────────────────────────────────────────────┘
```

## Assets

| Asset | Value | Location |
|-------|-------|----------|
| User data (files, datasets) | CRITICAL | ZFS pools on `/mnt/*` |
| ZFS pool metadata | CRITICAL | Pool vdevs |
| Encryption keys (loaded) | CRITICAL | Kernel memory (ZFS) |
| SQLite database | HIGH | `/var/lib/dplaneos/dplaneos.db` |
| Session tokens | HIGH | SQLite `sessions` table |
| LDAP bind password | HIGH | SQLite `ldap_config` table |
| Telegram bot token | MEDIUM | SQLite `telegram_config` table |
| Audit logs | MEDIUM | SQLite `audit_logs` table |
| Configuration | LOW | SQLite tables + CLI flags |

## Threat Actors

| Actor | Capability | Goal |
|-------|-----------|------|
| Remote unauthenticated | HTTP requests to reverse proxy | Data theft, service disruption |
| Remote authenticated (low-priv) | Valid session, `viewer` or `user` role | Privilege escalation, unauthorized data access |
| Local network attacker | Can reach port 9000 if misconfigured | Full API access without TLS |
| Physical attacker | Access to hardware | Disk theft, boot manipulation |
| Malicious container | Docker container with host mounts | Escape to host filesystem |

## Threats & Mitigations

### T1: Command Injection via API Parameters

**Vector**: Attacker sends `{"pool":"tank; rm -rf /"}` to ZFS endpoint.

**Mitigation**:
- All parameters validated by allowlist validators (`ValidatePoolName`, `ValidateDevicePath`, etc.)
- Go `exec.Command` passes arguments as array — no shell interpolation
- Tested: `; rm -rf /`, `$(reboot)`, backticks, pipe operators — all return HTTP 400

**Residual risk**: LOW. Would require a bug in Go's `exec.Command` itself.

### T2: Authentication Bypass

**Vector**: Attacker crafts requests without valid session.

**Mitigation**:
- Session middleware on every request (except `/health`)
- Fail-closed: DB errors → reject
- Token format validation + DB lookup + username match
- No anonymous API access

**Residual risk**: LOW.

### T3: Privilege Escalation (RBAC Bypass)

**Vector**: `viewer` role user attempts `storage:write` operations.

**Mitigation**:
- RBAC middleware checks resource:action permission before handler runs
- System roles (`admin`, `operator`, `user`, `viewer`) are immutable (`is_system = 1`)
- Role assignments support expiry

**Residual risk**: LOW. Some endpoints don't yet have RBAC middleware applied (they use session auth only). Future work.

### T4: SQL Injection

**Vector**: Malicious input in API parameters reaches SQL queries.

**Mitigation**:
- All SQL uses parameterized queries (`?` placeholders)
- No string concatenation in SQL construction
- Input validation rejects metacharacters before they reach the DB layer

**Residual risk**: NEGLIGIBLE.

### T5: Cross-Site Scripting (XSS)

**Vector**: Stored XSS via file names, share names, or configuration values.

**Mitigation**:
- Frontend uses `textContent` for dynamic data (not `innerHTML`)
- API responses are JSON (not rendered HTML)
- Content-Security-Policy headers recommended at reverse proxy level

**Residual risk**: LOW. Some pages use `innerHTML` for HTML templates — these should be audited.

### T6: Denial of Service

**Vector**: Flood of API requests exhausts resources.

**Mitigation**:
- Rate limiting middleware
- systemd `MemoryMax=512M` prevents OOM
- SQLite `busy_timeout=30000` prevents lock starvation
- Buffered audit logging prevents I/O stalls
- Graceful shutdown drains connections

**Residual risk**: MEDIUM. A determined attacker with valid credentials could trigger expensive operations (ZFS scrub, Docker pull). Rate limiting helps but doesn't fully prevent this.

### T7: Data at Rest (Stolen Hardware)

**Vector**: Attacker steals physical server or disks.

**Mitigation**:
- ZFS native encryption supported (AES-256-GCM)
- Encryption keys unloaded on shutdown
- UI supports lock/unlock/change-key operations

**Residual risk**: LOW if encryption is enabled. HIGH if not — user responsibility.

### T8: Man-in-the-Middle

**Vector**: Attacker intercepts traffic between browser and server.

**Mitigation**:
- `dplaned` listens on localhost only by default
- TLS termination at reverse proxy (nginx/caddy/Pangolin)
- Session tokens in headers (not URL parameters)

**Residual risk**: LOW if properly configured. HIGH if exposed directly to network without TLS.

### T9: LDAP Credential Exposure

**Vector**: LDAP bind password stored in SQLite, accessible to root.

**Mitigation**:
- Password stored in DB (not plaintext config file)
- Only accessible via authenticated API
- Bind password not returned in GET responses (redacted)

**Residual risk**: MEDIUM. Root access to the server exposes the DB file. This is inherent to single-server NAS architecture.

### T10: Container Escape

**Vector**: Malicious Docker container with host filesystem mount.

**Mitigation**:
- D-PlaneOS manages containers but doesn't control their security policies
- Users must configure bind mounts and network policies appropriately

**Residual risk**: HIGH. This is a Docker-level concern, not a D-PlaneOS concern. User education required.

## Attack Surface Summary

| Surface | Exposure | Mitigations |
|---------|----------|-------------|
| HTTP API (85 routes) | All authenticated except `/health` | Session + RBAC + input validation |
| WebSocket (`/ws/monitor`) | Authenticated | Session validation before upgrade |
| `exec.Command` (ZFS, Docker, system) | Internal only | Allowlist validators, no shell |
| SQLite database | Filesystem access | WAL mode, backup, permissions |
| Systemd service | Root process | MemoryMax, RestartSec, graceful shutdown |

## Future Security Work

- [ ] Per-endpoint RBAC enforcement (currently some endpoints use session auth only)
- [ ] Linux capabilities instead of full root
- [ ] API request signing for critical operations
- [ ] CSP headers in frontend responses
- [ ] Encrypted SQLite database (SQLCipher)
- [ ] 2FA support for web login
