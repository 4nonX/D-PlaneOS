/**
 * D-PlaneOS Shared Navigation Logic (Companion)
 *
 * Provides navigateToSection() and toggleSubNav() called by onclick
 * handlers in page HTML. NAV_LINKS / PAGE_MAP are defined in nav-flyout.js
 * (the single source of truth).
 *
 * Also provides keyboard shortcuts and connection status monitoring.
 */

// Navigation behavior (called by onclick in nav HTML)
function navigateToSection(section) {
  if (section === 'dashboard') {
    window.location.href = 'index.html';
  }
}

function toggleSubNav(section, button) {
  document.querySelectorAll('.nav-link').forEach(function(link) {
    link.classList.remove('active');
  });
  if (button) {
    button.classList.add('active');
  }
  document.querySelectorAll('.nav-sub').forEach(function(sub) {
    sub.classList.remove('active');
    sub.style.display = '';
  });
  var subNav = document.getElementById('sub-' + section);
  if (subNav) {
    subNav.classList.add('active');
    subNav.style.display = 'flex';
    document.body.classList.add('has-subnav');
  }
}

// Keyboard shortcuts
function showKeyboardHelp() {
  if (window.EnhancedUI) {
    var shortcuts = 'Keyboard Shortcuts:\n\ng + d \u2192 Dashboard\ng + s \u2192 Storage\ng + c \u2192 Compute\ng + n \u2192 Network\ng + u \u2192 Identity\ng + e \u2192 Security\ng + y \u2192 System\n\nr \u2192 Refresh page\nEsc \u2192 Close modals\n? \u2192 Show this help';
    EnhancedUI.toast(shortcuts, 'info', 8000);
  }
}

(function() {
  if (window.__navSharedKeydownRegistered) return;
  window.__navSharedKeydownRegistered = true;
  var keySequence = '';
  var keyTimeout;
  document.addEventListener('keydown', function(e) {
    if (['INPUT', 'TEXTAREA', 'SELECT'].indexOf(e.target.tagName) >= 0) return;
    keySequence += e.key;
    clearTimeout(keyTimeout);
    var shortcuts = {
      'gd': 'index.html',
      'gs': function() { toggleSubNav('storage',  document.querySelector('[data-section="storage"]')); },
      'gc': function() { toggleSubNav('compute',  document.querySelector('[data-section="compute"]')); },
      'gn': function() { toggleSubNav('network',  document.querySelector('[data-section="network"]')); },
      'gu': function() { toggleSubNav('identity', document.querySelector('[data-section="identity"]')); },
      'ge': function() { toggleSubNav('security', document.querySelector('[data-section="security"]')); },
      'gy': function() { toggleSubNav('system',   document.querySelector('[data-section="system"]')); }
    };
    if (shortcuts[keySequence]) {
      e.preventDefault();
      var action = shortcuts[keySequence];
      if (typeof action === 'function') action();
      else window.location.href = action;
      keySequence = '';
    } else if (e.key === '?') {
      e.preventDefault();
      showKeyboardHelp();
    }
    keyTimeout = setTimeout(function() { keySequence = ''; }, 1000);
  });
})();

(function() {
  if (window.__navSharedCMRegistered) return;
  window.__navSharedCMRegistered = true;
  if (window.ConnectionMonitor) {
    window.ConnectionMonitor.on('statusChange', function(online) {
      var status = document.getElementById('navConnectionStatus');
      if (!status) return;
      if (online) {
        status.classList.remove('offline');
        status.classList.add('online');
        status.innerHTML = '<span class="status-dot"></span><span>Connected</span>';
      } else {
        status.classList.remove('online');
        status.classList.add('offline');
        status.innerHTML = '<span class="status-dot"></span><span>Offline</span>';
      }
    });
  }
})();
