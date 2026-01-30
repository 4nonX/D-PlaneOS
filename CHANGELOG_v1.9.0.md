# D-PlaneOS v1.9.0 - Critical Security & Bug Fixes Release

**Release Date:** 2026-01-30
**Previous Version:** v1.8.0

---

## 🔒 CRITICAL SECURITY FIXES

### 1. auth.php - PHP Parse Error & Duplicate Code (CRITICAL)

**Issue:** File contained duplicate function body causing PHP fatal parse error
- Lines 243-276: Duplicate function body
- Lines 241-309: Orphaned code block
- Result: 56 closing braces vs 55 opening braces
- **Impact:** System completely non-functional - PHP crashes on every request

**Fix Applied:**
- Removed duplicate/orphaned code blocks (lines 243-309)
- Corrected brace matching (56→39 matching pairs)
- File now passes PHP syntax validation

**Severity:** ⚠️ CRITICAL - System unusable without this fix

---

### 2. security.php - Incomplete HTTPS Redirect (HIGH)

**Issue:** Commented-out HTTPS redirect code had orphaned closing braces
```php
# if (!isset($_SERVER['HTTPS']) || $_SERVER['HTTPS'] !== 'on') {
#     if ($_SERVER['SERVER_NAME'] !== 'localhost' ...
    }  // ← Orphaned braces not commented
}      // ← Orphaned braces not commented
```

**Fix Applied:**
- Removed incomplete HTTPS redirect block entirely
- Eliminated orphaned closing braces
- File now passes PHP syntax validation

**Severity:** ⚠️ HIGH - PHP parse errors, system may fail to start

---

### 3. command-broker.php - Missing Shell Argument Escaping (HIGH)

**Issue:** Command arguments passed to vsprintf() without shell escaping
```php
// BEFORE (vulnerable):
$cmd = 'sudo ' . vsprintf($spec['cmd'], $args) . ' 2>&1';

// AFTER (secure):
$escapedArgs = array_map('escapeshellarg', $args);
$cmd = 'sudo ' . vsprintf($spec['cmd'], $escapedArgs) . ' 2>&1';
```

**Fix Applied:**
- Added `escapeshellarg()` to all command arguments before sprintf
- Strengthens defense against command injection attacks

**Severity:** ⚠️ HIGH - Potential command injection vulnerability

---

## 🗄️ DATABASE SCHEMA IMPROVEMENTS

### schema.sql - Comprehensive Corrections

**1. Foreign Key Reference Fixed**
- **BEFORE:** `FOREIGN KEY (disk_path) REFERENCES disk_tracking(disk_path)` (TEXT reference)
- **AFTER:** `FOREIGN KEY (disk_id) REFERENCES disk_tracking(id)` (INTEGER reference)
- **Impact:** More efficient and safer referential integrity

**2. Data Validation Added (40+ CHECK constraints)**
```sql
-- Enum validation
share_type TEXT NOT NULL CHECK(share_type IN ('smb', 'nfs'))
direction TEXT NOT NULL CHECK(direction IN ('push', 'pull'))
severity TEXT NOT NULL CHECK(severity IN ('info', 'warning', 'error', 'critical'))

// Range validation
battery_charge INTEGER CHECK(battery_charge BETWEEN 0 AND 100)
priority INTEGER DEFAULT 0 CHECK(priority BETWEEN 0 AND 3)
keep_count INTEGER NOT NULL CHECK(keep_count > 0)

// Boolean validation
enabled INTEGER DEFAULT 1 CHECK(enabled IN (0, 1))
```

**3. Automatic Timestamp Updates (4 triggers added)**
- `update_shares_timestamp`
- `update_smb_users_timestamp`
- `update_user_quotas_timestamp`
- `update_rclone_remotes_timestamp`

**4. Foreign Key Behaviors Improved**
```sql
// Preserve audit trail when users are deleted
FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE SET NULL

// Keep snapshot history when schedule is deleted
FOREIGN KEY (schedule_id) REFERENCES snapshot_schedules(id) ON DELETE SET NULL
```

