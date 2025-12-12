let selectedClient = null;
let clients = [];
let currentFilter = 'all';
let currentPage = 1;
const itemsPerPage = 10;
let searchQuery = '';
let clientsLoadTimeout = null;
// Cache suggested ports per client to avoid re-fetching when not adding
const suggestedPortCache = {};
let healthState = null;

async function fetchJSON(url, options = {}) {
    try {
        const response = await fetch(url, options);
        if (response.status === 401) {
            window.location.href = '/login';
            return { ok: false, status: 401 };
        }
        const data = await response.json().catch(() => null);
        return { ok: response.ok, status: response.status, data };
    } catch (err) {
        console.error('Request failed', err);
        return { ok: false, status: 0, error: err };
    }
}

function setStatusDot(dotId, state) {
    const dot = document.getElementById(dotId);
    if (!dot) return;
    dot.classList.remove('healthy', 'degraded', 'down');
    if (state) dot.classList.add(state);
}

function setText(id, value) {
    const el = document.getElementById(id);
    if (el) el.textContent = value;
}

function formatDuration(seconds) {
    if (typeof seconds !== 'number' || seconds < 0) return '--';
    const hours = Math.floor(seconds / 3600);
    const minutes = Math.floor((seconds % 3600) / 60);
    if (hours >= 48) return `${Math.round(hours / 24)}d`;
    if (hours >= 1) return `${hours}h ${minutes}m`;
    return `${minutes}m`;
}

async function loadClients() {
    // Debounce multiple rapid calls
    if (clientsLoadTimeout) clearTimeout(clientsLoadTimeout);
    clientsLoadTimeout = setTimeout(async () => {
        try {
            const response = await fetch('/api/clients');
            if (response.status === 401) {
                window.location.href = '/login';
                return;
            }
            clients = await response.json() || [];
            currentPage = 1;
            renderClientList();
            updateStats();
            updateClientPill();
        } catch (err) {
            console.error('Error loading clients:', err);
        }
    }, 800);
}

function renderClientList() {
    const list = document.getElementById('clientList');
    
    // Filter clients based on current filter
    let filteredClients = clients;
    if (currentFilter === 'online') {
        filteredClients = clients.filter(c => c.status === 'online');
    } else if (currentFilter === 'offline') {
        filteredClients = clients.filter(c => c.status !== 'online');
    }
    
    // Apply search filter
    if (searchQuery.trim()) {
        const q = searchQuery.toLowerCase();
        filteredClients = filteredClients.filter(c => {
            const alias = (c.alias || '').toLowerCase();
            const hostname = (c.hostname || '').toLowerCase();
            const ip = (c.ip || '').toLowerCase();
            return alias.includes(q) || hostname.includes(q) || ip.includes(q);
        });
    }
    
    if (filteredClients.length === 0) {
        list.innerHTML = `
            <div class="empty-state" style="padding: 40px 10px;">
                <p style="font-size: 13px;">${clients.length === 0 ? 'No clients' : 'No ' + currentFilter + ' clients'}</p>
            </div>
        `;
        return;
    }

    // Pagination
    const totalPages = Math.ceil(filteredClients.length / itemsPerPage);
    const start = (currentPage - 1) * itemsPerPage;
    const end = start + itemsPerPage;
    const pageClients = filteredClients.slice(start, end);

    let html = pageClients.map(client => `
        <li class="client-item ${selectedClient?.id === client.id ? 'active' : ''}" 
            onclick="selectClient(event, ${JSON.stringify(client).replace(/"/g, '&quot;')})">
            <div style="display: flex; justify-content: space-between; align-items: flex-start; gap: 8px;">
                <div style="flex: 1;">
                    <div class="client-name">${escapeHtml(client.alias || client.hostname || client.id)}</div>
                    <div class="client-meta">${escapeHtml(client.ip || 'Unknown IP')} • ${escapeHtml(client.os || 'Unknown OS')}</div>
                </div>
                <span class="status-badge ${client.status === 'online' ? 'online' : 'offline'}" style="white-space: nowrap;">${client.status}</span>
            </div>
        </li>
    `).join('');

    // Add pagination controls
    if (totalPages > 1) {
        html += `
            <div style="display: flex; justify-content: center; gap: 8px; margin-top: 16px; padding-top: 12px; border-top: 1px solid var(--border);">
                <button onclick="prevPage()" class="btn btn-secondary" style="padding: 6px 10px; font-size: 12px;" ${currentPage === 1 ? 'disabled' : ''}>← Prev</button>
                <div style="display: flex; align-items: center; font-size: 12px; color: var(--text-light);">${currentPage} / ${totalPages}</div>
                <button onclick="nextPage()" class="btn btn-secondary" style="padding: 6px 10px; font-size: 12px;" ${currentPage === totalPages ? 'disabled' : ''}>Next →</button>
            </div>
        `;
    }

    list.innerHTML = html;
}

function nextPage() {
    const totalPages = Math.ceil(getFilteredClients().length / itemsPerPage);
    if (currentPage < totalPages) {
        currentPage++;
        renderClientList();
    }
}

function prevPage() {
    if (currentPage > 1) {
        currentPage--;
        renderClientList();
    }
}

function getFilteredClients() {
    let filtered = clients;
    if (currentFilter === 'online') {
        filtered = clients.filter(c => c.status === 'online');
    } else if (currentFilter === 'offline') {
        filtered = clients.filter(c => c.status !== 'online');
    }
    if (searchQuery.trim()) {
        const q = searchQuery.toLowerCase();
        filtered = filtered.filter(c => {
            const alias = (c.alias || '').toLowerCase();
            const hostname = (c.hostname || '').toLowerCase();
            const ip = (c.ip || '').toLowerCase();
            return alias.includes(q) || hostname.includes(q) || ip.includes(q);
        });
    }
    return filtered;
}

