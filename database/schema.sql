-- D-PlaneOS Database Schema (Corrected)
-- SQLite3 database for system metadata

-- Users table
CREATE TABLE IF NOT EXISTS users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT UNIQUE NOT NULL CHECK(length(username) >= 3),
    password TEXT NOT NULL,
    email TEXT,
    role TEXT DEFAULT 'user' CHECK(role IN ('admin', 'user', 'readonly')),
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    last_login DATETIME
);

-- Widget configuration
CREATE TABLE IF NOT EXISTS widgets (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    type TEXT NOT NULL,
    position TEXT NOT NULL CHECK(position IN ('left', 'right', 'center')),
    config TEXT,
    enabled INTEGER DEFAULT 1 CHECK(enabled IN (0, 1)),
    order_index INTEGER DEFAULT 0,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- App shortcuts
CREATE TABLE IF NOT EXISTS app_shortcuts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    name TEXT NOT NULL,
    url TEXT NOT NULL,
    icon TEXT DEFAULT 'globe',
    order_index INTEGER DEFAULT 0,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- Audit log for all operations
CREATE TABLE IF NOT EXISTS audit_log (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER,
    action TEXT NOT NULL,
    resource_type TEXT,
    resource_name TEXT,
    details TEXT,
    ip_address TEXT,
    timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE SET NULL
);

-- System settings
CREATE TABLE IF NOT EXISTS settings (
    key TEXT PRIMARY KEY,
    value TEXT,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- SMART test history
CREATE TABLE IF NOT EXISTS smart_history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    disk_path TEXT NOT NULL,
    test_type TEXT NOT NULL CHECK(test_type IN ('short', 'long', 'conveyance')),
    health_status TEXT CHECK(health_status IN ('PASSED', 'FAILED', 'UNKNOWN')),
    temperature INTEGER,
    power_on_hours INTEGER,
    test_result TEXT,
    raw_output TEXT,
    timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Scrub schedules
CREATE TABLE IF NOT EXISTS scrub_schedules (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    pool_name TEXT NOT NULL,
    schedule_type TEXT NOT NULL CHECK(schedule_type IN ('daily', 'weekly', 'monthly')),
    day_of_month INTEGER CHECK(day_of_month BETWEEN 1 AND 31),
    enabled INTEGER DEFAULT 1 CHECK(enabled IN (0, 1)),
    last_run DATETIME,
    next_run DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- ZFS replication tasks
CREATE TABLE IF NOT EXISTS replication_tasks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    source_dataset TEXT NOT NULL,
    destination_host TEXT NOT NULL,
    destination_dataset TEXT NOT NULL,
    schedule_type TEXT NOT NULL CHECK(schedule_type IN ('manual', 'hourly', 'daily', 'weekly')),
    last_run DATETIME,
    last_status TEXT CHECK(last_status IN ('success', 'failed', 'running', NULL)),
    enabled INTEGER DEFAULT 1 CHECK(enabled IN (0, 1)),
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Alert settings
CREATE TABLE IF NOT EXISTS alert_settings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    alert_type TEXT NOT NULL,
    enabled INTEGER DEFAULT 0 CHECK(enabled IN (0, 1)),
    webhook_url TEXT,
    config TEXT,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Alert history
CREATE TABLE IF NOT EXISTS alert_history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    alert_type TEXT NOT NULL,
    severity TEXT NOT NULL CHECK(severity IN ('info', 'warning', 'error', 'critical')),
    message TEXT NOT NULL,
    details TEXT,
    sent INTEGER DEFAULT 0 CHECK(sent IN (0, 1)),
    timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Metrics history for analytics
CREATE TABLE IF NOT EXISTS metrics_history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    metric_type TEXT NOT NULL,
    resource_name TEXT,
    value REAL NOT NULL,
    timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Network shares (SMB/NFS)
CREATE TABLE IF NOT EXISTS shares (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT UNIQUE NOT NULL,
    dataset_path TEXT NOT NULL,
    share_type TEXT NOT NULL CHECK(share_type IN ('smb', 'nfs')),
    enabled INTEGER DEFAULT 1 CHECK(enabled IN (0, 1)),
    
    -- SMB specific
    smb_guest_ok INTEGER DEFAULT 0 CHECK(smb_guest_ok IN (0, 1)),
    smb_read_only INTEGER DEFAULT 0 CHECK(smb_read_only IN (0, 1)),
    smb_browseable INTEGER DEFAULT 1 CHECK(smb_browseable IN (0, 1)),
    smb_valid_users TEXT, -- Comma-separated (consider normalizing later)
    
    -- NFS specific
    nfs_allowed_networks TEXT, -- e.g., '192.168.1.0/24,10.0.0.0/8'
    nfs_read_only INTEGER DEFAULT 0 CHECK(nfs_read_only IN (0, 1)),
    nfs_sync TEXT DEFAULT 'async' CHECK(nfs_sync IN ('sync', 'async')),
    nfs_no_root_squash INTEGER DEFAULT 0 CHECK(nfs_no_root_squash IN (0, 1)),
    
    -- Common
    comment TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Samba users for share access
CREATE TABLE IF NOT EXISTS smb_users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT UNIQUE NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- User quotas table
CREATE TABLE IF NOT EXISTS user_quotas (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT NOT NULL,
    dataset_path TEXT NOT NULL,
    quota_bytes INTEGER NOT NULL CHECK(quota_bytes > 0),
    enabled INTEGER DEFAULT 1 CHECK(enabled IN (0, 1)),
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(username, dataset_path)
);

-- System notifications
CREATE TABLE IF NOT EXISTS notifications (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    title TEXT NOT NULL,
    message TEXT NOT NULL,
    type TEXT NOT NULL CHECK(type IN ('info', 'warning', 'error', 'success')),
    category TEXT CHECK(category IN ('disk', 'pool', 'system', 'replication', 'quota', NULL)),
    priority INTEGER DEFAULT 0 CHECK(priority BETWEEN 0 AND 3),
    read INTEGER DEFAULT 0 CHECK(read IN (0, 1)),
    dismissed INTEGER DEFAULT 0 CHECK(dismissed IN (0, 1)),
    action_url TEXT,
    details TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    expires_at DATETIME
);

-- Disk tracking
CREATE TABLE IF NOT EXISTS disk_tracking (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    disk_path TEXT UNIQUE NOT NULL,
    disk_serial TEXT,
    disk_model TEXT,
    disk_size INTEGER,
    first_seen DATETIME DEFAULT CURRENT_TIMESTAMP,
    last_seen DATETIME DEFAULT CURRENT_TIMESTAMP,
    status TEXT DEFAULT 'healthy' CHECK(status IN ('healthy', 'warning', 'critical', 'failing', 'replaced')),
    in_pool TEXT,
    notes TEXT,
    replacement_date DATETIME,
    replaced_by TEXT
);

-- Disk maintenance log (FIXED: Now references id instead of disk_path)
CREATE TABLE IF NOT EXISTS disk_maintenance_log (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    disk_id INTEGER NOT NULL,
    action_type TEXT NOT NULL CHECK(action_type IN ('smart_test', 'replacement', 'note', 'status_change')),
    description TEXT NOT NULL,
    performed_by TEXT,
    result TEXT,
    timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (disk_id) REFERENCES disk_tracking(id) ON DELETE CASCADE
);

-- UPS status tracking
CREATE TABLE IF NOT EXISTS ups_status (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    ups_name TEXT NOT NULL,
    status TEXT CHECK(status IN ('ONLINE', 'ONBATT', 'LOWBATT', 'COMMOK', 'COMMBAD', NULL)),
    battery_charge INTEGER CHECK(battery_charge BETWEEN 0 AND 100),
    battery_runtime INTEGER CHECK(battery_runtime >= 0),
    load INTEGER CHECK(load BETWEEN 0 AND 100),
    input_voltage REAL,
    output_voltage REAL,
    temperature REAL,
    last_check DATETIME DEFAULT CURRENT_TIMESTAMP,
    ups_model TEXT,
    ups_serial TEXT
);

-- Snapshot automation schedules
CREATE TABLE IF NOT EXISTS snapshot_schedules (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    dataset_path TEXT NOT NULL,
    frequency TEXT NOT NULL CHECK(frequency IN ('hourly', 'daily', 'weekly', 'monthly')),
    keep_count INTEGER NOT NULL CHECK(keep_count > 0),
    enabled INTEGER DEFAULT 1 CHECK(enabled IN (0, 1)),
    last_run DATETIME,
    next_run DATETIME,
    name_prefix TEXT DEFAULT 'auto-',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Snapshot retention log
CREATE TABLE IF NOT EXISTS snapshot_history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    dataset_path TEXT NOT NULL,
    snapshot_name TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    deleted_at DATETIME,
    size_bytes INTEGER,
    schedule_id INTEGER,
    FOREIGN KEY (schedule_id) REFERENCES snapshot_schedules(id) ON DELETE SET NULL
);

-- Rclone remote configurations
CREATE TABLE IF NOT EXISTS rclone_remotes (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT UNIQUE NOT NULL,
    remote_type TEXT NOT NULL,
    config TEXT NOT NULL, -- JSON config
    enabled INTEGER DEFAULT 1 CHECK(enabled IN (0, 1)),
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Rclone sync tasks
CREATE TABLE IF NOT EXISTS rclone_tasks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    remote_id INTEGER NOT NULL,
    source_path TEXT NOT NULL,
    destination_path TEXT NOT NULL,
    direction TEXT NOT NULL CHECK(direction IN ('push', 'pull')),
    sync_type TEXT NOT NULL CHECK(sync_type IN ('sync', 'copy', 'move')),
    schedule_type TEXT CHECK(schedule_type IN ('manual', 'hourly', 'daily', 'weekly', NULL)),
    enabled INTEGER DEFAULT 1 CHECK(enabled IN (0, 1)),
    last_run DATETIME,
    last_status TEXT CHECK(last_status IN ('success', 'failed', 'running', NULL)),
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (remote_id) REFERENCES rclone_remotes(id) ON DELETE CASCADE
);

-- Create indices for performance
CREATE INDEX IF NOT EXISTS idx_audit_log_timestamp ON audit_log(timestamp);
CREATE INDEX IF NOT EXISTS idx_audit_log_user ON audit_log(user_id);
CREATE INDEX IF NOT EXISTS idx_widgets_user ON widgets(user_id);
CREATE INDEX IF NOT EXISTS idx_metrics_history ON metrics_history(metric_type, timestamp);
CREATE INDEX IF NOT EXISTS idx_alert_history ON alert_history(timestamp);
CREATE INDEX IF NOT EXISTS idx_shares_type ON shares(share_type);
CREATE INDEX IF NOT EXISTS idx_shares_dataset ON shares(dataset_path);
CREATE INDEX IF NOT EXISTS idx_rclone_tasks_remote ON rclone_tasks(remote_id);
CREATE INDEX IF NOT EXISTS idx_rclone_tasks_enabled ON rclone_tasks(enabled);
CREATE INDEX IF NOT EXISTS idx_disk_maintenance_disk ON disk_maintenance_log(disk_id);
CREATE INDEX IF NOT EXISTS idx_notifications_type ON notifications(type, read);

-- Triggers for updated_at columns
CREATE TRIGGER IF NOT EXISTS update_shares_timestamp 
AFTER UPDATE ON shares
BEGIN
    UPDATE shares SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;

CREATE TRIGGER IF NOT EXISTS update_smb_users_timestamp 
AFTER UPDATE ON smb_users
BEGIN
    UPDATE smb_users SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;

CREATE TRIGGER IF NOT EXISTS update_user_quotas_timestamp 
AFTER UPDATE ON user_quotas
BEGIN
    UPDATE user_quotas SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;

CREATE TRIGGER IF NOT EXISTS update_rclone_remotes_timestamp 
AFTER UPDATE ON rclone_remotes
BEGIN
    UPDATE rclone_remotes SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;

-- Create default admin user ONLY if not exists
-- NOTE: Change password immediately after first login!
-- Default password: admin123
INSERT OR IGNORE INTO users (id, username, password, email, role) 
VALUES (1, 'admin', '$2y$10$92IXUNpkjO0rOQ5byMi.Ye4oKoEa3Ro9llC/.og/at2.uheWG/igi', 'admin@localhost', 'admin');

-- Create default widgets for admin
INSERT OR IGNORE INTO widgets (user_id, type, position, enabled, order_index) VALUES
(1, 'datetime', 'left', 1, 0),
(1, 'system', 'left', 1, 1);
