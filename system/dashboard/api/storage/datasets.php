<?php
require_once __DIR__ . '/../../includes/auth.php';
requireAuth();
header('Content-Type: application/json');

try {
    $method = $_SERVER['REQUEST_METHOD'];
    
    if ($method === 'GET') {
        $action = $_GET['action'] ?? 'list';
        
        if ($action === 'list') {
            // List all datasets
            $output = [];
            execCommand('sudo /usr/sbin/zfs list -H -o name,used,avail,refer,mountpoint,type', $output);
            
            $datasets = [];
            foreach ($output as $line) {
                if (empty(trim($line))) continue;
                $parts = preg_split('/\s+/', trim($line), 6);
                if (count($parts) >= 6) {
                    $datasets[] = [
                        'name' => $parts[0],
                        'used' => $parts[1],
                        'avail' => $parts[2],
                        'refer' => $parts[3],
                        'mountpoint' => $parts[4],
                        'type' => $parts[5],
                        'depth' => substr_count($parts[0], '/')
                    ];
                }
            }
            
            echo json_encode(['success' => true, 'datasets' => $datasets]);
            
        } elseif ($action === 'properties') {
            // Get all properties for a dataset
            $name = validateInput($_GET['name'] ?? '', 'dataset_name');
            if (!$name) throw new Exception('Dataset name required');
            
            $output = [];
            execCommand('sudo /usr/sbin/zfs get all ' . escapeshellarg($name) . ' -H', $output);
            
            $properties = [];
            foreach ($output as $line) {
                $parts = preg_split('/\s+/', trim($line), 4);
                if (count($parts) >= 4) {
                    $properties[$parts[1]] = [
                        'value' => $parts[2],
                        'source' => $parts[3]
                    ];
                }
            }
            
            echo json_encode(['success' => true, 'properties' => $properties]);
            
        } else {
            throw new Exception('Unknown action');
        }
        
    } elseif ($method === 'POST') {
        $input = json_decode(file_get_contents('php://input'), true);
        $action = $input['action'] ?? '';
        
        if ($action === 'create') {
            $name = validateInput($input['name'] ?? '', 'dataset_name');
            $type = $input['type'] ?? 'filesystem';
            
            if (!$name) {
                throw new Exception('Invalid dataset name');
            }
            
            $cmd = 'sudo /usr/sbin/zfs create';
            
            if ($type === 'volume') {
                $size = $input['size'] ?? '1G';
                $cmd .= ' -V ' . escapeshellarg($size);
            }
            
            $cmd .= ' ' . escapeshellarg($name);
            
            $output = [];
            if (!execCommand($cmd, $output, $ret)) {
                throw new Exception('Dataset creation failed: ' . implode("\n", $output));
            }
            
            // Set optional properties
            if (isset($input['compression'])) {
                execCommand('sudo /usr/sbin/zfs set compression=' . escapeshellarg($input['compression']) . ' ' . escapeshellarg($name), $output);
            }
            if (isset($input['quota'])) {
                execCommand('sudo /usr/sbin/zfs set quota=' . escapeshellarg($input['quota']) . ' ' . escapeshellarg($name), $output);
            }
            
            logAction('dataset_create', 'dataset', $name);
            
            echo json_encode(['success' => true, 'dataset' => $name]);
            
        } elseif ($action === 'destroy') {
            $name = validateInput($input['name'] ?? '', 'dataset_name');
            $recursive = $input['recursive'] ?? false;
            
            if (!$name) {
                throw new Exception('Invalid dataset name');
            }
            
            $cmd = 'sudo /usr/sbin/zfs destroy';
            if ($recursive) {
                $cmd .= ' -r';
            }
            $cmd .= ' ' . escapeshellarg($name);
            
            $output = [];
            if (!execCommand($cmd, $output, $ret)) {
                throw new Exception('Dataset destruction failed: ' . implode("\n", $output));
            }
            
            logAction('dataset_destroy', 'dataset', $name);
            
            echo json_encode(['success' => true, 'dataset' => $name]);
            
        } elseif ($action === 'snapshot') {
            $dataset = validateInput($input['dataset'] ?? '', 'dataset_name');
            $snapname = validateInput($input['snapname'] ?? '', 'dataset_name');
            
            if (!$dataset || !$snapname) {
                throw new Exception('Invalid dataset or snapshot name');
            }
            
            $fullname = $dataset . '@' . $snapname;
            $cmd = 'sudo /usr/sbin/zfs snapshot ' . escapeshellarg($fullname);
            
            $output = [];
            if (!execCommand($cmd, $output, $ret)) {
                throw new Exception('Snapshot creation failed: ' . implode("\n", $output));
            }
            
            logAction('snapshot_create', 'snapshot', $fullname);
            
            echo json_encode(['success' => true, 'snapshot' => $fullname]);
            
        } elseif ($action === 'bulk_snapshot') {
            $dataset = validateInput($input['dataset'] ?? '', 'dataset_name');
            $snapname = validateInput($input['snapname'] ?? '', 'dataset_name');
            $recursive = $input['recursive'] ?? true;
            
            if (!$dataset || !$snapname) {
                throw new Exception('Invalid dataset or snapshot name');
            }
            
            $fullname = $dataset . '@' . $snapname;
            $cmd = 'sudo /usr/sbin/zfs snapshot';
            if ($recursive) {
                $cmd .= ' -r';
            }
            $cmd .= ' ' . escapeshellarg($fullname);
            
            $output = [];
            if (!execCommand($cmd, $output, $ret)) {
                throw new Exception('Bulk snapshot failed: ' . implode("\n", $output));
            }
            
            logAction('bulk_snapshot_create', 'snapshot', $fullname);
            
            echo json_encode(['success' => true, 'snapshot' => $fullname]);
            
        } elseif ($action === 'set_property') {
            $name = validateInput($input['name'] ?? '', 'dataset_name');
            $property = $input['property'] ?? '';
            $value = $input['value'] ?? '';
            
            if (!$name || !$property) {
                throw new Exception('Invalid parameters');
            }
            
            $cmd = 'sudo /usr/sbin/zfs set ' . escapeshellarg($property . '=' . $value) . ' ' . escapeshellarg($name);
            
            $output = [];
            if (!execCommand($cmd, $output, $ret)) {
                throw new Exception('Property set failed: ' . implode("\n", $output));
            }
            
            logAction('dataset_set_property', 'dataset', $name, "$property=$value");
            
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