function updateSearch(value) {
    searchQuery = value;
    currentPage = 1;
    renderClientList();
}

function filterClients(filter) {
    currentFilter = filter;
    currentPage = 1;
    
    // Update active filter tab
    document.querySelectorAll('.filter-tab').forEach(tab => {
        tab.classList.remove('active');
    });
    event.target.classList.add('active');
    
    renderClientList();
}
function selectClient(event, client) {
    event.preventDefault();
    selectedClient = client;
    renderClientList();
    showClientInColumns(client);
}

function showClientInColumns(client) {
    // Update MIDDLE column: Proxy Management
    const proxyMiddle = document.getElementById('proxyMiddle');
    proxyMiddle.innerHTML = `
        <h3>🌐 Proxy for ${escapeHtml(client.hostname || client.id)}</h3>
        <ul class="proxy-list" id="proxyList">
            <div class="empty-proxy-state">Loading proxies...</div>
        </ul>
        <div class="proxy-form">
            <h5 style="font-size: 14px; margin-bottom: 12px;">➕ Add New Proxy</h5>
            <div class="proxy-form-group">
                <label>Client Address (host:port)</label>
                <input type="text" id="proxyRemoteAddr" placeholder="127.0.0.1:22">
            </div>
            <div class="proxy-form-group">
                <label>Server Port</label>
                <input type="number" id="proxyLocalPort" placeholder="10033">
            </div>
            <div class="proxy-form-group">
                <label>Protocol</label>
                <select id="proxyProtocol">
                    <option value="TCP">TCP</option>
                    <option value="HTTP">HTTP</option>
                    <option value="HTTPS">HTTPS</option>
                </select>
            </div>
            <button onclick="addProxy()">➕ Add Proxy Connection</button>
        </div>
    `;
    
    // Update RIGHT column: Client Details & Controls
    const clientRight = document.getElementById('clientRight');
    clientRight.innerHTML = `
        <h3>📋 Client Details</h3>
        <div style="margin-bottom: 20px;">
            <div class="details-row">
                <div class="details-label">Client ID</div>
                <div class="details-value" style="font-family: monospace; font-size: 11px; word-break: break-all;">${escapeHtml(client.id)}</div>
            </div>
            <div class="details-row">
                <div class="details-label">Alias</div>
                <div style="display: flex; gap: 8px; align-items: center;">
                    <input type="text" id="aliasInput" placeholder="Enter alias..." value="${escapeHtml(client.alias || '')}" style="flex: 1; padding: 6px 8px; border: 1px solid var(--border); border-radius: 4px;">
                    <button onclick="saveAlias()" style="padding: 6px 12px; background: var(--primary); color: white; border: none; border-radius: 4px; cursor: pointer;">✓</button>
                </div>
            </div>
            <div class="details-row">
                <div class="details-label">Hostname</div>
                <div class="details-value">${escapeHtml(client.hostname || 'N/A')}</div>
            </div>
            <div class="details-row">
                <div class="details-label">Operating System</div>
                <div class="details-value">${escapeHtml(client.os || 'N/A')} / ${escapeHtml(client.arch || 'N/A')}</div>
            </div>
            <div class="details-row">
                <div class="details-label">IP Address</div>
                <div class="details-value">${escapeHtml(client.ip || 'N/A')}</div>
            </div>
            <div class="details-row">
                <div class="details-label">Status</div>
                <div class="details-value"><span class="status-badge ${client.status === 'online' ? 'online' : 'offline'}">${client.status}</span></div>
            </div>
        </div>
        
        <h3 style="margin-top: 25px;">⚡ Control Panel</h3>
        <div class="details-actions">
            <button class="btn-action-primary" onclick="openClientPanel()">🖥️ Control</button>
            <button class="btn-action-primary" onclick="sendCommand()">⌨️ Terminal</button>
            <button class="btn-action-secondary" onclick="viewStats()">📊 Stats</button>
            <button class="btn-action-secondary" onclick="browseLogs()">📋 Logs</button>
            <button class="btn-action-danger" onclick="confirmRemove()">🗑️ Remove</button>
            <button class="btn-action-danger" onclick="confirmUninstall()">❌ Uninstall</button>
        </div>
    `;
    
    // Load proxies for this client
    loadProxies();
}
async function loadProxies() {
    if (!selectedClient) return;

    const clientId = selectedClient.ID || selectedClient.id;
    try {
        const response = await fetch(`/api/proxy/list?clientId=${encodeURIComponent(clientId)}`);
        if (response.status === 401) {
            window.location.href = '/login';
            return;
        }
        if (response.ok) {
            const proxies = await response.json() || [];
            renderProxies(proxies);
        }
    } catch (err) {
        console.error('Error loading proxies:', err);
        renderProxies([]);
    }
}

function setSuggestedPortValue(port) {
    const input = document.getElementById('proxyLocalPort');
    if (!input) return;
    // Only set if the user has not typed anything and not editing
    const isEditing = document.getElementById('proxyIdToEdit');
    if (isEditing && isEditing.value) return;
    if (input.value) return;
    input.value = port;
    input.placeholder = port;
}

