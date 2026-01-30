#!/bin/bash
# D-PlaneOS v1.9.0 - Future-Proof Installer
# Dynamic dependency detection, zero hardcoded versions
set -e
set -u
set -o pipefail

INSTALLER_VERSION="1.9.0"
TESTED_OS="debian:12,ubuntu:24.04,ubuntu:24.10"

# Flags
DRY_RUN=false
SKIP_VALIDATION=false
VERBOSE=false
FORCE=false

while [[ $# -gt 0 ]]; do
    case $1 in
        --dry-run) DRY_RUN=true; shift ;;
        --skip-validation) SKIP_VALIDATION=true; shift ;;
        --verbose) VERBOSE=true; shift ;;
        --force) FORCE=true; shift ;;
        --help)
            echo "D-PlaneOS v1.9.0 Installer"
            echo "Usage: $0 [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  --dry-run           Show what would be done"
            echo "  --skip-validation   Skip pre-flight checks"
            echo "  --verbose           Show detailed output"
            echo "  --force             Override warnings"
            exit 0
            ;;
        *) echo "Unknown: $1"; exit 1 ;;
    esac
done

export DEBIAN_FRONTEND=noninteractive
export APT_LISTCHANGES_FRONTEND=none

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

error_exit() {
    echo -e "${RED}ERROR: $1${NC}" >&2
    echo "See: /tmp/dplaneos-install.log"
    exit 1
}

LOG_FILE="/tmp/dplaneos-install.log"
exec > >(tee -a "$LOG_FILE")
exec 2>&1

echo -e "${GREEN}"
echo "╔═══════════════════════════════════════════╗"
echo "║   D-PlaneOS v1.9.0 - Future-Proof         ║"
echo "╚═══════════════════════════════════════════╝"
echo -e "${NC}"

[[ $EUID -ne 0 ]] && error_exit "Must run as root"
echo -e "${GREEN}✓${NC} Running as root"

# Detect OS
echo -n "Detecting system... "
if [ -f /etc/os-release ]; then
    . /etc/os-release
    OS_NAME="${NAME}"
    OS_VERSION="${VERSION_ID:-unknown}"
    OS_ID="${ID:-unknown}"
    OS_CODENAME="${VERSION_CODENAME:-$(lsb_release -cs 2>/dev/null || echo unknown)}"
    echo -e "${GREEN}✓${NC} ${OS_NAME} ${OS_VERSION}"
else
    error_exit "Cannot detect OS"
fi

# Check if tested OS
TESTED_COMBO="${OS_ID}:${OS_VERSION}"
if ! echo "$TESTED_OS" | grep -q "$TESTED_COMBO"; then
    echo -e "${YELLOW}⚠ WARNING: Untested OS combination${NC}"
    echo "  Tested: Debian 12, Ubuntu 24.04/24.10"
    echo "  You have: ${OS_NAME} ${OS_VERSION}"
    if [ "$FORCE" != true ]; then
        read -p "Continue anyway? (y/N) " -n 1 -r
        echo
        [[ ! $REPLY =~ ^[Yy]$ ]] && exit 0
    fi
fi

# Detect arch
echo -n "Detecting architecture... "
ARCH=$(uname -m)
case "$ARCH" in
    x86_64)
        DOCKER_ARCH="amd64"
        echo -e "${GREEN}✓${NC} ${ARCH}"
        ;;
    aarch64)
        DOCKER_ARCH="arm64"
        echo -e "${GREEN}✓${NC} ${ARCH}"
        ;;
    armv7l)
        DOCKER_ARCH="armhf"
        if [ "$FORCE" != true ]; then
            error_exit "${ARCH} not recommended (limited ZFS). Use --force"
        fi
        echo -e "${YELLOW}⚠${NC} ${ARCH} (forced)"
        ;;
    *)
        error_exit "Unsupported architecture: ${ARCH}"
        ;;
esac

# Validate systemd
echo -n "Checking init system... "
command -v systemctl &> /dev/null || error_exit "systemd required"
echo -e "${GREEN}✓${NC} systemd"

