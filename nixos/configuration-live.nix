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

    # Disk/partition management (reused from installer)
    # Not used for live boot (no installation), but kept for future install-to-disk option
    # ../inputs/disko/nixosModules.disko
    # ./disko.nix

    # Ephemeral filesystem management
    ../inputs/impermanence/nixosModules.impermanence
    ./impermanence.nix

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
  boot.loader.efi.canTouchEfiVariables = false;

  # Enable impermanence for ephemeral root
  boot.impermanence.enable = lib.mkForce true;

  # ── Kernel and ZFS configuration (match installed system) ──────────────────
  boot.kernelPackages = pkgs.linuxPackages_6_12;
  boot.zfs.package = pkgs.zfs;
  boot.supportedFilesystems = [ "zfs" "vfat" "ext4" ];

  # ARC cache: 17GB (same as installed, for consistent behavior)
  boot.kernelParams = [ "zfs.zfs_arc_max=17179869184" ];

  # ── Network configuration (same as standalone) ────────────────────────────
  networking.useNetworkd = true;
  networking.useDHCP = true;

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
  services.openssh = {
    enable = true;
    settings = {
      PermitRootLogin = "yes";
      PasswordAuthentication = true;
      UsePAM = true;
    };
  };

  # ── System packages (minimal, reuse from standalone) ────────────────────
  # Additional tools useful in live environment are added in live-iso.nix

  # ── No automatic garbage collection in live (tmpfs-based) ──────────────
  nix.gc.automatic = false;

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
