# D-PlaneOS v1.13.0 Release Notes
## "Disaster Recovery Edition"
### Release Date: January 31, 2026

---

## 🎯 Overview

Version 1.13.0 introduces two **enterprise-critical** features that transform D-PlaneOS from an expert-focused NAS into a **truly production-ready system** with complete disaster recovery capabilities:

1. **Config Backup & Restore**: One-click encrypted system configuration backups
2. **Disk Replacement Wizard**: UI-guided hard drive replacement with live resilver tracking

These features address the most important gaps for **long-term system reliability** and **non-CLI user accessibility**.

---

## 🚀 What's New

### 1. Config Backup & Restore System

**The Problem:** 
In v1.12.0, if your server died, recovering your exact configuration (users, shares, Docker apps, settings) required manual reconstruction or deep CLI knowledge.

**The Solution:**
A complete backup/restore system that creates encrypted, portable configuration archives.

#### Features:
- ✅ **One-Click Backup**: Single button creates encrypted `.tar.gz.enc` archive
- ✅ **Comprehensive Scope**: Backs up EVERYTHING except actual data files:
  - All user accounts & passwords
  - Docker container configurations  
  - SMB/NFS share settings
  - SSL certificates
  - Scheduled tasks & cron jobs
  - App Store repository settings
  - System preferences

- ✅ **Military-Grade Encryption**: AES-256-CBC with PBKDF2 key derivation
- ✅ **Automated Scheduling**: Set it and forget it - daily/weekly/monthly backups
- ✅ **One-Click Restore**: Upload backup file + password = instant recovery
- ✅ **Metadata Tracking**: Each backup includes:
  - D-PlaneOS version
  - Timestamp
  - Installed applications
  - ZFS pool list
  - System hardware info

#### How It Works:

**Creating a Backup:**
1. Navigate to **Settings → Backup & Restore**
2. Click **"Create Backup"**
3. System collects all configuration files
4. Creates tar.gz archive
5. Encrypts with AES-256-CBC
6. Generates unique password (shown once!)
7. Stores encrypted backup in `/var/backups/dplaneos/`

**Password Management:**
- Each backup has a **unique 32-character password**
- Password is shown ONCE in a modal - **you MUST save it!**
- Without the password, backup is useless (unbreakable encryption)
- Password hash is stored in DB for reference (not plaintext)

**Restoring a Backup:**
1. Navigate to **Settings → Backup & Restore**
2. Upload your `.tar.gz.enc` file
3. Enter the backup password
4. System automatically:
   - Creates pre-restore backup of current config
   - Decrypts archive
   - Verifies checksums
   - Restores database
   - Restores all config files
   - Restarts services
5. System reboots with restored configuration

**Automated Backups:**
- Schedule via **Settings → Backup & Restore → Schedule**
- Options: Daily, Weekly, Monthly
- Set time (e.g., "2:00 AM")
- Auto-cleanup: Delete backups older than X days
- Email notifications (optional)
- Runs via `/etc/cron.d/dplaneos-backup`

#### API Endpoints:
```bash
POST /api/backup.php?action=create      # Create new backup
GET  /api/backup.php?action=list        # List all backups
GET  /api/backup.php?action=download&file=...  # Download backup
POST /api/backup.php?action=restore     # Restore from backup
POST /api/backup.php?action=delete&file=...    # Delete backup
POST /api/backup.php?action=schedule    # Configure auto-backup
```

#### Security Features:
- ✅ Admin-only access
- ✅ Filename validation (prevent path traversal)
- ✅ Checksum verification (SHA-256)
- ✅ Encrypted at rest
- ✅ Secure password generation (cryptographically random)

---

### 2. Disk Replacement Wizard

**The Problem:**
When a hard drive fails in v1.12.0, users had to:
1. SSH into server
2. Run `zpool status` to identify failed disk
3. Run `zpool offline tank /dev/sdX`
4. Physically swap disk
5. Run `zpool replace tank /dev/sdX /dev/sdY`
6. Manually check resilver progress

**The Solution:**
A **5-step guided wizard** that walks you through disk replacement with live progress tracking.

#### The 5-Step Process:

**Step 1: Identify Failed Disk**
- Wizard scans all ZFS pools
- Automatically detects FAULTED/UNAVAIL disks
- Shows:
  - Device path (`/dev/sdc`)
  - Error counts (read/write/checksum)
  - SMART data (serial number, model, capacity)
