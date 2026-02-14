# D-PlaneOS NixOS — Technical Reference

## Zero Code Changes Required

The D-PlaneOS daemon supports configurable paths via CLI flags. The NixOS configuration passes these flags automatically:

```
dplaned \
  -config-dir /var/lib/dplaneos/config \
  -smb-conf /var/lib/dplaneos/smb-shares.conf \
  -db /var/lib/dplaneos/dplaneos.db \
  -listen 127.0.0.1:9000
```

On Debian, the defaults (`/etc/dplaneos`, `/etc/samba/smb.conf`) are used automatically — no flags needed. The same binary works on both distros.

## Key Difference from Debian

On Debian, `install.sh` writes config files to `/etc/` imperatively. On NixOS, everything is declared in `configuration.nix`:

| Debian | NixOS |
|--------|-------|
| `smb.conf` written directly by daemon | Daemon writes to `/var/lib/dplaneos/smb-shares.conf`, included by Nix-managed Samba |
| `apt install` packages | Packages declared in `environment.systemPackages` |
| `systemctl enable` services | Services declared in `services.*` or `systemd.services.*` |
| Config drift over time | Configuration is the single source of truth |
| Broken update → manual fix | Broken update → `nixos-rebuild switch --rollback` |

## Rollback

```bash
# Last working version:
sudo nixos-rebuild switch --rollback

# List all generations:
sudo nix-env --list-generations --profile /nix/var/nix/profiles/system

# Boot menu: select an older generation at the GRUB/systemd-boot screen
```

## Version Control Your NAS

```bash
cd /etc/nixos
sudo git init
sudo git add .
sudo git commit -m "D-PlaneOS v2.0.0 initial setup"

# After any change:
sudo git add -A && sudo git commit -m "description of change"
sudo nixos-rebuild switch
```

## Automatic Updates (optional)

Add to `configuration.nix`:
```nix
  system.autoUpgrade = {
    enable = true;
    dates = "04:00";
    allowReboot = false;
  };
```

## Security Features

- `sudo` requires password (`wheelNeedsPassword = true`)
- Daemon runs with `ProtectSystem = "strict"`, `PrivateTmp`, capability bounding
- SQLite backups use `.backup` command (WAL-safe, consistent snapshots)
- Daemon starts only after `zfs-mount.service` completes (no race condition)
- OOM protection: daemon limited to 1 GB, `OOMScoreAdjust = -900`
- Nginx blocks PHP execution, hidden files, and sensitive directories

## Getting Package Hashes

For the standalone configuration (not needed for flake):

```bash
# Source hash:
nix-shell -p nix-prefetch-github --run "nix-prefetch-github 4nonX dplaneos --rev v2.0.0"

# Vendor hash (Go dependencies):
# Set vendorHash = ""; then run nixos-rebuild switch
# The error message shows the correct hash
```
