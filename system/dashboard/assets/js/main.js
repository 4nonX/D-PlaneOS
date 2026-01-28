// D-PlaneOS Dashboard - Real API Integration

// ============================================
// CSRF TOKEN MANAGEMENT
// ============================================

let csrfToken = null;

// Get CSRF token on page load
async function initCSRF() {
    try {
        const res = await window.originalFetch('/api/csrf-token.php');
        const data = await res.json();
        if (data.success) {
            csrfToken = data.token;
        }
    } catch (e) {
        console.error('Failed to get CSRF token:', e);
    }
}

// Override native fetch() to auto-inject CSRF tokens
(function() {
    window.originalFetch = window.fetch;
    window.fetch = function(url, options = {}) {
        options.headers = options.headers || {};
        
        // Add CSRF token for state-changing requests
        const method = (options.method || 'GET').toUpperCase();
        if (['POST', 'PUT', 'DELETE'].includes(method)) {
            if (csrfToken) {
                options.headers['X-CSRF-Token'] = csrfToken;
            }
        }
        
        // Call original fetch
        return window.originalFetch(url, options).then(async response => {
            // Refresh token if expired
            if (response.status === 403) {
                const clone = response.clone();
                try {
                    const data = await clone.json();
                    if (data.error && data.error.includes('CSRF')) {
                        await initCSRF();
                        // Retry with new token
                        if (csrfToken) {
                            options.headers['X-CSRF-Token'] = csrfToken;
                            return window.originalFetch(url, options);
                        }
                    }
                } catch (e) {
                    // Not JSON, return original response
                }
            }
            return response;
        });
    };
})();

// Initialize on load
document.addEventListener('DOMContentLoaded', initCSRF);

// ============================================
// PAGE NAVIGATION
// ============================================

// Page navigation
document.querySelectorAll('.nav-btn').forEach(btn => {
    btn.addEventListener('click', () => {
        const page = btn.dataset.page;
        showPage(page);
    });
});

function showPage(page) {
    document.querySelectorAll('.page').forEach(p => p.classList.remove('active'));
    document.querySelectorAll('.nav-btn').forEach(b => b.classList.remove('active'));
    
    document.getElementById(`page-${page}`).classList.add('active');
    document.querySelector(`[data-page="${page}"]`).classList.add('active');
    
    // Load page data
    switch(page) {
        case 'dashboard':
            loadDashboard();
            break;
        case 'storage':
            loadPools();
            break;
        case 'datasets':
            loadDatasets();
            break;
        case 'files':
            loadFiles();
            break;
        case 'shares':
            loadShares();
            break;
        case 'disks':
            loadDiskHealth();
            break;
        case 'snapshots':
            loadSnapshots();
            break;
        case 'ups':
            loadUPS();
            break;
        case 'users':
            loadUsers();
            break;
        case 'logs':
            loadLogs();
            break;
        case 'containers':
            loadContainers();
            break;
        case 'services':
            loadServices();
            break;
        case 'monitoring':
            loadMonitoring();
            break;
        case 'encryption':
            loadEncryptedDatasets();
            break;
        case 'rclone':
            loadRclone();
            break;
        case 'analytics':
            loadAnalytics();
            break;
        case 'replication':
            loadReplication();
            break;
        case 'alerts':
            loadAlertHistory();
            break;
    }
}

// Dashboard - System Stats
async function loadDashboard() {
    try {
        const res = await fetch('/api/system/stats.php');
        const data = await res.json();
        
        if (data.success) {
            const stats = data.stats;
            document.getElementById('cpu-usage').textContent = `${stats.cpu_percent}%`;
            document.getElementById('memory-usage').textContent = `${stats.memory_percent}% (${stats.memory_used_human})`;
            document.getElementById('uptime').textContent = stats.uptime;
            document.getElementById('load-avg').textContent = `${stats.load_1m} / ${stats.load_5m} / ${stats.load_15m}`;
        }
    } catch (e) {
        console.error('Failed to load stats:', e);
    }
}

// Storage - Pools
async function loadPools() {
    const container = document.getElementById('pools-list');
    container.innerHTML = '<div class="loading">Loading pools...</div>';
    
    try {
        const res = await fetch('/api/storage/pools.php?action=list');
        const data = await res.json();
        
        if (data.success) {
            if (data.pools.length === 0) {
                container.innerHTML = '<p>No pools found. Create your first pool!</p>';
                return;
            }
            
            let html = '';
            
            for (const pool of data.pools) {
                // Get topology for this pool
                const topoRes = await fetch(`/api/storage/pools.php?action=topology&name=${pool.name}`);
                const topoData = await topoRes.json();
                const topology = topoData.success ? topoData.topology : { vdevs: [] };
                
                // Pool card with visual capacity
                html += `
                    <div class="pool-card" style="--usage-percent: ${pool.usage_percent}%;">
                        <div class="pool-header">
                            <div class="pool-title">
                                <div class="pool-name">
                                    <h3>${pool.name}</h3>
                                    <span class="badge badge-${pool.health === 'ONLINE' ? 'success' : 'warning'}">${pool.health}</span>
                                </div>
                                <div class="button-group">
                                    <button class="btn-secondary btn-sm" onclick="scrubPool('${pool.name}')">Scrub</button>
                                    <button class="btn-sm" onclick="expandPool('${pool.name}')">Expand</button>
                                    <button class="btn-danger btn-sm" onclick="confirmPoolDestroy('${pool.name}')">Destroy</button>
                                </div>
                            </div>
                            
                            <p style="margin: 0.75rem 0; color: #aaa;">
                                <strong>Capacity:</strong> ${pool.size} | 
                                <strong>Used:</strong> ${pool.alloc} (${pool.usage_percent}%)
                            </p>
                            
                            <div class="capacity-bar">
                                <div class="capacity-fill" style="width: ${pool.usage_percent}%;">
                                    <span class="capacity-text">${pool.usage_percent}% Used (${pool.alloc} / ${pool.size})</span>
                                </div>
                            </div>
                        </div>
                        
                        <div class="pool-topology">
                `;
                
                // Render vdevs
                if (topology.vdevs && topology.vdevs.length > 0) {
                    topology.vdevs.forEach(vdev => {
                        const vdevIcon = vdev.type.includes('mirror') ? 
                            '<svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor"><rect x="3" y="8" width="8" height="8" rx="1"/><rect x="13" y="8" width="8" height="8" rx="1"/></svg>' :
                            '<svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor"><rect x="3" y="5" width="18" height="4" rx="1"/><rect x="3" y="10" width="18" height="4" rx="1"/><rect x="3" y="15" width="18" height="4" rx="1"/></svg>';
                        
                        html += `
                            <div class="vdev">
                                <div class="vdev-header">
                                    ${vdevIcon}
                                    ${vdev.type.toUpperCase()} VDEV
                                </div>
                                <div class="disk-list">
                        `;
                        
                        vdev.disks.forEach(disk => {
                            html += `
                                <div class="disk-item">
                                    <div class="disk-name">💾 ${disk.device}</div>
                                    <div class="disk-state"><span class="badge badge-${disk.state === 'ONLINE' ? 'success' : 'warning'}">${disk.state}</span></div>
                                </div>
                            `;
                        });
                        
                        html += `
                                </div>
                            </div>
                        `;
                    });
                } else {
                    html += '<p style="color: #aaa; padding: 1rem;">No topology data available</p>';
                }
                
                html += `
                        </div>
                    </div>
                `;
            }
            
            container.innerHTML = html;
        } else {
            container.innerHTML = `<div class="error">Error: ${data.error}</div>`;
        }
    } catch (e) {
        container.innerHTML = `<div class="error">Failed to load pools: ${e.message}</div>`;
    }
}

function showCreatePool() {
    const modal = document.getElementById('modal');
    const body = document.getElementById('modal-body');
    
    body.innerHTML = `
        <h3>Create Pool</h3>
        <form id="create-pool-form">
            <div class="form-group">
                <label>Pool Name</label>
                <input type="text" name="name" required pattern="[a-zA-Z0-9_-]+" placeholder="tank">
            </div>
            <div class="form-group">
                <label>RAID Type</label>
                <select name="raid" required>
                    <option value="stripe">Stripe (RAID0)</option>
                    <option value="mirror">Mirror (RAID1)</option>
                    <option value="raidz1">RAIDZ1</option>
                    <option value="raidz2">RAIDZ2</option>
                    <option value="raidz3">RAIDZ3</option>
                </select>
            </div>
            <div class="form-group">
                <label>Disks (comma-separated paths)</label>
                <input type="text" name="disks" required placeholder="/dev/sdb,/dev/sdc">
                <small>Example: /dev/sdb,/dev/sdc,/dev/sdd</small>
            </div>
            <button type="submit" class="btn-primary">Create Pool</button>
        </form>
    `;
    
    modal.classList.add('active');
    
    document.getElementById('create-pool-form').addEventListener('submit', async (e) => {
        e.preventDefault();
        const formData = new FormData(e.target);
        const disks = formData.get('disks').split(',').map(d => d.trim());
        
        try {
            const res = await fetch('/api/storage/pools.php', {
                method: 'POST',
                headers: {'Content-Type': 'application/json'},
                body: JSON.stringify({
                    action: 'create',
                    name: formData.get('name'),
                    raid: formData.get('raid'),
                    disks: disks,
                    force: true
                })
            });
            
            const data = await res.json();
            
            if (data.success) {
                closeModal();
                loadPools();
                alert('Pool created successfully!');
            } else {
                alert('Error: ' + data.error);
            }
        } catch (e) {
            alert('Failed to create pool: ' + e.message);
        }
    });
}

async function scrubPool(name) {
    if (!confirm(`Start scrub on pool "${name}"?`)) return;
    
    try {
        const res = await fetch('/api/storage/pools.php', {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({action: 'scrub', name: name, operation: 'start'})
        });
        
        const data = await res.json();
        if (data.success) {
            alert('Scrub started successfully!');
        } else {
            alert('Error: ' + data.error);
        }
    } catch (e) {
        alert('Failed to start scrub: ' + e.message);
    }
}

function scheduleScrub(name) {
    const modal = document.getElementById('modal');
    const body = document.getElementById('modal-body');
    
    body.innerHTML = `
        <h3>Schedule Scrub: ${name}</h3>
        <form id="schedule-scrub-form">
            <div class="form-group">
                <label>Schedule Type</label>
                <select name="schedule_type" required>
                    <option value="monthly">Monthly</option>
                    <option value="weekly">Weekly</option>
                </select>
            </div>
            <div class="form-group">
                <label>Day (1-31 for monthly, 0-6 for weekly)</label>
                <input type="number" name="day" value="1" min="0" max="31" required>
            </div>
            <button type="submit" class="btn-primary">Schedule</button>
        </form>
    `;
    
    modal.classList.add('active');
    
    document.getElementById('schedule-scrub-form').addEventListener('submit', async (e) => {
        e.preventDefault();
        const formData = new FormData(e.target);
        
        try {
            const res = await fetch('/api/storage/pools.php', {
                method: 'POST',
                headers: {'Content-Type': 'application/json'},
                body: JSON.stringify({
                    action: 'schedule_scrub',
                    name: name,
                    schedule_type: formData.get('schedule_type'),
                    day_of_month: parseInt(formData.get('day'))
                })
            });
            
            const data = await res.json();
            if (data.success) {
                closeModal();
                alert('Scrub scheduled successfully!');
            } else {
                alert('Error: ' + data.error);
            }
        } catch (e) {
            alert('Failed to schedule scrub: ' + e.message);
        }
    });
}

async function destroyPool(name) {
    if (!confirm(`DESTROY pool "${name}"? ALL DATA WILL BE LOST!\n\nType the pool name to confirm:`)) return;
    const confirmName = prompt(`Type "${name}" to confirm:`);
    if (confirmName !== name) {
        alert('Pool name does not match. Cancelling.');
        return;
    }
    
    try {
        const res = await fetch('/api/storage/pools.php', {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({action: 'destroy', name: name})
        });
        
        const data = await res.json();
        if (data.success) {
            loadPools();
            alert('Pool destroyed.');
        } else {
            alert('Error: ' + data.error);
        }
    } catch (e) {
        alert('Failed to destroy pool: ' + e.message);
    }
}

// Datasets
async function loadDatasets() {
    const container = document.getElementById('datasets-list');
    container.innerHTML = '<div class="loading">Loading datasets...</div>';
    
    try {
        const res = await fetch('/api/storage/datasets.php');
        const data = await res.json();
        
        if (data.success) {
            if (data.datasets.length === 0) {
                container.innerHTML = '<p>No datasets found.</p>';
                return;
            }
            
            let html = '<table><thead><tr><th>Name</th><th>Used</th><th>Available</th><th>Referenced</th><th>Mountpoint</th><th>Actions</th></tr></thead><tbody>';
            
            data.datasets.forEach(ds => {
                html += `
                    <tr>
                        <td style="padding-left: ${ds.depth * 20}px">${ds.name}</td>
                        <td>${ds.used}</td>
                        <td>${ds.avail}</td>
                        <td>${ds.refer}</td>
                        <td>${ds.mountpoint}</td>
                        <td>
                            <button class="btn-secondary" onclick="snapshotDataset('${ds.name}')">Snapshot</button>
                            <button class="btn-danger" onclick="destroyDataset('${ds.name}')">Destroy</button>
                        </td>
                    </tr>
                `;
            });
            
            html += '</tbody></table>';
            container.innerHTML = html;
        } else {
            container.innerHTML = `<div class="error">Error: ${data.error}</div>`;
        }
    } catch (e) {
        container.innerHTML = `<div class="error">Failed to load datasets: ${e.message}</div>`;
    }
}

function showCreateDataset() {
    const modal = document.getElementById('modal');
    const body = document.getElementById('modal-body');
    
    body.innerHTML = `
        <h3>Create Dataset</h3>
        <form id="create-dataset-form">
            <div class="form-group">
                <label>Dataset Name</label>
                <input type="text" name="name" required placeholder="tank/data">
            </div>
            <button type="submit" class="btn-primary">Create Dataset</button>
        </form>
    `;
    
    modal.classList.add('active');
    
    document.getElementById('create-dataset-form').addEventListener('submit', async (e) => {
        e.preventDefault();
        const formData = new FormData(e.target);
        
        try {
            const res = await fetch('/api/storage/datasets.php', {
                method: 'POST',
                headers: {'Content-Type': 'application/json'},
                body: JSON.stringify({
                    action: 'create',
                    name: formData.get('name'),
                    type: 'filesystem'
                })
            });
            
            const data = await res.json();
            if (data.success) {
                closeModal();
                loadDatasets();
                alert('Dataset created successfully!');
            } else {
                alert('Error: ' + data.error);
            }
        } catch (e) {
            alert('Failed to create dataset: ' + e.message);
        }
    });
}

function showBulkSnapshot() {
    const modal = document.getElementById('modal');
    const body = document.getElementById('modal-body');
    
    body.innerHTML = `
        <h3>Bulk Snapshot</h3>
        <form id="bulk-snapshot-form">
            <div class="form-group">
                <label>Parent Dataset</label>
                <input type="text" name="dataset" required placeholder="tank">
            </div>
            <div class="form-group">
                <label>Snapshot Name</label>
                <input type="text" name="snapname" required placeholder="backup-$(date +%Y%m%d)">
            </div>
            <div class="form-group">
                <label>
                    <input type="checkbox" name="recursive" checked> Recursive (include all children)
                </label>
            </div>
            <button type="submit" class="btn-primary">Create Snapshots</button>
        </form>
    `;
    
    modal.classList.add('active');
    
    document.getElementById('bulk-snapshot-form').addEventListener('submit', async (e) => {
        e.preventDefault();
        const formData = new FormData(e.target);
        
        try {
            const res = await fetch('/api/storage/datasets.php', {
                method: 'POST',
                headers: {'Content-Type': 'application/json'},
                body: JSON.stringify({
                    action: 'bulk_snapshot',
                    dataset: formData.get('dataset'),
                    snapname: formData.get('snapname'),
                    recursive: formData.get('recursive') === 'on'
                })
            });
            
            const data = await res.json();
            if (data.success) {
                closeModal();
                alert('Bulk snapshots created successfully!');
            } else {
                alert('Error: ' + data.error);
            }
        } catch (e) {
            alert('Failed to create bulk snapshots: ' + e.message);
        }
    });
}

async function snapshotDataset(name) {
    const snapname = prompt(`Enter snapshot name for "${name}":`);
    if (!snapname) return;
    
    try {
        const res = await fetch('/api/storage/datasets.php', {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({action: 'snapshot', dataset: name, snapname: snapname})
        });
        
        const data = await res.json();
        if (data.success) {
            alert('Snapshot created!');
        } else {
            alert('Error: ' + data.error);
        }
    } catch (e) {
        alert('Failed to create snapshot: ' + e.message);
    }
}

async function destroyDataset(name) {
    if (!confirm(`DESTROY dataset "${name}"? ALL DATA WILL BE LOST!`)) return;
    
    try {
        const res = await fetch('/api/storage/datasets.php', {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({action: 'destroy', name: name})
        });
        
        const data = await res.json();
        if (data.success) {
            loadDatasets();
            alert('Dataset destroyed.');
        } else {
            alert('Error: ' + data.error);
        }
    } catch (e) {
        alert('Failed to destroy dataset: ' + e.message);
    }
}

// Containers
async function loadContainers() {
    const container = document.getElementById('containers-list');
    container.innerHTML = '<div class="loading">Loading containers...</div>';
    
    try {
        const res = await fetch('/api/containers/containers.php');
        const data = await res.json();
        
        if (data.success) {
            if (data.containers.length === 0) {
                container.innerHTML = '<p>No containers found.</p>';
                return;
            }
            
            let html = '<table><thead><tr><th>Name</th><th>Image</th><th>Status</th><th>Ports</th><th>Actions</th></tr></thead><tbody>';
            
            data.containers.forEach(c => {
                const actionBtn = c.running 
                    ? `<button class="btn-secondary" onclick="stopContainer('${c.name}')">Stop</button>`
                    : `<button class="btn-primary" onclick="startContainer('${c.name}')">Start</button>`;
                
                html += `
                    <tr>
                        <td><strong>${c.name}</strong></td>
                        <td>${c.image}</td>
                        <td>${c.status}</td>
                        <td>${c.ports.join(', ') || 'None'}</td>
                        <td>
                            ${actionBtn}
                            <button class="btn-secondary" onclick="restartContainer('${c.name}')">Restart</button>
                            <button class="btn-danger" onclick="removeContainer('${c.name}')">Remove</button>
                        </td>
                    </tr>
                `;
            });
            
            html += '</tbody></table>';
            container.innerHTML = html;
        } else {
            container.innerHTML = `<div class="error">Error: ${data.error}</div>`;
        }
    } catch (e) {
        container.innerHTML = `<div class="error">Failed to load containers: ${e.message}</div>`;
    }
}

