<?php
/**
 * D-PlaneOS Shares Management
 * Backend for SMB/NFS share configuration
 */

// SMB Configuration
function configureSambaShare($share) {
    $config = "[{$share['name']}]\n";
    $config .= "   path = {$share['dataset_path']}\n";
    $config .= "   browseable = " . ($share['smb_browseable'] ? 'yes' : 'no') . "\n";
    $config .= "   read only = " . ($share['smb_read_only'] ? 'yes' : 'no') . "\n";
    $config .= "   guest ok = " . ($share['smb_guest_ok'] ? 'yes' : 'no') . "\n";
    
    if (!empty($share['smb_valid_users'])) {
        $config .= "   valid users = {$share['smb_valid_users']}\n";
    }
    
    if (!empty($share['comment'])) {
        $config .= "   comment = {$share['comment']}\n";
    }
    
    $config .= "   create mask = 0664\n";
    $config .= "   directory mask = 0775\n";
    $config .= "   vfs objects = acl_xattr\n";
    $config .= "   map acl inherit = yes\n";
    $config .= "   store dos attributes = yes\n";
    
    return $config;
}

function writeSambaConfig() {
    $db = getDB();
    $stmt = $db->query("SELECT * FROM shares WHERE share_type = 'smb' AND enabled = 1");
    $shares = $stmt->fetchAll();
    
    // Read base config
    $baseConfig = "/etc/samba/smb.conf.base";
    if (!file_exists($baseConfig)) {
        // Create base config if not exists
        $base = "[global]\n";
        $base .= "   workgroup = WORKGROUP\n";
        $base .= "   server string = D-PlaneOS NAS\n";
        $base .= "   security = user\n";
        $base .= "   map to guest = bad user\n";
        $base .= "   dns proxy = no\n";
        $base .= "   load printers = no\n";
        $base .= "   printing = bsd\n";
        $base .= "   printcap name = /dev/null\n";
        $base .= "   disable spoolss = yes\n";
        $base .= "   log file = /var/log/samba/log.%m\n";
        $base .= "   max log size = 50\n";
        $base .= "   vfs objects = acl_xattr\n";
        $base .= "   map acl inherit = yes\n";
        $base .= "   store dos attributes = yes\n\n";
        
        atomicFileWrite($baseConfig, $base, 0644);
    }
    
    $config = file_get_contents($baseConfig);
    
    // Add shares
    foreach ($shares as $share) {
        $config .= "\n" . configureSambaShare($share);
    }
    
    // Write to temp file first
    $tempFile = '/tmp/smb.conf.new';
    atomicFileWrite($tempFile, $config, 0644);
    
    // Test config
    $output = [];
    exec('testparm -s ' . escapeshellarg($tempFile) . ' 2>&1', $output, $ret);
    
    if ($ret === 0) {
        // Config valid, copy to actual location
        copy($tempFile, '/etc/samba/smb.conf');
        unlink($tempFile);
        
        // Reload samba
        exec('sudo systemctl reload smbd 2>&1', $output, $ret);
        
        return ['success' => true, 'message' => 'Samba configuration updated'];
    } else {
        unlink($tempFile);
        return ['success' => false, 'error' => 'Invalid Samba configuration: ' . implode("\n", $output)];
    }
}

// NFS Configuration
function configureNFSExport($share) {
    $export = $share['dataset_path'];
    
    if (!empty($share['nfs_allowed_networks'])) {
        $networks = explode(',', $share['nfs_allowed_networks']);
        foreach ($networks as $network) {
            $network = trim($network);
            $options = [];
            
            if ($share['nfs_read_only']) {
                $options[] = 'ro';
            } else {
                $options[] = 'rw';
            }
            
            $options[] = $share['nfs_sync'];
            
            if ($share['nfs_no_root_squash']) {
                $options[] = 'no_root_squash';
            } else {
                $options[] = 'root_squash';
            }
            
            $options[] = 'no_subtree_check';
            
            $export .= " {$network}(" . implode(',', $options) . ")";
        }
    } else {
        // Default: allow all with ro
        $export .= " *(ro,sync,no_subtree_check,root_squash)";
    }
    
    return $export;
}

