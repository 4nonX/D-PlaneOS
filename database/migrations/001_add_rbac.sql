-- Migration: Add RBAC support to existing installations
-- This migration is idempotent and can be run multiple times safely

-- Add role column if it doesn't exist (SQLite doesn't support IF NOT EXISTS for columns)
-- Check if column exists by attempting to select it
-- This will be handled by PHP migration script

-- For manual migration:
-- 1. Backup database first: cp /var/dplane/data/dplane.db /var/dplane/data/dplane.db.backup
-- 2. Run: sqlite3 /var/dplane/data/dplane.db < /opt/dplaneos/database/migrations/001_add_rbac.sql

BEGIN TRANSACTION;

-- Add role column (will error if already exists, which is fine)
ALTER TABLE users ADD COLUMN role TEXT DEFAULT 'user' CHECK(role IN ('admin', 'user', 'readonly'));

-- Update existing users to admin role (backward compatibility)
UPDATE users SET role = 'admin' WHERE role IS NULL;

-- Ensure at least one admin exists
UPDATE users SET role = 'admin' WHERE id = 1;

COMMIT;
