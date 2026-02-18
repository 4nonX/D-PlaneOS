/**
 * D-PlaneOS Shared Navigation Logic
 * Single source of truth for nav behavior used across all pages.
 * Replaces duplicate inline nav blocks in network.html, pools.html, users.html, etc.
 */

// Navigation behavior
function navigateToSection(section) {
  if (section === 'dashboard') {
    window.location.href = 'index.html';
  }
}

function toggleSubNav(section, button) {
  document.querySelectorAll('.nav-link').forEach(link => {
    link.classList.remove('active');
  });
  if (button) {
    button.classList.add('active');
  }
  document.querySelectorAll('.nav-sub').forEach(sub => {
    sub.classList.remove('active');
  });
  const subNav = document.getElementById(`sub-${section}`);
  if (subNav) {
    subNav.classList.add('active');
    document.body.classList.add('has-subnav');
  }
}

// Auto-detect current section and page on load
(function() {
  const currentPage = window.location.pathname.split('/').pop() || 'index.html';
  const pageMap = {
    'acl-manager.html':           { section: 'storage',   page: 'acl-manager' },
    'audit.html':                 { section: 'security',  page: 'audit' },
    'certificates.html':          { section: 'security',  page: 'certificates' },
    'cloud-sync.html':            { section: 'storage',   page: 'cloud-sync' },
    'directory-service.html':     { section: 'identity',  page: 'directory-service' },
    'docker.html':                { section: 'compute',   page: 'docker' },
    'docker-containers-ui.html':  { section: 'compute',   page: 'docker' },
    'files-enhanced.html':        { section: 'storage',   page: 'files-enhanced' },
    'files.html':                 { section: 'storage',   page: 'files' },
    'firewall.html':              { section: 'security',  page: 'firewall' },
    'groups.html':                { section: 'identity',  page: 'groups' },
    'hardware.html':              { section: 'system',    page: 'hardware' },
    'index.html':                 { section: 'dashboard', page: null },
    'ipmi.html':                  { section: 'system',    page: 'ipmi' },
    'logs.html':                  { section: 'system',    page: 'logs' },
    'modules.html':               { section: 'compute',   page: 'modules' },
    'network-dns.html':           { section: 'network',   page: 'network-dns' },
    'network-interfaces.html':    { section: 'network',   page: 'network-interfaces' },
    'network-routing.html':       { section: 'network',   page: 'network-routing' },
    'network.html':               { section: 'network',   page: 'network' },
    'pools-advanced.html':        { section: 'storage',   page: 'pools-advanced' },
    'pools.html':                 { section: 'storage',   page: 'pools' },
    'power-management.html':      { section: 'system',    page: 'power-management' },
    'quotas.html':                { section: 'storage',   page: 'quotas' },
    'rbac-management.html':       { section: 'identity',  page: 'rbac-management' },
    'removable-media-ui.html':    { section: 'system',    page: 'removable-media-ui' },
    'replication.html':           { section: 'storage',   page: 'replication' },
    'reporting.html':             { section: 'system',    page: 'reporting' },
    'security.html':              { section: 'security',  page: 'security' },
    'settings.html':              { section: 'system',    page: 'settings' },
    'shares.html':                { section: 'storage',   page: 'shares' },
    'snapshot-scheduler.html':    { section: 'storage',   page: 'snapshot-scheduler' },
    'system-monitoring.html':     { section: 'system',    page: 'system-monitoring' },
    'system-settings.html':       { section: 'system',    page: 'system-settings' },
    'ups.html':                   { section: 'system',    page: 'ups' },
    'users.html':                 { section: 'identity',  page: 'users' },
    'zfs-encryption.html':        { section: 'storage',   page: 'zfs-encryption' }
  };
  const current = pageMap[currentPage];
  if (current) {
    const sectionBtn = document.querySelector(`[data-section="${current.section}"]`);
    if (sectionBtn) sectionBtn.classList.add('active');
    if (current.section !== 'dashboard') {
      const subNav = document.getElementById(`sub-${current.section}`);
      if (subNav) {
        subNav.classList.add('active');
        document.body.classList.add('has-subnav');
        if (current.page) {
          const pageLink = subNav.querySelector(`[data-page="${current.page}"]`);
          if (pageLink) pageLink.classList.add('active');
        }
      }
    }
  }
})();

// Keyboard shortcuts
function showKeyboardHelp() {
  if (window.EnhancedUI) {
    const shortcuts = `Keyboard Shortcuts:\n\ng + d → Dashboard\ng + s → Storage\ng + c → Compute\ng + n → Network\ng + u → Identity\ng + e → Security\ng + y → System\n\nr → Refresh page\nEsc → Close modals\n? → Show this help`;
    EnhancedUI.toast(shortcuts, 'info', 8000);
  }
}

// Enhanced keyboard shortcuts (registered once)
(function() {
  if (window.__navSharedKeydownRegistered) return;
  window.__navSharedKeydownRegistered = true;
  let keySequence = '';
  let keyTimeout;
  document.addEventListener('keydown', (e) => {
    if (['INPUT', 'TEXTAREA', 'SELECT'].includes(e.target.tagName)) return;
    keySequence += e.key;
    clearTimeout(keyTimeout);
    const shortcuts = {
      'gd': 'index.html',
      'gs': () => toggleSubNav('storage',   document.querySelector('[data-section="storage"]')),
      'gc': () => toggleSubNav('compute',   document.querySelector('[data-section="compute"]')),
      'gn': () => toggleSubNav('network',   document.querySelector('[data-section="network"]')),
      'gu': () => toggleSubNav('identity',  document.querySelector('[data-section="identity"]')),
      'ge': () => toggleSubNav('security',  document.querySelector('[data-section="security"]')),
      'gy': () => toggleSubNav('system',    document.querySelector('[data-section="system"]'))
    };
    if (shortcuts[keySequence]) {
      e.preventDefault();
      const action = shortcuts[keySequence];
      if (typeof action === 'function') action();
      else window.location.href = action;
      keySequence = '';
    } else if (e.key === '?') {
      e.preventDefault();
      showKeyboardHelp();
    }
    keyTimeout = setTimeout(() => keySequence = '', 1000);
  });
})();

// Connection status monitoring (registered once)
(function() {
  if (window.__navSharedCMRegistered) return;
  window.__navSharedCMRegistered = true;
  if (window.ConnectionMonitor) {
    window.ConnectionMonitor.on('statusChange', (online) => {
      const status = document.getElementById('navConnectionStatus');
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
