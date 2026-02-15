# D-PlaneOS v2.1.1-HARDENED - Dependencies

**Full List of Required Dependencies**

---

## ✅ Most dependencies are included.

The package includes **ALL** required PHP-Includes, CSS, JavaScript and Configuration Files.

---

## 📦 Internal Dependencies (IN THE PACKAGE)

### PHP Includes (22 Files)

Alle in: `app/includes/`

**Core:**
- ✅ `config.php` - System-Configuration
- ✅ `auth.php` - Authentication
- ✅ `rbac.php` - Role-Based Access Control
- ✅ `security.php` - Security-Functions & Logger-Class
- ✅ `security-middleware.php` - Security Middleware
- ✅ `functions.php` - Helper-Functions

**Database:**
- ✅ `db.php` - Database Abstraction
- ✅ `db-factory.php` - Database Factory (SQLite/PostgreSQL)

**Features:**
- ✅ `permissions.php` - Permission Management
- ✅ `encryption.php` - Encryption Library
- ✅ `totp.php` - Two-Factor Authentication
- ✅ `password_reset.php` - Password Reset Logic
- ✅ `external-auth.php` - External Auth (LDAP, etc.)

**System:**
- ✅ `daemon-client.php` - Go Daemon Communication
- ✅ `router.php` - Request Router
- ✅ `command.php` - Command Execution
- ✅ `zfs_helper.php` - ZFS Helper Functions
- ✅ `module_manager.php` - Module Management
- ✅ `nut-monitor.php` - UPS Monitoring

**Navigation:**
- ✅ `navigation.html` - Main Navigation
- ✅ `nav-production.html` - Production Navigation

### CSS Assets (7 Dateien)

Alle in: `app/assets/css/`

**Material Design 3:**
- ✅ `m3-tokens.css` - Design Tokens
- ✅ `m3-components.css` - Material Components
- ✅ `m3-animations.css` - Animations
- ✅ `m3-icons.css` - Icon Styles
- ✅ `design-tokens.css` - Custom Design Tokens

**UI:**
- ✅ `ui-components.css` - UI Component Styles
- ✅ `enhanced-ui.css` - Enhanced UI Styles

### JavaScript Assets (10 Dateien)

Alle in: `app/assets/js/`

**Core:**
- ✅ `core.js` - Core Functions
- ✅ `ui-components.js` - UI Components
- ✅ `enhanced-ui.js` - Enhanced UI Features

**Features:**
- ✅ `form-validator.js` - Form Validation
- ✅ `connection-monitor.js` - Connection Monitoring
- ✅ `keyboard-shortcuts.js` - Keyboard Shortcuts
- ✅ `theme-engine.js` - Theme Management
- ✅ `realtime-client.js` - Real-time Updates

**Material Design:**
- ✅ `m3-ripple.js` - Material Ripple Effect

**Legacy:**
- ✅ `ui-components-old.js` - Old UI Components (backward compat)

### PWA Assets (2 Dateien)

- ✅ `app/sw.js` - Service Worker
- ✅ `app/manifest.json` - PWA Manifest

---

## 🌐 External Dependencies (CDN)

### Material Symbols Icons

**Used in:** IPMI UI, Cloud Sync UI

**CDN:**
```html
<link rel="stylesheet" href="https://fonts.googleapis.com/css2?family=Material+Symbols+Rounded:opsz,wght,FILL,GRAD@20..48,100..700,0..1,-50..200">
```

**Fallback:** System funktioniert ohne Icons, aber weniger schön

**Offline Alternative:**
```bash
# Icons lokal hosten (optional)
wget https://fonts.googleapis.com/css2?family=Material+Symbols+Rounded:opsz,wght,FILL,GRAD@20..48,100..700,0..1,-50..200 -O material-symbols.css
# In HTML ersetzen mit lokalem Pfad
```

---

## 🐧 System-Dependencies (to be installed from external sources)

### Required (Base-System)

**Linux:**
- Ubuntu 22.04+ / Debian 12+ / NixOS 25.11 (recommended)
- RHEL 8+ / Rocky Linux 8+ / AlmaLinux 8+
- Oder jede moderne Linux-Distribution

**Web Server:**
```bash
# Apache (empfohlen)
sudo apt install apache2 php libapache2-mod-php

# ODER Nginx
sudo apt install nginx php-fpm
```

