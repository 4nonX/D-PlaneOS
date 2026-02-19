/**
 * D-PlaneOS Nav Flyout Enhancement
 * 
 * Adds hover-intent flyout behavior to the existing top-nav.
 * Also serves as the single source of truth for sub-nav link definitions,
 * dynamically injecting canonical sub-nav content on every page.
 *
 * Behavior:
 *   Desktop (pointer:fine) → hover opens sub-nav after 120ms intent delay
 *   Touch/Mobile           → existing click behavior (unchanged)
 *   Keyboard               → existing keyboard shortcuts (unchanged)
 *
 * No HTML changes required. No CSS changes required.
 * Drop-in enhancement for all pages.
 */

// ─── Canonical Sub-Nav Link Definitions ──────────────────────
// THE single source of truth. Order = display order.
var NAV_LINKS = {
  storage: [
    { page: 'pools',              href: 'pools.html',              icon: 'database',             label: 'ZFS Pools' },
    { page: 'pools-advanced',     href: 'pools.html#advanced',     icon: 'photo_camera',         label: 'Snapshots' },
    { page: 'snapshot-scheduler', href: 'snapshot-scheduler.html', icon: 'schedule',             label: 'Scheduler' },
    { page: 'shares',             href: 'shares.html',             icon: 'folder_shared',        label: 'Shares' },
    { page: 'replication',        href: 'replication.html',        icon: 'sync',                 label: 'Replication' },
    { page: 'files',              href: 'files.html',              icon: 'folder_open',          label: 'Explorer' },
    { page: 'quotas',             href: 'pools.html#quotas',       icon: 'data_usage',           label: 'Quotas' },
    { page: 'acl-manager',        href: 'acl-manager.html',        icon: 'admin_panel_settings', label: 'ACLs' },
    { page: 'zfs-encryption',     href: 'pools.html#encryption',   icon: 'enhanced_encryption',  label: 'Encryption' },
    { page: 'cloud-sync',         href: 'cloud-sync.html',         icon: 'cloud_sync',           label: 'Cloud Sync' }
  ],
  compute: [
    { page: 'docker',   href: 'docker.html',    icon: 'dns',       label: 'Docker Stacks' },
    { page: 'git-sync', href: 'git-sync.html',   icon: 'sync',      label: 'Git Sync' },
    { page: 'modules',  href: 'modules.html',    icon: 'extension', label: 'App Modules' }
  ],
  network: [
    { page: 'network',            href: 'network.html',             icon: 'lan',               label: 'Overview' },
    { page: 'network-interfaces', href: 'network.html#interfaces',  icon: 'settings_ethernet', label: 'Interfaces' },
    { page: 'network-routing',    href: 'network.html#routing',     icon: 'alt_route',         label: 'Routing' },
    { page: 'network-dns',        href: 'network.html#dns',         icon: 'dns',               label: 'DNS' }
  ],
  identity: [
    { page: 'users',             href: 'users.html',              icon: 'group',         label: 'Users' },
    { page: 'groups',            href: 'users.html#groups',       icon: 'groups',        label: 'Groups' },
    { page: 'rbac-management',   href: 'users.html#rbac',         icon: 'verified_user', label: 'RBAC' },
    { page: 'directory-service', href: 'directory-service.html',  icon: 'domain',        label: 'Directory' }
  ],
  security: [
    { page: 'security',     href: 'security.html',              icon: 'shield',                label: 'Overview' },
    { page: 'audit',        href: 'audit.html',                 icon: 'history',               label: 'Audit Log' },
    { page: 'firewall',     href: 'security.html#firewall',     icon: 'local_fire_department', label: 'Firewall' },
    { page: 'certificates', href: 'security.html#certificates', icon: 'verified',              label: 'Certificates' }
  ],
  system: [
    { page: 'settings',           href: 'settings.html',              icon: 'settings',               label: 'General' },
    { page: 'system-settings',    href: 'settings.html#config',       icon: 'tune',                   label: 'Configuration' },
    { page: 'reporting',          href: 'reporting.html',             icon: 'monitoring',             label: 'Metrics' },
    { page: 'system-monitoring',  href: 'reporting.html#monitoring',  icon: 'analytics',              label: 'Live Health' },
    { page: 'logs',               href: 'reporting.html#logs',        icon: 'description',            label: 'Logs' },
    { page: 'hardware',           href: 'reporting.html#hardware',    icon: 'memory',                 label: 'Hardware' },
    { page: 'ipmi',               href: 'ipmi.html',                  icon: 'developer_board',        label: 'IPMI' },
    { page: 'ups',                href: 'ups.html',                   icon: 'battery_charging_full',  label: 'UPS' },
    { page: 'power-management',   href: 'settings.html#power',        icon: 'power_settings_new',     label: 'Power' },
    { page: 'removable-media-ui', href: 'removable-media-ui.html',   icon: 'usb',                    label: 'Media' }
  ]
};

