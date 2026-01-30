<?php
require_once __DIR__ . '/../../includes/auth.php';
requireAuth();
header('Content-Type: application/json');

// Docker Compose YAML validation to prevent command injection
function validateDockerComposeYaml($yaml) {
    // Basic YAML parsing check
    if (!function_exists('yaml_parse')) {
        // Fallback: basic validation without yaml extension
        if (strpos($yaml, '<?php') !== false || strpos($yaml, '<?=') !== false) {
            throw new Exception('Invalid YAML content detected');
        }
    } else {
        try {
            $data = yaml_parse($yaml);
        } catch (Exception $e) {
            throw new Exception('Invalid YAML format');
        }
        
        if (!isset($data['services']) || !is_array($data['services'])) {
            throw new Exception('No services defined in compose file');
        }
        
        // Validate each service for dangerous configurations
        foreach ($data['services'] as $name => $service) {
            if (!is_array($service)) continue;
            
            // Block dangerous options that could lead to privilege escalation
            $dangerous = ['privileged', 'cap_add', 'security_opt', 'pid', 'ipc', 'userns_mode'];
            foreach ($dangerous as $key) {
                if (isset($service[$key])) {
                    throw new Exception("Security: Option '$key' not allowed in compose files");
                }
            }
            
            // Check network_mode for host mode
            if (isset($service['network_mode']) && $service['network_mode'] === 'host') {
                throw new Exception("Security: host network mode not allowed");
            }
            
            // Validate volumes - no critical system mounts
            if (isset($service['volumes'])) {
                foreach ($service['volumes'] as $volume) {
                    if (is_string($volume)) {
                        // Check for bind mounts to sensitive directories
                        if (preg_match('#^(/|/root|/etc|/var|/sys|/proc|/dev):#', $volume)) {
                            throw new Exception("Security: System directory mounts not allowed: $volume");
                        }
                    }
                }
            }
            
            // Validate image source
            if (isset($service['image'])) {
                $image = $service['image'];
                // Block suspicious patterns
                if (preg_match('#[;&|`$]#', $image)) {
                    throw new Exception("Security: Invalid characters in image name");
                }
            }
            
            // Validate command for injection attempts
            if (isset($service['command'])) {
                $cmd = is_array($service['command']) ? implode(' ', $service['command']) : $service['command'];
                if (preg_match('#[;&|`]#', $cmd)) {
                    throw new Exception("Security: Potentially dangerous command detected");
                }
            }
        }
    }
    
    return true;
}

