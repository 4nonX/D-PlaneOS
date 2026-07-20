# D-PlaneOS Live Boot Configuration
# ─────────────────────────────────────────────────────────────────────────────
# Boots directly into D-PlaneOS without requiring installation to disk.
# System runs from RAM with optional USB persistence.
#
# Key differences from installed system:
#   - Root filesystem: tmpfs (ephemeral)
#   - Persistence: optional USB mount
#   - ZFS pools: auto-imported from attached drives
#   - OTA updates: disabled (no persistent /boot)
#   - Daemon state: lives in RAM or optional USB
#
# Boot flow:
#   1. ISO boots with read-only squashfs root
#   2. impermanence overlays /var, /etc with tmpfs
#   3. dplane-zfs-auto-import scans and imports any pools
#   4. D-PlaneOS daemon starts and serves UI on port 9000
#   5. User can manage pools, run Docker, optionally install to disk

{ config, lib, pkgs, ... }:

{
  imports = [
    # Base configuration (shared with installed system)
    ./configuration-standalone.nix
    ./module.nix

    # Live-specific additions
    ./live-zfs-auto-import.nix
    ./live-persistence.nix

    # System generation scripts (shared)
    ./ota-module.nix
    ./dplane-generated.nix
  ];

  # ── Disable features not applicable to live boot ───────────────────────────

  # OTA updates require persistent /boot partition. Live boot has no persistent storage.
  services.dplaneos.ota.enable = lib.mkForce false;

  # Boot loader is not applicable to live ISO (boots from GRUB/bootloader already)
  boot.loader.grub.enable = false;
  # Use mkForce to override applianceConfig's true (live ISO doesn't use EFI)
  boot.loader.efi.canTouchEfiVariables = lib.mkForce false;

  # ── Kernel and ZFS configuration (match installed system) ──────────────────
  # Use mkDefault so applianceConfig can set this without conflict
  boot.kernelPackages = lib.mkDefault pkgs.linuxPackages_6_12;
  boot.zfs.package = lib.mkDefault pkgs.zfs;
  boot.supportedFilesystems = [ "zfs" "vfat" "ext4" ];

  # ARC cache: 17GB (same as installed, for consistent behavior)
  # Use mkDefault so applianceConfig can set this without conflict
  boot.kernelParams = lib.mkDefault [ "zfs.zfs_arc_max=17179869184" ];

  # ── Network configuration (live boot requires DHCP for automatic connectivity) ────────────────────────────
  networking.useNetworkd = true;
  # Use mkForce to override standalone's false (live boot needs DHCP to get network automatically)
  networking.useDHCP = lib.mkForce true;

  # ── D-PlaneOS daemon and frontend ──────────────────────────────────────────
  # (Provided by applianceConfig in flake.nix, same as installed system)
  # services.dplaneos.daemonPackage = ...
  # services.dplaneos.frontendPackage = ...

  # ── Assertions (same as installed system) ────────────────────────────────
  assertions = [
    {
      assertion = lib.versionAtLeast config.boot.kernelPackages.kernel.version "6.12";
      message = "D-PlaneOS requires Linux kernel >= 6.12 LTS.";
    }
    {
      assertion = lib.versionAtLeast config.boot.zfs.package.version "2.3";
      message = "D-PlaneOS requires OpenZFS >= 2.3 (LTS branch).";
    }
  ];

  # ── System information in MOTD ───────────────────────────────────────────
  services.getty.helpLine = ''

    Welcome to D-PlaneOS Live Boot
    ──────────────────────────────────────────────────────────────────────

    Web UI:              http://localhost:9000
    SSH:                 root@<ip-address>

    ZFS pools will be automatically imported from attached drives.

    Commands:
      zpool list              # List pools
      zfs list                # List datasets
      docker ps               # List containers

    System state is ephemeral (lost on shutdown).
    To persist daemon config, mount USB labeled "dplane-persist".

    Type 'exit' to return to login prompt.
  '';

  # ── SSH for remote access to live environment ───────────────────────────
  # Live boot allows password auth for convenience; override module.nix's false
  services.openssh = {
    enable = true;
    settings = {
      PermitRootLogin = lib.mkForce "yes";
      PasswordAuthentication = lib.mkForce true;
      UsePAM = true;
    };
  };

  # ── System packages (minimal, reuse from standalone) ────────────────────
  # Additional tools useful in live environment are added in live-iso.nix

  # ── No automatic garbage collection in live (tmpfs-based) ──────────────
  # Use mkForce to override applianceConfig's true (tmpfs limited, GC would thrash)
  nix.gc.automatic = lib.mkForce false;

  # ── Documentation disabled (appliance, live environment) ────────────────
  documentation.nixos.enable = false;

  # ── System state: use impermanence for ephemeral root ───────────────────
  # See live-persistence.nix for details on /persist mount

  # ── Ensure services start after ZFS auto-import ───────────────────────
  systemd.services.dplaneos = lib.mkIf (config.services.dplaneos.enable or false) {
    after = [ "dplane-zfs-auto-import.service" ];
  };

  systemd.services.docker = lib.mkIf config.virtualisation.docker.enable {
    after = [ "dplane-zfs-auto-import.service" ];
  };
}
