<?php
/**
 * D-PlaneOS Authentication Library
 * Handles user authentication, sessions, and audit logging
 */

// CRITICAL: Load security hardening FIRST
require_once __DIR__ . '/security.php';

define('DB_PATH', '/var/dplane/database/dplane.db');
define('SESSION_TIMEOUT', 1800); // 30 minutes

// Load command broker
require_once __DIR__ . '/command-broker.php';

// Initialize session
if (session_status() === PHP_SESSION_NONE) {
    session_start();
}

/**
 * Get database connection
 */
function getDB() {
    static $db = null;
    static $readOnly = false;
    
    if ($db === null) {
        try {
            // Check if database exists and is writable
            if (!file_exists(DB_PATH)) {
                error_log("CRITICAL: Database file does not exist: " . DB_PATH);
                die(json_encode(['success' => false, 'error' => 'System database not found']));
            }
            
            if (!is_writable(DB_PATH)) {
                error_log("WARNING: Database is not writable, entering read-only mode");
                $readOnly = true;
            }
            
            $db = new PDO('sqlite:' . DB_PATH);
            $db->setAttribute(PDO::ATTR_ERRMODE, PDO::ERRMODE_EXCEPTION);
            $db->setAttribute(PDO::ATTR_DEFAULT_FETCH_MODE, PDO::FETCH_ASSOC);
            
            // Test database integrity
            try {
                $db->query('SELECT 1 FROM users LIMIT 1');
            } catch (PDOException $e) {
                error_log("CRITICAL: Database integrity check failed: " . $e->getMessage());
                die(json_encode(['success' => false, 'error' => 'System database is corrupted']));
            }
            
        } catch (PDOException $e) {
            error_log("Database connection failed: " . $e->getMessage());
            die(json_encode(['success' => false, 'error' => 'Database connection failed']));
        }
    }
    
    // Warn if in read-only mode
    if ($readOnly && isset($_SERVER['REQUEST_METHOD']) && $_SERVER['REQUEST_METHOD'] !== 'GET') {
        http_response_code(503);
        die(json_encode(['success' => false, 'error' => 'System is in read-only mode']));
    }
    
    return $db;
}

/**
 * Check if user is authenticated
 */
function isAuthenticated() {
    if (!isset($_SESSION['user_id']) || !isset($_SESSION['last_activity'])) {
        return false;
    }
    
    // Check session timeout
    if (time() - $_SESSION['last_activity'] > SESSION_TIMEOUT) {
        session_unset();
        session_destroy();
        return false;
    }
    
    $_SESSION['last_activity'] = time();
    return true;
}

/**
 * Require authentication - call at top of protected pages/APIs
 */
function requireAuth() {
    if (!isAuthenticated()) {
        if (php_sapi_name() === 'cli') {
            return; // Allow CLI access
        }
        
        // Check if this is an API request
        if (strpos($_SERVER['REQUEST_URI'], '/api/') !== false) {
            http_response_code(401);
            header('Content-Type: application/json');
            die(json_encode(['success' => false, 'error' => 'Authentication required']));
        }
        
        // Redirect to login for page requests
        header('Location: /login.php');
        exit;
    }
}

/**
 * Get current user
 */
function getCurrentUser() {
    if (!isAuthenticated()) {
        return null;
    }
    
    $db = getDB();
    $stmt = $db->prepare('SELECT id, username, email FROM users WHERE id = ?');
    $stmt->execute([$_SESSION['user_id']]);
    return $stmt->fetch();
}

/**
 * Log user action for audit trail
 */