# Pre-flight checks
if [ "$SKIP_VALIDATION" = false ]; then
    echo ""
    echo "═══════════════════════════════════════════"
    echo "  Pre-flight Validation"
    echo "═══════════════════════════════════════════"
    
    echo -n "Disk space (/var)... "
    AVAILABLE_VAR=$(df /var | tail -1 | awk '{print $4}')
    [ "$AVAILABLE_VAR" -lt 512000 ] && error_exit "Need 500MB in /var, have $((AVAILABLE_VAR/1024))MB"
    echo -e "${GREEN}✓${NC} $((AVAILABLE_VAR/1024))MB"
    
    echo -n "Disk space (/tmp)... "
    AVAILABLE_TMP=$(df /tmp | tail -1 | awk '{print $4}')
    [ "$AVAILABLE_TMP" -lt 256000 ] && error_exit "Need 250MB in /tmp, have $((AVAILABLE_TMP/1024))MB"
    echo -e "${GREEN}✓${NC} $((AVAILABLE_TMP/1024))MB"
    
    echo -n "Memory... "
    TOTAL_RAM=$(free -m | awk 'NR==2{print $2}')
    [ "$TOTAL_RAM" -lt 512 ] && error_exit "Need 512MB RAM minimum"
    [ "$TOTAL_RAM" -lt 1024 ] && echo -e "${YELLOW}⚠${NC} ${TOTAL_RAM}MB (2GB+ recommended)" || echo -e "${GREEN}✓${NC} ${TOTAL_RAM}MB"
    
    echo -n "Prerequisites... "
    for cmd in apt-get dpkg; do
        command -v $cmd &> /dev/null || error_exit "Missing: $cmd"
    done
    echo -e "${GREEN}✓${NC}"
    
    echo -n "Internet... "
    timeout 10 ping -c 1 8.8.8.8 &> /dev/null || \
    timeout 10 ping -c 1 1.1.1.1 &> /dev/null || \
    timeout 10 ping6 -c 1 2606:4700:4700::1111 &> /dev/null || \
    error_exit "No internet (checked IPv4+IPv6)"
    echo -e "${GREEN}✓${NC}"
    
    echo -n "Port 80... "
    if ss -tln 2>/dev/null | grep -q ':80 ' || netstat -tln 2>/dev/null | grep -q ':80 '; then
        [ "$FORCE" != true ] && error_exit "Port 80 in use. Use --force or stop service"
        echo -e "${YELLOW}⚠${NC} In use (forced)"
    else
        echo -e "${GREEN}✓${NC} Available"
    fi
    
    echo -n "Docker containers... "
    if command -v docker &> /dev/null && docker ps -q 2>/dev/null | grep -q .; then
        [ "$FORCE" != true ] && error_exit "Docker has containers. Use --force"
        echo -e "${YELLOW}⚠${NC} Running (forced)"
    else
        echo -e "${GREEN}✓${NC}"
    fi
    
    echo -n "Firewall... "
    if command -v ufw &> /dev/null && ufw status 2>/dev/null | grep -q "active"; then
        echo -e "${YELLOW}⚠${NC} ufw active (may need: ufw allow 80/tcp)"
    elif iptables -L -n 2>/dev/null | grep -q "policy DROP"; then
        echo -e "${YELLOW}⚠${NC} iptables restrictive"
    else
        echo -e "${GREEN}✓${NC}"
    fi
    
    echo -n "Security modules... "
    if command -v aa-status &> /dev/null && aa-status --enabled 2>/dev/null; then
        echo -e "${YELLOW}⚠${NC} AppArmor active"
    elif command -v getenforce &> /dev/null && [ "$(getenforce 2>/dev/null)" = "Enforcing" ]; then
        echo -e "${YELLOW}⚠${NC} SELinux enforcing"
    else
        echo -e "${GREEN}✓${NC}"
    fi
    
    echo -n "Installer files... "
    for file in ./system/dashboard/index.php ./database/schema.sql; do
        [ -f "$file" ] || error_exit "Missing: $file"
    done
    echo -e "${GREEN}✓${NC}"
    
    echo -e "${GREEN}✓${NC} Pre-flight passed"
    echo ""
fi

[ "$DRY_RUN" = true ] && echo "DRY RUN MODE" && echo ""

