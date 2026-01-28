<?php
require_once __DIR__ . '/includes/auth.php';
requireAuth();
$user = getCurrentUser();
?>
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>D-PlaneOS Dashboard</title>
    <link rel="stylesheet" href="/assets/css/main.css">
</head>
<body>
    <div class="app">
        <nav class="topbar">
            <div class="topbar-left">
                <h1>D-PlaneOS</h1>
            </div>
            <div class="topbar-center">
                <div id="nav-container">
                    <button class="nav-btn active" data-page="dashboard" draggable="true">Dashboard</button>
                    <button class="nav-btn" data-page="storage" draggable="true">Storage</button>
                    <button class="nav-btn" data-page="datasets" draggable="true">Datasets</button>
                    <button class="nav-btn" data-page="encryption" draggable="true">🔐 Encryption</button>
                    <button class="nav-btn" data-page="files" draggable="true">Files</button>
                    <button class="nav-btn" data-page="shares" draggable="true">Shares</button>
                    <button class="nav-btn" data-page="disks" draggable="true">Disk Health</button>
                    <button class="nav-btn" data-page="snapshots" draggable="true">Snapshots</button>
                    <button class="nav-btn" data-page="ups" draggable="true">UPS Monitor</button>
                    <button class="nav-btn" data-page="users" draggable="true">Users</button>
                    <button class="nav-btn" data-page="containers" draggable="true">Containers</button>
                    <button class="nav-btn" data-page="services" draggable="true">Services</button>
                    <button class="nav-btn" data-page="monitoring" draggable="true">Monitoring</button>
                    <button class="nav-btn" data-page="logs" draggable="true">System Logs</button>
                    <button class="nav-btn" data-page="rclone" draggable="true">Cloud Sync</button>
                    <button class="nav-btn" data-page="analytics" draggable="true">Analytics</button>
                    <button class="nav-btn" data-page="replication" draggable="true">Replication</button>
                    <button class="nav-btn" data-page="alerts" draggable="true">Alerts</button>
                </div>
            </div>
            <div class="topbar-right">
                <button class="notification-bell" onclick="toggleNotifications()" title="Notifications">
                    <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                        <path d="M18 8A6 6 0 0 0 6 8c0 7-3 9-3 9h18s-3-2-3-9"></path>
                        <path d="M13.73 21a2 2 0 0 1-3.46 0"></path>
                    </svg>
                    <span class="notification-badge" id="notification-count" style="display:none;">0</span>
                </button>
                <span class="username"><?= htmlspecialchars($user['username']) ?></span>
                <a href="/logout.php" class="logout-btn">Logout</a>
            </div>
        </nav>

        <div class="main-content">
            <div id="page-dashboard" class="page active">
                <h2>System Overview</h2>
                <div class="stats-grid">
                    <div class="stat-card">
                        <h3>CPU Usage</h3>
                        <div class="stat-value" id="cpu-usage">--</div>
                    </div>
                    <div class="stat-card">
                        <h3>Memory Usage</h3>
                        <div class="stat-value" id="memory-usage">--</div>
                    </div>
                    <div class="stat-card">
                        <h3>Uptime</h3>
                        <div class="stat-value" id="uptime">--</div>
                    </div>
                    <div class="stat-card">
                        <h3>Load Average</h3>
                        <div class="stat-value" id="load-avg">--</div>
                    </div>
                </div>
            </div>

            <div id="page-storage" class="page">
                <div class="page-header">
                    <h2>Storage Pools</h2>
                    <button class="btn-primary" onclick="showCreatePool()">Create Pool</button>
                </div>
                <div id="pools-list"></div>
            </div>

            <div id="page-datasets" class="page">
                <div class="page-header">
                    <h2>Datasets</h2>
                    <button class="btn-primary" onclick="showCreateDataset()">Create Dataset</button>
                    <button class="btn-secondary" onclick="showBulkSnapshot()">Bulk Snapshot</button>
                </div>
                <div id="datasets-list"></div>
            </div>

            <div id="page-files" class="page">
                <div class="page-header">
                    <h2>File Browser</h2>
                    <div class="button-group">
                        <button class="btn-primary" onclick="createFolder()">New Folder</button>
                        <button class="btn-secondary" onclick="loadFiles()">Refresh</button>
                    </div>
                </div>
                <div id="files-breadcrumb" style="margin-bottom: 1rem; color: #667eea;">📁 /mnt</div>
                <div id="files-list"></div>
            </div>

            <div id="page-shares" class="page">
                <div class="page-header">
                    <h2>Network Shares</h2>
                    <div class="button-group">
                        <button class="btn-primary" onclick="showCreateShareModal()">Create Share</button>
                        <button class="btn-secondary" onclick="showAllQuotas()">All Quotas</button>
                    </div>
                </div>
                <div id="shares-list"></div>
            </div>

            <div id="page-disks" class="page">
                <div class="page-header">
                    <h2>Disk Health Monitoring</h2>
                    <button class="btn-secondary" onclick="refreshDiskHealth()">Refresh</button>
                </div>
                
                <div class="disk-health-summary" id="disk-health-summary">
                    <div class="health-stat-card">
                        <h3>Total Disks</h3>
                        <div class="stat-value" id="total-disks">--</div>
                    </div>
                    <div class="health-stat-card healthy">
                        <h3>Healthy</h3>
                        <div class="stat-value" id="healthy-disks">--</div>
                    </div>
                    <div class="health-stat-card warning">
                        <h3>Warning</h3>
                        <div class="stat-value" id="warning-disks">--</div>
                    </div>
                    <div class="health-stat-card critical">
                        <h3>Critical</h3>
                        <div class="stat-value" id="critical-disks">--</div>
                    </div>
                </div>
                
                <div id="disks-health-list"></div>
            </div>

            <div id="page-snapshots" class="page">
                <div class="page-header">
                    <h2>Automatic Snapshots</h2>
                    <button class="btn-primary" onclick="showCreateScheduleModal()">Create Schedule</button>
                </div>
                
                <h3 style="margin-bottom: 1rem; color: #667eea;">Active Schedules</h3>
                <div id="snapshot-schedules"></div>
                
                <div class="page-header" style="margin-top: 2rem;">
                    <h3>Recent Snapshots</h3>
                </div>
                <div id="snapshot-history"></div>
            </div>
                        <select id="snapshot-dataset-filter" onchange="loadSnapshotsForDataset(this.value)" class="form-select">
                            <option value="">Select dataset...</option>
                        </select>
                        <button class="btn-secondary" onclick="refreshSnapshots()">Refresh</button>
                    </div>
                </div>
                
                <div id="snapshots-list"></div>
            </div>

            <div id="page-ups" class="page">
                <div class="page-header">
                    <h2>UPS/USV Monitor</h2>
                    <button class="btn-secondary" onclick="refreshUPS()">Refresh</button>
                </div>
                
                <div class="ups-status-container" id="ups-status-container">
                    <div class="info-box">
                        Loading UPS status...
                    </div>
                </div>
                
                <div class="page-header" style="margin-top: 2rem;">
                    <h3>Shutdown Configuration</h3>
                </div>
                
                <div class="card">
                    <div class="card-content">
                        <p><strong>Automatic Shutdown Settings:</strong></p>
                        <p>System will initiate shutdown when:</p>
                        <ul>
                            <li>Battery level drops below: <strong>20%</strong></li>
                            <li>Runtime remaining is less than: <strong>5 minutes</strong></li>
                        </ul>
                        <button class="btn-secondary" onclick="configureUPSShutdown()">Configure</button>
                    </div>
                </div>
            </div>

            <div id="page-users" class="page">
                <div class="page-header">
                    <h2>User Management</h2>
                    <button class="btn-primary" onclick="showCreateUser()">Create User</button>
                </div>
                
                <div class="info-box">
                    <p><strong>User accounts provide:</strong></p>
                    <ul style="margin-left:1.5rem;margin-top:0.5rem;">
                        <li>Dashboard access (web login)</li>
                        <li>SMB/CIFS share access</li>
                        <li>NFS share access (if configured)</li>
                        <li>SSH access (if enabled)</li>
                    </ul>
                </div>
                
                <div id="users-list"></div>
            </div>

            <div id="page-logs" class="page">
                <div class="page-header">
                    <h2>System Logs</h2>
                    <div class="button-group">
                        <select id="log-type-select" onchange="loadLogs()" class="form-select">
                            <option value="system">System Log</option>
                            <option value="dplaneos">D-PlaneOS Audit</option>
                            <option value="zfs">ZFS Events</option>
                            <option value="service">Service Log</option>
                        </select>
                        <select id="log-service-select" onchange="loadLogs()" class="form-select" style="display:none;">
                            <option value="smbd">SMB Server</option>
                            <option value="nmbd">NetBIOS Server</option>
                            <option value="nfs-server">NFS Server</option>
                            <option value="docker">Docker</option>
                            <option value="nginx">Web Server</option>
                            <option value="nut-server">UPS Server</option>
                        </select>
                        <input type="number" id="log-lines" value="100" min="10" max="1000" class="form-input" style="width:100px;" onchange="loadLogs()">
                        <button class="btn-secondary" onclick="loadLogs()">Refresh</button>
                    </div>
                </div>
                
                <div class="log-viewer" id="log-viewer">
                    <div class="info-box">
                        Loading logs...
                    </div>
                </div>
            </div>

            <div id="page-rclone" class="page">
                <h2>Cloud Sync (Rclone)</h2>
                
                <h3>Remotes</h3>
                <div class="page-header">
                    <button class="btn-primary" onclick="showCreateRemoteModal()">Add Remote</button>
                </div>
                <div id="rclone-remotes-list"></div>
                
                <h3 style="margin-top: 2rem;">Sync Tasks</h3>
                <div class="page-header">
                    <button class="btn-primary" onclick="showCreateRcloneTaskModal()">Create Task</button>
                </div>
                <div id="rclone-tasks-list"></div>
            </div>

            <div id="page-containers" class="page">
                <div class="page-header">
                    <h2>Containers</h2>
                    <button class="btn-primary" onclick="showDeployContainer()">Deploy Container</button>
                </div>
                <div id="containers-list"></div>
            </div>

            <div id="page-services" class="page">
                <div class="page-header">
                    <h2>System Services</h2>
                    <button class="btn-secondary" onclick="loadServices()">Refresh</button>
                </div>
                <div id="services-list"></div>
            </div>

            <div id="page-monitoring" class="page">
                <div class="page-header">
                    <h2>Real-time Monitoring</h2>
                    <p style="color: #aaa; font-size: 0.9rem;">Auto-refreshes every 2 seconds</p>
                </div>
                <div style="display: grid; grid-template-columns: repeat(auto-fit, minmax(400px, 1fr)); gap: 1rem;">
                    <div class="card">
                        <div class="card-content" id="monitoring-cpu"></div>
                    </div>
                    <div class="card">
                        <div class="card-content" id="monitoring-memory"></div>
                    </div>
                    <div class="card">
                        <div class="card-content" id="monitoring-network"></div>
                    </div>
                    <div class="card" style="grid-column: span 2;">
                        <div class="card-content" id="monitoring-processes"></div>
                    </div>
                </div>
            </div>

            <div id="page-encryption" class="page">
                <div class="page-header">
                    <h2>🔐 ZFS Native Encryption</h2>
                    <div class="button-group">
                        <button class="btn-primary" onclick="showCreateEncryptedDataset()">Create Encrypted Dataset</button>
                        <button class="btn-success" onclick="loadAllKeys()">Unlock All</button>
                        <button class="btn-secondary" onclick="loadEncryptedDatasets()">Refresh</button>
                    </div>
                </div>
                
                <div class="info-box">
                    <h3>🛡️ Enterprise-Grade Data Protection</h3>
                    <p><strong>AES-256-GCM encryption</strong> protects your data at rest. Even if disks are stolen or sent for RMA, your data remains secure.</p>
                    <p><strong>Important:</strong> Without your password, encrypted data is PERMANENTLY inaccessible. Store passwords securely!</p>
                </div>
                
                <div id="encryption-list"></div>
            </div>

            <div id="page-analytics" class="page">
                <div class="page-header">
                    <h2>Historical Analytics</h2>
                    <select id="time-range" onchange="loadAnalytics()">
                        <option value="24">Last 24 Hours</option>
                        <option value="168">Last 7 Days</option>
                        <option value="720">Last 30 Days</option>
                    </select>
                </div>
                <div class="analytics-grid">
                    <div class="chart-card">
                        <h3>CPU Usage</h3>
                        <canvas id="chart-cpu" width="600" height="200"></canvas>
                    </div>
                    <div class="chart-card">
                        <h3>Memory Usage</h3>
                        <canvas id="chart-memory" width="600" height="200"></canvas>
                    </div>
                    <div class="chart-card">
                        <h3>Pool Usage</h3>
                        <canvas id="chart-pool" width="600" height="200"></canvas>
                    </div>
                    <div class="chart-card">
                        <h3>Disk Temperatures</h3>
                        <canvas id="chart-temp" width="600" height="200"></canvas>
                    </div>
                </div>
            </div>

            <div id="page-replication" class="page">
                <div class="page-header">
                    <h2>ZFS Replication</h2>
                    <button class="btn-primary" onclick="showCreateReplication()">New Task</button>
                </div>
                <div id="replication-list"></div>
            </div>

            <div id="page-alerts" class="page">
                <div class="page-header">
                    <h2>Alerts & Monitoring</h2>
                    <button class="btn-primary" onclick="configureAlerts()">Configure</button>
                    <button class="btn-secondary" onclick="checkAlerts()">Check Now</button>
                </div>
                <div id="alerts-content">
                    <h3>Alert History</h3>
                    <div id="alert-history"></div>
                </div>
            </div>
        </div>
    </div>

    <div id="modal" class="modal">
        <div class="modal-content">
            <span class="modal-close" onclick="closeModal()">&times;</span>
            <div id="modal-body"></div>
        </div>
    </div>

    <div id="notification-center" class="notification-center">
        <div class="notification-header">
            <h3>Notifications</h3>
            <div class="notification-actions">
                <button class="btn-link" onclick="markAllNotificationsRead()">Mark all read</button>
                <button class="notification-close" onclick="toggleNotifications()">&times;</button>
            </div>
        </div>
        <div class="notification-list" id="notification-list">
            <div class="notification-placeholder">
                <p>Loading notifications...</p>
            </div>
        </div>
    </div>

    <!-- Encryption Banner for Locked Datasets -->
    <div id="encryption-banner" style="display: none; position: fixed; top: 70px; left: 50%; transform: translateX(-50%); background: rgba(255, 152, 0, 0.15); border: 2px solid #FF9800; border-radius: 8px; padding: 1rem 2rem; z-index: 9999; box-shadow: 0 4px 20px rgba(0,0,0,0.5); backdrop-filter: blur(10px);">
        <strong>🔒 <span id="pending-keys-count">0</span> encrypted dataset(s) locked</strong>
        <button class="btn-sm btn-success" onclick="loadAllKeys()" style="margin-left: 1rem;">Unlock All</button>
        <button onclick="document.getElementById('encryption-banner').style.display='none'" style="background: none; border: none; color: #fff; margin-left: 1rem; cursor: pointer;">&times;</button>
    </div>

    <!-- Create Pool Modal -->
    <div id="modal-create-pool" class="modal">
        <div class="modal-content">
            <button class="modal-close" onclick="closeModal('modal-create-pool')">&times;</button>
            <h2>Create Storage Pool</h2>
            
            <form id="create-pool-form" onsubmit="event.preventDefault(); createPool();">
                <div class="form-group">
                    <label>Pool Name</label>
                    <input type="text" name="name" class="form-input" placeholder="tank" required pattern="[a-zA-Z0-9_-]+">
                    <p style="color: #aaa; font-size: 0.85rem; margin-top: 0.5rem;">Letters, numbers, dashes, and underscores only</p>
                </div>
                
                <div class="form-group">
                    <label>RAID Type</label>
                    <select name="raid" class="form-select" id="raid-type-select" onchange="updateRaidInfo()">
                        <option value="stripe">Stripe (No redundancy - Fast)</option>
                        <option value="mirror">Mirror (2+ disks - 50% capacity)</option>
                        <option value="raidz1" selected>RAIDZ1 (3+ disks - 1 disk parity)</option>
                        <option value="raidz2">RAIDZ2 (4+ disks - 2 disk parity)</option>
                        <option value="raidz3">RAIDZ3 (5+ disks - 3 disk parity)</option>
                    </select>
                    <p id="raid-info" style="color: #aaa; font-size: 0.85rem; margin-top: 0.5rem;">
                        RAIDZ1: Can survive 1 disk failure. Requires minimum 3 disks.
                    </p>
                </div>
                
                <div class="form-group">
                    <label>Select Disks</label>
                    <div id="disk-selector" class="disk-selector">
                        <p style="color: #aaa;">Loading disks...</p>
                    </div>
                </div>
                
                <div class="button-group">
                    <button type="submit" class="btn-primary">Create Pool</button>
                    <button type="button" class="btn-secondary" onclick="closeModal('modal-create-pool')">Cancel</button>
                </div>
            </form>
        </div>
    </div>

    <!-- Destroy Pool Modal -->
    <div id="modal-destroy-pool" class="modal">
        <div class="modal-content">
            <button class="modal-close" onclick="closeModal('modal-destroy-pool')">&times;</button>
            <h2>Destroy Storage Pool</h2>
            
            <div class="confirm-box">
                <h3>⚠️ DESTRUCTIVE ACTION</h3>
                <p><strong>This will permanently delete the pool and ALL data on it!</strong></p>
                <p>Pool: <strong id="destroy-pool-name"></strong></p>
                <p style="margin-top: 0.5rem;">This action <strong>CANNOT BE UNDONE</strong>.</p>
                
                <div class="confirm-checkbox">
                    <input type="checkbox" id="confirm-destroy-checkbox">
                    <label for="confirm-destroy-checkbox">I understand this will delete all data</label>
                </div>
            </div>
            
            <div class="button-group">
                <button class="btn-danger" onclick="destroyPool(document.getElementById('destroy-pool-name').textContent)" id="destroy-pool-btn" disabled>Destroy Pool</button>
                <button class="btn-secondary" onclick="closeModal('modal-destroy-pool')">Cancel</button>
            </div>
        </div>
    </div>

    <!-- Create Share Modal -->
    <div id="modal-create-share" class="modal">
        <div class="modal-content">
            <button class="modal-close" onclick="closeModal('modal-create-share')">&times;</button>
            <h2>Create Network Share</h2>
            
            <form id="create-share-form" onsubmit="event.preventDefault(); createShare();">
                <div class="form-group">
                    <label>Share Name</label>
                    <input type="text" name="name" class="form-input" placeholder="media" required>
                </div>
                
                <div class="form-group">
                    <label>Share Type</label>
                    <select name="share_type" class="form-select" id="share-type-select" onchange="updateShareTypeOptions()">
                        <option value="smb">SMB (Windows/macOS/Linux)</option>
                        <option value="nfs">NFS (Linux/Unix)</option>
                    </select>
                </div>
                
                <div class="form-group">
                    <label>Dataset Path</label>
                    <select name="dataset_path" class="form-select" id="share-dataset-select">
                        <option value="/mnt/tank/data">/mnt/tank/data</option>
                    </select>
                </div>
                
                <div id="smb-options">
                    <div class="form-group">
                        <label>
                            <input type="checkbox" name="smb_read_only"> Read Only
                        </label>
                    </div>
                    
                    <div class="form-group">
                        <label>
                            <input type="checkbox" name="smb_guest_ok"> Guest Access (No password)
                        </label>
                    </div>
                </div>
                
                <div id="nfs-options" style="display: none;">
                    <div class="form-group">
                        <label>Allowed Networks (CIDR)</label>
                        <input type="text" name="nfs_allowed_networks" class="form-input" placeholder="192.168.1.0/24">
                    </div>
                    
                    <div class="form-group">
                        <label>
                            <input type="checkbox" name="nfs_read_only"> Read Only
                        </label>
                    </div>
                </div>
                
                <div class="button-group">
                    <button type="submit" class="btn-primary">Create Share</button>
                    <button type="button" class="btn-secondary" onclick="closeModal('modal-create-share')">Cancel</button>
                </div>
            </form>
        </div>
    </div>

    <!-- Delete Share Modal -->
    <div id="modal-delete-share" class="modal">
        <div class="modal-content">
            <button class="modal-close" onclick="closeModal('modal-delete-share')">&times;</button>
            <h2>Delete Network Share</h2>
            
            <div class="confirm-box">
                <h3>⚠️ WARNING</h3>
                <p>This will remove the share: <strong id="delete-share-name"></strong></p>
                <p style="margin-top: 0.5rem;">The underlying data will NOT be deleted, only the share configuration.</p>
                <input type="hidden" id="delete-share-id">
                
                <div class="confirm-checkbox">
                    <input type="checkbox" id="confirm-share-delete-checkbox">
                    <label for="confirm-share-delete-checkbox">I understand this will remove the share</label>
                </div>
            </div>
            
            <div class="button-group">
                <button class="btn-danger" onclick="deleteShare(document.getElementById('delete-share-id').value)" id="delete-share-btn" disabled>Delete Share</button>
                <button class="btn-secondary" onclick="closeModal('modal-delete-share')">Cancel</button>
            </div>
        </div>
    </div>

    <!-- Create Snapshot Schedule Modal -->
    <div id="modal-create-schedule" class="modal">
        <div class="modal-content">
            <button class="modal-close" onclick="closeModal('modal-create-schedule')">&times;</button>
            <h2>Create Snapshot Schedule</h2>
            
            <form id="create-schedule-form" onsubmit="event.preventDefault(); createSchedule();">
                <div class="form-group">
                    <label>Dataset</label>
                    <select name="dataset_path" class="form-select" id="schedule-dataset-select">
                        <option value="tank/data">tank/data</option>
                    </select>
                </div>
                
                <div class="form-group">
                    <label>Frequency</label>
                    <select name="frequency" class="form-select">
                        <option value="hourly">Hourly</option>
                        <option value="daily" selected>Daily</option>
                        <option value="weekly">Weekly</option>
                        <option value="monthly">Monthly</option>
                    </select>
                </div>
                
                <div class="form-group">
                    <label>Time (for daily/weekly/monthly)</label>
                    <input type="time" name="time" class="form-input" value="02:00">
                </div>
                
                <div class="form-group">
                    <label>Keep (number of snapshots to retain)</label>
                    <input type="number" name="keep_count" class="form-input" value="7" min="1">
                </div>
                
                <div class="info-box">
                    <p>Snapshots will be automatically created and old ones cleaned up based on retention policy.</p>
                </div>
                
                <div class="button-group">
                    <button type="submit" class="btn-primary">Create Schedule</button>
                    <button type="button" class="btn-secondary" onclick="closeModal('modal-create-schedule')">Cancel</button>
                </div>
            </form>
        </div>
    </div>

    <!-- Restore Snapshot Modal -->
    <div id="modal-restore-snapshot" class="modal">
        <div class="modal-content">
            <button class="modal-close" onclick="closeModal('modal-restore-snapshot')">&times;</button>
            <h2>Restore Snapshot</h2>
            
            <div class="confirm-box">
                <h3>⚠️ IMPORTANT</h3>
                <p><strong>Rolling back will DESTROY all changes made after this snapshot!</strong></p>
                <p style="margin-top: 0.5rem;">
                    Dataset: <strong id="restore-dataset-name"></strong><br>
                    Snapshot: <strong id="restore-snapshot-name"></strong>
                </p>
                <p style="margin-top: 0.5rem;">All data created or modified after this snapshot will be permanently lost.</p>
                
                <div class="confirm-checkbox">
                    <input type="checkbox" id="confirm-restore-checkbox">
                    <label for="confirm-restore-checkbox">I understand data will be lost</label>
                </div>
            </div>
            
            <div class="button-group">
                <button class="btn-success" onclick="restoreSnapshot(document.getElementById('restore-dataset-name').textContent, document.getElementById('restore-snapshot-name').textContent)" id="restore-snapshot-btn" disabled>Restore Snapshot</button>
                <button class="btn-secondary" onclick="closeModal('modal-restore-snapshot')">Cancel</button>
            </div>
        </div>
    </div>

    <!-- Encryption Modal -->
    <div id="encryption-modal" class="modal">
        <div class="modal-content">
            <button class="modal-close" onclick="closeModal('encryption-modal')">&times;</button>
            <h2 id="encryption-modal-title">Create Encrypted Dataset</h2>
            
            <form id="encryption-form" onsubmit="event.preventDefault(); createEncryptedDataset();">
                <div class="form-group">
                    <label>Dataset Name</label>
                    <input type="text" name="name" class="form-input" placeholder="tank/encrypted" required>
                    <p style="color: #aaa; font-size: 0.85rem; margin-top: 0.5rem;">Example: tank/private or tank/encrypted</p>
                </div>
                
                <div class="form-group">
                    <label>Encryption Algorithm</label>
                    <select name="encryption" class="form-select">
                        <option value="aes-256-gcm" selected>AES-256-GCM (Recommended)</option>
                        <option value="aes-192-gcm">AES-192-GCM</option>
                        <option value="aes-128-gcm">AES-128-GCM</option>
                        <option value="aes-256-ccm">AES-256-CCM</option>
                        <option value="aes-192-ccm">AES-192-CCM</option>
                        <option value="aes-128-ccm">AES-128-CCM</option>
                    </select>
                </div>
                
                <div class="form-group">
                    <label>Password</label>
                    <input type="password" name="password" class="form-input" placeholder="Minimum 8 characters" minlength="8" required>
                </div>
                
                <div class="form-group">
                    <label>Confirm Password</label>
                    <input type="password" name="confirm_password" class="form-input" placeholder="Re-enter password" minlength="8" required>
                </div>
                
                <div class="info-box" style="background: rgba(244, 67, 54, 0.1); border-left-color: #f44336; margin: 1rem 0;">
                    <h3>⚠️ CRITICAL WARNING</h3>
                    <p><strong>Without this password, your data is PERMANENTLY INACCESSIBLE.</strong></p>
                    <p>Store this password securely in a password manager or safe location.</p>
                    <p>D-PlaneOS does NOT have a password recovery mechanism.</p>
                </div>
                
                <div class="button-group">
                    <button type="submit" class="btn-primary">Create Encrypted Dataset</button>
                    <button type="button" class="btn-secondary" onclick="closeModal('encryption-modal')">Cancel</button>
                </div>
            </form>
        </div>
    </div>

    <script src="/assets/js/main.js"></script>
</body>
</html>
