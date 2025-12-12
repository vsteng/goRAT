/**
 * Common JavaScript Utilities
 * Shared functions across all pages
 */

/**
 * Fetch helper with error handling and authentication
 * @param {string} url - The URL to fetch
 * @param {object} options - Fetch options
 * @returns {Promise} Response JSON or throws error
 */
async function fetchJSON(url, options = {}) {
    const defaultOptions = {
        headers: {
            'Content-Type': 'application/json',
            ...options.headers
        },
        credentials: 'include',
        ...options
    };

    const response = await fetch(url, defaultOptions);
    
    if (response.status === 401) {
        // Redirect to login if unauthorized
        window.location.href = '/login';
        return null;
    }

    if (!response.ok) {
        throw new Error(`HTTP ${response.status}: ${response.statusText}`);
    }

    return response.json();
}

/**
 * Format bytes into human-readable format
 * @param {number} bytes - Number of bytes
 * @returns {string} Formatted string (B, KB, MB, GB)
 */
function formatBytes(bytes) {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return Math.round((bytes / Math.pow(k, i)) * 100) / 100 + ' ' + sizes[i];
}

/**
 * Format duration in milliseconds to readable format
 * @param {number} ms - Duration in milliseconds
 * @returns {string} Formatted duration (e.g., "1h 23m 45s")
 */
function formatDuration(ms) {
    if (!ms || ms <= 0) return '0s';
    
    const totalSeconds = Math.floor(ms / 1000);
    const hours = Math.floor(totalSeconds / 3600);
    const minutes = Math.floor((totalSeconds % 3600) / 60);
    const seconds = totalSeconds % 60;
    
    const parts = [];
    if (hours > 0) parts.push(`${hours}h`);
    if (minutes > 0) parts.push(`${minutes}m`);
    if (seconds > 0) parts.push(`${seconds}s`);
    
    return parts.join(' ') || '0s';
}

/**
 * Escape HTML special characters
 * @param {string} text - Text to escape
 * @returns {string} Escaped HTML
 */
