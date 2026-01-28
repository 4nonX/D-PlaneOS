<?php
require_once __DIR__ . '/../../includes/auth.php';
requireAuth();
header('Content-Type: application/json');

try {
    $method = $_SERVER['REQUEST_METHOD'];
    
    if ($method === 'GET') {
        $action = $_GET['action'] ?? 'system';
        $lines = isset($_GET['lines']) ? (int)$_GET['lines'] : 100;
        $lines = min(max($lines, 10), 1000); // Limit between 10 and 1000
        
        if ($action === 'system') {
            // Get system log (journalctl)
            $output = [];
            $cmd = "sudo journalctl -n " . escapeshellarg($lines) . " --no-pager 2>&1";
            execCommand($cmd, $output);
            
            echo json_encode([
                'success' => true,
                'log_type' => 'system',
                'lines' => $output
            ]);
            
        } elseif ($action === 'service') {
            // Get specific service log
            $service = validateInput($_GET['service'] ?? '', 'name');
            
            if (!$service) {
                throw new Exception('Service name required');
            }
            
            // Whitelist of allowed services
            $allowedServices = [
                'smbd', 'nmbd', 'nfs-server', 'docker', 'nginx', 
                'php8.2-fpm', 'nut-server', 'nut-monitor'
            ];
            
            if (!in_array($service, $allowedServices)) {
                throw new Exception('Service not allowed');
            }
            
            $output = [];
            $cmd = "sudo journalctl -u " . escapeshellarg($service) . " -n " . escapeshellarg($lines) . " --no-pager 2>&1";
            execCommand($cmd, $output);
            
            echo json_encode([
                'success' => true,
                'log_type' => 'service',
                'service' => $service,
                'lines' => $output
            ]);
            
        } elseif ($action === 'dplaneos') {
            // Get D-PlaneOS audit log from database
            $db = getDB();
            $stmt = $db->prepare('SELECT * FROM audit_log 
                ORDER BY timestamp DESC 
                LIMIT ?');
            $stmt->execute([$lines]);
            
            $logs = [];
            while ($row = $stmt->fetch()) {
                $logs[] = [
                    'timestamp' => $row['timestamp'],
                    'user' => $row['user_id'],
                    'action' => $row['action'],
                    'resource' => $row['resource_type'] . ': ' . $row['resource_name'],
                    'details' => $row['details'],
                    'ip' => $row['ip_address']
                ];
            }
            
            echo json_encode([
                'success' => true,
                'log_type' => 'dplaneos',
                'entries' => $logs
            ]);
            
        } elseif ($action === 'zfs') {
            // Get ZFS events
            $output = [];
            $cmd = "sudo zpool events -H 2>&1 | tail -n " . escapeshellarg($lines);
            execCommand($cmd, $output);
            
            echo json_encode([
                'success' => true,
                'log_type' => 'zfs',
                'lines' => $output
            ]);
            
        } elseif ($action === 'docker') {
            // Get Docker logs for a specific container
            $container = validateInput($_GET['container'] ?? '', 'name');
            
            if (!$container) {
                throw new Exception('Container name required');
            }
            
            $output = [];
            $cmd = "sudo docker logs --tail " . escapeshellarg($lines) . " " . escapeshellarg($container) . " 2>&1";
            execCommand($cmd, $output);
            
            echo json_encode([
                'success' => true,
                'log_type' => 'docker',
                'container' => $container,
                'lines' => $output
            ]);
            
        } elseif ($action === 'available_services') {
            // List available services for viewing
            $services = [
                ['name' => 'smbd', 'description' => 'Samba SMB Server'],
                ['name' => 'nmbd', 'description' => 'Samba NetBIOS Server'],
                ['name' => 'nfs-server', 'description' => 'NFS Server'],
                ['name' => 'docker', 'description' => 'Docker Service'],
                ['name' => 'nginx', 'description' => 'Web Server'],
                ['name' => 'php8.2-fpm', 'description' => 'PHP FastCGI Process Manager'],
                ['name' => 'nut-server', 'description' => 'Network UPS Tools Server'],
                ['name' => 'nut-monitor', 'description' => 'Network UPS Tools Monitor']
            ];
            
            echo json_encode([
                'success' => true,
                'services' => $services
            ]);
            
        } elseif ($action === 'search') {
            // Search logs for a pattern
            $query = $_GET['query'] ?? '';
            $logType = $_GET['log_type'] ?? 'system';
            
            if (!$query) {
                throw new Exception('Search query required');
            }
            
            // Escape query for grep
            $escapedQuery = escapeshellarg($query);
            
            $output = [];
            if ($logType === 'system') {
                $cmd = "sudo journalctl --no-pager | grep -i $escapedQuery | tail -n " . escapeshellarg($lines);
            } elseif ($logType === 'dplaneos') {
                // Search in database
                $db = getDB();
                $stmt = $db->prepare('SELECT * FROM audit_log 
                    WHERE action LIKE ? OR resource_name LIKE ? OR details LIKE ?
                    ORDER BY timestamp DESC 
                    LIMIT ?');
                $searchPattern = '%' . $query . '%';
                $stmt->execute([$searchPattern, $searchPattern, $searchPattern, $lines]);
                
                $results = [];
                while ($row = $stmt->fetch()) {
                    $results[] = [
                        'timestamp' => $row['timestamp'],
                        'user' => $row['user_id'],
                        'action' => $row['action'],
                        'resource' => $row['resource_type'] . ': ' . $row['resource_name'],
                        'details' => $row['details']
                    ];
                }
                
                echo json_encode([
                    'success' => true,
                    'log_type' => 'dplaneos',
                    'query' => $query,
                    'results' => $results
                ]);
                exit;
            } else {
                throw new Exception('Search not supported for this log type');
            }
            
            execCommand($cmd, $output);
            
            echo json_encode([
                'success' => true,
                'log_type' => $logType,
                'query' => $query,
                'lines' => $output
            ]);
        }
    }
    
} catch (Exception $e) {
    http_response_code(400);
    echo json_encode(['success' => false, 'error' => $e->getMessage()]);
}
?>
