<?php
require_once __DIR__ . '/../../includes/auth.php';
requireAuth();
header('Content-Type: application/json');

try {
    $method = $_SERVER['REQUEST_METHOD'];
    
    if ($method === 'GET') {
        $action = $_GET['action'] ?? 'status';
        
        if ($action === 'status') {
            // Get current UPS status via upsc command
            $output = [];
            
            // First, get list of UPS devices
            execCommand('upsc -l 2>/dev/null', $output, $ret);
            
            $upsDevices = [];
            
            if ($ret === 0 && !empty($output)) {
                foreach ($output as $upsName) {
                    $upsName = trim($upsName);
                    if (empty($upsName)) continue;
                    
                    // Get detailed status for this UPS
                    $upsData = [];
                    execCommand("upsc " . escapeshellarg($upsName) . " 2>/dev/null", $upsData, $ret2);
                    
                    if ($ret2 === 0) {
                        $upsInfo = parseUPSData($upsData);
                        $upsInfo['name'] = $upsName;
                        
                        // Store in database
                        updateUPSDatabase($upsName, $upsInfo);
                        
                        $upsDevices[] = $upsInfo;
                    }
                }
            }
            
            // If no UPS detected, check database for last known status
            if (empty($upsDevices)) {
                $db = getDB();
                $stmt = $db->query('SELECT * FROM ups_status ORDER BY last_check DESC LIMIT 1');
                $lastStatus = $stmt->fetch();
                
                if ($lastStatus) {
                    $upsDevices[] = [
                        'name' => $lastStatus['ups_name'],
                        'status' => 'OFFLINE',
                        'battery_charge' => $lastStatus['battery_charge'],
                        'battery_runtime' => $lastStatus['battery_runtime'],
                        'load' => $lastStatus['load'],
                        'model' => $lastStatus['ups_model'],
                        'last_check' => $lastStatus['last_check'],
                        'error' => 'UPS not responding - check NUT daemon'
                    ];
                }
            }
            
            echo json_encode([
                'success' => true, 
                'ups_devices' => $upsDevices,
                'nut_installed' => isNUTInstalled()
            ]);
            
        } elseif ($action === 'history') {
            // Get UPS history from database
            $hours = $_GET['hours'] ?? 24;
            
            $db = getDB();
            $stmt = $db->prepare('SELECT * FROM ups_status 
                WHERE last_check > datetime("now", "-" || ? || " hours")
                ORDER BY last_check DESC');
            $stmt->execute([$hours]);
            
            $history = [];
            while ($row = $stmt->fetch()) {
                $history[] = $row;
            }
            
            echo json_encode(['success' => true, 'history' => $history]);
            
        } elseif ($action === 'config') {
            // Get NUT configuration status
            $config = [
                'nut_installed' => isNUTInstalled(),
                'nut_running' => isNUTRunning(),
                'monitored_devices' => []
            ];
            
            if ($config['nut_installed']) {
                $output = [];
                execCommand('upsc -l 2>/dev/null', $output);
                $config['monitored_devices'] = array_filter(array_map('trim', $output));
            }
            
            echo json_encode(['success' => true, 'config' => $config]);
        }
        
    } elseif ($method === 'POST') {
        $input = json_decode(file_get_contents('php://input'), true);
        $action = $input['action'] ?? '';
        
        if ($action === 'test_shutdown') {
            // Test shutdown command (doesn't actually shutdown)
            $upsName = validateInput($input['ups_name'] ?? '', 'name');
            
            if (!$upsName) {
                throw new Exception('UPS name required');
            }
            
            // Create notification about test
            createNotification(
                'UPS Shutdown Test',
                "Testing shutdown procedures for UPS: $upsName",
                'info',
                'system',
                1,
                json_encode(['ups' => $upsName])
            );
            
            auditLog('test', 'ups', $upsName, 'Shutdown test initiated');
            
            echo json_encode([
                'success' => true, 
                'message' => 'Shutdown test logged - in production this would trigger graceful shutdown'
            ]);
            
        } elseif ($action === 'configure_shutdown') {
            // Configure shutdown settings
            $batteryLevel = validateInput($input['battery_level'] ?? 20, 'integer');
            $runtime = validateInput($input['runtime'] ?? 300, 'integer');
            
            // Store settings (in production, this would update /etc/nut/upsmon.conf)
            $db = getDB();
            $stmt = $db->prepare('INSERT OR REPLACE INTO settings (key, value, updated_at) 
                VALUES (?, ?, CURRENT_TIMESTAMP)');
            
            $stmt->execute(['ups_shutdown_battery_level', $batteryLevel]);
            $stmt->execute(['ups_shutdown_runtime', $runtime]);
            
            auditLog('configure', 'ups', 'shutdown_settings', 
                "Set shutdown at {$batteryLevel}% or {$runtime}s runtime");
            
            echo json_encode(['success' => true, 'message' => 'Shutdown settings updated']);
        }
    }
    
} catch (Exception $e) {
    http_response_code(400);
    echo json_encode(['success' => false, 'error' => $e->getMessage()]);
}

// Helper function to parse UPS data from upsc output
function parseUPSData($lines) {
    $data = [
        'status' => 'UNKNOWN',
        'battery_charge' => null,
        'battery_runtime' => null,
        'load' => null,
        'input_voltage' => null,
        'output_voltage' => null,
        'temperature' => null,
        'model' => null,
        'serial' => null
    ];
    
    foreach ($lines as $line) {
        if (preg_match('/^([\w.]+):\s*(.+)$/', $line, $matches)) {
            $key = $matches[1];
            $value = trim($matches[2]);
            
            switch ($key) {
                case 'ups.status':
                    $data['status'] = $value;
                    break;
                case 'battery.charge':
                    $data['battery_charge'] = (int)$value;
                    break;
                case 'battery.runtime':
                    $data['battery_runtime'] = (int)$value;
                    break;
                case 'ups.load':
                    $data['load'] = (int)$value;
                    break;
                case 'input.voltage':
                    $data['input_voltage'] = (float)$value;
                    break;
                case 'output.voltage':
                    $data['output_voltage'] = (float)$value;
                    break;
                case 'ups.temperature':
                    $data['temperature'] = (float)$value;
                    break;
                case 'ups.model':
                    $data['model'] = $value;
                    break;
                case 'ups.serial':
                    $data['serial'] = $value;
                    break;
            }
        }
    }
    
    return $data;
}

// Helper function to update UPS status in database
function updateUPSDatabase($upsName, $upsInfo) {
    $db = getDB();
    
    $stmt = $db->prepare('INSERT INTO ups_status 
        (ups_name, status, battery_charge, battery_runtime, load, input_voltage, 
         output_voltage, temperature, last_check, ups_model, ups_serial)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, ?, ?)');
    
    $stmt->execute([
        $upsName,
        $upsInfo['status'] ?? 'UNKNOWN',
        $upsInfo['battery_charge'],
        $upsInfo['battery_runtime'],
        $upsInfo['load'],
        $upsInfo['input_voltage'],
        $upsInfo['output_voltage'],
        $upsInfo['temperature'],
        $upsInfo['model'],
        $upsInfo['serial']
    ]);
    
    // Check for critical conditions
    if ($upsInfo['status'] === 'ONBATT' || $upsInfo['status'] === 'LOWBATT') {
        createNotification(
            'UPS Alert: On Battery Power',
            "UPS $upsName is running on battery ({$upsInfo['battery_charge']}% remaining)",
            $upsInfo['status'] === 'LOWBATT' ? 'error' : 'warning',
            'system',
            $upsInfo['status'] === 'LOWBATT' ? 3 : 2,
            json_encode($upsInfo)
        );
    }
    
    if ($upsInfo['battery_charge'] !== null && $upsInfo['battery_charge'] < 20) {
        createNotification(
            'UPS Critical: Low Battery',
            "UPS $upsName battery at {$upsInfo['battery_charge']}% - system shutdown imminent",
            'error',
            'system',
            3,
            json_encode($upsInfo)
        );
    }
}

// Helper function to check if NUT is installed
function isNUTInstalled() {
    $output = [];
    execCommand('which upsc 2>/dev/null', $output, $ret);
    return $ret === 0;
}

// Helper function to check if NUT is running
function isNUTRunning() {
    $output = [];
    execCommand('systemctl is-active nut-server 2>/dev/null', $output, $ret);
    return $ret === 0 && trim($output[0] ?? '') === 'active';
}

// Helper function to create notifications (should be in includes/auth.php but repeating here)
function createNotification($title, $message, $type, $category, $priority, $details = null) {
    $db = getDB();
    $stmt = $db->prepare('INSERT INTO notifications (title, message, type, category, priority, details)
        VALUES (?, ?, ?, ?, ?, ?)');
    $stmt->execute([$title, $message, $type, $category, $priority, $details]);
}
?>
