<?php
require_once __DIR__ . '/../../includes/auth.php';
requireAuth();
header('Content-Type: application/json');

try {
    $method = $_SERVER['REQUEST_METHOD'];
    
    if ($method === 'GET') {
        $action = $_GET['action'] ?? 'all';
        
        if ($action === 'all') {
            // Get all metrics in one call
            $metrics = [
                'cpu' => getCPUMetrics(),
                'memory' => getMemoryMetrics(),
                'network' => getNetworkMetrics(),
                'disk_io' => getDiskIOMetrics(),
                'processes' => getTopProcesses(10),
                'timestamp' => time(),
            ];
            
            echo json_encode(['success' => true, 'metrics' => $metrics]);
            
        } elseif ($action === 'cpu') {
            $metrics = getCPUMetrics();
            echo json_encode(['success' => true, 'cpu' => $metrics]);
            
        } elseif ($action === 'memory') {
            $metrics = getMemoryMetrics();
            echo json_encode(['success' => true, 'memory' => $metrics]);
            
        } elseif ($action === 'network') {
            $metrics = getNetworkMetrics();
            echo json_encode(['success' => true, 'network' => $metrics]);
            
        } elseif ($action === 'disk_io') {
            $metrics = getDiskIOMetrics();
            echo json_encode(['success' => true, 'disk_io' => $metrics]);
            
        } elseif ($action === 'processes') {
            $limit = intval($_GET['limit'] ?? 20);
            $processes = getTopProcesses($limit);
            echo json_encode(['success' => true, 'processes' => $processes]);
            
        } elseif ($action === 'system_info') {
            $info = getSystemInfo();
            echo json_encode(['success' => true, 'system_info' => $info]);
            
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

function getCPUMetrics() {
    $metrics = [
        'cores' => [],
        'total_usage' => 0,
        'load_average' => [],
    ];
    
    // Get number of CPU cores
    $coreCount = intval(shell_exec('nproc'));
    
    // Get load average
    $loadavg = file_get_contents('/proc/loadavg');
    $parts = explode(' ', $loadavg);
    $metrics['load_average'] = [
        '1min' => floatval($parts[0]),
        '5min' => floatval($parts[1]),
        '15min' => floatval($parts[2]),
    ];
    
    // Get CPU usage per core
    $stat1 = file_get_contents('/proc/stat');
    usleep(100000); // Wait 100ms
    $stat2 = file_get_contents('/proc/stat');
    
    $lines1 = explode("\n", $stat1);
    $lines2 = explode("\n", $stat2);
    
    $totalUsage = 0;
    $coreIndex = 0;
    
    foreach ($lines1 as $i => $line1) {
        if (strpos($line1, 'cpu') !== 0) continue;
        if ($line1 === substr($line1, 0, 3)) continue; // Skip 'cpu' (total) line for now
        
        $line2 = $lines2[$i];
        $usage = calculateCPUUsage($line1, $line2);
        
        if ($usage !== null) {
            $metrics['cores'][] = [
                'core' => $coreIndex,
                'usage' => round($usage, 1),
            ];
            $totalUsage += $usage;
            $coreIndex++;
        }
        
        if ($coreIndex >= $coreCount) break;
    }
    
    $metrics['total_usage'] = $coreCount > 0 ? round($totalUsage / $coreCount, 1) : 0;
    $metrics['core_count'] = $coreCount;
    
    return $metrics;
}

function calculateCPUUsage($line1, $line2) {
    $stats1 = preg_split('/\s+/', trim($line1));
    $stats2 = preg_split('/\s+/', trim($line2));
    
    if (count($stats1) < 5 || count($stats2) < 5) return null;
    
    // user, nice, system, idle, iowait, irq, softirq
    $idle1 = intval($stats1[4]);
    $idle2 = intval($stats2[4]);
    
    $total1 = 0;
    $total2 = 0;
    for ($i = 1; $i <= 7; $i++) {
        $total1 += intval($stats1[$i] ?? 0);
        $total2 += intval($stats2[$i] ?? 0);
    }
    
    $totalDiff = $total2 - $total1;
    $idleDiff = $idle2 - $idle1;
    
    if ($totalDiff === 0) return 0;
    
    return (($totalDiff - $idleDiff) / $totalDiff) * 100;
}

function getMemoryMetrics() {
    $meminfo = file_get_contents('/proc/meminfo');
    $lines = explode("\n", $meminfo);
    
    $metrics = [];
    foreach ($lines as $line) {
        if (preg_match('/^(\w+):\s+(\d+)/', $line, $matches)) {
            $metrics[$matches[1]] = intval($matches[2]) * 1024; // Convert KB to bytes
        }
    }
    
    $total = $metrics['MemTotal'] ?? 0;
    $free = $metrics['MemFree'] ?? 0;
    $available = $metrics['MemAvailable'] ?? 0;
    $buffers = $metrics['Buffers'] ?? 0;
    $cached = $metrics['Cached'] ?? 0;
    $used = $total - $available;
    
    return [
        'total' => $total,
        'total_human' => formatBytes($total),
        'used' => $used,
        'used_human' => formatBytes($used),
        'free' => $free,
        'free_human' => formatBytes($free),
        'available' => $available,
        'available_human' => formatBytes($available),
        'buffers' => $buffers,
        'buffers_human' => formatBytes($buffers),
        'cached' => $cached,
        'cached_human' => formatBytes($cached),
        'usage_percent' => $total > 0 ? round(($used / $total) * 100, 1) : 0,
    ];
}

function getNetworkMetrics() {
    $interfaces = [];
    $netdev = file_get_contents('/proc/net/dev');
    $lines = explode("\n", $netdev);
    
    foreach ($lines as $line) {
        if (strpos($line, ':') === false) continue;
        
        list($interface, $stats) = explode(':', $line);
        $interface = trim($interface);
        
        // Skip loopback
        if ($interface === 'lo') continue;
        
        $parts = preg_split('/\s+/', trim($stats));
        
        if (count($parts) >= 16) {
            $interfaces[] = [
                'interface' => $interface,
                'rx_bytes' => intval($parts[0]),
                'rx_bytes_human' => formatBytes(intval($parts[0])),
                'rx_packets' => intval($parts[1]),
                'rx_errors' => intval($parts[2]),
                'rx_dropped' => intval($parts[3]),
                'tx_bytes' => intval($parts[8]),
                'tx_bytes_human' => formatBytes(intval($parts[8])),
                'tx_packets' => intval($parts[9]),
                'tx_errors' => intval($parts[10]),
                'tx_dropped' => intval($parts[11]),
            ];
        }
    }
    
    return $interfaces;
}

function getDiskIOMetrics() {
    $disks = [];
    $diskstats = file_get_contents('/proc/diskstats');
    $lines = explode("\n", $diskstats);
    
    foreach ($lines as $line) {
        $parts = preg_split('/\s+/', trim($line));
        
        if (count($parts) < 14) continue;
        
        $device = $parts[2];
        
        // Only include physical disks (sd*, nvme*, vd*)
        if (!preg_match('/^(sd[a-z]|nvme\d+n\d+|vd[a-z])$/', $device)) continue;
        
        $disks[] = [
            'device' => $device,
            'reads' => intval($parts[3]),
            'reads_merged' => intval($parts[4]),
            'sectors_read' => intval($parts[5]),
            'read_time_ms' => intval($parts[6]),
            'writes' => intval($parts[7]),
            'writes_merged' => intval($parts[8]),
            'sectors_written' => intval($parts[9]),
            'write_time_ms' => intval($parts[10]),
            'io_in_progress' => intval($parts[11]),
            'io_time_ms' => intval($parts[12]),
        ];
    }
    
    return $disks;
}

function getTopProcesses($limit = 20) {
    $cmd = 'ps aux --sort=-%cpu,%mem | head -n ' . intval($limit + 1);
    exec($cmd, $output);
    
    $processes = [];
    
    // Skip header line
    for ($i = 1; $i < count($output); $i++) {
        $parts = preg_split('/\s+/', trim($output[$i]), 11);
        
        if (count($parts) >= 11) {
            $processes[] = [
                'user' => $parts[0],
                'pid' => intval($parts[1]),
                'cpu' => floatval($parts[2]),
                'memory' => floatval($parts[3]),
                'vsz' => intval($parts[4]),
                'rss' => intval($parts[5]),
                'tty' => $parts[6],
                'stat' => $parts[7],
                'start' => $parts[8],
                'time' => $parts[9],
                'command' => $parts[10],
            ];
        }
    }
    
    return $processes;
}

function getSystemInfo() {
    $info = [];
    
    // Hostname
    $info['hostname'] = trim(shell_exec('hostname'));
    
    // Uptime
    $uptime = file_get_contents('/proc/uptime');
    $parts = explode(' ', $uptime);
    $uptime_seconds = intval($parts[0]);
    $info['uptime_seconds'] = $uptime_seconds;
    $info['uptime_human'] = formatUptime($uptime_seconds);
    
    // Kernel
    $info['kernel'] = trim(shell_exec('uname -r'));
    
    // OS
    if (file_exists('/etc/os-release')) {
        $osrelease = parse_ini_file('/etc/os-release');
        $info['os'] = $osrelease['PRETTY_NAME'] ?? 'Unknown';
    }
    
    // CPU model
    $cpuinfo = file_get_contents('/proc/cpuinfo');
    if (preg_match('/model name\s*:\s*(.+)/i', $cpuinfo, $matches)) {
        $info['cpu_model'] = trim($matches[1]);
    }
    
    return $info;
}

function formatBytes($bytes, $precision = 2) {
    $units = ['B', 'KB', 'MB', 'GB', 'TB'];
    $bytes = max($bytes, 0);
    $pow = floor(($bytes ? log($bytes) : 0) / log(1024));
    $pow = min($pow, count($units) - 1);
    $bytes /= pow(1024, $pow);
    return round($bytes, $precision) . ' ' . $units[$pow];
}

function formatUptime($seconds) {
    $days = floor($seconds / 86400);
    $hours = floor(($seconds % 86400) / 3600);
    $minutes = floor(($seconds % 3600) / 60);
    
    $parts = [];
    if ($days > 0) $parts[] = $days . 'd';
    if ($hours > 0) $parts[] = $hours . 'h';
    if ($minutes > 0) $parts[] = $minutes . 'm';
    
    return implode(' ', $parts);
}