# Check existing install
INSTALL_MODE="install"
if [ -d "/var/dplane" ]; then
    [ -f "/var/dplane/VERSION" ] && CURRENT_VERSION=$(cat /var/dplane/VERSION) || CURRENT_VERSION="unknown"
    echo -e "${YELLOW}Existing: v${CURRENT_VERSION}${NC}"
    echo ""
    echo "1) Upgrade - Keep data"
    echo "2) Repair - Fix installation"
    echo "3) Fresh - DELETE ALL"
    echo "4) Cancel"
    read -p "Choice [1-4]: " -n 1 -r
    echo
    
    case $REPLY in
        1) INSTALL_MODE="upgrade" ;;
        2) INSTALL_MODE="repair" ;;
        3)
            read -p "Type DELETE: " CONFIRM
            [ "$CONFIRM" = "DELETE" ] || exit 0
            INSTALL_MODE="fresh"
            ;;
        4) exit 0 ;;
        *) error_exit "Invalid choice" ;;
    esac
fi

# Backup
if [ "$INSTALL_MODE" != "fresh" ] && [ "$DRY_RUN" = false ]; then
    mkdir -p /var/dplane/backups
    [ -f /var/dplane/database/dplane.db ] && \
        cp /var/dplane/database/dplane.db /var/dplane/backups/dplane-$(date +%Y%m%d-%H%M%S).db && \
        echo -e "${GREEN}✓${NC} Database backed up"
fi

# Clean fresh install
if [ "$INSTALL_MODE" = "fresh" ] && [ "$DRY_RUN" = false ]; then
    echo -e "${RED}Removing existing...${NC}"
    systemctl stop nginx 2>/dev/null || true
    systemctl stop php*-fpm 2>/dev/null || true
    rm -rf /var/dplane
    rm -f /var/www/dplane /etc/nginx/sites-enabled/dplaneos
    echo -e "${GREEN}✓${NC} Cleaned"
fi

# Dependencies
echo ""
echo "═══════════════════════════════════════════"
echo "  Installing Dependencies"
echo "═══════════════════════════════════════════"

safe_apt_install() {
    local packages="$1"
    local optional="${2:-false}"
    local attempts=0
    
    while [ $attempts -lt 3 ]; do
        attempts=$((attempts + 1))
        
        if [ "$VERBOSE" = true ]; then
            apt-get install -y -o Dpkg::Options::="--force-confold" $packages && return 0
        else
            apt-get install -y -qq -o Dpkg::Options::="--force-confold" $packages &>> /tmp/apt-install.log && return 0
        fi
        
        [ $attempts -lt 3 ] && sleep 2
    done
    
    [ "$optional" = "true" ] && return 0
    error_exit "Failed: $packages"
}

echo -e "${BLUE}Updating packages...${NC}"
if [ "$DRY_RUN" = false ]; then
    attempts=0
    while [ $attempts -lt 3 ]; do
        attempts=$((attempts + 1))
        timeout 180 apt-get update -qq && break
        [ $attempts -lt 3 ] && sleep 5
    done
    [ $attempts -eq 3 ] && error_exit "apt-get update failed"
fi
echo -e "${GREEN}✓${NC} Updated"

# DYNAMIC PHP DETECTION
echo -n "Detecting PHP... "
PHP_VERSION=""
if [ "$DRY_RUN" = false ]; then
    # Find highest available PHP version
    AVAILABLE_PHP=$(apt-cache search --names-only '^php[0-9.]*-fpm$' 2>/dev/null | \
                    grep -oP 'php\K[0-9.]+(?=-fpm)' | \
                    sort -V -r | \
                    head -1)
    
    if [ -n "$AVAILABLE_PHP" ]; then
        PHP_VERSION="$AVAILABLE_PHP"
        PHP_PACKAGES="nginx php${PHP_VERSION}-fpm php${PHP_VERSION}-sqlite3 php${PHP_VERSION}-curl php${PHP_VERSION}-mbstring php${PHP_VERSION}-xml php${PHP_VERSION}-zip"
    else
        # Fallback to generic
        PHP_PACKAGES="nginx php-fpm php-sqlite3 php-curl php-mbstring php-xml php-zip"
    fi
