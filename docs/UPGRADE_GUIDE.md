# D-PlaneOS Upgrade Guide: v1.12.0 → v1.13.0

## Pre-Upgrade Checklist

Before starting the upgrade, ensure:

- [ ] You have root/sudo access to the server
- [ ] Current version is v1.12.0 (check `/var/www/dplaneos/VERSION`)
- [ ] System is healthy (`zpool status` shows no critical errors)
- [ ] You have recent ZFS snapshots of important data
- [ ] You have 30 minutes of maintenance window
- [ ] You've reviewed the [release notes](RELEASE_NOTES_v1.13.0.md)

---

## Upgrade Methods

### Method 1: Automated Script (Recommended)

**Estimated time: 5-10 minutes**

```bash
# Download release package
cd /tmp
wget https://github.com/your-org/dplaneos/releases/download/v1.13.0/dplaneos-v1.13.0.tar.gz

# Verify checksum
sha256sum dplaneos-v1.13.0.tar.gz
# Compare with published checksum

# Extract
tar xzf dplaneos-v1.13.0.tar.gz
cd dplaneos-v1.13.0

# Run upgrade script
sudo ./scripts/upgrade-to-v1.13.0.sh
```

**The script automatically:**
1. Creates pre-upgrade database backup
2. Installs new API files (`backup.php`, `disk-replacement.php`)
3. Installs new JavaScript modules
4. Runs database migration (creates new tables)
5. Creates backup directory (`/var/backups/dplaneos`)
6. Installs auto-backup script
7. Installs dependencies (OpenSSL, smartmontools)
8. Updates version to 1.13.0

**Output should end with:**
```
╔══════════════════════════════════════════════════════╗
║          Upgrade to v1.13.0 Complete! ✓             ║
╚══════════════════════════════════════════════════════╝
```

---

### Method 2: Manual Upgrade

**For advanced users who want full control:**

#### Step 1: Backup Current System
```bash
# Create database backup
sudo cp /var/www/dplaneos/database.sqlite \
        /var/backups/dplaneos-pre-v1.13.0-$(date +%Y%m%d).sqlite

# Backup current web files (optional)
sudo tar czf /var/backups/dplaneos-web-backup-$(date +%Y%m%d).tar.gz \
        /var/www/dplaneos/
```

#### Step 2: Download & Extract
```bash
cd /tmp
wget https://github.com/your-org/dplaneos/releases/download/v1.13.0/dplaneos-v1.13.0.tar.gz
tar xzf dplaneos-v1.13.0.tar.gz
cd dplaneos-v1.13.0
```

#### Step 3: Install New API Files
```bash
sudo cp api/backup.php /var/www/dplaneos/api/
sudo cp api/disk-replacement.php /var/www/dplaneos/api/
sudo chown www-data:www-data /var/www/dplaneos/api/*.php
sudo chmod 644 /var/www/dplaneos/api/*.php
```

#### Step 4: Install JavaScript Files
```bash
sudo mkdir -p /var/www/dplaneos/js
sudo cp js/backup.js /var/www/dplaneos/js/
sudo cp js/disk-replacement.js /var/www/dplaneos/js/
sudo chown www-data:www-data /var/www/dplaneos/js/*.js
sudo chmod 644 /var/www/dplaneos/js/*.js
```

#### Step 5: Run Database Migration
```bash
sudo sqlite3 /var/www/dplaneos/database.sqlite < sql/003_backup_and_disk_replacement.sql
```

#### Step 6: Create Backup Directory
```bash
sudo mkdir -p /var/backups/dplaneos
sudo chown www-data:www-data /var/backups/dplaneos
sudo chmod 700 /var/backups/dplaneos
```

#### Step 7: Install Scripts
```bash
sudo cp scripts/auto-backup.php /var/www/dplaneos/scripts/
sudo chmod +x /var/www/dplaneos/scripts/auto-backup.php
sudo chown www-data:www-data /var/www/dplaneos/scripts/auto-backup.php
sudo touch /var/log/dplaneos-backup.log
sudo chown www-data:www-data /var/log/dplaneos-backup.log
```

#### Step 8: Install Dependencies
```bash
# OpenSSL (for encryption)
sudo apt-get update
sudo apt-get install -y openssl

# smartmontools (for disk health)
sudo apt-get install -y smartmontools
```