function showDeployContainer() {
    const modal = document.getElementById('modal');
    const body = document.getElementById('modal-body');
    
    body.innerHTML = `
        <h3>Deploy Container</h3>
        <form id="deploy-form">
            <div class="form-group">
                <label>Compose Name</label>
                <input type="text" name="name" required placeholder="myapp">
            </div>
            <div class="form-group">
                <label>Docker Compose YAML</label>
                <textarea name="yaml" rows="10" required placeholder="version: '3'
services:
  app:
    image: nginx
    ports:
      - 8080:80"></textarea>
            </div>
            <button type="submit" class="btn-primary">Deploy</button>
        </form>
    `;
    
    modal.classList.add('active');
    
    document.getElementById('deploy-form').addEventListener('submit', async (e) => {
        e.preventDefault();
        const formData = new FormData(e.target);
        
        try {
            const res = await fetch('/api/containers/containers.php', {
                method: 'POST',
                headers: {'Content-Type': 'application/json'},
                body: JSON.stringify({
                    action: 'deploy',
                    name: formData.get('name'),
                    yaml: formData.get('yaml')
                })
            });
            
            const data = await res.json();
            if (data.success) {
                closeModal();
                loadContainers();
                alert('Container deployed!');
            } else {
                alert('Error: ' + data.error);
            }
        } catch (e) {
            alert('Failed to deploy: ' + e.message);
        }
    });
}

async function startContainer(name) {
    try {
        const res = await fetch('/api/containers/containers.php', {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({action: 'start', name: name})
        });
        
        const data = await res.json();
        if (data.success) {
            loadContainers();
        } else {
            alert('Error: ' + data.error);
        }
    } catch (e) {
        alert('Failed: ' + e.message);
    }
}

async function stopContainer(name) {
    try {
        const res = await fetch('/api/containers/containers.php', {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({action: 'stop', name: name})
        });
        
        const data = await res.json();
        if (data.success) {
            loadContainers();
        } else {
            alert('Error: ' + data.error);
        }
    } catch (e) {
        alert('Failed: ' + e.message);
    }
}

async function restartContainer(name) {
    try {
        const res = await fetch('/api/containers/containers.php', {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({action: 'restart', name: name})
        });
        
        const data = await res.json();
        if (data.success) {
            loadContainers();
        } else {
            alert('Error: ' + data.error);
        }
    } catch (e) {
        alert('Failed: ' + e.message);
    }
}

async function removeContainer(name) {
    if (!confirm(`Remove container "${name}"?`)) return;
    
    try {
        const res = await fetch('/api/containers/containers.php', {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({action: 'remove', name: name})
        });
        
        const data = await res.json();
        if (data.success) {
            loadContainers();
        } else {
            alert('Error: ' + data.error);
        }
    } catch (e) {
        alert('Failed: ' + e.message);
    }
}

// Modal
function closeModal() {
    document.getElementById('modal').classList.remove('active');
}

// Analytics functions
async function loadAnalytics() {
    const hours = parseInt(document.getElementById('time-range').value);
    
    // Load all metrics
    await Promise.all([
        loadMetricChart('cpu_usage', 'chart-cpu', hours, '%'),
        loadMetricChart('memory_usage', 'chart-memory', hours, '%'),
        loadPoolUsageChart(hours),
        loadDiskTempChart(hours)
    ]);
}

async function loadMetricChart(metricType, canvasId, hours, unit) {
    const canvas = document.getElementById(canvasId);
    if (!canvas) return;
    
    // Show skeleton loader
    canvas.style.display = 'none';
    const parent = canvas.parentElement;
    const skeleton = document.createElement('div');
    skeleton.className = 'chart-loading';
    skeleton.id = `${canvasId}-skeleton`;
    parent.appendChild(skeleton);
    
    try {
        const res = await fetch(`/api/system/metrics.php?metric_type=${metricType}&hours=${hours}`);
        const data = await res.json();
        
        // Remove skeleton
        skeleton.remove();
        canvas.style.display = 'block';
        
        if (data.success && data.metrics.length > 0) {
            drawChart(canvasId, data.metrics, unit);
        } else {
            drawNoData(canvasId);
        }
    } catch (e) {
        console.error('Failed to load metric:', e);
        skeleton.remove();
        canvas.style.display = 'block';
        drawNoData(canvasId);
    }
}

async function loadPoolUsageChart(hours) {
    try {
        const res = await fetch(`/api/system/metrics.php?metric_type=pool_usage&hours=${hours}`);
        const data = await res.json();
        
        if (data.success && data.metrics.length > 0) {
            // Group by resource name
            const pools = {};
            data.metrics.forEach(m => {
                if (!pools[m.resource_name]) pools[m.resource_name] = [];
                pools[m.resource_name].push(m);
            });
            
            // Draw first pool's data (or combine multiple pools)
            const firstPool = Object.values(pools)[0];
            if (firstPool) {
                drawChart('chart-pool', firstPool, '%');
            }
        } else {
            drawNoData('chart-pool');
        }
    } catch (e) {
        console.error('Failed to load pool metrics:', e);
        drawNoData('chart-pool');
    }
}

async function loadDiskTempChart(hours) {
    try {
        const res = await fetch(`/api/system/metrics.php?metric_type=disk_temperature&hours=${hours}`);
        const data = await res.json();
        
        if (data.success && data.metrics.length > 0) {
            // Group by disk
            const disks = {};
            data.metrics.forEach(m => {
                if (!disks[m.resource_name]) disks[m.resource_name] = [];
                disks[m.resource_name].push(m);
            });
            
            // Draw first disk's data
            const firstDisk = Object.values(disks)[0];
            if (firstDisk) {
                drawChart('chart-temp', firstDisk, '°C');
            }
        } else {
            drawNoData('chart-temp');
        }
    } catch (e) {
        console.error('Failed to load temp metrics:', e);
        drawNoData('chart-temp');
    }
}

function drawChart(canvasId, metrics, unit) {
    const canvas = document.getElementById(canvasId);
    if (!canvas) return;
    
    const ctx = canvas.getContext('2d');
    const width = canvas.width;
    const height = canvas.height;
    
    // Clear canvas
    ctx.clearRect(0, 0, width, height);
    
    if (metrics.length === 0) {
        drawNoData(canvasId);
        return;
    }
    
    // Get min/max values
    const values = metrics.map(m => m.value);
    const maxValue = Math.max(...values);
    const minValue = Math.min(...values);
    const range = maxValue - minValue || 1;
    
    // Padding
    const padding = 40;
    const graphWidth = width - padding * 2;
    const graphHeight = height - padding * 2;
    
    // Draw grid
    ctx.strokeStyle = 'rgba(255, 255, 255, 0.1)';
    ctx.lineWidth = 1;
    for (let i = 0; i <= 4; i++) {
        const y = padding + (graphHeight / 4) * i;
        ctx.beginPath();
        ctx.moveTo(padding, y);
        ctx.lineTo(width - padding, y);
        ctx.stroke();
    }
    
    // Draw axes labels
    ctx.fillStyle = '#aaa';
    ctx.font = '12px sans-serif';
    ctx.textAlign = 'right';
    for (let i = 0; i <= 4; i++) {
        const value = maxValue - (range / 4) * i;
        const y = padding + (graphHeight / 4) * i + 4;
        ctx.fillText(value.toFixed(1) + unit, padding - 5, y);
    }
    
    // Draw line
    ctx.strokeStyle = '#667eea';
    ctx.lineWidth = 2;
    ctx.beginPath();
    
    metrics.forEach((m, i) => {
        const x = padding + (graphWidth / (metrics.length - 1)) * i;
        const y = padding + graphHeight - ((m.value - minValue) / range) * graphHeight;
        
        if (i === 0) {
            ctx.moveTo(x, y);
        } else {
            ctx.lineTo(x, y);
        }
    });
    
    ctx.stroke();
    
    // Draw points
    ctx.fillStyle = '#667eea';
    metrics.forEach((m, i) => {
        const x = padding + (graphWidth / (metrics.length - 1)) * i;
        const y = padding + graphHeight - ((m.value - minValue) / range) * graphHeight;
        
        ctx.beginPath();
        ctx.arc(x, y, 3, 0, Math.PI * 2);
        ctx.fill();
    });
    
    // Draw gradient fill
    const gradient = ctx.createLinearGradient(0, padding, 0, height - padding);
    gradient.addColorStop(0, 'rgba(102, 126, 234, 0.3)');
    gradient.addColorStop(1, 'rgba(102, 126, 234, 0)');
    
    ctx.fillStyle = gradient;
    ctx.beginPath();
    ctx.moveTo(padding, height - padding);
    
    metrics.forEach((m, i) => {
        const x = padding + (graphWidth / (metrics.length - 1)) * i;
        const y = padding + graphHeight - ((m.value - minValue) / range) * graphHeight;
        ctx.lineTo(x, y);
    });
    
    ctx.lineTo(width - padding, height - padding);
    ctx.closePath();
    ctx.fill();
}

function drawNoData(canvasId) {
    const canvas = document.getElementById(canvasId);
    if (!canvas) return;
    
    const ctx = canvas.getContext('2d');
    const width = canvas.width;
    const height = canvas.height;
    
    ctx.clearRect(0, 0, width, height);
    
    ctx.fillStyle = '#aaa';
    ctx.font = '16px sans-serif';
    ctx.textAlign = 'center';
    ctx.textBaseline = 'middle';
    ctx.fillText('No data available', width / 2, height / 2);
}

// Auto-refresh stats
setInterval(() => {
    const activePage = document.querySelector('.page.active').id;
    if (activePage === 'page-dashboard') {
        loadDashboard();
    }
}, 5000);

// Initial load
loadDashboard();

// Replication functions
async function loadReplication() {
    const container = document.getElementById('replication-list');
    container.innerHTML = '<div class="loading">Loading replication tasks...</div>';
    
    try {
        const res = await fetch('/api/storage/replication.php');
        const data = await res.json();
        
        if (data.success) {
            if (data.tasks.length === 0) {
                container.innerHTML = '<p>No replication tasks configured.</p>';
                return;
            }
            
            let html = '<table><thead><tr><th>Name</th><th>Source</th><th>Destination</th><th>Last Run</th><th>Status</th><th>Actions</th></tr></thead><tbody>';
            
            data.tasks.forEach(task => {
                html += `
                    <tr>
                        <td><strong>${task.name}</strong></td>
                        <td>${task.source_dataset}</td>
                        <td>${task.destination_host}:${task.destination_dataset}</td>
                        <td>${task.last_run || 'Never'}</td>
                        <td>${task.last_status || 'Pending'}</td>
                        <td>
                            <button class="btn-primary" onclick="runReplication(${task.id})">Run</button>
                            <button class="btn-danger" onclick="deleteReplication(${task.id})">Delete</button>
                        </td>
                    </tr>
                `;
            });
            
            html += '</tbody></table>';
            container.innerHTML = html;
        } else {
            container.innerHTML = `<div class="error">Error: ${data.error}</div>`;
        }
    } catch (e) {
        container.innerHTML = `<div class="error">Failed to load tasks: ${e.message}</div>`;
    }
}

function showCreateReplication() {
    const modal = document.getElementById('modal');
    const body = document.getElementById('modal-body');
    
    body.innerHTML = `
        <h3>Create Replication Task</h3>
        <form id="create-replication-form">
            <div class="form-group">
                <label>Task Name</label>
                <input type="text" name="name" required placeholder="backup-to-remote">
            </div>
            <div class="form-group">
                <label>Source Dataset</label>
                <input type="text" name="source" required placeholder="tank/data">
            </div>
            <div class="form-group">
                <label>Destination Host</label>
                <input type="text" name="dest_host" required placeholder="backup.example.com">
            </div>
            <div class="form-group">
                <label>Destination Dataset</label>
                <input type="text" name="dest_dataset" required placeholder="backup/tank-data">
            </div>
            <div class="form-group">
                <label>Schedule</label>
                <select name="schedule">
                    <option value="manual">Manual</option>
                    <option value="daily">Daily</option>
                    <option value="weekly">Weekly</option>
                </select>
            </div>
            <button type="submit" class="btn-primary">Create Task</button>
        </form>
    `;
    
    modal.classList.add('active');
    
    document.getElementById('create-replication-form').addEventListener('submit', async (e) => {
        e.preventDefault();
        const formData = new FormData(e.target);
        
        try {
            const res = await fetch('/api/storage/replication.php', {
                method: 'POST',
                headers: {'Content-Type': 'application/json'},
                body: JSON.stringify({
                    action: 'create',
                    name: formData.get('name'),
                    source_dataset: formData.get('source'),
                    destination_host: formData.get('dest_host'),
                    destination_dataset: formData.get('dest_dataset'),
                    schedule_type: formData.get('schedule')
                })
            });
            
            const data = await res.json();
            if (data.success) {
                closeModal();
                loadReplication();
                alert('Replication task created!');
            } else {
                alert('Error: ' + data.error);
            }
        } catch (e) {
            alert('Failed to create task: ' + e.message);
        }
    });
}

async function runReplication(taskId) {
    if (!confirm('Run replication now? This will create a snapshot and send to destination.')) return;
    
    const modal = document.getElementById('modal');
    const body = document.getElementById('modal-body');
    
    // Show progress modal
    body.innerHTML = `
        <h3>Replication in Progress</h3>
        <div class="progress-container">
            <div class="progress-bar">
                <div id="progress-fill" class="progress-fill" style="width: 0%"></div>
            </div>
            <div id="progress-text" class="progress-text">Starting...</div>
        </div>
        <div id="progress-status" class="progress-status"></div>
    `;
    modal.classList.add('active');
    
    try {
        // Start replication
        const res = await fetch('/api/storage/replication.php', {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({action: 'run', task_id: taskId})
        });
        
        // Poll for progress
        let pollCount = 0;
        const maxPolls = 3600; // 1 hour max (1 poll per second)
        
        const pollProgress = setInterval(async () => {
            pollCount++;
            
            try {
                const progressRes = await fetch(`/api/storage/replication.php?action=progress&task_id=${taskId}`);
                const progressData = await progressRes.json();
                
                if (progressData.success && progressData.progress) {
                    const prog = progressData.progress;
                    
                    if (prog.status === 'sending') {
                        const percent = prog.percent || 0;
                        document.getElementById('progress-fill').style.width = percent + '%';
                        document.getElementById('progress-text').textContent = `Sending data: ${percent}%`;
                        document.getElementById('progress-status').textContent = 'Transferring snapshot...';
                    } else if (prog.status === 'complete') {
                        clearInterval(pollProgress);
                        document.getElementById('progress-fill').style.width = '100%';
                        document.getElementById('progress-text').textContent = 'Complete!';
                        document.getElementById('progress-status').textContent = 'Replication completed successfully';
                        
                        setTimeout(() => {
                            closeModal();
                            loadReplication();
                        }, 2000);
                    } else if (prog.status === 'failed') {
                        clearInterval(pollProgress);
                        document.getElementById('progress-status').innerHTML = 
                            '<div class="error">Replication failed. Check logs for details.</div>';
                    }
                }
                
                // Timeout after max polls
                if (pollCount >= maxPolls) {
                    clearInterval(pollProgress);
                    document.getElementById('progress-status').innerHTML = 
                        '<div class="error">Progress tracking timed out. Check task status.</div>';
                }
            } catch (e) {
                console.error('Progress polling error:', e);
            }
        }, 1000); // Poll every second
        
        const data = await res.json();
        if (!data.success) {
            throw new Error(data.error);
        }
        
    } catch (e) {
        closeModal();
        alert('Failed: ' + e.message);
    }
}

async function deleteReplication(taskId) {
    if (!confirm('Delete this replication task?')) return;
    
    try {
        const res = await fetch('/api/storage/replication.php', {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({action: 'delete', task_id: taskId})
        });
        
        const data = await res.json();
        if (data.success) {
            loadReplication();
        } else {
            alert('Error: ' + data.error);
        }
    } catch (e) {
        alert('Failed: ' + e.message);
    }
}

// Alerts functions
async function loadAlertHistory() {
    const container = document.getElementById('alert-history');
    container.innerHTML = '<div class="loading">Loading alert history...</div>';
    
    try {
        const res = await fetch('/api/system/alerts.php?action=history');
        const data = await res.json();
        
        if (data.success) {
            if (data.history.length === 0) {
                container.innerHTML = '<p>No alerts in history.</p>';
                return;
            }
            
            let html = '<table><thead><tr><th>Time</th><th>Type</th><th>Severity</th><th>Message</th><th>Sent</th></tr></thead><tbody>';
            
            data.history.forEach(alert => {
                html += `
                    <tr>
                        <td>${alert.timestamp}</td>
                        <td>${alert.alert_type}</td>
                        <td><span class="severity-${alert.severity}">${alert.severity}</span></td>
                        <td>${alert.message}</td>
                        <td>${alert.sent ? '✓' : '✗'}</td>
                    </tr>
                `;
            });
            
            html += '</tbody></table>';
            container.innerHTML = html;
        } else {
            container.innerHTML = `<div class="error">Error: ${data.error}</div>`;
        }
    } catch (e) {
        container.innerHTML = `<div class="error">Failed to load history: ${e.message}</div>`;
    }
}

function configureAlerts() {
    const modal = document.getElementById('modal');
    const body = document.getElementById('modal-body');
    
    body.innerHTML = `
        <h3>Configure Alerts</h3>
        <form id="configure-alerts-form">
            <div class="form-group">
                <label>Alert Type</label>
                <select name="alert_type" required>
                    <option value="pool_health">Pool Health (DEGRADED)</option>
                    <option value="smart_health">SMART Health (Disk Failure)</option>
                    <option value="replication_failure">Replication Failure</option>
                </select>
            </div>
            <div class="form-group">
                <label>
                    <input type="checkbox" name="enabled" checked> Enable Alerts
                </label>
            </div>
            <div class="form-group">
                <label>Webhook URL (Discord/Telegram)</label>
                <input type="text" name="webhook_url" placeholder="https://discord.com/api/webhooks/...">
            </div>
            <button type="button" class="btn-secondary" onclick="testWebhook()">Test Webhook</button>
            <button type="submit" class="btn-primary">Save Configuration</button>
        </form>
    `;
    
    modal.classList.add('active');
    
    document.getElementById('configure-alerts-form').addEventListener('submit', async (e) => {
        e.preventDefault();
        const formData = new FormData(e.target);
        
        try {
            const res = await fetch('/api/system/alerts.php', {
                method: 'POST',
                headers: {'Content-Type': 'application/json'},
                body: JSON.stringify({
                    action: 'configure',
                    alert_type: formData.get('alert_type'),
                    enabled: formData.get('enabled') ? 1 : 0,
                    webhook_url: formData.get('webhook_url')
                })
            });
            
            const data = await res.json();
            if (data.success) {
                closeModal();
                alert('Alert configuration saved!');
            } else {
                alert('Error: ' + data.error);
            }
        } catch (e) {
            alert('Failed to save: ' + e.message);
        }
    });
}

