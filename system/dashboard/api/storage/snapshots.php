<?php
require_once __DIR__ . '/../../includes/auth.php';
requireAuth();
header('Content-Type: application/json');

try {
    $method = $_SERVER['REQUEST_METHOD'];
    
    if ($method === 'GET') {
        $action = $_GET['action'] ?? 'list';
        
        if ($action === 'list') {
            // List all snapshot schedules
            $db = getDB();
            $stmt = $db->query('SELECT * FROM snapshot_schedules ORDER BY dataset_path, frequency');
            
            $schedules = [];
            while ($row = $stmt->fetch()) {
                // Get count of snapshots for this schedule
                $countStmt = $db->prepare('SELECT COUNT(*) as count FROM snapshot_history 
                    WHERE schedule_id = ? AND deleted_at IS NULL');
                $countStmt->execute([$row['id']]);
                $count = $countStmt->fetch()['count'];
                
                $row['current_count'] = $count;
                $schedules[] = $row;
            }
            
            echo json_encode(['success' => true, 'schedules' => $schedules]);
            
        } elseif ($action === 'snapshots') {
            // List all snapshots for a dataset
            $dataset = $_GET['dataset'] ?? '';
            
            if (!$dataset) {
                throw new Exception('Dataset path required');
            }
            
            // Get snapshots from ZFS
            $output = [];
            $cmd = 'sudo /usr/sbin/zfs list -t snapshot -H -o name,used,creation -r ' . escapeshellarg($dataset);
            execCommand($cmd, $output);
            
            $snapshots = [];
            foreach ($output as $line) {
                $parts = preg_split('/\s+/', trim($line), 3);
                if (count($parts) >= 3) {
                    list($fullName, $used, $created) = $parts;
                    
                    // Parse dataset@snapshot
                    if (strpos($fullName, '@') !== false) {
                        list($ds, $snapName) = explode('@', $fullName, 2);
                        
                        $snapshots[] = [
                            'full_name' => $fullName,
                            'dataset' => $ds,
                            'snapshot_name' => $snapName,
                            'used' => $used,
                            'created' => $created,
                            'is_auto' => strpos($snapName, 'auto-') === 0
                        ];
                    }
                }
            }
            
            echo json_encode(['success' => true, 'snapshots' => $snapshots]);
            
        } elseif ($action === 'history') {
            // Get snapshot creation/deletion history
            $dataset = $_GET['dataset'] ?? '';
            
            $db = getDB();
            if ($dataset) {
                $stmt = $db->prepare('SELECT * FROM snapshot_history 
                    WHERE dataset_path = ? 
                    ORDER BY created_at DESC LIMIT 100');
                $stmt->execute([$dataset]);
            } else {
                $stmt = $db->query('SELECT * FROM snapshot_history 
                    ORDER BY created_at DESC LIMIT 100');
            }
            
            $history = [];
            while ($row = $stmt->fetch()) {
                $history[] = $row;
            }
            
            echo json_encode(['success' => true, 'history' => $history]);
        }
        
    } elseif ($method === 'POST') {
        $input = json_decode(file_get_contents('php://input'), true);
        $action = $input['action'] ?? '';
        
        if ($action === 'create_schedule') {
            // Create new snapshot schedule
            $dataset = validateInput($input['dataset_path'] ?? '', 'path');
            $frequency = validateInput($input['frequency'] ?? '', 'name');
            $keepCount = validateInput($input['keep_count'] ?? 0, 'integer');
            
            if (!$dataset || !$frequency || $keepCount < 1) {
                throw new Exception('Dataset, frequency, and keep count required');
            }
            
            if (!in_array($frequency, ['hourly', 'daily', 'weekly', 'monthly'])) {
                throw new Exception('Invalid frequency');
            }
            
            // Verify dataset exists
            $output = [];
            $cmd = 'sudo /usr/sbin/zfs list -H ' . escapeshellarg($dataset);
            if (!execCommand($cmd, $output, $ret)) {
                throw new Exception('Dataset does not exist');
            }
            
            $db = getDB();
            $stmt = $db->prepare('INSERT INTO snapshot_schedules 
                (dataset_path, frequency, keep_count, name_prefix)
                VALUES (?, ?, ?, ?)');
            
            $prefix = 'auto-' . $frequency . '-';
            $stmt->execute([$dataset, $frequency, $keepCount, $prefix]);
            
            auditLog('create', 'snapshot_schedule', $dataset, 
                "Created $frequency schedule (keep $keepCount)");
            
            createNotification(
                'Snapshot Schedule Created',
                "Automatic $frequency snapshots enabled for $dataset (retain $keepCount)",
                'success',
                'system',
                1
            );
            
            echo json_encode(['success' => true, 'message' => 'Schedule created']);
            
        } elseif ($action === 'delete_schedule') {
            $id = validateInput($input['id'] ?? 0, 'integer');
            
            if (!$id) {
                throw new Exception('Schedule ID required');
            }
            
            $db = getDB();
            $stmt = $db->prepare('SELECT * FROM snapshot_schedules WHERE id = ?');
            $stmt->execute([$id]);
            $schedule = $stmt->fetch();
            
            if (!$schedule) {
                throw new Exception('Schedule not found');
            }
            
            // Delete schedule
            $stmt = $db->prepare('DELETE FROM snapshot_schedules WHERE id = ?');
            $stmt->execute([$id]);
            
            auditLog('delete', 'snapshot_schedule', $schedule['dataset_path'], 
                "Deleted {$schedule['frequency']} schedule");
            
            echo json_encode(['success' => true, 'message' => 'Schedule deleted']);
            
        } elseif ($action === 'run_now') {
            // Manually trigger snapshot creation for a schedule
            $id = validateInput($input['id'] ?? 0, 'integer');
            
            if (!$id) {
                throw new Exception('Schedule ID required');
            }
            
            $db = getDB();
            $stmt = $db->prepare('SELECT * FROM snapshot_schedules WHERE id = ?');
            $stmt->execute([$id]);
            $schedule = $stmt->fetch();
            
            if (!$schedule) {
                throw new Exception('Schedule not found');
            }
            
            // Create snapshot
            $timestamp = date('YmdHis');
            $snapName = $schedule['name_prefix'] . $timestamp;
            $fullName = $schedule['dataset_path'] . '@' . $snapName;
            
            $output = [];
            $cmd = 'sudo /usr/sbin/zfs snapshot ' . escapeshellarg($fullName);
            
            if (!execCommand($cmd, $output, $ret)) {
                throw new Exception('Failed to create snapshot: ' . implode("\n", $output));
            }
            
            // Log to database
            $stmt = $db->prepare('INSERT INTO snapshot_history 
                (dataset_path, snapshot_name, schedule_id)
                VALUES (?, ?, ?)');
            $stmt->execute([$schedule['dataset_path'], $snapName, $id]);
            
            // Update schedule last_run
            $stmt = $db->prepare('UPDATE snapshot_schedules 
                SET last_run = CURRENT_TIMESTAMP 
                WHERE id = ?');
            $stmt->execute([$id]);
            
            // Enforce retention policy
            enforceRetentionPolicy($id, $schedule);
            
            auditLog('create', 'snapshot', $fullName, 'Manual snapshot via schedule');
            
            echo json_encode([
                'success' => true, 
                'message' => 'Snapshot created',
                'snapshot' => $fullName
            ]);
            
        } elseif ($action === 'delete_snapshot') {
            // Delete a specific snapshot
            $fullName = validateInput($input['snapshot'] ?? '', 'name');
            
            if (!$fullName || strpos($fullName, '@') === false) {
                throw new Exception('Invalid snapshot name');
            }
            
            list($dataset, $snapName) = explode('@', $fullName, 2);
            
            $output = [];
            $cmd = 'sudo /usr/sbin/zfs destroy ' . escapeshellarg($fullName);
            
            if (!execCommand($cmd, $output, $ret)) {
                throw new Exception('Failed to delete snapshot: ' . implode("\n", $output));
            }
            
            // Update database
            $db = getDB();
            $stmt = $db->prepare('UPDATE snapshot_history 
                SET deleted_at = CURRENT_TIMESTAMP 
                WHERE dataset_path = ? AND snapshot_name = ? AND deleted_at IS NULL');
            $stmt->execute([$dataset, $snapName]);
            
            auditLog('delete', 'snapshot', $fullName, 'Snapshot deleted');
            
            echo json_encode(['success' => true, 'message' => 'Snapshot deleted']);
            
        } elseif ($action === 'toggle_schedule') {
            $id = validateInput($input['id'] ?? 0, 'integer');
            
            if (!$id) {
                throw new Exception('Schedule ID required');
            }
            
            $db = getDB();
            $stmt = $db->prepare('SELECT * FROM snapshot_schedules WHERE id = ?');
            $stmt->execute([$id]);
            $schedule = $stmt->fetch();
            
            if (!$schedule) {
                throw new Exception('Schedule not found');
            }
            
            $newEnabled = $schedule['enabled'] ? 0 : 1;
            
            $stmt = $db->prepare('UPDATE snapshot_schedules SET enabled = ? WHERE id = ?');
            $stmt->execute([$newEnabled, $id]);
            
            auditLog('update', 'snapshot_schedule', $schedule['dataset_path'], 
                $newEnabled ? 'Enabled schedule' : 'Disabled schedule');
            
            echo json_encode(['success' => true, 'message' => 'Schedule toggled']);
            
        } elseif ($action === 'rollback') {
            $dataset = validateInput($input['dataset'] ?? '', 'dataset_name');
            $snapshot = validateInput($input['snapshot'] ?? '', 'snapshot_name');
            
            if (!$dataset || !$snapshot) {
                throw new Exception('Dataset and snapshot name required');
            }
            
            $fullName = $dataset . '@' . $snapshot;
            
            // Verify snapshot exists
            $output = [];
            $cmd = 'sudo /usr/sbin/zfs list -t snapshot -H ' . escapeshellarg($fullName);
            if (!execCommand($cmd, $output, $ret)) {
                throw new Exception('Snapshot does not exist');
            }
            
            // Rollback
            $cmd = 'sudo /usr/sbin/zfs rollback -r ' . escapeshellarg($fullName);
            if (!execCommand($cmd, $output, $ret)) {
                throw new Exception('Rollback failed: ' . implode("\n", $output));
            }
            
            auditLog('rollback', 'snapshot', $fullName, 'Dataset rolled back to snapshot');
            
            echo json_encode(['success' => true, 'message' => 'Rolled back to snapshot']);
        }
    }
    
} catch (Exception $e) {
    http_response_code(400);
    echo json_encode(['success' => false, 'error' => $e->getMessage()]);
}

// Helper function to enforce retention policy
function enforceRetentionPolicy($scheduleId, $schedule) {
    $db = getDB();
    
    // Get all snapshots for this schedule
    $stmt = $db->prepare('SELECT * FROM snapshot_history 
        WHERE schedule_id = ? AND deleted_at IS NULL 
        ORDER BY created_at DESC');
    $stmt->execute([$scheduleId]);
    
    $snapshots = [];
    while ($row = $stmt->fetch()) {
        $snapshots[] = $row;
    }
    
    // If we have more than keep_count, delete oldest
    if (count($snapshots) > $schedule['keep_count']) {
        $toDelete = array_slice($snapshots, $schedule['keep_count']);
        
        foreach ($toDelete as $snap) {
            $fullName = $snap['dataset_path'] . '@' . $snap['snapshot_name'];
            
            $output = [];
            $cmd = 'sudo /usr/sbin/zfs destroy ' . escapeshellarg($fullName);
            
            if (execCommand($cmd, $output, $ret)) {
                // Mark as deleted in database
                $stmt = $db->prepare('UPDATE snapshot_history 
                    SET deleted_at = CURRENT_TIMESTAMP 
                    WHERE id = ?');
                $stmt->execute([$snap['id']]);
                
                auditLog('delete', 'snapshot', $fullName, 'Retention policy cleanup');
            }
        }
    }
}

function createNotification($title, $message, $type, $category, $priority, $details = null) {
    $db = getDB();
    $stmt = $db->prepare('INSERT INTO notifications (title, message, type, category, priority, details)
        VALUES (?, ?, ?, ?, ?, ?)');
    $stmt->execute([$title, $message, $type, $category, $priority, $details]);
}
?>