**PHP:**
```bash
# PHP 8.0+ mit Extensions
sudo apt install php8.1 php8.1-{cli,fpm,mbstring,xml,zip,pdo,sqlite3,pgsql,curl,json}

# Prüfen
php -v
php -m | grep -E "pdo|mbstring|json|zip"
```

**Database:**
```bash
# SQLite (Standard, wird automatisch installiert mit PHP)
sudo apt install sqlite3 php8.1-sqlite3

# ODER PostgreSQL (optional)
sudo apt install postgresql postgresql-contrib php8.1-pgsql
```

### Optional (Features)

**ZFS Support:**
```bash
# Ubuntu/Debian
sudo apt install zfsutils-linux

# RHEL/Rocky/Alma
sudo dnf install zfs
sudo modprobe zfs
```

**Docker:**
```bash
# Official Docker Installation
curl -fsSL https://get.docker.com -o get-docker.sh
sudo sh get-docker.sh

# Add web user to docker group
sudo usermod -aG docker www-data
```

**IPMI Monitoring:**
```bash
# ipmitool für IPMI-Features
sudo apt install ipmitool    # Debian/Ubuntu
sudo yum install ipmitool    # RHEL/CentOS
```

**Cloud Sync:**
```bash
# rclone für Cloud-Sync-Features
curl https://rclone.org/install.sh | sudo bash
```

**Go Compiler (Daemon):**
```bash
# Go 1.19+ für Daemon-Compilation
wget https://go.dev/dl/go1.21.6.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.21.6.linux-amd64.tar.gz
export PATH=$PATH:/usr/local/go/bin
```

---

## 📋 Installation Command-Overview

### Debian/Ubuntu Minimal

```bash
# Basis-System
sudo apt update
sudo apt install -y apache2 php php-{cli,mbstring,xml,zip,pdo,sqlite3,curl,json}

# ZFS (für Storage)
sudo apt install -y zfsutils-linux

# Docker (für Container)
curl -fsSL https://get.docker.com -o get-docker.sh
sudo sh get-docker.sh
sudo usermod -aG docker www-data
```

### Debian/Ubuntu Complete

```bash
# Alles installieren
sudo apt update
sudo apt install -y \
    apache2 \
    php php-{cli,fpm,mbstring,xml,zip,pdo,sqlite3,pgsql,curl,json} \
    zfsutils-linux \
    sqlite3 \
    ipmitool

# Docker
curl -fsSL https://get.docker.com -o get-docker.sh
sudo sh get-docker.sh
sudo usermod -aG docker www-data

# rclone
curl https://rclone.org/install.sh | sudo bash

# Go (optional, für Daemon-Compilation)
wget https://go.dev/dl/go1.21.6.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.21.6.linux-amd64.tar.gz
```

### RHEL/Rocky/Alma Minimal

```bash
# Basis-System
sudo yum install -y httpd php php-{cli,mbstring,xml,pdo,mysqlnd,json}

# ZFS
sudo yum install -y zfs
sudo modprobe zfs

# Docker
sudo yum install -y docker
sudo systemctl enable --now docker
sudo usermod -aG docker apache
```

---

## ✅ Dependencies Check Script

**Erstelle:** `check-dependencies.sh`

```bash
#!/bin/bash
# D-PlaneOS v5.1-HARDENED - Dependency Checker

echo "=== D-PlaneOS Dependency Check ==="
echo ""

# PHP
echo -n "PHP: "
if command -v php &> /dev/null; then
    PHP_VERSION=$(php -v | head -1 | cut -d' ' -f2 | cut -d'.' -f1,2)
    echo "✓ Found ($PHP_VERSION)"
else
    echo "✗ Not found - REQUIRED"
fi

# PHP Extensions
echo "PHP Extensions:"
for ext in pdo mbstring json zip sqlite3; do
    echo -n "  $ext: "
    if php -m | grep -q "^$ext$"; then
        echo "✓"
    else
        echo "✗ Missing"
    fi
done

# Web Server
echo -n "Web Server: "
if systemctl is-active --quiet apache2 || systemctl is-active --quiet httpd; then
    echo "✓ Apache running"
elif systemctl is-active --quiet nginx; then
    echo "✓ Nginx running"
else
    echo "✗ No active web server"
fi

# ZFS
echo -n "ZFS: "
if command -v zpool &> /dev/null; then
    echo "✓ Installed"
else
    echo "⚠ Not installed (optional)"
fi

# Docker
echo -n "Docker: "
if command -v docker &> /dev/null; then
    echo "✓ Installed"
else
    echo "⚠ Not installed (optional)"
fi

# IPMI
echo -n "ipmitool: "
if command -v ipmitool &> /dev/null; then
    echo "✓ Installed"
else
    echo "⚠ Not installed (optional)"
fi

# rclone
echo -n "rclone: "
if command -v rclone &> /dev/null; then
    echo "✓ Installed"
else
    echo "⚠ Not installed (optional)"
fi

echo ""
echo "=== Summary ==="
echo "✓ = Installed/Working"
echo "✗ = Missing/Required"
echo "⚠ = Optional (feature-dependent)"
```