async function testWebhook() {
    const webhookUrl = document.querySelector('input[name="webhook_url"]').value;
    if (!webhookUrl) {
        alert('Enter webhook URL first');
        return;
    }
    
    try {
        const res = await fetch('/api/system/alerts.php', {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({action: 'test', webhook_url: webhookUrl})
        });
        
        const data = await res.json();
        if (data.success) {
            alert('Test webhook sent! Check your Discord/Telegram.');
        } else {
            alert('Error: ' + data.error);
        }
    } catch (e) {
        alert('Failed: ' + e.message);
    }
}

async function checkAlerts() {
    try {
        const res = await fetch('/api/system/alerts.php', {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({action: 'check'})
        });
        
        const data = await res.json();
        if (data.success) {
            loadAlertHistory();
            alert(`Alert check complete. Found ${data.count} alerts.`);
        } else {
            alert('Error: ' + data.error);
        }
    } catch (e) {
        alert('Failed: ' + e.message);
    }
}

// ========================================
// SHARES MANAGEMENT
// ========================================

async function loadShares() {
    try {
        const res = await fetch('/api/storage/shares.php');
        const data = await res.json();
        if (data.success) displayShares(data.shares);
    } catch (e) {
        console.error('Failed to load shares:', e);
    }
}

function displayShares(shares) {
    const container = document.getElementById('shares-list');
    if (!container) return;
    
    if (shares.length === 0) {
        container.innerHTML = '<p style="text-align:center;color:#aaa;">No shares configured</p>';
        return;
    }
    
    container.innerHTML = shares.map(share => `
        <div class="card">
            <div class="card-header">
                <div>
                    <h3>${share.name}</h3>
                    <span class="badge ${share.share_type === 'smb' ? 'badge-primary' : 'badge-secondary'}">${share.share_type.toUpperCase()}</span>
                    <span class="badge ${share.enabled ? 'badge-success' : 'badge-warning'}">${share.enabled ? 'Enabled' : 'Disabled'}</span>
                    <span class="badge badge-info">${share.status}</span>
                </div>
                <div class="button-group">
                    <button class="btn-info btn-sm" onclick="manageQuotas('${share.dataset_path}', '${share.name}')">Quotas</button>
                    <button class="btn-secondary btn-sm" onclick="editShare(${share.id})">Edit</button>
                    <button class="btn-danger btn-sm" onclick="deleteShare(${share.id}, '${share.name}')">Delete</button>
                </div>
            </div>
            <div class="card-content">
                <p><strong>Path:</strong> ${share.dataset_path}</p>
                ${share.comment ? `<p><strong>Comment:</strong> ${share.comment}</p>` : ''}
                ${share.share_type === 'smb' ? `
                    <p><strong>Guest:</strong> ${share.smb_guest_ok ? 'Yes' : 'No'} | <strong>Read Only:</strong> ${share.smb_read_only ? 'Yes' : 'No'}</p>
                    ${share.smb_valid_users ? `<p><strong>Users:</strong> ${share.smb_valid_users}</p>` : ''}
                ` : `
                    <p><strong>Networks:</strong> ${share.nfs_allowed_networks || 'All (*)'}</p>
                    <p><strong>Read Only:</strong> ${share.nfs_read_only ? 'Yes' : 'No'} | <strong>Sync:</strong> ${share.nfs_sync}</p>
                `}
            </div>
        </div>
    `).join('');
}

function showCreateShareModal() {
    const modal = document.getElementById('modal');
    const body = document.getElementById('modal-body');
    
    body.innerHTML = `
        <h3>Create Share</h3>
        <form id="create-share-form">
            <div class="form-group">
                <label>Name *</label>
                <input type="text" name="name" required pattern="[a-zA-Z0-9_-]+">
            </div>
            <div class="form-group">
                <label>Dataset *</label>
                <input type="text" name="dataset_path" required placeholder="tank/data">
            </div>
            <div class="form-group">
                <label>Type *</label>
                <select name="share_type" required onchange="toggleShareOptions(this.value)">
                    <option value="smb">SMB</option>
                    <option value="nfs">NFS</option>
                </select>
            </div>
            <div class="form-group">
                <label>Comment</label>
                <input type="text" name="comment">
            </div>
            
            <div id="smb-options">
                <h4>SMB Options</h4>
                <div class="form-group">
                    <label><input type="checkbox" name="smb_guest_ok"> Guest Access</label>
                </div>
                <div class="form-group">
                    <label><input type="checkbox" name="smb_read_only"> Read Only</label>
                </div>
                <div class="form-group">
                    <label>Valid Users</label>
                    <input type="text" name="smb_valid_users" placeholder="user1,user2">
                </div>
            </div>
            
            <div id="nfs-options" style="display:none;">
                <h4>NFS Options</h4>
                <div class="form-group">
                    <label>Networks</label>
                    <input type="text" name="nfs_allowed_networks" placeholder="192.168.1.0/24">
                </div>
                <div class="form-group">
                    <label><input type="checkbox" name="nfs_read_only"> Read Only</label>
                </div>
                <div class="form-group">
                    <label>Sync</label>
                    <select name="nfs_sync">
                        <option value="async">Async</option>
                        <option value="sync">Sync</option>
                    </select>
                </div>
            </div>
            
            <div class="button-group">
                <button type="submit" class="btn-primary">Create</button>
                <button type="button" class="btn-secondary" onclick="closeModal()">Cancel</button>
            </div>
        </form>
    `;
    
    modal.classList.add('active');
    
    document.getElementById('create-share-form').onsubmit = async (e) => {
        e.preventDefault();
        const fd = new FormData(e.target);
        const data = {
            action: 'create',
            name: fd.get('name'),
            dataset_path: fd.get('dataset_path'),
            share_type: fd.get('share_type'),
            comment: fd.get('comment'),
            smb_guest_ok: fd.get('smb_guest_ok') ? 1 : 0,
            smb_read_only: fd.get('smb_read_only') ? 1 : 0,
            smb_valid_users: fd.get('smb_valid_users'),
            nfs_allowed_networks: fd.get('nfs_allowed_networks'),
            nfs_read_only: fd.get('nfs_read_only') ? 1 : 0,
            nfs_sync: fd.get('nfs_sync')
        };
        
        try {
            const res = await fetch('/api/storage/shares.php', {
                method: 'POST',
                headers: {'Content-Type': 'application/json'},
                body: JSON.stringify(data)
            });
            const result = await res.json();
            if (result.success) {
                closeModal();
                loadShares();
            } else {
                alert('Error: ' + result.error);
            }
        } catch (e) {
            alert('Failed: ' + e.message);
        }
    };
}

function toggleShareOptions(type) {
    document.getElementById('smb-options').style.display = type === 'smb' ? 'block' : 'none';
    document.getElementById('nfs-options').style.display = type === 'nfs' ? 'block' : 'none';
}

async function deleteShare(id, name) {
    if (!confirm(`Delete share "${name}"?`)) return;
    try {
        const res = await fetch('/api/storage/shares.php', {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({action: 'delete', id})
        });
        const data = await res.json();
        if (data.success) loadShares();
        else alert('Error: ' + data.error);
    } catch (e) {
        alert('Failed: ' + e.message);
    }
}

// ========================================
// POOL EXPANSION
// ========================================

function showAddVdevModal(poolName) {
    const modal = document.getElementById('modal');
    const body = document.getElementById('modal-body');
    
    body.innerHTML = `
        <h3>Add VDEV to ${poolName}</h3>
        <form id="add-vdev-form">
            <div class="form-group">
                <label>VDEV Type</label>
                <select name="vdev_type" required>
                    <option value="">Stripe (no redundancy)</option>
                    <option value="mirror">Mirror</option>
                    <option value="raidz">RAIDZ (single parity)</option>
                    <option value="raidz2">RAIDZ2 (double parity)</option>
                    <option value="raidz3">RAIDZ3 (triple parity)</option>
                </select>
            </div>
            
            <div class="form-group">
                <label>Select Disks</label>
                <div id="disk-selection"></div>
            </div>
            
            <div class="button-group">
                <button type="submit" class="btn-primary">Add VDEV</button>
                <button type="button" class="btn-secondary" onclick="closeModal()">Cancel</button>
            </div>
        </form>
    `;
    
    // Load available disks
    fetch('/api/system/disks.php').then(res => res.json()).then(data => {
        if (data.success) {
            const container = document.getElementById('disk-selection');
            container.innerHTML = data.disks.map(disk => `
                <label>
                    <input type="checkbox" name="disks" value="${disk.path}">
                    ${disk.name} - ${disk.size} (${disk.model || 'Unknown'})
                </label>
            `).join('<br>');
        }
    });
    
    modal.classList.add('active');
    
    document.getElementById('add-vdev-form').onsubmit = async (e) => {
        e.preventDefault();
        const fd = new FormData(e.target);
        const disks = Array.from(fd.getAll('disks'));
        
        if (disks.length === 0) {
            alert('Please select at least one disk');
            return;
        }
        
        if (!confirm(`Add ${disks.length} disk(s) to pool ${poolName}?`)) return;
        
        try {
            const res = await fetch('/api/storage/pools.php', {
                method: 'POST',
                headers: {'Content-Type': 'application/json'},
                body: JSON.stringify({
                    action: 'add_vdev',
                    name: poolName,
                    vdev_type: fd.get('vdev_type'),
                    disks: disks
                })
            });
            const result = await res.json();
            if (result.success) {
                closeModal();
                loadPools();
                alert('VDEV added successfully!');
            } else {
                alert('Error: ' + result.error);
            }
        } catch (e) {
            alert('Failed: ' + e.message);
        }
    };
}

function showReplaceDiskModal(poolName) {
    const modal = document.getElementById('modal');
    const body = document.getElementById('modal-body');
    
    body.innerHTML = `
        <h3>Replace Disk in ${poolName}</h3>
        <form id="replace-disk-form">
            <div class="form-group">
                <label>Old Disk (to replace)</label>
                <select name="old_disk" required id="old-disk-select"></select>
            </div>
            
            <div class="form-group">
                <label>New Disk (replacement)</label>
                <select name="new_disk" required id="new-disk-select"></select>
            </div>
            
            <p><strong>Note:</strong> The new disk must be at least as large as the old disk. Resilver will start automatically.</p>
            
            <div class="button-group">
                <button type="submit" class="btn-danger">Replace Disk</button>
                <button type="button" class="btn-secondary" onclick="closeModal()">Cancel</button>
            </div>
        </form>
    `;
    
    // Load pool disks
    fetch('/api/storage/pools.php?name=' + encodeURIComponent(poolName)).then(res => res.json()).then(data => {
        if (data.success && data.pool) {
            const oldSelect = document.getElementById('old-disk-select');
            // Parse vdev config to get disks
            const vdevs = data.pool.vdevs || [];
            vdevs.forEach(vdev => {
                if (vdev.devices) {
                    vdev.devices.forEach(dev => {
                        const opt = document.createElement('option');
                        opt.value = dev.name;
                        opt.textContent = `${dev.name} (${dev.state})`;
                        oldSelect.appendChild(opt);
                    });
                }
            });
        }
    });
    
    // Load available disks
    fetch('/api/system/disks.php').then(res => res.json()).then(data => {
        if (data.success) {
            const newSelect = document.getElementById('new-disk-select');
            data.disks.forEach(disk => {
                const opt = document.createElement('option');
                opt.value = disk.path;
                opt.textContent = `${disk.name} - ${disk.size}`;
                newSelect.appendChild(opt);
            });
        }
    });
    
    modal.classList.add('active');
    
    document.getElementById('replace-disk-form').onsubmit = async (e) => {
        e.preventDefault();
        const fd = new FormData(e.target);
        
        const oldDisk = fd.get('old_disk');
        const newDisk = fd.get('new_disk');
        
        if (!confirm(`Replace ${oldDisk} with ${newDisk} in pool ${poolName}?\n\nThis will start a resilver process.`)) {
            return;
        }
        
        try {
            const res = await fetch('/api/storage/pools.php', {
                method: 'POST',
                headers: {'Content-Type': 'application/json'},
                body: JSON.stringify({
                    action: 'replace_disk',
                    name: poolName,
                    old_disk: oldDisk,
                    new_disk: newDisk
                })
            });
            const result = await res.json();
            if (result.success) {
                closeModal();
                loadPools();
                alert('Disk replacement started! Resilver in progress.');
            } else {
                alert('Error: ' + result.error);
            }
        } catch (e) {
            alert('Failed: ' + e.message);
        }
    };
}

function showAttachDiskModal(poolName) {
    const modal = document.getElementById('modal');
    const body = document.getElementById('modal-body');
    
    body.innerHTML = `
        <h3>Attach Disk to ${poolName}</h3>
        <p>Convert a single disk to a mirror by attaching another disk.</p>
        <form id="attach-disk-form">
            <div class="form-group">
                <label>Existing Disk (to mirror)</label>
                <select name="existing_disk" required id="existing-disk-select"></select>
            </div>
            
            <div class="form-group">
                <label>New Disk (to attach)</label>
                <select name="new_disk" required id="attach-new-disk-select"></select>
            </div>
            
            <p><strong>Note:</strong> This converts a stripe to a mirror. Resilver will start automatically.</p>
            
            <div class="button-group">
                <button type="submit" class="btn-primary">Attach Disk</button>
                <button type="button" class="btn-secondary" onclick="closeModal()">Cancel</button>
            </div>
        </form>
    `;
    
    // Load pool disks
    fetch('/api/storage/pools.php?name=' + encodeURIComponent(poolName)).then(res => res.json()).then(data => {
        if (data.success && data.pool) {
            const existingSelect = document.getElementById('existing-disk-select');
            const vdevs = data.pool.vdevs || [];
            vdevs.forEach(vdev => {
                if (vdev.devices) {
                    vdev.devices.forEach(dev => {
                        const opt = document.createElement('option');
                        opt.value = dev.name;
                        opt.textContent = `${dev.name} (${dev.state})`;
                        existingSelect.appendChild(opt);
                    });
                }
            });
        }
    });
    
    // Load available disks
    fetch('/api/system/disks.php').then(res => res.json()).then(data => {
        if (data.success) {
            const newSelect = document.getElementById('attach-new-disk-select');
            data.disks.forEach(disk => {
                const opt = document.createElement('option');
                opt.value = disk.path;
                opt.textContent = `${disk.name} - ${disk.size}`;
                newSelect.appendChild(opt);
            });
        }
    });
    
    modal.classList.add('active');
    
    document.getElementById('attach-disk-form').onsubmit = async (e) => {
        e.preventDefault();
        const fd = new FormData(e.target);
        
        const existingDisk = fd.get('existing_disk');
        const newDisk = fd.get('new_disk');
        
        if (!confirm(`Attach ${newDisk} to ${existingDisk} in pool ${poolName}?\n\nThis will create a mirror and start resilver.`)) {
            return;
        }
        
        try {
            const res = await fetch('/api/storage/pools.php', {
                method: 'POST',
                headers: {'Content-Type': 'application/json'},
                body: JSON.stringify({
                    action: 'attach_disk',
                    name: poolName,
                    existing_disk: existingDisk,
                    new_disk: newDisk
                })
            });
            const result = await res.json();
            if (result.success) {
                closeModal();
                loadPools();
                alert('Disk attached successfully! Resilver in progress.');
            } else {
                alert('Error: ' + result.error);
            }
        } catch (e) {
            alert('Failed: ' + e.message);
        }
    };
}

// ========================================
// RCLONE CLOUD SYNC
// ========================================

async function loadRclone() {
    loadRcloneRemotes();
    loadRcloneTasks();
}

async function loadRcloneRemotes() {
    const res = await fetch('/api/system/rclone.php?action=list_remotes');
    const data = await res.json();
    if (data.success) {
        const container = document.getElementById('rclone-remotes-list');
        if (data.remotes.length === 0) {
            container.innerHTML = '<p style="color:#aaa;">No remotes configured</p>';
        } else {
            container.innerHTML = data.remotes.map(r => `
                <div class="card">
                    <h4>${r.name} (${r.remote_type})</h4>
                    <button class="btn-secondary" onclick="testRemote(${r.id})">Test</button>
                    <button class="btn-danger" onclick="deleteRemote(${r.id})">Delete</button>
                </div>
            `).join('');
        }
    }
}

async function loadRcloneTasks() {
    const res = await fetch('/api/system/rclone.php?action=list_tasks');
    const data = await res.json();
    if (data.success) {
        const container = document.getElementById('rclone-tasks-list');
        if (data.tasks.length === 0) {
            container.innerHTML = '<p style="color:#aaa;">No tasks configured</p>';
        } else {
            container.innerHTML = data.tasks.map(t => `
                <div class="card">
                    <h4>${t.name}</h4>
                    <p><strong>Remote:</strong> ${t.remote_name} | <strong>Direction:</strong> ${t.direction} | <strong>Type:</strong> ${t.sync_type}</p>
                    <p><strong>Source:</strong> ${t.source_path} → <strong>Dest:</strong> ${t.destination_path}</p>
                    ${t.last_run ? `<p><strong>Last Run:</strong> ${new Date(t.last_run).toLocaleString()} (${t.last_status})</p>` : ''}
                    <button class="btn-primary" onclick="runRcloneTask(${t.id})">Run Now</button>
                    <button class="btn-danger" onclick="deleteRcloneTask(${t.id})">Delete</button>
                </div>
            `).join('');
        }
    }
}

function showCreateRemoteModal() {
    const modal = document.getElementById('modal');
    const body = document.getElementById('modal-body');
    body.innerHTML = `
        <h3>Add Remote</h3>
        <form id="create-remote-form">
            <div class="form-group">
                <label>Name *</label>
                <input type="text" name="name" required>
            </div>
            <div class="form-group">
                <label>Type *</label>
                <select name="remote_type" required id="remote-type-select"></select>
            </div>
            <div id="remote-config"></div>
            <p><small>Enter configuration as JSON. See <a href="https://rclone.org/docs/" target="_blank">rclone.org/docs</a> for parameters.</small></p>
            <div class="form-group">
                <label>Config (JSON)</label>
                <textarea name="config" rows="10" placeholder='{"access_key_id": "...", "secret_access_key": "..."}'></textarea>
            </div>
            <div class="button-group">
                <button type="submit" class="btn-primary">Add</button>
                <button type="button" class="btn-secondary" onclick="closeModal()">Cancel</button>
            </div>
        </form>
    `;
    fetch('/api/system/rclone.php?action=backends').then(res => res.json()).then(data => {
        const select = document.getElementById('remote-type-select');
        Object.entries(data.backends).forEach(([key, label]) => {
            select.innerHTML += `<option value="${key}">${label}</option>`;
        });
    });
    modal.classList.add('active');
    document.getElementById('create-remote-form').onsubmit = async (e) => {
        e.preventDefault();
        const fd = new FormData(e.target);
        try {
            const config = JSON.parse(fd.get('config') || '{}');
            const res = await fetch('/api/system/rclone.php', {
                method: 'POST',
                headers: {'Content-Type': 'application/json'},
                body: JSON.stringify({
                    action: 'create_remote',
                    name: fd.get('name'),
                    remote_type: fd.get('remote_type'),
                    config: config
                })
            });
            const result = await res.json();
            if (result.success) {
                closeModal();
                loadRcloneRemotes();
            } else {
                alert('Error: ' + result.error);
            }
        } catch (e) {
            alert('Invalid JSON or error: ' + e.message);
        }
    };
}

