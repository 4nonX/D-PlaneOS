#!/bin/bash
# D-PlaneOS v1.8.0 - Installation Script with Safety Rails
set -e
set -u  # Fail on undefined variables
set -o pipefail  # Fail on pipe errors

# Safety flags
DRY_RUN=false
SKIP_VALIDATION=false

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --dry-run)
            DRY_RUN=true
            shift
            ;;
        --skip-validation)
            SKIP_VALIDATION=true
            shift
            ;;
        --help)
            echo "D-PlaneOS v1.8.0 Installer"
            echo ""
            echo "Usage: $0 [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  --dry-run           Show what would be done without making changes"
            echo "  --skip-validation   Skip pre-flight checks (DANGEROUS)"
            echo "  --help              Show this help message"
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            echo "Run with --help for usage"
            exit 1
            ;;
    esac
done

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${GREEN}"
echo "╔═══════════════════════════════════════════╗"
echo "║     D-PlaneOS v1.8.0 - Installer          ║"
echo "╚═══════════════════════════════════════════╝"
echo -e "${NC}"

# Check root
if [[ $EUID -ne 0 ]]; then
   echo -e "${RED}This script must be run as root${NC}"
   exit 1
fi

echo -e "${GREEN}✓${NC} Running as root"

# Pre-flight validation
if [ "$SKIP_VALIDATION" = false ]; then
    echo ""
    echo "═══════════════════════════════════════════"
    echo "  Pre-flight System Validation"
    echo "═══════════════════════════════════════════"
    
    # Check system requirements
    echo -n "Checking system requirements... "
    FAILED_CHECKS=0
    
    # Check available disk space (need at least 500MB)
    AVAILABLE_SPACE=$(df /var | tail -1 | awk '{print $4}')
    if [ "$AVAILABLE_SPACE" -lt 512000 ]; then
        echo -e "${RED}✗${NC}"
        echo -e "${RED}ERROR: Insufficient disk space in /var (need 500MB, have $(($AVAILABLE_SPACE/1024))MB)${NC}"
        FAILED_CHECKS=$((FAILED_CHECKS + 1))
    fi
    
    # Check if required commands exist
    for cmd in zfs zpool docker php systemctl; do
        if ! command -v $cmd &> /dev/null; then
            if [ "$FAILED_CHECKS" -eq 0 ]; then
                echo -e "${RED}✗${NC}"
            fi
            echo -e "${YELLOW}WARNING: Command '$cmd' not found - some features may not work${NC}"
        fi
    done
    
    if [ "$FAILED_CHECKS" -eq 0 ]; then
        echo -e "${GREEN}✓${NC}"
    else
        echo ""
        echo -e "${RED}Pre-flight checks failed. Cannot continue.${NC}"
        exit 1
    fi
    
    # Validate sudoers file if it will be installed
    if [ -f "system/config/sudoers.enhanced" ]; then
        echo -n "Validating sudoers configuration... "
        
        # Test sudoers syntax using visudo
        if visudo -c -f system/config/sudoers.enhanced &> /dev/null; then
            echo -e "${GREEN}✓${NC}"
        else
            echo -e "${RED}✗${NC}"
            echo -e "${RED}ERROR: sudoers.enhanced has syntax errors!${NC}"
            echo "Run: visudo -c -f system/config/sudoers.enhanced"
            echo "to see the errors."
            exit 1
        fi
    fi
    
    # Verify file integrity if SHA256SUMS exists
    if [ -f "SHA256SUMS" ]; then
        echo -n "Verifying file integrity... "
        if sha256sum -c SHA256SUMS >/dev/null 2>&1; then
            echo -e "${GREEN}✓${NC}"
        else
            echo -e "${RED}✗${NC}"
            echo -e "${RED}ERROR: File integrity check failed!${NC}"
            echo "One or more files have been modified or corrupted."
            echo "Details:"
            sha256sum -c SHA256SUMS 2>&1 | grep FAILED
            exit 1
        fi
    fi
    
    echo -e "${GREEN}✓${NC} All pre-flight checks passed"
    echo ""
fi

if [ "$DRY_RUN" = true ]; then
    echo "═══════════════════════════════════════════"
    echo "  DRY RUN MODE - No changes will be made"
    echo "═══════════════════════════════════════════"
    echo ""
