# D-PlaneOS

**A transparent, Docker-native NAS operating system built on ZFS**

Version 1.9.0 | Status: **POWER USER READY** ⚡

---

## Philosophy

- **Docker Compose as First-Class Citizen** - No app store, no templates. You control your containers.
- **Transparency over Abstraction** - Native ZFS and Docker commands, not hidden behind layers
- **Security from Day One** - Enterprise-grade privilege separation and command injection protection
- **Sleek Design + Lots of Power** - Clean UI that never gets in the way, with professional capabilities underneath

## Features

### Licence Change and Critical Bug Fixes (v1.9.0)
- license: moving to PolyForm Noncommercial
- auth.php - PHP Parse Error & Duplicate Code (CRITICAL)
- Removed duplicate/orphaned code blocks (lines 243-309)

### File Management (New in v1.8.0) 📁
- **Complete File Browser**: Upload, download, preview text files
- **Folder Operations**: Create, delete, rename, move, copy folders
- **Search Functionality**: Find files across all datasets
- **Permission Management**: chmod/chown directly from UI
- **Security**: Restricted to ZFS mountpoints only

### ZFS Native Encryption (New in v1.8.0) 🔐
- **AES-256-GCM Encryption**: Enterprise-grade encryption at rest
- **One-Click Dataset Encryption**: Checkbox during dataset creation
- **Key Management**: Load/unload encryption keys from UI
- **Password Management**: Change encryption passwords without data loss
- **Bulk Key Loading**: Unlock all datasets with master password
- **Boot-Time Prompts**: UI notification for locked datasets
- **RMA Protection**: Stolen or returned hardware is unreadable

### Service Control (New in v1.8.0) ⚙️
- **Systemd Integration**: Start/stop/restart all system services
- **Service Monitoring**: Real-time status and resource usage
- **Boot Persistence**: Enable/disable services at boot
- **Service Logs**: View last N lines of service logs
- **Managed Services**: SMB, NFS, SSH, Docker, Fail2ban, ZFS services

### Real-time Monitoring (New in v1.8.0) 📊
- **CPU Monitoring**: Per-core usage and load averages
- **Memory Tracking**: Real-time RAM usage with caching stats
- **Network Stats**: Per-interface bandwidth and packet stats
- **Disk I/O**: Read/write operations per device
- **Process Monitor**: Top processes by CPU/memory
- **System Info**: Hostname, uptime, kernel, OS distribution

### Critical System Protection (v1.7.0) 🔌
- **UPS/USV Monitoring**: Full integration with Network UPS Tools (NUT)
- **Battery Status**: Real-time charge, runtime, load, voltage monitoring
- **Auto-Shutdown**: Configurable graceful shutdown on low battery
- **Status Alerts**: Notifications for battery power and low battery conditions
- **Multi-UPS Support**: Monitor multiple UPS devices simultaneously

### Data Protection & Recovery (v1.7.0) 🕒
- **Automatic Snapshots**: Schedule hourly, daily, weekly, monthly snapshots
- **Retention Policies**: Smart cleanup - keep 7 daily, 4 weekly, 12 monthly
- **One-Click Snapshots**: Manual snapshot creation on demand
- **Snapshot Browser**: View all snapshots with creation dates and sizes
- **Schedule Management**: Enable/disable schedules without deletion
- **Protection Against**: Ransomware, accidental deletion, corruption

### System Diagnostics (v1.7.0) 📜
- **System Log Viewer**: View journalctl logs directly in browser
- **Service Logs**: Monitor SMB, NFS, Docker, NUT, and more
- **D-PlaneOS Audit Log**: Complete user action history with timestamps
- **ZFS Event Log**: Track all ZFS events and operations
- **Docker Container Logs**: Debug containers without SSH access

### Storage Management
- **ZFS Pools**: Create, destroy, scrub, monitor health
- **Pool Expansion**: Add VDEVs, replace disks, convert stripes to mirrors
- **Datasets**: Create, destroy, snapshot, bulk operations
- **SMART Monitoring**: Disk health tracking with historical data
- **Scheduled Scrubs**: Automatic weekly/monthly pool verification

### Disk Health Monitoring (v1.6.0)
- **Comprehensive Dashboard**: Real-time health overview of all disks
- **SMART Data**: Temperature, power-on hours, reallocated sectors
- **Status Tracking**: Healthy, Warning, Critical, Failing states
- **SMART Tests**: Run short/long tests directly from UI
- **Maintenance Log**: Complete audit trail of all disk actions
- **Replacement Tracking**: Log disk replacements with serial numbers

### System Notifications (v1.6.0)
- **Notification Center**: Elegant slide-out panel with real-time updates
- **Priority Levels**: Low, Normal, High, Critical alerts
- **Categories**: Disk, Pool, System, Replication, Quota
- **Auto-Polling**: Check for new notifications every 30 seconds
- **Smart Cleanup**: Auto-dismiss old notifications