async function fetchSuggestedPortForNewProxy(proxies) {
    const input = document.getElementById('proxyLocalPort');
    if (!input) return;

    const isEditing = document.getElementById('proxyIdToEdit');
    if (isEditing && isEditing.value) return; // do not override when editing

    // Only fetch when no proxies for this client (new proxy scenario)
    if (Array.isArray(proxies) && proxies.length > 0) return;

    // Do not fetch if user already typed a value
    if (input.value) return;

    const clientId = selectedClient?.ID || selectedClient?.id;
    if (!clientId) return;

    // Use cached suggestion if available
    if (suggestedPortCache[clientId]) {
        setSuggestedPortValue(suggestedPortCache[clientId]);
        return;
    }

    try {
        const resp = await fetch(`/api/proxy/suggest?clientId=${encodeURIComponent(clientId)}`);
        if (!resp.ok) return;
        const data = await resp.json();
        const ports = data.suggestedPorts || data.ports || [];
        if (ports.length > 0) {
            suggestedPortCache[clientId] = ports[0];
            setSuggestedPortValue(ports[0]);
        }
    } catch (err) {
        console.warn('Failed to fetch suggested port', err);
    }
}

function renderProxies(proxies) {
    const list = document.getElementById('proxyList');
                if (!list) return;
    
    if (!proxies || proxies.length === 0) {
        list.innerHTML = `
            <div class="empty-proxy-state">
                <p style="font-size: 13px;">No proxy connections</p>
            </div>
        `;
        // Suggest a port only when adding the first proxy and not in edit mode
        fetchSuggestedPortForNewProxy(proxies);
        return;
    }

    list.innerHTML = proxies.map(proxy => `
        <li class="proxy-item">
            <div class="proxy-source">:${proxy.LocalPort} → ${escapeHtml(proxy.RemoteHost)}:${proxy.RemotePort}</div>
            <div class="proxy-meta">
                <span>Protocol: ${escapeHtml(proxy.Protocol)}</span>
                <span>Status: ${escapeHtml(proxy.Status)}</span>
            </div>
            <button onclick="editProxy('${escapeHtml(proxy.ID)}', '${escapeHtml(proxy.RemoteHost)}', ${proxy.RemotePort}, ${proxy.LocalPort}, '${escapeHtml(proxy.Protocol)}')">✏️ Edit</button>
            <button onclick="deleteProxy('${escapeHtml(proxy.ID)}')">🗑️ Delete</button>
        </li>
    `).join('');
}

async function deleteProxy(proxyId) {
    if (!proxyId) {
        showStatus('Error', 'Invalid proxy ID');
        return;
    }
    
    try {
        const response = await fetch(`/api/proxy/close?id=${encodeURIComponent(proxyId)}`, {
            method: 'POST'
        });

        if (response.ok) {
            showStatus('Success', 'Proxy connection closed!');
            setTimeout(loadProxies, 500);
        } else if (response.status === 401) {
            window.location.href = '/login';
        } else {
            showStatus('Error', 'Failed to close proxy');
        }
    } catch (err) {
        console.error('Error:', err);
        showStatus('Error', 'Error closing proxy connection');
    }
}

function editProxy(proxyId, remoteHost, remotePort, localPort, protocol) {
    // Set form fields with current values
    document.getElementById('proxyRemoteAddr').value = `${remoteHost}:${remotePort}`;
    document.getElementById('proxyLocalPort').value = localPort;
    document.getElementById('proxyProtocol').value = protocol.toUpperCase();
    
    // Store the proxy ID in a hidden field for update
    let hiddenId = document.getElementById('proxyIdToEdit');
    if (!hiddenId) {
        hiddenId = document.createElement('input');
        hiddenId.id = 'proxyIdToEdit';
        hiddenId.type = 'hidden';
        document.getElementById('proxyLocalPort').parentElement.appendChild(hiddenId);
    }
    hiddenId.value = proxyId;
    
    // Change button text
    const addBtn = document.querySelector('button[onclick="addProxy()"]');
    if (addBtn) {
        addBtn.textContent = '💾 Update Proxy Connection';
        addBtn.onclick = function() { updateProxy(); };
    }
    
    // Scroll to form
    document.getElementById('proxyLocalPort').focus();
}

async function updateProxy() {
    const remoteAddr = document.getElementById('proxyRemoteAddr').value.trim();
    const localPort = document.getElementById('proxyLocalPort').value.trim();
    const protocol = document.getElementById('proxyProtocol').value;
    const proxyId = document.getElementById('proxyIdToEdit').value;

    if (!proxyId) {
        showStatus('Error', 'No proxy selected for edit');
        return;
    }

    if (!remoteAddr || !localPort) {
        showStatus('Error', 'Please fill in all fields');
        return;
    }

    const [remoteHost, remotePort] = remoteAddr.split(':');
    if (!remoteHost || !remotePort) {
        showStatus('Error', 'Invalid address format. Use: 127.0.0.1:22');
        return;
    }

    if (isNaN(localPort) || isNaN(remotePort)) {
        showStatus('Error', 'Port numbers must be numeric');
        return;
    }

    try {
        const response = await fetch('/api/proxy/edit', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                proxyId: proxyId,
                remoteHost: remoteHost,
                remotePort: parseInt(remotePort),
                localPort: parseInt(localPort),
                protocol: protocol
            })
        });

        if (response.ok) {
            showStatus('Success', 'Proxy connection updated!');
            cancelProxyEdit();
            setTimeout(loadProxies, 500);
        } else if (response.status === 401) {
            window.location.href = '/login';
        } else {
            const errText = await response.text();
            showStatus('Error', 'Failed to update proxy: ' + errText);
        }
    } catch (err) {
        console.error('Error:', err);
        showStatus('Error', 'Error updating proxy connection');
    }
}

function cancelProxyEdit() {
    document.getElementById('proxyRemoteAddr').value = '';
    document.getElementById('proxyLocalPort').value = '';
    document.getElementById('proxyProtocol').value = 'TCP';
    const hiddenId = document.getElementById('proxyIdToEdit');
    if (hiddenId) {
        hiddenId.value = '';
    }
    
    const addBtn = document.querySelector('button[onclick="updateProxy()"]');
    if (addBtn) {
        addBtn.textContent = '➕ Add Proxy Connection';
        addBtn.onclick = function() { addProxy(); };
    }

    // If there are no proxies, re-suggest a port after cancelling edit
    fetchSuggestedPortForNewProxy([]);
}

