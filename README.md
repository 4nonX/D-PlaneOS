# D-PlaneOS v2.0.0 — Enterprise NAS Operating System

Open-source NAS OS with Material Design 3 UI, ZFS storage, Docker containers, RBAC, and LDAP/Active Directory integration.

## Quick Start

```bash
tar xzf dplaneos-v2.0.0-production-vendored.tar.gz
cd dplaneos
sudo make install   # Pre-built binary, no compiler needed
sudo systemctl start dplaned
```

Web UI: `https://your-server` (nginx reverse proxy on port 443 → daemon on 9000)

**Default login:** `admin` / `admin` (change immediately after first login)

> **Rebuilding from source?** You need Go 1.22+ and gcc: `make build` compiles fresh.

### Off-Pool Database Backup (recommended for 52TB+)

Edit `/etc/systemd/system/dplaned.service` and add `-backup-path`:
```
ExecStart=/opt/dplaneos/daemon/dplaned -db /var/lib/dplaneos/dplaneos.db -backup-path /mnt/usb/dplaneos.db.backup
```
Creates a VACUUM INTO backup on startup + every 24 hours.

## Features

- **Storage:** ZFS pools, snapshots, replication, encryption, quotas, file explorer
- **Compute:** Docker container management, app modules
- **Network:** Interface config, routing, DNS
- **Identity:** User management, groups, **LDAP/Active Directory** (v2.0.0)
- **Security:** RBAC (4 roles), audit logging, API tokens, firewall
- **System:** Settings, logs, UPS management, hardware detection
- **UI:** Material Design 3, dark theme, responsive, keyboard shortcuts

## LDAP / Active Directory (New in v2.0.0)

Navigate to **Identity → Directory Service** to configure. Supports:

- Active Directory, OpenLDAP, FreeIPA (one-click presets)
- Group → Role mapping (auto-assign permissions)
- Just-In-Time user provisioning
- Background sync with audit trail
- TLS 1.2+ enforced

## Architecture

- **Frontend:** HTML5 + Material Design 3, flyout navigation, no framework dependencies
- **Backend:** Go daemon (`dplaned`, 8MB) on port 9000, 85 API routes
- **Database:** SQLite with WAL mode, `synchronous=FULL`, daily VACUUM INTO backup
- **Web Server:** nginx reverse proxy (TLS termination)
- **Storage:** ZFS (native kernel module) + ZED hook for real-time disk failure alerts
- **Security:** Input validation on all exec.Command (regex whitelist), RBAC (4 roles, 34 permissions), injection-hardened, OOM-protected (512MB limit)

## Documentation

- `CHANGELOG-v2.0.0.md` — What's new
- `RELEASE-NOTES-v2.0.0.md` — GitHub release notes
- `ADMIN-GUIDE.md` — Full administration guide (v2.0.0)
- `ERROR-REFERENCE.md` — API error codes and diagnostics
- `TROUBLESHOOTING.md` — Build issues, ZED setup, common fixes
- `LDAP-REFERENCE.md` — LDAP technical reference
- `INSTALLATION-GUIDE.md` — Detailed installation steps
- `scripts/build-release.sh` — Automated release builder with smoke tests

## License

Open source. See LICENSE file.
