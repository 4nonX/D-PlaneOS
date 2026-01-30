# D-PlaneOS v1.9.0 Release Summary

## Security Fixes Applied (7 Critical/High)

### Critical (5)
✅ Session fixation vulnerability - Added `session_regenerate_id(true)` after login
✅ Sudoers wildcard rules - Created safe wrapper scripts for SMB user management
✅ Unvalidated action parameter - Added whitelist validation for container API
✅ Race conditions in file operations - Implemented atomic writes with file locking
✅ YAML injection in Docker Compose - Added comprehensive validation before deployment

### High Priority (2)
✅ Weak random number generation - Changed `rand()` to `random_int()`
✅ Integer overflow in logs - Capped lines parameter at 10,000

## New Feature: RBAC User Management

### What's New
- **3 Role Types:**
  - **Admin**: Full system access, user management
  - **User**: Manage own resources, view system status
  - **Readonly**: View-only access, no modifications

- **User Management UI** at `/users.php`
- **User Management API** at `/api/system/users.php`
- **Automatic Migration** - Existing users become admin automatically

### Key Features
- Create/edit/delete users with role assignment
- Prevent last admin deletion
- Prevent self-role modification
- Integration with SMB/system users (except readonly)
- Full audit logging
- Permission helper functions in code

### Backward Compatibility
**ZERO breaking changes:**
- Existing installations upgrade automatically
- All existing users become admin
- All functionality continues to work
- Migration is safe and idempotent

## Files Modified

### Security Fixes
1. `system/dashboard/login.php` - Session regeneration
2. `system/dashboard/includes/security.php` - Atomic writes, random_int
3. `system/dashboard/includes/shares.php` - Atomic file operations
4. `system/dashboard/api/containers/containers.php` - Action validation, YAML validation, bounds checking
5. `system/config/sudoers.enhanced` - Safe wrapper scripts
6. `system/scripts/smb-user-add.sh` - NEW: Safe user creation
7. `system/scripts/smb-user-del.sh` - NEW: Safe user deletion

### RBAC Implementation
8. `database/schema.sql` - Added role column
9. `database/migrate.php` - NEW: Automatic migration script
10. `database/migrations/001_add_rbac.sql` - NEW: SQL migration
11. `system/dashboard/includes/auth.php` - RBAC functions
12. `system/dashboard/api/system/users.php` - Full RBAC support
13. `system/dashboard/users.php` - NEW: User management UI

## Deployment

### Fresh Install
1. Extract tarball to `/opt/dplaneos`
2. Run installer
3. Login as admin (default password: admin123)
4. Change admin password immediately
5. Access `/users.php` to create additional users

### Upgrade from v1.8.x or earlier
1. **Backup first:** `cp -r /opt/dplaneos /opt/dplaneos.backup`
2. **Backup database:** `cp /var/dplane/database/dplane.db /var/dplane/database/dplane.db.backup`
3. Stop D-PlaneOS: `systemctl stop dplaneos`
4. Extract tarball over existing installation
5. Copy new sudoers: `cp /opt/dplaneos/system/config/sudoers.enhanced /etc/sudoers.d/dplaneos`
6. Verify permissions: `chmod +x /opt/dplaneos/system/scripts/*.sh`
7. Start D-PlaneOS: `systemctl start dplaneos`
8. Login - migration runs automatically
9. Verify: Access `/users.php`, all existing users should be admin

### Quick Migration Check
```bash
# Run migration manually if needed
cd /opt/dplaneos/database
php migrate.php

# Should output:
# ✓ Migration completed successfully!
# RBAC is now enabled
```

## Access Control

### Protected Resources
Use new functions in your code:

```php
requireAdmin();      // Only admins
requireWrite();      // Admins and users
requireRole('user'); // Specific role

if (isAdmin()) { /* show admin options */ }
if (canWrite()) { /* show edit buttons */ }
```

### API Permissions
Add to existing APIs:
```php
require_once __DIR__ . '/includes/auth.php';
requireAuth();
requireWrite(); // Add this line
```

## Testing Checklist

### Security Validation
- [ ] Session ID changes after login
- [ ] Cannot create users via sudo useradd wildcard
- [ ] Invalid actions rejected in container API
- [ ] Concurrent file writes don't corrupt configs
- [ ] Malicious YAML rejected in compose deployment
- [ ] Log requests capped at 10k lines

### RBAC Validation
- [ ] Migration completes without errors
- [ ] Existing users have admin role
- [ ] Can create user with each role type
- [ ] Admin can access /users.php
- [ ] Regular user cannot access /users.php
- [ ] Readonly user cannot modify anything
- [ ] Cannot delete last admin
- [ ] Cannot change own role
- [ ] System users created correctly

## Known Limitations

1. **Single-host only** - No distributed RBAC yet
2. **No resource-level permissions** - Role applies globally
3. **No custom roles** - Only admin/user/readonly
4. **No LDAP/AD** - Local authentication only
5. **CLI access bypasses RBAC** - Direct PHP scripts run as admin

## Future Roadmap

- [ ] Resource-level permissions (per-container, per-share)
- [ ] Custom role definitions
- [ ] API key authentication
- [ ] LDAP/Active Directory integration
- [ ] Audit log viewer
- [ ] User activity monitoring
- [ ] Two-factor authentication

## Rollback

If issues occur:
```bash
# Stop service
systemctl stop dplaneos

# Restore backup
rm -rf /opt/dplaneos
cp -r /opt/dplaneos.backup /opt/dplaneos

# Restore database
cp /var/dplane/database/dplane.db.backup /var/dplane/database/dplane.db

# Start service
systemctl start dplaneos
```

## Support

Logs location:
- System: `/var/log/dplaneos/`
- PHP errors: `/var/log/nginx/error.log` or `/var/log/apache2/error.log`
- Audit: Database table `audit_log`

Check migration status:
```bash
sqlite3 /var/dplane/database/dplane.db "PRAGMA table_info(users);" | grep role
# Should show: role|TEXT|0|'user'|0
```

---

**Version:** 1.9.0  
**Release Date:** 2026-01-30  
**Security Fixes:** 7 (5 Critical, 2 High)  
**New Features:** RBAC User Management  
**Breaking Changes:** None  
**Migration:** Automatic