async function addProxy() {
    const remoteAddr = document.getElementById('proxyRemoteAddr').value.trim();
    const localPort = document.getElementById('proxyLocalPort').value.trim();
    const protocol = document.getElementById('proxyProtocol').value;

    if (!remoteAddr || !localPort) {
        showStatus('Error', 'Please fill in all fields');
        return;
    }

    const [remoteHost, remotePort] = remoteAddr.split(':');
    if (!remoteHost || !remotePort) {
        showStatus('Error', 'Invalid address format. Use: 127.0.0.1:22');
        return;
    }

    if (isNaN(localPort) || isNaN(remotePort)) {
        showStatus('Error', 'Port numbers must be numeric');
        return;
    }

    try {
        const clientId = selectedClient.ID || selectedClient.id;
        const response = await fetch('/api/proxy/create', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                clientId: clientId,
                remoteHost: remoteHost,
                remotePort: parseInt(remotePort),
                localPort: parseInt(localPort),
                protocol: protocol
            })
        });

        if (response.ok) {
            showStatus('Success', 'Proxy connection created!');
            document.getElementById('proxyRemoteAddr').value = '';
            document.getElementById('proxyLocalPort').value = '';
            setTimeout(loadProxies, 800);
        } else if (response.status === 401) {
            window.location.href = '/login';
        } else {
            showStatus('Error', 'Failed to create proxy');
        }
    } catch (err) {
        console.error('Error:', err);
        showStatus('Error', 'Error creating proxy connection');
    }
}

function updateClientPill() {
    const total = clients.length;
    const online = clients.filter(c => c.status === 'online').length;
    setText('clientsSummary', total === 0 ? 'No clients' : `${online}/${total} online`);
    setStatusDot('clientsDot', online > 0 ? 'healthy' : 'down');
}

function updateStats() {
    const total = clients.length;
    const online = clients.filter(c => c.status === 'online').length;
    const offline = total - online;

    setText('totalClients', total);
    setText('onlineClients', online);
    setText('offlineClients', offline);
    setText('activeProxies', '0'); // TODO: fetch actual proxy count

    // Update system overview using health data if present
    if (healthState && typeof healthState.uptime_seconds === 'number') {
        setText('serverUptime', formatDuration(healthState.uptime_seconds));
        setText('totalConnections', healthState.active_clients ?? total);
    } else {
        setText('serverUptime', '--');
        setText('totalConnections', total);
    }
    
    if (clients.length > 0) {
        const latest = clients.reduce((a, b) => 
            new Date(a.last_seen) > new Date(b.last_seen) ? a : b
        );
        const lastSeenDate = new Date(latest.last_seen);
        const minutesAgo = Math.floor((Date.now() - lastSeenDate.getTime()) / 60000);
        setText('lastActivity', minutesAgo < 1 ? 'Now' : minutesAgo + 'm ago');
    } else {
        setText('lastActivity', '--');
    }
}

async function refreshHealth() {
    const result = await fetchJSON('/api/health');
    if (!result.ok || !result.data) {
        setStatusDot('healthDot', 'down');
        setStatusDot('uptimeDot', 'down');
        setText('healthLabel', 'Unavailable');
        setText('uptimeLabel', '--');
        return;
    }

    healthState = result.data;
    const status = (healthState.status || 'unknown').toLowerCase();
    const dotState = status === 'healthy' ? 'healthy' : status === 'degraded' ? 'degraded' : 'down';

    setStatusDot('healthDot', dotState);
    setText('healthLabel', status === 'healthy' ? 'Healthy' : status === 'degraded' ? 'Degraded' : 'Check server');

    if (typeof healthState.uptime_seconds === 'number') {
        setStatusDot('uptimeDot', 'healthy');
        setText('uptimeLabel', formatDuration(healthState.uptime_seconds));
    } else {
        setStatusDot('uptimeDot', 'degraded');
        setText('uptimeLabel', '--');
    }

    // Active clients can inform connection load
    if (typeof healthState.active_clients === 'number') {
        setText('totalConnections', healthState.active_clients);
    }

    updateStats();
    updateClientPill();
}

function openClientPanel() {
    if (!selectedClient) return;
    window.open(`/client-details?id=${encodeURIComponent(selectedClient.id)}`, '_blank', 'width=1400,height=800');
}

async function saveAlias() {
    if (!selectedClient) return;
    
    const aliasInput = document.getElementById('aliasInput');
    const alias = aliasInput.value.trim();
    
    try {
        const response = await fetch('/api/client/alias', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                client_id: selectedClient.id,
                alias: alias
            })
        });
        
        if (response.ok) {
            selectedClient.alias = alias;
            showStatus('Success', 'Alias updated!');
            renderClientList();
        } else if (response.status === 401) {
            window.location.href = '/login';
        } else {
            showStatus('Error', 'Failed to update alias');
        }
    } catch (err) {
        console.error('Error:', err);
        showStatus('Error', 'Error updating alias');
    }
}

function sendCommand() {
    if (!selectedClient) return;
    window.open(`/terminal?client=${encodeURIComponent(selectedClient.id)}`, '_blank', 'width=1000,height=600');
}

function viewStats() {
    if (!selectedClient) {
        showStatus('Error', 'Please select a client first');
        return;
    }
    showStatus('Stats', `Client: ${selectedClient.hostname || selectedClient.id}\nOS: ${selectedClient.os}\nIP: ${selectedClient.ip}`);
}

