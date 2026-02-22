# D-PlaneOS v3.2.1 — Enterprise NAS Operating System

Open-source NAS OS with Material Design 3 UI, ZFS storage, Docker containers, RBAC, and LDAP/Active Directory integration.

## Quick Start

### Debian/Ubuntu (one-step full NAS)

**Option A — one-liner (no download first):**
```bash
curl -fsSL https://get.dplaneos.io | sudo bash
```

**Option B — from release tarball:**
```bash
tar xzf dplaneos-v3.2.1.tar.gz
cd dplaneos-v3.2.1   # or the extracted directory name
sudo ./install.sh
```

The installer sets up the daemon, nginx, ZFS tools, **Samba (SMB/CIFS), NFS, and Docker** so you get a full NAS in one run. When it finishes, open **http://your-server** in a browser.

**Default login:** username `admin`; password is shown at the end of install (or set via setup wizard on first login).

### NixOS
```bash
cd D-PlaneOS/nixos
sudo bash setup-nixos.sh
sudo nixos-rebuild switch --flake .#dplaneos
```

See [nixos/README.md](nixos/README.md) for the full NixOS guide.

### Minimal install (binary only, no Samba/NFS/Docker)

If you already have nginx and dependencies and only want the daemon: `sudo make install` then `sudo systemctl start dplaned`. See [INSTALLATION-GUIDE.md](INSTALLATION-GUIDE.md) for details.

> **Building from source?** You need Go 1.21+ and gcc. Run `./install.sh` — it will build the daemon if no pre-built binary is present.

### Off-Pool Database Backup (recommended for large pools)

Edit `/etc/systemd/system/dplaned.service` and add `-backup-path`:
```
ExecStart=/opt/dplaneos/daemon/dplaned -db /var/lib/dplaneos/dplaneos.db -backup-path /mnt/usb/dplaneos.db.backup
```
Creates a VACUUM INTO backup on startup + every 24 hours.

## Features

- **Storage:** ZFS pools, snapshots, replication, encryption, quotas, file explorer
- **Compute:** Docker container management, app modules, Docker Compose
- **Network:** Interface config, routing, DNS
- **Identity:** User management, groups, LDAP/Active Directory
- **Security:** RBAC (4 roles), audit logging, API tokens, firewall
- **System:** Settings, logs, UPS management, hardware detection
- **UI:** Material Design 3, dark theme, responsive, keyboard shortcuts

## Features (v3.2.1)

### Safe Container Updates
`POST /api/docker/update` — ZFS snapshot → pull → restart → health check. On failure: instant rollback. No other NAS OS does this.

### ZFS Time Machine
Browse any snapshot like a folder. Find a deleted file from yesterday, restore just that one file — no full rollback needed.

### Ephemeral Docker Sandbox
Test any container on a ZFS clone (zero disk cost). Stop the container, the clone disappears. No residue.

### ZFS Health Predictor
Deep pool health monitoring: per-disk error tracking, checksum error detection, risk levels (low/medium/high/critical), S.M.A.R.T. integration. Warns you before a disk dies, not after.

### NixOS Config Guard
On NixOS systems: validate `configuration.nix` before applying, list/rollback generations, dry-activate checks. Cannot brick your system.

### ZFS Replication (Remote)
Native `zfs send | ssh remote zfs recv` — block-level replication that's 100x faster than rsync and preserves all snapshots on the remote.

## LDAP / Active Directory

Navigate to **Identity → Directory Service** to configure. Supports:

- Active Directory, OpenLDAP, FreeIPA (one-click presets)
- Group → Role mapping (auto-assign permissions)
- Just-In-Time user provisioning
- Background sync with audit trail
- TLS 1.2+ enforced

## Architecture

- **Frontend:** HTML5 + Material Design 3, flyout navigation, no framework dependencies
- **Backend:** Go daemon (`dplaned`, 8MB) on port 9000, 170+ API routes
- **Database:** SQLite with WAL mode, `synchronous=FULL`, daily `.backup` (WAL-safe)
- **Web Server:** nginx reverse proxy (TLS termination)
- **Storage:** ZFS (native kernel module) + ZED hook for real-time disk failure alerts
- **Security:** Input validation on all exec.Command (regex whitelist), RBAC (4 roles, 28 permissions), injection-hardened, OOM-protected (512MB limit)
- **NixOS:** Full support via Flake — entire NAS defined in a single `configuration.nix`

## Documentation

- `CHANGELOG.md` — Full version history
- `ADMIN-GUIDE.md` — Full administration guide
- `ERROR-REFERENCE.md` — API error codes and diagnostics
- `TROUBLESHOOTING.md` — Build issues, ZED setup, common fixes
- `nixos/README.md` — NixOS installation and configuration
- `INSTALLATION-GUIDE.md` — Detailed installation steps

## License

Open source. See LICENSE file.
