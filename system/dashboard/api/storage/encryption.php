<?php
require_once __DIR__ . '/../../includes/auth.php';
requireAuth();
header('Content-Type: application/json');

try {
    $method = $_SERVER['REQUEST_METHOD'];
    
    if ($method === 'GET') {
        $action = $_GET['action'] ?? 'list';
        
        if ($action === 'list') {
            // List all encrypted datasets with their encryption status
            $output = [];
            execCommand('sudo /usr/sbin/zfs get -H encryption,keystatus,encryptionroot -t filesystem,volume', $output);
            
            $encrypted = [];
            $current = null;
            
            foreach ($output as $line) {
                $parts = preg_split('/\s+/', trim($line), 4);
                if (count($parts) >= 4) {
                    $name = $parts[0];
                    $property = $parts[1];
                    $value = $parts[2];
                    
                    if ($property === 'encryption') {
                        if ($value !== 'off' && $value !== '-') {
                            $current = [
                                'name' => $name,
                                'encryption' => $value,
                                'keystatus' => 'unknown',
                                'encryptionroot' => '-'
                            ];
                        }
                    } elseif ($current && $current['name'] === $name) {
                        if ($property === 'keystatus') {
                            $current['keystatus'] = $value;
                        } elseif ($property === 'encryptionroot') {
                            $current['encryptionroot'] = $value;
                            $encrypted[] = $current;
                            $current = null;
                        }
                    }
                }
            }
            
            echo json_encode(['success' => true, 'datasets' => $encrypted]);
            
        } elseif ($action === 'status') {
            // Get encryption status for specific dataset
            $name = validateInput($_GET['name'] ?? '', 'dataset_name');
            if (!$name) throw new Exception('Dataset name required');
            
            $output = [];
            execCommand('sudo /usr/sbin/zfs get -H encryption,keystatus,encryptionroot,keyformat ' . escapeshellarg($name), $output);
            
            $status = [
                'name' => $name,
                'encrypted' => false,
                'encryption' => 'off',
                'keystatus' => '-',
                'encryptionroot' => '-',
                'keyformat' => '-'
            ];
            
            foreach ($output as $line) {
                $parts = preg_split('/\s+/', trim($line), 4);
                if (count($parts) >= 3) {
                    $property = $parts[1];
                    $value = $parts[2];
                    
                    if ($property === 'encryption' && $value !== 'off') {
                        $status['encrypted'] = true;
                        $status['encryption'] = $value;
                    } elseif ($property === 'keystatus') {
                        $status['keystatus'] = $value;
                    } elseif ($property === 'encryptionroot') {
                        $status['encryptionroot'] = $value;
                    } elseif ($property === 'keyformat') {
                        $status['keyformat'] = $value;
                    }
                }
            }
            
            echo json_encode(['success' => true, 'status' => $status]);
            
        } elseif ($action === 'pending_keys') {
            // List datasets that need keys loaded (keystatus = unavailable)
            $output = [];
            execCommand('sudo /usr/sbin/zfs get -H keystatus -t filesystem,volume', $output);
            
            $pending = [];
            foreach ($output as $line) {
                $parts = preg_split('/\s+/', trim($line), 4);
                if (count($parts) >= 3 && $parts[2] === 'unavailable') {
                    $pending[] = $parts[0];
                }
            }
            
            echo json_encode(['success' => true, 'datasets' => $pending]);
            
        } else {
            throw new Exception('Unknown action');
        }
        
    } elseif ($method === 'POST') {
        $input = json_decode(file_get_contents('php://input'), true);
        $action = $input['action'] ?? '';
        
        if ($action === 'create_encrypted') {
            // Create new encrypted dataset
            $name = validateInput($input['name'] ?? '', 'dataset_name');
            $password = $input['password'] ?? '';
            $encryption = $input['encryption'] ?? 'aes-256-gcm';
            $keyformat = $input['keyformat'] ?? 'passphrase';
            
            if (!$name) throw new Exception('Dataset name required');
            if (!$password) throw new Exception('Password required for encryption');
            
            // Validate encryption algorithm
            $validAlgorithms = ['aes-128-ccm', 'aes-192-ccm', 'aes-256-ccm', 'aes-128-gcm', 'aes-192-gcm', 'aes-256-gcm'];
            if (!in_array($encryption, $validAlgorithms)) {
                throw new Exception('Invalid encryption algorithm');
            }
            
            // Create encrypted dataset
            $cmd = 'echo ' . escapeshellarg($password) . ' | sudo /usr/sbin/zfs create -o encryption=' . escapeshellarg($encryption) . ' -o keyformat=' . escapeshellarg($keyformat);
            
            // Optional properties
            if (isset($input['quota'])) {
                $cmd .= ' -o quota=' . escapeshellarg($input['quota']);
            }
            if (isset($input['compression'])) {
                $cmd .= ' -o compression=' . escapeshellarg($input['compression']);
            }
            
            $cmd .= ' ' . escapeshellarg($name);
            
            $output = [];
            if (!execCommand($cmd, $output, $ret)) {
                throw new Exception('Failed to create encrypted dataset: ' . implode("\n", $output));
            }
            
            logAction('encryption_create', 'dataset', $name, "encryption=$encryption");
            
            echo json_encode(['success' => true, 'dataset' => $name]);
            
        } elseif ($action === 'load_key') {
            // Load encryption key for dataset
            $name = validateInput($input['name'] ?? '', 'dataset_name');
            $password = $input['password'] ?? '';
            
            if (!$name) throw new Exception('Dataset name required');
            if (!$password) throw new Exception('Password required');
            
            $cmd = 'echo ' . escapeshellarg($password) . ' | sudo /usr/sbin/zfs load-key ' . escapeshellarg($name);
            
            $output = [];
            if (!execCommand($cmd, $output, $ret)) {
                throw new Exception('Failed to load key: ' . implode("\n", $output));
            }
            
            // Mount the dataset after loading key
            execCommand('sudo /usr/sbin/zfs mount ' . escapeshellarg($name), $output);
            
            logAction('encryption_load_key', 'dataset', $name);
            
            echo json_encode(['success' => true, 'dataset' => $name]);
            
        } elseif ($action === 'unload_key') {
            // Unload encryption key (requires unmounting first)
            $name = validateInput($input['name'] ?? '', 'dataset_name');
            
            if (!$name) throw new Exception('Dataset name required');
            
            // Unmount first
            $output = [];
            execCommand('sudo /usr/sbin/zfs unmount ' . escapeshellarg($name), $output);
            
            // Unload key
            $cmd = 'sudo /usr/sbin/zfs unload-key ' . escapeshellarg($name);
            if (!execCommand($cmd, $output, $ret)) {
                throw new Exception('Failed to unload key: ' . implode("\n", $output));
            }
            
            logAction('encryption_unload_key', 'dataset', $name);
            
            echo json_encode(['success' => true, 'dataset' => $name]);
            
        } elseif ($action === 'change_key') {
            // Change encryption password
            $name = validateInput($input['name'] ?? '', 'dataset_name');
            $oldPassword = $input['old_password'] ?? '';
            $newPassword = $input['new_password'] ?? '';
            
            if (!$name) throw new Exception('Dataset name required');
            if (!$oldPassword) throw new Exception('Old password required');
            if (!$newPassword) throw new Exception('New password required');
            
            // First verify old password by loading key
            $cmd = 'echo ' . escapeshellarg($oldPassword) . ' | sudo /usr/sbin/zfs load-key ' . escapeshellarg($name);
            $output = [];
            if (!execCommand($cmd, $output, $ret)) {
                throw new Exception('Incorrect old password');
            }
            
            // Change key
            $cmd = 'echo ' . escapeshellarg($newPassword) . ' | sudo /usr/sbin/zfs change-key -o keyformat=passphrase ' . escapeshellarg($name);
            if (!execCommand($cmd, $output, $ret)) {
                throw new Exception('Failed to change key: ' . implode("\n", $output));
            }
            
            logAction('encryption_change_key', 'dataset', $name);
            
            echo json_encode(['success' => true, 'dataset' => $name]);
            
        } elseif ($action === 'load_all_keys') {
            // Attempt to load keys for all encrypted datasets with single password
            $password = $input['password'] ?? '';
            
            if (!$password) throw new Exception('Password required');
            
            // Get list of datasets with unavailable keys
            $output = [];
            execCommand('sudo /usr/sbin/zfs get -H keystatus -t filesystem,volume', $output);
            
            $results = [];
            foreach ($output as $line) {
                $parts = preg_split('/\s+/', trim($line), 4);
                if (count($parts) >= 3 && $parts[2] === 'unavailable') {
                    $dataset = $parts[0];
                    
                    $cmd = 'echo ' . escapeshellarg($password) . ' | sudo /usr/sbin/zfs load-key ' . escapeshellarg($dataset) . ' 2>&1';
                    $keyOutput = [];
                    $success = execCommand($cmd, $keyOutput, $ret);
                    
                    $results[$dataset] = [
                        'success' => $success,
                        'message' => $success ? 'Key loaded' : 'Failed: ' . implode(' ', $keyOutput)
                    ];
                    
                    // Try to mount if key loaded successfully
                    if ($success) {
                        execCommand('sudo /usr/sbin/zfs mount ' . escapeshellarg($dataset), $keyOutput);
                    }
                }
            }
            
            $successCount = count(array_filter($results, function($r) { return $r['success']; }));
            $totalCount = count($results);
            
            logAction('encryption_load_all_keys', 'encryption', "$successCount/$totalCount datasets");
            
            echo json_encode([
                'success' => true,
                'loaded' => $successCount,
                'total' => $totalCount,
                'results' => $results
            ]);
            
        } elseif ($action === 'inherit_encryption') {
            // Create child dataset that inherits parent's encryption
            $name = validateInput($input['name'] ?? '', 'dataset_name');
            
            if (!$name) throw new Exception('Dataset name required');
            
            // Child datasets automatically inherit encryption from parent
            $cmd = 'sudo /usr/sbin/zfs create';
            
            if (isset($input['quota'])) {
                $cmd .= ' -o quota=' . escapeshellarg($input['quota']);
            }
            if (isset($input['compression'])) {
                $cmd .= ' -o compression=' . escapeshellarg($input['compression']);
            }
            
            $cmd .= ' ' . escapeshellarg($name);
            
            $output = [];
            if (!execCommand($cmd, $output, $ret)) {
                throw new Exception('Failed to create dataset: ' . implode("\n", $output));
            }
            
            logAction('encryption_inherit', 'dataset', $name);
            
            echo json_encode(['success' => true, 'dataset' => $name]);
            
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