function browseLogs() {
    if (!selectedClient) {
        showStatus('Error', 'Please select a client first');
        return;
    }
    showStatus('Logs', `Logs for ${selectedClient.hostname || selectedClient.id} would be displayed here`);
}

function confirmRemove() {
    if (!selectedClient) {
        showStatus('Error', 'Please select a client first');
        return;
    }
    showConfirm('Remove Client', `Remove ${escapeHtml(selectedClient.hostname || selectedClient.id)} from the list?`, removeClient);
}

function confirmUninstall() {
    if (!selectedClient) {
        showStatus('Error', 'Please select a client first');
        return;
    }
    showConfirm('Uninstall Client', `Uninstall from ${escapeHtml(selectedClient.hostname || selectedClient.id)}?\n\nThis cannot be undone.`, uninstallClient);
}

async function removeClient() {
    if (!selectedClient) {
        showStatus('Error', 'Please select a client first');
        return;
    }

    closeModal();

    try {
        const response = await fetch('/api/client', {
            method: 'DELETE',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ id: selectedClient.id })
        });

        if (!response.ok) {
            let message = 'Failed to remove client';
            try {
                const err = await response.json();
                if (err && err.error) message = err.error;
            } catch (_) {
                // ignore JSON parse errors
            }
            throw new Error(message);
        }

        showStatus('Removed', 'Client removed from list');
    } catch (err) {
        showStatus('Error', err.message || 'Failed to remove client');
        return;
    }

    selectedClient = null;
    // Clear middle and right columns
    document.getElementById('proxyMiddle').innerHTML = '<div style="text-align: center; padding: 60px 20px; color: var(--text-light);"><p style="font-size: 48px; margin-bottom: 15px;">🌐</p><p>Select a client to manage proxies</p></div>';
    document.getElementById('clientRight').innerHTML = '<div style="text-align: center; padding: 60px 20px; color: var(--text-light);"><p style="font-size: 48px; margin-bottom: 15px;">📋</p><p>Select a client to view details</p></div>';
    await loadClients();
}

function uninstallClient() {
    closeModal();
    showStatus('Uninstall Sent', 'Uninstall command sent to client');
    selectedClient = null;
    document.getElementById('proxyMiddle').innerHTML = '<div style="text-align: center; padding: 60px 20px; color: var(--text-light);"><p style="font-size: 48px; margin-bottom: 15px;">🌐</p><p>Select a client to manage proxies</p></div>';
    document.getElementById('clientRight').innerHTML = '<div style="text-align: center; padding: 60px 20px; color: var(--text-light);"><p style="font-size: 48px; margin-bottom: 15px;">📋</p><p>Select a client to view details</p></div>';
    setTimeout(() => loadClients(), 2000);
}

function openClientDetailsModal() {
    showStatus('Add Client', 'New client management coming soon');
}

function showSection(section) {
    // Update page title
    const titles = {
        'dashboard': 'Dashboard',
        'clients': 'Clients',
        'proxy': 'Proxy Management',
        'settings': 'Settings'
    };
    document.getElementById('pageTitle').textContent = titles[section] || 'Dashboard';

    // Update active tab in sidebar
    document.querySelectorAll('.sidebar-menu a').forEach(link => {
        link.classList.remove('active');
    });
    const activeLink = document.querySelector(`.sidebar-menu a[onclick*="${section}"]`);
    if (activeLink) activeLink.classList.add('active');

    // Show corresponding content section
    document.querySelectorAll('.content-section').forEach(sec => {
        sec.classList.remove('active');
    });
    const sectionMap = {
        'dashboard': 'dashboardSection',
        'clients': 'clientsSection',
        'proxy': 'proxySection',
        'users': 'usersSection',
        'settings': 'settingsSection'
    };
    const targetSection = document.getElementById(sectionMap[section]);
    if (targetSection) targetSection.classList.add('active');

    // If switching to clients tab, load clients
    if (section === 'clients') {
        loadClients();
    }
    // If switching to proxy tab, load all proxies
    if (section === 'proxy') {
        loadAllProxies();
    }
    // If switching to users tab, load users
    if (section === 'users') {
        loadUsers();
    }
    // If switching to settings tab, load settings
    if (section === 'settings') {
        loadUpdatePaths();
    }
}

function showStatus(title, message) {
    document.getElementById('statusTitle').textContent = title;
    document.getElementById('statusMessage').textContent = message;
    document.getElementById('statusModal').classList.add('active');
}

function showConfirm(title, message, callback) {
    confirmCallback = callback;
    document.getElementById('confirmTitle').textContent = title;
    document.getElementById('confirmMessage').textContent = message;
    document.getElementById('confirmModal').classList.add('active');
}

let confirmCallback = null;
function confirmAction() {
    if (confirmCallback) confirmCallback();
}

function closeModal() {
    document.getElementById('statusModal').classList.remove('active');
    document.getElementById('confirmModal').classList.remove('active');
}

function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

function formatDate(dateStr) {
    if (!dateStr) return 'N/A';
    const date = new Date(dateStr);
    return date.toLocaleString();
}

async function logout() {
    await fetch('/api/logout', { method: 'POST' });
    window.location.href = '/login';
}

// User Management Functions
async function loadUsers() {
    try {
        const response = await fetch('/api/users', {
            credentials: 'include'
        });
        if (response.status === 401) {
            window.location.href = '/login';
            return;
        }
        const users = await response.json() || [];
        renderUsersTable(users);
    } catch (err) {
        console.error('Error loading users:', err);
        document.getElementById('usersTableBody').innerHTML = `
            <tr><td colspan="7" style="text-align: center; color: var(--danger);">Error loading users</td></tr>
        `;
    }
}

