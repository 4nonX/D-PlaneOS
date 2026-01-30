<?php
// Database migration handler
// Run this once after upgrading to add RBAC support

require_once __DIR__ . '/../system/dashboard/includes/auth.php';

function migrateDatabase() {
    $db = getDB();
    
    // Check if migration is needed
    try {
        $stmt = $db->query("SELECT role FROM users LIMIT 1");
        echo "✓ Role column already exists, migration not needed.\n";
        
        // Ensure at least one admin exists
        $adminCount = $db->query("SELECT COUNT(*) as count FROM users WHERE role = 'admin'")->fetch();
        if ($adminCount['count'] == 0) {
            echo "! No admin users found, setting first user as admin...\n";
            $db->exec("UPDATE users SET role = 'admin' WHERE id = 1");
            echo "✓ First user set as admin.\n";
        }
        
        return true;
    } catch (PDOException $e) {
        // Column doesn't exist, need to migrate
        echo "→ Role column not found, running migration...\n";
    }
    
    try {
        $db->beginTransaction();
        
        // Add role column
        echo "→ Adding role column...\n";
        $db->exec("ALTER TABLE users ADD COLUMN role TEXT DEFAULT 'user' CHECK(role IN ('admin', 'user', 'readonly'))");
        
        // Set all existing users to admin role for backward compatibility
        echo "→ Setting existing users as admins (backward compatibility)...\n";
        $db->exec("UPDATE users SET role = 'admin' WHERE role IS NULL");
        
        // Ensure user ID 1 is admin
        $db->exec("UPDATE users SET role = 'admin' WHERE id = 1");
        
        $db->commit();
        
        echo "✓ Migration completed successfully!\n";
        echo "\nRBAC is now enabled:\n";
        echo "  - admin: Full system access\n";
        echo "  - user: Can manage own resources, view system\n";
        echo "  - readonly: View-only access\n";
        
        return true;
    } catch (PDOException $e) {
        if ($db->inTransaction()) {
            $db->rollBack();
        }
        echo "✗ Migration failed: " . $e->getMessage() . "\n";
        return false;
    }
}

// Run migration if called directly
if (php_sapi_name() === 'cli') {
    echo "D-PlaneOS RBAC Migration\n";
    echo "========================\n\n";
    
    if (migrateDatabase()) {
        exit(0);
    } else {
        exit(1);
    }
}
