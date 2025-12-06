# Quick Start - New UX Flow

## Overview

The application now has a simplified two-stage workflow:
1. **Dashboard**: View and filter all clients
2. **Proxy Management**: Manage proxies for a specific client

## Getting Started

### 1. Start the Server

```bash
cd /Users/tengbozhang/chrom
./bin/server
```

Access at: `http://localhost:8080`

### 2. Login

- Enter credentials
- See success message
- Automatically redirected to Dashboard after ~1.5 seconds

### 3. Dashboard - Client Summary

**Page**: `/dashboard-summary`

**Components**:
- **Header**: Logo and Logout button
- **Stats Cards**: Total clients, Online, Offline counts
- **Filter Buttons**: 
  - "All Clients" - Show all clients (default)
  - "Online Only" - Show connected clients only
  - "Offline Only" - Show disconnected clients only
- **Client Table**: ID, Hostname, OS, Status, Action
  
**Actions**:
- Click status filter to change view
- Click "Manage Proxies" button on any client
- Proxies page opens with that client's data

### 4. Proxy Management - Client Specific

**Page**: `/client/{clientId}/proxies`

**Components**:
- **Top Menu** (with dashed separator below):
  - Client name and ID
  - Hostname and OS
  - Online/Offline status (🟢 Online / 🔴 Offline)
- **Back Button**: Return to dashboard
- **Proxy List**:
  - Local Port (server side)
  - Remote Address (client side: host:port)
  - Protocol (TCP/HTTP/HTTPS)
  - Status (Active/Inactive)
  - Delete button
- **Add New Proxy Section**:
  - Client Address field (e.g., 127.0.0.1:22)
  - Server Port field (e.g., 10033)
  - Protocol dropdown
  - Add Proxy button

**Add Proxy Example**:
- Client Address: `192.168.1.100:22` (SSH on client machine)
- Server Port: `10033` (listen port on server)
- Protocol: `TCP`
- Click "Add Proxy" → Adds connection
- Shows success message → Reloads list after 1 second

**Delete Proxy**:
- Click "Delete" button on any proxy
- Confirm action
- Proxy removed from list

## API Endpoints

All endpoints require authentication (session cookie).

### Get All Clients
```
GET /api/clients
Response: [{ID, Hostname, OS, IsConnected, ...}, ...]
```

### Get Single Client
```
GET /api/client?id={clientId}
Response: {ID, Hostname, OS, IsConnected, ...}
```

### List Proxies for Client
```
GET /api/proxy/list?clientId={clientId}
Response: [{ID, ClientID, LocalPort, RemoteHost, RemotePort, Protocol, Status}, ...]
```

### Create Proxy
```
POST /api/proxy/create
Body: {
  "clientId": "{clientId}",
  "remoteHost": "127.0.0.1",
  "remotePort": 22,
  "localPort": 10033,
  "protocol": "TCP"
}
Response: {ID, status}
```

### Delete Proxy
```
POST /api/proxy/close
Body: {"proxyId": "{proxyId}"}
Response: {status}
```

## Features & Behaviors

### Auto-Refresh
- Dashboard: Reloads client list every 5 seconds
- Proxy page: Reloads proxy list every 5 seconds

### Status Indicators
- 🟢 **Online**: Client is currently connected
- 🔴 **Offline**: Client is not connected

### Filtering
- Clients can be filtered by connection status
- Filter persists while on dashboard page
- Reset to "All" when navigating away

### Error Handling
- Invalid credentials: Shows error on login
- Network errors: Alerts user with friendly message
- Failed actions: Shows error alert, allows retry

### Alerts
- **Success**: Green background, auto-hides after 3 seconds
- **Error**: Red background, remains visible for user action

## Common Tasks

### View All Clients
1. Go to Dashboard
2. Click "All Clients" filter (default)
3. See complete list

### View Only Active Clients
1. Go to Dashboard
2. Click "Online Only" filter
3. See only connected clients

### Add SSH Tunnel (Example)
1. Go to Dashboard
2. Click "Manage Proxies" on target client
3. In Client Address: `127.0.0.1:22`
4. In Server Port: `10033`
5. Leave Protocol as `TCP`
6. Click "Add Proxy"
7. See success message
8. New proxy appears in list

### Delete a Proxy
1. On Proxy page, find proxy in list
2. Click "Delete" button
3. Confirm deletion
4. Proxy removed

### Logout
- Click "Logout" button (top right)
- Redirected to login page
- Session cleared

## Keyboard Shortcuts

None currently implemented. All actions through UI buttons.

## Troubleshooting

### "Connection error. Please try again." on login
- Check server is running
- Check network connectivity
- Verify correct address/port

### Clients not loading
- Check authentication (session may have expired)
- Click logout and login again
- Verify server is running

### Proxy creation failed
- Check all fields are filled
- Verify port format (numeric only)
- Check client address format (host:port)
- Port must be available on server

### Page keeps redirecting
- Session may have expired (>24 hours or invalid)
- Login again to refresh session

## Technical Stack

- **Backend**: Go (Golang)
- **Frontend**: HTML5, CSS3, Vanilla JavaScript
- **Communication**: HTTP/REST, WebSocket
- **Authentication**: Session cookies (24-hour TTL)

## Customization

### Change Dashboard Stats Colors
Edit `web/templates/dashboard-summary.html`, search for `.stat-card .number` CSS:
```css
color: #667eea; /* Change this color */
```

### Change Filter Button Styling
Search for `.filter-btn` CSS class in same file.

### Change Auto-Refresh Rate
Look for `setInterval(loadClients, 5000)` (5000ms = 5 seconds):
```javascript
setInterval(loadClients, 3000); // Change to 3 seconds
```

## Security Notes

- All routes require valid session cookie
- Expired sessions redirect to login
- Passwords sent over HTTPS in production
- Session ID validated on each request
- CSRF protection via session validation

## Performance

- Dashboard loads ~50 clients instantly
- Filter operation immediate (client-side)
- Proxy operations < 100ms typical
- Auto-refresh handles lag gracefully