function escapeHtml(text) {
    if (!text) return '';
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

/**
 * Format date to locale string
 * @param {string} dateStr - Date string (ISO format)
 * @returns {string} Formatted date
 */
function formatDate(dateStr) {
    if (!dateStr) return 'N/A';
    try {
        const date = new Date(dateStr);
        return date.toLocaleString();
    } catch (e) {
        return dateStr;
    }
}

/**
 * Format date to time only
 * @param {string} dateStr - Date string (ISO format)
 * @returns {string} Time in HH:MM:SS format
 */
function formatTime(dateStr) {
    if (!dateStr) return 'N/A';
    try {
        const date = new Date(dateStr);
        return date.toLocaleTimeString();
    } catch (e) {
        return dateStr;
    }
}

/**
 * Show a notification/status message
 * @param {string} title - Title of notification
 * @param {string} message - Message body
 * @param {string} type - Type: 'success', 'error', 'warning', 'info'
 * @param {number} duration - Duration to show (ms), 0 = persistent
 */
function showNotification(title, message, type = 'info', duration = 3000) {
    const notification = document.createElement('div');
    notification.className = `notification notification-${type}`;
    notification.innerHTML = `
        <div class="notification-content">
            <div class="notification-title">${escapeHtml(title)}</div>
            <div class="notification-message">${escapeHtml(message)}</div>
        </div>
        <button class="notification-close" onclick="this.parentElement.remove()">×</button>
    `;
    
    // Add to page if not already present
    let container = document.getElementById('notifications-container');
    if (!container) {
        container = document.createElement('div');
        container.id = 'notifications-container';
        container.style.cssText = `
            position: fixed;
            top: 20px;
            right: 20px;
            z-index: 9999;
            max-width: 400px;
            width: 90%;
        `;
        document.body.appendChild(container);
    }
    
    container.appendChild(notification);
    
    if (duration > 0) {
        setTimeout(() => {
            if (notification.parentElement) {
                notification.remove();
            }
        }, duration);
    }
    
    return notification;
}

/**
 * Copy text to clipboard
 * @param {string} text - Text to copy
 * @returns {Promise<boolean>} Success status
 */
async function copyToClipboard(text) {
    try {
        if (navigator.clipboard && window.isSecureContext) {
            await navigator.clipboard.writeText(text);
            return true;
        } else {
            // Fallback for older browsers
            const textarea = document.createElement('textarea');
            textarea.value = text;
            textarea.style.position = 'fixed';
            textarea.style.opacity = '0';
            document.body.appendChild(textarea);
            textarea.select();
            const result = document.execCommand('copy');
            textarea.remove();
            return result;
        }
    } catch (err) {
        console.error('Failed to copy:', err);
        return false;
    }
}

/**
 * Debounce function calls
 * @param {function} func - Function to debounce
 * @param {number} delay - Delay in milliseconds
 * @returns {function} Debounced function
 */
function debounce(func, delay) {
    let timeoutId;
    return function(...args) {
        clearTimeout(timeoutId);
        timeoutId = setTimeout(() => func.apply(this, args), delay);
    };
}

/**
 * Throttle function calls
 * @param {function} func - Function to throttle
 * @param {number} limit - Minimum time between calls in milliseconds
 * @returns {function} Throttled function
 */
function throttle(func, limit) {
    let inThrottle;
    return function(...args) {
        if (!inThrottle) {
            func.apply(this, args);
            inThrottle = true;
            setTimeout(() => inThrottle = false, limit);
        }
    };
}

/**
 * Get URL query parameters
 * @returns {object} Query parameters as key-value pairs
 */
function getQueryParams() {
    const params = {};
    const searchParams = new URLSearchParams(window.location.search);
    for (const [key, value] of searchParams) {
        params[key] = value;
    }
    return params;
}

/**
 * Set document title with suffix
 * @param {string} title - Page title
 * @param {string} suffix - Suffix (e.g., "RAT Dashboard")
 */
function setPageTitle(title, suffix = 'RAT') {
    document.title = suffix ? `${title} - ${suffix}` : title;
}

/**
 * Add CSS class with animation
 * @param {HTMLElement} element - Element to animate
 * @param {string} className - Class name
 * @param {number} duration - Animation duration in ms
 */
function animateClass(element, className, duration = 300) {
    element.classList.add(className);
    setTimeout(() => element.classList.remove(className), duration);
}

/**
 * Show/hide loading spinner
 * @param {boolean} show - Show or hide
 * @param {string} message - Optional message
 */
function showLoadingSpinner(show = true, message = 'Loading...') {
    let spinner = document.getElementById('global-spinner');
    if (!spinner && show) {
        spinner = document.createElement('div');
        spinner.id = 'global-spinner';
        spinner.innerHTML = `
            <div class="spinner-overlay">
                <div class="spinner-content">
                    <div class="spinner"></div>
                    <p>${escapeHtml(message)}</p>
                </div>
            </div>
        `;
        spinner.style.cssText = `
            position: fixed;
            top: 0;
            left: 0;
            right: 0;
            bottom: 0;
            background: rgba(0, 0, 0, 0.5);
            display: flex;
            align-items: center;
            justify-content: center;
            z-index: 9998;
        `;
        document.body.appendChild(spinner);
    } else if (spinner && !show) {
        spinner.remove();
    }
}

/**
 * Logout user and redirect to login
 * @returns {Promise<void>}
 */
async function logoutUser() {
    try {
        await fetch('/api/logout', { method: 'POST' });
    } catch (err) {
        console.error('Logout error:', err);
    }
    window.location.href = '/login';
}

/**
 * Add CSS to page dynamically
 * @param {string} href - CSS file path
 */
function addStylesheet(href) {
    const link = document.createElement('link');
    link.rel = 'stylesheet';
    link.href = href;
    document.head.appendChild(link);
}

/**
 * Add script to page dynamically
 * @param {string} src - Script file path
 * @param {object} options - Script options (async, defer, etc.)
 * @returns {Promise<void>}
 */
function addScript(src, options = {}) {
    return new Promise((resolve, reject) => {
        const script = document.createElement('script');
        script.src = src;
        
        if (options.async !== false) script.async = true;
        if (options.defer) script.defer = true;
        if (options.type) script.type = options.type;
        
        script.onload = resolve;
        script.onerror = reject;
        
        document.body.appendChild(script);
    });
}
