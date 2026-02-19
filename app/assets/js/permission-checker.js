// Permission Checker Utility
// Provides frontend permission checking and UI element hiding/showing

class PermissionChecker {
    constructor() {
        this.permissions = new Map();
        this.roles = [];
        this.loaded = false;
    }

    // Load current user's permissions
    async load() {
        try {
            const response = await fetch('/api/rbac/me/permissions', {
                headers: {
                    'X-Session-Token': this.getSessionToken()
                }
            });

            if (!response.ok) {
                throw new Error('Failed to load permissions');
            }

            const data = await response.json();
            
            // Store permissions in Map for fast lookup
            data.permissions.forEach(perm => {
                const key = `${perm.resource}:${perm.action}`;
                this.permissions.set(key, true);
            });

            // Also store the 'can' map if provided
            if (data.can) {
                Object.entries(data.can).forEach(([key, value]) => {
                    this.permissions.set(key, value);
                });
            }

            this.loaded = true;

            // Also load roles
            await this.loadRoles();
            
            // Trigger permission-loaded event
            window.dispatchEvent(new CustomEvent('permissions-loaded', { 
                detail: { permissions: data.permissions } 
            }));

            return data.permissions;

        } catch (error) {
            console.error('Failed to load permissions:', error);
            this.loaded = false;
            return [];
        }
    }

    // Load current user's roles
    async loadRoles() {
        try {
            const response = await fetch('/api/rbac/me/roles', {
                headers: { 'X-Session-Token': this.getSessionToken() }
            });
            if (response.ok) {
                const data = await response.json();
                this.roles = data.roles || [];
            }
        } catch (e) {
            console.warn('Failed to load roles:', e);
        }
    }

    // Server-side permission check (for critical operations)
    async checkPermission(resource, action) {
        try {
            const response = await fetch('/api/rbac/check', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    'X-Session-Token': this.getSessionToken()
                },
                body: JSON.stringify({ resource, action })
            });
            const data = await response.json();
            return data.allowed === true;
        } catch (e) {
            return false;
        }
    }

    // Get user roles for a specific user (admin)
    async getUserRoles(userId) {
        try {
            const r = await fetch(`/api/rbac/users/${userId}/roles`, {
                headers: { 'X-Session-Token': this.getSessionToken() }
            });
            const d = await r.json();
            return d.roles || [];
        } catch (e) { return []; }
    }

    // Get user permissions for a specific user (admin)
    async getUserPermissions(userId) {
        try {
            const r = await fetch(`/api/rbac/users/${userId}/permissions`, {
                headers: { 'X-Session-Token': this.getSessionToken() }
            });
            const d = await r.json();
            return d.permissions || [];
        } catch (e) { return []; }
    }

    // Assign role to user (admin)
    async assignRoleToUser(userId, roleId) {
        const r = await fetch(`/api/rbac/users/${userId}/roles`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json', 'X-Session-Token': this.getSessionToken() },
            body: JSON.stringify({ role_id: roleId })
        });
        return r.json();
    }

    // Remove role from user (admin)
    async removeRoleFromUser(userId, roleId) {
        const r = await fetch(`/api/rbac/users/${userId}/roles/${roleId}`, {
            method: 'DELETE',
            headers: { 'X-Session-Token': this.getSessionToken() }
        });
        return r.json();
    }

    // Check if user has a specific permission
    can(resource, action) {
        const key = `${resource}:${action}`;
        return this.permissions.get(key) === true;
    }

    // Check if user has any of the permissions
    canAny(...permissions) {
        return permissions.some(perm => {
            const [resource, action] = perm.split(':');
            return this.can(resource, action);
        });
    }

    // Check if user has all of the permissions
    canAll(...permissions) {
        return permissions.every(perm => {
            const [resource, action] = perm.split(':');
            return this.can(resource, action);
        });
    }

    // Get session token from cookie or localStorage
    getSessionToken() {
        // Try cookie first
        const cookies = document.cookie.split(';');
        for (let cookie of cookies) {
            const [name, value] = cookie.trim().split('=');
            if (name === 'session_token') {
                return value;
            }
        }

        // Fallback to localStorage
        return localStorage.getItem('session_token') || '';
    }

    // Hide element if user doesn't have permission
    hideIfCannot(element, resource, action) {
        if (!this.can(resource, action)) {
            if (typeof element === 'string') {
                element = document.querySelector(element);
            }
            if (element) {
                element.style.display = 'none';
            }
        }
    }

    // Show element only if user has permission
    showIfCan(element, resource, action) {
        if (typeof element === 'string') {
            element = document.querySelector(element);
        }
        
        if (element) {
            element.style.display = this.can(resource, action) ? '' : 'none';
        }
    }

    // Disable element if user doesn't have permission
    disableIfCannot(element, resource, action) {
        if (!this.can(resource, action)) {
            if (typeof element === 'string') {
                element = document.querySelector(element);
            }
            if (element) {
                element.disabled = true;
                element.style.opacity = '0.5';
                element.style.cursor = 'not-allowed';
            }
        }
    }

    // Apply permissions to all elements with data-permission attribute
    applyToPage() {
        // Find all elements with data-permission
        document.querySelectorAll('[data-permission]').forEach(element => {
            const permission = element.getAttribute('data-permission');
            const [resource, action] = permission.split(':');
            
            if (!this.can(resource, action)) {
                const hideMode = element.getAttribute('data-permission-mode') || 'hide';
                
                if (hideMode === 'hide') {
                    element.style.display = 'none';
                } else if (hideMode === 'disable') {
                    element.disabled = true;
                    element.style.opacity = '0.5';
                    element.style.cursor = 'not-allowed';
                }
            }
        });

        // Find all elements with data-permission-any
        document.querySelectorAll('[data-permission-any]').forEach(element => {
            const permissions = element.getAttribute('data-permission-any').split(',');
            const hasAny = this.canAny(...permissions);
            
            if (!hasAny) {
                const hideMode = element.getAttribute('data-permission-mode') || 'hide';
                
                if (hideMode === 'hide') {
                    element.style.display = 'none';
                } else if (hideMode === 'disable') {
                    element.disabled = true;
                }
            }
        });

        // Find all elements with data-permission-all
        document.querySelectorAll('[data-permission-all]').forEach(element => {
            const permissions = element.getAttribute('data-permission-all').split(',');
            const hasAll = this.canAll(...permissions);
            
            if (!hasAll) {
                const hideMode = element.getAttribute('data-permission-mode') || 'hide';
                
                if (hideMode === 'hide') {
                    element.style.display = 'none';
                } else if (hideMode === 'disable') {
                    element.disabled = true;
                }
            }
        });
    }

    // Check if permission checking is enabled
    isEnabled() {
        return this.loaded;
    }
}

// Create global instance
window.permissions = new PermissionChecker();

// Auto-load permissions when DOM is ready
document.addEventListener('DOMContentLoaded', async () => {
    try {
        await window.permissions.load();
        
        // Apply permissions to current page
        window.permissions.applyToPage();
        
    } catch (error) {
        console.warn('Permission checking disabled:', error);
    }
});

// Provide helper function for inline checks
function can(resource, action) {
    return window.permissions.can(resource, action);
}

function canAny(...permissions) {
    return window.permissions.canAny(...permissions);
}

function canAll(...permissions) {
    return window.permissions.canAll(...permissions);
}
