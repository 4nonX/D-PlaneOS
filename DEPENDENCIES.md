# D-PlaneOS v2.2.0 — Dependencies

**Complete dependency manifest and system requirements for the Declarative Storage Engine.**

---

## ✅ All Internal Dependencies Included

The distribution package is self-contained. It includes all PHP logic, Material Design 3 assets, and the internal Go-daemon bridge. No external framework installation (like Laravel or React) is required.

---

## 📦 Internal Dependencies (IN-PACKAGE)

### PHP Includes (22 Files)
Located in: `app/includes/`

**Core Engine Logic:**
- ✅ `config.php` - v2.2.0 System Environment Settings
- ✅ `auth.php` - Session & JWT Authentication Engine
- ✅ `rbac.php` - v2.2.0 Role-Based Access Control (34+ Permissions)
- ✅ `security.php` - Security Hardening & Audit Logger
- ✅ `security-middleware.php` - Input Sanitization & CSRF Protection
- ✅ `functions.php` - Global Helper Functions

**Database & Persistence:**
- ✅ `db.php` - Database Abstraction (SQLite/PostgreSQL)
- ✅ `db-factory.php` - v2.2.0 Database Connection Factory

**Feature Modules:**
- ✅ `permissions.php` - Permission Bitmask Logic
- ✅ `encryption.php` - AES-256-GCM Encryption Library
- ✅ `totp.php` - MFA/Two-Factor Authentication
- ✅ `password_reset.php` - Secure Recovery Logic
- ✅ `external-auth.php` - v2.2.0 LDAP/AD Directory Logic

**System Integration:**
- ✅ `daemon-client.php` - v2.2.0 Go-Daemon Communication (Unix Sockets)
- ✅ `router.php` - Request Router
- ✅ `command.php` - Hardened Command Execution
- ✅ `zfs_helper.php` - ZFS-Native Helper Functions
- ✅ `module_manager.php` - Plugin & Module Management
- ✅ `nut-monitor.php` - UPS Monitoring (NUT Integration)

**UI Navigation:**
- ✅ `navigation.html` - Base Navigation Template
- ✅ `nav-production.html` - Hardened Production Navigation

### CSS Assets (7 Files)
Located in: `app/assets/css/`

- ✅ `m3-tokens.css` - Material Design 3 Design Tokens
- ✅ `m3-components.css` - Material Components (v2.2.0 Optimized)
- ✅ `m3-animations.css` - UI Transitions
- ✅ `m3-icons.css` - Layout Icon Positioning
- ✅ `design-tokens.css` - Custom Theme Overrides
- ✅ `ui-components.css` - Base Component Styles
- ✅ `enhanced-ui.css` - Specialized UI Polish for v2.2.0

### JavaScript Assets (10 Files)
Located in: `app/assets/js/`

- ✅ `core.js` - Main Engine Logic
- ✅ `ui-components.js` - UI Interaction Layer
- ✅ `enhanced-ui.js` - Advanced Interface Features
- ✅ `form-validator.js` - Real-time Regex Validation
- ✅ `connection-monitor.js` - Daemon Connectivity Heartbeat
- ✅ `keyboard-shortcuts.js` - Power-user Controls
- ✅ `theme-engine.js` - Dynamic Light/Dark/Auto Switching
- ✅ `realtime-client.js` - v2.2.0 Real-time Update Engine
- ✅ `m3-ripple.js` - Material Ripple Interactions
- ✅ `ui-components-old.js` - Legacy Compatibility Layer

---

## 🌐 External Dependencies (CDN)

### Material Symbols Icons
Used throughout the v2.2.0 UI for high-fidelity iconography.

**CDN Reference:**
```html
<link rel="stylesheet" href="[https://fonts.googleapis.com/css2?family=Material+Symbols+Rounded:opsz,wght,FILL,GRAD@20..48,100..700,0..1,-50..200](https://fonts.googleapis.com/css2?family=Material+Symbols+Rounded:opsz,wght,FILL,GRAD@20..48,100..700,0..1,-50..200)">
```
*Note: In air-gapped environments, icons will fall back to text labels.*

---

## 🐧 System Dependencies (External)

### Base Requirements (v2.2.0 Standard)

**Operating Systems:**
- **NixOS 23.11+ / 24.05+** (Required for Flake/Declarative features)
- Ubuntu 22.04+ / Debian 12+
- RHEL 9+ / Rocky Linux 9+

**Environment:**
```bash
# Ubuntu/Debian Standard
sudo apt install apache2 php8.1 libapache2-mod-php php8.1-{cli,fpm,mbstring,xml,zip,pdo,sqlite3,curl}

# NixOS (flake.nix snippet)
services.httpd.enable = true;
services.phpfpm.pools.dplane = { ... };
```

### Binary Dependencies (v2.2.0 Engine)

**ZFS & Storage:**
- `zfsutils-linux` / `zfs` binary must be accessible in system PATH.
- ZFS Kernel module must be loaded.

**GitOps State Sync:**
- `git` binary (Used by the Go-daemon for state mirroring).
- `ssh-agent` or configured SSH keys for repository access.

**Docker Management:**
- Docker Engine 24.0+.
- Web-user (e.g., `www-data` or `apache`) must be in the `docker` group.

**Hardware Monitoring:**
- `ipmitool` (for IPMI sensor data).
- `nut` (for UPS monitoring).

---

## 📋 Installation Command Overview

### Debian/Ubuntu Full Installation

```bash
# 1. Base Environment
sudo apt update
sudo apt install -y apache2 php php-{cli,fpm,mbstring,xml,zip,pdo,sqlite3,curl,json}

# 2. Storage, Containers, & State Sync
sudo apt install -y zfsutils-linux docker.io git

# 3. Permissions
sudo usermod -aG docker www-data
```

---

## 📊 Dependency Matrix

| Component | Category | Required | Provided In-Package |
|:--- |:--- |:--- |:--- |
| **D-Plane Engine (PHP)** | Application | Yes | **Yes** |
| **Go-Daemon Bridge** | Middleware | Yes | **Yes** |
| **UI Assets (M3)** | Frontend | Yes | **Yes** |
| **ZFS Utils** | Kernel/Binary | Yes | No (Host OS) |
| **Git Binary** | State Sync | Optional | No (Host OS) |
| **Docker Engine** | Service | Optional | No (Host OS) |

---

## ✅ v2.2.0 Environment Verification

Run this check on your host:

```bash
#!/bin/bash
echo "=== D-PlaneOS v2.2.0 Environment Check ==="

# Check Go Daemon Connectivity Path
[ -S /var/run/dplane/dplaned.sock ] && echo "✓ Go Daemon Socket Active" || echo "⚠ Go Daemon Socket Missing"

# Check PHP
php -m | grep -qi pdo_sqlite && echo "✓ SQLite PDO Active" || echo "✗ SQLite PDO Missing"

# Check GitOps Readiness
git --version >/dev/null 2>&1 && echo "✓ Git Binary Ready for Sync" || echo "⚠ Git Missing"
```

---

**Final Note:** v2.2.0-HARDENED is designed for deterministic environments. Ensure your host system meets these binary requirements to enable the full GitOps and NixOS feature set.