// ─── Derived page→section map ────────────────────────────────
// Maps both 'page.html' and 'page.html#hash' to their section/page info.
var PAGE_MAP = { 'index.html': { section: 'dashboard', page: null } };
Object.keys(NAV_LINKS).forEach(function(section) {
  NAV_LINKS[section].forEach(function(link) {
    PAGE_MAP[link.href] = { section: section, page: link.page };
    // Also map the base page (without hash) to this section
    var basePage = link.href.split('#')[0];
    if (!PAGE_MAP[basePage]) {
      PAGE_MAP[basePage] = { section: section, page: link.page };
    }
  });
});

// ─── Dynamic Sub-Nav Injection ───────────────────────────────
// Replaces whatever sub-nav HTML exists with the canonical links.
// Wrapped in ready-check because script loads in <head> before nav DOM.
function _doInjectSubNavs() {
  var currentPage = window.location.pathname.split('/').pop() || 'index.html';
  var currentHash = window.location.hash || '';
  var currentFull = currentPage + currentHash;

  // Try full match (page.html#hash) first, then base page
  var current = PAGE_MAP[currentFull] || PAGE_MAP[currentPage];

  Object.keys(NAV_LINKS).forEach(function(section) {
    var container = document.getElementById('sub-' + section);
    if (!container) return;

    var inner = container.querySelector('.nav-sub-inner');
    if (!inner) {
      inner = document.createElement('div');
      inner.className = 'nav-sub-inner';
      container.innerHTML = '';
      container.appendChild(inner);
    }

    inner.innerHTML = NAV_LINKS[section].map(function(link) {
      var isActive = (current && current.page === link.page) ? ' active' : '';
      return '<a href="' + link.href + '" class="sub-link' + isActive + '" data-page="' + link.page + '">' +
        '<span class="material-symbols-rounded">' + link.icon + '</span>' +
        link.label + '</a>';
    }).join('');
  });

  // Activate current section
  if (current && current.section !== 'dashboard') {
    var subNav = document.getElementById('sub-' + current.section);
    if (subNav) {
      subNav.classList.add('active');
      subNav.style.display = 'flex';
      document.body.classList.add('has-subnav');
    }
  }
}

// Run when DOM is ready (script may load in <head>)
if (document.readyState === 'loading') {
  document.addEventListener('DOMContentLoaded', _doInjectSubNavs);
} else {
  _doInjectSubNavs();
}