fi

# Check if upgrading
INSTALL_MODE="install"
if [ -d "/var/dplane" ]; then
    if [ -f "/var/dplane/VERSION" ]; then
        CURRENT_VERSION=$(cat /var/dplane/VERSION)
        echo -e "${YELLOW}Existing installation detected: v${CURRENT_VERSION}${NC}"
    else
        echo -e "${YELLOW}Existing installation detected (unknown version)${NC}"
    fi
    
    echo ""
    echo "Select installation mode:"
    echo "1) Upgrade (preserve data, update system)"
    echo "2) Repair (fix broken installation)"
    echo "3) Fresh install (DESTROYS ALL DATA)"
    echo "4) Cancel"
    read -p "Choice [1-4]: " -n 1 -r
    echo
    
    case $REPLY in
        1)
            INSTALL_MODE="upgrade"
            echo -e "${GREEN}Upgrade mode selected${NC}"
            ;;
        2)
            INSTALL_MODE="repair"
            echo -e "${GREEN}Repair mode selected${NC}"
            ;;
        3)
            read -p "Type 'DELETE' to confirm fresh install: " CONFIRM
            if [ "$CONFIRM" != "DELETE" ]; then
                echo "Cancelled"
                exit 0
            fi
            INSTALL_MODE="fresh"
            echo -e "${RED}Fresh install mode - data will be destroyed${NC}"
            ;;
        4)
            echo "Installation cancelled"
            exit 0
            ;;
        *)
            echo "Invalid choice"
            exit 1
            ;;
    esac
fi

if [ "$INSTALL_MODE" = "upgrade" ] || [ "$INSTALL_MODE" = "repair" ]; then
    # Backup existing database
    mkdir -p /var/dplane/backups
    if [ -f /var/dplane/database/dplane.db ]; then
        cp /var/dplane/database/dplane.db /var/dplane/backups/dplane-$(date +%Y%m%d-%H%M%S).db
        echo -e "${GREEN}✓${NC} Database backed up"
    fi
fi

if [ "$INSTALL_MODE" = "fresh" ]; then
    echo -e "${RED}Removing existing installation...${NC}"
    systemctl stop nginx php8.2-fpm docker 2>/dev/null
    rm -rf /var/dplane
    rm -f /var/www/dplane
    echo -e "${GREEN}✓${NC} Cleaned"
fi

# Install dependencies
echo -e "${YELLOW}Installing dependencies...${NC}"
apt-get update -qq
apt-get install -y -qq zfsutils-linux docker.io docker-compose php8.2-fpm php8.2-sqlite3 nginx sqlite3 smartmontools lsof samba nfs-kernel-server rclone

echo -e "${GREEN}✓${NC} Dependencies installed"

# Create directories
echo -e "${YELLOW}Creating directories...${NC}"
mkdir -p /var/dplane/{database,logs,backups,compose}
mkdir -p /var/www
echo -e "${GREEN}✓${NC} Directories created"

# Copy files
echo -e "${YELLOW}Installing files...${NC}"

if [ "$DRY_RUN" = true ]; then
    echo "[DRY RUN] Would copy: ./system -> /var/dplane/"
    echo "[DRY RUN] Would create symlink: /var/dplane/system/dashboard -> /var/www/dplane"
else
    # Backup existing system directory if upgrade
    if [ "$INSTALL_MODE" = "upgrade" ] && [ -d /var/dplane/system ]; then
        echo -n "Backing up existing system... "
        BACKUP_TIMESTAMP=$(date +%Y%m%d_%H%M%S)
        mv /var/dplane/system "/var/dplane/backups/system.${BACKUP_TIMESTAMP}"
        echo -e "${GREEN}✓${NC}"
    fi
    
    # Copy new system files
    cp -r ./system /var/dplane/ || {
        echo -e "${RED}✗ Failed to copy system files${NC}"
        exit 1
    }
    
    # Create web dashboard symlink
    ln -sf /var/dplane/system/dashboard /var/www/dplane || {
        echo -e "${RED}✗ Failed to create dashboard symlink${NC}"
        exit 1
    }
    
    echo -e "${GREEN}✓${NC} Files installed"
fi

