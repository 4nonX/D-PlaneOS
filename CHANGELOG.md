# D-PlaneOS Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/), and this project adheres to [Semantic Versioning](https://semver.org/).

---

## v2.0.0 (2026-02-12) — **"Ground-Up Rewrite"**

### ⚡ The Complete Platform Rewrite

D-PlaneOS v2.0.0 is a full architectural rewrite. The PHP/Apache stack that powered v1.x has been replaced by a single Go binary (`dplaned`) serving both the API and the frontend. No PHP, no Apache, no Node — one 8 MB binary does everything.

This is not an upgrade from v1.14.0-OMEGA. It is a clean-room reimplementation retaining full feature parity and adding 20+ new capabilities.

### Architecture Change

| Component | v1.x (PHP) | v2.0.0 (Go) |
|-----------|-----------|-------------|
| Backend | PHP-FPM + Apache | Single Go binary (`dplaned`, 8 MB) |
| Database | SQLite via PHP PDO | SQLite via `go-sqlite3` with WAL, 64 MB cache, FULL sync |
| Auth | PHP sessions + cookies | Session tokens + RBAC middleware |
| Frontend | PHP-rendered SPA | Static HTML + vanilla JS, hybrid SPA router |
| Config | `/etc/dplaneos/*.conf` | SQLite + CLI flags |
| Install | `install.sh` (50+ steps) | `make install` (one command) |
| Process model | Apache workers + FPM pool | Single binary, goroutines |
| Memory limit | 512 MB (FPM pool) | 512 MB (systemd MemoryMax) |

### Added — Backend

- **Go daemon** (`dplaned`) — 38 Go source files, 85 API routes, single binary
- **Schema auto-initialization** — 11 SQLite tables + 7 indexes created on first start
- **Default data seeding** — 4 RBAC roles (admin/operator/user/viewer), 27 permissions, admin user, LDAP config, Telegram config — all bootstrapped automatically
- **LDAP/Active Directory integration** — full LDAP handler with bind, search, sync, group mapping, JIT provisioning, test connection, sync history
- **RBAC engine** — roles, permissions, role-permission mapping, user-role assignment with expiry, permission cache with TTL
- **Buffered audit logging** — batched inserts (100-event buffer, 5-second flush) to prevent I/O stalls on 52 TB pools
- **Background system monitoring** — goroutine-based metrics collection (CPU, memory, disk I/O, network, ZFS pool health)
- **WebSocket live updates** — `/ws/monitor` endpoint for real-time system metrics
- **ZFS encryption management** — create encrypted datasets, lock/unlock, change keys, list encryption status
- **Removable media handler** — detect, mount, unmount, eject USB/removable devices with full input validation
- **Replication** — ZFS send/receive, incremental send, remote replication
- **Snapshot scheduler** — create/list/delete schedules, run-now capability
- **Firewall management** — UFW status, add/remove rules
- **SSL/TLS certificate management** — list, generate self-signed, activate
- **Trash/Recycle bin** — move to trash, restore, empty, list
- **Power management** — disk spindown configuration, immediate spindown, disk power status
- **Cloud sync** — rclone-based cloud backup and sync (UI + backend)
- **IPMI hardware monitoring** — real-time sensor data via `ipmitool`
- **Telegram notifications** — configure bot token/chat ID, test delivery, toggle on/off
- **Docker management** — list containers, start/stop/restart/remove, image pull
- **File management** — list, upload, delete, rename, copy, mkdir, chmod, chown, properties
- **ACL management** — get/set POSIX ACLs on files and directories
- **Quota management** — ZFS user/group quotas
- **NFS export management** — list exports, reload (graceful degradation if NFS not installed)
- **SMB management** — reload config, test config
- **Backup** — rsync-based backup
- **System logs** — journalctl integration with filtering
- **UPS monitoring** — NUT integration for UPS status
- **Reporting/Metrics** — current metrics + historical data
- **Network management** — interface listing and configuration
- **Graceful shutdown** — SIGTERM/SIGINT handler with connection draining
- **OOM protection** — systemd `MemoryMax=512M` enforced
- **Off-pool backup** — `-backup-path` flag for daily VACUUM INTO to separate disk

### Added — Security

- **Input validation on all `exec.Command` calls** — `ValidatePoolName`, `ValidateDatasetName`, `ValidateDevicePath`, `ValidateMountPoint`, `ValidateIP` — all shell metacharacters blocked
- **Command whitelist** — only explicitly allowed binaries can be executed
- **Session middleware** — every API request validated (format check + DB lookup + user match)
- **Rate limiting middleware** — request throttling on all endpoints
- **Injection protection** — tested with `; rm -rf /`, `$(reboot)`, path traversal, all return HTTP 400
- **Fail-closed session validation** — any DB error rejects the request (no fallback to permissive mode)
- **ZED hook integration** — ZFS Event Daemon triggers alerts on pool errors
- **Audit logging** — all state-changing operations logged with user, action, resource, IP, timestamp

