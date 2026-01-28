<?php
/**
 * D-PlaneOS Rclone Management
 * Backend for cloud sync with any rclone-supported backend
 */

function getRcloneConfigPath() {
    return '/var/dplane/rclone/rclone.conf';
}

function ensureRcloneConfig() {
    $configDir = '/var/dplane/rclone';
    if (!is_dir($configDir)) {
        mkdir($configDir, 0755, true);
        chown($configDir, 'www-data');
    }
    
    $configFile = getRcloneConfigPath();
    if (!file_exists($configFile)) {
        touch($configFile);
        chmod($configFile, 0600);
        chown($configFile, 'www-data');
    }
    
    return $configFile;
}

function writeRcloneConfig() {
    $db = getDB();
    $stmt = $db->query('SELECT * FROM rclone_remotes WHERE enabled = 1');
    $remotes = $stmt->fetchAll();
    
    $configFile = ensureRcloneConfig();
    $config = '';
    
    foreach ($remotes as $remote) {
        $remoteConfig = json_decode($remote['config'], true);
        
        $config .= "[{$remote['name']}]\n";
        $config .= "type = {$remote['remote_type']}\n";
        
        foreach ($remoteConfig as $key => $value) {
            if ($key !== 'type') {
                $config .= "$key = $value\n";
            }
        }
        
        $config .= "\n";
    }
    
    file_put_contents($configFile, $config);
    chmod($configFile, 0600);
    chown($configFile, 'www-data');
    
    return ['success' => true];
}

function runRcloneSync($task, $remote) {
    $configFile = getRcloneConfigPath();
    
    // Build command
    $cmd = 'rclone';
    
    // Sync type
    if ($task['sync_type'] === 'sync') {
        $cmd .= ' sync';
    } elseif ($task['sync_type'] === 'copy') {
        $cmd .= ' copy';
    } elseif ($task['sync_type'] === 'move') {
        $cmd .= ' move';
    } else {
        $cmd .= ' sync'; // default
    }
    
    // Paths
    if ($task['direction'] === 'push') {
        // Local to remote
        $cmd .= ' ' . escapeshellarg($task['source_path']);
        $cmd .= ' ' . escapeshellarg($remote['name'] . ':' . $task['destination_path']);
    } else {
        // Remote to local
        $cmd .= ' ' . escapeshellarg($remote['name'] . ':' . $task['source_path']);
        $cmd .= ' ' . escapeshellarg($task['destination_path']);
    }
    
    // Options
    $cmd .= ' --config ' . escapeshellarg($configFile);
    $cmd .= ' --stats 1s';
    $cmd .= ' --stats-one-line';
    $cmd .= ' --log-level INFO';
    
    // Progress tracking
    $progressFile = "/tmp/rclone-{$task['id']}.progress";
    $cmd .= ' 2>&1 | tee ' . escapeshellarg($progressFile);
    
    // Execute
    $output = [];
    exec($cmd, $output, $ret);
    
    return [
        'success' => ($ret === 0),
        'output' => implode("\n", $output),
        'progress_file' => $progressFile
    ];
}

function getRcloneBackends() {
    // All rclone backends (https://rclone.org/)
    return [
        '1fichier' => '1Fichier',
        'acd' => 'Amazon Drive',
        'azureblob' => 'Azure Blob Storage',
        'b2' => 'Backblaze B2',
        'box' => 'Box',
        'cache' => 'Cache',
        'chunker' => 'Chunker',
        'compress' => 'Compress',
        'crypt' => 'Crypt',
        'dropbox' => 'Dropbox',
        'drive' => 'Google Drive',
        'fichier' => 'Fichier',
        'ftp' => 'FTP',
        'gdocs' => 'Google Docs',
        'googlecloudstorage' => 'Google Cloud Storage',
        'googlephotos' => 'Google Photos',
        'hasher' => 'Hasher',
        'hdfs' => 'HDFS',
        'http' => 'HTTP',
        'hubic' => 'Hubic',
        'internetarchive' => 'Internet Archive',
        'jottacloud' => 'Jottacloud',
        'koofr' => 'Koofr',
        'mailru' => 'Mail.ru Cloud',
        'mega' => 'Mega',
        'memory' => 'Memory',
        'netstorage' => 'Akamai NetStorage',
        'onedrive' => 'Microsoft OneDrive',
        'opendrive' => 'OpenDrive',
        'pcloud' => 'pCloud',
        'premiumizeme' => 'premiumize.me',
        'putio' => 'Put.io',
        'qingstor' => 'QingStor',
        's3' => 'Amazon S3',
        'seafile' => 'Seafile',
        'sftp' => 'SFTP',
        'sharefile' => 'Citrix ShareFile',
        'sia' => 'Sia',
        'smb' => 'SMB / CIFS',
        'storj' => 'Storj',
        'sugarsync' => 'SugarSync',
        'swift' => 'OpenStack Swift',
        'union' => 'Union',
        'uptobox' => 'Uptobox',
        'webdav' => 'WebDAV',
        'yandex' => 'Yandex Disk',
        'zoho' => 'Zoho WorkDrive'
    ];
}

function testRcloneRemote($remoteName) {
    $configFile = getRcloneConfigPath();
    
    $output = [];
    $cmd = 'rclone lsd ' . escapeshellarg($remoteName . ':') . 
           ' --config ' . escapeshellarg($configFile) . 
           ' --max-depth 1 2>&1';
    
    exec($cmd, $output, $ret);
    
    return [
        'success' => ($ret === 0),
        'output' => implode("\n", $output)
    ];
}
