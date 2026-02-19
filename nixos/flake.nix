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