### Network Shares (v1.5.0)
- **SMB Shares**: Guest access, authentication, granular permissions
- **NFS Shares**: Network restrictions, sync modes, export management
- **User Management**: SMB user accounts via integrated UI (v1.7.0)
- **Live Status**: Real-time share status monitoring
- **User Quotas** (v1.5.1): Per-user storage limits with visual usage tracking

### Cloud Sync (v1.5.0)
- **70+ Cloud Providers**: Full rclone backend support (S3, Google Drive, Dropbox, B2, etc.)
- **Flexible Tasks**: Push/pull, sync/copy/move operations
- **Progress Tracking**: Real-time sync status and logs
- **Any Cloud**: JSON-based config supports all rclone features

### Container Management
- **Native Docker**: Direct docker-compose.yml deployment
- **Full Control**: Start, stop, restart, remove containers
- **Live Logs**: Container log streaming in UI

### Data Protection
- **ZFS Replication**: Send/receive to remote hosts via SSH
- **Progress Tracking**: Real-time replication status
- **Automatic Alerts**: Webhook notifications on failures

### Monitoring & Alerts
- **System Metrics**: CPU, memory, load, disk usage
- **Historical Analytics**: Time-series charts with customizable ranges
- **Webhook Alerts**: Discord, Telegram, or custom endpoints
- **Comprehensive Audit Log**: All operations tracked

### Security
- **Privilege Separation**: Enhanced sudoers with least-privilege model
- **Command Injection Protection**: Active pattern blocking and validation
- **Database Integrity**: Automatic consistency checks
- **Session Management**: 30-minute timeout, CSRF protection
- **Rate Limiting**: Login attempt throttling

## Requirements

- **OS**: Ubuntu 22.04+ or Debian 12+
- **RAM**: 4GB minimum (2GB works but slower)
- **Storage**: ZFS-capable system (native or DKMS)
- **Access**: Root/sudo privileges

## Installation

```bash
tar -xzf dplaneos-v1.5.0.tar.gz
cd dplaneos-v1.5.0
sudo bash install.sh
```

Installation takes ~2 minutes and automatically installs:
- ZFS utilities
- Docker & Docker Compose
- PHP 8.2 (FPM + SQLite)
- Nginx web server
- Samba & NFS servers
- Rclone
- SMART monitoring tools

## First Login

1. Navigate to `http://your-server-ip`
2. Login with:
   - **Username**: `admin`
   - **Password**: `admin`
3. **⚠️ CHANGE PASSWORD IMMEDIATELY** via Settings

## Upgrade from v1.4.x

```bash
tar -xzf dplaneos-v1.5.0.tar.gz
cd dplaneos-v1.5.0
sudo bash install.sh
```

- Zero data loss
- Automatic database migration
- ~30 seconds downtime
- 100% backward compatible

## Documentation

- **[CHANGELOG.md](CHANGELOG.md)** - Complete version history
- **[SECURITY.md](SECURITY.md)** - Security model and procedures
- **[THREAT-MODEL.md](THREAT-MODEL.md)** - Security architecture analysis
- **[RECOVERY.md](RECOVERY.md)** - Disaster recovery playbook

## What D-PlaneOS IS

- Single-user NAS operating system
- ZFS-focused storage platform
- Docker-first container runtime
- Self-hosted, open-source

## What D-PlaneOS IS NOT

- ❌ Multi-tenant system (no RBAC)
- ❌ App marketplace (manual Docker Compose)
- ❌ High-availability cluster
- ❌ Enterprise compliance certified

## Security Recommendations

1. **Change default password** immediately after installation
2. **Use HTTPS** - Deploy behind Caddy or nginx reverse proxy
3. **Firewall** - Restrict access to trusted networks only
4. **Keep updated** - Monitor releases for security patches

## Architecture

- **Backend**: PHP 8.2 with SQLite database
- **Frontend**: Vanilla JavaScript (no frameworks)
- **Storage**: Native ZFS commands
- **Containers**: Docker Engine (no Kubernetes)
- **Web Server**: Nginx with PHP-FPM

## Browser Support

- Chrome/Edge 90+
- Firefox 88+
- Safari 14+
- Mobile responsive (768px+)

## Performance

- Tested on 4GB RAM systems
- SSD recommended for database
- HDD fine for ZFS pools
- ~50MB memory footprint for dashboard

## Support

- **Issues**: [GitHub Issues](https://github.com/yourusername/dplaneos/issues)
- **Discussions**: [GitHub Discussions](https://github.com/yourusername/dplaneos/discussions)
- **Community**: See community section in docs

## Contributing

Contributions welcome! Please read [CONTRIBUTING.md](CONTRIBUTING.md) before submitting PRs.

Key areas for contribution:
- Bug fixes and testing
- Documentation improvements
- Feature enhancements (aligned with project philosophy)
- Translations

## License

[MIT License](LICENSE) - See LICENSE file for details

## Credits

Built with:
- [ZFS on Linux](https://github.com/openzfs/zfs)
- [Docker](https://www.docker.com/)
- [Rclone](https://rclone.org/)
- [Samba](https://www.samba.org/)

---

**⭐ If you find D-PlaneOS useful, please star the repository!**

Made with transparency by the community, for the community.
