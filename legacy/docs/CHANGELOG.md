# D-PlaneOS Changelog

## [1.13.0] - 2026-01-31

### 🎉 Major New Features

#### Config Backup & Restore System
- **One-click system backup**: Export entire configuration to encrypted archive
- **AES-256-CBC encryption**: Secure backups with unique password per archive
- **Comprehensive backup scope**:
  - SQLite database (all users, settings, apps)
  - Docker Compose files
  - Samba/NFS configurations
  - SSL certificates
  - Cron jobs & scheduled tasks
  - App Store repository settings
- **Automated scheduling**: Daily, weekly, or monthly backups via cron
- **Restore from backup**: Upload encrypted backup to restore configuration
- **Backup management**: List, download, delete old backups
- **Metadata tracking**: Each backup includes system version, installed apps, ZFS pools
- **Checksum verification**: SHA-256 checksums for data integrity

#### Disk Replacement Wizard
- **5-step guided process** for replacing failed disks
- **Automatic failed disk detection**: Identifies FAULTED/UNAVAIL disks
- **SMART data integration**: Shows disk serial, model, error counts
- **Safe offline procedure**: Takes disk offline before physical replacement
- **New disk scanning**: Auto-detects newly installed replacement disks
- **One-click replacement**: Initiates `zpool replace` with validation
- **Live resilver tracking**: Real-time progress monitoring with:
  - Percentage complete
  - Data scanned / remaining
  - Transfer speed
  - Estimated time remaining
- **Action logging**: Complete audit trail of all disk operations

### 🔧 Backend Improvements

#### New APIs
- `/api/backup.php`: Complete backup/restore management
  - `action=create`: Create encrypted backup
  - `action=list`: List all backups
  - `action=download`: Download specific backup
  - `action=restore`: Restore from uploaded backup
  - `action=delete`: Remove backup
  - `action=schedule`: Configure automated backups

- `/api/disk-replacement.php`: Disk replacement workflow
  - `action=status`: Get pool and disk status
  - `action=identify-failed`: Find failed disks with details
  - `action=offline`: Take disk offline
  - `action=scan-new`: Scan for new replacement disks
  - `action=replace`: Start disk replacement & resilver
  - `action=resilver-progress`: Monitor resilver progress
  - `action=complete`: Mark replacement complete

#### New Database Tables
```sql
config_backups         - Backup archive metadata
disk_replacements      - Disk replacement tracking
disk_actions          - Audit log of disk operations
system_settings       - Key-value settings storage
```

#### New Scripts
- `/scripts/auto-backup.php`: Automated backup via cron
- `/scripts/upgrade-to-v1.13.0.sh`: Upgrade script from v1.12.0

### 🎨 Frontend Enhancements

#### New JavaScript Modules
- `backup.js`: BackupManager class with full UI integration
  - Password modal with copy-to-clipboard
  - Backup table with metadata display
  - Upload & restore with progress
  - Scheduling interface

- `disk-replacement.js`: DiskReplacementWizard class
  - Step-by-step wizard modal
  - Pool status dashboard
  - Disk selection interfaces
  - Real-time resilver progress bar
  - Completion summary

#### New UI Pages
- **Settings → Backup & Restore**:
  - Create backup button
  - Backup list table
  - Restore from backup
  - Schedule automated backups
  
- **Storage → Disk Health** (new section):
  - Pool status cards
  - Failed disk alerts
  - Launch replacement wizard

### 📦 Dependencies Added
- OpenSSL (encryption/decryption)
- smartmontools (disk SMART data)

### 🔒 Security Enhancements
- Password hashing for stored backup passwords
- Filename validation to prevent path traversal
- Admin-only access to backup/restore functions
- Encrypted backup archives with AES-256-CBC
- SHA-256 checksum verification

### 🐛 Bug Fixes
- None (new feature release)

### 📊 Performance
- Backup creation: ~10-30 seconds (depending on configuration size)
- Resilver monitoring: 5-second polling interval
- Minimal impact on system resources

### 📚 Documentation
- Complete RELEASE_NOTES_v1.13.0.md
- Updated UPGRADE_GUIDE.md
- New CONFIG_BACKUP_SYSTEM.md
- New DISK_REPLACEMENT_GUIDE.md

### ⚠️ Breaking Changes
None - fully backward compatible with v1.12.0

### 🔄 Migration Notes
Database automatically migrates on upgrade. No manual intervention required.

Pre-upgrade backup created automatically at:
`/var/backups/dplaneos-pre-v1.13.0-[timestamp].sqlite`

### 📝 Notes
- Backup passwords are CRITICAL - store securely!
- Backups do NOT include actual data files (use ZFS snapshots for data)
- Automated backups require cron configuration
- Disk replacement requires physical access to server

---

## [1.12.0] - 2026-01-31
*(Previous version - App Store release)*

---

## [1.11.0] - 2026-01-31
*(Previous version - ZFS Wizard & Live Scrub)*

---

**Full changelog:** https://github.com/your-org/dplaneos/blob/main/CHANGELOG.md