### Added — Frontend

- **41 HTML pages** — 36 with full navigation, 5 standalone (setup-wizard, reset-wizard, dashboard-redirect, docker sub-UIs)
- **Hierarchical navigation** — 6 top-level sections (Storage, Compute, Network, Identity, Security, System) with 35 sub-navigation links
- **Hover-intent flyout** — desktop sub-nav opens on hover with 120 ms intent delay + 280 ms grace period, touch/mobile unaffected
- **Hybrid SPA router** — sub-nav clicks swap content via fetch + fade (200 ms) without full page reload; cross-section clicks do full navigation
- **Material Symbols** — locally hosted icon font, no external CDN dependency
- **Design system** — CSS custom properties for colors, spacing, typography; Material Design 3 components
- **UI polish layer** — harmonized timing (140/260/200 ms tiers), unified easing (`cubic-bezier(0.4, 0, 0.2, 1)`), 8 px spacing grid, state clarity (active/hover/focus differentiation), `prefers-reduced-motion` support
- **Anti-flash** — `html{background:#0a0a0a}` inline style prevents white flash on load
- **Keyboard shortcuts** — `g+d` Dashboard, `g+s` Storage, `g+c` Compute, etc.; `?` shows help
- **Connection monitor** — detects backend connectivity loss, shows reconnection status
- **Form validation** — client-side validation library
- **Toast notifications** — non-blocking success/error/info/warning messages

### Added — DevOps

- **Makefile** — `make build`, `make install`, `make clean`, `make help`
- **systemd service** — `dplaned.service` with auto-restart, watchdog, resource limits
- **CI/CD script** — `scripts/build-release.sh` with smoke tests
- **Upgrade script** — `scripts/upgrade-with-rollback.sh` with ZFS snapshot rollback
- **Error reference** — `ERROR-REFERENCE.md` documenting all HTTP codes, validation errors, diagnostics

### Changed (from v1.14.0-OMEGA)

- Backend language: **PHP → Go**
- Process model: **Apache + FPM pool → single binary**
- Auth system: **PHP sessions → token-based sessions with RBAC middleware**
- Database access: **PHP PDO → Go `database/sql` with connection pooling**
- Command execution: **PHP `exec()` with string validation → Go `exec.Command` with argument-level validation**
- Frontend rendering: **PHP-rendered HTML → static HTML with JS fetch**
- Navigation: **sidebar SPA → top-nav with section flyouts and hybrid router**
- Color scheme: **`#667eea` → `#8a9cff`** (lighter, higher contrast)
- Nav style: **pill sub-nav → underline top-nav with pill sub-nav**

### Removed

- PHP backend (all `.php` files)
- Apache configuration
- PHP-FPM configuration
- Node.js dependencies
- `install.sh` multi-step installer (replaced by `make install`)
- `auth.php` execCommand() security theater (replaced by Go input validators)
- Dead `navigation.js` RBAC nav system (replaced by static HTML nav + `nav-flyout.js`)

### Fixed

- **Schema initialization** — v1.x required manual migration scripts; v2.0.0 auto-creates all tables on first start
- **NFS list 500 error** — `exportfs` not found now returns empty list instead of Internal Server Error
- **RBAC time.Time scan error** — SQLite TEXT columns now correctly mapped to Go string types
- **12 orphaned pages** — pages existed with working backends but were unreachable from navigation; all now linked
- **pageMap sync** — inline JS page-to-section mapping now covers all 36 navigable pages
- **Anti-flash on injected pages** — duplicate `<style>` tags cleaned up

### ⚠️ Breaking Changes

- **Not an in-place upgrade from v1.x.** Fresh install required. Data (ZFS pools, shares, Docker containers) is preserved on disk — only the management layer changes.
- **No PHP dependency.** Systems with only the Go binary can run D-PlaneOS.
- **API paths changed.** v1.x used `/api/storage/files.php?action=list`; v2.0.0 uses `/api/files/list`. Frontend handles this transparently.

---

## v1.14.0-OMEGA (2026-02-01) — **"OMEGA Edition"**

First fully production-ready PHP release. Fixes 7 critical infrastructure bugs that caused silent failures on fresh installs.

### Fixed (7 Critical Infrastructure Bugs)

