<?php
require_once __DIR__ . '/../../includes/auth.php';
requireAuth();
header('Content-Type: application/json');

function sendReplicationAlert($taskName, $error, $severity = 'critical') {
    $db = getDB();
    
    // Log alert to history
    $stmt = $db->prepare('INSERT INTO alert_history (alert_type, severity, message, details) VALUES (?, ?, ?, ?)');
    $stmt->execute([
        'replication_failure',
        $severity,
        "Replication task '$taskName' failed",
        json_encode(['task' => $taskName, 'error' => $error])
    ]);
    
    // Check if webhooks configured for replication failures
    $stmt = $db->prepare('SELECT * FROM alert_settings WHERE alert_type = ? AND enabled = 1');
    $stmt->execute(['replication_failure']);
    $settings = $stmt->fetchAll();
    
    foreach ($settings as $setting) {
        if (!empty($setting['webhook_url'])) {
            $message = "🔴 **Replication Failed**\n\n**Task:** $taskName\n**Error:** " . substr($error, 0, 200);
            
            // Send webhook
            $data = json_encode([
                'content' => $message,
                'username' => 'D-PlaneOS Alert'
            ]);
            
            $ch = curl_init($setting['webhook_url']);
            curl_setopt($ch, CURLOPT_HTTPHEADER, ['Content-Type: application/json']);
            curl_setopt($ch, CURLOPT_POST, 1);
            curl_setopt($ch, CURLOPT_POSTFIELDS, $data);
            curl_setopt($ch, CURLOPT_RETURNTRANSFER, true);
            curl_setopt($ch, CURLOPT_TIMEOUT, 10);
            curl_exec($ch);
            curl_close($ch);
        }
    }
}

