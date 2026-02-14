# ═══════════════════════════════════════════════════════════════
#  D-PlaneOS v2.0.0 — NixOS Konfiguration (Flake-Version)
# ═══════════════════════════════════════════════════════════════
#
#  Installation:
#    1. git clone https://github.com/4nonX/dplaneos-nixos
#    2. cd dplaneos-nixos
#    3. sudo bash setup-nixos.sh
#    4. sudo nixos-rebuild switch --flake .#dplaneos
#
#  Kaputt? → sudo nixos-rebuild switch --rollback
#
# ═══════════════════════════════════════════════════════════════

# dplaned, dplaneos-frontend, dplaneos-recovery kommen vom Flake
{ config, pkgs, lib, dplaned, dplaneos-frontend, dplaneos-recovery, ... }:

let

  # ┌─────────────────────────────────────────────────────────┐
  # │  HIER ÄNDERN (1/4): Dein ZFS Pool-Name                 │
  # └─────────────────────────────────────────────────────────┘
  zpools = [ "tank" ];

  # Samba Arbeitsgruppe
  sambaWorkgroup = "WORKGROUP";

in {

  # ═══════════════════════════════════════════════════════════
  #  SYSTEM
  # ═══════════════════════════════════════════════════════════

  system.stateVersion = "24.11";

  # Flakes aktivieren (damit --flake funktioniert)
  nix.settings.experimental-features = [ "nix-command" "flakes" ];

  networking.hostName = "dplaneos";

  # ┌─────────────────────────────────────────────────────────┐
  # │  HIER ÄNDERN (2/4): Host-ID für ZFS                    │
  # │                                                         │
  # │  Generiere eine mit:                                    │
  # │    head -c4 /dev/urandom | od -A none -t x4 | tr -d ' '│
  # │  Oder lass setup-nixos.sh das automatisch machen.       │
  # └─────────────────────────────────────────────────────────┘
  networking.hostId = "HIER_AENDERN";

  # ┌─────────────────────────────────────────────────────────┐
  # │  HIER ÄNDERN (3/4): Zeitzone                            │
  # └─────────────────────────────────────────────────────────┘
  time.timeZone = "Europe/Berlin";

  i18n.defaultLocale = "en_US.UTF-8";

  # ═══════════════════════════════════════════════════════════
  #  BOOTLOADER
  # ═══════════════════════════════════════════════════════════

  # ┌─────────────────────────────────────────────────────────┐
  # │  HIER ÄNDERN (4/4): Bootloader                          │
  # │                                                         │
  # │  Option A: UEFI (Standard seit ~2012) → so lassen       │
  # │  Option B: Altes BIOS → A auskommentieren, B einkomm.  │
  # └─────────────────────────────────────────────────────────┘

  # Option A: UEFI
  boot.loader.systemd-boot.enable = true;
  boot.loader.efi.canTouchEfiVariables = true;

  # Option B: BIOS/MBR
  # boot.loader.grub.enable = true;
  # boot.loader.grub.device = "/dev/sda";

  # ═══════════════════════════════════════════════════════════
  #  ZFS
  # ═══════════════════════════════════════════════════════════

  boot.supportedFilesystems = [ "zfs" ];
  boot.zfs.forceImportRoot = false;
  boot.zfs.extraPools = zpools;

  services.zfs.autoScrub = {
    enable = true;
    interval = "monthly";
  };

  services.zfs.autoSnapshot = {
    enable = true;
    frequent = 4;
    hourly = 24;
    daily = 7;
    weekly = 4;
    monthly = 12;
  };

  # ARC: 1 GB pro TB Speicher, min 4 GB
  #   16 GB RAM → 8589934592 (8 GB)
  #   32 GB RAM → 17179869184 (16 GB)  ← aktuell
  #   64 GB RAM → 34359738368 (32 GB)
  boot.kernelParams = [
    "zfs.zfs_arc_max=17179869184"
  ];

  # ═══════════════════════════════════════════════════════════
  #  KERNEL TUNING
  # ═══════════════════════════════════════════════════════════

  boot.kernel.sysctl = {
    "fs.inotify.max_user_watches" = 524288;
    "fs.inotify.max_user_instances" = 512;
    "vm.swappiness" = 10;
    "vm.vfs_cache_pressure" = 50;
    "net.core.rmem_max" = 16777216;
    "net.core.wmem_max" = 16777216;
  };

  # ═══════════════════════════════════════════════════════════
  #  NGINX
  # ═══════════════════════════════════════════════════════════

  services.nginx = {
    enable = true;
    recommendedGzipSettings = true;
    recommendedOptimisation = true;
    recommendedProxySettings = true;

    virtualHosts."dplaneos" = {
      default = true;
      root = "${dplaneos-frontend}";

      extraConfig = ''
        add_header X-Frame-Options "SAMEORIGIN" always;
        add_header X-Content-Type-Options "nosniff" always;
        add_header X-XSS-Protection "1; mode=block" always;
        add_header Referrer-Policy "no-referrer-when-downgrade" always;
      '';

      locations = {
        "~* \\.(css|js|jpg|jpeg|png|gif|ico|svg|woff|woff2|ttf|eot)$" = {
          extraConfig = "access_log off; expires 30d; add_header Cache-Control \"public, immutable\";";
        };
        "/" = { tryFiles = "$uri $uri/ /pages/index.html"; };
        "/api/" = {
          proxyPass = "http://127.0.0.1:9000";
          proxyWebsockets = false;
          extraConfig = "proxy_read_timeout 120s; proxy_connect_timeout 10s;";
        };
        "/ws/" = {
          proxyPass = "http://127.0.0.1:9000";
          proxyWebsockets = true;
          extraConfig = "proxy_read_timeout 86400s;";
        };
        "/health" = { proxyPass = "http://127.0.0.1:9000/health"; };
        "~ \\.php$" = { extraConfig = "deny all;"; };
        "~ /\\." = { extraConfig = "deny all;"; };
        "~ /(config|daemon|scripts|systemd)/" = { extraConfig = "deny all;"; };
      };
    };
  };

  # ═══════════════════════════════════════════════════════════
  #  D-PLANEOS DAEMON
  # ═══════════════════════════════════════════════════════════

  systemd.services.dplaned = {
    description = "D-PlaneOS System Daemon";
    after = [ "network.target" "zfs-import.target" "zfs-mount.service" ];
    wants = [ "zfs-import.target" ];
    requires = [ "zfs-mount.service" ];
    wantedBy = [ "multi-user.target" ];

    serviceConfig = {
      Type = "simple";
      ExecStart = lib.concatStringsSep " " [
        "${dplaned}/bin/dplaned"
        "-db /var/lib/dplaneos/dplaneos.db"
        "-listen 127.0.0.1:9000"
        "-config-dir /var/lib/dplaneos/config"
        "-smb-conf /var/lib/dplaneos/smb-shares.conf"
      ];
      WorkingDirectory = "/var/lib/dplaneos";
      Restart = "always";
      RestartSec = 5;
      User = "root";
      Group = "root";
      RuntimeDirectory = "dplaneos";
      RuntimeDirectoryMode = "0755";
      StateDirectory = "dplaneos";
      LogsDirectory = "dplaneos";
      NoNewPrivileges = true;
      PrivateTmp = true;
      ProtectSystem = "strict";
      ProtectHome = true;
      ReadWritePaths = [ "/var/log/dplaneos" "/var/lib/dplaneos" "/run/dplaneos" ];
      AmbientCapabilities = [ "CAP_SYS_ADMIN" "CAP_NET_ADMIN" "CAP_DAC_READ_SEARCH" "CAP_CHOWN" "CAP_FOWNER" ];
      LimitNOFILE = 65536;
      TasksMax = 4096;
      MemoryMax = "1G";
      MemoryHigh = "768M";
      OOMScoreAdjust = -900;
    };
  };

  systemd.tmpfiles.rules = [
    "d /var/lib/dplaneos 0750 root root -"
    "d /var/lib/dplaneos/backups 0750 root root -"
    "d /var/lib/dplaneos/config 0750 root root -"
    "d /var/lib/dplaneos/config/ssl 0700 root root -"
    "f /var/lib/dplaneos/smb-shares.conf 0644 root root -"
  ];

  # ═══════════════════════════════════════════════════════════
  #  SAMBA
  # ═══════════════════════════════════════════════════════════

  services.samba = {
    enable = true;
    openFirewall = true;

    settings.global = {
      workgroup = sambaWorkgroup;
      "server string" = "D-PlaneOS NAS";
      security = "user";
      "map to guest" = "Bad User";
      "log file" = "/var/log/samba/log.%m";
      "max log size" = "1000";
      "socket options" = "TCP_NODELAY IPTOS_LOWDELAY SO_RCVBUF=131072 SO_SNDBUF=131072";
      "read raw" = "yes";
      "write raw" = "yes";
      "use sendfile" = "yes";
      "aio read size" = "16384";
      "aio write size" = "16384";
    };

    extraConfig = "include = /var/lib/dplaneos/smb-shares.conf";
  };

  # ═══════════════════════════════════════════════════════════
  #  NFS + DOCKER
  # ═══════════════════════════════════════════════════════════

  services.nfs.server.enable = true;

  virtualisation.docker = {
    enable = true;
    storageDriver = "zfs";
    autoPrune = { enable = true; dates = "weekly"; };
    daemon.settings = {
      "log-driver" = "json-file";
      "log-opts" = { "max-size" = "10m"; "max-file" = "3"; };
      "default-address-pools" = [ { base = "172.17.0.0/16"; size = 24; } ];
    };
  };

  # ═══════════════════════════════════════════════════════════
  #  NETZWERK
  # ═══════════════════════════════════════════════════════════

  networking.firewall = {
    enable = true;
    allowedTCPPorts = [ 80 443 445 2049 ];
    allowedUDPPorts = [ 5353 ];
  };

  services.avahi = {
    enable = true;
    nssmdns4 = true;
    publish = { enable = true; addresses = true; workstation = true; };
  };

  # ═══════════════════════════════════════════════════════════
  #  MONITORING + TOOLS
  # ═══════════════════════════════════════════════════════════

  services.smartd = {
    enable = true;
    autodetect = true;
    notifications.wall.enable = true;
  };

  environment.systemPackages = with pkgs; [
    dplaned dplaneos-recovery
    zfs smartmontools hdparm lsof
    ethtool iperf3
    htop tmux git sqlite docker-compose
    nano
  ];

  # ═══════════════════════════════════════════════════════════
  #  SSH + BENUTZER
  # ═══════════════════════════════════════════════════════════

  services.openssh = {
    enable = true;
    settings = {
      PermitRootLogin = "yes";
      PasswordAuthentication = true;
    };
  };

  users.users.admin = {
    isNormalUser = true;
    extraGroups = [ "wheel" "docker" ];
  };

  security.sudo.wheelNeedsPassword = true;

  # ═══════════════════════════════════════════════════════════
  #  BACKUPS
  # ═══════════════════════════════════════════════════════════

  services.cron = {
    enable = true;
    systemCronJobs = [
      "0 3 * * *  root  ${pkgs.sqlite}/bin/sqlite3 /var/lib/dplaneos/dplaneos.db \".backup /var/lib/dplaneos/backups/dplaneos-$(date +\\%Y\\%m\\%d).db\" && find /var/lib/dplaneos/backups -mtime +30 -delete"
    ];
  };
}