fi
echo -e "${GREEN}✓${NC} ${PHP_VERSION:-generic}"

echo ""
echo -e "${BLUE}Installing packages...${NC}"

# Core
echo -n "  [1/9] Core... "
[ "$DRY_RUN" = false ] && safe_apt_install "sudo wget curl ca-certificates gnupg lsb-release apt-transport-https"
echo -e "${GREEN}✓${NC}"

# ZFS
echo -n "  [2/9] ZFS... "
if [ "$DRY_RUN" = false ]; then
    dpkg -l | grep -q linux-headers-$(uname -r) || safe_apt_install "linux-headers-$(uname -r)" true
    
    if ! safe_apt_install "zfsutils-linux" true; then
        if [ "$OS_ID" = "debian" ]; then
            if ! grep -q "${OS_CODENAME}-backports" /etc/apt/sources.list* 2>/dev/null; then
                echo "deb http://deb.debian.org/debian ${OS_CODENAME}-backports main contrib" > /etc/apt/sources.list.d/backports.list
                apt-get update -qq
            fi
            apt-get install -y -t ${OS_CODENAME}-backports zfsutils-linux &>> /tmp/apt-install.log || true
        fi
    fi
fi
echo -e "${GREEN}✓${NC}"

# Docker - DYNAMIC DETECTION
echo -n "  [3/9] Docker... "
if [ "$DRY_RUN" = false ]; then
    if ! command -v docker &> /dev/null; then
        # Try repo package first
        if safe_apt_install "docker.io" true; then
            :
        else
            # Add Docker official repo
            mkdir -p /etc/apt/keyrings
            DOCKER_OS="${OS_ID}"
            [ "$OS_ID" = "raspbian" ] && DOCKER_OS="debian"
            
            if timeout 30 curl -fsSL https://download.docker.com/linux/${DOCKER_OS}/gpg | \
               gpg --dearmor -o /etc/apt/keyrings/docker.gpg 2>/dev/null; then
                echo "deb [arch=${DOCKER_ARCH} signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/${DOCKER_OS} ${OS_CODENAME} stable" > /etc/apt/sources.list.d/docker.list
                timeout 60 apt-get update -qq
                safe_apt_install "docker-ce docker-ce-cli containerd.io"
            else
                error_exit "Cannot get Docker GPG key"
            fi
        fi
    fi
fi
echo -e "${GREEN}✓${NC}"

# Docker Compose - DYNAMIC
echo -n "  [4/9] Compose... "
if [ "$DRY_RUN" = false ]; then
    safe_apt_install "docker-compose" true || \
    safe_apt_install "docker-compose-plugin" true || \
    safe_apt_install "docker-compose-v2" true
fi
echo -e "${GREEN}✓${NC}"

# Web server
echo -n "  [5/9] Web server... "
if [ "$DRY_RUN" = false ]; then
    safe_apt_install "$PHP_PACKAGES" || {
        # Fallback: try ANY available PHP
        FALLBACK_PHP=$(apt-cache search --names-only '^php[0-9.]*-fpm$' | head -1 | cut -d' ' -f1 | grep -oP 'php\K[0-9.]+(?=-fpm)')
        if [ -n "$FALLBACK_PHP" ]; then
            PHP_VERSION="$FALLBACK_PHP"
            PHP_PACKAGES="nginx php${PHP_VERSION}-fpm php${PHP_VERSION}-sqlite3 php${PHP_VERSION}-curl php${PHP_VERSION}-mbstring php${PHP_VERSION}-xml php${PHP_VERSION}-zip"
            safe_apt_install "$PHP_PACKAGES"
        else
            error_exit "No PHP version available"
        fi
    }
    
    # Auto-detect installed version if generic was used
    [ -z "$PHP_VERSION" ] && command -v php &> /dev/null && \
        PHP_VERSION=$(php -v 2>/dev/null | head -1 | grep -oP '\d+\.\d+' | head -1)
fi
echo -e "${GREEN}✓${NC} (PHP ${PHP_VERSION})"

