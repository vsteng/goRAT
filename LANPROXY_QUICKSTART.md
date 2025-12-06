# LanProxy Integration - Quick Start Guide

## Accessing the New Features

### 1. Enhanced Dashboard

**URL**: `http://localhost:8080/dashboard-new`

The new dashboard replaces the old one and provides:
- Real-time client list with status indicators
- Statistics cards showing online/offline counts
- Click any client to view detailed information
- Slide-out details panel with quick actions

**Navigation:**
- Click on "Dashboard" in sidebar (active by default)
- Click any client name to open details panel
- Use "Control" button to open client management page
- Use "Terminal" button for remote command execution

### 2. Client Details & Management

**URL**: `http://localhost:8080/client-details?id={CLIENT_ID}`

Provides comprehensive client management with tabs:

#### Overview Tab
- CPU, Memory, Disk, and Uptime stats
- Hardware information (CPU cores, total memory)
- Network information (local and public IPs)
- Installation and version details

#### File Browser Tab
- Navigate client's file system
- Click folders to browse
- Download files
- Delete files
- Upload files (when implemented)

**Usage Example:**
```
1. Click "Browse" button in toolbar
2. Enter path: /home/username
3. Files appear in table below
4. Click file name to open in folder
5. Use Download/Delete buttons for actions
```

#### Terminal Tab
- Execute remote commands
- View output in real-time
- Dark terminal interface

**Usage Example:**
```
1. Switch to "⌨️ Terminal" tab
2. Type command in input field
3. Press Enter to execute
4. Results appear above in terminal output
5. Continue entering more commands
```

#### Processes Tab
- View all running processes
- See CPU and memory usage
- Kill individual processes
- Search processes

**Usage Example:**
```
1. Switch to "⚙️ Processes" tab
2. Processes list auto-loads
3. Click "Refresh" to update
4. Use "Kill" button to terminate process
5. Search field filters process list
```

#### System Info Tab
- Detailed hardware specs
- Installation information
- Security status

#### Actions Tab
- Screenshot capture
- System restart/sleep
- Lock screen
- Send message
- Uninstall client

### 3. Proxy Management (Backend APIs)

**Create Proxy Tunnel:**
```bash
curl -X POST http://localhost:8080/api/proxy/create \
  -H "Content-Type: application/json" \
  -d '{
    "client_id": "abc123",
    "remote_host": "192.168.1.100",
    "remote_port": 3306,
    "local_port": 3307,
    "protocol": "tcp"
  }'
```

**List Proxy Connections:**
```bash
curl http://localhost:8080/api/proxy/list?client_id=abc123
```

**Get Proxy Statistics:**
```bash
curl http://localhost:8080/api/proxy/stats?id=proxy-id
```

**Close Proxy Connection:**
```bash
curl -X POST http://localhost:8080/api/proxy/close?id=proxy-id
```

### 4. Client Information APIs

**Get Client Details:**
```bash
curl http://localhost:8080/api/client?id=abc123
```

**List Client Files:**
```bash
curl http://localhost:8080/api/files?client_id=abc123&path=/
```

**List Client Processes:**
```bash
curl http://localhost:8080/api/processes?client_id=abc123
```

## UI Features in Detail

### Client List
- **Status Badge**: Green (online) or red (offline)
- **Click to Select**: Highlight selected client
- **Details Panel**: Shows on right side when selected
- **Real-time Updates**: Refreshes every 10 seconds

### Quick Actions
```
From Details Panel:
- 🖥️ Control  → Opens full client control page
- 📊 Stats    → Shows system statistics
- ⌨️ Terminal → Remote command execution
- 📋 Logs     → View system logs
- 🗑️ Remove   → Remove from client list
- ❌ Uninstall → Uninstall on client
```

### Responsive Design
- **Desktop**: Full 3-column layout with sidebar
- **Tablet**: Collapsible sidebar, adjusted grid
- **Mobile**: Single column, touch-friendly buttons

## Keyboard Shortcuts

| Shortcut | Action |
|----------|--------|
| `Enter` | Execute terminal command |
| `Esc` | Close details panel |
| `Ctrl+C` | Cancel current operation (in terminal) |

## Data Display Format

### File Browser
```
Name          | Size      | Modified           | Actions
-----------------------------------------------------------
📁 Documents | -         | 2024-01-10 14:30 | Download Delete
📄 config.ini| 2.5 KB    | 2024-01-09 09:15 | Download Delete
```

