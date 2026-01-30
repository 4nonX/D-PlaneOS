<?php
/**
 * D-PlaneOS Security Hardening Layer
 * REQUIRED for internet-facing deployments
 * 
 * This file MUST be included before auth.php in ALL entry points
 */

// Prevent direct access
if (!defined('DPLANEOS_SECURITY_INIT')) {
    define('DPLANEOS_SECURITY_INIT', true);
}

// ============================================
// SECURITY CONFIGURATION
// ============================================

define('RATE_LIMIT_ENABLED', true);
define('RATE_LIMIT_REQUESTS', 100);        // Max requests per window
define('RATE_LIMIT_WINDOW', 300);          // 5 minutes
define('RATE_LIMIT_BAN_DURATION', 3600);   // 1 hour ban

define('BRUTE_FORCE_LIMIT', 5);            // Max failed login attempts
define('BRUTE_FORCE_WINDOW', 300);         // Within 5 minutes
define('BRUTE_FORCE_BAN', 1800);           // 30 minute ban

define('CSRF_TOKEN_LENGTH', 32);
define('CSRF_TOKEN_LIFETIME', 3600);       // 1 hour

// IP allowlist (empty = allow all, DANGEROUS for internet)
// Example: define('IP_ALLOWLIST', ['192.168.1.0/24', '10.0.0.0/8']);
define('IP_ALLOWLIST', []);

// Security headers
define('SECURITY_HEADERS', [
    'X-Frame-Options' => 'DENY',
    'X-Content-Type-Options' => 'nosniff',
    'X-XSS-Protection' => '1; mode=block',
    'Referrer-Policy' => 'strict-origin-when-cross-origin',
    'Permissions-Policy' => 'geolocation=(), microphone=(), camera=()',
    'Content-Security-Policy' => "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'; connect-src 'self'; frame-ancestors 'none';"
]);

// ============================================
// RATE LIMITING
// ============================================

class RateLimiter {
    private static $db = null;
    
    private static function getDB() {
        if (self::$db === null) {
            self::$db = new PDO('sqlite:/var/dplane/database/dplane.db');
            self::$db->setAttribute(PDO::ATTR_ERRMODE, PDO::ERRMODE_EXCEPTION);
            
            // Create rate limit table if not exists
            self::$db->exec('
                CREATE TABLE IF NOT EXISTS rate_limits (
                    ip TEXT PRIMARY KEY,
                    requests INTEGER DEFAULT 0,
                    window_start INTEGER,
                    banned_until INTEGER DEFAULT 0
                )
            ');
        }
        return self::$db;
    }
    
    public static function check($ip) {
        if (!RATE_LIMIT_ENABLED) return true;
        
        $db = self::getDB();
        $now = time();
        
        // Check if IP is banned
        $stmt = $db->prepare('SELECT banned_until FROM rate_limits WHERE ip = ?');
        $stmt->execute([$ip]);
        $row = $stmt->fetch();
        
        if ($row && $row['banned_until'] > $now) {
            http_response_code(429);
            header('Retry-After: ' . ($row['banned_until'] - $now));
            die(json_encode([
                'success' => false, 
                'error' => 'Too many requests. Banned until ' . date('Y-m-d H:i:s', $row['banned_until'])
            ]));
        }
        
        // Get or create rate limit record
        $stmt = $db->prepare('SELECT * FROM rate_limits WHERE ip = ?');
        $stmt->execute([$ip]);
        $row = $stmt->fetch();
        
        if (!$row) {
            // First request
            $stmt = $db->prepare('INSERT INTO rate_limits (ip, requests, window_start) VALUES (?, 1, ?)');
            $stmt->execute([$ip, $now]);
            return true;
        }
        
        // Check if window expired
        if ($now - $row['window_start'] > RATE_LIMIT_WINDOW) {
            // Reset window
            $stmt = $db->prepare('UPDATE rate_limits SET requests = 1, window_start = ? WHERE ip = ?');
            $stmt->execute([$now, $ip]);
            return true;
        }
        
        // Increment request count
        $newCount = $row['requests'] + 1;
        
        if ($newCount > RATE_LIMIT_REQUESTS) {
            // Ban IP
            $bannedUntil = $now + RATE_LIMIT_BAN_DURATION;
            $stmt = $db->prepare('UPDATE rate_limits SET banned_until = ? WHERE ip = ?');
            $stmt->execute([$bannedUntil, $ip]);
            
            error_log("SECURITY: Rate limit exceeded for IP $ip - banned until " . date('Y-m-d H:i:s', $bannedUntil));
            
            http_response_code(429);
            header('Retry-After: ' . RATE_LIMIT_BAN_DURATION);
            die(json_encode(['success' => false, 'error' => 'Rate limit exceeded. Try again later.']));
        }
        
        $stmt = $db->prepare('UPDATE rate_limits SET requests = ? WHERE ip = ?');
        $stmt->execute([$newCount, $ip]);
        return true;
    }
}

// ============================================
// BRUTE FORCE PROTECTION
// ============================================

class BruteForceProtection {
    private static $db = null;
    