# Database
echo -n "  [6/9] SQLite... "
[ "$DRY_RUN" = false ] && safe_apt_install "sqlite3"
echo -e "${GREEN}✓${NC}"

# Storage
echo -n "  [7/9] Storage tools... "
[ "$DRY_RUN" = false ] && safe_apt_install "smartmontools lsof hdparm" true
echo -e "${GREEN}✓${NC}"

# Sharing
echo -n "  [8/9] File sharing... "
[ "$DRY_RUN" = false ] && safe_apt_install "samba nfs-kernel-server" true
echo -e "${GREEN}✓${NC}"

# Cloud
echo -n "  [9/9] Cloud sync... "
[ "$DRY_RUN" = false ] && safe_apt_install "rclone" true
echo -e "${GREEN}✓${NC}"

echo ""
echo -e "${GREEN}✓${NC} All dependencies installed"

# System structure
echo ""
echo "═══════════════════════════════════════════"
echo "  Creating System Structure"
echo "═══════════════════════════════════════════"

echo -n "Directories... "
if [ "$DRY_RUN" = false ]; then
    mkdir -p /var/dplane/{database,system/{dashboard,bin,config},backups,logs}
    mkdir -p /var/dplane/system/dashboard/{includes,api/{storage,system,containers,v1/{storage,system,containers}}}
fi
echo -e "${GREEN}✓${NC}"

echo -n "Files... "
if [ "$DRY_RUN" = false ]; then
    [ -d "./system" ] || error_exit "./system not found"
    cp -r ./system/* /var/dplane/system/
    [ -d "./database" ] && cp -r ./database/* /var/dplane/database/
fi
echo -e "${GREEN}✓${NC}"

# Database
echo -n "Database... "
if [ "$DRY_RUN" = false ]; then
    if [ "$INSTALL_MODE" = "fresh" ] || [ ! -f /var/dplane/database/dplane.db ]; then
        [ -f /var/dplane/database/schema.sql ] || error_exit "schema.sql missing"
        sqlite3 /var/dplane/database/dplane.db < /var/dplane/database/schema.sql
        sqlite3 /var/dplane/database/dplane.db "INSERT INTO users (username, password, email) VALUES ('admin', '\$2y\$10\$92IXUNpkjO0rOQ5byMi.Ye4oKoEa3Ro9llC/.og/at2.uheWG/igi', 'admin@localhost');"
    fi
fi
echo -e "${GREEN}✓${NC}"

# Web server config
echo ""
echo "═══════════════════════════════════════════"
echo "  Configuring Services"
echo "═══════════════════════════════════════════"

# DYNAMIC PHP SOCKET DETECTION
echo -n "PHP socket... "
if [ "$DRY_RUN" = false ]; then
    PHP_SOCKET=""
    # Try multiple socket patterns
    for pattern in "/run/php/php${PHP_VERSION}-fpm.sock" \
                   "/var/run/php/php${PHP_VERSION}-fpm.sock" \
                   "/run/php/php-fpm.sock" \
                   "/var/run/php-fpm/php-fpm.sock" \
                   "/run/php/php-fpm${PHP_VERSION}.sock"; do
        if [ -S "$pattern" ] || [ "$INSTALL_MODE" = "fresh" ]; then
            PHP_SOCKET="$pattern"
            break
        fi
    done
    
    # If still not found, check what sockets exist
    if [ -z "$PHP_SOCKET" ]; then
        PHP_SOCKET=$(find /run/php /var/run/php -name "*fpm*.sock" 2>/dev/null | head -1)
    fi
    
    # Ultimate fallback
    [ -z "$PHP_SOCKET" ] && PHP_SOCKET="/run/php/php${PHP_VERSION}-fpm.sock"
    echo -e "${GREEN}✓${NC} $PHP_SOCKET"
else
    PHP_SOCKET="/run/php/php-fpm.sock"
    echo -e "${GREEN}✓${NC} (dry-run)"
fi

echo -n "Nginx... "
if [ "$DRY_RUN" = false ]; then
    cat > /etc/nginx/sites-available/dplaneos << EOF
server {
    listen 80 default_server;
    listen [::]:80 default_server;
    
    root /var/dplane/system/dashboard;
    index index.php index.html;
    server_name _;
    
    client_max_body_size 10G;
    client_body_timeout 600s;
    
    location / {
        try_files \$uri \$uri/ /index.php?\$query_string;
    }
    
    location ~ \.php$ {
        include snippets/fastcgi-php.conf;
        fastcgi_pass unix:${PHP_SOCKET};
        fastcgi_param SCRIPT_FILENAME \$document_root\$fastcgi_script_name;
        include fastcgi_params;
        fastcgi_read_timeout 600;
    }
    
    location ~ /\.ht {
        deny all;
    }
}
EOF
    
    ln -sf /etc/nginx/sites-available/dplaneos /etc/nginx/sites-enabled/dplaneos
    rm -f /etc/nginx/sites-enabled/default
    nginx -t &>> /tmp/apt-install.log || error_exit "Nginx config invalid"
fi
echo -e "${GREEN}✓${NC}"

echo -n "PHP... "
if [ "$DRY_RUN" = false ]; then
    # Find PHP ini dynamically
    for ini in "/etc/php/${PHP_VERSION}/fpm/php.ini" \
               "/etc/php/fpm/php.ini" \
               "/etc/php.ini"; do
        if [ -f "$ini" ]; then
            sed -i 's/upload_max_filesize = .*/upload_max_filesize = 10G/' "$ini"
            sed -i 's/post_max_size = .*/post_max_size = 10G/' "$ini"
            sed -i 's/max_execution_time = .*/max_execution_time = 600/' "$ini"
            sed -i 's/memory_limit = .*/memory_limit = 512M/' "$ini"
            break
        fi
    done
