/**
 * Terminal Page JavaScript
 * Handles WebSocket communication and terminal UI
 */

const clientId = document.body.dataset.clientId || window.location.hash.slice(1);
const terminal = document.getElementById('terminal');
const commandInput = document.getElementById('commandInput');
const statusEl = document.getElementById('status');
let ws;
let commandHistory = [];
let historyIndex = -1;

/**
 * Connect to terminal WebSocket
 */
function connectTerminal() {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = `${protocol}//${window.location.host}/api/terminal?client=${encodeURIComponent(clientId)}`;
    
    ws = new WebSocket(wsUrl);
    
    ws.onopen = () => {
        updateTerminalStatus('connected');
        addTerminalOutput('Connected to terminal session', 'success');
    };
    
    ws.onmessage = (event) => {
        try {
            const data = JSON.parse(event.data);
            
            if (data.type === 'output') {
                addTerminalOutput(data.data);
            } else if (data.type === 'error') {
                addTerminalOutput(data.data, 'error');
            } else if (data.type === 'exit') {
                addTerminalOutput(`Process exited with code ${data.code}`, 'info');
            }
        } catch (err) {
            console.error('Failed to parse message:', err);
        }
    };
    
    ws.onerror = (error) => {
        addTerminalOutput('WebSocket error occurred', 'error');
        console.error('WebSocket error:', error);
    };
    
    ws.onclose = () => {
        updateTerminalStatus('disconnected');
        addTerminalOutput('Disconnected from terminal session', 'error');
        // Attempt to reconnect after 3 seconds
        setTimeout(() => {
            addTerminalOutput('Attempting to reconnect...', 'info');
            connectTerminal();
        }, 3000);
    };
}

/**
 * Update terminal connection status
 * @param {string} status - 'connected' or 'disconnected'
 */
function updateTerminalStatus(status) {
    if (!statusEl) return;
    statusEl.className = `status ${status}`;
    statusEl.textContent = status === 'connected' ? 'Connected' : 'Disconnected';
}

/**
 * Add output line to terminal
 * @param {string} text - Text to display
 * @param {string} className - CSS class (optional)
 */
function addTerminalOutput(text, className = '') {
    if (!terminal) return;
    
    const line = document.createElement('div');
    line.className = `terminal-line ${className}`;
    line.textContent = text;
    line.style.whiteSpace = 'pre-wrap';
    line.style.wordWrap = 'break-word';
    
    terminal.appendChild(line);
    terminal.scrollTop = terminal.scrollHeight;
}

/**
 * Send command to terminal
 * @param {string} command - Command to execute
 */
function sendTerminalCommand(command) {
    if (!ws || ws.readyState !== WebSocket.OPEN) {
        addTerminalOutput('Not connected to server', 'error');
        return;
    }
    
    addTerminalOutput(`$ ${command}`, 'prompt');
    
    try {
        ws.send(JSON.stringify({
            type: 'input',
            data: command + '\n'
        }));
        
        // Add to history
        commandHistory.unshift(command);
        if (commandHistory.length > 100) {
            commandHistory.pop();
        }
        historyIndex = -1;
    } catch (err) {
        console.error('Failed to send command:', err);
        addTerminalOutput('Failed to send command', 'error');
    }
}

/**
 * Clear terminal output
 */
function clearTerminal() {
    if (terminal) {
        terminal.innerHTML = '';
    }
}

/**
 * Send interrupt signal (Ctrl+C)
 */
function interruptCommand() {
    if (!ws || ws.readyState !== WebSocket.OPEN) {
        addTerminalOutput('Not connected to server', 'error');
        return;
    }
    
    try {
        ws.send(JSON.stringify({
            type: 'interrupt'
        }));
        addTerminalOutput('^C', 'prompt');
    } catch (err) {
        console.error('Failed to send interrupt:', err);
    }
}

/**
 * Navigate command history
 * @param {number} direction - 1 for up, -1 for down
 */
function navigateCommandHistory(direction) {
    if (direction > 0) {
        // Up arrow
        if (historyIndex < commandHistory.length - 1) {
            historyIndex++;
            commandInput.value = commandHistory[historyIndex];
        }
    } else {
        // Down arrow
        if (historyIndex > 0) {
            historyIndex--;
            commandInput.value = commandHistory[historyIndex];
        } else {
            historyIndex = -1;
            commandInput.value = '';
        }
    }
}

// Initialize event listeners
document.addEventListener('DOMContentLoaded', function() {
    // Wire toolbar buttons without inline handlers
    const btnClear = document.getElementById('btnClear');
    const btnInterrupt = document.getElementById('btnInterrupt');
    if (btnClear) btnClear.addEventListener('click', () => clearTerminal());
    if (btnInterrupt) btnInterrupt.addEventListener('click', () => interruptCommand());

    if (commandInput) {
        commandInput.addEventListener('keydown', function(e) {
            if (e.key === 'Enter') {
                const command = commandInput.value.trim();
                if (command) {
                    sendTerminalCommand(command);
                    commandInput.value = '';
                }
            } else if (e.key === 'ArrowUp') {
                e.preventDefault();
                navigateCommandHistory(1);
            } else if (e.key === 'ArrowDown') {
                e.preventDefault();
                navigateCommandHistory(-1);
            }
        });
        
        // Focus input on load
        commandInput.focus();
    }
    
    // Connect to WebSocket
    connectTerminal();
});