1. **www-data sudo permissions missing** (CRITICAL) — every privileged command failed silently; added comprehensive sudoers config
2. **SQLite write permissions** (CRITICAL) — first login always failed; installer now sets correct ownership on runtime directories
3. **Login loop on cold start** (HIGH) — dashboard rendered before auth check; added `body{display:none}` until session confirmed
4. **API timeout handling** (HIGH) — `iwlist scan` with no timeout hung entire FPM pool; all hardware detection now uses `timeout 3`
5. **Silent session expiry** (MEDIUM) — heartbeat polling detects expiry, auto-redirect to login
6. **No loading feedback** (LOW) — global LoadingOverlay with spinner and double-click prevention
7. **Style flash on load** (LOW) — styles injected before DOM render

### Added

- Server-side auth checks on all API endpoints
- 1-hour session timeout with inactivity enforcement
- 401 JSON responses for unauthorized access
- Removed `Access-Control-Allow-Origin: *` (same-origin enforced)
- Post-install integrity checker (`scripts/audit-dplaneos.sh`)
- Reproducible build script (`scripts/CREATE-OMEGA-PACKAGE.sh`)

---

## v1.14.0 (2026-01-31) — **"UI Revolution"**

Complete frontend rebuild: 10 fully functional pages, responsive, customizable.

### Added

- 10 management pages wired to 16 backend APIs (Dashboard, Storage, Docker, Shares, Network, Users, Settings, Backup, Files, Customize)
- Customization system: 10+ color parameters, sidebar width slider, custom CSS upload, 3 preset themes, theme export/import
- Frontend JS modules: `main.js`, `sidebar.js`, `pool-wizard.js`, `hardware-monitor.js`, `ux-feedback.js`
- CSS theme system with CSS variables

### Changed

- Frontend moved from PHP-rendered pages to static HTML + vanilla JS SPA
- All pages share single `main.js` app shell

---

## v1.13.1 (2026-01-31) — **"Hardening Pass"**

### Added

- Docker brutal cleanup on restore
- Log rotation with `copytruncate` strategy
- ZFS auto-expand trigger after disk replacement

### Fixed

- Edge cases in backup/restore workflow
- Log file handling during active operations
- Pool expansion detection after hardware changes

---

## v1.13.0 (2026-01-31) — **"Future-Proof Installer"**

### Added

- Dynamic PHP version detection (no more hardcoded package lists)
- Automatic PHP socket location detection
- Pre-flight validation: disk space (20 GB), memory (4 GB), connectivity, port conflicts, OS compatibility

### Changed

- Complete installer rewrite replacing all hardcoded dependency versions
- Improved ARM/Raspberry Pi support

### Fixed

- Installer hanging on missing dependencies
- Kernel headers missing for ZFS on ARM
- Docker repository configuration issues
- PHP version detection failures on newer Debian/Ubuntu

---

## v1.12.0 (2026-01-31) — **"The Big Fix"**

45 vulnerabilities from comprehensive penetration test.

### Fixed (10 Critical)

1. **C-01: Systemic XSS** — 282 unescaped interpolation points wrapped with `esc()` and `escJS()`
2. **C-02: SMB command injection** — raw `$_GET['name']` in `shell_exec`; applied `escapeshellarg()`, password piped via temp file
3. **C-03: Network command injection** — unescaped IPs in `exec()`; applied `filter_var(FILTER_VALIDATE_IP)` + `escapeshellarg()`
4. **C-04: Disk replacement dual injection** — command injection + SQL injection in same endpoint; fixed both
5. **C-05: ZFS admin bypass** — `create` action missing from admin whitelist; any user could create pools
6. **C-06: Backup path traversal** — `../` in filename could delete files outside backup dir; applied `basename()`
7. **C-07: SSE stream corruption** — HTTP router dumped JSON into SSE stream; added include guard
8. **C-08: NFS cp not in sudoers** — NFS export updates silently failed; added sudoers entry
9. **C-09: Auto-backup auth failure** — HTTP calls with no session cookie; implemented service-token system
10. **C-10: Notifications system broken** — schema mismatch, no router, no frontend path; fixed all three

### Fixed (7 High Severity)

- H-11 through H-17: dashboard metrics, pool wizard, share cards, repository list, ZFS scrub status, Docker quick actions — all repaired

### Added

- SSH key validation, Tailscale configurator
- Complete XSS mitigation framework (`utils.js`)
- Rate limiting for all state-changing operations
- Input validation across entire codebase

---

## v1.11.0 (2026-01-31) — **"Vibecoded Security Theater Fix"**

### Fixed (CRITICAL)

- **Command injection via flawed string check** — `execCommand()` checked if the *string* `"escapeshellarg"` appeared in the command, not whether arguments were actually escaped. 108 vulnerable call sites across the entire API surface. Complete rewrite with strict command whitelisting, proper metacharacter blocking, and comprehensive audit logging.

---

## v1.10.0 (2026-01-31) — **"Smart State Polling & One-Click Updates"**

### Added