function renderUsersTable(users) {
    console.log('Rendering users:', users); // Debug log
    const tbody = document.getElementById('usersTableBody');
    if (users.length === 0) {
        tbody.innerHTML = '<tr><td colspan="7" style="text-align: center; padding: 40px; color: var(--text-light);">No users found</td></tr>';
        return;
    }
    tbody.innerHTML = users.map(user => `
        <tr>
            <td><strong>${escapeHtml(user.Username)}</strong></td>
            <td>${escapeHtml(user.FullName || 'N/A')}</td>
            <td><span class="badge" style="background: var(--info); color: white; padding: 4px 8px; border-radius: 4px; font-size: 11px;">${escapeHtml(user.Role || 'user')}</span></td>
            <td><span class="status-badge ${user.Status === 'active' ? 'online' : 'offline'}">${escapeHtml(user.Status)}</span></td>
            <td>${formatDate(user.CreatedAt)}</td>
            <td>${user.LastLogin ? formatDate(user.LastLogin) : 'Never'}</td>
            <td>
                <button class="btn btn-sm" style="padding: 5px 10px; font-size: 12px; margin-right: 5px; background: var(--primary);" 
                        onclick="showEditUserForm('${escapeHtml(user.Username)}', '${escapeHtml(user.FullName || '')}', '${user.Role}')">
                    ✏️ Edit
                </button>
                <button class="btn btn-sm" style="padding: 5px 10px; font-size: 12px; margin-right: 5px; background: var(--warning);" 
                        onclick="toggleUserStatus('${escapeHtml(user.Username)}', '${user.Status}')">
                    ${user.Status === 'active' ? '🔒 Disable' : '🔓 Enable'}
                </button>
                <button class="btn btn-sm btn-danger" style="padding: 5px 10px; font-size: 12px;" 
                        onclick="deleteUser('${escapeHtml(user.Username)}')">
                    🗑️ Delete
                </button>
            </td>
        </tr>
    `).join('');
}

function showAddUserForm() {
    document.getElementById('addUserForm').style.display = 'block';
    document.getElementById('newUsername').value = '';
    document.getElementById('newPassword').value = '';
    document.getElementById('newFullName').value = '';
    document.getElementById('newRole').value = 'admin';
}

function hideAddUserForm() {
    document.getElementById('addUserForm').style.display = 'none';
}

function showEditUserForm(username, fullName, role) {
    document.getElementById('editUserForm').style.display = 'block';
    document.getElementById('editUsername').value = username;
    document.getElementById('editFullName').value = fullName;
    document.getElementById('editPassword').value = '';
    document.getElementById('editRole').value = role;
    // Scroll to the edit form
    document.getElementById('editUserForm').scrollIntoView({ behavior: 'smooth' });
}

function hideEditUserForm() {
    document.getElementById('editUserForm').style.display = 'none';
}

async function createUser() {
    const username = document.getElementById('newUsername').value.trim();
    const password = document.getElementById('newPassword').value;
    const fullName = document.getElementById('newFullName').value.trim();
    const role = document.getElementById('newRole').value;

    if (!username || !password) {
        showStatus('Error', 'Username and password are required');
        return;
    }

    if (password.length < 6) {
        showStatus('Error', 'Password must be at least 6 characters');
        return;
    }

    try {
        const response = await fetch('/api/users', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ username, password, full_name: fullName, role })
        });

        const result = await response.json();
        if (response.ok) {
            showStatus('Success', 'User created successfully');
            hideAddUserForm();
            loadUsers();
        } else {
            showStatus('Error', result.error || 'Failed to create user');
        }
    } catch (err) {
        console.error('Error creating user:', err);
        showStatus('Error', 'Failed to create user');
    }
}

async function toggleUserStatus(username, currentStatus) {
    const newStatus = currentStatus === 'active' ? 'inactive' : 'active';
    try {
        const response = await fetch(`/api/users/${encodeURIComponent(username)}`, {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ status: newStatus })
        });

        const result = await response.json();
        if (response.ok) {
            showStatus('Success', `User ${newStatus === 'active' ? 'enabled' : 'disabled'} successfully`);
            loadUsers();
        } else {
            showStatus('Error', result.error || 'Failed to update user');
        }
    } catch (err) {
        console.error('Error updating user:', err);
        showStatus('Error', 'Failed to update user');
    }
}

async function saveUserEdit() {
    const username = document.getElementById('editUsername').value.trim();
    const fullName = document.getElementById('editFullName').value.trim();
    const password = document.getElementById('editPassword').value;
    const role = document.getElementById('editRole').value;

    if (!username) {
        showStatus('Error', 'Username is required');
        return;
    }

    if (password && password.length < 6) {
        showStatus('Error', 'Password must be at least 6 characters');
        return;
    }

    try {
        const updateData = {
            full_name: fullName,
            role: role
        };

        // Only include password if it's provided
        if (password) {
            updateData.password = password;
        }

        const response = await fetch(`/api/users/${encodeURIComponent(username)}`, {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(updateData)
        });

        const result = await response.json();
        if (response.ok) {
            showStatus('Success', 'User updated successfully');
            hideEditUserForm();
            loadUsers();
        } else {
            showStatus('Error', result.error || 'Failed to update user');
        }
    } catch (err) {
        console.error('Error updating user:', err);
        showStatus('Error', 'Failed to update user');
    }
}

async function deleteUser(username) {
    showConfirm('Delete User', `Are you sure you want to delete user "${username}"?`, async () => {
        try {
            const response = await fetch(`/api/users/${encodeURIComponent(username)}`, {
                method: 'DELETE'
            });

            const result = await response.json();
            if (response.ok) {
                showStatus('Success', 'User deleted successfully');
                loadUsers();
            } else {
                showStatus('Error', result.error || 'Failed to delete user');
            }
            closeModal();
        } catch (err) {
            console.error('Error deleting user:', err);
            showStatus('Error', 'Failed to delete user');
            closeModal();
        }
    });
}

