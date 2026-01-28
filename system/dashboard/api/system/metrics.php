<?php
require_once __DIR__ . '/../../includes/auth.php';
requireAuth();
header('Content-Type: application/json');

function collectMetrics() {
    $db = getDB();
    
    // CPU usage
    $output = [];
    execCommand('top -bn1 | grep "Cpu(s)"', $output);
    if (!empty($output)) {
        preg_match('/(\d+\.\d+)\s*id/', $output[0], $matches);
        $cpuUsage = 100 - floatval($matches[1] ?? 100);
        $stmt = $db->prepare('INSERT INTO metrics_history (metric_type, value) VALUES (?, ?)');
        $stmt->execute(['cpu_usage', $cpuUsage]);
    }
    
    // Memory usage
    $output = [];
    execCommand('free -b', $output);
    if (count($output) >= 2) {
        $parts = preg_split('/\s+/', $output[1]);
        $total = floatval($parts[1] ?? 1);
        $used = floatval($parts[2] ?? 0);
        $memUsage = ($used / $total) * 100;
        $stmt = $db->prepare('INSERT INTO metrics_history (metric_type, value) VALUES (?, ?)');
        $stmt->execute(['memory_usage', $memUsage]);
    }
    
    // Pool usage
    $output = [];
    execCommand('sudo /usr/sbin/zpool list -H -p -o name,size,alloc', $output);
    foreach ($output as $line) {
        $parts = preg_split('/\s+/', trim($line));
        if (count($parts) >= 3) {
            $poolName = $parts[0];
            $size = floatval($parts[1]);
            $alloc = floatval($parts[2]);
            $usage = $size > 0 ? ($alloc / $size) * 100 : 0;
            
            $stmt = $db->prepare('INSERT INTO metrics_history (metric_type, resource_name, value) VALUES (?, ?, ?)');
            $stmt->execute(['pool_usage', $poolName, $usage]);
        }
    }
    
    // Disk temperatures
    $output = [];
    execCommand('sudo /usr/bin/lsblk -n -o NAME,TYPE', $output);
    foreach ($output as $line) {
        $parts = preg_split('/\s+/', trim($line));
        if (count($parts) >= 2 && $parts[1] === 'disk') {
            $disk = "/dev/{$parts[0]}";
            $tempOutput = [];
            execCommand("sudo /usr/sbin/smartctl -A $disk", $tempOutput);
            
            foreach ($tempOutput as $tempLine) {
                if (preg_match('/Temperature.*:\s*(\d+)/', $tempLine, $matches)) {
                    $temp = floatval($matches[1]);
                    $stmt = $db->prepare('INSERT INTO metrics_history (metric_type, resource_name, value) VALUES (?, ?, ?)');
                    $stmt->execute(['disk_temperature', $disk, $temp]);
                    break;
                }
            }
        }
    }
}

try {
    $method = $_SERVER['REQUEST_METHOD'];
    
    if ($method === 'GET') {
        $metricType = $_GET['metric_type'] ?? '';
        $resourceName = $_GET['resource_name'] ?? null;
        $hours = intval($_GET['hours'] ?? 24);
        
        if (!$metricType) {
            throw new Exception('Metric type required');
        }
        
        $db = getDB();
        $sql = 'SELECT * FROM metrics_history WHERE metric_type = ? AND timestamp > datetime("now", "-' . $hours . ' hours")';
        $params = [$metricType];
        
        if ($resourceName) {
            $sql .= ' AND resource_name = ?';
            $params[] = $resourceName;
        }
        
        $sql .= ' ORDER BY timestamp ASC';
        
        $stmt = $db->prepare($sql);
        $stmt->execute($params);
        
        $metrics = [];
        while ($row = $stmt->fetch()) {
            $metrics[] = [
                'timestamp' => $row['timestamp'],
                'value' => floatval($row['value']),
                'resource_name' => $row['resource_name']
            ];
        }
        
        echo json_encode(['success' => true, 'metrics' => $metrics]);
        
    } elseif ($method === 'POST') {
        $input = json_decode(file_get_contents('php://input'), true);
        $action = $input['action'] ?? '';
        
        if ($action === 'collect') {
            collectMetrics();
            echo json_encode(['success' => true, 'message' => 'Metrics collected']);
            
        } elseif ($action === 'cleanup') {
            // Remove metrics older than 30 days
            $db = getDB();
            $stmt = $db->exec('DELETE FROM metrics_history WHERE timestamp < datetime("now", "-30 days")');
            echo json_encode(['success' => true, 'message' => 'Old metrics cleaned']);
            
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