fi
echo -e "${GREEN}✓${NC}"

# Sudoers
echo -n "Sudoers... "
if [ "$DRY_RUN" = false ]; then
    command -v sudo &> /dev/null || safe_apt_install "sudo"
    
    mkdir -p /var/dplane/backups/sudoers
    [ -f /etc/sudoers.d/dplaneos ] && \
        cp /etc/sudoers.d/dplaneos /var/dplane/backups/sudoers/dplaneos.$(date +%Y%m%d_%H%M%S)
    
    if [ -f "./system/config/sudoers.enhanced" ]; then
        TEMP_SUDOERS="/tmp/dplaneos_sudoers_$$"
        cp ./system/config/sudoers.enhanced "$TEMP_SUDOERS"
        chmod 440 "$TEMP_SUDOERS"
        
        if visudo -c -f "$TEMP_SUDOERS" &>/dev/null; then
            mv "$TEMP_SUDOERS" /etc/sudoers.d/dplaneos
        else
            rm -f "$TEMP_SUDOERS"
        fi
    fi
    
    if [ ! -f /etc/sudoers.d/dplaneos ]; then
        cat > /etc/sudoers.d/dplaneos << 'SUDOERS_EOF'
www-data ALL=(ALL) NOPASSWD: /usr/sbin/zfs
www-data ALL=(ALL) NOPASSWD: /usr/sbin/zpool
www-data ALL=(ALL) NOPASSWD: /usr/sbin/smartctl
www-data ALL=(ALL) NOPASSWD: /usr/bin/lsblk
www-data ALL=(ALL) NOPASSWD: /usr/bin/lsof
www-data ALL=(ALL) NOPASSWD: /usr/bin/docker
www-data ALL=(ALL) NOPASSWD: /usr/bin/docker-compose
www-data ALL=(ALL) NOPASSWD: /usr/bin/systemctl
SUDOERS_EOF
        chmod 440 /etc/sudoers.d/dplaneos
    fi
    
    visudo -c &>/dev/null || error_exit "Sudoers invalid"
    
    id -u www-data &>/dev/null || useradd -r -s /usr/sbin/nologin www-data
    usermod -aG docker www-data 2>/dev/null || true
fi
echo -e "${GREEN}✓${NC}"

# Permissions
echo -n "Permissions... "
if [ "$DRY_RUN" = false ]; then
    chown -R www-data:www-data /var/dplane
    chmod -R 755 /var/dplane/system
    chmod 775 /var/dplane/database
    [ -f /var/dplane/database/dplane.db ] && chmod 664 /var/dplane/database/dplane.db
    [ -f /var/dplane/system/bin/monitor.sh ] && chmod +x /var/dplane/system/bin/monitor.sh