async function loadUpdatePaths() {
    // Load settings from API endpoint
    const platforms = [
        { id: 'updatePathWindowsAmd64', key: 'windows-amd64' },
        { id: 'updatePathWindows386', key: 'windows-386' },
        { id: 'updatePathLinuxAmd64', key: 'linux-amd64' },
        { id: 'updatePathLinux386', key: 'linux-386' },
        { id: 'updatePathDarwinAmd64', key: 'darwin-amd64' },
        { id: 'updatePathDarwinArm64', key: 'darwin-arm64' }
    ];
    
    try {
        const response = await fetch('/api/settings');
        if (response.status === 401) {
            window.location.href = '/login';
            return;
        }
        
        if (response.ok) {
            const settings = await response.json() || {};
            platforms.forEach(platform => {
                const el = document.getElementById(platform.id);
                if (el) {
                    el.value = settings[`update_path_${platform.key}`] || '';
                }
            });
        }
    } catch (err) {
        console.error('Error loading settings:', err);
    }
}

async function saveUpdatePaths() {
    const platforms = [
        { id: 'updatePathWindowsAmd64', key: 'windows-amd64' },
        { id: 'updatePathWindows386', key: 'windows-386' },
        { id: 'updatePathLinuxAmd64', key: 'linux-amd64' },
        { id: 'updatePathLinux386', key: 'linux-386' },
        { id: 'updatePathDarwinAmd64', key: 'darwin-amd64' },
        { id: 'updatePathDarwinArm64', key: 'darwin-arm64' }
    ];
    
    const settings = {};
    platforms.forEach(platform => {
        const el = document.getElementById(platform.id);
        if (el) {
            const value = el.value.trim();
            if (value) {
                settings[`update_path_${platform.key}`] = value;
            }
        }
    });
    
    try {
        const response = await fetch('/api/settings', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(settings)
        });

        if (response.ok) {
            showStatus('Success', 'Update paths saved successfully!');
        } else if (response.status === 401) {
            window.location.href = '/login';
        } else {
            showStatus('Error', 'Failed to save settings');
        }
    } catch (err) {
        console.error('Error saving settings:', err);
        showStatus('Error', 'Error saving settings');
    }
}

function clearUpdateForm() {
    document.getElementById('updatePlatform').value = '';
    document.getElementById('updateVersion').value = '';
    document.getElementById('updateForce').value = 'false';
    document.getElementById('updateStats').style.display = 'none';
    document.getElementById('updateLogContainer').style.display = 'none';
}

async function pushClientsUpdate() {
    const platform = document.getElementById('updatePlatform').value.trim();
    const version = document.getElementById('updateVersion').value.trim();
    const forceUpdate = document.getElementById('updateForce').value === 'true';

    if (!platform) {
        showStatus('Error', 'Please select a platform');
        return;
    }

    if (!version) {
        showStatus('Error', 'Please enter a version number');
        return;
    }

    // Show update log
    document.getElementById('updateStats').style.display = 'block';
    document.getElementById('updateLogContainer').style.display = 'block';
    document.getElementById('updateLog').innerHTML = '<p style="color: var(--text-light);">Processing...</p>';

    try {
        const response = await fetch('/api/push-update', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                platform: platform,
                version: version,
                force: forceUpdate
            })
        });

        if (response.status === 401) {
            window.location.href = '/login';
            return;
        }

        const result = await response.json();

        // Update stats
        document.getElementById('matchingCount').textContent = result.total_matching || 0;
        document.getElementById('updatesSent').textContent = result.updates_sent || 0;
        document.getElementById('updatesFailed').textContent = result.updates_failed || 0;

        // Update log
        const logLines = (result.log || []).map(entry => {
            const time = entry.timestamp ? new Date(entry.timestamp).toLocaleTimeString() : '--:--:--';
            const status = entry.status ? `[${entry.status.toUpperCase()}]` : '[INFO]';
            const message = entry.message || entry.client_id || '';
            return `${time} ${status} ${message}`;
        });

        document.getElementById('updateLog').innerHTML = logLines.length > 0 
            ? logLines.map(line => `<div>${escapeHtml(line)}</div>`).join('')
            : '<p style="color: var(--text-light);">No log entries</p>';

        if (response.ok) {
            showStatus('Success', `Update pushed to ${result.updates_sent || 0} client(s)`);
        } else {
            showStatus('Partial Success', `Pushed to ${result.updates_sent || 0}, failed: ${result.updates_failed || 0}`);
        }
    } catch (err) {
        console.error('Error pushing update:', err);
        document.getElementById('updateLog').innerHTML = `<p style="color: var(--danger);">Error: ${escapeHtml(err.message)}</p>`;
        showStatus('Error', 'Failed to push update');
    }
}

// Proxy Management (All Proxies View)
async function loadAllProxies() {
    try {
        const response = await fetch('/api/proxy/list');
        if (response.status === 401) {
            window.location.href = '/login';
            return;
        }
        if (response.ok) {
            const proxies = await response.json() || [];
            renderAllProxies(proxies);
        } else {
            renderAllProxies([]);
        }
    } catch (err) {
        console.error('Error loading proxies:', err);
        renderAllProxies([]);
    }
}