#### Step 9: Update Version
```bash
echo "1.13.0" | sudo tee /var/www/dplaneos/VERSION
```

#### Step 10: Verify Installation
```bash
# Check version
cat /var/www/dplaneos/VERSION
# Should output: 1.13.0

# Check API files exist
ls -la /var/www/dplaneos/api/backup.php
ls -la /var/www/dplaneos/api/disk-replacement.php

# Check database tables
sudo sqlite3 /var/www/dplaneos/database.sqlite "SELECT name FROM sqlite_master WHERE type='table' AND name LIKE '%backup%';"
# Should show: config_backups

# Check permissions
ls -ld /var/backups/dplaneos
# Should show: drwx------ ... www-data www-data
```

---

## Post-Upgrade Steps

### 1. Test Config Backup Feature

```bash
# Access web UI
https://your-server-ip

# Navigate to: Settings → Backup & Restore
# Click: "Create Backup"
# Result: Backup created with password shown
# IMPORTANT: Save the password!
```

### 2. Test Disk Health Monitoring

```bash
# Navigate to: Storage → Disk Health (new section)
# Should show: All pools with disk status
```

### 3. Configure Automated Backups (Optional)

```bash
# In UI: Settings → Backup & Restore → Schedule
# Set:
#   - Frequency: Daily
#   - Time: 02:00
#   - Keep backups for: 30 days
# Click: "Schedule Automatic Backups"
```

### 4. Download First Backup

```bash
# In UI: Settings → Backup & Restore
# In backup list, click: "Download"
# Save to secure location (USB drive, cloud storage)
```

---

## Verification Checklist

After upgrade, verify:

- [ ] Version shows `1.13.0` in UI
- [ ] Settings → Backup & Restore page loads
- [ ] Can create a test backup
- [ ] Backup password is shown and copyable
- [ ] Can download backup file
- [ ] Storage → Disk Health shows pool status
- [ ] All existing functionality still works:
  - [ ] Docker containers still running
  - [ ] SMB/NFS shares still accessible
  - [ ] ZFS pools still ONLINE
  - [ ] App Store still functional

---

## Rollback Procedure

If upgrade fails or causes issues:

### Option 1: Restore Database Only

```bash
# Find your pre-upgrade backup
ls -la /var/backups/dplaneos-pre-v1.13.0-*.sqlite

# Restore it
sudo cp /var/backups/dplaneos-pre-v1.13.0-YYYYMMDD.sqlite \
        /var/www/dplaneos/database.sqlite

# Revert version
echo "1.12.0" | sudo tee /var/www/dplaneos/VERSION

# Remove new API files (optional)
sudo rm /var/www/dplaneos/api/backup.php
sudo rm /var/www/dplaneos/api/disk-replacement.php
sudo rm /var/www/dplaneos/js/backup.js
sudo rm /var/www/dplaneos/js/disk-replacement.js

# Restart Apache
sudo systemctl restart apache2
```

### Option 2: Full System Restore

If you have a full web directory backup:

```bash
# Stop Apache
sudo systemctl stop apache2

# Restore entire directory
sudo rm -rf /var/www/dplaneos
sudo tar xzf /var/backups/dplaneos-web-backup-YYYYMMDD.tar.gz -C /

# Start Apache
sudo systemctl start apache2
```

---

## Troubleshooting

### Issue: Database migration failed

**Symptoms:**
```
Error: near "CREATE": syntax error
```

**Solution:**
```bash
# Check SQLite version
sqlite3 --version
# Should be 3.x.x

# Manually run migration
sudo sqlite3 /var/www/dplaneos/database.sqlite
sqlite> .read /tmp/dplaneos-v1.13.0/sql/003_backup_and_disk_replacement.sql
sqlite> .tables
# Should see: config_backups, disk_replacements, disk_actions
```

### Issue: "Permission denied" when creating backup

**Symptoms:**
```
Failed to create backup directory
```

**Solution:**
```bash
sudo mkdir -p /var/backups/dplaneos
sudo chown www-data:www-data /var/backups/dplaneos
sudo chmod 700 /var/backups/dplaneos
```

### Issue: OpenSSL not found

**Symptoms:**
```
openssl: command not found
```

**Solution:**
```bash
sudo apt-get update
sudo apt-get install -y openssl

# Verify
openssl version
```

