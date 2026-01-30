<?php
/**
 * User Management API with RBAC
 * Requires admin role
 */

require_once __DIR__ . '/../../includes/auth.php';
requireAuth();
requireAdmin(); // Only admins can manage users

header('Content-Type: application/json');

try {
    $method = $_SERVER['REQUEST_METHOD'];
    $db = getDB();
    
    if ($method === 'GET') {
        $action = $_GET['action'] ?? 'list';
        
        if ($action === 'list') {
            // List all users with roles
            $stmt = $db->query('SELECT id, username, email, role, created_at, last_login FROM users ORDER BY id');
            $users = $stmt->fetchAll();
            
            echo json_encode(['success' => true, 'users' => $users]);
            
        } elseif ($action === 'get') {
            $userId = intval($_GET['id'] ?? 0);
            
            if (!$userId) {
                throw new Exception('User ID required');
            }
            
            $stmt = $db->prepare('SELECT id, username, email, role, created_at, last_login FROM users WHERE id = ?');
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
            // Create new user with role
            $username = validateInput($input['username'] ?? '', 'username');
            $password = $input['password'] ?? '';
            $email = validateInput($input['email'] ?? '', 'email');
            $role = $input['role'] ?? 'user';
            
            // Validation
            if (!$username || !$password) {
                throw new Exception('Username and password required');
            }
            
            if (!preg_match('/^[a-zA-Z0-9_-]{3,32}$/', $username)) {
                throw new Exception('Username must be 3-32 characters (alphanumeric, dash, underscore only)');
            }
            
            if (strlen($password) < 8) {
                throw new Exception('Password must be at least 8 characters');
            }
            
            if (!in_array($role, ['admin', 'user', 'readonly'], true)) {
                throw new Exception('Invalid role. Must be: admin, user, or readonly');
            }
            
            // Check if username exists
            $stmt = $db->prepare('SELECT id FROM users WHERE username = ?');
            $stmt->execute([$username]);
            if ($stmt->fetch()) {
                throw new Exception('Username already exists');
            }
            
            // Hash password
            $hashedPassword = password_hash($password, PASSWORD_DEFAULT);
            
            // Create user in database with role
            $stmt = $db->prepare('INSERT INTO users (username, password, email, role) VALUES (?, ?, ?, ?)');
            $stmt->execute([$username, $hashedPassword, $email, $role]);
            
            $userId = $db->lastInsertId();
            
            // Create system user for SMB/NFS access (only if not readonly)
            if ($role !== 'readonly') {
                $output = [];
                $cmd = 'sudo /opt/dplaneos/system/scripts/smb-user-add.sh ' . escapeshellarg($username) . ' ' . escapeshellarg($password);
                execCommand($cmd, $output, $ret);
                
                if ($ret !== 0) {
                    // Log warning but don't fail
                    error_log("Warning: Failed to create system user for $username: " . implode("\n", $output));
                }
            }
            
            logAction('user_create', 'user', $username, "Created user with role: $role");
            
            echo json_encode([
                'success' => true,
                'message' => 'User created successfully',
                'user_id' => $userId
            ]);
            
        } elseif ($action === 'update') {
            // Update user
            $userId = intval($input['id'] ?? 0);
            if (!$userId) {
                throw new Exception('User ID required');
            }
            
            // Get current user data
            $stmt = $db->prepare('SELECT * FROM users WHERE id = ?');
            $stmt->execute([$userId]);
            $user = $stmt->fetch();
            
            if (!$user) {
                throw new Exception('User not found');
            }
            
            // Prevent modifying own role
            if ($userId == $_SESSION['user_id'] && isset($input['role'])) {
                throw new Exception('Cannot modify your own role');
            }
            
            // Prevent removing admin role from last admin
            if (isset($input['role']) && $input['role'] !== 'admin' && $user['role'] === 'admin') {
                $adminCount = $db->query("SELECT COUNT(*) as count FROM users WHERE role = 'admin'")->fetch();
                if ($adminCount['count'] <= 1) {
                    throw new Exception('Cannot remove admin role from last administrator');
                }
            }
            
            $updates = [];
            $params = [];
            
            if (isset($input['email'])) {
                $email = validateInput($input['email'], 'email');
                $updates[] = 'email = ?';
                $params[] = $email;
            }
            
            if (isset($input['role'])) {
                $role = $input['role'];
                if (!in_array($role, ['admin', 'user', 'readonly'], true)) {
                    throw new Exception('Invalid role');
                }
                $updates[] = 'role = ?';
                $params[] = $role;
            }
            
            if (empty($updates)) {
                throw new Exception('No fields to update');
            }
            
            $params[] = $userId;
            $sql = 'UPDATE users SET ' . implode(', ', $updates) . ' WHERE id = ?';
            $stmt = $db->prepare($sql);
            $stmt->execute($params);
            
            logAction('user_update', 'user', $user['username'], 'Updated: ' . implode(', ', array_keys($input)));
            
            echo json_encode(['success' => true, 'message' => 'User updated']);
            
        } elseif ($action === 'change_password') {
            $userId = intval($input['id'] ?? 0);
            $newPassword = $input['new_password'] ?? '';
            
            if (!$userId || !$newPassword) {
                throw new Exception('User ID and password required');
            }
            
            if (strlen($newPassword) < 8) {
                throw new Exception('Password must be at least 8 characters');
            }
            
            // Get username
            $stmt = $db->prepare('SELECT username, role FROM users WHERE id = ?');
            $stmt->execute([$userId]);
            $user = $stmt->fetch();
            
            if (!$user) {
                throw new Exception('User not found');
            }
            
            // Hash new password
            $hashedPassword = password_hash($newPassword, PASSWORD_DEFAULT);
            
            // Update database
            $stmt = $db->prepare('UPDATE users SET password = ? WHERE id = ?');
            $stmt->execute([$hashedPassword, $userId]);
            
            // Update system user password (if not readonly)
            if ($user['role'] !== 'readonly') {
                $output = [];
                $cmd = '(echo ' . escapeshellarg($newPassword) . '; echo ' . escapeshellarg($newPassword) . ') | sudo smbpasswd -a -s ' . escapeshellarg($user['username']);
                execCommand($cmd, $output);
            }
            
            logAction('user_update', 'user', $user['username'], 'Password changed');
            
            echo json_encode(['success' => true, 'message' => 'Password changed']);
            
        } elseif ($action === 'delete') {
            $userId = intval($input['id'] ?? 0);
            
            if (!$userId) {
                throw new Exception('User ID required');
            }
            
            // Prevent self-deletion
            if ($userId == $_SESSION['user_id']) {
                throw new Exception('Cannot delete your own account');
            }
            
            // Get user data
            $stmt = $db->prepare('SELECT * FROM users WHERE id = ?');
            $stmt->execute([$userId]);
            $user = $stmt->fetch();
            
            if (!$user) {
                throw new Exception('User not found');
            }
            
            // Prevent deleting last admin
            if ($user['role'] === 'admin') {
                $adminCount = $db->query("SELECT COUNT(*) as count FROM users WHERE role = 'admin'")->fetch();
                if ($adminCount['count'] <= 1) {
                    throw new Exception('Cannot delete the last administrator');
                }
            }
            
            // Delete from database (cascades will clean up related data)
            $stmt = $db->prepare('DELETE FROM users WHERE id = ?');
            $stmt->execute([$userId]);
            
            // Delete system user (if exists)
            if ($user['role'] !== 'readonly') {
                $output = [];
                $cmd = 'sudo /opt/dplaneos/system/scripts/smb-user-del.sh ' . escapeshellarg($user['username']);
                execCommand($cmd, $output, $ret);
                
                if ($ret !== 0) {
                    error_log("Warning: Failed to delete system user for {$user['username']}: " . implode("\n", $output));
                }
            }
            
            logAction('user_delete', 'user', $user['username'], 'User deleted');
            
            echo json_encode(['success' => true, 'message' => 'User deleted']);
        }
    }
    
} catch (Exception $e) {
    http_response_code(400);
    echo json_encode(['success' => false, 'error' => $e->getMessage()]);
}
