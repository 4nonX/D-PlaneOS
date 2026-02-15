# D-PlaneOS v2.1.0 — Security Policy

## Supported Versions

| Version | Supported |
|---------|-----------|
| v2.1.0  | ✅ Current release |
| v1.14.0-OMEGA | ⚠️ Legacy — critical fixes only |
| < v1.14.0 | ❌ End of life |

## Reporting a Vulnerability

**Do not open a public GitHub issue for security vulnerabilities.**

Email: Create a GitHub issue with the `security` label and mark it confidential, or contact via the repository's security advisory feature.

Response times:
- **Critical** (RCE, auth bypass, data exposure): 24 hours
- **High** (privilege escalation, injection): 72 hours
- **Medium/Low** (information disclosure, DoS): 1 week

## Architecture Overview

D-PlaneOS v2.1.0 runs as a single Go binary (`dplaned`) under systemd. There is no PHP, no Apache, no Node.js in the runtime stack.

```
Client (Browser)
    │
    ├── HTTPS (reverse proxy: nginx/caddy)
    │
    └── HTTP :9000 (localhost only)
            │
        ┌───┴───┐
        │ dplaned │  ← single Go binary
        └───┬───┘
            │
    ┌───────┼───────────┐
    │       │           │
  SQLite  exec.Command  WebSocket
  (WAL)   (validated)   (/ws/monitor)
```

## Security Model

### 1. Authentication

Every API request (except `/health`) requires two headers:
- `X-Session-ID` — session token (20–100 chars, alphanumeric)
- `X-User` — username

The session middleware validates:
1. Both headers present → else 401
2. Token format valid → else 401
3. Token exists in `sessions` table and not expired → else 401
4. `X-User` matches session owner → else 401

**Fail-closed**: any database error during validation rejects the request. There is no fallback to permissive mode.

### 2. Authorization (RBAC)

Four built-in roles: `admin`, `operator`, `user`, `viewer`.

Permissions are resource-action pairs (e.g., `storage:write`, `docker:delete`). The RBAC middleware checks permissions before handler execution. Permission cache uses a TTL to avoid per-request DB queries.

Role assignments support expiry dates. Expired assignments are ignored at query time.

### 3. Input Validation (Command Injection Prevention)

All arguments passed to `exec.Command` are validated by dedicated functions:

| Validator | Blocks |
|-----------|--------|
| `ValidatePoolName` | shell metacharacters, spaces, path separators |
| `ValidateDatasetName` | same + must match `pool/dataset` format |
| `ValidateDevicePath` | must start with `/dev/`, no `..`, no metacharacters |
| `ValidateMountPoint` | must be under `/mnt/` or `/media/`, no `..` |
| `ValidateIP` | must pass `net.ParseIP` |
| `IsValidSessionToken` | alphanumeric, 20–100 chars |

These validators use **allowlists** (valid characters), not blocklists. Any input that doesn't match is rejected with HTTP 400 before any command is constructed.

Go's `exec.Command(name, arg1, arg2, ...)` passes arguments as an array, not a shell string. There is no shell interpolation.

### 4. Database Security

- **WAL mode** with `FULL` synchronous — crash-safe writes
- **64 MB page cache** — reduces disk I/O
- **30-second busy timeout** — prevents "database locked" during WAL checkpoints
- **Periodic WAL checkpoint** — every 5 minutes to prevent WAL bloat
- **Daily VACUUM INTO** — creates clean backup copy, configurable to off-pool path
- **Prepared statements** throughout — no string-concatenated SQL
- **Startup schema init** — `CREATE TABLE IF NOT EXISTS` on every boot, idempotent

### 5. Network Security

- `dplaned` listens on `127.0.0.1:9000` by default (localhost only)
- External access requires a reverse proxy (nginx, caddy, Pangolin)
- No `Access-Control-Allow-Origin: *` headers
- Rate limiting middleware on all endpoints
- WebSocket endpoint validates session before upgrade

### 6. Process Security

- Runs as root (required for ZFS, disk, network operations)
- systemd `MemoryMax=512M` prevents OOM from runaway operations
- `Restart=always` with 5-second delay
- Graceful shutdown on SIGTERM — drains active connections
- All command executions logged to audit trail

### 7. Audit Logging

Every state-changing operation is logged:
- Timestamp, user, action, resource, details, IP address, success/failure
- Buffered writer (100 events, 5-second flush) prevents I/O bottlenecks
- Stored in `audit_logs` SQLite table

## Known Limitations

- **Root execution**: `dplaned` runs as root because ZFS, disk management, and network configuration require it. Privilege separation via capabilities is a future goal.
- **No TLS termination**: `dplaned` serves HTTP. TLS must be handled by a reverse proxy.
- **Session tokens in headers**: tokens are sent via `X-Session-ID` header, not cookies. The frontend stores them in memory (not localStorage).

## Hardening Checklist

- [ ] Place behind TLS-terminating reverse proxy
- [ ] Restrict firewall to only necessary ports
- [ ] Use `-backup-path` flag pointing to a different disk
- [ ] Review audit logs regularly (`/api/system/logs`)
- [ ] Set up Telegram notifications for critical events
- [ ] Keep host OS updated (ZFS, kernel, systemd)
