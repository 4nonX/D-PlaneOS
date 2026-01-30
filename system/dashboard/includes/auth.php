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
    $stmt = $db->prepare('SELECT id, username, email, role FROM users WHERE id = ?');
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
    // Extract and validate any pool names
    if (preg_match_all('/\b([a-zA-Z0-9_-]+)\b/', $cmd, $matches)) {
        foreach ($matches[1] as $token) {
            // Skip command names and flags
            if (in_array($token, ['sudo', 'usr', 'sbin', 'bin', 'zpool', 'zfs', 'docker', 'smartctl', 'lsblk'])) continue;
            if (strpos($token, '-') === 0) continue; // Skip flags
            
            // Validate tokens that look like names
            if (strlen($token) > 2 && !preg_match('/^[a-zA-Z0-9_\/-]+$/', $token)) {
                error_log("SECURITY: Rejected suspicious token in command: $token");
                $output[] = "Invalid parameter detected";
                $return_code = 1;
                return false;
            }
        }
    }
    
    // Check for command injection patterns
    $dangerous = ['&&', '||', ';', '|', '`', '$', '>', '<', "\n", "\r"];
    foreach ($dangerous as $char) {
        if (strpos($cmd, $char) !== false && strpos($cmd, 'escapeshellarg') === false) {
            error_log("SECURITY: Blocked command injection attempt: " . substr($cmd, 0, 100));
            $output[] = "Command contains dangerous characters";
            $return_code = 1;
            return false;
        }
    }
    
    exec($cmd . ' 2>&1', $output, $return_code);
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

/**
 * RBAC: Get current user's role
 */
function getCurrentUserRole() {
    $user = getCurrentUser();
    return $user ? ($user['role'] ?? 'user') : null;
}

/**
 * RBAC: Check if user has specific role
 */
function hasRole($role) {
    $currentRole = getCurrentUserRole();
    if (!$currentRole) return false;
    
    // Exact role match
    if ($currentRole === $role) return true;
    
    // Admin has all permissions
    if ($currentRole === 'admin') return true;
    
    return false;
}

/**
 * RBAC: Require specific role (or admin)
 */
function requireRole($role, $errorMessage = 'Insufficient permissions') {
    if (!hasRole($role)) {
        if (strpos($_SERVER['REQUEST_URI'], '/api/') !== false) {
            http_response_code(403);
            header('Content-Type: application/json');
            die(json_encode(['success' => false, 'error' => $errorMessage]));
        }
        
        http_response_code(403);
        die('<h1>403 Forbidden</h1><p>' . htmlspecialchars($errorMessage) . '</p>');
    }
}

/**
 * RBAC: Check if current user is admin
 */
function isAdmin() {
    return getCurrentUserRole() === 'admin';
}

/**
 * RBAC: Check if user can write/modify (admin or user role)
 */
function canWrite() {
    $role = getCurrentUserRole();
    return in_array($role, ['admin', 'user'], true);
}

/**
 * RBAC: Check if user can read (all roles)
 */
function canRead() {
    return isAuthenticated();
}

/**
 * RBAC: Require write permissions
 */
function requireWrite($errorMessage = 'Write access required') {
    if (!canWrite()) {
        if (strpos($_SERVER['REQUEST_URI'], '/api/') !== false) {
            http_response_code(403);
            header('Content-Type: application/json');
            die(json_encode(['success' => false, 'error' => $errorMessage]));
        }
        
        http_response_code(403);
        die('<h1>403 Forbidden</h1><p>' . htmlspecialchars($errorMessage) . '</p>');
    }
}

/**
 * RBAC: Require admin role
 */
function requireAdmin($errorMessage = 'Administrator access required') {
    requireRole('admin', $errorMessage);
}
