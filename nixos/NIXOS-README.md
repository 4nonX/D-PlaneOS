# D-PlaneOS on NixOS — The Immutable NAS

NixOS is a **first-class platform** for D-PlaneOS. The combination gives you
something no other NAS can offer:

| Layer | Technology | What it means |
|-------|-----------|--------------|
| System | NixOS | Declarative, reproducible, rollback with one command |
| Data | ZFS | Snapshots, checksums, encryption, compression |
| Containers | GitOps | Docker stacks version-controlled in Git repos |

Every piece of your NAS state is either **declarative** (Nix), **snapshotted** (ZFS),
or **version-controlled** (Git). Nothing is ever lost.

## Quick Start

```bash
git clone https://github.com/4nonX/D-PlaneOS
cd D-PlaneOS/nixos
sudo bash setup-nixos.sh
sudo nixos-rebuild switch --flake .#dplaneos
```

`setup-nixos.sh` auto-detects your ZFS pools, timezone, and boot loader.

## Update

```bash
cd D-PlaneOS/nixos
git pull
sudo nixos-rebuild switch --flake .#dplaneos
```

## Rollback

Something broke? One command:

```bash
sudo nixos-rebuild switch --rollback
```

Or pick a specific generation:

```bash
nixos-rebuild list-generations
sudo nixos-rebuild switch --generation 42
```

## What's Included

The NixOS configuration sets up everything the Debian installer does:

- **ZFS** — pools, auto-scrub, auto-snapshots (15min/hourly/daily/weekly/monthly)
- **Samba** — SMB file sharing with performance tuning
- **NFS** — Unix/Linux file sharing
- **Docker** — native ZFS storage driver, weekly auto-prune
- **Docker-ZFS boot gate** — Docker waits for ZFS pools before starting
- **SMART monitoring** — disk health with wall notifications
- **Avahi** — `dplaneos.local` mDNS discovery
- **Nginx** — reverse proxy with 7-day WebSocket timeout
- **Firewall** — HTTP, SMB, NFS, SSH ports open
- **Daily DB backups** — systemd timer with 30-day retention
- **Recovery CLI** — `sudo dplaneos-recovery`
- **Removable media** — udev rules for USB device detection

## Vendor Hash (First Build)

Nix requires a hash of Go dependencies. On first build, you'll see:

```
hash mismatch in fixed-output derivation:
  got: sha256-AbCdEf1234...=
```

Copy that hash into `flake.nix` replacing `vendorHash = null;` and rebuild.
`setup-nixos.sh` attempts to do this automatically.

## Files

| File | Purpose |
|------|---------|
| `flake.nix` | Nix flake — builds dplaned, frontend, recovery CLI |
| `configuration.nix` | Full NixOS system config (flake version) |
| `configuration-standalone.nix` | Standalone version (no flake, imports packages directly) |
| `setup-nixos.sh` | Interactive setup helper |

## Why NixOS for a NAS?

- **Atomic upgrades**: System updates are all-or-nothing. No partial states.
- **Generations**: Every config change creates a bootable snapshot. Pick any one.
- **Reproducible**: Same flake.nix = same system, anywhere, anytime.
- **Git-native**: Your entire NAS config is a Git repo. `git log` is your changelog.
- **No drift**: The system *is* the config file. Nothing else.