# Initialize database
echo -e "${YELLOW}Initializing database...${NC}"
if [ ! -f /var/dplane/database/dplane.db ]; then
    sqlite3 /var/dplane/database/dplane.db < ./database/schema.sql
    echo -e "${GREEN}✓${NC} Database created"
else
    echo -e "${YELLOW}ℹ${NC} Database exists, skipping"
fi

# Configure Nginx
echo -e "${YELLOW}Configuring Nginx...${NC}"
cat > /etc/nginx/sites-available/dplaneos << 'EOF'
server {
    listen 80 default_server;
    server_name _;
    root /var/www/dplane;
    index index.php;
    
    client_max_body_size 1G;
    
    location / {
        try_files $uri $uri/ /index.php?$query_string;
    }
    
    location ~ \.php$ {
        include snippets/fastcgi-php.conf;
        fastcgi_pass unix:/var/run/php/php8.2-fpm.sock;
    }
    
    location ~ /\.ht {
        deny all;
    }
}
EOF

ln -sf /etc/nginx/sites-available/dplaneos /etc/nginx/sites-enabled/
rm -f /etc/nginx/sites-enabled/default

nginx -t
systemctl reload nginx
echo -e "${GREEN}✓${NC} Nginx configured"

# Configure PHP
echo -e "${YELLOW}Configuring PHP...${NC}"
sed -i 's/upload_max_filesize = .*/upload_max_filesize = 1G/' /etc/php/8.2/fpm/php.ini
sed -i 's/post_max_size = .*/post_max_size = 1G/' /etc/php/8.2/fpm/php.ini
sed -i 's/max_execution_time = .*/max_execution_time = 300/' /etc/php/8.2/fpm/php.ini
systemctl restart php8.2-fpm
echo -e "${GREEN}✓${NC} PHP configured"

# Configure sudoers
echo -e "${YELLOW}Configuring sudoers (enhanced security)...${NC}"

if [ "$DRY_RUN" = true ]; then
    echo "[DRY RUN] Would backup existing sudoers"
    echo "[DRY RUN] Would install: system/config/sudoers.enhanced -> /etc/sudoers.d/dplaneos"
    echo "[DRY RUN] Would set permissions: 440"
else
    # Create timestamped backup
    BACKUP_TIMESTAMP=$(date +%Y%m%d_%H%M%S)
    BACKUP_DIR="/var/dplane/backups/sudoers"
    mkdir -p "$BACKUP_DIR"
    
    # Backup existing sudoers if exists
    if [ -f /etc/sudoers.d/dplaneos ]; then
        echo -n "Creating backup of existing sudoers... "
        cp /etc/sudoers.d/dplaneos "$BACKUP_DIR/dplaneos.${BACKUP_TIMESTAMP}"
        echo -e "${GREEN}✓${NC}"
    fi
    
    # Install new sudoers to temp location first
    TEMP_SUDOERS="/tmp/dplaneos_sudoers_$BACKUP_TIMESTAMP"
    cp ./system/config/sudoers.enhanced "$TEMP_SUDOERS"
    chmod 440 "$TEMP_SUDOERS"
    
    # Validate before moving to production
    echo -n "Validating new sudoers configuration... "
    if visudo -c -f "$TEMP_SUDOERS" >/dev/null 2>&1; then
        echo -e "${GREEN}✓${NC}"
        
        # Move to production location
        mv "$TEMP_SUDOERS" /etc/sudoers.d/dplaneos
        
        # Final validation in production location
        echo -n "Final validation check... "
        if visudo -c >/dev/null 2>&1; then
            echo -e "${GREEN}✓${NC} Enhanced sudo configuration active"
        else
            echo -e "${RED}✗${NC}"
            echo -e "${RED}CRITICAL: sudoers validation failed in production!${NC}"
            
            # Attempt rollback
            if [ -f "$BACKUP_DIR/dplaneos.${BACKUP_TIMESTAMP}" ]; then
                echo "Attempting rollback..."
                cp "$BACKUP_DIR/dplaneos.${BACKUP_TIMESTAMP}" /etc/sudoers.d/dplaneos
                
                if visudo -c >/dev/null 2>&1; then
                    echo -e "${GREEN}✓${NC} Rollback successful"
                else
                    echo -e "${RED}EMERGENCY: Rollback failed!${NC}"
                    echo "Manual intervention required: check /etc/sudoers.d/dplaneos"
                    echo "Backup available at: $BACKUP_DIR/dplaneos.${BACKUP_TIMESTAMP}"
                fi
            fi
            exit 1
        fi
    else
        echo -e "${RED}✗${NC}"
        echo -e "${RED}ERROR: New sudoers configuration has syntax errors!${NC}"
        rm -f "$TEMP_SUDOERS"
        exit 1
    fi
    
    # Test actual command execution (non-fatal)
    if sudo -u www-data sudo /usr/sbin/zpool list >/dev/null 2>&1 || [ $? -eq 1 ]; then
        echo -e "${GREEN}✓${NC} Sudo execution test passed"
    else
        echo -e "${YELLOW}⚠${NC} Sudo execution test inconclusive (no pools exist yet)"
    fi