function showCreateRcloneTaskModal() {
    const modal = document.getElementById('modal');
    const body = document.getElementById('modal-body');
    body.innerHTML = `
        <h3>Create Sync Task</h3>
        <form id="create-rclone-task-form">
            <div class="form-group">
                <label>Name *</label>
                <input type="text" name="name" required>
            </div>
            <div class="form-group">
                <label>Remote *</label>
                <select name="remote_id" required id="task-remote-select"></select>
            </div>
            <div class="form-group">
                <label>Direction *</label>
                <select name="direction" required>
                    <option value="push">Push (Local → Remote)</option>
                    <option value="pull">Pull (Remote → Local)</option>
                </select>
            </div>
            <div class="form-group">
                <label>Source Path *</label>
                <input type="text" name="source_path" required placeholder="/tank/data or remote_path">
            </div>
            <div class="form-group">
                <label>Destination Path *</label>
                <input type="text" name="destination_path" required>
            </div>
            <div class="form-group">
                <label>Sync Type *</label>
                <select name="sync_type" required>
                    <option value="sync">Sync (mirror)</option>
                    <option value="copy">Copy (one-way)</option>
                    <option value="move">Move (delete source)</option>
                </select>
            </div>
            <div class="button-group">
                <button type="submit" class="btn-primary">Create</button>
                <button type="button" class="btn-secondary" onclick="closeModal()">Cancel</button>
            </div>
        </form>
    `;
    fetch('/api/system/rclone.php?action=list_remotes').then(res => res.json()).then(data => {
        const select = document.getElementById('task-remote-select');
        data.remotes.forEach(r => {
            select.innerHTML += `<option value="${r.id}">${r.name}</option>`;
        });
    });
    modal.classList.add('active');
    document.getElementById('create-rclone-task-form').onsubmit = async (e) => {
        e.preventDefault();
        const fd = new FormData(e.target);
        const res = await fetch('/api/system/rclone.php', {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({
                action: 'create_task',
                name: fd.get('name'),
                remote_id: parseInt(fd.get('remote_id')),
                direction: fd.get('direction'),
                source_path: fd.get('source_path'),
                destination_path: fd.get('destination_path'),
                sync_type: fd.get('sync_type')
            })
        });
        const result = await res.json();
        if (result.success) {
            closeModal();
            loadRcloneTasks();
        } else {
            alert('Error: ' + result.error);
        }
    };
}

async function runRcloneTask(id) {
    if (!confirm('Run sync task now?')) return;
    try {
        const res = await fetch('/api/system/rclone.php', {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({action: 'run_task', task_id: id})
        });
        const data = await res.json();
        if (data.success) {
            alert('Sync completed successfully!');
            loadRcloneTasks();
        } else {
            alert('Sync failed: ' + data.output);
        }
    } catch (e) {
        alert('Error: ' + e.message);
    }
}

async function testRemote(id) {
    try {
        const res = await fetch('/api/system/rclone.php', {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({action: 'test_remote', id})
        });
        const data = await res.json();
        alert(data.success ? 'Remote is working!' : 'Test failed: ' + data.output);
    } catch (e) {
        alert('Error: ' + e.message);
    }
}

async function deleteRemote(id) {
    if (!confirm('Delete remote?')) return;
    const res = await fetch('/api/system/rclone.php', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({action: 'delete_remote', id})
    });
    const data = await res.json();
    if (data.success) loadRcloneRemotes();
}

async function deleteRcloneTask(id) {
    if (!confirm('Delete task?')) return;
    const res = await fetch('/api/system/rclone.php', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({action: 'delete_task', id})
    });
    const data = await res.json();
    if (data.success) loadRcloneTasks();
}

// ========================================
// QUOTA MANAGEMENT
// ========================================

async function manageQuotas(datasetPath, shareName) {
    const modal = document.getElementById('modal');
    const body = document.getElementById('modal-body');
    
    // Load quotas for this dataset
    try {
        const res = await fetch(`/api/storage/quotas.php?action=get_by_dataset&dataset_path=${encodeURIComponent(datasetPath)}`);
        const data = await res.json();
        
        if (!data.success) {
            alert('Error loading quotas: ' + data.error);
            return;
        }
        
        const quotas = data.quotas || [];
        
        body.innerHTML = `
            <h3>User Quotas for ${shareName}</h3>
            <p style="color:#aaa;font-size:0.9em;margin-bottom:1rem;">Dataset: ${datasetPath}</p>
            
            <div class="page-header" style="margin-bottom:1rem;">
                <button class="btn-primary btn-sm" onclick="showAddQuotaModal('${datasetPath}', '${shareName}')">
                    Add User Quota
                </button>
            </div>
            
            ${quotas.length === 0 ? `
                <p style="text-align:center;color:#aaa;margin:2rem 0;">No quotas configured</p>
            ` : `
                <div class="quotas-grid">
                    ${quotas.map(q => `
                        <div class="quota-card">
                            <div class="quota-header">
                                <div>
                                    <h4>${q.username}</h4>
                                    <span class="badge ${q.enabled ? 'badge-success' : 'badge-warning'}">
                                        ${q.enabled ? 'Active' : 'Disabled'}
                                    </span>
                                </div>
                                <div class="button-group">
                                    <button class="btn-secondary btn-sm" onclick="editQuota(${q.id}, '${datasetPath}', '${shareName}')">Edit</button>
                                    <button class="btn-danger btn-sm" onclick="deleteQuota(${q.id}, '${q.username}', '${datasetPath}', '${shareName}')">Delete</button>
                                </div>
                            </div>
                            <div class="quota-usage">
                                <div class="quota-info">
                                    <span class="usage-text">${q.used_formatted} / ${q.quota_formatted}</span>
                                    <span class="usage-percent">${q.usage_percent}%</span>
                                </div>
                                <div class="progress-bar">
                                    <div class="progress-fill ${q.usage_percent >= 90 ? 'danger' : q.usage_percent >= 75 ? 'warning' : 'success'}" 
                                         style="width: ${Math.min(q.usage_percent, 100)}%"></div>
                                </div>
                            </div>
                        </div>
                    `).join('')}
                </div>
            `}
            
            <div class="button-group" style="margin-top:1.5rem;">
                <button class="btn-secondary" onclick="closeModal()">Close</button>
            </div>
        `;
        
        modal.classList.add('active');
        
    } catch (e) {
        alert('Failed to load quotas: ' + e.message);
    }
}

async function showAllQuotas() {
    const modal = document.getElementById('modal');
    const body = document.getElementById('modal-body');
    
    try {
        const res = await fetch('/api/storage/quotas.php?action=list');
        const data = await res.json();
        
        if (!data.success) {
            alert('Error loading quotas: ' + data.error);
            return;
        }
        
        const quotas = data.quotas || [];
        
        // Group quotas by dataset
        const grouped = {};
        quotas.forEach(q => {
            if (!grouped[q.dataset_path]) {
                grouped[q.dataset_path] = [];
            }
            grouped[q.dataset_path].push(q);
        });
        
        body.innerHTML = `
            <h3>All User Quotas</h3>
            
            ${Object.keys(grouped).length === 0 ? `
                <p style="text-align:center;color:#aaa;margin:2rem 0;">No quotas configured</p>
            ` : Object.keys(grouped).map(dataset => `
                <div class="dataset-quota-section">
                    <h4 style="margin:1.5rem 0 0.5rem 0;color:#4CAF50;">${dataset}</h4>
                    <div class="quotas-grid">
                        ${grouped[dataset].map(q => `
                            <div class="quota-card">
                                <div class="quota-header">
                                    <div>
                                        <h4>${q.username}</h4>
                                        <span class="badge ${q.enabled ? 'badge-success' : 'badge-warning'}">
                                            ${q.enabled ? 'Active' : 'Disabled'}
                                        </span>
                                    </div>
                                </div>
                                <div class="quota-usage">
                                    <div class="quota-info">
                                        <span class="usage-text">${q.used_formatted} / ${q.quota_formatted}</span>
                                        <span class="usage-percent">${q.usage_percent}%</span>
                                    </div>
                                    <div class="progress-bar">
                                        <div class="progress-fill ${q.usage_percent >= 90 ? 'danger' : q.usage_percent >= 75 ? 'warning' : 'success'}" 
                                             style="width: ${Math.min(q.usage_percent, 100)}%"></div>
                                    </div>
                                </div>
                            </div>
                        `).join('')}
                    </div>
                </div>
            `).join('')}
            
            <div class="button-group" style="margin-top:1.5rem;">
                <button class="btn-secondary" onclick="closeModal()">Close</button>
            </div>
        `;
        
        modal.classList.add('active');
        
    } catch (e) {
        alert('Failed to load quotas: ' + e.message);
    }
}

function showAddQuotaModal(datasetPath, shareName) {
    const modal = document.getElementById('modal');
    const body = document.getElementById('modal-body');
    
    body.innerHTML = `
        <h3>Add User Quota</h3>
        <p style="color:#aaa;font-size:0.9em;margin-bottom:1rem;">Dataset: ${datasetPath}</p>
        
        <form id="add-quota-form">
            <div class="form-group">
                <label>Username *</label>
                <input type="text" name="username" required pattern="[a-zA-Z0-9_-]+" 
                       placeholder="e.g., johndoe">
                <small>Linux username (alphanumeric, underscore, hyphen)</small>
            </div>
            
            <div class="form-row">
                <div class="form-group" style="flex:2;">
                    <label>Quota Size *</label>
                    <input type="number" name="quota_size" required min="1" step="1" 
                           placeholder="e.g., 500">
                </div>
                <div class="form-group" style="flex:1;">
                    <label>Unit</label>
                    <select name="quota_unit">
                        <option value="MB">MB</option>
                        <option value="GB" selected>GB</option>
                        <option value="TB">TB</option>
                    </select>
                </div>
            </div>
            
            <div class="info-box">
                <strong>Note:</strong> This sets a ZFS userquota, limiting how much data this user can write to the dataset.
            </div>
            
            <div class="button-group">
                <button type="submit" class="btn-primary">Create Quota</button>
                <button type="button" class="btn-secondary" onclick="manageQuotas('${datasetPath}', '${shareName}')">
                    Back
                </button>
            </div>
        </form>
    `;
    
    modal.classList.add('active');
    
    document.getElementById('add-quota-form').onsubmit = async (e) => {
        e.preventDefault();
        const fd = new FormData(e.target);
        
        try {
            const res = await fetch('/api/storage/quotas.php', {
                method: 'POST',
                headers: {'Content-Type': 'application/json'},
                body: JSON.stringify({
                    action: 'create',
                    username: fd.get('username'),
                    dataset_path: datasetPath,
                    quota_size: parseInt(fd.get('quota_size')),
                    quota_unit: fd.get('quota_unit')
                })
            });
            
            const data = await res.json();
            
            if (data.success) {
                manageQuotas(datasetPath, shareName);
            } else {
                alert('Error: ' + data.error);
            }
        } catch (e) {
            alert('Failed: ' + e.message);
        }
    };
}

async function editQuota(id, datasetPath, shareName) {
    const modal = document.getElementById('modal');
    const body = document.getElementById('modal-body');
    
    // Get current quota data
    try {
        const res = await fetch(`/api/storage/quotas.php?action=get_by_dataset&dataset_path=${encodeURIComponent(datasetPath)}`);
        const data = await res.json();
        
        if (!data.success) {
            alert('Error loading quota: ' + data.error);
            return;
        }
        
        const quota = data.quotas.find(q => q.id === id);
        if (!quota) {
            alert('Quota not found');
            return;
        }
        
        // Convert bytes to appropriate unit
        let size, unit;
        const gb = quota.quota_bytes / (1024 * 1024 * 1024);
        const tb = quota.quota_bytes / (1024 * 1024 * 1024 * 1024);
        
        if (tb >= 1) {
            size = tb;
            unit = 'TB';
        } else if (gb >= 1) {
            size = gb;
            unit = 'GB';
        } else {
            size = quota.quota_bytes / (1024 * 1024);
            unit = 'MB';
        }
        
        body.innerHTML = `
            <h3>Edit Quota for ${quota.username}</h3>
            <p style="color:#aaa;font-size:0.9em;margin-bottom:1rem;">Dataset: ${datasetPath}</p>
            
            <form id="edit-quota-form">
                <div class="form-row">
                    <div class="form-group" style="flex:2;">
                        <label>Quota Size *</label>
                        <input type="number" name="quota_size" required min="1" step="0.01" 
                               value="${size.toFixed(2)}">
                    </div>
                    <div class="form-group" style="flex:1;">
                        <label>Unit</label>
                        <select name="quota_unit">
                            <option value="MB" ${unit === 'MB' ? 'selected' : ''}>MB</option>
                            <option value="GB" ${unit === 'GB' ? 'selected' : ''}>GB</option>
                            <option value="TB" ${unit === 'TB' ? 'selected' : ''}>TB</option>
                        </select>
                    </div>
                </div>
                
                <div class="info-box">
                    <strong>Current Usage:</strong> ${quota.used_formatted} (${quota.usage_percent}%)
                </div>
                
                <div class="button-group">
                    <button type="submit" class="btn-primary">Update Quota</button>
                    <button type="button" class="btn-secondary" onclick="manageQuotas('${datasetPath}', '${shareName}')">
                        Cancel
                    </button>
                </div>
            </form>
        `;
        
        modal.classList.add('active');
        
        document.getElementById('edit-quota-form').onsubmit = async (e) => {
            e.preventDefault();
            const fd = new FormData(e.target);
            
            try {
                const res = await fetch('/api/storage/quotas.php', {
                    method: 'POST',
                    headers: {'Content-Type': 'application/json'},
                    body: JSON.stringify({
                        action: 'update',
                        id: id,
                        quota_size: parseFloat(fd.get('quota_size')),
                        quota_unit: fd.get('quota_unit')
                    })
                });
                
                const data = await res.json();
                
                if (data.success) {
                    manageQuotas(datasetPath, shareName);
                } else {
                    alert('Error: ' + data.error);
                }
            } catch (e) {
                alert('Failed: ' + e.message);
            }
        };
        
    } catch (e) {
        alert('Failed to load quota: ' + e.message);
    }
}

async function deleteQuota(id, username, datasetPath, shareName) {
    if (!confirm(`Remove quota for ${username}? This will allow unlimited space usage.`)) {
        return;
    }
    
    try {
        const res = await fetch('/api/storage/quotas.php', {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({
                action: 'delete',
                id: id
            })
        });
        
        const data = await res.json();
        
        if (data.success) {
            manageQuotas(datasetPath, shareName);
        } else {
            alert('Error: ' + data.error);
        }
    } catch (e) {
        alert('Failed: ' + e.message);
    }
}


// ========================================
// DISK HEALTH MONITORING
// ========================================

async function loadDiskHealth() {
    try {
        const res = await fetch('/api/storage/disk-health.php?action=list');
        const data = await res.json();
        
        if (data.success) {
            displayDiskHealth(data.disks);
        }
    } catch (e) {
        console.error('Failed to load disk health:', e);
    }
}

function displayDiskHealth(disks) {
    // Update summary stats
    const totalDisks = disks.length;
    const healthyDisks = disks.filter(d => d.status === 'healthy').length;
    const warningDisks = disks.filter(d => d.status === 'warning').length;
    const criticalDisks = disks.filter(d => d.status === 'critical' || d.status === 'failing').length;
    
    document.getElementById('total-disks').textContent = totalDisks;
    document.getElementById('healthy-disks').textContent = healthyDisks;
    document.getElementById('warning-disks').textContent = warningDisks;
    document.getElementById('critical-disks').textContent = criticalDisks;
    
    // Display disk list
    const container = document.getElementById('disks-health-list');
    if (!container) return;
    
    if (disks.length === 0) {
        container.innerHTML = '<p style="text-align:center;color:#aaa;">No disks detected</p>';
        return;
    }
    
    container.innerHTML = disks.map(disk => {
        const statusColor = {
            'healthy': 'success',
            'warning': 'warning',
            'critical': 'danger',
            'failing': 'danger'
        }[disk.status] || 'secondary';
        
        const tempColor = disk.temperature ? (
            disk.temperature > 50 ? 'danger' :
            disk.temperature > 40 ? 'warning' : 'success'
        ) : 'secondary';
        
        return `
            <div class="disk-health-card ${disk.status}">
                <div class="disk-card-header">
                    <div>
                        <h3>${disk.name}</h3>
                        <span class="badge badge-${statusColor}">${disk.status.toUpperCase()}</span>
                        ${disk.in_pool ? `<span class="badge badge-primary">Pool: ${disk.in_pool}</span>` : ''}
                    </div>
                    <div class="button-group">
                        <button class="btn-info btn-sm" onclick="showDiskDetails('${disk.path}')">Details</button>
                        <button class="btn-secondary btn-sm" onclick="showDiskActions('${disk.path}')">Actions</button>
                    </div>
                </div>
                <div class="disk-card-body">
                    <div class="disk-info-grid">
                        <div class="disk-info-item">
                            <span class="info-label">Model</span>
                            <span class="info-value">${disk.model}</span>
                        </div>
                        <div class="disk-info-item">
                            <span class="info-label">Size</span>
                            <span class="info-value">${disk.size_human}</span>
                        </div>
                        <div class="disk-info-item">
                            <span class="info-label">Serial</span>
                            <span class="info-value">${disk.serial}</span>
                        </div>
                        <div class="disk-info-item">
                            <span class="info-label">SMART Health</span>
                            <span class="info-value">${disk.health}</span>
                        </div>
                        <div class="disk-info-item">
                            <span class="info-label">Temperature</span>
                            <span class="info-value badge-${tempColor}">${disk.temperature !== null ? disk.temperature + '°C' : 'N/A'}</span>
                        </div>
                        <div class="disk-info-item">
                            <span class="info-label">Power On Hours</span>
                            <span class="info-value">${disk.power_on_hours !== null ? disk.power_on_hours.toLocaleString() : 'N/A'}</span>
                        </div>
                        ${disk.reallocated_sectors !== null && disk.reallocated_sectors > 0 ? `
                            <div class="disk-info-item warning-highlight">
                                <span class="info-label">⚠️ Reallocated Sectors</span>
                                <span class="info-value">${disk.reallocated_sectors}</span>
                            </div>
                        ` : ''}
                        ${disk.pending_sectors !== null && disk.pending_sectors > 0 ? `
                            <div class="disk-info-item warning-highlight">
                                <span class="info-label">⚠️ Pending Sectors</span>
                                <span class="info-value">${disk.pending_sectors}</span>
                            </div>
                        ` : ''}
                    </div>
                </div>
            </div>
        `;
    }).join('');
}

