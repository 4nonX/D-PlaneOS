# ═══════════════════════════════════════════════════════════════
#  D-PlaneOS v3.0.0 — NixOS Flake
# ═══════════════════════════════════════════════════════════════
#
#  Your complete NAS as a Git repo.
#
#  First install:
#    git clone https://github.com/4nonX/D-PlaneOS
#    cd D-PlaneOS/nixos
#    sudo bash setup-nixos.sh
#    sudo nixos-rebuild switch --flake .#dplaneos
#
#  Update:
#    git pull
#    sudo nixos-rebuild switch --flake .#dplaneos
#
#  Rollback:
#    sudo nixos-rebuild switch --rollback
#
# ═══════════════════════════════════════════════════════════════
{
  description = "D-PlaneOS — NAS Operating System on NixOS";

  # ─── Inputs ──────────────────────────────────────────────

  inputs = {
    # NixOS 24.11 (stable)
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-24.11";

    # D-PlaneOS source code
    dplaneos-src = {
      url = "github:4nonX/dplaneos/v3.0.0";
      flake = false;
    };
  };

  # ─── Outputs ─────────────────────────────────────────────

  outputs = { self, nixpkgs, dplaneos-src, ... }:
  let
    system = "x86_64-linux";
    pkgs = import nixpkgs {
      inherit system;
      config.allowUnfreePredicate = pkg: builtins.elem (nixpkgs.lib.getName pkg) [
        "dplaned"
        "dplaneos-frontend"
        "dplaneos-recovery"
      ];
    };

    # ─── D-PlaneOS Packages ──────────────────────────────

    # Go daemon
    dplaned = pkgs.buildGoModule {
      pname = "dplaned";
      version = "3.0.0";
      src = dplaneos-src;
      sourceRoot = "source/daemon";

      # Go dependencies hash
      # First build: set to null → error message shows correct hash
      vendorHash = null;

      # SQLite requires CGO
      CGO_ENABLED = 1;
      nativeBuildInputs = [ pkgs.pkg-config ];
      buildInputs = [ pkgs.sqlite ];

      meta = with pkgs.lib; {
        description = "D-PlaneOS NAS System Daemon";
        homepage = "https://github.com/4nonX/dplaneos";
        license = licenses.unfree;  # PolyForm Shield 1.0.0
        platforms = platforms.linux;
      };
    };

    # Frontend (static files)
    dplaneos-frontend = pkgs.stdenv.mkDerivation {
      pname = "dplaneos-frontend";
      version = "3.0.0";
      src = dplaneos-src;

      installPhase = ''
        mkdir -p $out
        cp -r app/* $out/
      '';
    };

    # Recovery CLI
    dplaneos-recovery = pkgs.writeShellScriptBin "dplaneos-recovery" ''
      DB="/var/lib/dplaneos/dplaneos.db"
      echo "D-PlaneOS Recovery CLI v3.0.0"
      echo ""
      echo "1) Reset admin password"
      echo "2) Show system status"
      echo "3) Create database backup"
      echo "4) Exit"
      read -p "Choice: " choice
      case $choice in
        1)
          read -sp "New admin password: " pw; echo
          HASH=$(${pkgs.python3.withPackages (p: [p.bcrypt])}/bin/python3 -c "import bcrypt,sys; print(bcrypt.hashpw(sys.argv[1].encode(), bcrypt.gensalt(10)).decode())" "$pw")
          ${pkgs.sqlite}/bin/sqlite3 "$DB" "UPDATE users SET password_hash='$HASH', must_change_password=1 WHERE username='admin';"
          echo "Admin password reset. You must change it on next login."
          ;;
        2)
          echo "Database: $(du -h $DB 2>/dev/null || echo 'not found')"
          echo "Sessions: $(${pkgs.sqlite}/bin/sqlite3 "$DB" "SELECT COUNT(*) FROM sessions;" 2>/dev/null || echo '0')"
          echo "Users: $(${pkgs.sqlite}/bin/sqlite3 "$DB" "SELECT COUNT(*) FROM users;" 2>/dev/null || echo '0')"
          ;;
        3)
          mkdir -p /var/lib/dplaneos/backups
          BACKUP="/var/lib/dplaneos/backups/dplaneos-$(date +%Y%m%d-%H%M%S).db"
          ${pkgs.sqlite}/bin/sqlite3 "$DB" ".backup $BACKUP"
          echo "Backup saved: $BACKUP"
          ;;
        *) echo "Bye." ;;
      esac
    '';

  in {

    # ─── NixOS System Configuration ─────────────────────

    nixosConfigurations.dplaneos = nixpkgs.lib.nixosSystem {
      inherit system;

      specialArgs = {
        inherit dplaned dplaneos-frontend dplaneos-recovery;
      };

      modules = [
        /etc/nixos/hardware-configuration.nix
        ./configuration.nix
      ];
    };

    # ─── Installer ISO ──────────────────────────────────
    #
    # Build:  nix build .#nixosConfigurations.dplaneos-iso.config.system.build.isoImage
    # Result: result/iso/dplaneos-3.0.0-x86_64-linux.iso
    # Flash:  dd if=result/iso/*.iso of=/dev/sdX bs=4M status=progress
    #
    # The ISO boots into a live D-PlaneOS environment:
    #   - Web UI available at http://dplaneos.local or the IP shown on screen
    #   - Dashboard runs in live mode (no data persisted)
    #   - "Install to Disk" option in the web UI runs the installer
    #   - Recovery: boot the ISO to mount and repair existing ZFS pools
    #

    nixosConfigurations.dplaneos-iso = nixpkgs.lib.nixosSystem {
      inherit system;

      specialArgs = {
        inherit dplaned dplaneos-frontend dplaneos-recovery;
      };

      modules = [
        "${nixpkgs}/nixos/modules/installer/cd-dvd/installation-cd-minimal.nix"

        ({ config, pkgs, lib, ... }: {

          # ── ISO metadata ────────────────────────────────
          isoImage.isoName = "dplaneos-3.0.0-x86_64-linux.iso";
          isoImage.volumeID = "DPLANEOS_3";
          isoImage.appendToMenuLabel = " D-PlaneOS Installer";

          # ── License ─────────────────────────────────────
          nixpkgs.config.allowUnfreePredicate = pkg:
            builtins.elem (nixpkgs.lib.getName pkg) [
              "dplaned" "dplaneos-frontend" "dplaneos-recovery"
            ];

          # ── System packages ─────────────────────────────
          environment.systemPackages = with pkgs; [
            dplaned dplaneos-frontend dplaneos-recovery

            # ZFS + storage
            zfs smartmontools hdparm parted
            dmidecode acl ethtool lsof pciutils usbutils

            # File sharing
            samba nfs-utils

            # Docker
            docker docker-compose

            # Utilities
            rclone rsync git openssh
            htop tmux nano curl wget sqlite
            dialog  # for the TUI installer
          ];

          # ── ZFS support in live environment ─────────────
          boot.supportedFilesystems = [ "zfs" ];
          boot.zfs.forceImportRoot = false;
          networking.hostId = "deadbeef";  # placeholder, installer replaces

          # ── Networking ──────────────────────────────────
          networking.hostName = "dplaneos";
          networking.firewall.allowedTCPPorts = [ 80 443 445 2049 22 ];
          services.avahi = {
            enable = true;
            nssmdns4 = true;
            publish = { enable = true; addresses = true; };
          };

          # ── Live D-PlaneOS daemon ───────────────────────
          # Runs in live mode — data stored in tmpfs, lost on reboot
          systemd.tmpfiles.rules = [
            "d /var/lib/dplaneos 0750 root root -"
            "d /var/lib/dplaneos/config 0750 root root -"
            "d /var/lib/dplaneos/backups 0750 root root -"
            "d /var/log/dplaneos 0750 root root -"
            "d /run/dplaneos 0755 root root -"
            "d /opt/dplaneos/app 0755 root root -"
          ];

          # Copy frontend to expected location
          system.activationScripts.dplaneos-frontend = ''
            if [ ! -d /opt/dplaneos/app/pages ]; then
              cp -r ${dplaneos-frontend}/* /opt/dplaneos/app/
              chmod -R 755 /opt/dplaneos/app
            fi
          '';

          # dplaned service (live mode)
          systemd.services.dplaned = {
            description = "D-PlaneOS System Daemon (Live)";
            after = [ "network.target" ];
            wantedBy = [ "multi-user.target" ];

            serviceConfig = {
              Type = "simple";
              ExecStart = "${dplaned}/bin/dplaned -db /var/lib/dplaneos/dplaneos.db -listen 127.0.0.1:9000";
              WorkingDirectory = "/opt/dplaneos";
              Restart = "always";
              RestartSec = 5;
              MemoryMax = "1G";
              MemoryHigh = "768M";
              OOMScoreAdjust = -900;
            };
          };

          # ── nginx reverse proxy ─────────────────────────
          services.nginx = {
            enable = true;
            virtualHosts."_" = {
              default = true;
              root = "/opt/dplaneos/app";

              locations."/" = {
                tryFiles = "$uri $uri/ /pages/index.html";
              };

              locations."/api/" = {
                proxyPass = "http://127.0.0.1:9000";
                extraConfig = ''
                  proxy_http_version 1.1;
                  proxy_set_header Host $host;
                  proxy_set_header X-Real-IP $remote_addr;
                  proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
                  proxy_read_timeout 120s;
                '';
              };

              locations."/ws/" = {
                proxyPass = "http://127.0.0.1:9000";
                proxyWebsockets = true;
                extraConfig = ''
                  proxy_connect_timeout 7d;
                  proxy_send_timeout 7d;
                  proxy_read_timeout 7d;
                '';
              };

              locations."/health" = {
                proxyPass = "http://127.0.0.1:9000/health";
              };

              extraConfig = ''
                client_max_body_size 10G;

                add_header X-Frame-Options "SAMEORIGIN" always;
                add_header X-Content-Type-Options "nosniff" always;

                location ~ \.php$ { deny all; }
                location ~ /\. { deny all; }
                location ~ /(config|daemon|scripts|systemd)/ { deny all; }
              '';
            };
          };

          # ── Docker (available but not started by default) ──
          virtualisation.docker.enable = true;

          # ── Installer script ────────────────────────────
          # Available as 'dplaneos-install' in the live environment
          environment.etc."dplaneos-installer.sh" = {
            mode = "0755";
            text = ''
              #!/usr/bin/env bash
              set -euo pipefail

              BOLD='\033[1m'
              NC='\033[0m'
              GREEN='\033[0;32m'
              RED='\033[0;31m'
              YELLOW='\033[1;33m'

              echo -e "''${BOLD}"
              echo "  ┌─────────────────────────────────────────┐"
              echo "  │     D-PlaneOS Installer v3.0.0          │"
              echo "  │     Install to disk from live ISO        │"
              echo "  └─────────────────────────────────────────┘"
              echo -e "''${NC}"
              echo ""

              # Detect disks
              echo "Available disks:"
              lsblk -d -o NAME,SIZE,MODEL,TYPE | grep disk
              echo ""

              read -p "Install NixOS to which disk? (e.g. sda, nvme0n1): " TARGET_DISK
              TARGET="/dev/$TARGET_DISK"

              if [ ! -b "$TARGET" ]; then
                echo -e "''${RED}Error: $TARGET is not a block device''${NC}"
                exit 1
              fi

              echo ""
              echo -e "''${YELLOW}WARNING: This will ERASE $TARGET entirely.''${NC}"
              echo "Data disks (for ZFS pool) are NOT affected."
              read -p "Type 'yes' to continue: " CONFIRM
              [ "$CONFIRM" = "yes" ] || exit 0

              echo ""
              echo "=== Step 1/5: Partitioning $TARGET ==="

              # Detect UEFI or BIOS
              if [ -d /sys/firmware/efi ]; then
                echo "UEFI detected"
                parted "$TARGET" -- mklabel gpt
                parted "$TARGET" -- mkpart ESP fat32 1MB 512MB
                parted "$TARGET" -- set 1 esp on
                parted "$TARGET" -- mkpart primary 512MB 100%

                # Wait for partitions
                sleep 2
                partprobe "$TARGET" 2>/dev/null || true
                sleep 1

                # Determine partition naming
                if [[ "$TARGET_DISK" == nvme* ]]; then
                  BOOT="''${TARGET}p1"
                  ROOT="''${TARGET}p2"
                else
                  BOOT="''${TARGET}1"
                  ROOT="''${TARGET}2"
                fi

                mkfs.fat -F 32 -n BOOT "$BOOT"
                mkfs.ext4 -L nixos "$ROOT"

                mount "$ROOT" /mnt
                mkdir -p /mnt/boot
                mount "$BOOT" /mnt/boot
              else
                echo "BIOS/Legacy detected"
                parted "$TARGET" -- mklabel msdos
                parted "$TARGET" -- mkpart primary 1MB 100%

                sleep 2
                partprobe "$TARGET" 2>/dev/null || true
                sleep 1

                if [[ "$TARGET_DISK" == nvme* ]]; then
                  ROOT="''${TARGET}p1"
                else
                  ROOT="''${TARGET}1"
                fi

                mkfs.ext4 -L nixos "$ROOT"
                mount "$ROOT" /mnt
              fi

              echo ""
              echo "=== Step 2/5: Generating hardware config ==="
              nixos-generate-config --root /mnt

              echo ""
              echo "=== Step 3/5: Installing D-PlaneOS NixOS config ==="

              # Copy D-PlaneOS flake to the new system
              mkdir -p /mnt/etc/dplaneos
              cp -r /etc/dplaneos-source/* /mnt/etc/dplaneos/ 2>/dev/null || true

              # Generate hostId
              HOSTID=$(head -c 8 /etc/machine-id 2>/dev/null || openssl rand -hex 4)

              # Detect ZFS pools
              POOLS=$(zpool list -H -o name 2>/dev/null || echo "")

              # Generate a minimal configuration.nix that imports D-PlaneOS
              cat > /mnt/etc/nixos/configuration.nix << 'NIXEOF'
              # Generated by D-PlaneOS installer
              # Edit /etc/dplaneos/configuration.nix for NAS settings
              { config, pkgs, ... }:
              {
                imports = [ ./hardware-configuration.nix ];

                # Boot loader
              NIXEOF

              if [ -d /sys/firmware/efi ]; then
                cat >> /mnt/etc/nixos/configuration.nix << 'NIXEOF'
                boot.loader.systemd-boot.enable = true;
                boot.loader.efi.canTouchEfiVariables = true;
              NIXEOF
              else
                cat >> /mnt/etc/nixos/configuration.nix << NIXEOF
                boot.loader.grub.enable = true;
                boot.loader.grub.device = "$TARGET";
              NIXEOF
              fi

              cat >> /mnt/etc/nixos/configuration.nix << NIXEOF

                networking.hostId = "$HOSTID";
                networking.hostName = "dplaneos";

                # Enable flakes
                nix.settings.experimental-features = [ "nix-command" "flakes" ];

                # Placeholder — after first boot, switch to flake:
                #   cd /etc/dplaneos && sudo nixos-rebuild switch --flake .#dplaneos
                boot.supportedFilesystems = [ "zfs" ];
                environment.systemPackages = with pkgs; [ git nano curl ];

                # Allow SSH for first login
                services.openssh.enable = true;

                # Initial user
                users.users.admin = {
                  isNormalUser = true;
                  extraGroups = [ "wheel" ];
                  initialPassword = "dplaneos";
                };

                system.stateVersion = "24.11";
              }
              NIXEOF

              echo ""
              echo "=== Step 4/5: Installing NixOS (this takes a few minutes) ==="
              nixos-install --no-root-passwd

              echo ""
              echo "=== Step 5/5: Copying D-PlaneOS source ==="
              mkdir -p /mnt/etc/dplaneos

              echo ""
              echo -e "''${GREEN}════════════════════════════════════════════''${NC}"
              echo -e "''${GREEN}  Installation complete!''${NC}"
              echo -e "''${GREEN}════════════════════════════════════════════''${NC}"
              echo ""
              echo "  Next steps:"
              echo "  1. Reboot:  reboot"
              echo "  2. Login:   admin / dplaneos"
              echo "  3. Clone D-PlaneOS source:"
              echo "     git clone https://github.com/4nonX/D-PlaneOS /etc/dplaneos"
              echo "  4. Run setup and build:"
              echo "     cd /etc/dplaneos/nixos"
              echo "     sudo bash setup-nixos.sh"
              echo "     sudo nixos-rebuild switch --flake .#dplaneos"
              echo ""
              echo "  The web UI will be at: http://dplaneos.local"
              echo ""
            '';
          };

          # Symlink installer to PATH
          system.activationScripts.installer-link = ''
            ln -sf /etc/dplaneos-installer.sh /usr/local/bin/dplaneos-install
          '';

          # ── Boot message ────────────────────────────────
          services.getty.helpLine = lib.mkForce ''

            ╔═══════════════════════════════════════════════╗
            ║  D-PlaneOS v3.0.0 — Live Environment         ║
            ╠═══════════════════════════════════════════════╣
            ║                                               ║
            ║  Web UI:  http://dplaneos.local                ║
            ║           (or use IP shown below)             ║
            ║                                               ║
            ║  Install: dplaneos-install                    ║
            ║  Recovery: dplaneos-recovery                   ║
            ║                                               ║
            ║  Login:   root (no password)                  ║
            ╚═══════════════════════════════════════════════╝

          '';

          # Auto-login on tty1
          services.getty.autologinUser = "root";
        })
      ];
    };

    # ─── Standalone Packages ─────────────────────────────

    packages.${system} = {
      inherit dplaned dplaneos-frontend dplaneos-recovery;
      default = dplaned;
    };

    # ─── Dev Shell ───────────────────────────────────────

    devShells.${system}.default = pkgs.mkShell {
      buildInputs = with pkgs; [
        go
        gopls
        sqlite
        pkg-config
      ];
      shellHook = ''
        echo "D-PlaneOS development environment"
        echo "  go build ./cmd/dplaned/"
        echo "  go test ./..."
      '';
    };
  };
}