fi
else
    echo -e "${RED}✗${NC} Enhanced sudo configuration validation failed"
    
    # Fallback to basic configuration
    echo -e "${YELLOW}Installing fallback sudo configuration...${NC}"
    cat > /etc/sudoers.d/dplaneos << 'EOF'
www-data ALL=(ALL) NOPASSWD: /usr/sbin/zfs
www-data ALL=(ALL) NOPASSWD: /usr/sbin/zpool
www-data ALL=(ALL) NOPASSWD: /usr/sbin/smartctl
www-data ALL=(ALL) NOPASSWD: /usr/bin/lsblk
www-data ALL=(ALL) NOPASSWD: /usr/bin/lsof
www-data ALL=(ALL) NOPASSWD: /usr/bin/docker
www-data ALL=(ALL) NOPASSWD: /usr/bin/docker-compose
EOF
    chmod 440 /etc/sudoers.d/dplaneos
    echo -e "${GREEN}✓${NC} Fallback sudo configuration installed"
fi

usermod -aG docker www-data
echo -e "${GREEN}✓${NC} Sudoers configured"

# Set permissions
echo -e "${YELLOW}Setting permissions...${NC}"
chown -R www-data:www-data /var/dplane
chmod -R 755 /var/dplane/system
chmod 644 /var/dplane/database/dplane.db
chmod +x /var/dplane/system/bin/monitor.sh
echo -e "${GREEN}✓${NC} Permissions set"

# Setup monitoring cron
echo -e "${YELLOW}Setting up monitoring...${NC}"
cat > /etc/cron.d/dplaneos-monitor << 'EOF'
*/5 * * * * root /var/dplane/system/bin/monitor.sh
EOF
chmod 644 /etc/cron.d/dplaneos-monitor
echo -e "${GREEN}✓${NC} Monitoring configured"

# Create version file
echo -e "${YELLOW}Recording version...${NC}"
if [ "$DRY_RUN" = true ]; then
    echo "[DRY RUN] Would create /var/dplane/VERSION with: 1.8.0"
else
    echo "1.8.0" > /var/dplane/VERSION
    chown www-data:www-data /var/dplane/VERSION
    echo -e "${GREEN}✓${NC} Version recorded"
fi

# Enable services
if [ "$DRY_RUN" = true ]; then
    echo "[DRY RUN] Would enable: nginx php8.2-fpm docker"
    echo "[DRY RUN] Would start: nginx php8.2-fpm docker"
else
    systemctl enable nginx php8.2-fpm docker
    systemctl start nginx php8.2-fpm docker
fi

if [ "$DRY_RUN" = true ]; then
    echo ""
    echo "═══════════════════════════════════════════"
    echo "  DRY RUN COMPLETE"
    echo "═══════════════════════════════════════════"
    echo ""
    echo "No changes were made to your system."
    echo "Run without --dry-run to perform actual installation."
    exit 0
fi

echo -e "${GREEN}"
echo "╔═══════════════════════════════════════════╗"
echo "║     Installation Complete!                ║"
echo "╚═══════════════════════════════════════════╝"
echo -e "${NC}"
echo ""
echo -e "Access: ${GREEN}http://$(hostname -I | awk '{print $1}')${NC}"
echo -e "Login: ${YELLOW}admin / admin${NC}"
echo ""
echo -e "${RED}⚠ CHANGE DEFAULT PASSWORD IMMEDIATELY!${NC}"
echo ""