    private static function getDB() {
        if (self::$db === null) {
            self::$db = new PDO('sqlite:/var/dplane/database/dplane.db');
            self::$db->setAttribute(PDO::ATTR_ERRMODE, PDO::ERRMODE_EXCEPTION);
            
            self::$db->exec('
                CREATE TABLE IF NOT EXISTS brute_force_log (
                    id INTEGER PRIMARY KEY AUTOINCREMENT,
                    ip TEXT,
                    username TEXT,
                    timestamp INTEGER,
                    success INTEGER DEFAULT 0
                )
            ');
            
            self::$db->exec('
                CREATE TABLE IF NOT EXISTS brute_force_bans (
                    ip TEXT PRIMARY KEY,
                    banned_until INTEGER
                )
            ');
        }
        return self::$db;
    }
    
    public static function checkBan($ip) {
        $db = self::getDB();
        $now = time();
        
        $stmt = $db->prepare('SELECT banned_until FROM brute_force_bans WHERE ip = ? AND banned_until > ?');
        $stmt->execute([$ip, $now]);
        $row = $stmt->fetch();
        
        if ($row) {
            http_response_code(403);
            die(json_encode([
                'success' => false,
                'error' => 'Account temporarily locked due to too many failed login attempts'
            ]));
        }
    }
    
    public static function recordAttempt($ip, $username, $success) {
        $db = self::getDB();
        $now = time();
        
        // Record attempt
        $stmt = $db->prepare('INSERT INTO brute_force_log (ip, username, timestamp, success) VALUES (?, ?, ?, ?)');
        $stmt->execute([$ip, $username, $now, $success ? 1 : 0]);
        
        if ($success) {
            // Clear failed attempts on successful login
            $cutoff = $now - BRUTE_FORCE_WINDOW;
            $stmt = $db->prepare('DELETE FROM brute_force_log WHERE ip = ? AND timestamp > ? AND success = 0');
            $stmt->execute([$ip, $cutoff]);
            return;
        }
        
        // Count recent failed attempts
        $cutoff = $now - BRUTE_FORCE_WINDOW;
        $stmt = $db->prepare('SELECT COUNT(*) as count FROM brute_force_log WHERE ip = ? AND timestamp > ? AND success = 0');
        $stmt->execute([$ip, $cutoff]);
        $row = $stmt->fetch();
        
        if ($row['count'] >= BRUTE_FORCE_LIMIT) {
            // Ban IP
            $bannedUntil = $now + BRUTE_FORCE_BAN;
            $stmt = $db->prepare('INSERT OR REPLACE INTO brute_force_bans (ip, banned_until) VALUES (?, ?)');
            $stmt->execute([$ip, $bannedUntil]);
            
            error_log("SECURITY: Brute force detected from IP $ip - banned until " . date('Y-m-d H:i:s', $bannedUntil));
            
            http_response_code(403);
            die(json_encode([
                'success' => false,
                'error' => 'Too many failed login attempts. Account temporarily locked.'
            ]));
        }
    }
    
    public static function cleanup() {
        $db = self::getDB();
        $now = time();
        
        // Clean up old logs
        $cutoff = $now - (BRUTE_FORCE_WINDOW * 2);
        $db->exec("DELETE FROM brute_force_log WHERE timestamp < $cutoff");
        
        // Clean up expired bans
        $db->exec("DELETE FROM brute_force_bans WHERE banned_until < $now");
    }
}

// ============================================
// CSRF PROTECTION
// ============================================

class CSRFProtection {
    public static function generateToken() {
        if (session_status() === PHP_SESSION_NONE) {
            session_start();
        }
        
        $token = bin2hex(random_bytes(CSRF_TOKEN_LENGTH));
        $_SESSION['csrf_token'] = $token;
        $_SESSION['csrf_token_time'] = time();
        
        return $token;
    }
    
    public static function validateToken($token) {
        if (session_status() === PHP_SESSION_NONE) {
            session_start();
        }
        
        if (!isset($_SESSION['csrf_token']) || !isset($_SESSION['csrf_token_time'])) {
            return false;
        }
        
        // Check token age
        if (time() - $_SESSION['csrf_token_time'] > CSRF_TOKEN_LIFETIME) {
            unset($_SESSION['csrf_token']);
            unset($_SESSION['csrf_token_time']);
            return false;
        }
        
        // Timing-safe comparison
        return hash_equals($_SESSION['csrf_token'], $token);
    }
    
    public static function requireToken() {
        $token = null;
        
        // Check header first (for AJAX)
        if (isset($_SERVER['HTTP_X_CSRF_TOKEN'])) {
            $token = $_SERVER['HTTP_X_CSRF_TOKEN'];
        }
        // Then POST data
        elseif (isset($_POST['csrf_token'])) {
            $token = $_POST['csrf_token'];
        }
        // Then GET (less secure, for rare cases)
        elseif (isset($_GET['csrf_token'])) {
            $token = $_GET['csrf_token'];
        }
        
        if (!$token || !self::validateToken($token)) {
            http_response_code(403);
            die(json_encode(['success' => false, 'error' => 'Invalid or expired CSRF token']));
        }
    }
}

// ============================================
// IP ALLOWLISTING
// ============================================

class IPAllowlist {
    public static function check($ip) {
        if (empty(IP_ALLOWLIST)) {
            return true; // No allowlist configured
        }
        
        foreach (IP_ALLOWLIST as $allowedCIDR) {
            if (self::ipInRange($ip, $allowedCIDR)) {
                return true;
            }
        }
        
        error_log("SECURITY: Access denied for IP $ip - not in allowlist");
        http_response_code(403);
        die(json_encode(['success' => false, 'error' => 'Access denied']));
    }
    
    private static function ipInRange($ip, $range) {
        if (strpos($range, '/') === false) {
            // Single IP
            return $ip === $range;
        }
        
        list($subnet, $bits) = explode('/', $range);
        $ip_long = ip2long($ip);
        $subnet_long = ip2long($subnet);
        $mask = -1 << (32 - $bits);
        $subnet_long &= $mask;
        
        return ($ip_long & $mask) == $subnet_long;
    }
}

// ============================================
// SECURITY HEADERS
// ============================================

function setSecurityHeaders() {
    foreach (SECURITY_HEADERS as $header => $value) {
        header("$header: $value");
    }
    
    
    // HSTS (if HTTPS)
    if (isset($_SERVER['HTTPS']) && $_SERVER['HTTPS'] === 'on') {
        header('Strict-Transport-Security: max-age=31536000; includeSubDomains; preload');
    }
}

// ============================================
// INITIALIZATION
// ============================================

// Apply security immediately
$clientIP = $_SERVER['REMOTE_ADDR'] ?? 'unknown';

// Set security headers
setSecurityHeaders();

// Check IP allowlist
IPAllowlist::check($clientIP);

// Rate limiting
RateLimiter::check($clientIP);

// Brute force check (for login pages)
if (strpos($_SERVER['REQUEST_URI'], 'login') !== false) {
    BruteForceProtection::checkBan($clientIP);
}

// Cleanup old records (1% chance)
if (random_int(1, 100) === 1) {
    BruteForceProtection::cleanup();
}

// Session security enhancements
if (session_status() === PHP_SESSION_NONE) {
    ini_set('session.cookie_httponly', 1);
    ini_set('session.cookie_secure', 1);  // HTTPS only
    ini_set('session.cookie_samesite', 'Strict');
    ini_set('session.use_strict_mode', 1);
    session_start();
}

// Session hijacking protection
if (!isset($_SESSION['user_agent'])) {
    $_SESSION['user_agent'] = $_SERVER['HTTP_USER_AGENT'] ?? '';
} else {
    if ($_SESSION['user_agent'] !== ($_SERVER['HTTP_USER_AGENT'] ?? '')) {
        error_log("SECURITY: Session hijacking attempt detected");
        session_unset();
        session_destroy();
        http_response_code(403);
        die(json_encode(['success' => false, 'error' => 'Session validation failed']));
    }
}

// Atomic file write helper to prevent race conditions
function atomicFileWrite($path, $content, $mode = 0644) {
    $tempFile = $path . '.' . uniqid('tmp', true);
    
    // Write to temp file with exclusive lock
    $fp = fopen($tempFile, 'wb');
    if (!$fp) {
        throw new Exception("Cannot create temp file for atomic write");
    }
    
    if (!flock($fp, LOCK_EX)) {
        fclose($fp);
        @unlink($tempFile);
        throw new Exception("Cannot lock temp file for atomic write");
    }
    
    if (fwrite($fp, $content) === false) {
        flock($fp, LOCK_UN);
        fclose($fp);
        @unlink($tempFile);
        throw new Exception("Cannot write to temp file");
    }
    
    fflush($fp);
    flock($fp, LOCK_UN);
    fclose($fp);
    
    chmod($tempFile, $mode);
    
    // Atomic rename (POSIX guarantees atomicity)
    if (!rename($tempFile, $path)) {
        @unlink($tempFile);
        throw new Exception("Cannot rename temp file to final location");
    }
    
    return true;
}

// CSRF token for state-changing requests
if ($_SERVER['REQUEST_METHOD'] === 'POST' || $_SERVER['REQUEST_METHOD'] === 'PUT' || $_SERVER['REQUEST_METHOD'] === 'DELETE') {
    // Skip CSRF for login (it has its own protection)
    if (strpos($_SERVER['REQUEST_URI'], 'login.php') === false) {
        CSRFProtection::requireToken();
    }
}