try {
    $method = $_SERVER['REQUEST_METHOD'];
    
    if ($method === 'GET') {
        $action = $_GET['action'] ?? 'list';
        
        if ($action === 'list') {
            // List all replication tasks
            $db = getDB();
            $stmt = $db->query('SELECT * FROM replication_tasks ORDER BY name');
            $tasks = [];
            while ($row = $stmt->fetch()) {
                $tasks[] = $row;
            }
            
            echo json_encode(['success' => true, 'tasks' => $tasks]);
            
        } elseif ($action === 'progress') {
            // Get progress for a specific task
            $taskId = validateInput($_GET['task_id'] ?? 0, 'integer');
            
            if (!$taskId) {
                throw new Exception('Task ID required');
            }
            
            $progressFile = "/tmp/replication-{$taskId}.progress";
            
            if (!file_exists($progressFile)) {
                echo json_encode(['success' => true, 'progress' => null]);
                return;
            }
            
            $progress = json_decode(file_get_contents($progressFile), true);
            
            // Check for pv progress data
            $pvFile = "$progressFile.pv";
            if (file_exists($pvFile)) {
                $pvData = file_get_contents($pvFile);
                if (preg_match('/(\d+)/', $pvData, $matches)) {
                    $progress['percent'] = intval($matches[1]);
                }
            }
            
            echo json_encode(['success' => true, 'progress' => $progress]);
        }
        
    } elseif ($method === 'POST') {
        $input = json_decode(file_get_contents('php://input'), true);
        $action = $input['action'] ?? '';
        
        if ($action === 'create') {
            $name = validateInput($input['name'] ?? '', 'string');
            $source = validateInput($input['source_dataset'] ?? '', 'dataset_name');
            $destHost = validateInput($input['destination_host'] ?? '', 'string');
            $destDataset = validateInput($input['destination_dataset'] ?? '', 'dataset_name');
            $schedule = $input['schedule_type'] ?? 'manual';
            
            if (!$name || !$source || !$destHost || !$destDataset) {
                throw new Exception('All fields required');
            }
            
            $db = getDB();
            $stmt = $db->prepare('INSERT INTO replication_tasks (name, source_dataset, destination_host, destination_dataset, schedule_type) VALUES (?, ?, ?, ?, ?)');
            $stmt->execute([$name, $source, $destHost, $destDataset, $schedule]);
            
            logAction('replication_create', 'replication', $name);
            
            echo json_encode(['success' => true, 'task' => $name]);
            
        } elseif ($action === 'run') {
            $taskId = validateInput($input['task_id'] ?? 0, 'integer');
            
            if (!$taskId) {
                throw new Exception('Task ID required');
            }
            
            $db = getDB();
            $stmt = $db->prepare('SELECT * FROM replication_tasks WHERE id = ?');
            $stmt->execute([$taskId]);
            $task = $stmt->fetch();
            
            if (!$task) {
                throw new Exception('Task not found');
            }
            
            // Create snapshot
            $snapName = 'repl-' . date('YmdHis');
            $fullSnap = $task['source_dataset'] . '@' . $snapName;
            
            $output = [];
            $cmd = 'sudo /usr/sbin/zfs snapshot ' . escapeshellarg($fullSnap);
            if (!execCommand($cmd, $output, $ret)) {
                $error = 'Snapshot failed: ' . implode("\n", $output);
                
                // Update task status
                $stmt = $db->prepare('UPDATE replication_tasks SET last_run = CURRENT_TIMESTAMP, last_status = ? WHERE id = ?');
                $stmt->execute(['failed', $taskId]);
                
                // Send alert
                sendReplicationAlert($task['name'], $error, 'critical');
                
                throw new Exception($error);
            }
            
            // Create progress tracking file
            $progressFile = "/tmp/replication-{$taskId}.progress";
            file_put_contents($progressFile, json_encode([
                'status' => 'sending',
                'started' => time(),
                'bytes_sent' => 0,
                'total_bytes' => 0
            ]));
            
            // Send to destination (with progress tracking)
            $sendCmd = 'sudo /usr/sbin/zfs send ' . escapeshellarg($fullSnap);
            $recvCmd = 'ssh ' . escapeshellarg($task['destination_host']) . 
                       ' sudo /usr/sbin/zfs receive ' . escapeshellarg($task['destination_dataset']);
            
            $output = [];
            $cmd = "$sendCmd | pv -n 2>$progressFile.pv | $recvCmd 2>&1";
            $success = execCommand($cmd, $output, $ret);
            
            // Update progress to complete
            file_put_contents($progressFile, json_encode([
                'status' => $success ? 'complete' : 'failed',
                'finished' => time(),
                'output' => implode("\n", $output)
            ]));
            
            // Update task status
            $status = $success ? 'success' : 'failed';
            $stmt = $db->prepare('UPDATE replication_tasks SET last_run = CURRENT_TIMESTAMP, last_status = ? WHERE id = ?');
            $stmt->execute([$status, $taskId]);
            
            if (!$success) {
                $error = 'Replication failed: ' . implode("\n", $output);
                
                // Send alert
                sendReplicationAlert($task['name'], $error, 'critical');
                
                throw new Exception($error);
            }
            
            logAction('replication_run', 'replication', $task['name'], $status);
            
            // Clean up progress file after 1 hour (via background job)
            exec("(sleep 3600 && rm -f $progressFile $progressFile.pv) > /dev/null 2>&1 &");
            
            echo json_encode(['success' => true, 'status' => $status]);
            
        } elseif ($action === 'delete') {
            $taskId = validateInput($input['task_id'] ?? 0, 'integer');
            
            if (!$taskId) {
                throw new Exception('Task ID required');
            }
            
            $db = getDB();
            $stmt = $db->prepare('DELETE FROM replication_tasks WHERE id = ?');
            $stmt->execute([$taskId]);
            
            logAction('replication_delete', 'replication', "task_$taskId");
            
            echo json_encode(['success' => true]);
            
        } else {
            throw new Exception('Unknown action');
        }
    } else {
        http_response_code(405);
        throw new Exception('Method not allowed');
    }
    
} catch (Exception $e) {
    http_response_code(400);
    echo json_encode(['success' => false, 'error' => $e->getMessage()]);
}
