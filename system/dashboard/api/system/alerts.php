<?php
require_once __DIR__ . '/../../includes/auth.php';
requireAuth();
header('Content-Type: application/json');

function sendWebhook($url, $message, $severity = 'info') {
    $data = json_encode([
        'content' => "**[$severity]** $message",
        'username' => 'D-PlaneOS Alert'
    ]);
    
    $ch = curl_init($url);
    curl_setopt($ch, CURLOPT_HTTPHEADER, ['Content-Type: application/json']);
    curl_setopt($ch, CURLOPT_POST, 1);
    curl_setopt($ch, CURLOPT_POSTFIELDS, $data);
    curl_setopt($ch, CURLOPT_RETURNTRANSFER, true);
    curl_setopt($ch, CURLOPT_TIMEOUT, 10);
    
    $result = curl_exec($ch);
    $httpCode = curl_getinfo($ch, CURLINFO_HTTP_CODE);
    curl_close($ch);
    
    return $httpCode >= 200 && $httpCode < 300;
}

function checkPoolHealth() {
    $output = [];
    execCommand('sudo /usr/sbin/zpool list -H -o name,health', $output);
    
    $alerts = [];
    foreach ($output as $line) {
        $parts = preg_split('/\s+/', trim($line));
        if (count($parts) >= 2 && $parts[1] !== 'ONLINE') {
            $alerts[] = [
                'type' => 'pool_health',
                'severity' => 'critical',
                'message' => "Pool {$parts[0]} is {$parts[1]}",
                'details' => json_encode(['pool' => $parts[0], 'health' => $parts[1]])
            ];
        }
    }
    
    return $alerts;
}

function checkSmartHealth() {
    $output = [];
    execCommand('sudo /usr/bin/lsblk -n -o NAME,TYPE', $output);
    
    $alerts = [];
    foreach ($output as $line) {
        $parts = preg_split('/\s+/', trim($line));
        if (count($parts) >= 2 && $parts[1] === 'disk') {
            $disk = "/dev/{$parts[0]}";
            $smartOutput = [];
            execCommand("sudo /usr/sbin/smartctl -H $disk", $smartOutput, $ret);
            
            foreach ($smartOutput as $smartLine) {
                if (preg_match('/SMART.*:\s*(.+)/', $smartLine, $matches)) {
                    $health = trim($matches[1]);
                    if (stripos($health, 'PASSED') === false && stripos($health, 'OK') === false) {
                        $alerts[] = [
                            'type' => 'smart_health',
                            'severity' => 'warning',
                            'message' => "Disk $disk SMART status: $health",
                            'details' => json_encode(['disk' => $disk, 'health' => $health])
                        ];
                    }
                    break;
                }
            }
        }
    }
    
    return $alerts;
}

try {
    $method = $_SERVER['REQUEST_METHOD'];
    
    if ($method === 'GET') {
        $action = $_GET['action'] ?? 'settings';
        
        if ($action === 'settings') {
            $db = getDB();
            $stmt = $db->query('SELECT * FROM alert_settings');
            $settings = [];
            while ($row = $stmt->fetch()) {
                $settings[] = $row;
            }
            
            echo json_encode(['success' => true, 'settings' => $settings]);
            
        } elseif ($action === 'history') {
            $db = getDB();
            $stmt = $db->query('SELECT * FROM alert_history ORDER BY timestamp DESC LIMIT 100');
            $history = [];
            while ($row = $stmt->fetch()) {
                $history[] = $row;
            }
            
            echo json_encode(['success' => true, 'history' => $history]);
        }
        
    } elseif ($method === 'POST') {
        $input = json_decode(file_get_contents('php://input'), true);
        $action = $input['action'] ?? '';
        
        if ($action === 'configure') {
            $alertType = $input['alert_type'] ?? '';
            $enabled = $input['enabled'] ?? 0;
            $webhookUrl = $input['webhook_url'] ?? '';
            
            if (!$alertType) {
                throw new Exception('Alert type required');
            }
            
            $db = getDB();
            $stmt = $db->prepare('SELECT id FROM alert_settings WHERE alert_type = ?');
            $stmt->execute([$alertType]);
            $existing = $stmt->fetch();
            
            if ($existing) {
                $stmt = $db->prepare('UPDATE alert_settings SET enabled = ?, webhook_url = ?, updated_at = CURRENT_TIMESTAMP WHERE alert_type = ?');
                $stmt->execute([$enabled, $webhookUrl, $alertType]);
            } else {
                $stmt = $db->prepare('INSERT INTO alert_settings (alert_type, enabled, webhook_url) VALUES (?, ?, ?)');
                $stmt->execute([$alertType, $enabled, $webhookUrl]);
            }
            
            logAction('alert_configure', 'alert', $alertType);
            
            echo json_encode(['success' => true]);
            
        } elseif ($action === 'check') {
            // Manual check for alerts
            $alerts = array_merge(checkPoolHealth(), checkSmartHealth());
            
            $db = getDB();
            foreach ($alerts as $alert) {
                // Save to history
                $stmt = $db->prepare('INSERT INTO alert_history (alert_type, severity, message, details) VALUES (?, ?, ?, ?)');
                $stmt->execute([$alert['type'], $alert['severity'], $alert['message'], $alert['details']]);
                
                // Get webhook settings
                $stmt = $db->prepare('SELECT * FROM alert_settings WHERE alert_type = ? AND enabled = 1');
                $stmt->execute([$alert['type']]);
                $setting = $stmt->fetch();
                
                if ($setting && !empty($setting['webhook_url'])) {
                    $sent = sendWebhook($setting['webhook_url'], $alert['message'], $alert['severity']);
                    
                    if ($sent) {
                        $db->exec("UPDATE alert_history SET sent = 1 WHERE id = " . $db->lastInsertId());
                    }
                }
            }
            
            echo json_encode(['success' => true, 'alerts' => $alerts, 'count' => count($alerts)]);
            
        } elseif ($action === 'test') {
            $webhookUrl = $input['webhook_url'] ?? '';
            
            if (!$webhookUrl) {
                throw new Exception('Webhook URL required');
            }
            
            $sent = sendWebhook($webhookUrl, 'Test alert from D-PlaneOS', 'info');
            
            if (!$sent) {
                throw new Exception('Failed to send test webhook');
            }
            
            echo json_encode(['success' => true, 'message' => 'Test webhook sent']);
            
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
