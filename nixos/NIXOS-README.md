# D-PlaneOS on NixOS

## What This Is

A single `configuration.nix` that turns a bare NixOS installation into a complete D-PlaneOS NAS. Everything is declarative — ZFS, Docker, Samba, NFS, nginx, the Go daemon, firewall, monitoring, scheduled tasks.

## Quick Start

```bash
# 1. Install NixOS (minimal ISO, no desktop)
#    https://nixos.org/download

# 2. Copy configuration
sudo cp configuration.nix /etc/nixos/configuration.nix

# 3. Edit the USER CONFIG section
sudo nano /etc/nixos/configuration.nix
#    - Set your ZFS pool names
#    - Set networking.hostId (REQUIRED for ZFS)
#    - Add your SSH key
#    - Adjust ZFS ARC memory limit for your RAM

# 4. Build and switch
sudo nixos-rebuild switch

# 5. Import your existing ZFS pools (if migrating)
sudo zpool import tank

# 6. Open browser → http://<server-ip>
#    Setup wizard will guide you through initial config
```

## What You Get

| Component | Config |
|-----------|--------|
| **D-PlaneOS daemon** | systemd service, auto-restart, OOM-protected (512MB limit) |
| **nginx** | Reverse proxy on :80, security headers, PHP blocked |
| **ZFS** | Auto-import pools at boot, monthly scrub, auto-snapshots |
| **Docker** | ZFS storage driver, weekly prune, JSON logging |
| **Samba** | Daemon-managed shares via include file |
| **NFS** | Server enabled, exports managed by daemon |
| **S.M.A.R.T.** | Auto-detect all disks, wall notifications |
| **Firewall** | Ports 80, 443, 445, 2049 only |
| **mDNS** | Discoverable as `dplaneos.local` |
| **SSH** | Key-only, no root password login |
| **Backups** | Daily DB backup at 3 AM, 30-day retention |

## Key Difference from Debian

On Debian, `install.sh` writes config files to `/etc/` imperatively. On NixOS, everything is declared in `configuration.nix`. This means:

| Debian | NixOS |
|--------|-------|
| `smb.conf` written directly by daemon | Daemon writes to `/var/lib/dplaneos/smb-shares.conf`, included by Nix-managed Samba |
| `apt install` packages | Packages declared in `environment.systemPackages` |
| `systemctl enable` services | Services declared in `services.*` or `systemd.services.*` |
| Config drift over time | Configuration is the single source of truth |
| Broken update → manual fix | Broken update → `nixos-rebuild switch --rollback` |

## Zero Code Changes Required

The D-PlaneOS daemon now supports configurable paths via CLI flags. The NixOS configuration passes these flags automatically:

```
dplaned \
  -config-dir /var/lib/dplaneos/config \
  -smb-conf /var/lib/dplaneos/smb-shares.conf \
  -db /var/lib/dplaneos/dplaneos.db \
  -listen 127.0.0.1:9000
```

On Debian, the defaults (`/etc/dplaneos`, `/etc/samba/smb.conf`) are used automatically — no flags needed. The same binary works on both distros.

## Rollback

```bash
# Something broke? One command:
sudo nixos-rebuild switch --rollback

# Or pick a specific generation:
sudo nix-env --list-generations --profile /nix/var/nix/profiles/system
sudo nixos-rebuild switch --profile /nix/var/nix/profiles/system -G 42
```

## Updating D-PlaneOS

```bash
# 1. Update version and hashes in configuration.nix
# 2. Rebuild
sudo nixos-rebuild switch

# That's it. Nix handles the rest.
```

## Version Control Your NAS

```bash
# Your entire NAS config in git
cd /etc/nixos
git init
git add configuration.nix
git commit -m "D-PlaneOS v2.0.0 initial setup"

# After any change:
git add -A && git commit -m "added new ZFS pool"
sudo nixos-rebuild switch
```

## FIXME Before Use

Search for `FIXME` in `configuration.nix` — you need to fill in:

1. **Source hashes** for `dplaned` and `dplaneos-frontend` packages — run `nix-prefetch-github 4nonX dplaneos --rev v2.0.0` after tagging the release on GitHub
2. **`networking.hostId`** — unique per machine, required for ZFS. Generate: `head -c4 /dev/urandom | od -A none -t x4 | tr -d ' '`
3. **Boot loader** — UEFI vs BIOS/MBR (uncomment the right lines)
4. **SSH key** for admin user
5. **ZFS ARC max** — adjust for your RAM (rule of thumb: 1 GB per TB of storage)
