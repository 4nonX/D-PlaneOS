<?php
require_once __DIR__ . '/../../includes/auth.php';
requireAuth();
header('Content-Type: application/json');

// Only admin can manage users
$currentUser = getCurrentUser();
if ($currentUser['username'] !== 'admin') {
    http_response_code(403);
    echo json_encode(['success' => false, 'error' => 'Admin access required']);
    exit;
}

try {
    $method = $_SERVER['REQUEST_METHOD'];
    
    if ($method === 'GET') {
        $action = $_GET['action'] ?? 'list';
        
        if ($action === 'list') {
            // List all users
            $db = getDB();
            $stmt = $db->query('SELECT id, username, email, created_at FROM users ORDER BY username');
            
            $users = [];
            while ($row = $stmt->fetch()) {
                $users[] = $row;
            }
            
            echo json_encode(['success' => true, 'users' => $users]);
            
        } elseif ($action === 'get') {
            $userId = $_GET['id'] ?? 0;
            
            if (!$userId) {
                throw new Exception('User ID required');
            }
            
            $db = getDB();
            $stmt = $db->prepare('SELECT id, username, email, created_at FROM users WHERE id = ?');
            $stmt->execute([$userId]);
            $user = $stmt->fetch();
            
            if (!$user) {
                throw new Exception('User not found');
            }
            
            echo json_encode(['success' => true, 'user' => $user]);
        }
        
    } elseif ($method === 'POST') {
        $input = json_decode(file_get_contents('php://input'), true);
        $action = $input['action'] ?? '';
        
        if ($action === 'create') {
            $username = validateInput($input['username'] ?? '', 'username');
            $password = $input['password'] ?? '';
            $email = validateInput($input['email'] ?? '', 'email');
            
            if (!$username || !$password) {
                throw new Exception('Username and password required');
            }
            
            // Validate username format
            if (!preg_match('/^[a-zA-Z0-9_-]{3,32}$/', $username)) {
                throw new Exception('Username must be 3-32 characters (alphanumeric, dash, underscore only)');
            }
            
            // Validate password strength
            if (strlen($password) < 8) {
                throw new Exception('Password must be at least 8 characters');
            }
            
            $db = getDB();
            
            // Check if username exists
            $stmt = $db->prepare('SELECT id FROM users WHERE username = ?');
            $stmt->execute([$username]);
            if ($stmt->fetch()) {
                throw new Exception('Username already exists');
            }
            
            // Hash password
            $hashedPassword = password_hash($password, PASSWORD_DEFAULT);
            
            // Create user
            $stmt = $db->prepare('INSERT INTO users (username, password, email, created_at)
                VALUES (?, ?, ?, CURRENT_TIMESTAMP)');
            $stmt->execute([$username, $hashedPassword, $email]);
            
            $userId = $db->lastInsertId();
            
            // Create Linux user (for SMB/NFS access)
            $output = [];
            $cmd = 'sudo useradd -m -s /bin/bash ' . escapeshellarg($username);
            execCommand($cmd, $output, $ret);
            
            if ($ret === 0) {
                // Set Linux password
                $cmd = 'echo ' . escapeshellarg($username . ':' . $password) . ' | sudo chpasswd';
                execCommand($cmd, $output);
                
                // Add to SMB
                $cmd = '(echo ' . escapeshellarg($password) . '; echo ' . escapeshellarg($password) . ') | sudo smbpasswd -a -s ' . escapeshellarg($username);
                execCommand($cmd, $output);
            }
            
            auditLog('create', 'user', $username, "User created");
            
            createNotification(
                'User Created',
                "New user '$username' has been created",
                'success',
                'system',
                1
            );
            
            echo json_encode(['success' => true, 'message' => 'User created', 'user_id' => $userId]);
            
        } elseif ($action === 'update') {
            $userId = validateInput($input['id'] ?? 0, 'integer');
            $email = validateInput($input['email'] ?? '', 'email');
            
            if (!$userId) {
                throw new Exception('User ID required');
            }
            
            $db = getDB();
            
            // Get current user info
            $stmt = $db->prepare('SELECT username FROM users WHERE id = ?');
            $stmt->execute([$userId]);
            $user = $stmt->fetch();
            
            if (!$user) {
                throw new Exception('User not found');
            }
            
            // Update email
            $stmt = $db->prepare('UPDATE users SET email = ? WHERE id = ?');
            $stmt->execute([$email, $userId]);
            
            auditLog('update', 'user', $user['username'], "Updated email");
            
            echo json_encode(['success' => true, 'message' => 'User updated']);
            
        } elseif ($action === 'change_password') {
            $userId = validateInput($input['id'] ?? 0, 'integer');
            $newPassword = $input['new_password'] ?? '';
            
            if (!$userId || !$newPassword) {
                throw new Exception('User ID and password required');
            }
            
            if (strlen($newPassword) < 8) {
                throw new Exception('Password must be at least 8 characters');
            }
            
            $db = getDB();
            
            // Get username
            $stmt = $db->prepare('SELECT username FROM users WHERE id = ?');
            $stmt->execute([$userId]);
            $user = $stmt->fetch();
            
            if (!$user) {
                throw new Exception('User not found');
            }
            
            // Cannot change admin password this way
            if ($user['username'] === 'admin' && $userId != $currentUser['id']) {
                throw new Exception('Cannot change admin password');
            }
            
            // Hash new password
            $hashedPassword = password_hash($newPassword, PASSWORD_DEFAULT);
            
            // Update database
            $stmt = $db->prepare('UPDATE users SET password = ? WHERE id = ?');
            $stmt->execute([$hashedPassword, $userId]);
            
            // Update Linux password
            $cmd = 'echo ' . escapeshellarg($user['username'] . ':' . $newPassword) . ' | sudo chpasswd';
            execCommand($cmd, $output);
            
            // Update SMB password
            $cmd = '(echo ' . escapeshellarg($newPassword) . '; echo ' . escapeshellarg($newPassword) . ') | sudo smbpasswd -a -s ' . escapeshellarg($user['username']);
            execCommand($cmd, $output);
            
            auditLog('update', 'user', $user['username'], "Password changed");
            
            echo json_encode(['success' => true, 'message' => 'Password changed']);
            
        } elseif ($action === 'delete') {
            $userId = validateInput($input['id'] ?? 0, 'integer');
            
            if (!$userId) {
                throw new Exception('User ID required');
            }
            
            // Cannot delete admin
            if ($userId == 1) {
                throw new Exception('Cannot delete admin user');
            }
            
            $db = getDB();
            
            // Get username
            $stmt = $db->prepare('SELECT username FROM users WHERE id = ?');
            $stmt->execute([$userId]);
            $user = $stmt->fetch();
            
            if (!$user) {
                throw new Exception('User not found');
            }
            
            // Delete from database
            $stmt = $db->prepare('DELETE FROM users WHERE id = ?');
            $stmt->execute([$userId]);
            
            // Delete Linux user (keep home directory for safety)
            $output = [];
            $cmd = 'sudo userdel ' . escapeshellarg($user['username']);
            execCommand($cmd, $output);
            
            // Remove from SMB
            $cmd = 'sudo smbpasswd -x ' . escapeshellarg($user['username']);
            execCommand($cmd, $output);
            
            auditLog('delete', 'user', $user['username'], "User deleted");
            
            createNotification(
                'User Deleted',
                "User '{$user['username']}' has been deleted",
                'warning',
                'system',
                1
            );
            
            echo json_encode(['success' => true, 'message' => 'User deleted']);
        }
    }
    
} catch (Exception $e) {
    http_response_code(400);
    echo json_encode(['success' => false, 'error' => $e->getMessage()]);
}

function createNotification($title, $message, $type, $category, $priority, $details = null) {
    $db = getDB();
    $stmt = $db->prepare('INSERT INTO notifications (title, message, type, category, priority, details)
        VALUES (?, ?, ?, ?, ?, ?)');
    $stmt->execute([$title, $message, $type, $category, $priority, $details]);
}
?>
