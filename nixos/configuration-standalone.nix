# ═══════════════════════════════════════════════════════════════
#  D-PlaneOS v2.0.0 — NixOS Konfiguration
# ═══════════════════════════════════════════════════════════════
#
#  Diese EINE Datei = dein komplettes NAS.
#
#  Anleitung: Lies NIXOS-INSTALL-GUIDE.md
#
#  Kurzversion:
#    1. NixOS installieren (Minimal ISO)
#    2. Diese Datei nach /etc/nixos/configuration.nix kopieren
#    3. Die 5 Stellen mit "HIER ÄNDERN" anpassen
#       (Ctrl+W in nano zum Suchen)
#    4. sudo nixos-rebuild switch
#    5. Browser öffnen → http://<server-ip>
#
#  Kaputt? → sudo nixos-rebuild switch --rollback
#
# ═══════════════════════════════════════════════════════════════

{ config, pkgs, lib, ... }:

let

  # ┌─────────────────────────────────────────────────────────┐
  # │  HIER ÄNDERN (1/5): Dein ZFS Pool-Name                 │
  # │                                                         │
  # │  Wie heißt dein ZFS-Pool? Wenn du noch keinen hast,    │
  # │  erstelle einen nach der Installation (siehe Anleitung) │
  # │  und trage den Namen hier ein.                          │
  # │                                                         │
  # │  Beispiele: "tank", "datapool", "nas"                   │
  # └─────────────────────────────────────────────────────────┘
  zpools = [ "tank" ];

  # Samba Arbeitsgruppe (Standard "WORKGROUP" passt fast immer)
  sambaWorkgroup = "WORKGROUP";

  # ─── D-PLANEOS PACKAGES (nicht ändern) ───────────────────

  # Go daemon (dplaned)
  dplaned = pkgs.buildGoModule rec {
    pname = "dplaned";
    version = "2.0.0";

    src = pkgs.fetchFromGitHub {
      owner = "4nonX";
      repo = "dplaneos";
      rev = "v${version}";
      # Diesen Hash bekommst du mit:
      #   nix-shell -p nix-prefetch-github --run "nix-prefetch-github 4nonX dplaneos --rev v2.0.0"
      # Den ausgegebenen sha256-Wert hier eintragen:
      hash = "sha256-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=";
    };

    sourceRoot = "${src.name}/daemon";

    # Diesen Hash bekommst du automatisch: Setze ihn auf "" (leerer String),
    # dann zeigt dir "nixos-rebuild switch" den richtigen Hash in der Fehlermeldung.
    vendorHash = "";

    CGO_ENABLED = 1;
    buildInputs = [ pkgs.sqlite ];

    meta = with lib; {
      description = "D-PlaneOS NAS System Daemon";
      homepage = "https://github.com/4nonX/dplaneos";
      license = licenses.mit;
    };
  };

  # Frontend (statische Webseite)
  dplaneos-frontend = pkgs.stdenv.mkDerivation rec {
    pname = "dplaneos-frontend";
    version = "2.0.0";

    src = pkgs.fetchFromGitHub {
      owner = "4nonX";
      repo = "dplaneos";
      rev = "v${version}";
      # Gleicher Hash wie oben:
      hash = "sha256-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=";
    };

    installPhase = ''
      mkdir -p $out
      cp -r app/* $out/
    '';
  };

  # Notfall-Tool für Passwort-Reset etc.
  dplaneos-recovery = pkgs.writeShellScriptBin "dplaneos-recovery" ''
    DB="/var/lib/dplaneos/dplaneos.db"
    echo "D-PlaneOS Recovery CLI v2.0.0"
    echo ""
    echo "1) Admin-Passwort zurücksetzen"
    echo "2) Systemstatus anzeigen"
    echo "3) Datenbank-Backup erstellen"
    echo "4) Beenden"
    read -p "Auswahl: " choice
    case $choice in
      1)
        read -sp "Neues Admin-Passwort: " pw; echo
        HASH=$(${pkgs.apacheHttpd}/bin/htpasswd -nbBC 10 "" "$pw" | cut -d: -f2)
        ${pkgs.sqlite}/bin/sqlite3 "$DB" "UPDATE users SET password_hash='$HASH' WHERE username='admin';"
        echo "Admin-Passwort zurückgesetzt."
        ;;
      2)
        echo "Datenbank: $(du -h $DB)"
        echo "Sessions: $(${pkgs.sqlite}/bin/sqlite3 "$DB" "SELECT COUNT(*) FROM sessions;")"
        echo "Benutzer: $(${pkgs.sqlite}/bin/sqlite3 "$DB" "SELECT COUNT(*) FROM users;")"
        ;;
      3)
        BACKUP="/var/lib/dplaneos/backups/dplaneos-$(date +%Y%m%d-%H%M%S).db"
        ${pkgs.sqlite}/bin/sqlite3 "$DB" ".backup $BACKUP"
        echo "Backup gespeichert: $BACKUP"
        ;;
      *) echo "Tschüss." ;;
    esac
  '';

in {

  # Hardware-Erkennung (von NixOS automatisch generiert — NICHT löschen!)
  imports = [
    ./hardware-configuration.nix
  ];

  # ═════════════════════════════════════════════════════════════
  #  SYSTEM
  # ═════════════════════════════════════════════════════════════

  system.stateVersion = "24.11";

  networking.hostName = "dplaneos";

  # ┌─────────────────────────────────────────────────────────┐
  # │  HIER ÄNDERN (2/5): Host-ID für ZFS                    │
  # │                                                         │
  # │  Generiere eine mit diesem Befehl:                      │
  # │    head -c4 /dev/urandom | od -A none -t x4 | tr -d ' '│
  # │                                                         │
  # │  Beispiel-Ausgabe: a8f3b2c1                             │
  # │  Diese 8 Zeichen unten eintragen:                       │
  # └─────────────────────────────────────────────────────────┘
  networking.hostId = "HIER_AENDERN";

  # ┌─────────────────────────────────────────────────────────┐
  # │  HIER ÄNDERN (3/5): Zeitzone                            │
  # │                                                         │
  # │  Deutschland: "Europe/Berlin"                           │
  # │  Österreich:  "Europe/Vienna"                           │
  # │  Schweiz:     "Europe/Zurich"                           │
  # └─────────────────────────────────────────────────────────┘
  time.timeZone = "Europe/Berlin";

  i18n.defaultLocale = "en_US.UTF-8";

  # ═════════════════════════════════════════════════════════════
  #  BOOTLOADER
  # ═════════════════════════════════════════════════════════════

  # ┌─────────────────────────────────────────────────────────┐
  # │  HIER ÄNDERN (4/5): Bootloader                          │
  # │                                                         │
  # │  OPTION A — UEFI (fast alle PCs/Server seit ~2012):     │
  # │    → Die nächsten 2 Zeilen so lassen.                   │
  # │                                                         │
  # │  OPTION B — Altes BIOS/MBR:                             │
  # │    → Die 2 Zeilen unter "Option A" auskommentieren      │
  # │      (# davor setzen)                                   │
  # │    → Die 2 Zeilen unter "Option B" einkommentieren      │
  # │      (# entfernen)                                      │
  # └─────────────────────────────────────────────────────────┘

  # Option A: UEFI (der Normalfall)
  boot.loader.systemd-boot.enable = true;
  boot.loader.efi.canTouchEfiVariables = true;

  # Option B: Altes BIOS/MBR
  # boot.loader.grub.enable = true;
  # boot.loader.grub.device = "/dev/sda";

  # ═════════════════════════════════════════════════════════════
  #  ZFS (Dateisystem für deine Daten)
  # ═════════════════════════════════════════════════════════════

  boot.supportedFilesystems = [ "zfs" ];
  boot.zfs.forceImportRoot = false;
  boot.zfs.extraPools = zpools;

  # Prüft monatlich die Datenintegrität
  services.zfs.autoScrub = {
    enable = true;
    interval = "monthly";
  };

  # Automatische Snapshots — wie eine Zeitmaschine für deine Daten!
  services.zfs.autoSnapshot = {
    enable = true;
    frequent = 4;    # Alle 15 Min, behalte 4
    hourly = 24;     # Stündlich, behalte 24
    daily = 7;       # Täglich, behalte 7
    weekly = 4;      # Wöchentlich, behalte 4
    monthly = 12;    # Monatlich, behalte 12
  };

  # ZFS Speicher-Cache (ARC)
  # Faustregel: 1 GB pro 1 TB Speicher, mindestens 4 GB
  #   16 GB RAM → 8 GB ARC (8589934592)
  #   32 GB RAM → 16 GB ARC (17179869184)
  #   64 GB RAM → 32 GB ARC (34359738368)
  boot.kernelParams = [
    "zfs.zfs_arc_max=17179869184"
  ];

  # ═════════════════════════════════════════════════════════════
  #  KERNEL TUNING (für NAS-Betrieb optimiert — nicht ändern)
  # ═════════════════════════════════════════════════════════════

  boot.kernel.sysctl = {
    "fs.inotify.max_user_watches" = 524288;
    "fs.inotify.max_user_instances" = 512;
    "vm.swappiness" = 10;
    "vm.vfs_cache_pressure" = 50;
    "net.core.rmem_max" = 16777216;
    "net.core.wmem_max" = 16777216;
  };

  # ═════════════════════════════════════════════════════════════
  #  WEBSERVER (zeigt die D-PlaneOS Oberfläche im Browser)
  # ═════════════════════════════════════════════════════════════

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

  # ═════════════════════════════════════════════════════════════
  #  D-PLANEOS DAEMON (das Herzstück — nicht ändern)
  # ═════════════════════════════════════════════════════════════

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

  # ═════════════════════════════════════════════════════════════
  #  SAMBA (Windows-Dateifreigaben)
  # ═════════════════════════════════════════════════════════════

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

    # Shares werden vom D-PlaneOS Daemon verwaltet
    extraConfig = "include = /var/lib/dplaneos/smb-shares.conf";
  };

  # ═════════════════════════════════════════════════════════════
  #  NFS (Linux/Mac-Dateifreigaben)
  # ═════════════════════════════════════════════════════════════

  services.nfs.server.enable = true;

  # ═════════════════════════════════════════════════════════════
  #  DOCKER (Container-Apps wie Plex, Nextcloud, Jellyfin...)
  # ═════════════════════════════════════════════════════════════

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

  # ═════════════════════════════════════════════════════════════
  #  NETZWERK & FIREWALL
  # ═════════════════════════════════════════════════════════════

  networking.firewall = {
    enable = true;
    allowedTCPPorts = [ 80 443 445 2049 ];  # HTTP, HTTPS, SMB, NFS
    allowedUDPPorts = [ 5353 ];              # mDNS
  };

  # NAS im Netzwerk als "dplaneos.local" sichtbar
  services.avahi = {
    enable = true;
    nssmdns4 = true;
    publish = { enable = true; addresses = true; workstation = true; };
  };

  # ═════════════════════════════════════════════════════════════
  #  FESTPLATTEN-ÜBERWACHUNG (S.M.A.R.T.)
  # ═════════════════════════════════════════════════════════════

  services.smartd = {
    enable = true;
    autodetect = true;
    notifications.wall.enable = true;
  };

  # ═════════════════════════════════════════════════════════════
  #  INSTALLIERTE PROGRAMME
  # ═════════════════════════════════════════════════════════════

  environment.systemPackages = with pkgs; [
    dplaned dplaneos-recovery
    zfs smartmontools hdparm lsof
    ethtool iperf3
    htop tmux git sqlite docker-compose
    nano
  ];

  # ═════════════════════════════════════════════════════════════
  #  SSH (Fernzugriff per Terminal)
  # ═════════════════════════════════════════════════════════════

  services.openssh = {
    enable = true;
    settings = {
      PermitRootLogin = "yes";
      PasswordAuthentication = true;
      # Tipp: Nach der Einrichtung auf SSH-Keys umstellen (sicherer):
      # PermitRootLogin = "prohibit-password";
      # PasswordAuthentication = false;
    };
  };

  # ═════════════════════════════════════════════════════════════
  #  BENUTZER
  # ═════════════════════════════════════════════════════════════

  # ┌─────────────────────────────────────────────────────────┐
  # │  HIER ÄNDERN (5/5): Admin-Benutzer                      │
  # │                                                         │
  # │  Passwort nach Installation setzen mit:                 │
  # │    sudo passwd admin                                    │
  # └─────────────────────────────────────────────────────────┘
  users.users.admin = {
    isNormalUser = true;
    extraGroups = [ "wheel" "docker" ];
  };

  # sudo braucht Passwort (sicherer)
  # Tipp: Falls du dich aussperrst → beim Booten im GRUB-Menü
  #        eine ältere Generation wählen, oder: dplaneos-recovery
  security.sudo.wheelNeedsPassword = true;

  # ═════════════════════════════════════════════════════════════
  #  AUTOMATISCHE AUFGABEN
  # ═════════════════════════════════════════════════════════════

  services.cron = {
    enable = true;
    systemCronJobs = [
      # Datenbank-Backup: jeden Tag um 3 Uhr, behalte 30 Tage
      # Nutzt sqlite3 .backup statt cp — sicher auch bei laufendem Daemon (WAL-Mode)
      "0 3 * * *  root  ${pkgs.sqlite}/bin/sqlite3 /var/lib/dplaneos/dplaneos.db \".backup /var/lib/dplaneos/backups/dplaneos-$(date +\\%Y\\%m\\%d).db\" && find /var/lib/dplaneos/backups -mtime +30 -delete"
    ];
  };
}
