{
  description = "D-PlaneOS v3.0.0 — NAS Operating System";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-25.11";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    let
      # ─── NixOS module (imported in configuration.nix) ─────────────────
      nixosModule = { config, lib, pkgs, ... }: {
        imports = [ self.nixosModules.dplaneos ];
      };

    in {
      # ─── NixOS module ─────────────────────────────────────────────────
      nixosModules.dplaneos = import ./module.nix;

      # ─── NixOS system configuration ───────────────────────────────────
      # Used by: nixos-rebuild switch --flake /etc/nixos#dplaneos
      nixosConfigurations.dplaneos = nixpkgs.lib.nixosSystem {
        system = "x86_64-linux";
        modules = [
          ./configuration-standalone.nix
          self.nixosModules.dplaneos
          {
            # Allow the PolyForm-licensed D-PlaneOS daemon.
            # NixOS classifies any non-OSI-approved license as "unfree".
            # PolyForm Shield 1.0.0 is source-available but restricts
            # commercial competition — hence this flag is required.
            nixpkgs.config.allowUnfreePredicate = pkg:
              builtins.elem (nixpkgs.lib.getName pkg) [
                "dplaneos-daemon"
              ];
          }
        ];
      };

      # ─── Package: dplaneos-daemon ─────────────────────────────────────
      packages.x86_64-linux.dplaneos-daemon =
        let pkgs = nixpkgs.legacyPackages.x86_64-linux;
        in pkgs.buildGoModule {
          pname = "dplaneos-daemon";
          version = "3.0.0";

          src = ../.;  # root of the D-PlaneOS repository

          # go-sqlite3 requires CGo; mattn/go-sqlite3 bundles libsqlite3.
          # CGo must be enabled.
          CGO_ENABLED = "1";

          # Vendor directory is already present in the repo (go mod vendor).
          # Use it directly — no network access needed during build.
          vendorHash = null;  # vendored: set to null to use vendor/

          subPackages = [ "daemon/cmd/dplaned" ];

          # go-sqlite3 needs gcc
          nativeBuildInputs = with pkgs; [ gcc ];

          ldflags = [
            "-s" "-w"
            "-X main.Version=3.0.0"
          ];

          meta = with nixpkgs.lib; {
            description = "D-PlaneOS NAS daemon (Go backend)";
            homepage    = "https://github.com/4nonX/D-PlaneOS";
            license     = licenses.unfree;  # PolyForm Shield 1.0.0
            maintainers = [];
            platforms   = [ "x86_64-linux" "aarch64-linux" ];
          };
        };

      # Default package
      packages.x86_64-linux.default =
        self.packages.x86_64-linux.dplaneos-daemon;
    };
}
