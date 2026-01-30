<?php
require_once __DIR__ . '/includes/auth.php';
requireAuth();
requireAdmin(); // Only admins can access user management

$user = getCurrentUser();
?>
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>User Management - D-PlaneOS</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
            background: #f5f5f5;
            padding: 20px;
        }
        .container {
            max-width: 1200px;
            margin: 0 auto;
            background: white;
            border-radius: 8px;
            padding: 30px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
        h1 {
            margin-bottom: 10px;
            color: #333;
        }
        .subtitle {
            color: #666;
            margin-bottom: 30px;
        }
        .header-actions {
            display: flex;
            justify-content: space-between;
            align-items: center;
            margin-bottom: 20px;
        }
        .btn {
            padding: 10px 20px;
            border: none;
            border-radius: 6px;
            cursor: pointer;
            font-size: 14px;
            font-weight: 500;
            transition: all 0.2s;
        }
        .btn-primary {
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            color: white;
        }
        .btn-primary:hover { transform: translateY(-1px); box-shadow: 0 4px 8px rgba(0,0,0,0.2); }
        .btn-secondary { background: #6c757d; color: white; }
        .btn-danger { background: #dc3545; color: white; }
        .btn-small { padding: 6px 12px; font-size: 13px; }
        table {
            width: 100%;
            border-collapse: collapse;
            margin-top: 20px;
        }
        th, td {
            padding: 12px;
            text-align: left;
            border-bottom: 1px solid #eee;
        }
        th {
            background: #f8f9fa;
            font-weight: 600;
            color: #495057;
        }
        .role-badge {
            padding: 4px 8px;
            border-radius: 4px;
            font-size: 12px;
            font-weight: 500;
        }
        .role-admin { background: #ffc107; color: #000; }
        .role-user { background: #007bff; color: white; }
        .role-readonly { background: #6c757d; color: white; }
        .actions {
            display: flex;
            gap: 8px;
        }
        .modal {
            display: none;
            position: fixed;
            top: 0;
            left: 0;
            width: 100%;
            height: 100%;
            background: rgba(0,0,0,0.5);
            justify-content: center;
            align-items: center;
            z-index: 1000;
        }
        .modal.show { display: flex; }
        .modal-content {
            background: white;
            padding: 30px;
            border-radius: 8px;
            max-width: 500px;
            width: 90%;
            max-height: 90vh;
            overflow-y: auto;
        }
        .modal-header {
            margin-bottom: 20px;
        }
        .modal-header h2 {
            color: #333;
        }
        .form-group {
            margin-bottom: 20px;
        }
        .form-group label {
            display: block;
            margin-bottom: 8px;
            font-weight: 500;
            color: #333;
        }
        .form-group input, .form-group select {
            width: 100%;
            padding: 10px;
            border: 2px solid #e0e0e0;
            border-radius: 6px;
            font-size: 14px;
        }
        .form-group input:focus, .form-group select:focus {
            outline: none;
            border-color: #667eea;
        }
        .form-actions {
            display: flex;
            gap: 10px;
            justify-content: flex-end;
            margin-top: 20px;
        }
        .error {
            background: #fee;
            color: #c33;
            padding: 12px;
            border-radius: 6px;
            margin-bottom: 20px;
            border: 1px solid #fcc;
        }
        .success {
            background: #efe;
            color: #3c3;
            padding: 12px;
            border-radius: 6px;
            margin-bottom: 20px;
            border: 1px solid #cfc;
        }
        .back-link {
            display: inline-block;
            margin-bottom: 20px;
            color: #667eea;
            text-decoration: none;
        }
        .back-link:hover { text-decoration: underline; }
        .role-description {
            font-size: 12px;
            color: #666;
            margin-top: 4px;
        }
    </style>
</head>
<body>
    <div class="container">
        <a href="/" class="back-link">← Back to Dashboard</a>
        
        <h1>User Management</h1>
        <p class="subtitle">Manage system users and their roles (RBAC)</p>
        
        <div id="message"></div>
        
        <div class="header-actions">
            <div>
                <strong>Role Types:</strong><br>
                <small>
                    <span class="role-badge role-admin">Admin</span> Full access |
                    <span class="role-badge role-user">User</span> Manage own resources |
                    <span class="role-badge role-readonly">Readonly</span> View only
                </small>
            </div>
            <button class="btn btn-primary" onclick="showCreateModal()">+ Create User</button>
        </div>
        
        <table id="usersTable">
            <thead>
                <tr>
                    <th>ID</th>
                    <th>Username</th>
                    <th>Email</th>
                    <th>Role</th>
                    <th>Created</th>
                    <th>Last Login</th>
                    <th>Actions</th>
                </tr>
            </thead>
            <tbody id="usersBody">
                <tr><td colspan="7" style="text-align:center;">Loading...</td></tr>
            </tbody>
        </table>
    </div>
    
    <!-- Create/Edit User Modal -->
    <div id="userModal" class="modal">
        <div class="modal-content">
            <div class="modal-header">
                <h2 id="modalTitle">Create User</h2>
            </div>
            <form id="userForm">
                <input type="hidden" id="userId" name="id">
                
                <div class="form-group">
                    <label for="username">Username *</label>
                    <input type="text" id="username" name="username" required pattern="[a-zA-Z0-9_-]{3,32}" 
                           title="3-32 characters, alphanumeric, dash, underscore only">
                </div>
                
                <div class="form-group" id="passwordGroup">
                    <label for="password">Password *</label>
                    <input type="password" id="password" name="password" minlength="8"
                           title="Minimum 8 characters">
                </div>
                
                <div class="form-group">
                    <label for="email">Email</label>
                    <input type="email" id="email" name="email">
                </div>
                
                <div class="form-group">
                    <label for="role">Role *</label>
                    <select id="role" name="role" required>
                        <option value="user">User - Can manage own resources</option>
                        <option value="readonly">Readonly - View only access</option>
                        <option value="admin">Admin - Full system access</option>
                    </select>
                    <div class="role-description" id="roleDesc"></div>
                </div>
                
                <div class="form-actions">
                    <button type="button" class="btn btn-secondary" onclick="closeModal()">Cancel</button>
                    <button type="submit" class="btn btn-primary" id="submitBtn">Create</button>
                </div>
            </form>
        </div>
    </div>
    
    <script>
        let users = [];
        
        // Load users on page load
        document.addEventListener('DOMContentLoaded', () => {
            loadUsers();
        });
        
        async function loadUsers() {
            try {
                const response = await fetch('/api/system/users.php?action=list');
                const data = await response.json();
                
                if (data.success) {
                    users = data.users;
                    renderUsers();
                } else {
                    showError(data.error);
                }
            } catch (error) {
                showError('Failed to load users: ' + error.message);
            }
        }
        
        function renderUsers() {
            const tbody = document.getElementById('usersBody');
            
            if (users.length === 0) {
                tbody.innerHTML = '<tr><td colspan="7" style="text-align:center;">No users found</td></tr>';
                return;
            }
            
            tbody.innerHTML = users.map(user => `
                <tr>
                    <td>${user.id}</td>
                    <td><strong>${escapeHtml(user.username)}</strong></td>
                    <td>${escapeHtml(user.email || '-')}</td>
                    <td><span class="role-badge role-${user.role}">${user.role}</span></td>
                    <td>${formatDate(user.created_at)}</td>
                    <td>${user.last_login ? formatDate(user.last_login) : 'Never'}</td>
                    <td class="actions">
                        <button class="btn btn-small btn-secondary" onclick="editUser(${user.id})">Edit</button>
                        <button class="btn btn-small btn-danger" onclick="deleteUser(${user.id}, '${escapeHtml(user.username)}')" 
                                ${user.role === 'admin' && users.filter(u => u.role === 'admin').length === 1 ? 'disabled title="Cannot delete last admin"' : ''}>
                            Delete
                        </button>
                    </td>
                </tr>
            `).join('');
        }
        
        function showCreateModal() {
            document.getElementById('modalTitle').textContent = 'Create User';
            document.getElementById('userForm').reset();
            document.getElementById('userId').value = '';
            document.getElementById('username').disabled = false;
            document.getElementById('passwordGroup').style.display = 'block';
            document.getElementById('password').required = true;
            document.getElementById('submitBtn').textContent = 'Create';
            document.getElementById('userModal').classList.add('show');
        }
        
        function editUser(id) {
            const user = users.find(u => u.id === id);
            if (!user) return;
            
            document.getElementById('modalTitle').textContent = 'Edit User';
            document.getElementById('userId').value = user.id;
            document.getElementById('username').value = user.username;
            document.getElementById('username').disabled = true;
            document.getElementById('email').value = user.email || '';
            document.getElementById('role').value = user.role;
            document.getElementById('passwordGroup').style.display = 'none';
            document.getElementById('password').required = false;
            document.getElementById('submitBtn').textContent = 'Update';
            document.getElementById('userModal').classList.add('show');
        }
        
        function closeModal() {
            document.getElementById('userModal').classList.remove('show');
        }
        
        document.getElementById('userForm').addEventListener('submit', async (e) => {
            e.preventDefault();
            
            const formData = new FormData(e.target);
            const userId = formData.get('id');
            const isEdit = userId !== '';
            
            const payload = {
                action: isEdit ? 'update' : 'create',
                username: formData.get('username'),
                email: formData.get('email'),
                role: formData.get('role')
            };
            
            if (!isEdit) {
                payload.password = formData.get('password');
            } else {
                payload.id = parseInt(userId);
            }
            
            try {
                const response = await fetch('/api/system/users.php', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(payload)
                });
                
                const data = await response.json();
                
                if (data.success) {
                    showSuccess(data.message);
                    closeModal();
                    loadUsers();
                } else {
                    showError(data.error);
                }
            } catch (error) {
                showError('Request failed: ' + error.message);
            }
        });
        
        async function deleteUser(id, username) {
            if (!confirm(`Delete user "${username}"?\n\nThis action cannot be undone.`)) {
                return;
            }
            
            try {
                const response = await fetch('/api/system/users.php', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ action: 'delete', id: id })
                });
                
                const data = await response.json();
                
                if (data.success) {
                    showSuccess(data.message);
                    loadUsers();
                } else {
                    showError(data.error);
                }
            } catch (error) {
                showError('Request failed: ' + error.message);
            }
        }
        
        function showError(message) {
            const div = document.getElementById('message');
            div.className = 'error';
            div.textContent = message;
            setTimeout(() => div.textContent = '', 5000);
        }
        
        function showSuccess(message) {
            const div = document.getElementById('message');
            div.className = 'success';
            div.textContent = message;
            setTimeout(() => div.textContent = '', 5000);
        }
        
        function formatDate(dateStr) {
            return new Date(dateStr).toLocaleString();
        }
        
        function escapeHtml(text) {
            const div = document.createElement('div');
            div.textContent = text;
            return div.innerHTML;
        }
    </script>
</body>
</html>
