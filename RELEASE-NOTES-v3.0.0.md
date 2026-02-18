# D-PlaneOS v3.0.0 Release Notes — **"Security Hardening & Native APIs"**

**Release Date:** 2026-02-18
**Upgrade from:** v2.x (drop-in replacement, auto-migrates database)

---

## ⚡ Highlights

### Native Docker API (No Shell, No SDK)
All container lifecycle operations now use the Docker Engine REST API directly over `/var/run/docker.sock`. Zero new dependencies, no CGO, no shell involved. Pure Go stdlib `net/http` client.

### Native Linux Netlink (No `ip` Command)
Network operations (IP config, VLANs, bonds, routes) use raw `AF_NETLINK` syscalls via new `internal/netlinkx` package. Replaces ~15 `ip(8)` exec calls.

### Cookie-Based Session Auth
Replaced `sessionStorage` + manual `X-Session-ID` headers with `HttpOnly; SameSite=Strict` cookies. Sessions persist across tab close and browser restart. Real CSRF protection instead of security theater.

### Pre-Release Security Audit
Comprehensive code audit identified and fixed 2 showstoppers, 3 high-priority, and 4 medium-priority issues before public release.

---

## 🔒 Security Fixes

### Critical: Path Traversal in File Read Operations
`files_extended.go` — 5 handler functions (`ListFiles`, `GetFileProperties`, `RenameFile`, `CopyFile`, `UploadChunk`) used `filepath.Clean()` without `validateFilePath()` restriction to allowed base paths. Authenticated users could access any system directory.

**Fix:** `validateFilePath()` with `allowedBasePaths` applied to all 5 functions.

### Critical: CSRF Token Generated But Never Validated
Frontend sent `X-CSRF-Token` header, backend generated tokens, but no middleware validated them.

**Fix:** Entire auth mechanism replaced with HttpOnly cookie-based sessions. `SameSite=Strict` provides real CSRF protection.

### Critical: Git Repository URL RCE via `ext::` Transport
Git's `ext::` transport protocol executes arbitrary subprocesses. A malicious repo URL would be stored in the DB and executed on next pull/clone.

**Fix:** `validateRepoURL()` enforces allowlist: `https://`, `http://`, `git://`, `ssh://`, `git@host:path`.

### High: chown/chmod Flag Injection
Owner/group/permissions passed unvalidated to `exec.Command`.

**Fix:** Regex validation — owner/group: `^[a-zA-Z0-9_][a-zA-Z0-9_.\-]*$`, permissions: `^[0-7]{3,4}$`.

### High: Fresh Install Login Crash
Missing `must_change_password` and `role` columns in `users` table schema.

**Fix:** `ALTER TABLE` migrations in schema initialization. Admin user seeded with bcrypt-hashed default password and forced password change.

### Medium: Rate Limiter Memory Leak
`requestCounts` map grew unbounded.

**Fix:** Periodic cleanup goroutine every 10 minutes.

---

## Added

### New Packages
- **`internal/dockerclient`** — Pure stdlib Docker Engine REST client (12 methods replacing 12 exec calls)
- **`internal/netlinkx`** — Raw netlink syscall client for network configuration (replaces ~15 `ip` exec calls)

### New API Capabilities
- **Per-user ZFS quotas** — `zfs userquota` / `zfs groupquota` via API
- **Per-group ZFS quotas** — native kernel-level quota enforcement
- **IPMI/BMC sensor data** — hardware monitoring via `ipmitool` (graceful no-op if unavailable)

### Security Improvements
- Cookie-based session auth (`HttpOnly; SameSite=Strict`)
- Path validation on all file read/write operations
- Input validation on chown/chmod operations
- Git URL scheme allowlist
- Rate limiter memory management
- Audit buffer bypass for security-critical events

---

## Changed

- **Default credentials:** `admin` / `dplaneos` (was: `admin` / no password — login was impossible)
- **Session mechanism:** HttpOnly cookies (was: sessionStorage — died on tab close, no multi-tab)
- **Docker operations:** REST API over Unix socket (was: `exec.Command("docker", ...)`)
- **Network operations:** Netlink syscalls (was: `exec.Command("ip", ...)`)
- **Dashboard:** Removed non-functional Docker images counter
- **Version:** Unified to 3.0.0 across all files (was: mixed v2.1.0/v2.5/v5.9.0)
- **Documentation:** All docs updated to v3.0.0

---

## Architecture

| Component | v2.x | v3.0.0 |
|-----------|------|--------|
| Docker operations | `exec.Command("docker", ...)` | REST API via `/var/run/docker.sock` |
| Network operations | `exec.Command("ip", ...)` | `AF_NETLINK` syscalls |
| Session auth | sessionStorage + manual headers | HttpOnly cookies |
| CSRF protection | Generated token, never validated | `SameSite=Strict` cookie |
| File operations (read) | `filepath.Clean()` only | `validateFilePath()` with allowedBasePaths |
| Admin password | None (login impossible) | bcrypt-hashed default + forced change |

### What Stays exec (with Justification)
| Call | Reason |
|------|--------|
| ZFS commands (59 calls) | go-libzfs requires CGO + `libzfs-dev`; whitelist validation prevents injection |
| `docker compose up/down/ps` | Compose v2 CLI plugin has no stable REST API |
| `docker stats --no-stream` | Stats streaming needs multiplexed chunked encoding; no user input |

---

## Upgrade Path

### From v2.x
Drop-in replacement. Database auto-migrates. Users must re-login once (session mechanism changed).

```bash
tar xzf dplaneos-v3.0.0.tar.gz
cd dplaneos && sudo make install
sudo systemctl restart dplaned
```

### Fresh Install
```bash
tar xzf dplaneos-v3.0.0.tar.gz
cd dplaneos && sudo make install
sudo systemctl start dplaned
```

Web UI: `http://your-server`
Login: `admin` / `dplaneos` (forced password change on first login)

---

## No Breaking Changes

All HTTP API paths, request/response shapes, and frontend behaviour are identical to v2.x. The migration is purely internal to the daemon. The only user-visible change is the one-time re-login after upgrade.

---

## Known Limitations

- ZFS operations remain `exec.Command`-based (mitigated by command whitelist validation)
- Redirect compatibility pages (`network-interfaces.html`, `groups.html`, `system-monitoring.html`) kept for bookmark compatibility
- WebSocket auth relies on cookie being sent with upgrade request (same-origin only)

---

## Support

**Security issues:** Report via GitHub issues with `security` label.
Response time: Critical 24h, High 72h, Medium/Low 1 week.

**Bug reports:** GitHub issue with version, steps to reproduce, and logs.

**Feature requests:** GitHub issue with `enhancement` label.