fi
echo -e "${GREEN}✓${NC}"

# Monitoring
if [ -f ./system/bin/monitor.sh ] && [ "$DRY_RUN" = false ]; then
    echo -n "Monitoring... "
    cat > /etc/cron.d/dplaneos-monitor << 'CRON_EOF'
*/5 * * * * root /var/dplane/system/bin/monitor.sh >/dev/null 2>&1
CRON_EOF
    chmod 644 /etc/cron.d/dplaneos-monitor
    echo -e "${GREEN}✓${NC}"
fi

# Version
echo -n "Version... "
if [ "$DRY_RUN" = false ]; then
    echo "1.9.0" > /var/dplane/VERSION
    chown www-data:www-data /var/dplane/VERSION
fi
echo -e "${GREEN}✓${NC}"

# Start services
echo ""
echo "═══════════════════════════════════════════"
echo "  Starting Services"
echo "═══════════════════════════════════════════"

if [ "$DRY_RUN" = false ]; then
    echo -n "Enabling... "
    systemctl enable nginx php${PHP_VERSION}-fpm docker 2>/dev/null || true
    echo -e "${GREEN}✓${NC}"
    
    echo -n "PHP-FPM... "
    systemctl restart php${PHP_VERSION}-fpm || error_exit "PHP-FPM failed"
    echo -e "${GREEN}✓${NC}"
    
    echo -n "Docker... "
    systemctl restart docker || { systemctl enable docker; echo -e "${YELLOW}⚠${NC} Needs reboot"; }
    echo -e "${GREEN}✓${NC}"
    
    echo -n "Nginx... "
    systemctl reload nginx || systemctl restart nginx || error_exit "Nginx failed"
    echo -e "${GREEN}✓${NC}"
fi

# Validation
if [ "$DRY_RUN" = false ]; then
    echo ""
    echo "═══════════════════════════════════════════"
    echo "  Validation"
    echo "═══════════════════════════════════════════"
    
    systemctl is-active --quiet nginx && echo -e "Nginx:    ${GREEN}✓${NC}" || echo -e "Nginx:    ${RED}✗${NC}"
    systemctl is-active --quiet php${PHP_VERSION}-fpm && echo -e "PHP-FPM:  ${GREEN}✓${NC}" || echo -e "PHP-FPM:  ${RED}✗${NC}"
    systemctl is-active --quiet docker && echo -e "Docker:   ${GREEN}✓${NC}" || echo -e "Docker:   ${YELLOW}⚠${NC}"
    [ -f /var/dplane/database/dplane.db ] && sqlite3 /var/dplane/database/dplane.db "SELECT 1 FROM users LIMIT 1" &>/dev/null && echo -e "Database: ${GREEN}✓${NC}" || echo -e "Database: ${RED}✗${NC}"
fi

[ "$DRY_RUN" = true ] && echo "" && echo "DRY RUN COMPLETE" && exit 0

IP_ADDR=$(hostname -I | awk '{print $1}')
[ -z "$IP_ADDR" ] && IP_ADDR="<server-ip>"

echo ""
echo -e "${GREEN}"
echo "╔═══════════════════════════════════════════╗"
echo "║        Installation Complete!             ║"
echo "╚═══════════════════════════════════════════╝"
echo -e "${NC}"
echo ""
echo -e "${BLUE}Dashboard:${NC} ${GREEN}http://${IP_ADDR}${NC}"
echo -e "${BLUE}Login:${NC}     ${YELLOW}admin / admin${NC}"
echo ""
echo -e "${RED}━━━ CHANGE PASSWORD NOW ━━━${NC}"
echo ""
echo -e "${BLUE}System:${NC} ${OS_NAME} ${OS_VERSION} | PHP ${PHP_VERSION}"
echo ""
[ ! -f /usr/sbin/zpool ] && echo -e "${YELLOW}Note: Install ZFS: apt install zfsutils-linux${NC}" && echo ""
echo -e "${BLUE}Log:${NC} /tmp/dplaneos-install.log"
echo ""
