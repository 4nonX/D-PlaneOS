<?php
require_once __DIR__ . '/../../includes/auth.php';
requireAuth();
header('Content-Type: application/json');

try {
    $method = $_SERVER['REQUEST_METHOD'];
    
    if ($method === 'GET') {
        $action = $_GET['action'] ?? 'list';
        
        if ($action === 'list') {
            // List all pools
            $output = [];
            execCommand('sudo /usr/sbin/zpool list -H -o name,size,alloc,free,health', $output, $ret);
            
            $pools = [];
            foreach ($output as $line) {
                if (empty(trim($line))) continue;
                $parts = preg_split('/\s+/', trim($line));
                if (count($parts) >= 5) {
                    // Get bytes for percentage  
                    $bytesOutput = [];
                    execCommand('sudo /usr/sbin/zpool list -H -p -o name,size,alloc', $bytesOutput);
                    
                    $poolData = null;
                    foreach ($bytesOutput as $bLine) {
                        $bParts = preg_split('/\s+/', trim($bLine));
                        if ($bParts[0] === $parts[0]) {
                            $poolData = $bParts;
                            break;
                        }
                    }
                    
                    $sizeBytes = floatval($poolData[1] ?? 0);
                    $allocBytes = floatval($poolData[2] ?? 0);
                    $usagePercent = $sizeBytes > 0 ? round(($allocBytes / $sizeBytes) * 100, 1) : 0;
                    
                    $pools[] = [
                        'name' => $parts[0],
                        'size' => $parts[1],
                        'alloc' => $parts[2],
                        'free' => $parts[3],
                        'health' => $parts[4],
                        'size_bytes' => $sizeBytes,
                        'alloc_bytes' => $allocBytes,
                        'usage_percent' => $usagePercent
                    ];
                }
            }
            
            echo json_encode(['success' => true, 'pools' => $pools]);
            
        } elseif ($action === 'topology') {
            // Get pool topology (vdev structure)
            $name = validateInput($_GET['name'] ?? '', 'pool_name');
            if (!$name) throw new Exception('Pool name required');
            
            $output = [];
            execCommand('sudo /usr/sbin/zpool status ' . escapeshellarg($name), $output);
            
            $topology = ['vdevs' => []];
            $currentVdev = null;
            $inConfig = false;
            
            foreach ($output as $line) {
                $line = trim($line);
                
                if (strpos($line, 'config:') !== false) {
                    $inConfig = true;
                    continue;
                }
                
                if (!$inConfig) continue;
                if (empty($line) || strpos($line, 'errors:') !== false) break;
                
                // Parse vdev structure
                if (preg_match('/^\s*(\S+)\s+(\S+)\s+(\S+)\s+(\S+)/', $line, $matches)) {
                    $device = $matches[1];
                    $state = $matches[2];
                    $read = $matches[3];
                    $write = $matches[4];
                    
                    if ($device === $name) continue;
                    if ($device === 'NAME') continue;
                    
                    // Check indentation to determine hierarchy
                    $indent = strlen($line) - strlen(ltrim($line));
                    
                    if ($indent === 0 || in_array($device, ['mirror', 'raidz1', 'raidz2', 'raidz3'])) {
                        // New vdev
                        $currentVdev = [
                            'type' => $device,
                            'state' => $state,
                            'disks' => []
                        ];
                        $topology['vdevs'][] = &$currentVdev;
                    } else {
                        // Disk in current vdev
                        if ($currentVdev !== null) {
                            $currentVdev['disks'][] = [
                                'device' => $device,
                                'state' => $state,
                                'read' => $read,
                                'write' => $write
                            ];
                        }
                    }
                }
            }
            
            echo json_encode(['success' => true, 'topology' => $topology]);
            
        } elseif ($action === 'available_disks') {
            // List available disks for pool creation
            $output = [];
            execCommand('lsblk -d -n -o NAME,SIZE,TYPE,MODEL | grep disk', $output);
            
            $disks = [];
            foreach ($output as $line) {
                $parts = preg_split('/\s+/', trim($line), 4);
                if (count($parts) >= 2) {
                    $diskName = '/dev/' . $parts[0];
                    
                    // Check if disk is already in use
                    $statusOutput = [];
                    execCommand('sudo /usr/sbin/zpool status 2>&1 | grep ' . escapeshellarg($parts[0]), $statusOutput, $ret);
                    $inUse = ($ret === 0);
                    
                    $disks[] = [
                        'device' => $diskName,
                        'name' => $parts[0],
                        'size' => $parts[1],
                        'model' => $parts[3] ?? 'Unknown',
                        'in_use' => $inUse
                    ];
                }
            }
            
            echo json_encode(['success' => true, 'disks' => $disks]);
            
        } else {
            throw new Exception('Unknown action');
        }
        
    } elseif ($method === 'POST') {
        $input = json_decode(file_get_contents('php://input'), true);
        $action = $input['action'] ?? '';
        
        if ($action === 'create') {
            $name = validateInput($input['name'] ?? '', 'pool_name');
            $raid = $input['raid'] ?? 'stripe';
            $disks = $input['disks'] ?? [];
            $force = $input['force'] ?? false;
            
            if (!$name) {
                throw new Exception('Invalid pool name');
            }
            
            if (empty($disks)) {
                throw new Exception('No disks selected');
            }
            
            // Validate RAID configuration
            $minDisks = ['stripe' => 1, 'mirror' => 2, 'raidz1' => 3, 'raidz2' => 4, 'raidz3' => 5];
            if (isset($minDisks[$raid]) && count($disks) < $minDisks[$raid]) {
                throw new Exception("$raid requires at least {$minDisks[$raid]} disks");
            }
            
            // Check if pool exists
            $output = [];
            execCommand('sudo /usr/sbin/zpool list ' . escapeshellarg($name), $output, $ret);
            if ($ret === 0) {
                throw new Exception("Pool '$name' already exists");
            }
            
            // Build command
            $cmd = 'sudo /usr/sbin/zpool create';
            if ($force) $cmd .= ' -f';
            $cmd .= ' ' . escapeshellarg($name);
            
            if ($raid === 'mirror') {
                $cmd .= ' mirror';
            } elseif ($raid === 'raidz1') {
                $cmd .= ' raidz';
            } elseif ($raid === 'raidz2') {
                $cmd .= ' raidz2';
            } elseif ($raid === 'raidz3') {
                $cmd .= ' raidz3';
            }
            
            foreach ($disks as $disk) {
                if (!preg_match('/^\/dev\/[a-zA-Z0-9\/]+$/', $disk)) {
                    throw new Exception("Invalid disk path: $disk");
                }
                $cmd .= ' ' . escapeshellarg($disk);
            }
            
            // Execute
            $output = [];
            if (!execCommand($cmd, $output, $ret)) {
                throw new Exception('Pool creation failed: ' . implode("\n", $output));
            }
            
            // Set defaults
            execCommand('sudo /usr/sbin/zfs set compression=lz4 ' . escapeshellarg($name), $output);
            execCommand('sudo /usr/sbin/zfs set atime=off ' . escapeshellarg($name), $output);
            
            logAction('pool_create', 'pool', $name, json_encode(['raid' => $raid, 'disks' => $disks]));
            
            echo json_encode(['success' => true, 'pool' => $name]);
            
        } elseif ($action === 'destroy') {
            $name = validateInput($input['name'] ?? '', 'pool_name');
            
            if (!$name) {
                throw new Exception('Invalid pool name');
            }
            
            // Check exists
            $output = [];
            if (!execCommand('sudo /usr/sbin/zpool list ' . escapeshellarg($name), $output)) {
                throw new Exception("Pool '$name' does not exist");
            }
            
            // Destroy
            $output = [];
            if (!execCommand('sudo /usr/sbin/zpool destroy -f ' . escapeshellarg($name), $output, $ret)) {
                throw new Exception('Pool destruction failed: ' . implode("\n", $output));
            }
            
            logAction('pool_destroy', 'pool', $name);
            
            echo json_encode(['success' => true, 'pool' => $name]);
            
        } elseif ($action === 'scrub') {
            $name = validateInput($input['name'] ?? '', 'pool_name');
            $operation = $input['operation'] ?? 'start'; // start or stop
            
            if (!$name) {
                throw new Exception('Invalid pool name');
            }
            
            $cmd = 'sudo /usr/sbin/zpool scrub';
            if ($operation === 'stop') {
                $cmd .= ' -s';
            }
            $cmd .= ' ' . escapeshellarg($name);
            
            $output = [];
            if (!execCommand($cmd, $output, $ret)) {
                throw new Exception('Scrub operation failed: ' . implode("\n", $output));
            }
            
            // Update last_run in schedule if exists
            if ($operation === 'start') {
                $db = getDB();
                $stmt = $db->prepare('UPDATE scrub_schedules SET last_run = CURRENT_TIMESTAMP WHERE pool_name = ?');
                $stmt->execute([$name]);
            }
            
            logAction('pool_scrub_' . $operation, 'pool', $name);
            
            echo json_encode(['success' => true, 'operation' => $operation]);
            
        } elseif ($action === 'schedule_scrub') {
            $name = validateInput($input['name'] ?? '', 'pool_name');
            $scheduleType = $input['schedule_type'] ?? 'monthly'; // monthly, weekly
            $dayOfMonth = validateInput($input['day_of_month'] ?? 1, 'integer');
            
            if (!$name) {
                throw new Exception('Invalid pool name');
            }
            
            $db = getDB();
            
            // Check if schedule exists
            $stmt = $db->prepare('SELECT id FROM scrub_schedules WHERE pool_name = ?');
            $stmt->execute([$name]);
            $existing = $stmt->fetch();
            
            if ($existing) {
                $stmt = $db->prepare('UPDATE scrub_schedules SET schedule_type = ?, day_of_month = ?, enabled = 1 WHERE pool_name = ?');
                $stmt->execute([$scheduleType, $dayOfMonth, $name]);
            } else {
                $stmt = $db->prepare('INSERT INTO scrub_schedules (pool_name, schedule_type, day_of_month) VALUES (?, ?, ?)');
                $stmt->execute([$name, $scheduleType, $dayOfMonth]);
            }
            
            // Create cron job
            $cronFile = "/etc/cron.d/dplaneos-scrub-$name";
            $cronContent = '';
            
            if ($scheduleType === 'monthly') {
                $cronContent = "0 2 $dayOfMonth * * root /usr/sbin/zpool scrub $name\n";
            } elseif ($scheduleType === 'weekly') {
                $day = $dayOfMonth % 7; // 0-6 for days of week
                $cronContent = "0 2 * * $day root /usr/sbin/zpool scrub $name\n";
            }
            
            file_put_contents($cronFile, $cronContent);
            chmod($cronFile, 0644);
            
            logAction('schedule_scrub', 'pool', $name, "$scheduleType on day $dayOfMonth");
            
            echo json_encode(['success' => true, 'schedule' => $scheduleType]);
            
        } elseif ($action === 'get_schedule') {
            $name = validateInput($input['name'] ?? '', 'pool_name');
            
            if (!$name) {
                throw new Exception('Invalid pool name');
            }
            
            $db = getDB();
            $stmt = $db->prepare('SELECT * FROM scrub_schedules WHERE pool_name = ?');
            $stmt->execute([$name]);
            $schedule = $stmt->fetch();
            
            echo json_encode(['success' => true, 'schedule' => $schedule]);
            
        } elseif ($action === 'add_vdev') {
            $name = validateInput($input['name'] ?? '', 'pool_name');
            $vdevType = validateInput($input['vdev_type'] ?? '', 'name');
            $disks = $input['disks'] ?? [];
            
            if (!$name || !$vdevType || empty($disks)) {
                throw new Exception('Pool name, vdev type, and disks required');
            }
            
            if (!in_array($vdevType, ['', 'mirror', 'raidz', 'raidz2', 'raidz3'])) {
                throw new Exception('Invalid vdev type');
            }
            
            // Build command
            $cmd = 'sudo /usr/sbin/zpool add ' . escapeshellarg($name);
            if ($vdevType !== '') {
                $cmd .= ' ' . escapeshellarg($vdevType);
            }
            foreach ($disks as $disk) {
                $disk = validateInput($disk, 'disk_path');
                $cmd .= ' ' . escapeshellarg($disk);
            }
            
            $output = [];
            if (!execCommand($cmd, $output, $ret)) {
                throw new Exception('Failed to add vdev: ' . implode("\n", $output));
            }
            
            logAction('add_vdev', 'pool', $name, "$vdevType with " . count($disks) . " disks");
            
            echo json_encode(['success' => true]);
            
        } elseif ($action === 'replace_disk') {
            $name = validateInput($input['name'] ?? '', 'pool_name');
            $oldDisk = validateInput($input['old_disk'] ?? '', 'disk_path');
            $newDisk = validateInput($input['new_disk'] ?? '', 'disk_path');
            
            if (!$name || !$oldDisk || !$newDisk) {
                throw new Exception('Pool name, old disk, and new disk required');
            }
            
            // Execute replacement
            $cmd = 'sudo /usr/sbin/zpool replace ' . escapeshellarg($name) . ' ' . 
                   escapeshellarg($oldDisk) . ' ' . escapeshellarg($newDisk);
            
            $output = [];
            if (!execCommand($cmd, $output, $ret)) {
                throw new Exception('Failed to replace disk: ' . implode("\n", $output));
            }
            
            logAction('replace_disk', 'pool', $name, "$oldDisk -> $newDisk");
            
            echo json_encode(['success' => true]);
            
        } elseif ($action === 'attach_disk') {
            $name = validateInput($input['name'] ?? '', 'pool_name');
            $existingDisk = validateInput($input['existing_disk'] ?? '', 'disk_path');
            $newDisk = validateInput($input['new_disk'] ?? '', 'disk_path');
            
            if (!$name || !$existingDisk || !$newDisk) {
                throw new Exception('Pool name, existing disk, and new disk required');
            }
            
            // Attach to create mirror
            $cmd = 'sudo /usr/sbin/zpool attach ' . escapeshellarg($name) . ' ' .
                   escapeshellarg($existingDisk) . ' ' . escapeshellarg($newDisk);
            
            $output = [];
            if (!execCommand($cmd, $output, $ret)) {
                throw new Exception('Failed to attach disk: ' . implode("\n", $output));
            }
            
            logAction('attach_disk', 'pool', $name, "$newDisk attached to $existingDisk");
            
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
