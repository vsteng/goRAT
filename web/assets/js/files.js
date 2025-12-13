/**
 * File Manager Page JavaScript
 * Handles file browsing, download, and upload operations
 */

const clientId = document.body.dataset.clientId;
const clientOS = (document.body.dataset.clientOs || 'linux').toLowerCase();

let currentPath = clientOS === 'windows' ? 'C:\\' : '/';
let drivesCache = null;

/**
 * Normalize Windows path format
 * @param {string} path - Path to normalize
 * @returns {string} Normalized path
 */
function normalizeWindowsPath(path) {
    if (!path) return 'C:\\';
    path = path.replace(/\//g, '\\');          // Convert all / → \
    path = path.replace(/\\\\+/g, '\\');       // Collapse multiple slashes
    if (/^[a-z]:$/i.test(path)) path += '\\';  // Turn C: → C:\
    return path;
}

/**
 * Normalize Unix path format
 * @param {string} path - Path to normalize
 * @returns {string} Normalized path
 */
function normalizeUnixPath(path) {
    if (!path) return '/';
    path = path.replace(/\\/g, '/');           // Convert \ → /
    path = path.replace(/\/+/g, '/');          // Collapse ///
    if (!path.startsWith('/')) path = '/' + path;
    return path;
}

/**
 * Normalize path based on OS
 * @param {string} path - Path to normalize
 * @returns {string} Normalized path
 */
function normalizePath(path) {
    return clientOS === 'windows'
        ? normalizeWindowsPath(path)
        : normalizeUnixPath(path);
}

/**
 * Load available drives (Windows only)
 */
async function loadDrives() {
    if (clientOS !== 'windows') return;
    
    try {
        const response = await fetch('/api/files/drives', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({
                client_id: clientId
            })
        });
        
        if (!response.ok) {
            throw new Error('Failed to get drives');
        }
        
        const result = await response.json();
        drivesCache = result.drives || [];
    } catch (err) {
        console.error('Error loading drives:', err);
        drivesCache = [];
    }
}

/**
 * Toggle drives panel visibility
 */
function showDrives() {
    const panel = document.getElementById('drivesPanel');
    const isVisible = panel.style.display !== 'none';
    panel.style.display = isVisible ? 'none' : 'block';
    if (isVisible) return;
    
    const drivesList = document.getElementById('drivesList');
    
    if (!drivesCache || drivesCache.length === 0) {
        drivesList.innerHTML = '<div style="text-align:center; padding:20px; color:#888;">No drives found</div>';
        return;
    }
    
    drivesList.innerHTML = drivesCache.map(drive => {
        return `
            <div class="drive-card" data-path="${escapeHtml(drive.name)}" style="background:white; padding:15px; border-radius:5px; border:1px solid #e1e1e1; cursor:pointer; transition:all 0.2s;">
                <div style="font-weight:600; margin-bottom:8px;">💾 ${escapeHtml(drive.name)}</div>
                <div style="font-size:12px; color:#666; margin-bottom:6px;">${escapeHtml(drive.label || 'Local Disk')}</div>
                <div style="font-size:11px; color:#999;">
                    ${drive.type} | 
                    ${formatSize(drive.free_size)} free of ${formatSize(drive.total_size)}
                </div>
            </div>
        `;
    }).join('');
}

/**
 * Browse a specific drive (Windows)
 * @param {string} drivePath - Drive path to browse
 */
function browseDrive(drivePath) {
    document.getElementById('drivesPanel').style.display = 'none';
    browsePath(drivePath);
}

/**
 * Browse a file path
 * @param {string} path - Path to browse (optional, uses prompt if not provided)
 */
async function browsePath(path) {
    if (path) {
        currentPath = normalizePath(path);
    } else {
        const input = prompt('Enter path:', currentPath);
        if (!input) return;
        currentPath = normalizePath(input);
    }
    
    const pathInput = document.getElementById('currentPath');
    if (pathInput) pathInput.value = currentPath;
    
    try {
        const response = await fetch('/api/files/browse', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({
                client_id: clientId,
                path: currentPath
            })
        });
        
        if (!response.ok) {
            throw new Error('Failed to browse files');
        }
        
        const result = await response.json();
        
        const files = (result.files || []).map(f => {
            f.path = normalizePath(f.path);
            return f;
        });
        displayFiles(files);
    } catch (err) {
        showNotification('Error', err.message, 'error');
        const tbody = document.getElementById('fileList');
        if (tbody) {
            tbody.innerHTML = '<tr><td colspan="4" style="text-align:center;padding:40px;color:#c33;">Error loading files</td></tr>';
        }
    }
}

/**
 * Display files in table
 * @param {Array} files - Array of file objects
 */
