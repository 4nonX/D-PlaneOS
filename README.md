# D-PlaneOS v3.0.0 — NAS Operating System

Open-source NAS OS built on ZFS, with a web UI, Docker container management, role-based access control, and LDAP/AD integration.

## Quick Start

### NixOS ISO (easiest)
```bash
# Build the ISO or download from Releases
nix build .#nixosConfigurations.dplaneos-iso.config.system.build.isoImage
# Flash to USB, boot, open http://dplaneos.local
# Type 'dplaneos-install' to install permanently
```

### Debian/Ubuntu
```bash
tar xzf dplaneos-v3.0.0.tar.gz
cd dplaneos
sudo make install   # Pre-built binary, no compiler needed
sudo systemctl start dplaned
```

### NixOS
```bash
cd nixos
sudo bash setup-nixos.sh
sudo nixos-rebuild switch --flake .#dplaneos
```

See [nixos/README.md](nixos/README.md) for the full NixOS guide.

Web UI: `http://your-server` (nginx reverse proxy on port 80 → daemon on 9000)

**Default login:** `admin` / `dplaneos` (must be changed on first login)

> **Rebuilding from source?** You need Go 1.22+ and gcc: `make build` compiles fresh.

### Off-Pool Database Backup (recommended for large pools)

Edit `/etc/systemd/system/dplaned.service` and add `-backup-path`:
```
ExecStart=/opt/dplaneos/daemon/dplaned -db /var/lib/dplaneos/dplaneos.db -backup-path /mnt/usb/dplaneos.db.backup
```
Creates a VACUUM INTO backup on startup + every 24 hours.

## Features

- **Storage:** ZFS pools, snapshots, replication, encryption, quotas, per-user/group quotas, file explorer
- **Compute:** Docker container management, app modules, Docker Compose, safe container updates via ZFS snapshots
- **Network:** Interface config, routing, DNS, VLAN, bond/LACP, NTP
- **Identity:** User management, groups, LDAP/Active Directory, JIT provisioning
- **Security:** RBAC (4 roles, 34 permissions), audit logging, API tokens, firewall, command whitelist with regex validation
- **Monitoring:** Real-time metrics, IOStat, ZFS events, IPMI/BMC sensor data, disk health predictor
- **System:** Settings, logs, UPS management, hardware detection, NixOS config guard
- **UI:** Material Design 3, dark theme, responsive, keyboard shortcuts

## What's New in v3.0.0

### Security Overhaul
- **Go daemon architecture:** 12 of 17 APIs migrated to daemon-based handlers, 5 wrapped for compatibility
- **Cookie-based session auth:** HttpOnly + SameSite=Strict cookies replace sessionStorage (persistent across tabs/restarts, CSRF-resistant)
- **Path validation on all file operations:** Read and write operations restricted to allowed base paths
- **Input validation on chown/chmod:** Owner, group, and permissions validated against strict regex patterns
- **Rate limiter memory management:** Periodic cleanup prevents unbounded memory growth

### Database Migrations
- Automatic schema migrations for `must_change_password` and `role` columns
- Default admin user created with password `dplaneos` and forced password change on first login

### Per-User and Per-Group ZFS Quotas
Native `zfs userquota` / `zfs groupquota` support via API.

### IPMI/BMC Sensor Data
Hardware monitoring via ipmitool (graceful no-op if unavailable).

### Previous Features (v2.x)
- ZFS Time Machine (browse snapshots as folders, single-file restore)
- Ephemeral Docker Sandbox (ZFS clones, zero disk cost)
- ZFS Health Predictor (per-disk error tracking, S.M.A.R.T. integration)
- ZFS Replication (native `zfs send | ssh remote zfs recv`)
- NixOS Config Guard (validate before apply, generation rollback)
- LDAP/Active Directory (AD, OpenLDAP, FreeIPA presets, group→role mapping, TLS 1.2+)

## Architecture

- **Frontend:** HTML5 + Material Design 3, flyout navigation, no framework dependencies
- **Backend:** Go daemon (`dplaned`) on port 9000, 171+ API routes
- **Database:** SQLite with WAL mode, `synchronous=FULL`, daily VACUUM INTO backup
- **Web Server:** nginx reverse proxy (TLS termination, security headers)
- **Storage:** ZFS (native kernel module) + ZED hook for real-time disk failure alerts
- **Security:** Command whitelist (regex-validated), RBAC, bcrypt passwords, session expiry with idle timeout, audit logging
- **NixOS:** Full support via Flake — entire NAS defined in a single `configuration.nix`

## Documentation

- `CHANGELOG.md` — Full version history
- `ADMIN-GUIDE.md` — Full administration guide
- `ERROR-REFERENCE.md` — API error codes and diagnostics
- `TROUBLESHOOTING.md` — Build issues, ZED setup, common fixes
- `nixos/README.md` — NixOS installation and configuration
- `LDAP-REFERENCE.md` — LDAP technical reference
- `INSTALLATION-GUIDE.md` — Detailed installation steps

## License

[PolyForm Shield 1.0.0](https://polyformproject.org/licenses/shield/1.0.0/) — free to use, modify, and distribute, but not to compete with D-PlaneOS. See [LICENSE](LICENSE) for full terms.
