<?php
require_once __DIR__ . '/../../includes/auth.php';
requireAuth();
header('Content-Type: application/json');

try {
    $stats = [];
    
    // CPU usage
    $output = [];
    execCommand('top -bn1 | grep "Cpu(s)"', $output);
    if (!empty($output)) {
        preg_match('/(\d+\.\d+)\s*id/', $output[0], $matches);
        $idle = floatval($matches[1] ?? 100);
        $stats['cpu_percent'] = round(100 - $idle, 1);
    } else {
        $stats['cpu_percent'] = 0;
    }
    
    // Memory usage
    $output = [];
    execCommand('free -b', $output);
    if (count($output) >= 2) {
        $parts = preg_split('/\s+/', $output[1]);
        $total = floatval($parts[1] ?? 1);
        $used = floatval($parts[2] ?? 0);
        $stats['memory_total'] = $total;
        $stats['memory_used'] = $used;
        $stats['memory_percent'] = round(($used / $total) * 100, 1);
        $stats['memory_total_human'] = formatBytes($total);
        $stats['memory_used_human'] = formatBytes($used);
    } else {
        $stats['memory_percent'] = 0;
    }
    
    // Uptime
    $output = [];
    execCommand('uptime -p', $output);
    $stats['uptime'] = $output[0] ?? 'unknown';
    
    // Load average
    $output = [];
    execCommand('uptime', $output);
    if (!empty($output)) {
        preg_match('/load average: ([\d.]+), ([\d.]+), ([\d.]+)/', $output[0], $matches);
        $stats['load_1m'] = floatval($matches[1] ?? 0);
        $stats['load_5m'] = floatval($matches[2] ?? 0);
        $stats['load_15m'] = floatval($matches[3] ?? 0);
    }
    
    echo json_encode(['success' => true, 'stats' => $stats]);
    
} catch (Exception $e) {
    http_response_code(400);
    echo json_encode(['success' => false, 'error' => $e->getMessage()]);
}