**Use:**
```bash
chmod +x check-dependencies.sh
./check-dependencies.sh
```

---

## 🔍 What is NOT INCLUDED in the Package

### Not included (has to be installed):

1. **Linux Kernel & OS** - Base-System
2. **PHP Interpreter** - sudo apt install php
3. **Web Server** - Apache or Nginx
4. **ZFS Kernel Module** - sudo apt install zfsutils-linux
5. **Docker Engine** - curl https://get.docker.com | sh
6. **ipmitool** - sudo apt install ipmitool
7. **rclone** - curl https://rclone.org/install.sh | bash
8. **Go Compiler** - wget https://go.dev/dl/...

### Why are these not included?

- **Linux/PHP/Web Server:** System-Level, via package manager
- **ZFS/Docker:** Kernel-Module & System-Services
- **ipmitool/rclone:** Optional, user-specific
- **Go:** Only required for Daemon-Compilation

---

## 📊 Dependency-Matrix

| Component | Depends on | Required | Included in the Package |
|------------|-------------|--------------|------------|
| **PHP Includes** | PHP | ✓ | ✓ |
| **CSS/JS Assets** | Web Server | ✓ | ✓ |
| **PWA** | Browser | ✓ | ✓ |
| **Material Icons** | Google CDN | ✗ | ✗ (extern) |
| **Storage Management** | ZFS | ✓ | ✗ (system) |
| **Docker Management** | Docker | ✓ | ✗ (system) |
| **IPMI Monitor** | ipmitool | ✗ | ✗ (optional) |
| **Cloud Sync** | rclone | ✗ | ✗ (optional) |
| **Go Daemon** | Go | ✗ | ✗ (optional) |

**Explanation:**
- ✓ In the Package = Datei enthalten
- ✗ External = Muss separat installiert werden
- ✓ Required = Required for Base-Functionality
- ✗ Optional = Only required for spezific Features

---

## 🎯 Summary

### ✅ Included in the Package:

- Alle PHP Includes (22 Dateien)
- Alle CSS Assets (7 Dateien)
- Alle JavaScript Assets (10 Dateien)
- PWA Support (sw.js, manifest.json)
- Alle UI Pages (22 HTML Dateien)
- Alle APIs (18 PHP Dateien)
- Installation Scripts
- Dokumentation

### 🌐 Externe Abhängigkeiten:

**CDN (Internet-Verbindung):**
- Material Symbols Icons (Google Fonts)

**System-Packages:**
- Linux OS
- PHP 8.0+
- Web Server (Apache/Nginx)
- SQLite oder PostgreSQL

**Optional (Features):**
- ZFS (Storage Management)
- Docker (Container Management)
- ipmitool (IPMI Monitoring)
- rclone (Cloud Sync)
- Go (Daemon Compilation)

---

## ✅ Order of Installation

```bash
# 1. Installing System-Dependencies
sudo apt install apache2 php php-{...} zfsutils-linux

# 2. Optional: Docker, ipmitool, rclone
sudo apt install docker.io ipmitool
curl https://rclone.org/install.sh | bash

# 3. D-PlaneOS installieren
tar -xzf dplaneos-v5.1-hardened-COMPLETE.tar.gz
cd dplaneos-v5.1-hardened-COMPLETE
sudo ./install.sh

# 4. Done!
```

---

**The Package is complete - all internal dependencies are included. Only System-Level-Tools hae to be  installied seperately (as with any other Webserver-Project).**
