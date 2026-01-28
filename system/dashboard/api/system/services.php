<?php
require_once __DIR__ . '/../../includes/auth.php';
requireAuth();
header('Content-Type: application/json');

// Key services we want to monitor
$MANAGED_SERVICES = [
    'smbd' => 'Samba File Sharing',
    'nmbd' => 'Samba NetBIOS',
    'nfs-server' => 'NFS Server',
    'ssh' => 'SSH Server',
    'docker' => 'Docker Engine',
    'fail2ban' => 'Fail2ban',
    'crowdsec' => 'CrowdSec',
    'zfs-import-cache' => 'ZFS Import',
    'zfs-mount' => 'ZFS Mount',
    'zfs-share' => 'ZFS Share',
    'prometheus' => 'Prometheus',
    'grafana-server' => 'Grafana',
];

try {
    $method = $_SERVER['REQUEST_METHOD'];
    
    if ($method === 'GET') {
        $action = $_GET['action'] ?? 'list';
        
        if ($action === 'list') {
            // List all managed services with their status
            $services = [];
            
            foreach ($MANAGED_SERVICES as $service => $description) {
                $status = getServiceStatus($service);
                $services[] = [
                    'name' => $service,
                    'description' => $description,
                    'status' => $status['status'],
                    'active' => $status['active'],
                    'enabled' => $status['enabled'],
                    'running_since' => $status['running_since'],
                    'memory' => $status['memory'],
                    'pid' => $status['pid'],
                ];
            }
            
            echo json_encode(['success' => true, 'services' => $services]);
            
        } elseif ($action === 'status') {
            // Get detailed status for a single service
            $service = $_GET['service'] ?? '';
            
            if (!isset($MANAGED_SERVICES[$service])) {
                throw new Exception('Unknown service');
            }
            
            $status = getServiceStatus($service);
            $logs = getServiceLogs($service, 50);
            
            echo json_encode([
                'success' => true,
                'service' => $service,
                'description' => $MANAGED_SERVICES[$service],
                'status' => $status,
                'logs' => $logs
            ]);
            
        } elseif ($action === 'logs') {
            // Get logs for a service
            $service = $_GET['service'] ?? '';
            $lines = intval($_GET['lines'] ?? 100);
            
            if (!isset($MANAGED_SERVICES[$service])) {
                throw new Exception('Unknown service');
            }
            
            $logs = getServiceLogs($service, $lines);
            
            echo json_encode(['success' => true, 'service' => $service, 'logs' => $logs]);
            
        } else {
            throw new Exception('Unknown action');
        }
        
    } elseif ($method === 'POST') {
        $input = json_decode(file_get_contents('php://input'), true);
        $action = $input['action'] ?? '';
        $service = $input['service'] ?? '';
        
        if (!isset($MANAGED_SERVICES[$service])) {
            throw new Exception('Unknown service');
        }
        
        if ($action === 'start') {
            $cmd = 'sudo systemctl start ' . escapeshellarg($service);
            exec($cmd, $output, $ret);
            
            if ($ret !== 0) {
                throw new Exception('Failed to start service: ' . implode("\n", $output));
            }
            
            logAction('service_start', 'service', $service);
            
            echo json_encode(['success' => true, 'service' => $service, 'action' => 'started']);
            
        } elseif ($action === 'stop') {
            $cmd = 'sudo systemctl stop ' . escapeshellarg($service);
            exec($cmd, $output, $ret);
            
            if ($ret !== 0) {
                throw new Exception('Failed to stop service: ' . implode("\n", $output));
            }
            
            logAction('service_stop', 'service', $service);
            
            echo json_encode(['success' => true, 'service' => $service, 'action' => 'stopped']);
            
        } elseif ($action === 'restart') {
            $cmd = 'sudo systemctl restart ' . escapeshellarg($service);
            exec($cmd, $output, $ret);
            
            if ($ret !== 0) {
                throw new Exception('Failed to restart service: ' . implode("\n", $output));
            }
            
            logAction('service_restart', 'service', $service);
            
            echo json_encode(['success' => true, 'service' => $service, 'action' => 'restarted']);
            
        } elseif ($action === 'enable') {
            $cmd = 'sudo systemctl enable ' . escapeshellarg($service);
            exec($cmd, $output, $ret);
            
            if ($ret !== 0) {
                throw new Exception('Failed to enable service: ' . implode("\n", $output));
            }
            
            logAction('service_enable', 'service', $service);
            
            echo json_encode(['success' => true, 'service' => $service, 'action' => 'enabled']);
            
        } elseif ($action === 'disable') {
            $cmd = 'sudo systemctl disable ' . escapeshellarg($service);
            exec($cmd, $output, $ret);
            
            if ($ret !== 0) {
                throw new Exception('Failed to disable service: ' . implode("\n", $output));
            }
            
            logAction('service_disable', 'service', $service);
            
            echo json_encode(['success' => true, 'service' => $service, 'action' => 'disabled']);
            
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

function getServiceStatus($service) {
    $status = [
        'status' => 'unknown',
        'active' => false,
        'enabled' => false,
        'running_since' => null,
        'memory' => null,
        'pid' => null,
    ];
    
    // Get service status
    $cmd = 'systemctl is-active ' . escapeshellarg($service) . ' 2>&1';
    exec($cmd, $output, $ret);
    $activeState = trim($output[0] ?? 'unknown');
    $status['active'] = ($activeState === 'active');
    $status['status'] = $activeState;
    
    // Get enabled status
    $cmd = 'systemctl is-enabled ' . escapeshellarg($service) . ' 2>&1';
    exec($cmd, $output, $ret);
    $enabledState = trim($output[0] ?? 'unknown');
    $status['enabled'] = ($enabledState === 'enabled');
    
    // Get detailed status if active
    if ($status['active']) {
        $cmd = 'systemctl show ' . escapeshellarg($service) . ' --no-pager';
        exec($cmd, $output);
        
        foreach ($output as $line) {
            if (strpos($line, 'MainPID=') === 0) {
                $status['pid'] = intval(substr($line, 8));
            } elseif (strpos($line, 'ActiveEnterTimestamp=') === 0) {
                $timestamp = substr($line, 21);
                if ($timestamp && $timestamp !== 'n/a') {
                    $status['running_since'] = date('Y-m-d H:i:s', strtotime($timestamp));
                }
            } elseif (strpos($line, 'MemoryCurrent=') === 0) {
                $memory = intval(substr($line, 14));
                if ($memory > 0) {
                    $status['memory'] = formatBytes($memory);
                }
            }
        }
    }
    
    return $status;
}

function getServiceLogs($service, $lines = 50) {
    $cmd = 'journalctl -u ' . escapeshellarg($service) . ' -n ' . intval($lines) . ' --no-pager --output=short-iso 2>&1';
    exec($cmd, $output);
    
    $logs = [];
    foreach ($output as $line) {
        $logs[] = $line;
    }
    
    return $logs;
}

function formatBytes($bytes, $precision = 2) {
    $units = ['B', 'KB', 'MB', 'GB', 'TB'];
    $bytes = max($bytes, 0);
    $pow = floor(($bytes ? log($bytes) : 0) / log(1024));
    $pow = min($pow, count($units) - 1);
    $bytes /= pow(1024, $pow);
    return round($bytes, $precision) . ' ' . $units[$pow];
}
