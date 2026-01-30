<?php
/**
 * D-PlaneOS Command Broker
 * Central, validated command execution layer
 * Prevents command injection by using strict whitelisting
 */

class CommandBroker {
    // Whitelist of allowed commands with parameter templates
    private static $COMMAND_WHITELIST = [
        // ZFS Pool commands
        'zpool_list' => [
            'cmd' => '/usr/sbin/zpool list -H %s -o %s',
            'params' => ['flags' => 'flags', 'fields' => 'string']
        ],
        'zpool_list_simple' => [
            'cmd' => '/usr/sbin/zpool list -H -o %s',
            'params' => ['fields' => 'string']
        ],
        'zpool_create' => [
            'cmd' => '/usr/sbin/zpool create %s %s %s',
            'params' => ['flags' => 'flags', 'name' => 'pool_name', 'vdev' => 'vdev_spec']
        ],
        'zpool_destroy' => [
            'cmd' => '/usr/sbin/zpool destroy -f %s',
            'params' => ['name' => 'pool_name']
        ],
        'zpool_scrub' => [
            'cmd' => '/usr/sbin/zpool scrub %s %s',
            'params' => ['flags' => 'flags', 'name' => 'pool_name']
        ],
        'zpool_status' => [
            'cmd' => '/usr/sbin/zpool status %s',
            'params' => ['name' => 'pool_name']
        ],
        
        // ZFS Dataset commands
        'zfs_list' => [
            'cmd' => '/usr/sbin/zfs list -H -o %s',
            'params' => ['fields' => 'string']
        ],
        'zfs_create' => [
            'cmd' => '/usr/sbin/zfs create %s %s',
            'params' => ['flags' => 'flags', 'name' => 'dataset_name']
        ],
        'zfs_destroy' => [
            'cmd' => '/usr/sbin/zfs destroy %s %s',
            'params' => ['flags' => 'flags', 'name' => 'dataset_name']
        ],
        'zfs_snapshot' => [
            'cmd' => '/usr/sbin/zfs snapshot %s %s',
            'params' => ['flags' => 'flags', 'snapshot' => 'snapshot_name']
        ],
        'zfs_set' => [
            'cmd' => '/usr/sbin/zfs set %s %s',
            'params' => ['property' => 'property', 'name' => 'dataset_name']
        ],
        'zfs_send' => [
            'cmd' => '/usr/sbin/zfs send %s',
            'params' => ['snapshot' => 'snapshot_name']
        ],
        'zfs_receive' => [
            'cmd' => '/usr/sbin/zfs receive %s',
            'params' => ['dataset' => 'dataset_name']
        ],
        
        // Disk commands
        'lsblk' => [
            'cmd' => '/usr/bin/lsblk -b -n -o %s',
            'params' => ['fields' => 'string']
        ],
        'smartctl_health' => [
            'cmd' => '/usr/sbin/smartctl -H %s',
            'params' => ['disk' => 'disk_path']
        ],
        'smartctl_info' => [
            'cmd' => '/usr/sbin/smartctl -a %s',
            'params' => ['disk' => 'disk_path']
        ],
        'smartctl_test' => [
            'cmd' => '/usr/sbin/smartctl -t %s %s',
            'params' => ['test_type' => 'test_type', 'disk' => 'disk_path']
        ],
        
        // Docker commands
        'docker_ps' => [
            'cmd' => '/usr/bin/docker ps -a --format %s',
            'params' => ['format' => 'string']
        ],
        'docker_start' => [
            'cmd' => '/usr/bin/docker start %s',
            'params' => ['container' => 'container_name']
        ],
        'docker_stop' => [
            'cmd' => '/usr/bin/docker stop %s',
            'params' => ['container' => 'container_name']
        ],
        'docker_restart' => [
            'cmd' => '/usr/bin/docker restart %s',
            'params' => ['container' => 'container_name']
        ],
        'docker_rm' => [
            'cmd' => '/usr/bin/docker rm -f %s',
            'params' => ['container' => 'container_name']
        ],
        'docker_compose' => [
            'cmd' => 'cd %s && /usr/bin/docker-compose -f %s up -d',
            'params' => ['workdir' => 'path', 'file' => 'path']
        ],
        
        // System commands
        'top' => [
            'cmd' => '/usr/bin/top -bn1',
            'params' => []
        ],
        'free' => [
            'cmd' => '/usr/bin/free -b',
            'params' => []
        ],
        'uptime' => [
            'cmd' => '/usr/bin/uptime',
            'params' => []
        ]
    ];
    