- Click **"Select This Disk"** to proceed

**Step 2: Take Disk Offline**
- System explains redundancy impact
- Shows exact command that will run:
  ```bash
  zpool offline tank /dev/sdc
  ```
- Click **"Take Disk Offline"**
- System executes command
- Logs action to database

**Step 3: Install New Disk**
- Instructions for physical disk replacement
- Click **"Scan for New Disks"**
- System rescans SCSI bus
- Lists all available disks not in use by ZFS
- Shows: Device, size, serial number
- Click **"Select"** on replacement disk

**Step 4: Replace & Resilver**
- Confirmation screen shows:
  - Pool name
  - Old disk (`/dev/sdc`)
  - New disk (`/dev/sde`)
- Click **"Start Replacement"**
- System runs: `zpool replace tank /dev/sdc /dev/sde`
- **Live Resilver Progress** appears:
  - Progress bar (67% complete)
  - Data scanned (1.22 TB)
  - Data remaining (600 GB)
  - Transfer speed (92 MB/s)
  - Time remaining (2h 14m)
- Updates every 5 seconds

**Step 5: Complete**
- Success screen with summary
- Pool back to ONLINE status
- Full audit log of replacement
- Click **"Finish"** to close wizard

#### Features:
- ✅ **Automatic Failed Disk Detection**: Scans `zpool status` output
- ✅ **SMART Data Integration**: Uses `smartctl` to show disk details
- ✅ **Safe Offline Procedure**: Validates pool state before offlining
- ✅ **New Disk Auto-Detection**: Rescans SCSI bus to find replacements
- ✅ **Live Resilver Tracking**: Real-time progress without CLI
- ✅ **Action Logging**: Complete audit trail in `disk_actions` table
- ✅ **Email Notifications**: Alert when resilver completes (optional)

#### API Endpoints:
```bash
GET  /api/disk-replacement.php?action=status              # Get pool status
GET  /api/disk-replacement.php?action=identify-failed&pool=tank  # Find failed disks
POST /api/disk-replacement.php?action=offline             # Offline disk
GET  /api/disk-replacement.php?action=scan-new            # Scan for new disks
POST /api/disk-replacement.php?action=replace             # Start replacement
GET  /api/disk-replacement.php?action=resilver-progress&pool=tank  # Get progress
POST /api/disk-replacement.php?action=complete            # Mark complete
```

#### Database Tracking:
```sql
disk_replacements:
  - id, pool, old_device, new_device
  - started_at, completed_at, status
  
disk_actions:
  - id, pool, device, action
  - details, created_at
```

---

## 📦 Installation

### New Installation (v1.13.0)
```bash
wget https://github.com/your-org/dplaneos/releases/download/v1.13.0/dplaneos-v1.13.0.tar.gz
tar xzf dplaneos-v1.13.0.tar.gz
cd dplaneos-v1.13.0
sudo ./install.sh
```

### Upgrade from v1.12.0
```bash
wget https://github.com/your-org/dplaneos/releases/download/v1.13.0/dplaneos-v1.13.0.tar.gz
tar xzf dplaneos-v1.13.0.tar.gz
cd dplaneos-v1.13.0
sudo ./scripts/upgrade-to-v1.13.0.sh
```

**Upgrade is fully automated:**
- Creates pre-upgrade database backup
- Installs new API files
- Runs database migration
- Sets up backup directory
- Installs cron scripts
- Updates version to 1.13.0

**No downtime required!**

---

## 🛡️ Security Considerations

### Config Backups
- **Password Storage**: Password hashes stored in database, plaintext shown once
- **Encryption**: AES-256-CBC with PBKDF2 (industry standard)
- **File Permissions**: `/var/backups/dplaneos` is 700 (owner-only)
- **Access Control**: Admin-only via auth.php
- **Validation**: Filename regex prevents path traversal

### Disk Replacement
- **Admin-Only**: Requires authenticated admin session
- **Validation**: Checks disk is not in use before replacement
- **Logging**: All actions logged with timestamp & details
- **Safe Defaults**: Won't offline disk if pool already degraded

---

## 📊 Performance Impact