(function () {
  'use strict';

  // Only enhance on devices with a fine pointer (mouse/trackpad)
  // Touch devices keep the existing click behavior
  const isDesktop = window.matchMedia('(pointer: fine)').matches;
  if (!isDesktop) return;

  let intentTimer = null;   // Delay before opening (prevents accidental triggers)
  let leaveTimer = null;    // Delay before closing (lets user move to sub-nav)
  let activeSection = null; // Currently hovered section ID

  const INTENT_DELAY = 120; // ms before opening on hover
  const LEAVE_DELAY = 280;  // ms grace period when moving between nav and sub-nav

  /**
   * Open a specific sub-nav (reuses existing DOM/CSS — no new elements)
   */
  function openSubNav(section, button) {
    clearTimeout(leaveTimer);

    if (activeSection === section) return; // Already open
    activeSection = section;

    // Remove active from all nav links
    document.querySelectorAll('.nav-link').forEach(function (link) {
      link.classList.remove('active');
    });

    // Activate the hovered button
    if (button) {
      button.classList.add('active');
    }

    // Hide all sub-navs
    document.querySelectorAll('.nav-sub').forEach(function (sub) {
      sub.classList.remove('active');
    });

    // Show the target sub-nav
    var subNav = document.getElementById('sub-' + section);
    if (subNav) {
      subNav.classList.add('active');
      document.body.classList.add('has-subnav');
    } else {
      // No sub-nav (e.g. Dashboard) — remove subnav spacing
      document.body.classList.remove('has-subnav');
    }
  }

  /**
   * Close all sub-navs (with grace period)
   */
  function scheduleClose() {
    clearTimeout(leaveTimer);
    leaveTimer = setTimeout(function () {
      // Only close if not hovering over any nav element
      // Check BEFORE nulling activeSection to prevent flicker on rapid re-entry
      var hovered = document.querySelectorAll('.nav-container:hover, .nav-sub:hover, .nav-link:hover');
      if (hovered.length > 0) return;

      activeSection = null;

      document.querySelectorAll('.nav-sub').forEach(function (sub) {
        sub.classList.remove('active');
      });

      // Re-activate the section for the current page
      restoreCurrentPage();
    }, LEAVE_DELAY);
  }

  /**
   * Restore nav state to match the current page URL
   * (so leaving the nav doesn't leave it in a random state)
   */
  function restoreCurrentPage() {
    var currentPage = window.location.pathname.split('/').pop() || 'index.html';
    var currentFull = currentPage + (window.location.hash || '');

    // Use canonical PAGE_MAP (defined above in this file)
    var entry = PAGE_MAP[currentFull] || PAGE_MAP[currentPage];
    var section = entry ? entry.section : 'dashboard';

    // Restore active nav-link
    document.querySelectorAll('.nav-link').forEach(function (link) {
      link.classList.remove('active');
    });
    var sectionBtn = document.querySelector('[data-section="' + section + '"]');
    if (sectionBtn) {
      sectionBtn.classList.add('active');
    }

    // Restore sub-nav if the current page has one
    if (section !== 'dashboard') {
      var subNav = document.getElementById('sub-' + section);
      if (subNav) {
        subNav.classList.add('active');
        document.body.classList.add('has-subnav');
      }
    } else {
      document.body.classList.remove('has-subnav');
    }

    activeSection = section;
  }

  /**
   * Attach hover handlers to all nav-links with sub-navs
   */
  function init() {
    var navLinks = document.querySelectorAll('.nav-link[data-section]');

    navLinks.forEach(function (link) {
      var section = link.getAttribute('data-section');

      // Hover intent: open after short delay
      link.addEventListener('pointerenter', function () {
        clearTimeout(intentTimer);
        clearTimeout(leaveTimer);
        intentTimer = setTimeout(function () {
          openSubNav(section, link);
        }, INTENT_DELAY);
      });

      // Cancel intent if pointer leaves before delay expires
      link.addEventListener('pointerleave', function () {
        clearTimeout(intentTimer);
        scheduleClose();
      });
    });

    // Keep sub-nav open while hovering over it
    document.querySelectorAll('.nav-sub').forEach(function (sub) {
      sub.addEventListener('pointerenter', function () {
        clearTimeout(leaveTimer);
      });
      sub.addEventListener('pointerleave', function () {
        scheduleClose();
      });
    });

    // Also keep alive when hovering over the main nav bar
    var navMain = document.querySelector('.nav-main');
    if (navMain) {
      navMain.addEventListener('pointerleave', function () {
        scheduleClose();
      });
    }
  }

  // Initialize when DOM is ready
  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }
})();
