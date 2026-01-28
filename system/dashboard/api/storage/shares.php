<?php
require_once __DIR__ . '/../../includes/auth.php';
require_once __DIR__ . '/../../includes/shares.php';
requireAuth();
header('Content-Type: application/json');

try {
    $method = $_SERVER['REQUEST_METHOD'];
    
    if ($method === 'GET') {
        $action = $_GET['action'] ?? 'list';
        
        if ($action === 'list') {
            // List all shares
            $db = getDB();
            $stmt = $db->query('SELECT * FROM shares ORDER BY name');
            $shares = [];
            while ($row = $stmt->fetch()) {
                $row['status'] = getShareStatus($row);
                $shares[] = $row;
            }
            
            echo json_encode(['success' => true, 'shares' => $shares]);
            
        } elseif ($action === 'smb_users') {
            // List SMB users
            $db = getDB();
            $stmt = $db->query('SELECT * FROM smb_users ORDER BY username');
            $users = [];
            while ($row = $stmt->fetch()) {
                $users[] = $row;
            }
            
            echo json_encode(['success' => true, 'users' => $users]);
        }
        
    } elseif ($method === 'POST') {
        $input = json_decode(file_get_contents('php://input'), true);
        $action = $input['action'] ?? '';
        
        if ($action === 'create') {
            // Create new share
            $name = validateInput($input['name'] ?? '', 'name');
            $datasetPath = validateInput($input['dataset_path'] ?? '', 'path');
            $shareType = validateInput($input['share_type'] ?? '', 'name');
            
            if (!$name || !$datasetPath || !$shareType) {
                throw new Exception('Name, dataset path, and share type required');
            }
            
            if (!in_array($shareType, ['smb', 'nfs'])) {
                throw new Exception('Invalid share type. Must be smb or nfs');
            }
            
            // Verify dataset exists
            $output = [];
            $cmd = 'sudo /usr/sbin/zfs list -H ' . escapeshellarg($datasetPath);
            if (!execCommand($cmd, $output, $ret)) {
                throw new Exception('Dataset does not exist: ' . $datasetPath);
            }
            
            // Get mountpoint
            $cmd = 'sudo /usr/sbin/zfs get -H -o value mountpoint ' . escapeshellarg($datasetPath);
            execCommand($cmd, $output, $ret);
            $mountpoint = trim($output[0] ?? '');
            
            if (empty($mountpoint) || $mountpoint === 'none') {
                throw new Exception('Dataset has no mountpoint');
            }
            
            $db = getDB();
            
            // Prepare share data
            $stmt = $db->prepare('INSERT INTO shares (name, dataset_path, share_type, enabled, comment,
                smb_guest_ok, smb_read_only, smb_browseable, smb_valid_users,
                nfs_allowed_networks, nfs_read_only, nfs_sync, nfs_no_root_squash)
                VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)');
            
            $stmt->execute([
                $name,
                $mountpoint, // Use mountpoint, not dataset path
                $shareType,
                $input['enabled'] ?? 1,
                $input['comment'] ?? '',
                $input['smb_guest_ok'] ?? 0,
                $input['smb_read_only'] ?? 0,
                $input['smb_browseable'] ?? 1,
                $input['smb_valid_users'] ?? '',
                $input['nfs_allowed_networks'] ?? '',
                $input['nfs_read_only'] ?? 0,
                $input['nfs_sync'] ?? 'async',
                $input['nfs_no_root_squash'] ?? 0
            ]);
            
            $shareId = $db->lastInsertId();
            
            // Apply configuration
            if ($shareType === 'smb') {
                $result = writeSambaConfig();
            } else {
                $result = writeNFSExports();
            }
            
            if (!$result['success']) {
                // Rollback
                $stmt = $db->prepare('DELETE FROM shares WHERE id = ?');
                $stmt->execute([$shareId]);
                throw new Exception($result['error']);
            }
            
            logAction('share_create', 'share', $name, $shareType);
            
            echo json_encode(['success' => true, 'id' => $shareId]);
            
        } elseif ($action === 'update') {
            // Update existing share
            $shareId = validateInput($input['id'] ?? 0, 'integer');
            
            if (!$shareId) {
                throw new Exception('Share ID required');
            }
            
            $db = getDB();
            $stmt = $db->prepare('SELECT * FROM shares WHERE id = ?');
            $stmt->execute([$shareId]);
            $share = $stmt->fetch();
            
            if (!$share) {
                throw new Exception('Share not found');
            }
            
            // Update fields
            $stmt = $db->prepare('UPDATE shares SET 
                enabled = ?, comment = ?,
                smb_guest_ok = ?, smb_read_only = ?, smb_browseable = ?, smb_valid_users = ?,
                nfs_allowed_networks = ?, nfs_read_only = ?, nfs_sync = ?, nfs_no_root_squash = ?,
                updated_at = CURRENT_TIMESTAMP
                WHERE id = ?');
            
            $stmt->execute([
                $input['enabled'] ?? $share['enabled'],
                $input['comment'] ?? $share['comment'],
                $input['smb_guest_ok'] ?? $share['smb_guest_ok'],
                $input['smb_read_only'] ?? $share['smb_read_only'],
                $input['smb_browseable'] ?? $share['smb_browseable'],
                $input['smb_valid_users'] ?? $share['smb_valid_users'],
                $input['nfs_allowed_networks'] ?? $share['nfs_allowed_networks'],
                $input['nfs_read_only'] ?? $share['nfs_read_only'],
                $input['nfs_sync'] ?? $share['nfs_sync'],
                $input['nfs_no_root_squash'] ?? $share['nfs_no_root_squash'],
                $shareId
            ]);
            
            // Apply configuration
            if ($share['share_type'] === 'smb') {
                $result = writeSambaConfig();
            } else {
                $result = writeNFSExports();
            }
            
            if (!$result['success']) {
                throw new Exception($result['error']);
            }
            
            logAction('share_update', 'share', $share['name'], 'updated');
            
            echo json_encode(['success' => true]);
            
        } elseif ($action === 'delete') {
            // Delete share
            $shareId = validateInput($input['id'] ?? 0, 'integer');
            
            if (!$shareId) {
                throw new Exception('Share ID required');
            }
            
            $db = getDB();
            $stmt = $db->prepare('SELECT * FROM shares WHERE id = ?');
            $stmt->execute([$shareId]);
            $share = $stmt->fetch();
            
            if (!$share) {
                throw new Exception('Share not found');
            }
            
            // Delete from database
            $stmt = $db->prepare('DELETE FROM shares WHERE id = ?');
            $stmt->execute([$shareId]);
            
            // Regenerate config
            if ($share['share_type'] === 'smb') {
                $result = writeSambaConfig();
            } else {
                $result = writeNFSExports();
            }
            
            if (!$result['success']) {
                throw new Exception($result['error']);
            }
            
            logAction('share_delete', 'share', $share['name'], 'deleted');
            
            echo json_encode(['success' => true]);
            
        } elseif ($action === 'smb_user_create') {
            // Create SMB user
            $username = validateInput($input['username'] ?? '', 'name');
            $password = $input['password'] ?? '';
            
            if (!$username || !$password) {
                throw new Exception('Username and password required');
            }
            
            if (strlen($password) < 6) {
                throw new Exception('Password must be at least 6 characters');
            }
            
            $result = addSambaUser($username, $password);
            
            if (!$result['success']) {
                throw new Exception($result['error']);
            }
            
            logAction('smb_user_create', 'smb_user', $username, 'created');
            
            echo json_encode(['success' => true]);
            
        } elseif ($action === 'smb_user_delete') {
            // Delete SMB user
            $username = validateInput($input['username'] ?? '', 'name');
            
            if (!$username) {
                throw new Exception('Username required');
            }
            
            $result = removeSambaUser($username);
            
            if (!$result['success']) {
                throw new Exception($result['error']);
            }
            
            logAction('smb_user_delete', 'smb_user', $username, 'deleted');
            
            echo json_encode(['success' => true]);
            
        } elseif ($action === 'smb_user_password') {
            // Change SMB user password
            $username = validateInput($input['username'] ?? '', 'name');
            $password = $input['password'] ?? '';
            
            if (!$username || !$password) {
                throw new Exception('Username and password required');
            }
            
            if (strlen($password) < 6) {
                throw new Exception('Password must be at least 6 characters');
            }
            
            $result = changeSambaPassword($username, $password);
            
            if (!$result['success']) {
                throw new Exception($result['error']);
            }
            
            logAction('smb_user_password', 'smb_user', $username, 'password changed');
            
            echo json_encode(['success' => true]);
        }
    }
} catch (Exception $e) {
    http_response_code(400);
    echo json_encode(['success' => false, 'error' => $e->getMessage()]);
}
