<?php
require_once __DIR__ . '/../../includes/auth.php';
requireAuth();
header('Content-Type: application/json');

try {
    $method = $_SERVER['REQUEST_METHOD'];
    
    if ($method === 'GET') {
        $action = $_GET['action'] ?? 'list';
        
        if ($action === 'list') {
            // Get all disks with comprehensive health data
            $output = [];
            execCommand('sudo /usr/bin/lsblk -b -d -n -o NAME,SIZE,TYPE,MODEL,SERIAL', $output);
            
            $disks = [];
            foreach ($output as $line) {
                $parts = preg_split('/\s+/', trim($line), 5);
                if (count($parts) >= 3 && $parts[2] === 'disk') {
                    $diskName = $parts[0];
                    $diskPath = "/dev/$diskName";
                    
                    // Get SMART status
                    $smartOutput = [];
                    execCommand("sudo /usr/sbin/smartctl -H $diskPath 2>/dev/null", $smartOutput, $smartRet);
                    
                    $health = 'UNKNOWN';
                    foreach ($smartOutput as $smartLine) {
                        if (preg_match('/SMART.*:\s*(.+)/', $smartLine, $matches)) {
                            $health = trim($matches[1]);
                            break;
                        }
                    }
                    
                    // Get temperature and power-on hours
                    $smartOutput = [];
                    execCommand("sudo /usr/sbin/smartctl -A $diskPath 2>/dev/null", $smartOutput);
                    
                    $temp = null;
                    $powerOnHours = null;
                    $reallocatedSectors = null;
                    $pendingSectors = null;
                    
                    foreach ($smartOutput as $smartLine) {
                        if (preg_match('/Temperature.*:\s*(\d+)/', $smartLine, $matches)) {
                            $temp = intval($matches[1]);
                        }
                        if (preg_match('/Power_On_Hours.*\s+(\d+)$/', $smartLine, $matches)) {
                            $powerOnHours = intval($matches[1]);
                        }
                        if (preg_match('/Reallocated_Sector.*\s+(\d+)$/', $smartLine, $matches)) {
                            $reallocatedSectors = intval($matches[1]);
                        }
                        if (preg_match('/Current_Pending_Sector.*\s+(\d+)$/', $smartLine, $matches)) {
                            $pendingSectors = intval($matches[1]);
                        }
                    }
                    
                    // Check if in pool
                    $poolOutput = [];
                    execCommand('sudo /usr/sbin/zpool status 2>&1 | grep -w ' . escapeshellarg($diskName), $poolOutput, $inPoolRet);
                    
                    $poolName = null;
                    if ($inPoolRet === 0 && !empty($poolOutput)) {
                        // Try to extract pool name
                        $zpoolList = [];
                        execCommand("sudo /usr/sbin/zpool list -H -o name", $zpoolList);
                        foreach ($zpoolList as $pool) {
                            $poolStatus = [];
                            execCommand("sudo /usr/sbin/zpool status " . escapeshellarg(trim($pool)) . " 2>&1 | grep -w " . escapeshellarg($diskName), $poolStatus, $ret);
                            if ($ret === 0) {
                                $poolName = trim($pool);
                                break;
                            }
                        }
                    }
                    
                    // Determine overall status
                    $status = 'healthy';
                    if (stripos($health, 'FAIL') !== false || ($reallocatedSectors !== null && $reallocatedSectors > 0)) {
                        $status = 'critical';
                    } elseif ($temp !== null && $temp > 50) {
                        $status = 'warning';
                    } elseif ($pendingSectors !== null && $pendingSectors > 0) {
                        $status = 'warning';
                    }
                    
                    // Get or update tracking record
                    $db = getDB();
                    $stmt = $db->prepare('SELECT * FROM disk_tracking WHERE disk_path = ?');
                    $stmt->execute([$diskPath]);
                    $tracking = $stmt->fetch();
                    
                    if (!$tracking) {
                        // Create new tracking record
                        $stmt = $db->prepare('INSERT INTO disk_tracking (disk_path, disk_serial, disk_model, disk_size, in_pool, status)
                            VALUES (?, ?, ?, ?, ?, ?)');
                        $stmt->execute([$diskPath, $parts[4] ?? null, $parts[3] ?? 'Unknown', $parts[1], $poolName, $status]);
                    } else {
                        // Update last seen and status
                        $stmt = $db->prepare('UPDATE disk_tracking SET last_seen = CURRENT_TIMESTAMP, status = ?, in_pool = ? WHERE disk_path = ?');
                        $stmt->execute([$status, $poolName, $diskPath]);
                    }
                    
                    $disks[] = [
                        'name' => $diskName,
                        'path' => $diskPath,
                        'size' => $parts[1],
                        'size_human' => formatBytes($parts[1]),
                        'model' => $parts[3] ?? 'Unknown',
                        'serial' => $parts[4] ?? 'Unknown',
                        'health' => $health,
                        'temperature' => $temp,
                        'power_on_hours' => $powerOnHours,
                        'reallocated_sectors' => $reallocatedSectors,
                        'pending_sectors' => $pendingSectors,
                        'in_pool' => $poolName,
                        'status' => $status
                    ];
                }
            }
            
            echo json_encode(['success' => true, 'disks' => $disks]);
            
        } elseif ($action === 'details') {
            $diskPath = $_GET['disk'] ?? '';
            
            if (!$diskPath) {
                throw new Exception('Disk path required');
            }
            
            // Get full SMART data
            $output = [];
            execCommand("sudo /usr/sbin/smartctl -a " . escapeshellarg($diskPath), $output);
            
            // Get tracking info
            $db = getDB();
            $stmt = $db->prepare('SELECT * FROM disk_tracking WHERE disk_path = ?');
            $stmt->execute([$diskPath]);
            $tracking = $stmt->fetch();
            
            // Get maintenance log
            $stmt = $db->prepare('SELECT * FROM disk_maintenance_log WHERE disk_path = ? ORDER BY timestamp DESC LIMIT 50');
            $stmt->execute([$diskPath]);
            $maintenanceLog = [];
            while ($row = $stmt->fetch()) {
                $maintenanceLog[] = $row;
            }
            
            // Get SMART history
            $stmt = $db->prepare('SELECT * FROM smart_history WHERE disk_path = ? ORDER BY timestamp DESC LIMIT 20');
            $stmt->execute([$diskPath]);
            $smartHistory = [];
            while ($row = $stmt->fetch()) {
                $smartHistory[] = $row;
            }
            
            echo json_encode([
                'success' => true,
                'raw_output' => implode("\n", $output),
                'tracking' => $tracking,
                'maintenance_log' => $maintenanceLog,
                'smart_history' => $smartHistory
            ]);
            
        } elseif ($action === 'history') {
            $diskPath = $_GET['disk'] ?? '';
            
            if (!$diskPath) {
                throw new Exception('Disk path required');
            }
            
            $db = getDB();
            $stmt = $db->prepare('SELECT * FROM smart_history WHERE disk_path = ? ORDER BY timestamp DESC LIMIT 100');
            $stmt->execute([$diskPath]);
            $history = [];
            while ($row = $stmt->fetch()) {
                $history[] = $row;
            }
            
            echo json_encode(['success' => true, 'history' => $history]);
        }
        
    } elseif ($method === 'POST') {
        $input = json_decode(file_get_contents('php://input'), true);
        $action = $input['action'] ?? '';
        
        if ($action === 'run_test') {
            $diskPath = validateInput($input['disk'] ?? '', 'disk_path');
            $testType = $input['test_type'] ?? 'short';
            
            if (!$diskPath) {
                throw new Exception('Disk path required');
            }
            
            if (!in_array($testType, ['short', 'long', 'conveyance'])) {
                throw new Exception('Invalid test type');
            }
            
            $output = [];
            $cmd = "sudo /usr/sbin/smartctl -t $testType " . escapeshellarg($diskPath);
            
            if (!execCommand($cmd, $output, $ret)) {
                throw new Exception('Failed to start test: ' . implode("\n", $output));
            }
            
            // Log to database
            $db = getDB();
            $stmt = $db->prepare('INSERT INTO smart_history (disk_path, test_type, test_result, raw_output)
                VALUES (?, ?, ?, ?)');
            $stmt->execute([$diskPath, $testType, 'started', implode("\n", $output)]);
            
            // Log maintenance action
            $stmt = $db->prepare('INSERT INTO disk_maintenance_log (disk_path, action_type, description, performed_by)
                VALUES (?, ?, ?, ?)');
            $stmt->execute([$diskPath, 'smart_test', "Started $testType test", getCurrentUser()['username']]);
            
            // Audit log
            auditLog('test', 'disk', $diskPath, "Started $testType SMART test");
            
            echo json_encode(['success' => true, 'message' => 'Test started', 'output' => implode("\n", $output)]);
            
        } elseif ($action === 'add_note') {
            $diskPath = validateInput($input['disk'] ?? '', 'disk_path');
            $note = $input['note'] ?? '';
            
            if (!$diskPath || !$note) {
                throw new Exception('Disk path and note required');
            }
            
            $db = getDB();
            
            // Update tracking notes
            $stmt = $db->prepare('UPDATE disk_tracking SET notes = ? WHERE disk_path = ?');
            $stmt->execute([$note, $diskPath]);
            
            // Log maintenance action
            $stmt = $db->prepare('INSERT INTO disk_maintenance_log (disk_path, action_type, description, performed_by)
                VALUES (?, ?, ?, ?)');
            $stmt->execute([$diskPath, 'note', $note, getCurrentUser()['username']]);
            
            auditLog('update', 'disk', $diskPath, 'Added note');
            
            echo json_encode(['success' => true, 'message' => 'Note added']);
            
        } elseif ($action === 'mark_replacement') {
            $diskPath = validateInput($input['disk'] ?? '', 'disk_path');
            $newSerial = validateInput($input['new_serial'] ?? '', 'name');
            
            if (!$diskPath) {
                throw new Exception('Disk path required');
            }
            
            $db = getDB();
            
            // Update tracking
            $stmt = $db->prepare('UPDATE disk_tracking SET status = ?, replacement_date = CURRENT_TIMESTAMP, replaced_by = ? 
                WHERE disk_path = ?');
            $stmt->execute(['replaced', $newSerial, $diskPath]);
            
            // Log maintenance action
            $stmt = $db->prepare('INSERT INTO disk_maintenance_log (disk_path, action_type, description, performed_by, result)
                VALUES (?, ?, ?, ?, ?)');
            $stmt->execute([
                $diskPath, 
                'replacement', 
                'Disk marked as replaced', 
                getCurrentUser()['username'],
                "Replaced by disk with serial: $newSerial"
            ]);
            
            auditLog('replace', 'disk', $diskPath, "Marked as replaced");
            
            // Create notification
            createNotification(
                'Disk Replaced',
                "Disk $diskPath has been marked as replaced",
                'success',
                'disk',
                1,
                json_encode(['disk' => $diskPath, 'new_serial' => $newSerial])
            );
            
            echo json_encode(['success' => true, 'message' => 'Disk marked as replaced']);
            
        } elseif ($action === 'update_status') {
            $diskPath = validateInput($input['disk'] ?? '', 'disk_path');
            $status = $input['status'] ?? '';
            
            if (!$diskPath || !$status) {
                throw new Exception('Disk path and status required');
            }
            
            if (!in_array($status, ['healthy', 'warning', 'critical', 'failing', 'replaced'])) {
                throw new Exception('Invalid status');
            }
            
            $db = getDB();
            
            $stmt = $db->prepare('UPDATE disk_tracking SET status = ? WHERE disk_path = ?');
            $stmt->execute([$status, $diskPath]);
            
            // Log maintenance action
            $stmt = $db->prepare('INSERT INTO disk_maintenance_log (disk_path, action_type, description, performed_by)
                VALUES (?, ?, ?, ?)');
            $stmt->execute([
                $diskPath,
                'status_change',
                "Status changed to: $status",
                getCurrentUser()['username']
            ]);
            
            auditLog('update', 'disk', $diskPath, "Status changed to $status");
            
            // Create notification if critical
            if ($status === 'critical' || $status === 'failing') {
                createNotification(
                    'Disk Health Alert',
                    "Disk $diskPath status changed to $status",
                    'error',
                    'disk',
                    3,
                    json_encode(['disk' => $diskPath, 'status' => $status])
                );
            }
            
            echo json_encode(['success' => true, 'message' => 'Status updated']);
        }
    }
    
} catch (Exception $e) {
    http_response_code(400);
    echo json_encode(['success' => false, 'error' => $e->getMessage()]);
}

// Helper function to create notifications
function createNotification($title, $message, $type, $category, $priority, $details = null) {
    $db = getDB();
    $stmt = $db->prepare('INSERT INTO notifications (title, message, type, category, priority, details)
        VALUES (?, ?, ?, ?, ?, ?)');
    $stmt->execute([$title, $message, $type, $category, $priority, $details]);
}

// Helper function to format bytes
function formatBytes($bytes) {
    if ($bytes == 0) return '0B';
    $units = ['B', 'KB', 'MB', 'GB', 'TB', 'PB'];
    $power = floor(log($bytes, 1024));
    $power = min($power, count($units) - 1);
    return round($bytes / pow(1024, $power), 2) . $units[$power];
}
?>