function writeNFSExports() {
    $db = getDB();
    $stmt = $db->query("SELECT * FROM shares WHERE share_type = 'nfs' AND enabled = 1");
    $shares = $stmt->fetchAll();
    
    $exports = "# D-PlaneOS NFS Exports\n";
    $exports .= "# Generated automatically - do not edit manually\n\n";
    
    foreach ($shares as $share) {
        if (!empty($share['comment'])) {
            $exports .= "# {$share['comment']}\n";
        }
        $exports .= configureNFSExport($share) . "\n";
    }
    
    // Write to temp file first
    $tempFile = '/tmp/exports.new';
    atomicFileWrite($tempFile, $exports, 0644);
    
    // Copy to actual location (atomic)
    atomicFileWrite('/etc/exports', $exports, 0644);
    unlink($tempFile);
    
    // Reload NFS
    $output = [];
    exec('sudo exportfs -ra 2>&1', $output, $ret);
    
    if ($ret === 0) {
        return ['success' => true, 'message' => 'NFS exports updated'];
    } else {
        return ['success' => false, 'error' => 'Failed to reload NFS: ' . implode("\n", $output)];
    }
}

// Samba User Management
function addSambaUser($username, $password) {
    // Create system user if not exists
    $output = [];
    exec('id ' . escapeshellarg($username) . ' 2>&1', $output, $ret);
    
    if ($ret !== 0) {
        // User doesn't exist, create it
        exec('sudo useradd -M -s /usr/sbin/nologin ' . escapeshellarg($username) . ' 2>&1', $output, $ret);
        if ($ret !== 0) {
            return ['success' => false, 'error' => 'Failed to create system user: ' . implode("\n", $output)];
        }
    }
    
    // Add to samba
    $process = proc_open(
        'sudo smbpasswd -a ' . escapeshellarg($username) . ' -s',
        [
            0 => ['pipe', 'r'],
            1 => ['pipe', 'w'],
            2 => ['pipe', 'w']
        ],
        $pipes
    );
    
    if (is_resource($process)) {
        fwrite($pipes[0], $password . "\n");
        fwrite($pipes[0], $password . "\n");
        fclose($pipes[0]);
        
        $output = stream_get_contents($pipes[1]);
        $error = stream_get_contents($pipes[2]);
        fclose($pipes[1]);
        fclose($pipes[2]);
        
        $ret = proc_close($process);
        
        if ($ret === 0) {
            // Enable user
            exec('sudo smbpasswd -e ' . escapeshellarg($username) . ' 2>&1');
            
            // Add to database
            $db = getDB();
            $stmt = $db->prepare('INSERT INTO smb_users (username) VALUES (?)');
            $stmt->execute([$username]);
            
            return ['success' => true, 'message' => 'Samba user created'];
        } else {
            return ['success' => false, 'error' => 'Failed to set Samba password: ' . $error];
        }
    }
    
    return ['success' => false, 'error' => 'Failed to execute smbpasswd'];
}

function removeSambaUser($username) {
    // Remove from samba
    $output = [];
    exec('sudo smbpasswd -x ' . escapeshellarg($username) . ' 2>&1', $output, $ret);
    
    // Remove system user
    exec('sudo userdel ' . escapeshellarg($username) . ' 2>&1', $output, $ret);
    
    // Remove from database
    $db = getDB();
    $stmt = $db->prepare('DELETE FROM smb_users WHERE username = ?');
    $stmt->execute([$username]);
    
    return ['success' => true, 'message' => 'Samba user removed'];
}

function changeSambaPassword($username, $newPassword) {
    $process = proc_open(
        'sudo smbpasswd -a ' . escapeshellarg($username) . ' -s',
        [
            0 => ['pipe', 'r'],
            1 => ['pipe', 'w'],
            2 => ['pipe', 'w']
        ],
        $pipes
    );
    
    if (is_resource($process)) {
        fwrite($pipes[0], $newPassword . "\n");
        fwrite($pipes[0], $newPassword . "\n");
        fclose($pipes[0]);
        
        $output = stream_get_contents($pipes[1]);
        $error = stream_get_contents($pipes[2]);
        fclose($pipes[1]);
        fclose($pipes[2]);
        
        $ret = proc_close($process);
        
        if ($ret === 0) {
            return ['success' => true, 'message' => 'Password changed'];
        } else {
            return ['success' => false, 'error' => 'Failed to change password: ' . $error];
        }
    }
    
    return ['success' => false, 'error' => 'Failed to execute smbpasswd'];
}

// Get share status
function getShareStatus($share) {
    if ($share['share_type'] === 'smb') {
        // Check if share is active in smbstatus
        $output = [];
        exec('sudo smbstatus -S 2>&1', $output);
        $shareName = $share['name'];
        foreach ($output as $line) {
            if (stripos($line, $shareName) !== false) {
                return 'active';
            }
        }
        return 'configured';
    } elseif ($share['share_type'] === 'nfs') {
        // Check if export is active
        $output = [];
        exec('sudo exportfs 2>&1', $output);
        $path = $share['dataset_path'];
        foreach ($output as $line) {
            if (stripos($line, $path) !== false) {
                return 'active';
            }
        }
        return 'configured';
    }
    
    return 'unknown';
}