- ETag-based smart polling (95% bandwidth reduction, 88% CPU reduction)
- ZFS snapshot-based update system with automatic rollback
- Update UI with real-time progress via SSE
- Pre-flight checks, smoke tests, automatic rollback on failure

### Changed

- License: MIT → PolyForm Noncommercial License 1.0.0

---

## v1.9.0 (2026-01-30) — **"RBAC & Security Fixes"**

### Added

- Role-Based Access Control: Admin, User, Readonly roles
- User management UI and API
- Safe SMB user management wrapper scripts
- Database migration system

### Fixed (7 Critical Security Issues)

1. Session fixation — sessions not regenerated after login
2. Wildcard sudoers rules — overly permissive `*` wildcards
3. Missing action parameter whitelist
4. Atomic file write race conditions
5. Docker Compose YAML injection
6. Weak random number generation (`rand()` → `random_int()`)
7. Log lines parameter unbounded (DoS risk) — capped at 10,000

---

## v1.8.0 (2026-01-28) — **"Power User Release"**

Every tab now functional — zero UI changes.

### Added

- **File browser** — list, upload, download, preview, search, create/delete/rename/move/copy folders, chmod, chown; restricted to `/mnt`
- **ZFS native encryption** — create encrypted datasets, load/unload keys, change passwords, bulk unlock, boot-time detection
- **System service control** — start/stop/restart/enable/disable systemd services, view status and logs
- **Real-time monitoring** — per-core CPU, memory, network, disk I/O, process list, system info from `/proc`

Result: all 14 tabs functional.

---

## v1.7.0 (2026-01-28) — **"The Paranoia Update"**

### Added

- **UPS/USV management** — NUT integration, real-time battery monitoring, auto shutdown at threshold, multi-UPS support
- **Automatic snapshot management** — hourly/daily/weekly/monthly schedules, configurable retention, automatic cleanup
- **System log viewer** — journalctl, service logs, audit log, ZFS events, Docker logs — all in browser

---

## v1.6.0 (2026-01-28) — **"Disk Health & Notifications"**

### Added

- SMART disk health monitoring with temperature alerts
- Disk replacement tracking and maintenance log
- System-wide notification center (slide-out panel, priority levels, categories, 7-day auto-cleanup)

---

## v1.5.1 (2026-01-28) — **"User Quotas"**

### Added

- Per-user ZFS quotas with real-time usage tracking
- Color-coded progress bars (green/yellow/red)
- Comprehensive responsive design audit

---

## v1.5.0 (2026-01-28) — **"UI/UX Stability"**

### Fixed

- Modal z-index overlap
- Button overflow on small screens
- Sidebar content overlap
- Chart layout shift

---

## v1.4.1 (2026-01-28) — **"UX & Reliability"**

### Added

- Visual replication progress with real-time updates
- Replication health alerts with webhook notifications

---

## v1.4.0 (2026-01-28) — **"Enterprise-Ready"**

### Added

- Least-privilege sudoers configuration with explicit allow-list
- `THREAT-MODEL.md` and `RECOVERY.md` documentation

---

## v1.3.1 (2026-01-28) — **"Security Hardening"**

### Added

- Enhanced `execCommand()` with input validation and injection detection
- Database integrity checks, read-only fallback on corruption
- API versioning (`/api/v1/`)

---

## v1.3.0 (2026-01-27) — **"Feature Release"**

### Added

- ZFS replication (send/receive to remote hosts)
- Alert system (Discord/Telegram webhooks)
- Historical analytics (30-day retention)
- Scrub scheduling with cron
- Bulk snapshot operations

---

## v1.2.0 — **"Initial Public Release"**

### Added

- ZFS management (pools, datasets, snapshots)
- Container management (Docker integration)
- System monitoring (CPU/memory/disk stats)
- Security (session auth, audit logging, CSRF protection)

---

## Upgrade Path

### v1.14.0-OMEGA → v2.0.0

**This is not an in-place upgrade.** v2.0.0 is a complete rewrite.

```bash
# Fresh install
tar xzf dplaneos-v2.0.0-production-vendored.tar.gz
cd dplaneos && sudo make install
sudo systemctl start dplaneos
```

Your ZFS pools, datasets, shares, and Docker containers remain on disk. Only the management layer changes. Re-import your configuration as needed.

### v1.x → v1.14.0-OMEGA

```bash
cd dplaneos-v1.14.0-OMEGA
sudo bash install.sh    # Select option 1 (Upgrade)
```

---

## Support

**Security issues:** Report via GitHub issues with `security` label.
Response time: Critical 24h, High 72h, Medium/Low 1 week.

**Bug reports:** GitHub issue with version, steps to reproduce, and logs.

**Feature requests:** GitHub issue with `enhancement` label.