function renderAllProxies(proxies) {
    const container = document.getElementById('allProxiesList');
    if (!container) return;

    if (!proxies || proxies.length === 0) {
        container.innerHTML = `
            <div style="text-align: center; padding: 40px; color: var(--text-light);">
                <p style="font-size: 18px; margin-bottom: 10px;">🌐</p>
                <p>No active proxy connections</p>
                <p style="font-size: 13px; margin-top: 8px;">Go to the Clients tab to create proxies</p>
            </div>
        `;
        return;
    }

    // Group proxies by client
    const proxyByClient = {};
    proxies.forEach(proxy => {
        if (!proxyByClient[proxy.ClientID]) {
            proxyByClient[proxy.ClientID] = [];
        }
        proxyByClient[proxy.ClientID].push(proxy);
    });

    container.innerHTML = Object.keys(proxyByClient).map(clientId => {
        const clientProxies = proxyByClient[clientId];
        const bytesInTotal = clientProxies.reduce((sum, p) => sum + (p.BytesIn || 0), 0);
        const bytesOutTotal = clientProxies.reduce((sum, p) => sum + (p.BytesOut || 0), 0);
        
        return `
            <div style="background: var(--surface); border: 1px solid var(--border); border-radius: 8px; padding: 20px; margin-bottom: 16px;">
                <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 16px;">
                    <h4 style="margin: 0; font-size: 15px;">
                        🖥️ Client: <span style="font-family: monospace; font-size: 13px;">${escapeHtml(clientId.substring(0, 16))}...</span>
                    </h4>
                    <span style="background: var(--info); color: white; padding: 4px 12px; border-radius: 12px; font-size: 12px; font-weight: 500;">
                        ${clientProxies.length} ${clientProxies.length === 1 ? 'proxy' : 'proxies'}
                    </span>
                </div>
                
                <div style="display: grid; gap: 12px;">
                    ${clientProxies.map(proxy => {
                        const statusColor = (proxy.UserCount || 0) > 0 ? 'var(--success)' : 'var(--text-light)';
                        const bytesIn = formatBytes(proxy.BytesIn || 0);
                        const bytesOut = formatBytes(proxy.BytesOut || 0);
                        const lastActive = proxy.LastActive ? new Date(proxy.LastActive).toLocaleString() : 'Never';
                        
                        return `
                            <div style="background: white; border: 1px solid var(--border); border-radius: 6px; padding: 16px;">
                                <div style="display: flex; justify-content: space-between; align-items: start; margin-bottom: 12px;">
                                    <div style="flex: 1;">
                                        <div style="font-weight: 600; font-size: 14px; margin-bottom: 6px; color: var(--primary);">
                                            :<span style="color: var(--success); font-weight: 700;">${proxy.LocalPort}</span> → ${escapeHtml(proxy.RemoteHost)}:${proxy.RemotePort}
                                        </div>
                                        <div style="display: flex; gap: 16px; font-size: 12px; color: var(--text-light);">
                                            <span>📡 ${escapeHtml(proxy.Protocol?.toUpperCase() || 'TCP')}</span>
                                            <span style="color: ${statusColor};">
                                                ${(proxy.UserCount || 0) > 0 ? `🟢 ${proxy.UserCount} active` : '⚪ Idle'}
                                            </span>
                                        </div>
                                    </div>
                                    <button onclick="deleteProxyFromAll('${escapeHtml(proxy.ID)}')" 
                                            style="padding: 6px 12px; background: var(--danger); color: white; border: none; border-radius: 4px; cursor: pointer; font-size: 12px;">
                                        🗑️ Close
                                    </button>
                                </div>
                                
                                <div style="display: grid; grid-template-columns: repeat(3, 1fr); gap: 12px; padding-top: 12px; border-top: 1px solid var(--border); font-size: 12px;">
                                    <div>
                                        <div style="color: var(--text-light); margin-bottom: 4px;">⬇️ Received</div>
                                        <div style="font-weight: 600;">${bytesIn}</div>
                                    </div>
                                    <div>
                                        <div style="color: var(--text-light); margin-bottom: 4px;">⬆️ Sent</div>
                                        <div style="font-weight: 600;">${bytesOut}</div>
                                    </div>
                                    <div>
                                        <div style="color: var(--text-light); margin-bottom: 4px;">⏱️ Last Active</div>
                                        <div style="font-weight: 600; font-size: 11px;">${lastActive.split(',')[1] || lastActive}</div>
                                    </div>
                                </div>
                            </div>
                        `;
                    }).join('')}
                </div>
                
                <div style="margin-top: 12px; padding-top: 12px; border-top: 1px solid var(--border); display: flex; gap: 24px; font-size: 13px; color: var(--text-light);">
                    <span>📊 Total In: <strong style="color: var(--text);">${formatBytes(bytesInTotal)}</strong></span>
                    <span>📊 Total Out: <strong style="color: var(--text);">${formatBytes(bytesOutTotal)}</strong></span>
                </div>
            </div>
        `;
    }).join('');
}

async function deleteProxyFromAll(proxyId) {
    if (!confirm('Close this proxy connection?')) return;
    
    try {
        const response = await fetch(`/api/proxy/close?id=${encodeURIComponent(proxyId)}`, {
            method: 'POST'
        });

        if (response.ok) {
            showStatus('Success', 'Proxy connection closed!');
            setTimeout(loadAllProxies, 500);
        } else if (response.status === 401) {
            window.location.href = '/login';
        } else {
            showStatus('Error', 'Failed to close proxy');
        }
    } catch (err) {
        console.error('Error:', err);
        showStatus('Error', 'Error closing proxy connection');
    }
}

function formatBytes(bytes) {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return Math.round((bytes / Math.pow(k, i)) * 100) / 100 + ' ' + sizes[i];
}

// Initialize
loadClients();
setInterval(loadClients, 10000);
refreshHealth();
setInterval(refreshHealth, 12000);

// Set initial active tab
showSection('dashboard');
