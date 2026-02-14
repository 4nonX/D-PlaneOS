# ═══════════════════════════════════════════════════════════════
#  D-PlaneOS v2.0.0 — NixOS Flake
# ═══════════════════════════════════════════════════════════════
#
#  Dein komplettes NAS als Git-Repo.
#
#  Erstinstallation:
#    git clone https://github.com/4nonX/dplaneos-nixos
#    cd dplaneos-nixos
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

  # ─── Inputs (Abhängigkeiten) ─────────────────────────────

  inputs = {
    # NixOS 24.11 (stable)
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-24.11";

    # D-PlaneOS Quellcode
    dplaneos-src = {
      url = "github:4nonX/dplaneos/v2.0.0";
      flake = false;  # Kein Flake, nur Quellcode
    };
  };

  # ─── Outputs ─────────────────────────────────────────────

  outputs = { self, nixpkgs, dplaneos-src, ... }:
  let
    system = "x86_64-linux";
    pkgs = import nixpkgs {
      inherit system;
      config.allowUnfree = false;
    };

    # ─── D-PlaneOS Packages ──────────────────────────────

    # Go daemon
    dplaned = pkgs.buildGoModule {
      pname = "dplaned";
      version = "2.0.0";
      src = dplaneos-src;
      sourceRoot = "source/daemon";

      # Go dependencies hash
      # Beim ersten Build: auf null setzen → Fehlermeldung zeigt korrekten Hash
      vendorHash = null;

      # SQLite braucht CGO
      CGO_ENABLED = 1;
      nativeBuildInputs = [ pkgs.pkg-config ];
      buildInputs = [ pkgs.sqlite ];

      meta = with pkgs.lib; {
        description = "D-PlaneOS NAS System Daemon";
        homepage = "https://github.com/4nonX/dplaneos";
        license = licenses.mit;
        platforms = platforms.linux;
      };
    };

    # Frontend (statische Dateien)
    dplaneos-frontend = pkgs.stdenv.mkDerivation {
      pname = "dplaneos-frontend";
      version = "2.0.0";
      src = dplaneos-src;

      installPhase = ''
        mkdir -p $out
        cp -r app/* $out/
      '';
    };

    # Recovery CLI
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

    # ─── NixOS System-Konfiguration ───────────────────────

    nixosConfigurations.dplaneos = nixpkgs.lib.nixosSystem {
      inherit system;

      # Packages als Argumente an die Module weiterreichen
      specialArgs = {
        inherit dplaned dplaneos-frontend dplaneos-recovery;
      };

      modules = [
        # Hardware-Erkennung (auf dem Zielrechner generiert)
        /etc/nixos/hardware-configuration.nix

        # D-PlaneOS NAS Konfiguration
        ./configuration.nix
      ];
    };

    # ─── Standalone Packages (optional, zum Testen) ──────

    packages.${system} = {
      inherit dplaned dplaneos-frontend dplaneos-recovery;
      default = dplaned;
    };

    # ─── Dev-Shell (für Entwicklung am Daemon) ───────────

    devShells.${system}.default = pkgs.mkShell {
      buildInputs = with pkgs; [
        go
        gopls
        sqlite
        pkg-config
      ];
      shellHook = ''
        echo "D-PlaneOS Entwicklungsumgebung"
        echo "  go build ./cmd/dplaned/"
        echo "  go test ./..."
      '';
    };
  };
}
