# D-PlaneOS v2.2.0 — Installation Guide

**Advanced NAS with Declarative State & RBAC**

---

## 1. System Requirements & Hardware Validation

### Hardware Minimums:
- CPU: Dual-core x86_64 (AES-NI support required for ZFS encryption)
- RAM: 4 GB Minimum. 16 GB+ is recommended for ZFS L2ARC/ZIL performance
- Storage: 20 GB for OS; separate physical drives for ZFS Data Pools
- Network: 1 Gbps Ethernet (v2.2.0 supports Multi-Path TCP)

### v2.2.0 Integrity Standards:
- ECC RAM: Strongly recommended. D-PlaneOS 2.2.0 uses dmidecode to check memory type. Non-ECC hardware will function but will trigger a "Data Integrity Risk" warning in the UI
- ZFS-Atomic Updates: This version utilizes ZFS snapshots to clone the environment before any container or system update. If the update fails, the Go-daemon performs an automated rollback to the previous snapshot

---

## 2. NixOS Installation (Declarative Flake)

v2.2.0 is optimized for NixOS. This ensures your NAS configuration is version-controlled and immutable.

### Step 1: Update flake.nix

Include the D-PlaneOS input and pass it to your modules:

{
  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs/nixos-24.05";
    dplaneos.url = "github:your-repo/dplaneos/v2.2.0";
  };

  outputs = { self, nixpkgs, dplaneos, ... }: {
    nixosConfigurations.nas-node = nixpkgs.lib.nixosSystem {
      system = "x86_64-linux";
      modules = [
        ./configuration.nix
        dplaneos.nixosModules.dplaneos
      ];
    };
  };
}

services.dplaneos = {
  enable = true;
  gitops.enable = true;       # Enables declarative state mirroring
  zfsBootGate = true;          # System holds boot until ZFS pools are online
  rbac.enforceHardened = true; # Disables default 'admin' if no password set
};

---

## 3. Standard Linux Installation (Ubuntu/Debian/RHEL)

Use this for traditional distros where Nix is not the primary manager.

Step 1: Download & Verify
wget https://github.com/your-repo/dplaneos/releases/download/v2.2.0/dplaneos-v2.2.0-HARDENED.tar.gz
sha256sum dplaneos-v2.2.0-HARDENED.tar.gz

Step 2: Installation Script
tar -xzf dplaneos-v2.2.0-HARDENED.tar.gz
cd dplaneos-v2.2.0
sudo ./install-v2.2.0.sh

What the Installer Does:
- Installs zfsutils-linux, docker.io, and sqlite3
- Compiles the Go-Daemon (v2.2.0) from source
- Sets up dplaneos-zfs-mount-wait.service to prevent UI startup on empty pools
- Generates a hardened Nginx config with TLS 1.3 defaults

---

## 4. Setup Wizard & Initial Logic

Once the service is running, navigate to https://<your-ip>

Step 1: Storage Configuration
The wizard will detect available drives. v2.2.0 logic defaults to RAID-Z2 for any array with 4+ drives to ensure enterprise-grade redundancy. ZFS encryption can be toggled here, with keys managed by the internal secure vault

Step 2: GitOps Identity
The system generates a unique Ed25519 SSH key. Add this key to your Git provider (GitHub/GitLab) as a Deploy Key with write access. This allows D-PlaneOS to save every UI change as a Git commit

Step 3: RBAC & Security
Create your primary administrator. v2.2.0 requires a minimum 12-character password with mandatory entropy checks. MFA (TOTP) can be enabled immediately after the first login

---

## 5. Maintenance & Troubleshooting

The "Boot-Gate" Protocol
If the UI displays `423 Locked: Storage Not Ready`, the Boot-Gate is working
Cause: One or more ZFS pools failed to mount during boot
Solution: Check hardware or run `zpool import -a` manually. The UI will unlock automatically once the pool is healthy

Logs & Monitoring
- Core Daemon: journalctl -u dplaned -f
- ZFS Health: zpool status
- Git Sync Status: Check the "GitOps" tab in the dashboard for sync lag or authentication errors

Uninstallation
- Manual Linux: Run sudo ./uninstall.sh inside the release folder
- NixOS: Remove the module from flake.nix, delete the service config, and run sudo nixos-rebuild switch
