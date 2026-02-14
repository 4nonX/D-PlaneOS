# D-PlaneOS on NixOS

> Your complete NAS defined in a single text file. Reproducible, versioned, indestructible.

## Two Installation Paths

### Path 1: Flake (recommended)

Reproducible, pinned versions, one command to update.

```bash
# On a running NixOS:
git clone https://github.com/4nonX/D-PlaneOS /tmp/dplaneos
cd /tmp/dplaneos/nixos

# Run setup helper (fills in host ID, timezone, etc.)
sudo bash setup-nixos.sh

# Build the system
sudo nixos-rebuild switch --flake .#dplaneos

# Open browser → http://dplaneos.local
```

**Update:**
```bash
cd /tmp/dplaneos/nixos && git pull
sudo nixos-rebuild switch --flake .#dplaneos
```

**Rollback:**
```bash
sudo nixos-rebuild switch --rollback
```

### Path 2: Standalone (without Flake)

Simpler if you don't want to use Flakes yet.

```bash
sudo cp configuration-standalone.nix /etc/nixos/configuration.nix
sudo bash setup-nixos.sh
sudo nixos-rebuild switch
```

## Files

| File | Description |
|------|-------------|
| `flake.nix` | Flake definition — packages, inputs, dev shell |
| `configuration.nix` | NAS config (Flake version, receives packages via `specialArgs`) |
| `configuration-standalone.nix` | NAS config (standalone, packages defined inline) |
| `setup-nixos.sh` | Setup helper — generates host ID, detects boot loader |
| `NIXOS-INSTALL-GUIDE.md` | Complete step-by-step guide for beginners |
| `NIXOS-README.md` | Technical reference, rollback, security details |

## System Requirements

- NixOS 24.11 (stable)
- Minimum 8 GB RAM (recommended: 32 GB for 16 GB ZFS ARC)
- Separate boot disk (SSD) + data disks (HDD/SSD for ZFS pool)
- Network connection

## What's Included

| Component | Details |
|-----------|---------|
| **ZFS** | Auto-import, monthly scrub, auto-snapshots (15min/hourly/daily/weekly/monthly) |
| **D-PlaneOS Daemon** | systemd service, OOM-protected (1 GB), hardened (ProtectSystem=strict) |
| **nginx** | Reverse proxy, security headers, PHP blocked |
| **Docker** | ZFS storage driver, weekly prune |
| **Samba** | Performance-tuned, dynamic shares via daemon |
| **NFS** | Server enabled |
| **S.M.A.R.T.** | Automatic disk health monitoring |
| **Firewall** | Only ports 80, 443, 445, 2049 open |
| **mDNS** | NAS discoverable as `dplaneos.local` |
| **SSH** | Password login for initial setup, SSH keys recommended after |
| **Backups** | Daily SQLite backup at 3 AM (`.backup`, WAL-safe) |
