-- D-PlaneOS Database Schema (CORRECTED)
-- SQLite3 database for system metadata

-- Users table
CREATE TABLE IF NOT EXISTS users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT UNIQUE NOT NULL,
    password TEXT NOT NULL,
    email TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    last_login DATETIME
);

-- Widget configuration
CREATE TABLE IF NOT EXISTS widgets (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    type TEXT NOT NULL,
    position TEXT NOT NULL,
    config TEXT,
    enabled INTEGER DEFAULT 1,
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
    FOREIGN KEY (user_id) REFERENCES users(id)
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
    test_type TEXT NOT NULL,
    health_status TEXT,
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
    schedule_type TEXT NOT NULL,
    day_of_month INTEGER,
    enabled INTEGER DEFAULT 1,
    last_run DATETIME,
    next_run DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Create default admin user (password: admin123)
INSERT OR IGNORE INTO users (id, username, password, email) 
VALUES (1, 'admin', '$2y$10$92IXUNpkjO0rOQ5byMi.Ye4oKoEa3Ro9llC/.og/at2.uheWG/igi', 'admin@localhost');

-- Create default widgets for admin
INSERT OR IGNORE INTO widgets (user_id, type, position, enabled, order_index) VALUES
(1, 'datetime', 'left', 1, 0),
(1, 'system', 'left', 1, 1);

-- ZFS replication tasks
CREATE TABLE IF NOT EXISTS replication_tasks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    source_dataset TEXT NOT NULL,
    destination_host TEXT NOT NULL,
    destination_dataset TEXT NOT NULL,
    schedule_type TEXT NOT NULL,
    last_run DATETIME,
    last_status TEXT,
    enabled INTEGER DEFAULT 1,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Alert settings
CREATE TABLE IF NOT EXISTS alert_settings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    alert_type TEXT NOT NULL,
    enabled INTEGER DEFAULT 0,
    webhook_url TEXT,
    config TEXT, -- JSON
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Alert history
CREATE TABLE IF NOT EXISTS alert_history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    alert_type TEXT NOT NULL,
    severity TEXT NOT NULL CHECK(severity IN ('low', 'medium', 'high', 'critical')),
    message TEXT NOT NULL,
    details TEXT,
    sent INTEGER DEFAULT 0,
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
    enabled INTEGER DEFAULT 1,
    
    -- SMB specific
    smb_guest_ok INTEGER DEFAULT 0,
    smb_read_only INTEGER DEFAULT 0,
    smb_browseable INTEGER DEFAULT 1,
    smb_valid_users TEXT, -- Comma-separated (TODO: normalize to junction table)
    
    -- NFS specific
    nfs_allowed_networks TEXT, -- e.g., '192.168.1.0/24,10.0.0.0/8'
    nfs_read_only INTEGER DEFAULT 0,
    nfs_sync TEXT DEFAULT 'async' CHECK(nfs_sync IN ('sync', 'async')),
    nfs_no_root_squash INTEGER DEFAULT 0,
    
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

-- User quotas table for dataset-level quota management
CREATE TABLE IF NOT EXISTS user_quotas (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT NOT NULL,
    dataset_path TEXT NOT NULL,
    quota_bytes INTEGER NOT NULL,
    enabled INTEGER DEFAULT 1,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(username, dataset_path)
);

-- System notifications table
CREATE TABLE IF NOT EXISTS notifications (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    title TEXT NOT NULL,
    message TEXT NOT NULL,
    type TEXT NOT NULL CHECK(type IN ('info', 'warning', 'error', 'success')),
    category TEXT CHECK(category IN ('disk', 'pool', 'system', 'replication', 'quota', 'general')),
    priority INTEGER DEFAULT 0 CHECK(priority BETWEEN 0 AND 3),
    read INTEGER DEFAULT 0,
    dismissed INTEGER DEFAULT 0,
    action_url TEXT,
    details TEXT, -- JSON
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    expires_at DATETIME
);

-- Disk tracking table
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

-- FIXED: Disk maintenance log now references disk_tracking.id
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

-- UPS/USV status tracking
CREATE TABLE IF NOT EXISTS ups_status (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    ups_name TEXT NOT NULL,
    status TEXT CHECK(status IN ('ONLINE', 'ONBATT', 'LOWBATT', 'COMMOK', 'COMMBAD', 'UNKNOWN')),
    battery_charge INTEGER CHECK(battery_charge BETWEEN 0 AND 100),
    battery_runtime INTEGER,
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
    enabled INTEGER DEFAULT 1,
    last_run DATETIME,
    next_run DATETIME,
    name_prefix TEXT,
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
    config TEXT NOT NULL, -- JSON
    enabled INTEGER DEFAULT 1,
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
    schedule_type TEXT CHECK(schedule_type IN ('manual', 'hourly', 'daily', 'weekly')),
    enabled INTEGER DEFAULT 1,
    last_run DATETIME,
    last_status TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (remote_id) REFERENCES rclone_remotes(id) ON DELETE CASCADE
);

-- Performance indices
CREATE INDEX IF NOT EXISTS idx_audit_log_timestamp ON audit_log(timestamp);
CREATE INDEX IF NOT EXISTS idx_audit_log_user ON audit_log(user_id);
CREATE INDEX IF NOT EXISTS idx_widgets_user ON widgets(user_id);
CREATE INDEX IF NOT EXISTS idx_metrics_history ON metrics_history(metric_type, timestamp);
CREATE INDEX IF NOT EXISTS idx_alert_history ON alert_history(timestamp);
CREATE INDEX IF NOT EXISTS idx_shares_type ON shares(share_type);
CREATE INDEX IF NOT EXISTS idx_shares_dataset ON shares(dataset_path);
CREATE INDEX IF NOT EXISTS idx_rclone_tasks_remote ON rclone_tasks(remote_id);
CREATE INDEX IF NOT EXISTS idx_rclone_tasks_enabled ON rclone_tasks(enabled);
CREATE INDEX IF NOT EXISTS idx_snapshot_schedules_dataset ON snapshot_schedules(dataset_path);
CREATE INDEX IF NOT EXISTS idx_disk_tracking_status ON disk_tracking(status);
CREATE INDEX IF NOT EXISTS idx_notifications_read ON notifications(read, dismissed);
CREATE INDEX IF NOT EXISTS idx_disk_maintenance_disk ON disk_maintenance_log(disk_id);

-- ADDED: Auto-update triggers for updated_at fields
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

CREATE TRIGGER IF NOT EXISTS update_settings_timestamp 
AFTER UPDATE ON settings
BEGIN
    UPDATE settings SET updated_at = CURRENT_TIMESTAMP WHERE key = NEW.key;
END;
