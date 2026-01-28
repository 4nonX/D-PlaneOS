<?php
require_once __DIR__ . '/../../includes/auth.php';
require_once __DIR__ . '/../../includes/rclone.php';
requireAuth();
header('Content-Type: application/json');

try {
    $method = $_SERVER['REQUEST_METHOD'];
    
    if ($method === 'GET') {
        $action = $_GET['action'] ?? 'list';
        
        if ($action === 'list_remotes') {
            $db = getDB();
            $stmt = $db->query('SELECT id, name, remote_type, enabled, created_at FROM rclone_remotes ORDER BY name');
            $remotes = [];
            while ($row = $stmt->fetch()) {
                $remotes[] = $row;
            }
            echo json_encode(['success' => true, 'remotes' => $remotes]);
            
        } elseif ($action === 'list_tasks') {
            $db = getDB();
            $stmt = $db->query('SELECT t.*, r.name as remote_name FROM rclone_tasks t 
                               LEFT JOIN rclone_remotes r ON t.remote_id = r.id 
                               ORDER BY t.name');
            $tasks = [];
            while ($row = $stmt->fetch()) {
                $tasks[] = $row;
            }
            echo json_encode(['success' => true, 'tasks' => $tasks]);
            
        } elseif ($action === 'backends') {
            echo json_encode(['success' => true, 'backends' => getRcloneBackends()]);
        }
        
    } elseif ($method === 'POST') {
        $input = json_decode(file_get_contents('php://input'), true);
        $action = $input['action'] ?? '';
        
        if ($action === 'create_remote') {
            $name = validateInput($input['name'] ?? '', 'name');
            $remoteType = validateInput($input['remote_type'] ?? '', 'name');
            $config = $input['config'] ?? [];
            
            if (!$name || !$remoteType) {
                throw new Exception('Name and remote type required');
            }
            
            $db = getDB();
            $stmt = $db->prepare('INSERT INTO rclone_remotes (name, remote_type, config) VALUES (?, ?, ?)');
            $stmt->execute([$name, $remoteType, json_encode($config)]);
            
            $remoteId = $db->lastInsertId();
            
            $result = writeRcloneConfig();
            if (!$result['success']) {
                throw new Exception('Failed to write rclone config');
            }
            
            logAction('rclone_remote_create', 'rclone_remote', $name, $remoteType);
            
            echo json_encode(['success' => true, 'id' => $remoteId]);
            
        } elseif ($action === 'delete_remote') {
            $remoteId = validateInput($input['id'] ?? 0, 'integer');
            
            if (!$remoteId) {
                throw new Exception('Remote ID required');
            }
            
            $db = getDB();
            $stmt = $db->prepare('SELECT name FROM rclone_remotes WHERE id = ?');
            $stmt->execute([$remoteId]);
            $remote = $stmt->fetch();
            
            if (!$remote) {
                throw new Exception('Remote not found');
            }
            
            $stmt = $db->prepare('DELETE FROM rclone_remotes WHERE id = ?');
            $stmt->execute([$remoteId]);
            
            writeRcloneConfig();
            
            logAction('rclone_remote_delete', 'rclone_remote', $remote['name'], 'deleted');
            
            echo json_encode(['success' => true]);
            
        } elseif ($action === 'test_remote') {
            $remoteId = validateInput($input['id'] ?? 0, 'integer');
            
            if (!$remoteId) {
                throw new Exception('Remote ID required');
            }
            
            $db = getDB();
            $stmt = $db->prepare('SELECT name FROM rclone_remotes WHERE id = ?');
            $stmt->execute([$remoteId]);
            $remote = $stmt->fetch();
            
            if (!$remote) {
                throw new Exception('Remote not found');
            }
            
            $result = testRcloneRemote($remote['name']);
            
            echo json_encode($result);
            
        } elseif ($action === 'create_task') {
            $name = validateInput($input['name'] ?? '', 'name');
            $remoteId = validateInput($input['remote_id'] ?? 0, 'integer');
            $sourcePath = $input['source_path'] ?? '';
            $destinationPath = $input['destination_path'] ?? '';
            $direction = validateInput($input['direction'] ?? '', 'name');
            $syncType = validateInput($input['sync_type'] ?? '', 'name');
            
            if (!$name || !$remoteId || !$sourcePath || !$destinationPath || !$direction || !$syncType) {
                throw new Exception('All fields required');
            }
            
            if (!in_array($direction, ['push', 'pull'])) {
                throw new Exception('Invalid direction');
            }
            
            if (!in_array($syncType, ['sync', 'copy', 'move'])) {
                throw new Exception('Invalid sync type');
            }
            
            $db = getDB();
            $stmt = $db->prepare('INSERT INTO rclone_tasks (name, remote_id, source_path, destination_path, direction, sync_type, schedule_type) 
                                 VALUES (?, ?, ?, ?, ?, ?, ?)');
            $stmt->execute([
                $name,
                $remoteId,
                $sourcePath,
                $destinationPath,
                $direction,
                $syncType,
                $input['schedule_type'] ?? 'manual'
            ]);
            
            $taskId = $db->lastInsertId();
            
            logAction('rclone_task_create', 'rclone_task', $name, "$direction $syncType");
            
            echo json_encode(['success' => true, 'id' => $taskId]);
            
        } elseif ($action === 'run_task') {
            $taskId = validateInput($input['task_id'] ?? 0, 'integer');
            
            if (!$taskId) {
                throw new Exception('Task ID required');
            }
            
            $db = getDB();
            $stmt = $db->prepare('SELECT t.*, r.* FROM rclone_tasks t 
                                 LEFT JOIN rclone_remotes r ON t.remote_id = r.id 
                                 WHERE t.id = ?');
            $stmt->execute([$taskId]);
            $task = $stmt->fetch();
            
            if (!$task) {
                throw new Exception('Task not found');
            }
            
            $result = runRcloneSync($task, $task);
            
            $status = $result['success'] ? 'success' : 'failed';
            $stmt = $db->prepare('UPDATE rclone_tasks SET last_run = CURRENT_TIMESTAMP, last_status = ? WHERE id = ?');
            $stmt->execute([$status, $taskId]);
            
            logAction('rclone_task_run', 'rclone_task', $task['name'], $status);
            
            echo json_encode($result);
            
        } elseif ($action === 'delete_task') {
            $taskId = validateInput($input['id'] ?? 0, 'integer');
            
            if (!$taskId) {
                throw new Exception('Task ID required');
            }
            
            $db = getDB();
            $stmt = $db->prepare('SELECT name FROM rclone_tasks WHERE id = ?');
            $stmt->execute([$taskId]);
            $task = $stmt->fetch();
            
            if (!$task) {
                throw new Exception('Task not found');
            }
            
            $stmt = $db->prepare('DELETE FROM rclone_tasks WHERE id = ?');
            $stmt->execute([$taskId]);
            
            logAction('rclone_task_delete', 'rclone_task', $task['name'], 'deleted');
            
            echo json_encode(['success' => true]);
        }
    }
} catch (Exception $e) {
    http_response_code(400);
    echo json_encode(['success' => false, 'error' => $e->getMessage()]);
}