async function showDiskDetails(diskPath) {
    const modal = document.getElementById('modal');
    const body = document.getElementById('modal-body');
    
    try {
        const res = await fetch(`/api/storage/disk-health.php?action=details&disk=${encodeURIComponent(diskPath)}`);
        const data = await res.json();
        
        if (!data.success) {
            alert('Error: ' + data.error);
            return;
        }
        
        body.innerHTML = `
            <h3>Disk Details: ${diskPath}</h3>
            
            <div class="tabs-container">
                <div class="tabs">
                    <button class="tab-btn active" data-tab="smart">SMART Data</button>
                    <button class="tab-btn" data-tab="history">Test History</button>
                    <button class="tab-btn" data-tab="maintenance">Maintenance Log</button>
                    ${data.tracking ? `<button class="tab-btn" data-tab="tracking">Tracking Info</button>` : ''}
                </div>
                
                <div class="tab-content active" id="tab-smart">
                    <pre class="smart-output">${data.raw_output}</pre>
                </div>
                
                <div class="tab-content" id="tab-history">
                    ${data.smart_history.length === 0 ? '<p style="color:#aaa;">No test history</p>' : `
                        <table class="data-table">
                            <thead>
                                <tr>
                                    <th>Date</th>
                                    <th>Test Type</th>
                                    <th>Health Status</th>
                                    <th>Temperature</th>
                                </tr>
                            </thead>
                            <tbody>
                                ${data.smart_history.map(h => `
                                    <tr>
                                        <td>${new Date(h.timestamp).toLocaleString()}</td>
                                        <td>${h.test_type}</td>
                                        <td>${h.health_status || 'N/A'}</td>
                                        <td>${h.temperature ? h.temperature + '°C' : 'N/A'}</td>
                                    </tr>
                                `).join('')}
                            </tbody>
                        </table>
                    `}
                </div>
                
                <div class="tab-content" id="tab-maintenance">
                    ${data.maintenance_log.length === 0 ? '<p style="color:#aaa;">No maintenance records</p>' : `
                        <table class="data-table">
                            <thead>
                                <tr>
                                    <th>Date</th>
                                    <th>Action</th>
                                    <th>Description</th>
                                    <th>By</th>
                                </tr>
                            </thead>
                            <tbody>
                                ${data.maintenance_log.map(m => `
                                    <tr>
                                        <td>${new Date(m.timestamp).toLocaleString()}</td>
                                        <td><span class="badge badge-secondary">${m.action_type}</span></td>
                                        <td>${m.description}</td>
                                        <td>${m.performed_by || 'System'}</td>
                                    </tr>
                                `).join('')}
                            </tbody>
                        </table>
                    `}
                </div>
                
                ${data.tracking ? `
                    <div class="tab-content" id="tab-tracking">
                        <div class="info-grid">
                            <div class="info-item">
                                <strong>First Seen:</strong> ${new Date(data.tracking.first_seen).toLocaleString()}
                            </div>
                            <div class="info-item">
                                <strong>Last Seen:</strong> ${new Date(data.tracking.last_seen).toLocaleString()}
                            </div>
                            <div class="info-item">
                                <strong>Status:</strong> <span class="badge badge-${data.tracking.status === 'healthy' ? 'success' : 'warning'}">${data.tracking.status}</span>
                            </div>
                            ${data.tracking.notes ? `
                                <div class="info-item full-width">
                                    <strong>Notes:</strong><br>${data.tracking.notes}
                                </div>
                            ` : ''}
                        </div>
                    </div>
                ` : ''}
            </div>
            
            <div class="button-group" style="margin-top:1.5rem;">
                <button class="btn-secondary" onclick="closeModal()">Close</button>
            </div>
        `;
        
        modal.classList.add('active');
        
        // Tab switching
        document.querySelectorAll('.tab-btn').forEach(btn => {
            btn.addEventListener('click', () => {
                document.querySelectorAll('.tab-btn').forEach(b => b.classList.remove('active'));
                document.querySelectorAll('.tab-content').forEach(c => c.classList.remove('active'));
                btn.classList.add('active');
                document.getElementById('tab-' + btn.dataset.tab).classList.add('active');
            });
        });
        
    } catch (e) {
        alert('Failed to load disk details: ' + e.message);
    }
}

function showDiskActions(diskPath) {
    const modal = document.getElementById('modal');
    const body = document.getElementById('modal-body');
    
    body.innerHTML = `
        <h3>Disk Actions: ${diskPath}</h3>
        
        <div class="action-grid">
            <div class="action-card" onclick="runSmartTest('${diskPath}', 'short')">
                <h4>Run Short Test</h4>
                <p>Quick SMART self-test (~2 minutes)</p>
            </div>
            
            <div class="action-card" onclick="runSmartTest('${diskPath}', 'long')">
                <h4>Run Long Test</h4>
                <p>Comprehensive test (1-2 hours)</p>
            </div>
            
            <div class="action-card" onclick="addDiskNote('${diskPath}')">
                <h4>Add Note</h4>
                <p>Add maintenance or observation note</p>
            </div>
            
            <div class="action-card" onclick="changeDiskStatus('${diskPath}')">
                <h4>Update Status</h4>
                <p>Manually change disk status</p>
            </div>
            
            <div class="action-card warning" onclick="markDiskReplacement('${diskPath}')">
                <h4>Mark as Replaced</h4>
                <p>Log disk replacement</p>
            </div>
        </div>
        
        <div class="button-group" style="margin-top:1.5rem;">
            <button class="btn-secondary" onclick="closeModal()">Close</button>
        </div>
    `;
    
    modal.classList.add('active');
}

async function runSmartTest(diskPath, testType) {
    if (!confirm(`Start ${testType} SMART test on ${diskPath}?\n\nThis will take ${testType === 'short' ? '~2 minutes' : '1-2 hours'}.`)) {
        return;
    }
    
    try {
        const res = await fetch('/api/storage/disk-health.php', {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({
                action: 'run_test',
                disk: diskPath,
                test_type: testType
            })
        });
        
        const data = await res.json();
        
        if (data.success) {
            alert(`SMART test started successfully.\n\nCheck test progress with:\nsudo smartctl -a ${diskPath}`);
            closeModal();
        } else {
            alert('Error: ' + data.error);
        }
    } catch (e) {
        alert('Failed: ' + e.message);
    }
}

function addDiskNote(diskPath) {
    const modal = document.getElementById('modal');
    const body = document.getElementById('modal-body');
    
    body.innerHTML = `
        <h3>Add Note for ${diskPath}</h3>
        
        <form id="add-note-form">
            <div class="form-group">
                <label>Note *</label>
                <textarea name="note" rows="5" required 
                    placeholder="E.g., Replaced fan, observed clicking noise, etc."></textarea>
            </div>
            
            <div class="button-group">
                <button type="submit" class="btn-primary">Save Note</button>
                <button type="button" class="btn-secondary" onclick="showDiskActions('${diskPath}')">Back</button>
            </div>
        </form>
    `;
    
    modal.classList.add('active');
    
    document.getElementById('add-note-form').onsubmit = async (e) => {
        e.preventDefault();
        const fd = new FormData(e.target);
        
        try {
            const res = await fetch('/api/storage/disk-health.php', {
                method: 'POST',
                headers: {'Content-Type': 'application/json'},
                body: JSON.stringify({
                    action: 'add_note',
                    disk: diskPath,
                    note: fd.get('note')
                })
            });
            
            const data = await res.json();
            
            if (data.success) {
                showDiskActions(diskPath);
            } else {
                alert('Error: ' + data.error);
            }
        } catch (e) {
            alert('Failed: ' + e.message);
        }
    };
}

function changeDiskStatus(diskPath) {
    const modal = document.getElementById('modal');
    const body = document.getElementById('modal-body');
    
    body.innerHTML = `
        <h3>Update Status for ${diskPath}</h3>
        
        <form id="status-form">
            <div class="form-group">
                <label>Status *</label>
                <select name="status" required>
                    <option value="healthy">✅ Healthy</option>
                    <option value="warning">⚠️ Warning</option>
                    <option value="critical">🔴 Critical</option>
                    <option value="failing">❌ Failing</option>
                    <option value="replaced">♻️ Replaced</option>
                </select>
            </div>
            
            <div class="info-box">
                Select the appropriate status based on SMART data, observations, or maintenance requirements.
            </div>
            
            <div class="button-group">
                <button type="submit" class="btn-primary">Update Status</button>
                <button type="button" class="btn-secondary" onclick="showDiskActions('${diskPath}')">Back</button>
            </div>
        </form>
    `;
    
    modal.classList.add('active');
    
    document.getElementById('status-form').onsubmit = async (e) => {
        e.preventDefault();
        const fd = new FormData(e.target);
        
        try {
            const res = await fetch('/api/storage/disk-health.php', {
                method: 'POST',
                headers: {'Content-Type': 'application/json'},
                body: JSON.stringify({
                    action: 'update_status',
                    disk: diskPath,
                    status: fd.get('status')
                })
            });
            
            const data = await res.json();
            
            if (data.success) {
                loadDiskHealth();
                closeModal();
            } else {
                alert('Error: ' + data.error);
            }
        } catch (e) {
            alert('Failed: ' + e.message);
        }
    };
}

function markDiskReplacement(diskPath) {
    const modal = document.getElementById('modal');
    const body = document.getElementById('modal-body');
    
    body.innerHTML = `
        <h3>Mark Disk as Replaced</h3>
        <p style="color:#FF9800;margin-bottom:1rem;">⚠️ This will mark ${diskPath} as replaced in the tracking system.</p>
        
        <form id="replacement-form">
            <div class="form-group">
                <label>New Disk Serial Number (optional)</label>
                <input type="text" name="new_serial" placeholder="Serial number of replacement disk">
            </div>
            
            <div class="button-group">
                <button type="submit" class="btn-danger">Mark as Replaced</button>
                <button type="button" class="btn-secondary" onclick="showDiskActions('${diskPath}')">Back</button>
            </div>
        </form>
    `;
    
    modal.classList.add('active');
    
    document.getElementById('replacement-form').onsubmit = async (e) => {
        e.preventDefault();
        const fd = new FormData(e.target);
        
        if (!confirm('Are you sure you want to mark this disk as replaced?')) {
            return;
        }
        
        try {
            const res = await fetch('/api/storage/disk-health.php', {
                method: 'POST',
                headers: {'Content-Type': 'application/json'},
                body: JSON.stringify({
                    action: 'mark_replacement',
                    disk: diskPath,
                    new_serial: fd.get('new_serial')
                })
            });
            
            const data = await res.json();
            
            if (data.success) {
                loadDiskHealth();
                loadNotifications(); // Refresh notifications
                closeModal();
            } else {
                alert('Error: ' + data.error);
            }
        } catch (e) {
            alert('Failed: ' + e.message);
        }
    };
}

function refreshDiskHealth() {
    loadDiskHealth();
}

// ========================================
// NOTIFICATIONS SYSTEM
// ========================================

async function loadNotifications() {
    try {
        const res = await fetch('/api/system/notifications.php?action=list');
        const data = await res.json();
        
        if (data.success) {
            displayNotifications(data.notifications);
            updateNotificationCount(data.notifications.filter(n => !n.read).length);
        }
    } catch (e) {
        console.error('Failed to load notifications:', e);
    }
}

function displayNotifications(notifications) {
    const list = document.getElementById('notification-list');
    if (!list) return;
    
    if (notifications.length === 0) {
        list.innerHTML = '<div class="notification-placeholder"><p>No notifications</p></div>';
        return;
    }
    
    list.innerHTML = notifications.map(n => {
        const iconMap = {
            'info': 'ℹ️',
            'success': '✅',
            'warning': '⚠️',
            'error': '🔴'
        };
        
        const priorityLabel = ['Low', 'Normal', 'High', 'Critical'][n.priority] || 'Normal';
        
        return `
            <div class="notification-item ${n.read ? 'read' : 'unread'} ${n.type}" data-id="${n.id}">
                <div class="notification-icon">${iconMap[n.type]}</div>
                <div class="notification-content">
                    <div class="notification-title">${n.title}</div>
                    <div class="notification-message">${n.message}</div>
                    <div class="notification-meta">
                        <span class="notification-time">${new Date(n.created_at).toLocaleString()}</span>
                        ${n.category ? `<span class="badge badge-secondary">${n.category}</span>` : ''}
                        ${n.priority > 1 ? `<span class="badge badge-warning">${priorityLabel}</span>` : ''}
                    </div>
                </div>
                <div class="notification-actions">
                    ${!n.read ? `<button class="btn-link" onclick="markNotificationRead(${n.id})">Mark Read</button>` : ''}
                    <button class="btn-link" onclick="dismissNotification(${n.id})">Dismiss</button>
                </div>
            </div>
        `;
    }).join('');
}

function updateNotificationCount(count) {
    const badge = document.getElementById('notification-count');
    if (badge) {
        if (count > 0) {
            badge.textContent = count > 99 ? '99+' : count;
            badge.style.display = 'block';
        } else {
            badge.style.display = 'none';
        }
    }
}

function toggleNotifications() {
    const center = document.getElementById('notification-center');
    if (center) {
        center.classList.toggle('active');
        if (center.classList.contains('active')) {
            loadNotifications();
        }
    }
}

async function markNotificationRead(id) {
    try {
        const res = await fetch('/api/system/notifications.php', {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({
                action: 'mark_read',
                id: id
            })
        });
        
        const data = await res.json();
        
        if (data.success) {
            loadNotifications();
        }
    } catch (e) {
        console.error('Failed to mark notification read:', e);
    }
}

async function markAllNotificationsRead() {
    try {
        const res = await fetch('/api/system/notifications.php', {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({
                action: 'mark_all_read'
            })
        });
        
        const data = await res.json();
        
        if (data.success) {
            loadNotifications();
        }
    } catch (e) {
        console.error('Failed to mark all notifications read:', e);
    }
}

async function dismissNotification(id) {
    try {
        const res = await fetch('/api/system/notifications.php', {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({
                action: 'dismiss',
                id: id
            })
        });
        
        const data = await res.json();
        
        if (data.success) {
            loadNotifications();
        }
    } catch (e) {
        console.error('Failed to dismiss notification:', e);
    }
}

// Poll for new notifications every 30 seconds
setInterval(() => {
    fetch('/api/system/notifications.php?action=unread_count')
        .then(res => res.json())
        .then(data => {
            if (data.success) {
                updateNotificationCount(data.count);
            }
        })
        .catch(e => console.error('Failed to check notifications:', e));
}, 30000);

// Initial notification count load
loadNotifications();

// ========================================
// UPS/USV MONITORING
// ========================================

async function loadUPS() {
    try {
        const res = await fetch('/api/system/ups.php?action=status');
        const data = await res.json();
        
        if (data.success) {
            displayUPS(data.ups_devices, data.nut_installed);
        }
    } catch (e) {
        console.error('Failed to load UPS:', e);
    }
}

function displayUPS(devices, nutInstalled) {
    const container = document.getElementById('ups-status-container');
    if (!container) return;
    
    if (!nutInstalled) {
        container.innerHTML = `
            <div class="info-box warning">
                <h3>⚠️ NUT (Network UPS Tools) Not Installed</h3>
                <p>To enable UPS monitoring, install NUT:</p>
                <pre>sudo apt install nut</pre>
                <p>Then configure your UPS in <code>/etc/nut/ups.conf</code></p>
            </div>
        `;
        return;
    }
    
    if (!devices || devices.length === 0) {
        container.innerHTML = `
            <div class="info-box">
                <h3>No UPS Detected</h3>
                <p>NUT is installed but no UPS devices are configured.</p>
                <p>Configure your UPS in <code>/etc/nut/ups.conf</code> and restart NUT service.</p>
            </div>
        `;
        return;
    }
    
    container.innerHTML = devices.map(ups => {
        const statusColor = {
            'OL': 'success',
            'ONLINE': 'success',
            'OB': 'warning',
            'ONBATT': 'warning',
            'LB': 'danger',
            'LOWBATT': 'danger',
            'OFFLINE': 'danger'
        }[ups.status] || 'secondary';
        
        const batteryColor = 
            ups.battery_charge > 80 ? 'success' :
            ups.battery_charge > 50 ? 'warning' :
            ups.battery_charge > 20 ? 'warning' : 'danger';
        
        const runtimeMinutes = ups.battery_runtime ? Math.floor(ups.battery_runtime / 60) : null;
        
        return `
            <div class="card" style="margin-bottom: 1rem;">
                <div class="card-header">
                    <div>
                        <h3>${ups.name}</h3>
                        <span class="badge badge-${statusColor}">${ups.status}</span>
                        ${ups.model ? `<span class="badge badge-secondary">${ups.model}</span>` : ''}
                    </div>
                </div>
                <div class="card-content">
                    <div class="ups-info-grid" style="display:grid;grid-template-columns:repeat(auto-fit,minmax(200px,1fr));gap:1rem;">
                        <div>
                            <strong>Battery Charge:</strong><br>
                            <span class="badge badge-${batteryColor}" style="font-size:1.2rem;">
                                ${ups.battery_charge !== null ? ups.battery_charge + '%' : 'N/A'}
                            </span>
                        </div>
                        <div>
                            <strong>Runtime Remaining:</strong><br>
                            <span style="font-size:1.1rem;">
                                ${runtimeMinutes !== null ? runtimeMinutes + ' minutes' : 'N/A'}
                            </span>
                        </div>
                        <div>
                            <strong>Load:</strong><br>
                            <span style="font-size:1.1rem;">
                                ${ups.load !== null ? ups.load + '%' : 'N/A'}
                            </span>
                        </div>
                        <div>
                            <strong>Input Voltage:</strong><br>
                            <span style="font-size:1.1rem;">
                                ${ups.input_voltage !== null ? ups.input_voltage + 'V' : 'N/A'}
                            </span>
                        </div>
                        <div>
                            <strong>Output Voltage:</strong><br>
                            <span style="font-size:1.1rem;">
                                ${ups.output_voltage !== null ? ups.output_voltage + 'V' : 'N/A'}
                            </span>
                        </div>
                        ${ups.temperature !== null ? `
                            <div>
                                <strong>Temperature:</strong><br>
                                <span style="font-size:1.1rem;">${ups.temperature}°C</span>
                            </div>
                        ` : ''}
                    </div>
                    
                    ${ups.status === 'ONBATT' || ups.status === 'OB' ? `
                        <div class="info-box warning" style="margin-top:1rem;">
                            ⚠️ <strong>System Running on Battery Power</strong><br>
                            Ensure AC power is restored soon to prevent shutdown.
                        </div>
                    ` : ''}
                    
                    ${ups.status === 'LOWBATT' || ups.status === 'LB' || (ups.battery_charge && ups.battery_charge < 20) ? `
                        <div class="info-box danger" style="margin-top:1rem;">
                            🔴 <strong>Critical Battery Level</strong><br>
                            System shutdown is imminent. Save your work immediately!
                        </div>
                    ` : ''}
                </div>
            </div>
        `;
    }).join('');
}

function refreshUPS() {
    loadUPS();
}

