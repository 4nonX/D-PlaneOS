// D-PlaneOS v2.0.0 - Enhanced UI Library
(function() {
  'use strict';
  
  const EnhancedUI = {
    toastContainer: null,
    toasts: [],
    
    init() {
      if (!this.toastContainer) {
        this.toastContainer = document.createElement('div');
        this.toastContainer.className = 'toast-container';
        document.body.appendChild(this.toastContainer);
      }
    },
    
    toast(message, type = 'info', duration = 4000) {
      this.init();
      
      const icons = {
        success: '✓',
        error: '✕',
        warning: '⚠',
        info: 'ℹ'
      };
      
      const toast = document.createElement('div');
      toast.className = `toast toast-${type} fade-in`;
      toast.innerHTML = `
        <div class="toast-icon">${icons[type]}</div>
        <div class="toast-content">
          <div class="toast-message">${message}</div>
        </div>
        <button class="toast-close" onclick="EnhancedUI.removeToast(this.parentElement)">×</button>
      `;
      
      this.toastContainer.appendChild(toast);
      this.toasts.push(toast);
      
      if (duration > 0) {
        setTimeout(() => this.removeToast(toast), duration);
      }
      
      return toast;
    },
    
    removeToast(toast) {
      toast.classList.add('removing');
      setTimeout(() => {
        toast.remove();
        this.toasts = this.toasts.filter(t => t !== toast);
      }, 300);
    },
    
    showLoading(text = 'Loading...') {
      let overlay = document.getElementById('loadingOverlay');
      if (!overlay) {
        overlay = document.createElement('div');
        overlay.id = 'loadingOverlay';
        overlay.className = 'loading-overlay';
        overlay.innerHTML = `
          <div class="loading-content">
            <div class="spinner spinner-lg"></div>
            <div style="margin-top:16px;color:rgba(255,255,255,0.8);">${text}</div>
          </div>
        `;
        document.body.appendChild(overlay);
      }
      overlay.style.display = 'flex';
    },
    
    hideLoading() {
      const overlay = document.getElementById('loadingOverlay');
      if (overlay) overlay.style.display = 'none';
    },
    
    async confirm(title, message) {
      return new Promise((resolve) => {
        const backdrop = document.createElement('div');
        backdrop.className = 'modal-backdrop';
        backdrop.style.cssText = 'position:fixed;inset:0;background:rgba(0,0,0,0.7);backdrop-filter:blur(4px);display:flex;align-items:center;justify-content:center;z-index:9998;';
        
        const modal = document.createElement('div');
        modal.className = 'modal-content';
        modal.style.cssText = 'background:rgba(30,41,59,0.98);border-radius:16px;border:1px solid rgba(255,255,255,0.1);max-width:480px;width:90%;backdrop-filter:blur(10px);';
        modal.innerHTML = `
          <div style="padding:24px;border-bottom:1px solid rgba(255,255,255,0.1);">
            <h3 style="margin:0;font-size:18px;font-weight:600;">${title}</h3>
          </div>
          <div style="padding:24px;color:rgba(255,255,255,0.8);line-height:1.6;">${message}</div>
          <div style="padding:16px 24px;border-top:1px solid rgba(255,255,255,0.1);display:flex;gap:12px;justify-content:flex-end;">
            <button class="btn" data-action="cancel">Cancel</button>
            <button class="btn btn-primary" data-action="confirm">Confirm</button>
          </div>
        `;
        
        backdrop.appendChild(modal);
        document.body.appendChild(backdrop);
        
        backdrop.addEventListener('click', (e) => {
          if (e.target === backdrop) {
            backdrop.remove();
            resolve(false);
          }
        });
        
        modal.querySelectorAll('button').forEach(btn => {
          btn.addEventListener('click', () => {
            backdrop.remove();
            resolve(btn.dataset.action === 'confirm');
          });
        });
      });
    },
    
    emptyState(options) {
      const div = document.createElement('div');
      div.className = 'empty-state';
      div.innerHTML = `
        ${options.icon ? `<div class="empty-state-icon">${options.icon}</div>` : ''}
        <div class="empty-state-title">${options.title || 'No data'}</div>
        <div class="empty-state-message">${options.message || 'Nothing to show here'}</div>
        ${options.action ? `<button class="empty-state-action" onclick="${options.action.onclick}">${options.action.label}</button>` : ''}
      `;
      return div;
    },
    
    skeleton(count = 3) {
      const container = document.createElement('div');
      for (let i = 0; i < count; i++) {
        const sk = document.createElement('div');
        sk.className = 'skeleton skeleton-card';
        sk.style.marginBottom = '16px';
        container.appendChild(sk);
      }
      return container;
    },
    
    progress(value, max = 100) {
      const container = document.createElement('div');
      container.className = 'progress-container';
      const bar = document.createElement('div');
      bar.className = 'progress-bar';
      bar.style.width = `${(value / max) * 100}%`;
      container.appendChild(bar);
      return container;
    }
  };
  
  // Expose globally
  window.EnhancedUI = EnhancedUI;
  window.ui = EnhancedUI; // Alias for backwards compatibility
  
  // Init on load
  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', () => EnhancedUI.init());
  } else {
    EnhancedUI.init();
  }
  
  // Enhance csrfFetch
  const originalFetch = window.csrfFetch;
  if (originalFetch) {
    window.csrfFetch = async function(url, options = {}) {
      const showLoading = options.showLoading !== false;
      const showError = options.showError !== false;
      
      if (showLoading) EnhancedUI.showLoading(options.loadingText);
      
      try {
        const response = await originalFetch(url, options);
        
        if (showLoading) EnhancedUI.hideLoading();
        
        if (!response.ok && showError) {
          if (response.status === 403) {
            EnhancedUI.toast('Access denied', 'error');
          } else if (response.status >= 500) {
            EnhancedUI.toast('Server error', 'error');
          }
        }
        
        return response;
      } catch (error) {
        if (showLoading) EnhancedUI.hideLoading();
        if (showError) EnhancedUI.toast(error.message || 'Network error', 'error');
        throw error;
      }
    };
  }
  
  // Connection monitor
  let isOnline = navigator.onLine;
  let statusEl = null;
  
  function updateConnectionStatus(online) {
    if (online === isOnline) return;
    isOnline = online;
    
    if (!statusEl) {
      statusEl = document.createElement('div');
      statusEl.className = 'connection-status';
      document.body.appendChild(statusEl);
    }
    
    if (online) {
      statusEl.className = 'connection-status online';
      statusEl.innerHTML = '<span class="status-dot"></span> Connected';
      setTimeout(() => statusEl.style.display = 'none', 3000);
    } else {
      statusEl.className = 'connection-status';
      statusEl.innerHTML = '<span class="status-dot"></span> Connection lost';
      statusEl.style.display = 'flex';
    }
  }
  
  window.addEventListener('online', () => updateConnectionStatus(true));
  window.addEventListener('offline', () => updateConnectionStatus(false));
  
  // Keyboard shortcuts
  let keySequence = '';
  let keyTimer = null;
  
  document.addEventListener('keydown', (e) => {
    if (['INPUT', 'TEXTAREA', 'SELECT'].includes(e.target.tagName)) return;
    
    keySequence += e.key;
    clearTimeout(keyTimer);
    
    // Navigation shortcuts
    const shortcuts = {
      'gd': 'index.html',
      'gf': 'files.html',
      'gs': 'pools.html',
      'gc': 'docker.html',
      'gu': 'users.html'
    };
    
    if (shortcuts[keySequence]) {
      e.preventDefault();
      window.location.href = shortcuts[keySequence];
      keySequence = '';
    } else if (e.key === '?') {
      e.preventDefault();
      EnhancedUI.toast('Shortcuts: g+d (Dashboard), g+f (Files), g+s (Storage), g+c (Docker), g+u (Users)', 'info', 6000);
    } else if (e.key === 'Escape') {
      document.querySelectorAll('.modal-backdrop').forEach(m => m.remove());
    } else {
      keyTimer = setTimeout(() => keySequence = '', 1000);
    }
  });
  
  // Form validation helper
  window.validateForm = function(form) {
    const inputs = form.querySelectorAll('[required]');
    let isValid = true;
    
    inputs.forEach(input => {
      const group = input.closest('.form-group');
      if (!group) return;
      
      if (!input.value.trim()) {
        group.classList.add('has-error');
        isValid = false;
      } else {
        group.classList.remove('has-error');
        group.classList.add('has-success');
      }
    });
    
    if (!isValid) {
      EnhancedUI.toast('Please fill all required fields', 'error');
    }
    
    return isValid;
  };
  
})();
