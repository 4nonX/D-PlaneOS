# D-PlaneOS v2.0.0 — Production Release

**Complete NAS operating system. Zero PHP dependencies. Pure Go backend.**

## What's New in This Release

### Complete Authentication System
- **Login page** (`/pages/login.html`) — dark theme, session-based auth
- **6 auth endpoints**: login, logout, check, session, change-password, CSRF
- **bcrypt password hashing** with timing-attack prevention
- **24-hour sessions** with automatic cleanup (every 15 min)
- **Audit logging** for all auth events

### 105 Go Daemon API Routes
Every frontend page is backed by a working Go handler. No dead endpoints.

| Category | Routes | Handler Files |
|----------|--------|---------------|
| Auth | 6 | auth.go |
| ZFS (pools, datasets, encryption) | 8 | zfs.go, zfs_encryption.go |
| Docker (containers, actions, logs) | 3 | docker.go, docker_logs.go |
| Files (CRUD, upload, permissions) | 8 | files.go, files_extended.go |
| Shares (SMB CRUD, NFS, reload) | 6 | shares.go, shares_crud.go |
| Users & Groups (CRUD) | 3 | users_groups.go |
| RBAC (roles, permissions) | 11 | rbac.go |
| LDAP/AD | 10 | ldap.go |
| System (status, settings, preflight, profile) | 9 | system.go, system_status.go, system_settings.go |
| Monitoring & Metrics | 4 | monitoring.go, system_extended.go |
| Snapshots & Schedules | 2 | system_extended.go |
| Backup & Replication | 3 | backup.go, replication.go |
| Network & Firewall | 3 | system_extended.go |
| Certificates | 3 | system_extended.go |
| Power Management | 3 | system_extended.go |
| ACL | 2 | system_extended.go |
| Trash | 4 | system_extended.go |
| Removable Media | 4 | removable_media.go |
| Settings (Telegram) | 3 | settings.go |
| Disk Discovery | 2 | disk_discovery.go |
| WebSocket | 1 | websocket.go |
| Health | 1 | main.go |

### Zero PHP Dependencies
- **0** `.php` references in frontend (was 112)
- **29** PHP endpoint patterns migrated to Go routes
- nginx config blocks PHP execution (`deny all`)
- No php-fpm required. `apt remove php*` is safe.

## Architecture

```
Browser → nginx (:80)
  ├── Static: /pages/*.html, /assets/* (served directly)
  ├── /api/*  → proxy → Go daemon (:9000)
  └── /ws/*   → proxy → Go daemon (:9000)

Go daemon (dplaned)
  ├── Public:  /api/auth/*, /api/csrf, /health
  ├── Auth:    sessionMiddleware (fail-closed)
  └── 105 API routes across 24 handler files
```

## Fresh Install Flow

1. Run `install.sh` → installs ZFS, Docker, Samba, nginx, builds daemon
2. User visits `http://server-ip`
3. `first-run-detection.js` → `GET /api/system/status` → `{first_run: true}`
4. Redirect to setup wizard → disk discovery → pool creation → user creation
5. `POST /api/system/setup-complete` → mark done
6. Redirect to login → dashboard

## Changed Files

**6 new files:**
- `app/pages/login.html` — Login page
- `daemon/internal/handlers/auth.go` — Authentication (378 lines)
- `daemon/internal/handlers/users_groups.go` — User & Group CRUD (318 lines)
- `daemon/internal/handlers/system_status.go` — System status/profile/preflight (270 lines)
- `daemon/internal/handlers/shares_crud.go` — SMB share CRUD (345 lines)
- `daemon/internal/handlers/docker_logs.go` — Container logs (56 lines)

**36 modified files:**
- `daemon/cmd/dplaned/main.go` — 20 new route registrations, auth middleware update
- `install.sh` — Version strings v2.0.0
- 8 JavaScript files — All auth/API references migrated
- 25 HTML pages — All PHP endpoint references migrated

## Security

- **Fail-closed auth**: Any DB error = request rejected
- **bcrypt**: Default cost, constant-time comparison
- **Session tokens**: 32 bytes (256-bit), cryptographically random
- **Input validation**: Username allowlisting, container name regex, path validation
- **RBAC**: Role-based access control with per-user permissions
- **nginx hardens**: PHP blocked, X-Frame-Options, X-Content-Type-Options
- **Audit trail**: All auth events, command execution, security events logged
