<?php
require_once __DIR__ . '/../../includes/auth.php';
requireAuth();
header('Content-Type: application/json');

try {
    $method = $_SERVER['REQUEST_METHOD'];
    
    if ($method === 'GET') {
        // List all disks
        $output = [];
        execCommand('sudo /usr/bin/lsblk -b -n -o NAME,SIZE,TYPE,MODEL', $output);
        
        $disks = [];
        foreach ($output as $line) {
            $parts = preg_split('/\s+/', trim($line), 4);
            if (count($parts) >= 3 && $parts[2] === 'disk') {
                $diskName = $parts[0];
                $diskPath = "/dev/$diskName";
                
                // Check if in pool
                $poolOutput = [];
                execCommand('sudo /usr/sbin/zpool status 2>&1 | grep -w ' . escapeshellarg($diskName), $poolOutput, $inPoolRet);
                
                $disks[] = [
                    'name' => $diskName,
                    'path' => $diskPath,
                    'size' => $parts[1],
                    'size_human' => formatBytes($parts[1]),
                    'model' => $parts[3] ?? 'Unknown',
                    'in_pool' => ($inPoolRet === 0)
                ];
            }
        }
        
        echo json_encode(['success' => true, 'disks' => $disks]);
        
    } elseif ($method === 'POST') {
        $input = json_decode(file_get_contents('php://input'), true);
        $action = $input['action'] ?? '';
        
        if ($action === 'smart_status') {
            $disk = validateInput($input['disk'] ?? '', 'disk_path');
            
            if (!$disk) {
                throw new Exception('Invalid disk path');
            }
            
            $output = [];
            execCommand('sudo /usr/sbin/smartctl -H ' . escapeshellarg($disk), $output, $ret);
            
            $health = 'UNKNOWN';
            foreach ($output as $line) {
                if (preg_match('/SMART.*:\s*(.+)/', $line, $matches)) {
                    $health = trim($matches[1]);
                    break;
                }
            }
            
            // Get temperature
            $temp = null;
            $output = [];
            execCommand('sudo /usr/sbin/smartctl -A ' . escapeshellarg($disk), $output);
            foreach ($output as $line) {
                if (preg_match('/Temperature.*:\s*(\d+)/', $line, $matches)) {
                    $temp = intval($matches[1]);
                    break;
                }
            }
            
            echo json_encode([
                'success' => true,
                'disk' => $disk,
                'health' => $health,
                'temperature' => $temp
            ]);
            
        } elseif ($action === 'smart_test') {
            $disk = validateInput($input['disk'] ?? '', 'disk_path');
            $testType = $input['test_type'] ?? 'short';
            
            if (!$disk) {
                throw new Exception('Invalid disk path');
            }
            
            if (!in_array($testType, ['short', 'long', 'conveyance'])) {
                throw new Exception('Invalid test type');
            }
            
            $output = [];
            $cmd = 'sudo /usr/sbin/smartctl -t ' . escapeshellarg($testType) . ' ' . escapeshellarg($disk);
            if (!execCommand($cmd, $output, $ret)) {
                throw new Exception('SMART test failed: ' . implode("\n", $output));
            }
            
            logAction('smart_test', 'disk', $disk, $testType);
            
            echo json_encode(['success' => true, 'test_type' => $testType]);
            
        } elseif ($action === 'smart_history') {
            $disk = validateInput($input['disk'] ?? '', 'disk_path');
            
            if (!$disk) {
                throw new Exception('Invalid disk path');
            }
            
            $db = getDB();
            $stmt = $db->prepare('SELECT * FROM smart_history WHERE disk_path = ? ORDER BY timestamp DESC LIMIT 50');
            $stmt->execute([$disk]);
            $history = [];
            while ($row = $stmt->fetch()) {
                $history[] = $row;
            }
            
            echo json_encode(['success' => true, 'history' => $history]);
            
        } elseif ($action === 'record_smart') {
            $disk = validateInput($input['disk'] ?? '', 'disk_path');
            
            if (!$disk) {
                throw new Exception('Invalid disk path');
            }
            
            // Get current SMART status
            $output = [];
            execCommand('sudo /usr/sbin/smartctl -a ' . escapeshellarg($disk), $output, $ret);
            $rawOutput = implode("\n", $output);
            
            $health = 'UNKNOWN';
            $temp = null;
            $hours = null;
            
            foreach ($output as $line) {
                if (preg_match('/SMART.*:\s*(.+)/', $line, $matches)) {
                    $health = trim($matches[1]);
                }
                if (preg_match('/Temperature.*:\s*(\d+)/', $line, $matches)) {
                    $temp = intval($matches[1]);
                }
                if (preg_match('/Power_On_Hours.*\s+(\d+)$/', $line, $matches)) {
                    $hours = intval($matches[1]);
                }
            }
            
            $db = getDB();
            $stmt = $db->prepare('INSERT INTO smart_history (disk_path, test_type, health_status, temperature, power_on_hours, test_result, raw_output) VALUES (?, ?, ?, ?, ?, ?, ?)');
            $stmt->execute([$disk, 'auto', $health, $temp, $hours, $health, $rawOutput]);
            
            echo json_encode(['success' => true, 'health' => $health])
            
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
