<?php
require_once __DIR__ . '/../../includes/auth.php';
requireAuth();
header('Content-Type: application/json');

try {
    $method = $_SERVER['REQUEST_METHOD'];
    
    if ($method === 'GET') {
        $action = $_GET['action'] ?? 'list';
        
        if ($action === 'list') {
            // Get all active notifications
            $db = getDB();
            $stmt = $db->query('SELECT * FROM notifications 
                WHERE (expires_at IS NULL OR expires_at > CURRENT_TIMESTAMP)
                AND dismissed = 0
                ORDER BY priority DESC, created_at DESC 
                LIMIT 100');
            
            $notifications = [];
            while ($row = $stmt->fetch()) {
                $notifications[] = $row;
            }
            
            echo json_encode(['success' => true, 'notifications' => $notifications]);
            
        } elseif ($action === 'unread_count') {
            // Get count of unread notifications
            $db = getDB();
            $stmt = $db->query('SELECT COUNT(*) as count FROM notifications 
                WHERE read = 0 
                AND dismissed = 0
                AND (expires_at IS NULL OR expires_at > CURRENT_TIMESTAMP)');
            $row = $stmt->fetch();
            
            echo json_encode(['success' => true, 'count' => $row['count']]);
            
        } elseif ($action === 'recent') {
            // Get recent notifications (last 24 hours)
            $db = getDB();
            $stmt = $db->query('SELECT * FROM notifications 
                WHERE created_at > datetime("now", "-24 hours")
                AND (expires_at IS NULL OR expires_at > CURRENT_TIMESTAMP)
                ORDER BY priority DESC, created_at DESC');
            
            $notifications = [];
            while ($row = $stmt->fetch()) {
                $notifications[] = $row;
            }
            
            echo json_encode(['success' => true, 'notifications' => $notifications]);
        }
        
    } elseif ($method === 'POST') {
        $input = json_decode(file_get_contents('php://input'), true);
        $action = $input['action'] ?? '';
        
        if ($action === 'create') {
            // Create new notification
            $title = $input['title'] ?? '';
            $message = $input['message'] ?? '';
            $type = $input['type'] ?? 'info';
            $category = $input['category'] ?? null;
            $priority = $input['priority'] ?? 0;
            
            if (!$title || !$message) {
                throw new Exception('Title and message required');
            }
            
            if (!in_array($type, ['info', 'warning', 'error', 'success'])) {
                throw new Exception('Invalid notification type');
            }
            
            $db = getDB();
            $stmt = $db->prepare('INSERT INTO notifications (title, message, type, category, priority)
                VALUES (?, ?, ?, ?, ?)');
            $stmt->execute([$title, $message, $type, $category, $priority]);
            
            echo json_encode(['success' => true, 'message' => 'Notification created']);
            
        } elseif ($action === 'mark_read') {
            $id = $input['id'] ?? 0;
            
            if (!$id) {
                throw new Exception('Notification ID required');
            }
            
            $db = getDB();
            $stmt = $db->prepare('UPDATE notifications SET read = 1 WHERE id = ?');
            $stmt->execute([$id]);
            
            echo json_encode(['success' => true, 'message' => 'Notification marked as read']);
            
        } elseif ($action === 'mark_all_read') {
            $db = getDB();
            $db->exec('UPDATE notifications SET read = 1 WHERE read = 0 AND dismissed = 0');
            
            echo json_encode(['success' => true, 'message' => 'All notifications marked as read']);
            
        } elseif ($action === 'dismiss') {
            $id = $input['id'] ?? 0;
            
            if (!$id) {
                throw new Exception('Notification ID required');
            }
            
            $db = getDB();
            $stmt = $db->prepare('UPDATE notifications SET dismissed = 1 WHERE id = ?');
            $stmt->execute([$id]);
            
            echo json_encode(['success' => true, 'message' => 'Notification dismissed']);
            
        } elseif ($action === 'dismiss_all') {
            $db = getDB();
            $db->exec('UPDATE notifications SET dismissed = 1 WHERE dismissed = 0');
            
            echo json_encode(['success' => true, 'message' => 'All notifications dismissed']);
            
        } elseif ($action === 'delete') {
            $id = $input['id'] ?? 0;
            
            if (!$id) {
                throw new Exception('Notification ID required');
            }
            
            $db = getDB();
            $stmt = $db->prepare('DELETE FROM notifications WHERE id = ?');
            $stmt->execute([$id]);
            
            echo json_encode(['success' => true, 'message' => 'Notification deleted']);
            
        } elseif ($action === 'cleanup') {
            // Delete old dismissed notifications (older than 7 days)
            $db = getDB();
            $db->exec('DELETE FROM notifications 
                WHERE dismissed = 1 
                AND created_at < datetime("now", "-7 days")');
            
            // Delete expired notifications
            $db->exec('DELETE FROM notifications 
                WHERE expires_at IS NOT NULL 
                AND expires_at < CURRENT_TIMESTAMP');
            
            echo json_encode(['success' => true, 'message' => 'Cleanup completed']);
        }
    }
    
} catch (Exception $e) {
    http_response_code(400);
    echo json_encode(['success' => false, 'error' => $e->getMessage()]);
}
?>
