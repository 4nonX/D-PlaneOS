/**
 * D-PlaneOS v3.0.0 Core - API Bridge
 * Connects M3 Frontend with Backend
 */

(function() {
    'use strict';

    // Session auth uses HttpOnly cookies (dplaneos_session) set by the server.
    // SameSite=Strict provides CSRF protection — browser never sends cookies
    // on cross-origin requests. No manual token management needed.

    // Helper: read username from cookie (set by server, NOT HttpOnly)
    function getUsername() {
        const match = document.cookie.match(/(?:^|;\s*)dplaneos_user=([^;]*)/);
        return match ? decodeURIComponent(match[1]) : '';
    }

    // Enhanced fetch with loading states and error handling
    // Session cookie is sent automatically by the browser — no manual headers.
    window.csrfFetch = async function(url, options = {}) {
        const showLoading = options.showLoading !== false;
        const showError = options.showError !== false;
        
        if (showLoading && window.EnhancedUI) {
            EnhancedUI.showLoading(options.loadingText || 'Loading...');
        }
        
        options.headers = {
            ...options.headers
        };

        // Handle JSON bodies
        if (options.body && typeof options.body === 'object' && !(options.body instanceof FormData)) {
            options.headers['Content-Type'] = 'application/json';
            options.body = JSON.stringify(options.body);
        }

        try {
            const response = await fetch(url, options);
            
            if (showLoading && window.EnhancedUI) {
                EnhancedUI.hideLoading();
            }
            
            // Handle auth errors
            if (response.status === 401) {
                if (showError && window.EnhancedUI) {
                    EnhancedUI.toast('Session expired. Please login again.', 'error');
                }
                setTimeout(() => window.location.href = '/pages/login.html', 1000);
                throw new Error('Authentication required');
            }
            
            // Handle permission errors
            if (response.status === 403) {
                if (showError && window.EnhancedUI) {
                    EnhancedUI.toast('Access denied. Insufficient permissions.', 'error');
                }
                throw new Error('Access denied');
            }
            
            // Handle server errors
            if (response.status >= 500) {
                if (showError && window.EnhancedUI) {
                    EnhancedUI.toast('Server error. Please try again later.', 'error');
                }
                throw new Error('Server error');
            }

            return response;
        } catch (error) {
            if (showLoading && window.EnhancedUI) {
                EnhancedUI.hideLoading();
            }
            
            if (showError && window.EnhancedUI && !error.message.includes('Authentication')) {
                EnhancedUI.toast(error.message || 'Network error. Please check your connection.', 'error');
            }
            
            console.error('API Error:', error);
            throw error;
        }
    };

    // Global API wrapper for convenience
    window.DPlane = {
        async api(endpoint, options = {}) {
            if (!endpoint.startsWith('/')) {
                endpoint = '/api/' + endpoint;
            }
            const response = await csrfFetch(endpoint, options);
            return await response.json();
        },
        getUsername: getUsername
    };

    // Authentication check — cookie is sent automatically
    async function checkAuth() {
        const page = window.location.pathname;
        if (page.includes('login.html') || page.includes('setup-wizard.html') || page.includes('reset-password')) {
            return true;
        }

        try {
            const response = await fetch('/api/auth/check');
            const data = await response.json();
            
            if (!data.authenticated) {
                window.location.href = '/pages/login.html';
                return false;
            }
            return true;
        } catch (error) {
            console.error('Auth check failed:', error);
            window.location.href = '/pages/login.html';
            return false;
        }
    }

    // Sidebar management
    function initSidebar() {
        const sidebar = document.getElementById('sidebar');
        if (!sidebar) return;

        // Restore state
        const isCollapsed = localStorage.getItem('sidebar-collapsed') === 'true';
        if (isCollapsed) sidebar.classList.add('collapsed');

        // Toggle handler
        const toggle = document.getElementById('sidebar-toggle');
        if (toggle) {
            toggle.addEventListener('click', () => {
                sidebar.classList.toggle('collapsed');
                localStorage.setItem('sidebar-collapsed', sidebar.classList.contains('collapsed'));
            });
        }

        // Set active page
        const currentPage = window.location.pathname.split('/').pop() || 'index.html';
        sidebar.querySelectorAll('.nav-link').forEach(link => {
            const href = link.getAttribute('href');
            if (href === currentPage || href === './' + currentPage) {
                link.classList.add('active');
            }
        });

        // Logout handler
        const logoutBtn = document.getElementById('logout-btn');
        if (logoutBtn) {
            logoutBtn.addEventListener('click', async (e) => {
                e.preventDefault();
                try {
                    await csrfFetch('/api/auth/logout', { method: 'POST' });
                } catch (error) {
                    console.error('Logout error:', error);
                }
                window.location.href = '/pages/login.html';
            });
        }
    }

    // Global error handler
    window.handleError = function(error, context = '') {
        console.error(`Error${context ? ' in ' + context : ''}:`, error);
        if (window.showToast) {
            window.showToast(`Error: ${error.message}`, 'error');
        }
    };

    // Initialize on load
    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', async () => {
            if (await checkAuth()) {
                initSidebar();
            }
        });
    } else {
        (async () => {
            if (await checkAuth()) {
                initSidebar();
            }
        })();
    }
})();