### Config Backup
- **Creation Time**: 10-30 seconds (depends on config size)
- **File Size**: Typically 5-50 MB compressed+encrypted
- **CPU Impact**: Minimal (compression + encryption)
- **Disk I/O**: Low (sequential writes)

### Disk Replacement
- **Wizard Overhead**: Negligible
- **Resilver Speed**: Same as CLI (depends on disk speed)
- **Monitoring Polling**: Every 5 seconds (low impact)

---

## 🔄 Database Changes

New tables created automatically on upgrade:

```sql
CREATE TABLE config_backups (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    filename TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    size INTEGER NOT NULL,
    metadata TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE disk_replacements (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    pool TEXT NOT NULL,
    old_device TEXT NOT NULL,
    new_device TEXT NOT NULL,
    started_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    completed_at DATETIME,
    status TEXT CHECK(status IN ('resilvering', 'completed', 'failed'))
);

CREATE TABLE disk_actions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    pool TEXT NOT NULL,
    device TEXT NOT NULL,
    action TEXT CHECK(action IN ('offline', 'online', 'replace', 'remove', 'attach')),
    details TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE system_settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

---

## 📝 Known Issues / Limitations

### Config Backup
- ⚠️ Backups do **NOT** include actual data files (by design)
  - Use ZFS snapshots for data backup
  - Config backups are for system recovery only
- ⚠️ Password is shown **ONLY ONCE**
  - If lost, backup is unrecoverable
  - No password recovery mechanism (security feature)
- ⚠️ Restore requires system restart
  - Plan for brief downtime

### Disk Replacement
- ⚠️ Requires physical server access
  - Cannot hot-swap over network
- ⚠️ Resilver can take hours/days
  - Depends on disk size and data amount
- ⚠️ Pool runs degraded during replacement
  - No redundancy until resilver completes

---

## 🎯 Use Cases

### Config Backup

**Scenario 1: Disaster Recovery**
Your server's boot drive dies. You install D-PlaneOS on new hardware, restore your backup, and you're back online with all settings intact.

**Scenario 2: Migration**
Moving to new server. Backup old system, install D-PlaneOS on new server, restore backup. Done.

**Scenario 3: Testing**
Want to test risky changes? Create backup first. If things break, restore to known-good state.

**Scenario 4: Compliance**
Automated daily backups ensure you can prove configuration history for audits.

### Disk Replacement

**Scenario 1: Failed Drive**
`zpool status` shows FAULTED disk. Launch wizard, follow 5 steps, done. No CLI needed.

**Scenario 2: Proactive Replacement**
SMART reports increasing error rate. Replace disk before it fails completely.

**Scenario 3: Capacity Upgrade**
Replace 4TB disks with 8TB disks one-by-one to grow pool.

**Scenario 4: Training**
New IT staff can replace disks without ZFS expertise. Wizard ensures correct procedure.

---

## 🚀 Future Enhancements (v1.14.0+)

Based on this release, planned features:

- **Cloud Backup Integration**: Upload config backups to S3/Backblaze
- **Incremental Backups**: Only backup changed configs
- **Restore Preview**: See what's in backup before restoring
- **Multi-Disk Replacement**: Replace multiple failed disks in parallel
- **Resilver Scheduling**: Pause/resume resilver during business hours
- **Email Notifications**: Alert on backup completion/failure
- **Webhook Integration**: Trigger external systems on events

---

## 📚 Documentation

- **Config Backup Guide**: `/docs/CONFIG_BACKUP_SYSTEM.md`
- **Disk Replacement Guide**: `/docs/DISK_REPLACEMENT_GUIDE.md`
- **API Reference**: `/docs/API.md`
- **Upgrade Guide**: `/docs/UPGRADE_GUIDE.md`

---

## 🙏 Credits

This release addresses feedback from production users who needed:
1. Easy disaster recovery for non-CLI users
2. Guided disk replacement process
3. Automated configuration backups

Special thanks to the D-PlaneOS community for feature requests and testing!

---

## 📞 Support

- **Issues**: https://github.com/your-org/dplaneos/issues
- **Discussions**: https://github.com/your-org/dplaneos/discussions
- **Documentation**: https://dplaneos.d-net.me/docs

---

**Download:** https://github.com/your-org/dplaneos/releases/tag/v1.13.0

**SHA256 Checksum:** *(generated during packaging)*

---

**Happy Self-Hosting! 🚀**