function displayFiles(files) {
    const tbody = document.getElementById('fileList');
    if (!tbody) return;
    
    if (files.length === 0) {
        tbody.innerHTML = '<tr><td colspan="4" style="text-align:center;padding:40px;color:#888;">Empty directory</td></tr>';
        return;
    }
    
    tbody.innerHTML = files.map(file => {
        return `
            <tr>
                <td>
                    <span class="file-icon">${file.is_dir ? '📁' : '📄'}</span>
                    <span class="file-name" data-path="${escapeHtml(file.path)}" data-is-dir="${file.is_dir}">${escapeHtml(file.name)}</span>
                </td>
                <td>${file.is_dir ? '-' : formatSize(file.size)}</td>
                <td>${formatDate(file.mod_time)}</td>
                <td>
                    ${!file.is_dir ? `<button class="action-btn" data-action="download" data-path="${escapeHtml(file.path)}">⬇️ Download</button>` : ''}
                </td>
            </tr>
        `;
    }).join('');
}

/**
 * Handle file click (directory navigation or download prompt)
 * @param {string} path - File path
 * @param {boolean} isDir - Is directory
 */
function handleFileClick(path, isDir) {
    if (isDir) {
        browsePath(path);
    } else {
        if (confirm('Download this file?')) {
            downloadFile(path);
        }
    }
}

/**
 * Download a file
 * @param {string} path - File path to download
 */
async function downloadFile(path) {
    try {
        const response = await fetch('/api/files/download', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({
                client_id: clientId,
                path: path
            })
        });
        
        if (!response.ok) {
            throw new Error('Download failed');
        }
        
        const blob = await response.blob();
        const url = window.URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.href = url;
        // Extract filename from path (handle both / and \ separators)
        const fileName = path.split(/[\\/]/).pop();
        a.download = fileName;
        document.body.appendChild(a);
        a.click();
        window.URL.revokeObjectURL(url);
        a.remove();
        
        showNotification('Success', `Downloaded ${escapeHtml(fileName)}`, 'success');
    } catch (err) {
        showNotification('Error', `Failed to download: ${err.message}`, 'error');
    }
}

/**
 * Navigate to parent directory
 */
function goUp() {
    if (clientOS === 'windows') {
        // Windows path handling
        const parts = currentPath.split('\\').filter(p => p);
        if (parts.length > 1) {  // Keep at least the drive letter
            parts.pop();
            currentPath = parts.join('\\') + '\\';
            browsePath(currentPath);
        }
    } else {
        // Unix path handling
        const parts = currentPath.split('/').filter(p => p);
        if (parts.length > 0) {
            parts.pop();
            currentPath = '/' + parts.join('/');
            browsePath(currentPath);
        }
    }
}

/**
 * Navigate to home directory
 */
function goHome() {
    currentPath = clientOS === 'windows' ? 'C:\\' : '/';
    browsePath(currentPath);
}

/**
 * Refresh current directory
 */
function refresh() {
    browsePath(currentPath);
}

// Initialize on page load
document.addEventListener('DOMContentLoaded', function() {
    // Show drives button for Windows clients
    const drivesBtn = document.getElementById('drivesBtn');
    if (clientOS === 'windows' && drivesBtn) {
        drivesBtn.style.display = 'inline-block';
        loadDrives();
    }
    
    // Wire toolbar buttons without inline handlers
    const btnBrowse = document.getElementById('btnBrowse');
    const btnUp = document.getElementById('btnUp');
    const btnHome = document.getElementById('btnHome');
    const btnRefresh = document.getElementById('btnRefresh');
    if (btnBrowse) btnBrowse.addEventListener('click', () => browsePath());
    if (btnUp) btnUp.addEventListener('click', () => goUp());
    if (btnHome) btnHome.addEventListener('click', () => goHome());
    if (btnRefresh) btnRefresh.addEventListener('click', () => refresh());

    // Drives button
    const drivesBtn2 = document.getElementById('drivesBtn');
    if (drivesBtn2) drivesBtn2.addEventListener('click', () => showDrives());

    // Event delegation for file list interactions
    const tbody = document.getElementById('fileList');
    if (tbody) {
        tbody.addEventListener('click', (e) => {
            const nameEl = e.target.closest('.file-name');
            if (nameEl) {
                const path = nameEl.dataset.path;
                const isDir = nameEl.dataset.isDir === 'true' || nameEl.dataset.isDir === true;
                if (isDir) {
                    browsePath(path);
                } else {
                    if (confirm('Download this file?')) {
                        downloadFile(path);
                    }
                }
                return;
            }

            const btn = e.target.closest('button.action-btn');
            if (btn && btn.dataset.action === 'download') {
                const path = btn.dataset.path;
                if (path) downloadFile(path);
            }
        });
    }

    // Event delegation for drives panel cards
    const drivesList = document.getElementById('drivesList');
    if (drivesList) {
        drivesList.addEventListener('click', (e) => {
            const card = e.target.closest('.drive-card');
            if (card && card.dataset.path) browseDrive(card.dataset.path);
        });
    }

    // Set initial path
    if (clientOS === 'windows') {
        currentPath = 'C:\\';
        const pathInput = document.getElementById('currentPath');
        if (pathInput) pathInput.value = currentPath;
        setTimeout(() => browsePath(currentPath), 500);
    } else {
        const pathInput = document.getElementById('currentPath');
        if (pathInput) pathInput.value = currentPath;
        browsePath(currentPath);
    }
});