### Process Manager
```
Name          | CPU  | Memory | Status   | Actions
-----------------------------------------------------------
svchost.exe  | 2.5% | 15.3%  | running  | Kill
explorer.exe | 5.1% | 45.6%  | running  | Kill
```

### Statistics
```
CPU Usage     30.2%
Memory Usage  48.7% (12.1 GB / 24.0 GB)
Disk Usage    62.4% (620 GB / 1 TB)
Uptime        45d 3h
```

## Common Tasks

### View All Connected Clients
1. Go to Dashboard (`/dashboard-new`)
2. Clients list shows all connected devices
3. Green badges indicate online clients

### Access Remote Terminal
1. Click on desired client
2. Click "🖥️ Control" button (opens new window)
3. Switch to "⌨️ Terminal" tab
4. Enter command and press Enter

### Browse Remote Files
1. Click on desired client
2. Click "🖥️ Control" button
3. Switch to "📁 File Browser" tab
4. Enter path in input field (e.g., `/home/user`)
5. Click "Browse"

### Check System Processes
1. Click on desired client
2. Click "🖥️ Control" button
3. Switch to "⚙️ Processes" tab
4. Processes auto-load
5. Use Kill button to terminate if needed

### Monitor System Stats
1. Click on desired client
2. Click "📊 Stats" from details panel
3. View CPU, Memory, Disk, and Uptime info

### Create Port Forward (Proxy)
1. Use API endpoint `/api/proxy/create`
2. Specify remote host and ports
3. Local port becomes accessible
4. Data tunnels through client

## Troubleshooting

### Client Not Appearing
- Check client is actually connected
- Verify client machine is online
- Try clicking "Refresh" button
- Check browser console for errors

### Files Not Loading
- Ensure path is valid on client
- Check client has read permissions
- Try different directory path
- Verify client is still connected

### Terminal Not Responding
- Check client connection status
- Verify command syntax is correct
- Try simple command first (e.g., `ls`)
- Check for long-running operations

### Proxy Connection Fails
- Verify remote host is reachable from client
- Check remote port is open/accessible
- Ensure local port is available
- Verify protocol setting is correct

## Advanced Features (When Fully Implemented)

### Real-time Monitoring
- Live CPU/Memory graphs
- Network bandwidth monitoring
- Disk usage trends
- System event logs

### File Management
- Bulk file operations
- Directory creation/deletion
- File permission modification
- Recursive file operations

### Process Management
- Process restart
- Priority adjustment
- Memory limit enforcement
- Process monitoring alerts

### System Administration
- Registry editing (Windows)
- Service management
- User account control
- System configuration

## Security Best Practices

1. **Always use HTTPS**: Enable TLS in production
2. **Strong Credentials**: Use complex passwords for web UI
3. **Session Management**: Log out when not in use
4. **Limit Access**: Restrict proxy creation to trusted IPs
5. **Monitor Logs**: Check audit logs for suspicious activity
6. **Update Clients**: Keep client software current

## Performance Tips

1. **Pagination**: Use pagination for large file lists
2. **Filtering**: Filter processes before viewing
3. **Batching**: Group operations together
4. **Caching**: Browser caches file lists
5. **Compression**: Enable gzip for transfers

## Browser Compatibility

- ✅ Chrome/Chromium 90+
- ✅ Firefox 88+
- ✅ Safari 14+
- ✅ Edge 90+
- ⚠️ IE 11 (not tested)

## Keyboard Navigation

All UI elements are accessible via keyboard:
- `Tab`: Navigate between elements
- `Enter`: Activate buttons
- `Space`: Toggle checkboxes
- `Esc`: Close modals/panels
- `Arrow Keys`: Navigate lists

## Getting Help

For issues or questions:
1. Check browser console (F12) for errors
2. Review API response status codes
3. Check server logs for detailed errors
4. Verify client is connected and responsive
5. Try different client if available

## Next Steps

After familiarizing with basic features:

1. **Create Proxy Tunnels**: Forward specific ports
2. **Monitor Multiple Clients**: Track system metrics
3. **Automate Tasks**: Use terminal for scripting
4. **Set Alerts**: Monitor for issues (when implemented)
5. **Generate Reports**: Export client statistics (when implemented)

---

For more detailed information, see `LANPROXY_INTEGRATION.md`