### Issue: SMART data not showing

**Symptoms:**
Disk replacement wizard doesn't show serial numbers

**Solution:**
```bash
sudo apt-get install -y smartmontools

# Test
sudo smartctl -i /dev/sda
```

### Issue: Backup creation hangs

**Symptoms:**
Spinner keeps spinning forever

**Solution:**
```bash
# Check Apache error log
sudo tail -f /var/log/apache2/error.log

# Check PHP execution time
php -i | grep max_execution_time

# Increase if needed (in /etc/php/8.2/apache2/php.ini)
max_execution_time = 300

# Restart Apache
sudo systemctl restart apache2
```

### Issue: Can't access backup.php

**Symptoms:**
```
404 Not Found
```

**Solution:**
```bash
# Verify file exists
ls -la /var/www/dplaneos/api/backup.php

# Check permissions
sudo chown www-data:www-data /var/www/dplaneos/api/backup.php
sudo chmod 644 /var/www/dplaneos/api/backup.php

# Check Apache config includes api/
cat /etc/apache2/sites-available/dplaneos.conf
```

---

## Database Schema Changes

The migration adds these tables:

```sql
-- Config backups tracking
CREATE TABLE config_backups (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    filename TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    size INTEGER NOT NULL,
    metadata TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    notes TEXT
);

-- Disk replacement tracking
CREATE TABLE disk_replacements (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    pool TEXT NOT NULL,
    old_device TEXT NOT NULL,
    new_device TEXT NOT NULL,
    started_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    completed_at DATETIME,
    status TEXT CHECK(status IN ('resilvering', 'completed', 'failed')),
    notes TEXT
);

-- Disk action audit log
CREATE TABLE disk_actions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    pool TEXT NOT NULL,
    device TEXT NOT NULL,
    action TEXT CHECK(action IN ('offline', 'online', 'replace', 'remove', 'attach')),
    details TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- System settings key-value store
CREATE TABLE system_settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

To verify migration:

```bash
sudo sqlite3 /var/www/dplaneos/database.sqlite "SELECT sql FROM sqlite_master WHERE name='config_backups';"
```

---

## Performance Considerations

### Backup Creation
- **First backup**: ~30 seconds (includes tar, gzip, encrypt)
- **Subsequent backups**: ~10-20 seconds
- **Disk space**: 5-50 MB per backup (varies with config)
- **CPU impact**: Low (mostly compression)

### Disk Replacement
- **Wizard overhead**: Negligible
- **Resilver speed**: Same as CLI (no performance change)
- **Monitoring**: 5-second polling (minimal CPU)

---

## Security Notes

### Password Security
- Backup passwords are **cryptographically random** (32 hex chars)
- Passwords shown **only once** - MUST be saved
- Password hashes stored in database (bcrypt)
- No password recovery mechanism (by design)

### File Permissions
```
/var/backups/dplaneos/              700 (drwx------)  www-data
/var/backups/dplaneos/*.tar.gz.enc  600 (-rw-------)  www-data
/var/www/dplaneos/api/backup.php    644 (-rw-r--r--)  www-data
```

### Encryption
- Algorithm: AES-256-CBC
- Key derivation: PBKDF2
- Salt: Randomly generated per backup
- Checksum: SHA-256 for integrity

---

## Support

If you encounter issues:

1. **Check logs:**
   - Apache: `/var/log/apache2/error.log`
   - Backup: `/var/log/dplaneos-backup.log`
   - System: `journalctl -u apache2`

2. **Verify permissions:**
   ```bash
   ls -la /var/www/dplaneos/api/
   ls -la /var/backups/dplaneos/
   ```

3. **Test APIs manually:**
   ```bash
   curl -X POST http://localhost/api/backup.php?action=create
   curl http://localhost/api/disk-replacement.php?action=status
   ```

4. **GitHub Issues:**
   https://github.com/your-org/dplaneos/issues

---

## Next Steps

After successful upgrade:

1. **Create first backup** and store password securely
2. **Test restore** on test system (if available)
3. **Schedule automated backups** for peace of mind
4. **Review disk health** regularly
5. **Update documentation** for your team

---

**Congratulations on upgrading to v1.13.0!** 🎉

You now have enterprise-grade disaster recovery capabilities.