**5. Performance Indexes Added**
- `idx_disk_maintenance_disk` - Disk maintenance lookups
- `idx_notifications_type` - Notification filtering

---

## 🔧 INSTALLATION FIXES

### sudoers.enhanced - Invalid Syntax Removed (CRITICAL)

**Issue:** Invalid `!NOPASSWD:` syntax causing installation failure
```bash
# Lines 81-91 had invalid syntax:
www-data ALL=(ALL) !NOPASSWD: /usr/bin/rm  # ← Invalid syntax
www-data ALL=(ALL) !NOPASSWD: /usr/bin/chmod
# ... 9 more invalid lines
```

**Fix Applied:**
- Removed 11 lines with invalid `!NOPASSWD:` syntax
- Sudoers file now passes `visudo -c` validation

**Severity:** ⚠️ CRITICAL - Installation fails on Debian Trixie/Ubuntu 24+

---

## ✅ VALIDATION & TESTING

All fixes have been validated:
- ✅ PHP syntax check passed on all 30 PHP files
- ✅ Database schema validated with SQLite3
- ✅ Sudoers file validated with `visudo -c`
- ✅ Installation tested on Raspberry Pi OS (Debian Trixie)
- ✅ System boots and runs successfully

---

## 📦 UPGRADE INSTRUCTIONS

### For Existing v1.8.0 Installations

**⚠️ BACKUP FIRST:**
```bash
# Backup database
sudo cp /var/dplane/database/dplane.db /var/dplane/database/dplane.db.backup-$(date +%Y%m%d)

# Backup web files
sudo tar -czf /root/dplane-backup-$(date +%Y%m%d).tar.gz /var/dplane/dashboard/
```

**Apply Update:**
```bash
# Stop services
sudo systemctl stop nginx php-fpm

# Apply code updates
sudo cp -r system/dashboard/* /var/dplane/dashboard/

# Apply sudoers fix (if needed)
sudo cp system/config/sudoers.enhanced /etc/sudoers.d/dplane-enhanced
sudo visudo -c -f /etc/sudoers.d/dplane-enhanced

# Restart services
sudo systemctl start php-fpm nginx

# Verify
curl -I http://localhost/
```

**Database Migration (Optional - Recommended for New Features):**
```bash
# The old schema will continue to work
# To apply improvements:
sudo sqlite3 /var/dplane/database/dplane.db < database/schema-migration-v1.9.0.sql
```

---

### For New Installations

Use the updated `install.sh` script - all fixes are automatically applied:

```bash
sudo bash install.sh
```

---

## 🔍 FILES CHANGED

**Critical Security Files:**
- `system/dashboard/includes/auth.php` - Parse error fixed
- `system/dashboard/includes/security.php` - Parse error fixed
- `system/dashboard/includes/command-broker.php` - Shell escaping added
- `system/config/sudoers.enhanced` - Invalid syntax removed

**Database:**
- `database/schema.sql` - 40+ improvements applied

**Documentation:**
- `VERSION` - Updated to 1.9.0
- `CHANGELOG_v1.9.0.md` - This file

---

## 🙏 ACKNOWLEDGMENTS

**Security Issue Reporter:**
- Reddit user who identified the flawed `execCommand()` function (auth.php)
- Prompted comprehensive security audit leading to these fixes

**Testing:**
- Raspberry Pi 5 (8GB) with Debian Trixie
- Confirmed working on fresh installation

---

## 📞 SUPPORT

**Issue Tracker:** https://github.com/4nonX/D-PlaneOS/issues
**Discussions:** https://github.com/4nonX/D-PlaneOS/discussions
**Website:** https://dplaneos.d-net.me/

---

## ⚖️ LICENSE

MIT License - See LICENSE file for details

---

**IMPORTANT:** If you are running v1.8.0, upgrade to v1.9.0 IMMEDIATELY. The system is non-functional in v1.8.0 due to PHP parse errors.