function logAction($action, $resource_type = null, $resource_name = null, $details = null) {
    try {
        $db = getDB();
        $stmt = $db->prepare('
            INSERT INTO audit_log (user_id, action, resource_type, resource_name, details, ip_address) 
            VALUES (?, ?, ?, ?, ?, ?)
        ');
        $stmt->execute([
            $_SESSION['user_id'] ?? null,
            $action,
            $resource_type,
            $resource_name,
            $details,
            $_SERVER['REMOTE_ADDR'] ?? 'unknown'
        ]);
    } catch (Exception $e) {
        error_log("Failed to log action: " . $e->getMessage());
    }
}

/**
 * Validate and sanitize input
 */
function validateInput($data, $type = 'string') {
    $data = trim($data);
    
    switch ($type) {
        case 'username':
            return preg_match('/^[a-zA-Z0-9_-]{3,32}$/', $data) ? $data : null;
        case 'pool_name':
        case 'dataset_name':
            return preg_match('/^[a-zA-Z0-9_\/-]+$/', $data) ? $data : null;
        case 'disk_path':
            return preg_match('/^\/dev\/[a-zA-Z0-9\/]+$/', $data) ? $data : null;
        case 'email':
            return filter_var($data, FILTER_VALIDATE_EMAIL) ?: null;
        case 'integer':
            return filter_var($data, FILTER_VALIDATE_INT);
        default:
            return $data;
    }
}

/**
 * Execute shell command safely with input validation
 * Enhanced version that validates inputs before execution
 */
function execCommand($cmd, &$output = [], &$return_code = 0) {
    $allowedCommands = [
        'sudo', '/usr/sbin/zpool', '/usr/sbin/zfs', '/usr/bin/docker',
        '/usr/sbin/smartctl', '/usr/bin/lsblk', '/usr/bin/df', '/bin/grep',
        '/bin/cat', '/usr/bin/rclone', '/usr/bin/systemctl', '/usr/sbin/service',
        '/usr/sbin/smbd', '/usr/sbin/smbpasswd', '/usr/bin/chpasswd',
    ];
    
    $parts = preg_split('/\s+/', trim($cmd), -1, PREG_SPLIT_NO_EMPTY);
    if (empty($parts)) {
        error_log("SECURITY: Empty command blocked");
        $output[] = "Invalid command";
        $return_code = 1;
        return false;
    }
    
    $commandIndex = ($parts[0] === 'sudo') ? 1 : 0;
    if (!isset($parts[$commandIndex])) {
        error_log("SECURITY: Invalid command structure");
        $output[] = "Invalid command";
        $return_code = 1;
        return false;
    }
    
    $baseCommand = $parts[$commandIndex];
    
    if (!in_array($baseCommand, $allowedCommands)) {
        error_log("SECURITY: Non-whitelisted command blocked: $baseCommand");
        $output[] = "Command not allowed";
        $return_code = 1;
        return false;
    }
    
    $dangerous = ['&&', '||', ';', '`', '$', "\n", "\r", "\t", '(', ')'];
    foreach ($dangerous as $char) {
        if (strpos($cmd, $char) !== false) {
            error_log("SECURITY: Blocked dangerous character: " . substr($cmd, 0, 100));
            $output[] = "Command contains dangerous characters";
            $return_code = 1;
            return false;
        }
    }
    
    if (strpos($cmd, '>') !== false && !preg_match('/2>&1/', $cmd)) {
        error_log("SECURITY: Blocked redirect");
        $output[] = "Command contains dangerous characters";
        $return_code = 1;
        return false;
    }
    
    if (strpos($cmd, '|') !== false) {
        if (strpos($cmd, 'docker') === false || strpos($cmd, '--format') === false || !preg_match('/(["|\']).*\|.*\1/', $cmd)) {
            error_log("SECURITY: Blocked pipe");
            $output[] = "Command contains dangerous characters";
            $return_code = 1;
            return false;
        }
    }
    
    error_log("CMD_EXEC: " . substr($cmd, 0, 200));
    exec($cmd . ' 2>&1', $output, $return_code);
    
    if ($return_code !== 0) {
        error_log("CMD_FAILED (code $return_code): " . substr($cmd, 0, 200));
    }
    
    return $return_code === 0;
}

/**
 * Execute command through secure broker
 */
function execSecure($commandKey, $params = [], &$output = [], &$return_code = 0) {
    try {
        return CommandBroker::execute($commandKey, $params, $output, $return_code);
    } catch (Exception $e) {
        error_log("Command broker error: " . $e->getMessage());
        $output[] = $e->getMessage();
        $return_code = 1;
        return false;
    }
}

/**
 * Format bytes to human readable
 */
function formatBytes($bytes) {
    $units = ['B', 'KB', 'MB', 'GB', 'TB', 'PB'];
    $bytes = max($bytes, 0);
    $pow = floor(($bytes ? log($bytes) : 0) / log(1024));
    $pow = min($pow, count($units) - 1);
    $bytes /= pow(1024, $pow);
    return round($bytes, 2) . ' ' . $units[$pow];
}