function configureUPSShutdown() {
    const modal = document.getElementById('modal');
    const body = document.getElementById('modal-body');
    
    body.innerHTML = `
        <h3>Configure UPS Shutdown</h3>
        
        <form id="ups-shutdown-form">
            <div class="form-group">
                <label>Shutdown when battery level drops below:</label>
                <input type="number" name="battery_level" value="20" min="5" max="50" required>
                <span>%</span>
            </div>
            
            <div class="form-group">
                <label>Shutdown when runtime is less than:</label>
                <input type="number" name="runtime" value="300" min="60" max="600" required>
                <span>seconds (5 minutes = 300 seconds)</span>
            </div>
            
            <div class="info-box">
                System will automatically initiate a graceful shutdown when either condition is met.
                This ensures ZFS pools are properly flushed and containers are stopped safely.
            </div>
            
            <div class="button-group" style="margin-top:1.5rem;">
                <button type="submit" class="btn-primary">Save Settings</button>
                <button type="button" class="btn-secondary" onclick="closeModal()">Cancel</button>
            </div>
        </form>
    `;
    
    modal.classList.add('active');
    
    document.getElementById('ups-shutdown-form').onsubmit = async (e) => {
        e.preventDefault();
        const fd = new FormData(e.target);
        
        try {
            const res = await fetch('/api/system/ups.php', {
                method: 'POST',
                headers: {'Content-Type': 'application/json'},
                body: JSON.stringify({
                    action: 'configure_shutdown',
                    battery_level: parseInt(fd.get('battery_level')),
                    runtime: parseInt(fd.get('runtime'))
                })
            });
            
            const data = await res.json();
            
            if (data.success) {
                closeModal();
                alert('Shutdown settings updated successfully');
            } else {
                alert('Error: ' + data.error);
            }
        } catch (e) {
            alert('Failed: ' + e.message);
        }
    };
}

// ========================================
// SNAPSHOT AUTOMATION
// ========================================

async function loadSnapshotSchedules() {
    try {
        const res = await fetch('/api/storage/snapshots.php?action=list');
        const data = await res.json();
        
        if (data.success) {
            displaySnapshotSchedules(data.schedules);
        }
    } catch (e) {
        console.error('Failed to load snapshot schedules:', e);
    }
}

function displaySnapshotSchedules(schedules) {
    const container = document.getElementById('snapshot-schedules-list');
    if (!container) return;
    
    if (schedules.length === 0) {
        container.innerHTML = `
            <div class="info-box">
                <p>No automatic snapshot schedules configured.</p>
                <p>Create a schedule to automatically protect your data against accidental deletion or corruption.</p>
            </div>
        `;
        return;
    }
    
    container.innerHTML = schedules.map(schedule => {
        const statusColor = schedule.enabled ? 'success' : 'secondary';
        const frequencyLabel = {
            'hourly': '🕐 Hourly',
            'daily': '📅 Daily',
            'weekly': '📆 Weekly',
            'monthly': '📆 Monthly'
        }[schedule.frequency] || schedule.frequency;
        
        return `
            <div class="card">
                <div class="card-header">
                    <div>
                        <h3>${schedule.dataset_path}</h3>
                        <span class="badge badge-primary">${frequencyLabel}</span>
                        <span class="badge badge-${statusColor}">${schedule.enabled ? 'Enabled' : 'Disabled'}</span>
                        <span class="badge badge-info">${schedule.current_count}/${schedule.keep_count} snapshots</span>
                    </div>
                    <div class="button-group">
                        <button class="btn-info btn-sm" onclick="runSnapshotNow(${schedule.id})">Run Now</button>
                        <button class="btn-secondary btn-sm" onclick="toggleSnapshotSchedule(${schedule.id})">
                            ${schedule.enabled ? 'Disable' : 'Enable'}
                        </button>
                        <button class="btn-danger btn-sm" onclick="deleteSnapshotSchedule(${schedule.id})">Delete</button>
                    </div>
                </div>
                <div class="card-content">
                    <p><strong>Retention:</strong> Keep ${schedule.keep_count} ${schedule.frequency} snapshots</p>
                    <p><strong>Name Prefix:</strong> ${schedule.name_prefix}</p>
                    ${schedule.last_run ? `<p><strong>Last Run:</strong> ${new Date(schedule.last_run).toLocaleString()}</p>` : ''}
                </div>
            </div>
        `;
    }).join('');
}

function showCreateSnapshotSchedule() {
    const modal = document.getElementById('modal');
    const body = document.getElementById('modal-body');
    
    body.innerHTML = `
        <h3>Create Snapshot Schedule</h3>
        
        <form id="snapshot-schedule-form">
            <div class="form-group">
                <label>Dataset Path *</label>
                <input type="text" name="dataset_path" placeholder="e.g., tank/data" required>
            </div>
            
            <div class="form-group">
                <label>Frequency *</label>
                <select name="frequency" required>
                    <option value="hourly">Hourly</option>
                    <option value="daily" selected>Daily</option>
                    <option value="weekly">Weekly</option>
                    <option value="monthly">Monthly</option>
                </select>
            </div>
            
            <div class="form-group">
                <label>Keep Count *</label>
                <input type="number" name="keep_count" value="7" min="1" max="365" required>
                <small>Number of snapshots to retain</small>
            </div>
            
            <div class="info-box">
                <strong>Recommended Retention Policies:</strong><br>
                • Hourly: Keep 24 (1 day)<br>
                • Daily: Keep 7 (1 week)<br>
                • Weekly: Keep 4 (1 month)<br>
                • Monthly: Keep 12 (1 year)
            </div>
            
            <div class="button-group" style="margin-top:1.5rem;">
                <button type="submit" class="btn-primary">Create Schedule</button>
                <button type="button" class="btn-secondary" onclick="closeModal()">Cancel</button>
            </div>
        </form>
    `;
    
    modal.classList.add('active');
    
    document.getElementById('snapshot-schedule-form').onsubmit = async (e) => {
        e.preventDefault();
        const fd = new FormData(e.target);
        
        try {
            const res = await fetch('/api/storage/snapshots.php', {
                method: 'POST',
                headers: {'Content-Type': 'application/json'},
                body: JSON.stringify({
                    action: 'create_schedule',
                    dataset_path: fd.get('dataset_path'),
                    frequency: fd.get('frequency'),
                    keep_count: parseInt(fd.get('keep_count'))
                })
            });
            
            const data = await res.json();
            
            if (data.success) {
                closeModal();
                loadSnapshotSchedules();
                loadNotifications();
            } else {
                alert('Error: ' + data.error);
            }
        } catch (e) {
            alert('Failed: ' + e.message);
        }
    };
}

async function runSnapshotNow(scheduleId) {
    if (!confirm('Create snapshot now for this schedule?')) return;
    
    try {
        const res = await fetch('/api/storage/snapshots.php', {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({
                action: 'run_now',
                id: scheduleId
            })
        });
        
        const data = await res.json();
        
        if (data.success) {
            alert('Snapshot created: ' + data.snapshot);
            loadSnapshotSchedules();
        } else {
            alert('Error: ' + data.error);
        }
    } catch (e) {
        alert('Failed: ' + e.message);
    }
}

async function toggleSnapshotSchedule(scheduleId) {
    try {
        const res = await fetch('/api/storage/snapshots.php', {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({
                action: 'toggle_schedule',
                id: scheduleId
            })
        });
        
        const data = await res.json();
        
        if (data.success) {
            loadSnapshotSchedules();
        } else {
            alert('Error: ' + data.error);
        }
    } catch (e) {
        alert('Failed: ' + e.message);
    }
}

async function deleteSnapshotSchedule(scheduleId) {
    if (!confirm('Delete this snapshot schedule? Existing snapshots will NOT be deleted.')) return;
    
    try {
        const res = await fetch('/api/storage/snapshots.php', {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({
                action: 'delete_schedule',
                id: scheduleId
            })
        });
        
        const data = await res.json();
        
        if (data.success) {
            loadSnapshotSchedules();
        } else {
            alert('Error: ' + data.error);
        }
    } catch (e) {
        alert('Failed: ' + e.message);
    }
}

function refreshSnapshots() {
    loadSnapshotSchedules();
}

// ========================================
// SYSTEM LOG VIEWER
// ========================================

async function loadLogs() {
    const logType = document.getElementById('log-type-select').value;
    const lines = document.getElementById('log-lines').value;
    const serviceSelect = document.getElementById('log-service-select');
    const viewer = document.getElementById('log-viewer');
    
    if (!viewer) return;
    
    // Show/hide service selector
    if (logType === 'service') {
        serviceSelect.style.display = 'inline-block';
    } else {
        serviceSelect.style.display = 'none';
    }
    
    viewer.innerHTML = '<div class="info-box">Loading logs...</div>';
    
    try {
        let url = `/api/system/logs.php?action=${logType}&lines=${lines}`;
        
        if (logType === 'service') {
            const service = serviceSelect.value;
            url += `&service=${service}`;
        }
        
        const res = await fetch(url);
        const data = await res.json();
        
        if (data.success) {
            displayLogs(data);
        } else {
            viewer.innerHTML = `<div class="info-box danger">Error: ${data.error}</div>`;
        }
    } catch (e) {
        viewer.innerHTML = `<div class="info-box danger">Failed to load logs: ${e.message}</div>`;
    }
}

function displayLogs(data) {
    const viewer = document.getElementById('log-viewer');
    if (!viewer) return;
    
    if (data.log_type === 'dplaneos') {
        // Display audit log as table
        if (data.entries.length === 0) {
            viewer.innerHTML = '<div class="info-box">No audit log entries found</div>';
            return;
        }
        
        viewer.innerHTML = `
            <table class="data-table">
                <thead>
                    <tr>
                        <th>Timestamp</th>
                        <th>User</th>
                        <th>Action</th>
                        <th>Resource</th>
                        <th>Details</th>
                        <th>IP</th>
                    </tr>
                </thead>
                <tbody>
                    ${data.entries.map(entry => `
                        <tr>
                            <td>${entry.timestamp}</td>
                            <td>${entry.user}</td>
                            <td><span class="badge badge-secondary">${entry.action}</span></td>
                            <td>${entry.resource}</td>
                            <td>${entry.details || '-'}</td>
                            <td>${entry.ip || '-'}</td>
                        </tr>
                    `).join('')}
                </tbody>
            </table>
        `;
    } else {
        // Display as terminal output
        if (!data.lines || data.lines.length === 0) {
            viewer.innerHTML = '<div class="info-box">No log entries found</div>';
            return;
        }
        
        viewer.innerHTML = `
            <pre class="log-output">${data.lines.join('\n')}</pre>
        `;
    }
}

// Initialize logs page when loaded
if (document.getElementById('log-type-select')) {
    document.getElementById('log-type-select').addEventListener('change', loadLogs);
}

// ========================================
// USER MANAGEMENT
// ========================================

async function loadUsers() {
    try {
        const res = await fetch('/api/system/users.php?action=list');
        const data = await res.json();
        
        if (data.success) {
            displayUsers(data.users);
        }
    } catch (e) {
        console.error('Failed to load users:', e);
    }
}

function displayUsers(users) {
    const container = document.getElementById('users-list');
    if (!container) return;
    
    if (users.length === 0) {
        container.innerHTML = '<div class="info-box">No users found</div>';
        return;
    }
    
    container.innerHTML = users.map(user => {
        const isAdmin = user.username === 'admin';
        
        return `
            <div class="card">
                <div class="card-header">
                    <div>
                        <h3>${user.username}</h3>
                        ${isAdmin ? '<span class="badge badge-danger">ADMIN</span>' : '<span class="badge badge-secondary">USER</span>'}
                    </div>
                    <div class="button-group">
                        <button class="btn-info btn-sm" onclick="changeUserPassword(${user.id}, '${user.username}')">Change Password</button>
                        ${!isAdmin ? `
                            <button class="btn-secondary btn-sm" onclick="editUser(${user.id})">Edit</button>
                            <button class="btn-danger btn-sm" onclick="deleteUser(${user.id}, '${user.username}')">Delete</button>
                        ` : ''}
                    </div>
                </div>
                <div class="card-content">
                    <p><strong>Email:</strong> ${user.email || 'Not set'}</p>
                    <p><strong>Created:</strong> ${new Date(user.created_at).toLocaleString()}</p>
                    <p><strong>User ID:</strong> ${user.id}</p>
                </div>
            </div>
        `;
    }).join('');
}

function showCreateUser() {
    const modal = document.getElementById('modal');
    const body = document.getElementById('modal-body');
    
    body.innerHTML = `
        <h3>Create New User</h3>
        
        <form id="create-user-form">
            <div class="form-group">
                <label>Username *</label>
                <input type="text" name="username" required 
                    pattern="[a-zA-Z0-9_-]{3,32}"
                    placeholder="e.g., john_doe"
                    title="3-32 characters: letters, numbers, dash, underscore only">
                <small>3-32 characters (alphanumeric, dash, underscore only)</small>
            </div>
            
            <div class="form-group">
                <label>Password *</label>
                <input type="password" name="password" required minlength="8"
                    placeholder="At least 8 characters">
                <small>Minimum 8 characters</small>
            </div>
            
            <div class="form-group">
                <label>Confirm Password *</label>
                <input type="password" name="password_confirm" required minlength="8">
            </div>
            
            <div class="form-group">
                <label>Email (optional)</label>
                <input type="email" name="email" placeholder="user@example.com">
            </div>
            
            <div class="info-box">
                <strong>User will have access to:</strong>
                <ul style="margin-left:1.5rem;margin-top:0.5rem;">
                    <li>D-PlaneOS dashboard</li>
                    <li>SMB/CIFS network shares</li>
                    <li>NFS shares (if configured)</li>
                </ul>
            </div>
            
            <div class="button-group" style="margin-top:1.5rem;">
                <button type="submit" class="btn-primary">Create User</button>
                <button type="button" class="btn-secondary" onclick="closeModal()">Cancel</button>
            </div>
        </form>
    `;
    
    modal.classList.add('active');
    
    document.getElementById('create-user-form').onsubmit = async (e) => {
        e.preventDefault();
        const fd = new FormData(e.target);
        
        // Validate passwords match
        if (fd.get('password') !== fd.get('password_confirm')) {
            alert('Passwords do not match!');
            return;
        }
        
        try {
            const res = await fetch('/api/system/users.php', {
                method: 'POST',
                headers: {'Content-Type': 'application/json'},
                body: JSON.stringify({
                    action: 'create',
                    username: fd.get('username'),
                    password: fd.get('password'),
                    email: fd.get('email')
                })
            });
            
            const data = await res.json();
            
            if (data.success) {
                closeModal();
                loadUsers();
                loadNotifications();
            } else {
                alert('Error: ' + data.error);
            }
        } catch (e) {
            alert('Failed: ' + e.message);
        }
    };
}

function editUser(userId) {
    // Load user data and show edit modal
    fetch(`/api/system/users.php?action=get&id=${userId}`)
        .then(res => res.json())
        .then(data => {
            if (!data.success) {
                alert('Error: ' + data.error);
                return;
            }
            
            const user = data.user;
            const modal = document.getElementById('modal');
            const body = document.getElementById('modal-body');
            
            body.innerHTML = `
                <h3>Edit User: ${user.username}</h3>
                
                <form id="edit-user-form">
                    <div class="form-group">
                        <label>Username</label>
                        <input type="text" value="${user.username}" disabled>
                        <small>Username cannot be changed</small>
                    </div>
                    
                    <div class="form-group">
                        <label>Email</label>
                        <input type="email" name="email" value="${user.email || ''}" placeholder="user@example.com">
                    </div>
                    
                    <div class="button-group" style="margin-top:1.5rem;">
                        <button type="submit" class="btn-primary">Save Changes</button>
                        <button type="button" class="btn-secondary" onclick="closeModal()">Cancel</button>
                    </div>
                </form>
            `;
            
            modal.classList.add('active');
            
            document.getElementById('edit-user-form').onsubmit = async (e) => {
                e.preventDefault();
                const fd = new FormData(e.target);
                
                try {
                    const res = await fetch('/api/system/users.php', {
                        method: 'POST',
                        headers: {'Content-Type': 'application/json'},
                        body: JSON.stringify({
                            action: 'update',
                            id: userId,
                            email: fd.get('email')
                        })
                    });
                    
                    const data = await res.json();
                    
                    if (data.success) {
                        closeModal();
                        loadUsers();
                    } else {
                        alert('Error: ' + data.error);
                    }
                } catch (e) {
                    alert('Failed: ' + e.message);
                }
            };
        });
}

function changeUserPassword(userId, username) {
    const modal = document.getElementById('modal');
    const body = document.getElementById('modal-body');
    
    body.innerHTML = `
        <h3>Change Password: ${username}</h3>
        
        <form id="password-form">
            <div class="form-group">
                <label>New Password *</label>
                <input type="password" name="new_password" required minlength="8"
                    placeholder="At least 8 characters">
            </div>
            
            <div class="form-group">
                <label>Confirm New Password *</label>
                <input type="password" name="password_confirm" required minlength="8">
            </div>
            
            <div class="info-box warning">
                <strong>⚠️ Note:</strong> This will change the password for both dashboard and SMB/NFS access.
            </div>
            
            <div class="button-group" style="margin-top:1.5rem;">
                <button type="submit" class="btn-primary">Change Password</button>
                <button type="button" class="btn-secondary" onclick="closeModal()">Cancel</button>
            </div>
        </form>
    `;
    
    modal.classList.add('active');
    
    document.getElementById('password-form').onsubmit = async (e) => {
        e.preventDefault();
        const fd = new FormData(e.target);
        
        // Validate passwords match
        if (fd.get('new_password') !== fd.get('password_confirm')) {
            alert('Passwords do not match!');
            return;
        }
        
        try {
            const res = await fetch('/api/system/users.php', {
                method: 'POST',
                headers: {'Content-Type': 'application/json'},
                body: JSON.stringify({
                    action: 'change_password',
                    id: userId,
                    new_password: fd.get('new_password')
                })
            });
            
            const data = await res.json();
            
            if (data.success) {
                alert('Password changed successfully');
                closeModal();
            } else {
                alert('Error: ' + data.error);
            }
        } catch (e) {
            alert('Failed: ' + e.message);
        }
    };
}

async function deleteUser(userId, username) {
    if (!confirm(`Delete user '${username}'?\n\nThis will remove dashboard access and SMB/NFS access.\nUser's home directory will be preserved.`)) {
        return;
    }
    
    try {
        const res = await fetch('/api/system/users.php', {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({
                action: 'delete',
                id: userId
            })
        });
        
        const data = await res.json();
        
        if (data.success) {
            loadUsers();
            loadNotifications();
        } else {
            alert('Error: ' + data.error);
        }
    } catch (e) {
        alert('Failed: ' + e.message);
    }
}

// ========================================
// FILE BROWSER
// ========================================

let currentPath = '/mnt';

async function loadFiles() {
    const container = document.getElementById('files-list');
    if (!container) return;
    
    container.innerHTML = '<div class="loading">Loading files...</div>';
    
    try {
        const res = await fetch(`/api/storage/files.php?action=list&path=${encodeURIComponent(currentPath)}`);
        const data = await res.json();
        
        if (data.success) {
            displayFiles(data);
        } else {
            container.innerHTML = `<p class="error">${data.error}</p>`;
        }
    } catch (e) {
        container.innerHTML = `<p class="error">Failed to load files: ${e.message}</p>`;
    }
}

