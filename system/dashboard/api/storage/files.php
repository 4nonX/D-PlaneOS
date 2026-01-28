<?php
require_once __DIR__ . '/../../includes/auth.php';
requireAuth();
header('Content-Type: application/json');

try {
    $method = $_SERVER['REQUEST_METHOD'];
    
    if ($method === 'GET') {
        $action = $_GET['action'] ?? 'list';
        
        if ($action === 'list') {
            // List directory contents
            $path = $_GET['path'] ?? '/mnt';
            $path = realpath($path);
            
            // Security: Ensure path is within /mnt
            if (!$path || strpos($path, '/mnt') !== 0) {
                throw new Exception('Invalid path');
            }
            
            if (!is_dir($path)) {
                throw new Exception('Path is not a directory');
            }
            
            $items = [];
            $files = scandir($path);
            
            foreach ($files as $file) {
                if ($file === '.' || $file === '..') continue;
                
                $fullPath = $path . '/' . $file;
                $stat = stat($fullPath);
                
                $item = [
                    'name' => $file,
                    'path' => $fullPath,
                    'type' => is_dir($fullPath) ? 'directory' : 'file',
                    'size' => $stat['size'],
                    'size_human' => formatBytes($stat['size']),
                    'modified' => $stat['mtime'],
                    'modified_human' => date('Y-m-d H:i:s', $stat['mtime']),
                    'permissions' => substr(sprintf('%o', fileperms($fullPath)), -4),
                    'owner' => posix_getpwuid($stat['uid'])['name'] ?? $stat['uid'],
                    'group' => posix_getgrgid($stat['gid'])['name'] ?? $stat['gid'],
                ];
                
                // Get MIME type for files
                if ($item['type'] === 'file' && function_exists('mime_content_type')) {
                    $item['mime'] = mime_content_type($fullPath);
                }
                
                $items[] = $item;
            }
            
            // Sort: directories first, then by name
            usort($items, function($a, $b) {
                if ($a['type'] !== $b['type']) {
                    return $a['type'] === 'directory' ? -1 : 1;
                }
                return strcasecmp($a['name'], $b['name']);
            });
            
            echo json_encode([
                'success' => true,
                'path' => $path,
                'items' => $items,
                'count' => count($items)
            ]);
            
        } elseif ($action === 'download') {
            // Download file
            $path = $_GET['path'] ?? '';
            $path = realpath($path);
            
            if (!$path || strpos($path, '/mnt') !== 0) {
                throw new Exception('Invalid path');
            }
            
            if (!is_file($path)) {
                throw new Exception('File not found');
            }
            
            logAction('file_download', 'file', $path);
            
            // Set headers for download
            header('Content-Type: application/octet-stream');
            header('Content-Disposition: attachment; filename="' . basename($path) . '"');
            header('Content-Length: ' . filesize($path));
            header('Cache-Control: must-revalidate');
            header('Pragma: public');
            
            // Stream file
            readfile($path);
            exit;
            
        } elseif ($action === 'preview') {
            // Preview file content (text files only)
            $path = $_GET['path'] ?? '';
            $path = realpath($path);
            $maxSize = 1024 * 1024; // 1MB max for preview
            
            if (!$path || strpos($path, '/mnt') !== 0) {
                throw new Exception('Invalid path');
            }
            
            if (!is_file($path)) {
                throw new Exception('File not found');
            }
            
            if (filesize($path) > $maxSize) {
                throw new Exception('File too large for preview (max 1MB)');
            }
            
            // Check if text file
            $mime = mime_content_type($path);
            if (strpos($mime, 'text/') !== 0 && $mime !== 'application/json') {
                throw new Exception('File is not a text file');
            }
            
            $content = file_get_contents($path);
            
            echo json_encode([
                'success' => true,
                'path' => $path,
                'content' => $content,
                'mime' => $mime,
                'size' => filesize($path)
            ]);
            
        } elseif ($action === 'search') {
            // Search for files
            $path = $_GET['path'] ?? '/mnt';
            $query = $_GET['query'] ?? '';
            $path = realpath($path);
            
            if (!$path || strpos($path, '/mnt') !== 0) {
                throw new Exception('Invalid path');
            }
            
            if (strlen($query) < 2) {
                throw new Exception('Search query too short (minimum 2 characters)');
            }
            
            $cmd = 'find ' . escapeshellarg($path) . ' -name ' . escapeshellarg('*' . $query . '*') . ' 2>/dev/null | head -100';
            $output = [];
            exec($cmd, $output);
            
            $results = [];
            foreach ($output as $file) {
                if (is_file($file) || is_dir($file)) {
                    $stat = stat($file);
                    $results[] = [
                        'name' => basename($file),
                        'path' => $file,
                        'type' => is_dir($file) ? 'directory' : 'file',
                        'size' => $stat['size'],
                        'size_human' => formatBytes($stat['size']),
                        'modified' => $stat['mtime'],
                        'modified_human' => date('Y-m-d H:i:s', $stat['mtime']),
                    ];
                }
            }
            
            echo json_encode([
                'success' => true,
                'query' => $query,
                'results' => $results,
                'count' => count($results)
            ]);
            
        } else {
            throw new Exception('Unknown action');
        }
        
    } elseif ($method === 'POST') {
        $input = json_decode(file_get_contents('php://input'), true);
        $action = $input['action'] ?? '';
        
        if ($action === 'create_folder') {
            $path = $input['path'] ?? '';
            $name = $input['name'] ?? '';
            
            // Validate
            if (!$name || preg_match('/[^\w\-\.]/', $name)) {
                throw new Exception('Invalid folder name');
            }
            
            $fullPath = realpath($path) . '/' . $name;
            
            if (strpos($fullPath, '/mnt') !== 0) {
                throw new Exception('Invalid path');
            }
            
            if (file_exists($fullPath)) {
                throw new Exception('Folder already exists');
            }
            
            if (!mkdir($fullPath, 0755, true)) {
                throw new Exception('Failed to create folder');
            }
            
            logAction('folder_create', 'folder', $fullPath);
            
            echo json_encode(['success' => true, 'path' => $fullPath]);
            
        } elseif ($action === 'delete') {
            $path = $input['path'] ?? '';
            $path = realpath($path);
            $recursive = $input['recursive'] ?? false;
            
            if (!$path || strpos($path, '/mnt') !== 0) {
                throw new Exception('Invalid path');
            }
            
            if (!file_exists($path)) {
                throw new Exception('File or folder not found');
            }
            
            if (is_dir($path)) {
                if ($recursive) {
                    $cmd = 'rm -rf ' . escapeshellarg($path);
                    exec($cmd, $output, $ret);
                    if ($ret !== 0) {
                        throw new Exception('Failed to delete folder');
                    }
                } else {
                    if (!rmdir($path)) {
                        throw new Exception('Failed to delete folder (not empty?)');
                    }
                }
            } else {
                if (!unlink($path)) {
                    throw new Exception('Failed to delete file');
                }
            }
            
            logAction('file_delete', is_dir($path) ? 'folder' : 'file', $path);
            
            echo json_encode(['success' => true, 'path' => $path]);
            
        } elseif ($action === 'rename') {
            $oldPath = $input['path'] ?? '';
            $newName = $input['new_name'] ?? '';
            $oldPath = realpath($oldPath);
            
            if (!$oldPath || strpos($oldPath, '/mnt') !== 0) {
                throw new Exception('Invalid path');
            }
            
            if (!$newName || preg_match('/[^\w\-\.\s]/', $newName)) {
                throw new Exception('Invalid name');
            }
            
            if (!file_exists($oldPath)) {
                throw new Exception('File or folder not found');
            }
            
            $newPath = dirname($oldPath) . '/' . $newName;
            
            if (file_exists($newPath)) {
                throw new Exception('Target already exists');
            }
            
            if (!rename($oldPath, $newPath)) {
                throw new Exception('Failed to rename');
            }
            
            logAction('file_rename', 'file', $oldPath . ' -> ' . $newPath);
            
            echo json_encode(['success' => true, 'old_path' => $oldPath, 'new_path' => $newPath]);
            
        } elseif ($action === 'move') {
            $sourcePath = $input['source'] ?? '';
            $destPath = $input['destination'] ?? '';
            $sourcePath = realpath($sourcePath);
            $destPath = realpath($destPath);
            
            if (!$sourcePath || strpos($sourcePath, '/mnt') !== 0) {
                throw new Exception('Invalid source path');
            }
            
            if (!$destPath || strpos($destPath, '/mnt') !== 0) {
                throw new Exception('Invalid destination path');
            }
            
            if (!file_exists($sourcePath)) {
                throw new Exception('Source not found');
            }
            
            if (!is_dir($destPath)) {
                throw new Exception('Destination is not a directory');
            }
            
            $newPath = $destPath . '/' . basename($sourcePath);
            
            if (file_exists($newPath)) {
                throw new Exception('Target already exists');
            }
            
            if (!rename($sourcePath, $newPath)) {
                throw new Exception('Failed to move');
            }
            
            logAction('file_move', 'file', $sourcePath . ' -> ' . $newPath);
            
            echo json_encode(['success' => true, 'source' => $sourcePath, 'destination' => $newPath]);
            
        } elseif ($action === 'copy') {
            $sourcePath = $input['source'] ?? '';
            $destPath = $input['destination'] ?? '';
            $sourcePath = realpath($sourcePath);
            $destPath = realpath($destPath);
            
            if (!$sourcePath || strpos($sourcePath, '/mnt') !== 0) {
                throw new Exception('Invalid source path');
            }
            
            if (!$destPath || strpos($destPath, '/mnt') !== 0) {
                throw new Exception('Invalid destination path');
            }
            
            if (!file_exists($sourcePath)) {
                throw new Exception('Source not found');
            }
            
            if (!is_dir($destPath)) {
                throw new Exception('Destination is not a directory');
            }
            
            $newPath = $destPath . '/' . basename($sourcePath);
            
            if (file_exists($newPath)) {
                throw new Exception('Target already exists');
            }
            
            if (is_dir($sourcePath)) {
                $cmd = 'cp -r ' . escapeshellarg($sourcePath) . ' ' . escapeshellarg($newPath);
                exec($cmd, $output, $ret);
                if ($ret !== 0) {
                    throw new Exception('Failed to copy directory');
                }
            } else {
                if (!copy($sourcePath, $newPath)) {
                    throw new Exception('Failed to copy file');
                }
            }
            
            logAction('file_copy', 'file', $sourcePath . ' -> ' . $newPath);
            
            echo json_encode(['success' => true, 'source' => $sourcePath, 'destination' => $newPath]);
            
        } elseif ($action === 'chmod') {
            $path = $input['path'] ?? '';
            $mode = $input['mode'] ?? '';
            $path = realpath($path);
            
            if (!$path || strpos($path, '/mnt') !== 0) {
                throw new Exception('Invalid path');
            }
            
            if (!preg_match('/^[0-7]{3,4}$/', $mode)) {
                throw new Exception('Invalid mode (use octal format like 755)');
            }
            
            if (!file_exists($path)) {
                throw new Exception('File not found');
            }
            
            if (!chmod($path, octdec($mode))) {
                throw new Exception('Failed to change permissions');
            }
            
            logAction('file_chmod', 'file', $path . ' -> ' . $mode);
            
            echo json_encode(['success' => true, 'path' => $path, 'mode' => $mode]);
            
        } elseif ($action === 'chown') {
            $path = $input['path'] ?? '';
            $owner = $input['owner'] ?? '';
            $group = $input['group'] ?? '';
            $path = realpath($path);
            
            if (!$path || strpos($path, '/mnt') !== 0) {
                throw new Exception('Invalid path');
            }
            
            if (!file_exists($path)) {
                throw new Exception('File not found');
            }
            
            $cmd = 'sudo chown ';
            if ($owner && $group) {
                $cmd .= escapeshellarg($owner . ':' . $group);
            } elseif ($owner) {
                $cmd .= escapeshellarg($owner);
            } else {
                throw new Exception('Owner or group required');
            }
            $cmd .= ' ' . escapeshellarg($path);
            
            exec($cmd, $output, $ret);
            if ($ret !== 0) {
                throw new Exception('Failed to change ownership');
            }
            
            logAction('file_chown', 'file', $path . ' -> ' . $owner . ':' . $group);
            
            echo json_encode(['success' => true, 'path' => $path]);
            
        } else {
            throw new Exception('Unknown action');
        }
        
    } elseif ($method === 'PUT') {
        // File upload
        $path = $_GET['path'] ?? '';
        $path = realpath($path);
        
        if (!$path || strpos($path, '/mnt') !== 0) {
            throw new Exception('Invalid path');
        }
        
        if (!is_dir($path)) {
            throw new Exception('Upload path is not a directory');
        }
        
        // Get file from raw input
        $fileName = $_GET['filename'] ?? 'upload_' . time();
        $fullPath = $path . '/' . basename($fileName);
        
        if (file_exists($fullPath)) {
            throw new Exception('File already exists');
        }
        
        $input = fopen('php://input', 'r');
        $output = fopen($fullPath, 'w');
        
        $bytes = stream_copy_to_stream($input, $output);
        
        fclose($input);
        fclose($output);
        
        if ($bytes === false) {
            throw new Exception('Upload failed');
        }
        
        logAction('file_upload', 'file', $fullPath . ' (' . formatBytes($bytes) . ')');
        
        echo json_encode([
            'success' => true,
            'path' => $fullPath,
            'size' => $bytes,
            'size_human' => formatBytes($bytes)
        ]);
        
    } else {
        http_response_code(405);
        throw new Exception('Method not allowed');
    }
    
} catch (Exception $e) {
    http_response_code(400);
    echo json_encode(['success' => false, 'error' => $e->getMessage()]);
}

function formatBytes($bytes, $precision = 2) {
    $units = ['B', 'KB', 'MB', 'GB', 'TB'];
    $bytes = max($bytes, 0);
    $pow = floor(($bytes ? log($bytes) : 0) / log(1024));
    $pow = min($pow, count($units) - 1);
    $bytes /= pow(1024, $pow);
    return round($bytes, $precision) . ' ' . $units[$pow];
}