try {
    $method = $_SERVER['REQUEST_METHOD'];
    
    if ($method === 'GET') {
        // List all containers
        $output = [];
        execCommand('sudo /usr/bin/docker ps -a --format "{{.ID}}|{{.Names}}|{{.Status}}|{{.Image}}|{{.Ports}}"', $output);
        
        $containers = [];
        foreach ($output as $line) {
            $parts = explode('|', $line);
            if (count($parts) >= 5) {
                $status = $parts[2];
                $running = (strpos($status, 'Up') === 0);
                
                // Parse ports
                $ports = [];
                if (!empty($parts[4])) {
                    preg_match_all('/0\.0\.0\.0:(\d+)/', $parts[4], $matches);
                    $ports = $matches[1];
                }
                
                $containers[] = [
                    'id' => $parts[0],
                    'name' => $parts[1],
                    'status' => $status,
                    'image' => $parts[3],
                    'ports' => $ports,
                    'running' => $running
                ];
            }
        }
        
        echo json_encode(['success' => true, 'containers' => $containers]);
        
    } elseif ($method === 'GET' && isset($_GET['action'])) {
        $action = $_GET['action'];
        
        // Whitelist valid actions to prevent logic bypass
        $validActions = ['stats', 'logs', 'inspect', 'export'];
        if (!in_array($action, $validActions, true)) {
            http_response_code(400);
            echo json_encode(['success' => false, 'error' => 'Invalid action']);
            exit;
        }
        
        if ($action === 'stats') {
            $name = $_GET['name'] ?? '';
            if (empty($name)) throw new Exception('Container name required');
            
            $output = [];
            $cmd = 'sudo /usr/bin/docker stats ' . escapeshellarg($name) . ' --no-stream --format "{{.CPUPerc}}|{{.MemUsage}}|{{.NetIO}}|{{.BlockIO}}"';
            execCommand($cmd, $output);
            
            if (empty($output[0])) throw new Exception('Failed to get stats');
            
            $parts = explode('|', $output[0]);
            echo json_encode(['success' => true, 'stats' => [
                'cpu' => $parts[0] ?? '0%',
                'memory' => $parts[1] ?? '0B / 0B',
                'network' => $parts[2] ?? '0B / 0B',
                'disk' => $parts[3] ?? '0B / 0B',
            ]]);
            
        } elseif ($action === 'logs') {
            $name = $_GET['name'] ?? '';
            $lines = min(max(intval($_GET['lines'] ?? 100), 1), 10000);  // Cap at 10k lines
            if (empty($name)) throw new Exception('Container name required');
            
            $output = [];
            execCommand('sudo /usr/bin/docker logs ' . escapeshellarg($name) . ' --tail ' . $lines . ' 2>&1', $output);
            echo json_encode(['success' => true, 'logs' => $output]);
            
        } elseif ($action === 'inspect') {
            $name = $_GET['name'] ?? '';
            if (empty($name)) throw new Exception('Container name required');
            
            $output = [];
            execCommand('sudo /usr/bin/docker inspect ' . escapeshellarg($name), $output);
            $data = json_decode(implode("\n", $output), true);
            echo json_encode(['success' => true, 'data' => $data[0] ?? []]);
            
        } else {
            throw new Exception('Unknown action');
        }
        
    } elseif ($method === 'POST') {
        $input = json_decode(file_get_contents('php://input'), true);
        $action = $input['action'] ?? '';
        
        if ($action === 'start') {
            $name = $input['name'] ?? '';
            if (empty($name)) {
                throw new Exception('Container name required');
            }
            
            $output = [];
            if (!execCommand('sudo /usr/bin/docker start ' . escapeshellarg($name), $output, $ret)) {
                throw new Exception('Start failed: ' . implode("\n", $output));
            }
            
            logAction('container_start', 'container', $name);
            echo json_encode(['success' => true]);
            
        } elseif ($action === 'stop') {
            $name = $input['name'] ?? '';
            if (empty($name)) {
                throw new Exception('Container name required');
            }
            
            $output = [];
            if (!execCommand('sudo /usr/bin/docker stop ' . escapeshellarg($name), $output, $ret)) {
                throw new Exception('Stop failed: ' . implode("\n", $output));
            }
            
            logAction('container_stop', 'container', $name);
            echo json_encode(['success' => true]);
            
        } elseif ($action === 'restart') {
            $name = $input['name'] ?? '';
            if (empty($name)) {
                throw new Exception('Container name required');
            }
            
            $output = [];
            if (!execCommand('sudo /usr/bin/docker restart ' . escapeshellarg($name), $output, $ret)) {
                throw new Exception('Restart failed: ' . implode("\n", $output));
            }
            
            logAction('container_restart', 'container', $name);
            echo json_encode(['success' => true]);
            
        } elseif ($action === 'deploy') {
            $yaml = $input['yaml'] ?? '';
            $name = $input['name'] ?? 'compose-' . time();
            
            if (empty($yaml)) {
                throw new Exception('Docker Compose YAML required');
            }
            
            // Validate YAML for security issues
            validateDockerComposeYaml($yaml);
            
            // Save compose file
            $composeDir = '/var/dplane/compose';
            if (!is_dir($composeDir)) {
                mkdir($composeDir, 0755, true);
            }
            
            $composePath = "$composeDir/$name.yml";
            if (file_put_contents($composePath, $yaml) === false) {
                throw new Exception('Failed to save compose file');
            }
            
            // Deploy
            $output = [];
            $cmd = 'cd ' . escapeshellarg($composeDir) . ' && sudo /usr/bin/docker-compose -f ' . escapeshellarg($composePath) . ' up -d';
            if (!execCommand($cmd, $output, $ret)) {
                throw new Exception('Deployment failed: ' . implode("\n", $output));
            }
            
            logAction('container_deploy', 'compose', $name);
            echo json_encode(['success' => true, 'name' => $name]);
            
        } elseif ($action === 'remove') {
            $name = $input['name'] ?? '';
            if (empty($name)) {
                throw new Exception('Container name required');
            }
            
            $output = [];
            if (!execCommand('sudo /usr/bin/docker rm -f ' . escapeshellarg($name), $output, $ret)) {
                throw new Exception('Remove failed: ' . implode("\n", $output));
            }
            
            logAction('container_remove', 'container', $name);
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