function displayFiles(data) {
    const container = document.getElementById('files-list');
    const breadcrumb = document.getElementById('files-breadcrumb');
    
    // Update breadcrumb
    if (breadcrumb) {
        breadcrumb.textContent = `📁 ${data.path}`;
    }
    
    if (data.items.length === 0) {
        container.innerHTML = '<p>Empty directory</p>';
        return;
    }
    
    let html = '<table class="files-table"><thead><tr><th>Name</th><th>Size</th><th>Modified</th><th>Permissions</th><th>Owner</th><th>Actions</th></tr></thead><tbody>';
    
    // Add parent directory link if not at root
    if (data.path !== '/mnt') {
        const parent = data.path.substring(0, data.path.lastIndexOf('/')) || '/mnt';
        html += `<tr onclick="navigateToFolder('${parent}')"><td>📁 ..</td><td>-</td><td>-</td><td>-</td><td>-</td><td>-</td></tr>`;
    }
    
    data.items.forEach(item => {
        const icon = item.type === 'directory' ? '📁' : '📄';
        const onclick = item.type === 'directory' ? `onclick="navigateToFolder('${item.path}')"` : '';
        
        html += `<tr ${onclick}>
            <td>${icon} ${item.name}</td>
            <td>${item.type === 'file' ? item.size_human : '-'}</td>
            <td>${item.modified_human}</td>
            <td>${item.permissions}</td>
            <td>${item.owner}:${item.group}</td>
            <td>
                ${item.type === 'file' ? `<button class="btn-sm" onclick="event.stopPropagation();downloadFile('${item.path}')">Download</button>` : ''}
                <button class="btn-sm btn-danger" onclick="event.stopPropagation();deleteFile('${item.path}', '${item.type}')">Delete</button>
            </td>
        </tr>`;
    });
    
    html += '</tbody></table>';
    container.innerHTML = html;
}

function navigateToFolder(path) {
    currentPath = path;
    loadFiles();
}

async function downloadFile(path) {
    window.location.href = `/api/storage/files.php?action=download&path=${encodeURIComponent(path)}`;
}

async function deleteFile(path, type) {
    if (!confirm(`Delete this ${type}?`)) return;
    
    try {
        const res = await fetch('/api/storage/files.php', {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({
                action: 'delete',
                path: path,
                recursive: type === 'directory'
            })
        });
        
        const data = await res.json();
        
        if (data.success) {
            loadFiles();
        } else {
            alert('Error: ' + data.error);
        }
    } catch (e) {
        alert('Failed: ' + e.message);
    }
}

async function createFolder() {
    const name = prompt('Folder name:');
    if (!name) return;
    
    try {
        const res = await fetch('/api/storage/files.php', {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({
                action: 'create_folder',
                path: currentPath,
                name: name
            })
        });
        
        const data = await res.json();
        
        if (data.success) {
            loadFiles();
        } else {
            alert('Error: ' + data.error);
        }
    } catch (e) {
        alert('Failed: ' + e.message);
    }
}

// ========================================
// SERVICE CONTROL
// ========================================

async function loadServices() {
    const container = document.getElementById('services-list');
    if (!container) return;
    
    container.innerHTML = '<div class="loading">Loading services...</div>';
    
    try {
        const res = await fetch('/api/system/services.php?action=list');
        const data = await res.json();
        
        if (data.success) {
            displayServices(data.services);
        } else {
            container.innerHTML = `<p class="error">${data.error}</p>`;
        }
    } catch (e) {
        container.innerHTML = `<p class="error">Failed to load services: ${e.message}</p>`;
    }
}

function displayServices(services) {
    const container = document.getElementById('services-list');
    
    let html = '<table class="services-table"><thead><tr><th>Service</th><th>Description</th><th>Status</th><th>Enabled</th><th>Memory</th><th>Actions</th></tr></thead><tbody>';
    
    services.forEach(service => {
        const statusBadge = service.active ? '<span class="badge badge-success">Active</span>' : '<span class="badge badge-secondary">Inactive</span>';
        const enabledBadge = service.enabled ? '<span class="badge badge-primary">Enabled</span>' : '<span class="badge badge-secondary">Disabled</span>';
        
        html += `<tr>
            <td><strong>${service.name}</strong></td>
            <td>${service.description}</td>
            <td>${statusBadge}</td>
            <td>${enabledBadge}</td>
            <td>${service.memory || '-'}</td>
            <td>
                ${service.active ? 
                    `<button class="btn-sm btn-danger" onclick="controlService('${service.name}', 'stop')">Stop</button>
                     <button class="btn-sm" onclick="controlService('${service.name}', 'restart')">Restart</button>` :
                    `<button class="btn-sm btn-success" onclick="controlService('${service.name}', 'start')">Start</button>`
                }
                <button class="btn-sm" onclick="viewServiceLogs('${service.name}')">Logs</button>
            </td>
        </tr>`;
    });
    
    html += '</tbody></table>';
    container.innerHTML = html;
}

async function controlService(service, action) {
    try {
        const res = await fetch('/api/system/services.php', {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({
                action: action,
                service: service
            })
        });
        
        const data = await res.json();
        
        if (data.success) {
            loadServices();
            loadNotifications();
        } else {
            alert('Error: ' + data.error);
        }
    } catch (e) {
        alert('Failed: ' + e.message);
    }
}

async function viewServiceLogs(service) {
    try {
        const res = await fetch(`/api/system/services.php?action=logs&service=${service}&lines=50`);
        const data = await res.json();
        
        if (data.success) {
            alert(`Recent logs for ${service}:\n\n${data.logs.join('\n')}`);
        } else {
            alert('Error: ' + data.error);
        }
    } catch (e) {
        alert('Failed: ' + e.message);
    }
}

// ========================================
// REAL-TIME MONITORING
// ========================================

let monitoringInterval = null;

async function loadMonitoring() {
    // Initial load
    await updateMonitoring();
    
    // Auto-refresh every 2 seconds
    if (monitoringInterval) clearInterval(monitoringInterval);
    monitoringInterval = setInterval(updateMonitoring, 2000);
}

async function updateMonitoring() {
    try {
        const res = await fetch('/api/system/realtime.php?action=all');
        const data = await res.json();
        
        if (data.success) {
            const metrics = data.metrics;
            
            // Update CPU
            const cpuEl = document.getElementById('monitoring-cpu');
            if (cpuEl) {
                cpuEl.innerHTML = `
                    <h3>CPU Usage</h3>
                    <div class="metric-value">${metrics.cpu.total_usage}%</div>
                    <p>Load: ${metrics.cpu.load_average['1min']} / ${metrics.cpu.load_average['5min']} / ${metrics.cpu.load_average['15min']}</p>
                    <div class="cores-grid">
                        ${metrics.cpu.cores.map(c => `<div class="core-bar"><div style="width:${c.usage}%"></div><span>Core ${c.core}: ${c.usage}%</span></div>`).join('')}
                    </div>
                `;
            }
            
            // Update Memory
            const memEl = document.getElementById('monitoring-memory');
            if (memEl) {
                memEl.innerHTML = `
                    <h3>Memory</h3>
                    <div class="metric-value">${metrics.memory.usage_percent}%</div>
                    <p>Used: ${metrics.memory.used_human} / ${metrics.memory.total_human}</p>
                    <p>Available: ${metrics.memory.available_human}</p>
                `;
            }
            
            // Update Network
            const netEl = document.getElementById('monitoring-network');
            if (netEl) {
                let netHtml = '<h3>Network</h3><table><tr><th>Interface</th><th>RX</th><th>TX</th></tr>';
                metrics.network.forEach(iface => {
                    netHtml += `<tr><td>${iface.interface}</td><td>${iface.rx_bytes_human}</td><td>${iface.tx_bytes_human}</td></tr>`;
                });
                netHtml += '</table>';
                netEl.innerHTML = netHtml;
            }
            
            // Update Processes
            const procEl = document.getElementById('monitoring-processes');
            if (procEl) {
                let procHtml = '<h3>Top Processes</h3><table><tr><th>PID</th><th>User</th><th>CPU%</th><th>MEM%</th><th>Command</th></tr>';
                metrics.processes.slice(0, 10).forEach(proc => {
                    procHtml += `<tr><td>${proc.pid}</td><td>${proc.user}</td><td>${proc.cpu}</td><td>${proc.memory}</td><td>${proc.command}</td></tr>`;
                });
                procHtml += '</table>';
                procEl.innerHTML = procHtml;
            }
        }
    } catch (e) {
        console.error('Failed to update monitoring:', e);
    }
}

// Stop monitoring when leaving page
document.addEventListener('visibilitychange', () => {
    if (document.hidden && monitoringInterval) {
        clearInterval(monitoringInterval);
        monitoringInterval = null;
    }
});


// ========================================
// ZFS ENCRYPTION MANAGEMENT (v1.8.0)
// ========================================

async function loadEncryptedDatasets() {
    const container = document.getElementById('encryption-list');
    if (!container) return;
    
    container.innerHTML = '<div class="loading">Loading encrypted datasets...</div>';
    
    try {
        const res = await fetch('/api/storage/encryption.php?action=list');
        const data = await res.json();
        
        if (data.success) {
            displayEncryptedDatasets(data.datasets);
        } else {
            container.innerHTML = `<p class="error">${data.error}</p>`;
        }
    } catch (e) {
        container.innerHTML = `<p class="error">Failed to load: ${e.message}</p>`;
    }
}

function displayEncryptedDatasets(datasets) {
    const container = document.getElementById('encryption-list');
    
    if (datasets.length === 0) {
        container.innerHTML = '<p>No encrypted datasets found.</p>';
        return;
    }
    
    let html = '<table><thead><tr><th>Dataset</th><th>Encryption</th><th>Key Status</th><th>Root</th><th>Actions</th></tr></thead><tbody>';
    
    datasets.forEach(ds => {
        const keyBadge = ds.keystatus === 'available' ? 
            '<span class="badge badge-success">Available</span>' : 
            '<span class="badge badge-warning">Unavailable</span>';
        
        html += `<tr>
            <td><strong>${ds.name}</strong></td>
            <td>${ds.encryption}</td>
            <td>${keyBadge}</td>
            <td>${ds.encryptionroot}</td>
            <td>
                ${ds.keystatus === 'unavailable' ? 
                    `<button class="btn-sm btn-success" onclick="loadKey('${ds.name}')">Load Key</button>` :
                    `<button class="btn-sm btn-secondary" onclick="unloadKey('${ds.name}')">Unload Key</button>`
                }
                <button class="btn-sm" onclick="changeEncryptionKey('${ds.name}')">Change Password</button>
            </td>
        </tr>`;
    });
    
    html += '</tbody></table>';
    container.innerHTML = html;
}

async function showCreateEncryptedDataset() {
    const modal = document.getElementById('encryption-modal');
    if (!modal) {
        alert('Encryption modal not found in UI');
        return;
    }
    
    document.getElementById('encryption-modal-title').textContent = 'Create Encrypted Dataset';
    document.getElementById('encryption-form').reset();
    modal.classList.add('active');
}

async function createEncryptedDataset() {
    const form = document.getElementById('encryption-form');
    const formData = new FormData(form);
    
    const name = formData.get('name');
    const password = formData.get('password');
    const confirmPassword = formData.get('confirm_password');
    const encryption = formData.get('encryption') || 'aes-256-gcm';
    
    if (!name || !password) {
        alert('Dataset name and password are required');
        return;
    }
    
    if (password !== confirmPassword) {
        alert('Passwords do not match');
        return;
    }
    
    if (password.length < 8) {
        alert('Password must be at least 8 characters');
        return;
    }
    
    try {
        const res = await fetch('/api/storage/encryption.php', {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({
                action: 'create_encrypted',
                name: name,
                password: password,
                encryption: encryption,
                keyformat: 'passphrase'
            })
        });
        
        const data = await res.json();
        
        if (data.success) {
            closeModal('encryption-modal');
            loadDatasets();
            loadEncryptedDatasets();
            loadNotifications();
            alert('Encrypted dataset created successfully!\n\nIMPORTANT: Save your password securely. Without it, your data is PERMANENTLY INACCESSIBLE.');
        } else {
            alert('Error: ' + data.error);
        }
    } catch (e) {
        alert('Failed: ' + e.message);
    }
}

async function loadKey(dataset) {
    const password = prompt(`Enter password for ${dataset}:`);
    if (!password) return;
    
    try {
        const res = await fetch('/api/storage/encryption.php', {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({
                action: 'load_key',
                name: dataset,
                password: password
            })
        });
        
        const data = await res.json();
        
        if (data.success) {
            loadEncryptedDatasets();
            loadNotifications();
            alert('Key loaded and dataset mounted successfully!');
        } else {
            alert('Error: ' + data.error);
        }
    } catch (e) {
        alert('Failed: ' + e.message);
    }
}

async function unloadKey(dataset) {
    if (!confirm(`Unload key for ${dataset}?\n\nThe dataset will be unmounted and inaccessible until the key is loaded again.`)) {
        return;
    }
    
    try {
        const res = await fetch('/api/storage/encryption.php', {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({
                action: 'unload_key',
                name: dataset
            })
        });
        
        const data = await res.json();
        
        if (data.success) {
            loadEncryptedDatasets();
            loadNotifications();
            alert('Key unloaded and dataset unmounted.');
        } else {
            alert('Error: ' + data.error);
        }
    } catch (e) {
        alert('Failed: ' + e.message);
    }
}

async function changeEncryptionKey(dataset) {
    const oldPassword = prompt(`Enter current password for ${dataset}:`);
    if (!oldPassword) return;
    
    const newPassword = prompt(`Enter NEW password for ${dataset}:`);
    if (!newPassword) return;
    
    if (newPassword.length < 8) {
        alert('New password must be at least 8 characters');
        return;
    }
    
    const confirmNew = prompt(`Confirm NEW password:`);
    if (newPassword !== confirmNew) {
        alert('New passwords do not match');
        return;
    }
    
    try {
        const res = await fetch('/api/storage/encryption.php', {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({
                action: 'change_key',
                name: dataset,
                old_password: oldPassword,
                new_password: newPassword
            })
        });
        
        const data = await res.json();
        
        if (data.success) {
            alert('Encryption password changed successfully!\n\nIMPORTANT: Save your new password securely.');
        } else {
            alert('Error: ' + data.error);
        }
    } catch (e) {
        alert('Failed: ' + e.message);
    }
}

async function loadAllKeys() {
    const password = prompt('Enter master password to unlock all encrypted datasets:');
    if (!password) return;
    
    try {
        const res = await fetch('/api/storage/encryption.php', {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({
                action: 'load_all_keys',
                password: password
            })
        });
        
        const data = await res.json();
        
        if (data.success) {
            loadEncryptedDatasets();
            loadNotifications();
            
            if (data.loaded === data.total) {
                alert(`Success! All ${data.total} encrypted datasets unlocked.`);
            } else {
                let msg = `Loaded ${data.loaded} of ${data.total} datasets.\n\nDetails:\n`;
                for (const [ds, result] of Object.entries(data.results)) {
                    msg += `\n${ds}: ${result.message}`;
                }
                alert(msg);
            }
        } else {
            alert('Error: ' + data.error);
        }
    } catch (e) {
        alert('Failed: ' + e.message);
    }
}

async function checkPendingKeys() {
    try {
        const res = await fetch('/api/storage/encryption.php?action=pending_keys');
        const data = await res.json();
        
        if (data.success && data.datasets.length > 0) {
            const banner = document.getElementById('encryption-banner');
            if (banner) {
                banner.style.display = 'block';
                document.getElementById('pending-keys-count').textContent = data.datasets.length;
            }
        }
    } catch (e) {
        console.error('Failed to check pending keys:', e);
    }
}

// Check for pending keys on page load
document.addEventListener('DOMContentLoaded', () => {
    checkPendingKeys();
});


// ========================================
// POOL MANAGEMENT - COMPLETE
// ========================================

async function showCreatePoolModal() {
    // Load available disks first
    const res = await fetch('/api/storage/pools.php?action=available_disks');
    const data = await res.json();
    
    if (!data.success) {
        alert('Failed to load available disks');
        return;
    }
    
    const modal = document.getElementById('modal-create-pool');
    if (!modal) {
        alert('Modal not found in UI');
        return;
    }
    
    // Populate disk selector
    const diskSelector = document.getElementById('disk-selector');
    if (diskSelector) {
        diskSelector.innerHTML = data.disks.map(disk => `
            <div class="disk-select-item ${disk.in_use ? 'in-use' : ''}" onclick="toggleDiskSelection(this, '${disk.device}')">
                <input type="checkbox" id="disk-${disk.name}" ${disk.in_use ? 'disabled' : ''}>
                <div>
                    <div style="font-weight: 600;">💾 ${disk.name} - ${disk.size}</div>
                    <div style="font-size: 0.85rem; color: ${disk.in_use ? '#666' : '#aaa'};">
                        ${disk.model}${disk.in_use ? ' (In use)' : ''}
                    </div>
                </div>
            </div>
        `).join('');
    }
    
    modal.classList.add('active');
}

function toggleDiskSelection(element, device) {
    if (element.classList.contains('in-use')) return;
    const checkbox = element.querySelector('input[type="checkbox"]');
    checkbox.checked = !checkbox.checked;
}

async function createPool() {
    const form = document.getElementById('create-pool-form');
    const formData = new FormData(form);
    
    const name = formData.get('name');
    const raid = formData.get('raid');
    
    // Get selected disks
    const selectedDisks = [];
    document.querySelectorAll('#disk-selector input[type="checkbox"]:checked').forEach(cb => {
        selectedDisks.push(cb.id.replace('disk-', ''));
    });
    
    if (!name) {
        alert('Pool name required');
        return;
    }
    
    if (selectedDisks.length === 0) {
        alert('Select at least one disk');
        return;
    }
    
    // Validate disk count for RAID type
    const minDisks = {
        'stripe': 1,
        'mirror': 2,
        'raidz1': 3,
        'raidz2': 4,
        'raidz3': 5
    };
    
    if (selectedDisks.length < minDisks[raid]) {
        alert(`${raid} requires at least ${minDisks[raid]} disks`);
        return;
    }
    
    try {
        const res = await fetch('/api/storage/pools.php', {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({
                action: 'create',
                name: name,
                raid: raid,
                disks: selectedDisks.map(d => '/dev/' + d)
            })
        });
        
        const data = await res.json();
        
        if (data.success) {
            closeModal('modal-create-pool');
            loadPools();
            loadNotifications();
            alert('Pool created successfully!');
        } else {
            alert('Error: ' + data.error);
        }
    } catch (e) {
        alert('Failed: ' + e.message);
    }
}

function confirmPoolDestroy(poolName) {
    const modal = document.getElementById('modal-destroy-pool');
    if (!modal) {
        if (confirm(`Destroy pool '${poolName}'? ALL DATA WILL BE LOST!`)) {
            destroyPool(poolName);
        }
        return;
    }
    
    document.getElementById('destroy-pool-name').textContent = poolName;
    document.getElementById('confirm-destroy-checkbox').checked = false;
    document.getElementById('destroy-pool-btn').disabled = true;
    modal.classList.add('active');
}

