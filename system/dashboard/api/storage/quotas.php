<?php
require_once __DIR__ . '/../../includes/auth.php';
requireAuth();
header('Content-Type: application/json');

try {
    $method = $_SERVER['REQUEST_METHOD'];
    
    if ($method === 'GET') {
        $action = $_GET['action'] ?? 'list';
        
        if ($action === 'list') {
            // List all quotas
            $db = getDB();
            $stmt = $db->query('SELECT * FROM user_quotas ORDER BY dataset_path, username');
            $quotas = [];
            while ($row = $stmt->fetch()) {
                // Get current usage from ZFS
                $output = [];
                $userquota = escapeshellarg($row['username']);
                $dataset = escapeshellarg($row['dataset_path']);
                $cmd = "sudo /usr/sbin/zfs get -H -o value userused@{$userquota} {$dataset} 2>/dev/null";
                execCommand($cmd, $output, $ret);
                
                $used_bytes = 0;
                if ($ret === 0 && !empty($output[0])) {
                    $used_str = trim($output[0]);
                    // Convert ZFS size format to bytes
                    $used_bytes = convertToBytes($used_str);
                }
                
                $row['used_bytes'] = $used_bytes;
                $row['used_formatted'] = formatBytes($used_bytes);
                $row['quota_formatted'] = formatBytes($row['quota_bytes']);
                $row['usage_percent'] = $row['quota_bytes'] > 0 ? round(($used_bytes / $row['quota_bytes']) * 100, 1) : 0;
                
                $quotas[] = $row;
            }
            
            echo json_encode(['success' => true, 'quotas' => $quotas]);
            
        } elseif ($action === 'get_by_dataset') {
            $datasetPath = $_GET['dataset_path'] ?? '';
            if (!$datasetPath) {
                throw new Exception('Dataset path required');
            }
            
            $db = getDB();
            $stmt = $db->prepare('SELECT * FROM user_quotas WHERE dataset_path = ? ORDER BY username');
            $stmt->execute([$datasetPath]);
            $quotas = [];
            while ($row = $stmt->fetch()) {
                // Get current usage
                $output = [];
                $userquota = escapeshellarg($row['username']);
                $dataset = escapeshellarg($datasetPath);
                $cmd = "sudo /usr/sbin/zfs get -H -o value userused@{$userquota} {$dataset} 2>/dev/null";
                execCommand($cmd, $output, $ret);
                
                $used_bytes = 0;
                if ($ret === 0 && !empty($output[0])) {
                    $used_str = trim($output[0]);
                    $used_bytes = convertToBytes($used_str);
                }
                
                $row['used_bytes'] = $used_bytes;
                $row['used_formatted'] = formatBytes($used_bytes);
                $row['quota_formatted'] = formatBytes($row['quota_bytes']);
                $row['usage_percent'] = $row['quota_bytes'] > 0 ? round(($used_bytes / $row['quota_bytes']) * 100, 1) : 0;
                
                $quotas[] = $row;
            }
            
            echo json_encode(['success' => true, 'quotas' => $quotas]);
        }
        
    } elseif ($method === 'POST') {
        $input = json_decode(file_get_contents('php://input'), true);
        $action = $input['action'] ?? '';
        
        if ($action === 'create') {
            // Create new quota
            $username = validateInput($input['username'] ?? '', 'name');
            $datasetPath = validateInput($input['dataset_path'] ?? '', 'path');
            $quotaSize = validateInput($input['quota_size'] ?? '', 'integer');
            $quotaUnit = validateInput($input['quota_unit'] ?? 'GB', 'name');
            
            if (!$username || !$datasetPath || !$quotaSize) {
                throw new Exception('Username, dataset path, and quota size required');
            }
            
            // Verify dataset exists
            $output = [];
            $cmd = 'sudo /usr/sbin/zfs list -H ' . escapeshellarg($datasetPath);
            if (!execCommand($cmd, $output, $ret)) {
                throw new Exception('Dataset does not exist: ' . $datasetPath);
            }
            
            // Convert to bytes
            $quotaBytes = convertSizeToBytes($quotaSize, $quotaUnit);
            
            // Set ZFS user quota
            $userquota = escapeshellarg($username);
            $quotaStr = $quotaSize . $quotaUnit;
            $dataset = escapeshellarg($datasetPath);
            $cmd = "sudo /usr/sbin/zfs set userquota@{$userquota}={$quotaStr} {$dataset}";
            
            if (!execCommand($cmd, $output, $ret)) {
                throw new Exception('Failed to set ZFS quota: ' . implode("\n", $output));
            }
            
            // Store in database
            $db = getDB();
            $stmt = $db->prepare('INSERT INTO user_quotas (username, dataset_path, quota_bytes, enabled)
                VALUES (?, ?, ?, 1)');
            $stmt->execute([$username, $datasetPath, $quotaBytes]);
            
            // Audit log
            auditLog('create', 'user_quota', "{$username}@{$datasetPath}", 
                "Set quota: {$quotaSize}{$quotaUnit}");
            
            echo json_encode(['success' => true, 'message' => 'Quota created successfully']);
            
        } elseif ($action === 'update') {
            $id = validateInput($input['id'] ?? '', 'integer');
            $quotaSize = validateInput($input['quota_size'] ?? '', 'integer');
            $quotaUnit = validateInput($input['quota_unit'] ?? 'GB', 'name');
            
            if (!$id || !$quotaSize) {
                throw new Exception('Quota ID and size required');
            }
            
            // Get existing quota
            $db = getDB();
            $stmt = $db->prepare('SELECT * FROM user_quotas WHERE id = ?');
            $stmt->execute([$id]);
            $quota = $stmt->fetch();
            
            if (!$quota) {
                throw new Exception('Quota not found');
            }
            
            // Convert to bytes
            $quotaBytes = convertSizeToBytes($quotaSize, $quotaUnit);
            
            // Update ZFS quota
            $userquota = escapeshellarg($quota['username']);
            $quotaStr = $quotaSize . $quotaUnit;
            $dataset = escapeshellarg($quota['dataset_path']);
            $cmd = "sudo /usr/sbin/zfs set userquota@{$userquota}={$quotaStr} {$dataset}";
            
            if (!execCommand($cmd, $output, $ret)) {
                throw new Exception('Failed to update ZFS quota: ' . implode("\n", $output));
            }
            
            // Update database
            $stmt = $db->prepare('UPDATE user_quotas SET quota_bytes = ?, updated_at = CURRENT_TIMESTAMP 
                WHERE id = ?');
            $stmt->execute([$quotaBytes, $id]);
            
            // Audit log
            auditLog('update', 'user_quota', "{$quota['username']}@{$quota['dataset_path']}", 
                "Updated quota: {$quotaSize}{$quotaUnit}");
            
            echo json_encode(['success' => true, 'message' => 'Quota updated successfully']);
            
        } elseif ($action === 'delete') {
            $id = validateInput($input['id'] ?? '', 'integer');
            
            if (!$id) {
                throw new Exception('Quota ID required');
            }
            
            // Get quota details
            $db = getDB();
            $stmt = $db->prepare('SELECT * FROM user_quotas WHERE id = ?');
            $stmt->execute([$id]);
            $quota = $stmt->fetch();
            
            if (!$quota) {
                throw new Exception('Quota not found');
            }
            
            // Remove ZFS quota
            $userquota = escapeshellarg($quota['username']);
            $dataset = escapeshellarg($quota['dataset_path']);
            $cmd = "sudo /usr/sbin/zfs set userquota@{$userquota}=none {$dataset}";
            
            if (!execCommand($cmd, $output, $ret)) {
                throw new Exception('Failed to remove ZFS quota: ' . implode("\n", $output));
            }
            
            // Delete from database
            $stmt = $db->prepare('DELETE FROM user_quotas WHERE id = ?');
            $stmt->execute([$id]);
            
            // Audit log
            auditLog('delete', 'user_quota', "{$quota['username']}@{$quota['dataset_path']}", 
                "Removed quota");
            
            echo json_encode(['success' => true, 'message' => 'Quota deleted successfully']);
            
        } elseif ($action === 'toggle') {
            $id = validateInput($input['id'] ?? '', 'integer');
            
            if (!$id) {
                throw new Exception('Quota ID required');
            }
            
            // Get quota
            $db = getDB();
            $stmt = $db->prepare('SELECT * FROM user_quotas WHERE id = ?');
            $stmt->execute([$id]);
            $quota = $stmt->fetch();
            
            if (!$quota) {
                throw new Exception('Quota not found');
            }
            
            $newEnabled = $quota['enabled'] ? 0 : 1;
            
            // Toggle ZFS quota
            $userquota = escapeshellarg($quota['username']);
            $dataset = escapeshellarg($quota['dataset_path']);
            
            if ($newEnabled) {
                // Enable - set quota
                $quotaStr = formatBytes($quota['quota_bytes']);
                $cmd = "sudo /usr/sbin/zfs set userquota@{$userquota}={$quotaStr} {$dataset}";
            } else {
                // Disable - set to none
                $cmd = "sudo /usr/sbin/zfs set userquota@{$userquota}=none {$dataset}";
            }
            
            if (!execCommand($cmd, $output, $ret)) {
                throw new Exception('Failed to toggle ZFS quota: ' . implode("\n", $output));
            }
            
            // Update database
            $stmt = $db->prepare('UPDATE user_quotas SET enabled = ?, updated_at = CURRENT_TIMESTAMP 
                WHERE id = ?');
            $stmt->execute([$newEnabled, $id]);
            
            // Audit log
            auditLog('update', 'user_quota', "{$quota['username']}@{$quota['dataset_path']}", 
                $newEnabled ? 'Enabled quota' : 'Disabled quota');
            
            echo json_encode(['success' => true, 'message' => 'Quota toggled successfully']);
        }
    }
    
} catch (Exception $e) {
    http_response_code(400);
    echo json_encode(['success' => false, 'error' => $e->getMessage()]);
}

// Helper function to convert ZFS size format to bytes
function convertToBytes($sizeStr) {
    if ($sizeStr === '0' || $sizeStr === '-') return 0;
    
    // Remove any whitespace
    $sizeStr = trim($sizeStr);
    
    // Extract number and unit
    if (preg_match('/^([\d.]+)([KMGTPE])?$/i', $sizeStr, $matches)) {
        $number = floatval($matches[1]);
        $unit = isset($matches[2]) ? strtoupper($matches[2]) : '';
        
        $multipliers = [
            'K' => 1024,
            'M' => 1024 * 1024,
            'G' => 1024 * 1024 * 1024,
            'T' => 1024 * 1024 * 1024 * 1024,
            'P' => 1024 * 1024 * 1024 * 1024 * 1024,
            'E' => 1024 * 1024 * 1024 * 1024 * 1024 * 1024
        ];
        
        return (int)($number * ($multipliers[$unit] ?? 1));
    }
    
    return 0;
}

// Helper function to convert size and unit to bytes
function convertSizeToBytes($size, $unit) {
    $multipliers = [
        'KB' => 1024,
        'MB' => 1024 * 1024,
        'GB' => 1024 * 1024 * 1024,
        'TB' => 1024 * 1024 * 1024 * 1024
    ];
    
    return (int)($size * ($multipliers[strtoupper($unit)] ?? 1));
}

// Helper function to format bytes to human-readable format
function formatBytes($bytes) {
    if ($bytes == 0) return '0B';
    
    $units = ['B', 'KB', 'MB', 'GB', 'TB', 'PB'];
    $power = floor(log($bytes, 1024));
    $power = min($power, count($units) - 1);
    
    return round($bytes / pow(1024, $power), 2) . $units[$power];
}
?>