    /**
     * Execute a whitelisted command with validated parameters
     */
    public static function execute($commandKey, $params = [], &$output = [], &$returnCode = 0) {
        if (!isset(self::$COMMAND_WHITELIST[$commandKey])) {
            throw new Exception("Command not whitelisted: $commandKey");
        }
        
        $spec = self::$COMMAND_WHITELIST[$commandKey];
        $args = [];
        
        // Validate and sanitize each parameter
        foreach ($spec['params'] as $paramName => $paramType) {
            if (!isset($params[$paramName])) {
                throw new Exception("Missing required parameter: $paramName");
            }
            
            $value = $params[$paramName];
            $sanitized = self::validateParameter($value, $paramType);
            
            if ($sanitized === false) {
                throw new Exception("Invalid parameter $paramName for type $paramType");
            }
            
            $args[] = $sanitized;
        }
        
        // Build command with validated arguments
        // SECURITY FIX v1.9.0: Escape all arguments before sprintf
        $escapedArgs = array_map('escapeshellarg', $args);
        $cmd = 'sudo ' . vsprintf($spec['cmd'], $escapedArgs) . ' 2>&1';
        
        // Execute
        exec($cmd, $output, $returnCode);
        
        return $returnCode === 0;
    }
    
    /**
     * Validate parameter based on type
     */
    private static function validateParameter($value, $type) {
        switch ($type) {
            case 'pool_name':
                return preg_match('/^[a-zA-Z0-9_-]{1,64}$/', $value) ? $value : false;
                
            case 'dataset_name':
                return preg_match('/^[a-zA-Z0-9_\/-]{1,256}$/', $value) ? $value : false;
                
            case 'snapshot_name':
                return preg_match('/^[a-zA-Z0-9_\/-@]{1,256}$/', $value) ? $value : false;
                
            case 'disk_path':
                return preg_match('/^\/dev\/[a-zA-Z0-9\/]{1,64}$/', $value) ? $value : false;
                
            case 'container_name':
                return preg_match('/^[a-zA-Z0-9_-]{1,64}$/', $value) ? $value : false;
                
            case 'path':
                $real = realpath($value);
                return ($real && strpos($real, '/var/dplane/') === 0) ? $real : false;
                
            case 'test_type':
                return in_array($value, ['short', 'long', 'conveyance']) ? $value : false;
                
            case 'property':
                // ZFS property in format key=value
                if (preg_match('/^([a-z:_]+)=(.+)$/', $value, $matches)) {
                    $key = $matches[1];
                    $val = $matches[2];
                    $allowed = ['compression', 'atime', 'quota', 'refquota', 'mountpoint'];
                    return in_array($key, $allowed) ? "$key=$val" : false;
                }
                return false;
                
            case 'vdev_spec':
                // RAID type + disk list
                if (preg_match('/^(mirror|raidz|raidz2|raidz3)?\s*(.+)$/', $value, $matches)) {
                    $disks = explode(' ', trim($matches[2]));
                    foreach ($disks as $disk) {
                        if (!preg_match('/^\/dev\/[a-zA-Z0-9\/]{1,64}$/', $disk)) {
                            return false;
                        }
                    }
                    return $value;
                }
                return false;
                
            case 'flags':
                // Command flags like -r, -f
                return preg_match('/^-[a-zA-Z]{1,5}$/', $value) ? $value : '';
                
            case 'string':
                // General string (used for field lists, formats, etc)
                return preg_match('/^[a-zA-Z0-9_,:.=\/ -]{1,256}$/', $value) ? $value : false;
                
            default:
                return false;
        }
    }
}