async function destroyPool(poolName) {
    try {
        const res = await fetch('/api/storage/pools.php', {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({
                action: 'destroy',
                name: poolName
            })
        });
        
        const data = await res.json();
        
        if (data.success) {
            closeModal('modal-destroy-pool');
            loadPools();
            loadNotifications();
            alert('Pool destroyed');
        } else {
            alert('Error: ' + data.error);
        }
    } catch (e) {
        alert('Failed: ' + e.message);
    }
}

async function scrubPool(poolName) {
    if (!confirm(`Start scrub on pool '${poolName}'?`)) return;
    
    try {
        const res = await fetch('/api/storage/pools.php', {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({
                action: 'scrub',
                name: poolName
            })
        });
        
        const data = await res.json();
        
        if (data.success) {
            alert('Scrub started');
            loadNotifications();
        } else {
            alert('Error: ' + data.error);
        }
    } catch (e) {
        alert('Failed: ' + e.message);
    }
}

function expandPool(poolName) {
    alert(`Expand pool: ${poolName}\n\nThis would open a dialog to add more VDEVs to the pool`);
}

// ========================================
// SHARES MANAGEMENT - COMPLETE
// ========================================

async function loadShares() {
    const container = document.getElementById('shares-list');
    if (!container) return;
    
    container.innerHTML = '<div class="loading">Loading shares...</div>';
    
    try {
        const res = await fetch('/api/storage/shares.php?action=list');
        const data = await res.json();
        
        if (data.success) {
            if (data.shares.length === 0) {
                container.innerHTML = '<p>No shares configured.</p>';
                return;
            }
            
            let html = '<table><thead><tr><th>Name</th><th>Type</th><th>Path</th><th>Status</th><th>Access</th><th>Actions</th></tr></thead><tbody>';
            
            data.shares.forEach(share => {
                const typeBadge = share.share_type === 'smb' ? 
                    '<span class="badge badge-primary">SMB</span>' : 
                    '<span class="badge badge-secondary">NFS</span>';
                
                const statusBadge = share.enabled ? 
                    '<span class="badge badge-success">Active</span>' : 
                    '<span class="badge badge-secondary">Disabled</span>';
                
                const access = share.share_type === 'smb' ? 
                    (share.smb_read_only ? 'Read Only' : 'Read/Write') :
                    (share.nfs_allowed_networks || 'All');
                
                html += `
                    <tr>
                        <td><strong>${share.name}</strong></td>
                        <td>${typeBadge}</td>
                        <td>${share.dataset_path}</td>
                        <td>${statusBadge}</td>
                        <td>${access}</td>
                        <td>
                            <button class="btn-sm" onclick="editShare(${share.id})">Edit</button>
                            <button class="btn-sm btn-danger" onclick="confirmShareDelete(${share.id}, '${share.name}')">Delete</button>
                        </td>
                    </tr>
                `;
            });
            
            html += '</tbody></table>';
            container.innerHTML = html;
        } else {
            container.innerHTML = `<p class="error">${data.error}</p>`;
        }
    } catch (e) {
        container.innerHTML = `<p class="error">Failed: ${e.message}</p>`;
    }
}

async function showCreateShareModal() {
    const modal = document.getElementById('modal-create-share');
    if (!modal) {
        alert('Modal not found');
        return;
    }
    
    // Load available datasets
    const res = await fetch('/api/storage/datasets.php?action=list');
    const data = await res.json();
    
    if (data.success && data.datasets) {
        const select = document.getElementById('share-dataset-select');
        if (select) {
            select.innerHTML = data.datasets
                .filter(d => d.type === 'filesystem')
                .map(d => `<option value="${d.name}">${d.name} (${d.mountpoint})</option>`)
                .join('');
        }
    }
    
    modal.classList.add('active');
}

async function createShare() {
    const form = document.getElementById('create-share-form');
    const formData = new FormData(form);
    
    const shareData = {
        action: 'create',
        name: formData.get('name'),
        dataset_path: formData.get('dataset_path'),
        share_type: formData.get('share_type'),
        enabled: 1
    };
    
    if (shareData.share_type === 'smb') {
        shareData.smb_guest_ok = formData.get('smb_guest_ok') ? 1 : 0;
        shareData.smb_read_only = formData.get('smb_read_only') ? 1 : 0;
    } else {
        shareData.nfs_allowed_networks = formData.get('nfs_allowed_networks');
        shareData.nfs_read_only = formData.get('nfs_read_only') ? 1 : 0;
    }
    
    try {
        const res = await fetch('/api/storage/shares.php', {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify(shareData)
        });
        
        const data = await res.json();
        
        if (data.success) {
            closeModal('modal-create-share');
            loadShares();
            loadNotifications();
            alert('Share created successfully!');
        } else {
            alert('Error: ' + data.error);
        }
    } catch (e) {
        alert('Failed: ' + e.message);
    }
}

function editShare(shareId) {
    alert(`Edit share ID: ${shareId}\n\nThis would open the edit modal with current values`);
}

function confirmShareDelete(shareId, shareName) {
    const modal = document.getElementById('modal-delete-share');
    if (!modal) {
        if (confirm(`Delete share '${shareName}'?`)) {
            deleteShare(shareId);
        }
        return;
    }
    
    document.getElementById('delete-share-name').textContent = shareName;
    document.getElementById('delete-share-id').value = shareId;
    document.getElementById('confirm-share-delete-checkbox').checked = false;
    document.getElementById('delete-share-btn').disabled = true;
    modal.classList.add('active');
}

async function deleteShare(shareId) {
    try {
        const res = await fetch('/api/storage/shares.php', {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({
                action: 'delete',
                id: shareId
            })
        });
        
        const data = await res.json();
        
        if (data.success) {
            closeModal('modal-delete-share');
            loadShares();
            loadNotifications();
            alert('Share deleted');
        } else {
            alert('Error: ' + data.error);
        }
    } catch (e) {
        alert('Failed: ' + e.message);
    }
}

function updateShareTypeOptions() {
    const shareType = document.getElementById('share-type-select');
    if (!shareType) return;
    
    const smbOptions = document.getElementById('smb-options');
    const nfsOptions = document.getElementById('nfs-options');
    
    if (shareType.value === 'smb') {
        if (smbOptions) smbOptions.style.display = 'block';
        if (nfsOptions) nfsOptions.style.display = 'none';
    } else {
        if (smbOptions) smbOptions.style.display = 'none';
        if (nfsOptions) nfsOptions.style.display = 'block';
    }
}

// ========================================
// SNAPSHOT MANAGEMENT - COMPLETE
// ========================================

async function loadSnapshots() {
    await loadSnapshotSchedules();
    await loadSnapshotHistory();
}

async function loadSnapshotSchedules() {
    const container = document.getElementById('snapshot-schedules');
    if (!container) return;
    
    container.innerHTML = '<div class="loading">Loading schedules...</div>';
    
    try {
        const res = await fetch('/api/storage/snapshots.php?action=schedules');
        const data = await res.json();
        
        if (data.success) {
            if (data.schedules.length === 0) {
                container.innerHTML = '<p>No schedules configured.</p>';
                return;
            }
            
            let html = '<table><thead><tr><th>Dataset</th><th>Frequency</th><th>Keep</th><th>Last Run</th><th>Status</th><th>Actions</th></tr></thead><tbody>';
            
            data.schedules.forEach(sched => {
                const statusBadge = sched.enabled ? 
                    '<span class="badge badge-success">Enabled</span>' : 
                    '<span class="badge badge-secondary">Disabled</span>';
                
                html += `
                    <tr>
                        <td><strong>${sched.dataset_path}</strong></td>
                        <td>${sched.frequency} at ${sched.time}</td>
                        <td>${sched.keep_count} snapshots</td>
                        <td>${sched.last_run || 'Never'}</td>
                        <td>${statusBadge}</td>
                        <td>
                            <button class="btn-sm" onclick="runSnapshotNow(${sched.id})">Run Now</button>
                            <button class="btn-sm btn-secondary" onclick="toggleSchedule(${sched.id})">
                                ${sched.enabled ? 'Disable' : 'Enable'}
                            </button>
                            <button class="btn-sm btn-danger" onclick="confirmScheduleDelete(${sched.id}, '${sched.dataset_path}')">Delete</button>
                        </td>
                    </tr>
                `;
            });
            
            html += '</tbody></table>';
            container.innerHTML = html;
        } else {
            container.innerHTML = `<p class="error">${data.error}</p>`;
        }
    } catch (e) {
        container.innerHTML = `<p class="error">Failed: ${e.message}</p>`;
    }
}

async function loadSnapshotHistory() {
    const container = document.getElementById('snapshot-history');
    if (!container) return;
    
    try {
        const res = await fetch('/api/storage/snapshots.php?action=history&limit=50');
        const data = await res.json();
        
        if (data.success) {
            if (data.snapshots.length === 0) {
                container.innerHTML = '<p>No snapshots found.</p>';
                return;
            }
            
            let html = '<table><thead><tr><th>Snapshot</th><th>Dataset</th><th>Created</th><th>Size</th><th>Actions</th></tr></thead><tbody>';
            
            data.snapshots.forEach(snap => {
                html += `
                    <tr>
                        <td><strong>${snap.snapshot_name}</strong></td>
                        <td>${snap.dataset_path}</td>
                        <td>${snap.created_at}</td>
                        <td>-</td>
                        <td>
                            <button class="btn-sm btn-success" onclick="confirmSnapshotRestore('${snap.dataset_path}', '${snap.snapshot_name}')">Restore</button>
                            <button class="btn-sm btn-danger" onclick="confirmSnapshotDelete('${snap.dataset_path}', '${snap.snapshot_name}')">Delete</button>
                        </td>
                    </tr>
                `;
            });
            
            html += '</tbody></table>';
            container.innerHTML = html;
        } else {
            container.innerHTML = `<p class="error">${data.error}</p>`;
        }
    } catch (e) {
        container.innerHTML = `<p class="error">Failed: ${e.message}</p>`;
    }
}

async function showCreateScheduleModal() {
    const modal = document.getElementById('modal-create-schedule');
    if (!modal) {
        alert('Modal not found');
        return;
    }
    
    // Load datasets
    const res = await fetch('/api/storage/datasets.php?action=list');
    const data = await res.json();
    
    if (data.success && data.datasets) {
        const select = document.getElementById('schedule-dataset-select');
        if (select) {
            select.innerHTML = data.datasets
                .filter(d => d.type === 'filesystem')
                .map(d => `<option value="${d.name}">${d.name}</option>`)
                .join('');
        }
    }
    
    modal.classList.add('active');
}

async function createSchedule() {
    const form = document.getElementById('create-schedule-form');
    const formData = new FormData(form);
    
    try {
        const res = await fetch('/api/storage/snapshots.php', {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({
                action: 'create_schedule',
                dataset_path: formData.get('dataset_path'),
                frequency: formData.get('frequency'),
                time: formData.get('time'),
                keep_count: parseInt(formData.get('keep_count'))
            })
        });
        
        const data = await res.json();
        
        if (data.success) {
            closeModal('modal-create-schedule');
            loadSnapshots();
            loadNotifications();
            alert('Schedule created!');
        } else {
            alert('Error: ' + data.error);
        }
    } catch (e) {
        alert('Failed: ' + e.message);
    }
}

function confirmSnapshotRestore(dataset, snapshot) {
    const modal = document.getElementById('modal-restore-snapshot');
    if (!modal) {
        if (confirm(`Restore ${dataset} to snapshot ${snapshot}? ALL CHANGES AFTER THIS SNAPSHOT WILL BE LOST!`)) {
            restoreSnapshot(dataset, snapshot);
        }
        return;
    }
    
    document.getElementById('restore-dataset-name').textContent = dataset;
    document.getElementById('restore-snapshot-name').textContent = snapshot;
    document.getElementById('confirm-restore-checkbox').checked = false;
    document.getElementById('restore-snapshot-btn').disabled = true;
    modal.classList.add('active');
}

async function restoreSnapshot(dataset, snapshot) {
    try {
        const res = await fetch('/api/storage/snapshots.php', {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({
                action: 'rollback',
                dataset: dataset,
                snapshot: snapshot
            })
        });
        
        const data = await res.json();
        
        if (data.success) {
            closeModal('modal-restore-snapshot');
            loadSnapshots();
            loadNotifications();
            alert('Snapshot restored!');
        } else {
            alert('Error: ' + data.error);
        }
    } catch (e) {
        alert('Failed: ' + e.message);
    }
}

function confirmSnapshotDelete(dataset, snapshot) {
    if (confirm(`Delete snapshot ${dataset}@${snapshot}?\n\nThis cannot be undone!`)) {
        deleteSnapshot(dataset, snapshot);
    }
}

async function deleteSnapshot(dataset, snapshot) {
    try {
        const res = await fetch('/api/storage/snapshots.php', {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({
                action: 'delete_snapshot',
                snapshot: `${dataset}@${snapshot}`
            })
        });
        
        const data = await res.json();
        
        if (data.success) {
            loadSnapshots();
            loadNotifications();
            alert('Snapshot deleted');
        } else {
            alert('Error: ' + data.error);
        }
    } catch (e) {
        alert('Failed: ' + e.message);
    }
}

async function runSnapshotNow(scheduleId) {
    if (!confirm('Create snapshot now?')) return;
    
    try {
        const res = await fetch('/api/storage/snapshots.php', {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({
                action: 'run_now',
                schedule_id: scheduleId
            })
        });
        
        const data = await res.json();
        
        if (data.success) {
            loadSnapshots();
            loadNotifications();
            alert('Snapshot created!');
        } else {
            alert('Error: ' + data.error);
        }
    } catch (e) {
        alert('Failed: ' + e.message);
    }
}

async function toggleSchedule(scheduleId) {
    try {
        const res = await fetch('/api/storage/snapshots.php', {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({
                action: 'toggle_schedule',
                id: scheduleId
            })
        });
        
        const data = await res.json();
        
        if (data.success) {
            loadSnapshots();
            loadNotifications();
        } else {
            alert('Error: ' + data.error);
        }
    } catch (e) {
        alert('Failed: ' + e.message);
    }
}

function confirmScheduleDelete(scheduleId, dataset) {
    if (confirm(`Delete snapshot schedule for ${dataset}?\n\nExisting snapshots will not be deleted.`)) {
        deleteSchedule(scheduleId);
    }
}

async function deleteSchedule(scheduleId) {
    try {
        const res = await fetch('/api/storage/snapshots.php', {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({
                action: 'delete_schedule',
                id: scheduleId
            })
        });
        
        const data = await res.json();
        
        if (data.success) {
            loadSnapshots();
            loadNotifications();
            alert('Schedule deleted');
        } else {
            alert('Error: ' + data.error);
        }
    } catch (e) {
        alert('Failed: ' + e.message);
    }
}

// ========================================
// DRAGGABLE NAVIGATION
// ========================================

let draggedNavElement = null;

document.addEventListener('DOMContentLoaded', () => {
    const navButtons = document.querySelectorAll('.nav-btn');
    
    navButtons.forEach(btn => {
        btn.addEventListener('dragstart', (e) => {
            draggedNavElement = btn;
            btn.classList.add('dragging');
            e.dataTransfer.effectAllowed = 'move';
        });
        
        btn.addEventListener('dragend', () => {
            btn.classList.remove('dragging');
            // Save order to localStorage
            saveNavOrder();
        });
        
        btn.addEventListener('dragover', (e) => {
            e.preventDefault();
            const container = document.getElementById('nav-container');
            const afterElement = getDragAfterElement(container, e.clientX);
            
            if (afterElement == null) {
                container.appendChild(draggedNavElement);
            } else {
                container.insertBefore(draggedNavElement, afterElement);
            }
        });
    });
    
    // Load saved nav order
    loadNavOrder();
});

function getDragAfterElement(container, x) {
    const draggableElements = [...container.querySelectorAll('.nav-btn:not(.dragging)')];
    
    return draggableElements.reduce((closest, child) => {
        const box = child.getBoundingClientRect();
        const offset = x - box.left - box.width / 2;
        
        if (offset < 0 && offset > closest.offset) {
            return { offset: offset, element: child };
        } else {
            return closest;
        }
    }, { offset: Number.NEGATIVE_INFINITY }).element;
}

function saveNavOrder() {
    const navButtons = document.querySelectorAll('.nav-btn');
    const order = Array.from(navButtons).map(btn => btn.dataset.page);
    localStorage.setItem('dplaneos_nav_order', JSON.stringify(order));
}

function loadNavOrder() {
    const saved = localStorage.getItem('dplaneos_nav_order');
    if (!saved) return;
    
    try {
        const order = JSON.parse(saved);
        const container = document.getElementById('nav-container');
        const buttons = {};
        
        // Collect all buttons
        document.querySelectorAll('.nav-btn').forEach(btn => {
            buttons[btn.dataset.page] = btn;
        });
        
        // Reorder
        order.forEach(page => {
            if (buttons[page]) {
                container.appendChild(buttons[page]);
            }
        });
    } catch (e) {
        console.error('Failed to load nav order:', e);
    }
}

// Checkbox confirmations for destructive actions
document.addEventListener('DOMContentLoaded', () => {
    const destroyCheckbox = document.getElementById('confirm-destroy-checkbox');
    if (destroyCheckbox) {
        destroyCheckbox.addEventListener('change', (e) => {
            const btn = document.getElementById('destroy-pool-btn');
            if (btn) btn.disabled = !e.target.checked;
        });
    }
    
    const shareDeleteCheckbox = document.getElementById('confirm-share-delete-checkbox');
    if (shareDeleteCheckbox) {
        shareDeleteCheckbox.addEventListener('change', (e) => {
            const btn = document.getElementById('delete-share-btn');
            if (btn) btn.disabled = !e.target.checked;
        });
    }
    
    const restoreCheckbox = document.getElementById('confirm-restore-checkbox');
    if (restoreCheckbox) {
        restoreCheckbox.addEventListener('change', (e) => {
            const btn = document.getElementById('restore-snapshot-btn');
            if (btn) btn.disabled = !e.target.checked;
        });
    }
});


// RAID Type Info Update
function updateRaidInfo() {
    const raidType = document.getElementById('raid-type-select');
    if (!raidType) return;
    
    const infoEl = document.getElementById('raid-info');
    if (!infoEl) return;
    
    const info = {
        'stripe': 'Stripe: No redundancy. Maximum performance and capacity. Any disk failure = total data loss. Minimum 1 disk.',
        'mirror': 'Mirror: 50% capacity. Can survive loss of half the disks. Minimum 2 disks.',
        'raidz1': 'RAIDZ1: Can survive 1 disk failure. Minimum 3 disks.',
        'raidz2': 'RAIDZ2: Can survive 2 disk failures. Minimum 4 disks. Recommended for large arrays.',
        'raidz3': 'RAIDZ3: Can survive 3 disk failures. Minimum 5 disks. Maximum redundancy.'
    };
    
    infoEl.textContent = info[raidType.value] || '';
}

