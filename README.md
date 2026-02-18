# D-PlaneOS v2.2.0 — Declarative GitOps Storage Appliance

A high-performance, OS-agnostic NAS management engine built in **Go**. Featuring native **NixOS Flake** integration, **Bidirectional Git-Sync**, and **ZFS-native** data protection.

## ⚡ The v2.2.0 Shift: State as Code
Version 2.2.0 introduces **Deterministic Storage Management**. By decoupling the system logic from the host OS, D-PlaneOS ensures your NAS configuration is reproducible, version-controlled, and hardware-independent.

---

## Quick Start

### NixOS (Recommended)
Add D-PlaneOS to your `flake.nix`:

```nix
inputs.dplaneos.url = "github:your-repo/dplaneos/v2.2.0";
```

Then include the module in your system configuration:

```nix
outputs = { self, nixpkgs, dplaneos }: {
  nixosConfigurations.nas = nixpkgs.lib.nixosSystem {
    modules = [ dplaneos.nixosModules.dplaneos ];
  };
};
```

### Debian/Ubuntu

```bash
# Download the v2.2.0 production binary
tar xzf dplaneos-v2.2.0.tar.gz
cd dplaneos
sudo make install
sudo systemctl start dplaned
```

Web UI: `http://your-server` (Default: `admin` / `admin`)

---

## Core Features

### 🔄 Bidirectional Git-Sync (New)
Never lose a configuration again. D-PlaneOS automatically mirrors your system state—including Docker stacks, ZFS datasets, and permissions—to a private Git repository. Your NAS is now managed as "Cattle, not Pets."

### ❄️ Native NixOS Integration
A first-class Nix ecosystem citizen. Deploy your entire NAS infrastructure via Flakes. v2.2.0 includes the **NixOS Generation Manager**, allowing you to audit and rollback system generations directly from the web UI.

### 🛡️ ZFS-Powered Compute
- **Safe Container Updates:** Automatic ZFS snapshot → pull → health check. Instant rollback on failure.
- **Ephemeral Sandboxing:** Test containers on zero-cost ZFS clones.
- **Time Machine:** Browse snapshots as directories and restore single files without a full pool rollback.

### 🔍 Technical Excellence
- **OS-Agnostic Core:** Runs on NixOS, Debian, or Ubuntu with zero host contamination.
- **Hard Boot-Gate:** Systemd integration ensures the daemon only starts once ZFS pools are verified and writable.
- **Adaptive ARC Limiter:** Intelligent cache management optimized for both ECC and high-capacity non-ECC hardware.
- **Health Predictor:** Real-time S.M.A.R.T. integration and ZFS event tracking to catch disk failures before they happen.

---

## Architecture

- **Engine:** Statistically typed Go daemon (`dplaned`), single ~7.2MB binary.
- **State:** SQLite with WAL mode + Bidirectional Git-Sync.
- **Front-end:** Material Design 3, vanilla JS (no framework bloat), hybrid SPA router.
- **Security:** Strict regex-based input validation for all system commands, RBAC with 4 roles, and OOM-protection.
- **Storage:** Native ZFS kernel module integration with ZED (ZFS Event Daemon) alerting.

## Documentation

- `CHANGELOG.md` — Full version history and technical fixes.
- `ARCHITECTURE.md` — Deep dive into the Go/NixOS design philosophy.
- `ADMIN-GUIDE.md` — Comprehensive administration and setup guide.
- `nixos/README.md` — Detailed NixOS Flake configuration reference.
- `ERROR-REFERENCE.md` — API diagnostics and troubleshooting.

## License

Open source. See the LICENSE file for details.
