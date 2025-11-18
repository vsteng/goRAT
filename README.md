# Server Manager

A comprehensive Go-based solution for managing multiple Windows and Linux servers from a central server node. The system consists of three components: Server, Client, and Client Monitor, communicating via secure WebSocket (WSS) connections.

## Features

### Server
- **Multi-client Management**: Accept and manage multiple simultaneous client connections
- **Unique Client IDs**: Enforce unique machine identifiers, automatically disconnect duplicates
- **Authentication**: Token-based authentication before executing commands
- **Client Metadata**: Store and track client OS, IP, status, and health metrics
- **RESTful API**: HTTP endpoints for client management and command execution
- **Web-based Management Interface**: Modern web UI with session-based authentication for managing clients
- **Real-time Terminal Sessions**: Interactive terminal access to connected clients via WebSocket
- **Nginx Ready**: Designed to run behind nginx reverse proxy with TLS termination
- **Flexible Deployment**: Supports both HTTP (with nginx) and direct TLS modes

### Client
- **Auto-Configuration**: Generates unique machine ID automatically (no config file needed)
- **Remote Command Execution**: Execute system commands with proper encoding handling (GBK on Windows)
- **Real-time Terminal**: Interactive shell sessions (bash/sh on Linux/macOS, cmd on Windows)
- **File Browser**: Browse, download, and upload files with metadata
- **Screenshot Capture**: Capture screen with configurable quality
- **Keylogger**: Monitor keyboard input (SSH, RDP, or general monitoring)
- **Self-Update**: Download and install updates with checksum verification
- **Auto-Start**: Configure automatic startup on boot (Windows registry / Linux systemd)
- **Secure Communication**: Enforces TLS certificate verification (HTTPS only)
- **Cross-Platform**: Full support for Windows and Linux

### Client Monitor
- **Health Monitoring**: Continuously check if client is running
- **Auto-Restart**: Automatically restart client if it crashes
- **Installation**: Install client binary if not present
- **Configurable**: Adjustable check intervals and restart policies

## Project Structure

```
.
├── cmd/
│   ├── server/          # Server main entry point
│   └── client/          # Client main entry point
├── server/              # Server implementation
│   ├── main.go
│   ├── client_manager.go
│   ├── handlers.go
│   ├── utils.go
│   └── errors.go
├── client/              # Client implementation
│   ├── main.go
│   ├── command.go       # Command execution
│   ├── file_browser.go  # File operations
│   ├── screenshot.go    # Screenshot capture
│   ├── keylogger.go     # Keylogging functionality
│   ├── updater.go       # Self-update mechanism
│   ├── autostart_windows.go  # Windows auto-start
│   ├── autostart_unix.go     # Linux auto-start
│   └── errors.go
├── client_monitor/      # Monitor implementation
│   ├── main.go
│   ├── monitor.go
│   ├── monitor_unix.go
│   └── monitor_windows.go
├── common/              # Shared types and protocols
│   ├── protocol.go      # Message types and payloads
│   └── utils.go         # Utility functions
├── scripts/
│   └── generate-certs.sh  # TLS certificate generation
├── config.example.json  # Example configuration
├── go.mod
└── README.md
```

## Installation

### Prerequisites

- Go 1.21 or higher
- Nginx (for production deployment with TLS termination)
- OpenSSL (for generating TLS certificates for nginx)

### Build

```bash
# Build all components
make build

# Or build individually
go build -o bin/server cmd/server/main.go
go build -o bin/client cmd/client/main.go
go build -o bin/client_monitor client_monitor/*.go
```

### Production Deployment with Nginx

The recommended production setup uses nginx as a reverse proxy with TLS termination:

1. **Generate TLS Certificates** (for nginx):
```bash
# Option 1: Self-signed (development/testing)
chmod +x scripts/generate-certs.sh
./scripts/generate-certs.sh

# Option 2: Let's Encrypt (production)
sudo certbot --nginx -d your-domain.com
```

2. **Configure Nginx**:
```bash
# Copy the example nginx config
sudo cp configs/nginx.conf /etc/nginx/sites-available/servermanager
sudo ln -s /etc/nginx/sites-available/servermanager /etc/nginx/sites-enabled/

# Edit the configuration
sudo nano /etc/nginx/sites-available/servermanager
# - Update server_name to your domain
# - Update ssl_certificate paths
# - Verify backend port matches server (default :8080)

# Test and reload
sudo nginx -t
sudo systemctl reload nginx
```

3. **Start the Server** (HTTP mode for nginx):
```bash
./bin/server -addr :8080 -token your-secret-token
```

### Development/Testing with Direct TLS

For development or testing without nginx:

```bash
# Generate certificates for direct TLS
./scripts/generate-certs.sh

# Start server with TLS enabled
./bin/server -addr :8443 -tls -cert certs/server.crt -key certs/server.key -token your-secret-token
```

## Configuration

### Server Configuration

The server can run in two modes:

**Production (HTTP mode with nginx):**
```bash
# Start server without TLS (nginx handles it)
./bin/server -addr :8080 -token your-secret-token -web-user admin -web-pass your-password
```

**2. TLS Mode (Development/Testing - direct access):**
```bash
# Start server with TLS
./bin/server -addr :8443 -tls -cert certs/server.crt -key certs/server.key -token your-secret-token -web-user admin -web-pass your-password
```

**Command-line Flags:**
- `-addr` - Listen address (default: `:8080`)
- `-tls` - Enable TLS (default: `false`)
- `-cert` - TLS certificate file (required if `-tls` enabled)
- `-key` - TLS private key file (required if `-tls` enabled)
- `-token` - Authentication token (required)
- `-web-user` - Web UI username (default: `admin`)
- `-web-pass` - Web UI password (default: `admin`)

**Accessing the Web Interface:**

After starting the server, open your web browser and navigate to:
- Production (with nginx): `https://your-domain.com/login`
- Development (direct): `http://localhost:8080/login`

Log in with the credentials specified in `-web-user` and `-web-pass` flags.

### Client Configuration

**The client no longer requires a configuration file.** It automatically generates a unique machine ID based on system hardware.

**Basic Usage:**
```bash
# Production (with nginx using HTTPS)
./bin/client -server wss://your-domain.com/ws -token your-secret-token

# Development (direct TLS connection)
./bin/client -server wss://localhost:8443/ws -token your-secret-token
```

**Command-line Flags:**
- `-server` - WebSocket server URL (required)
- `-token` - Authentication token (required)
- `-autostart` - Configure auto-start on boot (optional)

**Machine ID Generation:**
The client automatically generates a unique identifier from:
- Hostname
- Host UUID (from SMBIOS/DMI)
- OS-specific identifiers:
  - Windows: Machine GUID from registry
  - Linux: `/etc/machine-id` or `/var/lib/dbus/machine-id`
  - macOS: IOPlatformUUID

The ID is cached in:
- Windows: `%APPDATA%\ServerManager\machine-id`
- Linux: `~/.config/servermanager/machine-id`
- macOS: `~/Library/Application Support/servermanager/machine-id`

**Security:**
- The client **always** verifies TLS certificates (no skip option)
- Use `wss://` (WebSocket Secure) protocol
- Ensure server certificates are from a trusted CA in production

### Monitor Configuration

Create `monitor-config.json`:

```json
{
  "client_path": "/path/to/client",
  "client_args": ["-config", "/path/to/client-config.json"],
  "check_interval": "10s",
  "restart_delay": "5s",
  "max_restarts": -1
}
```

## Usage

### Start the Server

**Production (HTTP mode with nginx):**
```bash
# Server listens on localhost:8080, nginx handles TLS
./bin/server -addr :8080 -token your-secret-token -web-user admin -web-pass your-password
```

**Development (Direct TLS):**
```bash
# Server listens with TLS on :8443
./bin/server -addr :8443 -tls -cert certs/server.crt -key certs/server.key -token your-secret-token -web-user admin -web-pass your-password
```

The server will display:
```
Web UI will be available at http://localhost:8080/login
Web UI credentials - Username: admin, Password: your-password
```

### Using the Web Interface

1. **Access the Login Page**
   - Open your browser and navigate to the server URL (e.g., `http://localhost:8080/login`)
   - Enter the username and password configured when starting the server

2. **Dashboard**
   - View all connected clients with their status, OS, IP address, and last seen time
   - See real-time statistics (total clients, online, offline)
   - Click "Refresh" to manually update the client list (auto-refreshes every 10 seconds)

3. **Execute Commands**
   - Click the "Command" button next to any client
   - Enter a command to execute on that client
   - Results are sent asynchronously to the client

4. **Open Terminal Session**
   - Click the "Terminal" button next to any client
   - A new window opens with an interactive terminal
   - Type commands and see real-time output
   - Use Ctrl+C (or click "Interrupt") to stop running commands
   - Command history available with Up/Down arrow keys
   - Terminal sessions are fully interactive with the client's shell

5. **Security**
   - Sessions expire after 24 hours of inactivity
   - Always use HTTPS in production (via nginx)
   - Change default credentials immediately

### Start the Client

**Production (HTTPS through nginx):**
```bash
./bin/client -server wss://your-domain.com/ws -token your-secret-token

# Enable auto-start
./bin/client -server wss://your-domain.com/ws -token your-secret-token -autostart
```

**Development (Direct TLS):**
```bash
./bin/client -server wss://localhost:8443/ws -token your-secret-token
```

**Note:** The client automatically generates and caches a unique machine ID. No configuration file is needed.

### Start the Client Monitor

```bash
# Basic usage
./bin/client_monitor -client /path/to/client

# With client arguments
./bin/client_monitor -client /path/to/client -- -config /path/to/client-config.json

# Install and monitor
./bin/client_monitor -client /path/to/client -install /path/to/client-binary

# With config file
./bin/client_monitor -config monitor-config.json
```

## API Endpoints

### Web UI Endpoints

#### GET /login
Login page for web interface.

#### POST /api/login
Authenticate user for web interface.

**Request:**
```json
{
  "username": "admin",
  "password": "your-password"
}
```

**Response:**
```json
{
  "status": "success"
}
```

#### POST /api/logout
Logout from web interface.

#### GET /dashboard
Dashboard showing connected clients (requires authentication).

#### GET /terminal?client=CLIENT_ID
Interactive terminal interface for a specific client (requires authentication).

#### WebSocket /api/terminal?client=CLIENT_ID
WebSocket endpoint for real-time terminal sessions (requires authentication).

### API Endpoints (Client Management)

### GET /api/clients

Get list of connected clients.

**Response:**
```json
[
  {
    "id": "client-001",
    "os": "linux",
    "arch": "amd64",
    "hostname": "server-01",
    "ip": "192.168.1.100",
    "status": "online",
    "connected_at": "2024-01-01T00:00:00Z",
    "last_seen": "2024-01-01T00:01:00Z"
  }
]
```

### POST /api/command

Execute a command on a specific client.

**Request:**
```json
{
  "client_id": "client-001",
  "command": {
    "command": "ls",
    "args": ["-la"],
    "work_dir": "/tmp",
    "timeout": 30
  }
}
```

**Response:**
```json
{
  "status": "sent"
}
```

## Message Types

The system uses a WebSocket-based protocol with the following message types:

### Authentication
- `auth` - Client authentication
- `auth_response` - Authentication response

### Commands
- `execute_command` - Execute a system command
- `command_result` - Command execution result

### File Operations
- `browse_files` - Browse directory
- `file_list` - Directory listing result
- `download_file` - Request file download
- `upload_file` - Upload file
- `file_data` - File content transfer

### Screenshots
- `take_screenshot` - Request screenshot
- `screenshot_data` - Screenshot data

### Keylogger
- `start_keylogger` - Start keylogger
- `stop_keylogger` - Stop keylogger
- `keylogger_data` - Logged keystrokes

### Terminal Sessions
- `start_terminal` - Start interactive terminal session
- `stop_terminal` - Stop terminal session
- `terminal_input` - Input to terminal
- `terminal_output` - Output from terminal
- `terminal_resize` - Resize terminal window

### Updates
- `update` - Update client
- `update_status` - Update status report

### Health
- `heartbeat` - Client health status
- `ping`/`pong` - Connection keepalive

## Platform-Specific Features

### Windows
- Command output encoding conversion (GBK → UTF-8)
- Registry-based auto-start
- Batch file startup alternative
- Process management via tasklist

### Linux
- Systemd service auto-start
- Process management via pgrep
- Init script support (rc.local)

## Security Considerations

1. **TLS Certificates**: 
   - Use certificates from a trusted CA (Let's Encrypt) in production
   - Client always enforces TLS certificate verification
   - No option to skip certificate validation

2. **Authentication Tokens**: 
   - Use strong, randomly generated tokens
   - Keep tokens secret and never commit to version control
   - Rotate tokens regularly

3. **Network Security**: 
   - Run server behind nginx reverse proxy in production
   - Use firewall to restrict access to server port (8080)
   - Whitelist client IPs if possible
   - Use HTTPS/WSS only, never HTTP/WS

4. **Client Identity**:
   - Server enforces unique machine IDs
   - Duplicate IDs trigger automatic disconnection of old session
   - Machine IDs are hardware-based and persistent

5. **File Permissions**: 
   - Ensure client runs with minimal required permissions
   - Restrict file browser access to necessary directories

6. **Keylogger**: 
   - Use responsibly and ensure compliance with local laws
   - Obtain proper authorization before deployment
   - Store logs securely

7. **Update Verification**: 
   - Always verify checksums before applying updates
   - Use secure channel for update distribution

8. **Nginx Configuration**:
   - Keep nginx updated with security patches
   - Use strong TLS ciphers (TLS 1.2+)
   - Enable HSTS, OCSP stapling
   - Monitor access logs for suspicious activity

## Development

### Dependencies

- `github.com/gorilla/websocket` - WebSocket support
- `github.com/kbinani/screenshot` - Cross-platform screenshots
- `github.com/shirou/gopsutil/v3` - System statistics
- `golang.org/x/text` - Text encoding (GBK support)
- `golang.org/x/sys` - System-specific APIs

### Running Tests

```bash
go test ./...
```

### Building for Multiple Platforms

```bash
# Linux
GOOS=linux GOARCH=amd64 go build -o bin/client-linux cmd/client/main.go

# Windows
GOOS=windows GOARCH=amd64 go build -o bin/client-windows.exe cmd/client/main.go

# macOS
GOOS=darwin GOARCH=amd64 go build -o bin/client-darwin cmd/client/main.go
```

## Troubleshooting

### Client Can't Connect
- Verify server is running and accessible
- Check firewall rules (nginx: port 443, direct: port 8443)
- Verify TLS certificates are valid and trusted
- Ensure auth token matches server configuration
- Check nginx logs: `/var/log/nginx/servermanager_error.log`
- Verify nginx upstream is reachable: `curl http://localhost:8080/health`

### Duplicate Client ID Error
- Each client automatically generates a unique machine ID
- If you see "duplicate ID" errors, the machine ID cache may be corrupted
- Delete the cached ID file:
  - Windows: `%APPDATA%\ServerManager\machine-id`
  - Linux: `~/.config/servermanager/machine-id`
  - macOS: `~/Library/Application Support/servermanager/machine-id`
- Restart the client to regenerate

### TLS Certificate Verification Fails
- Ensure server is using a certificate from a trusted CA
- For nginx, check certificate paths in config
- Verify certificate is not expired: `openssl x509 -in /path/to/cert.crt -noout -dates`
- Client cannot skip certificate verification (by design)

### Nginx WebSocket Upgrade Fails
- Check nginx error log for "failed to send http2 header" or "upstream sent no valid HTTP/1.0 header"
- Verify `proxy_http_version 1.1` and upgrade headers are set
- Ensure server is listening on the correct port (default 8080)
- Test direct connection: `wscat -c ws://localhost:8080/ws`

### Command Encoding Issues (Windows)
- The client automatically handles GBK → UTF-8 conversion
- If issues persist, check system locale settings: `chcp` (should show 936 for GBK)

### Auto-Start Not Working
**Windows:**
- Check registry key: `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`
- Verify executable path is correct and accessible
- Check Windows Event Viewer for startup errors

**Linux:**
- Check systemd service: `systemctl --user status ServerManagerClient.service`
- View logs: `journalctl --user -u ServerManagerClient.service`
- Ensure service is enabled: `systemctl --user enable ServerManagerClient.service`

### Screenshot Fails
- Ensure display is available (not headless)
- Check permissions for screen capture
- On Linux, may need X11 session
- On macOS 15+, screenshot library is incompatible (known issue)
  - Build with `-tags noscreenshot` to disable

### Server Won't Start
- Check if port is already in use: `lsof -i :8080` (Linux/macOS) or `netstat -ano | findstr :8080` (Windows)
- Verify token is provided: `-token` flag is required
- In TLS mode, ensure cert and key files exist and are readable
- Check server logs for specific error messages

## License

This project is provided as-is for educational and internal use.

## Contributing

Contributions are welcome! Please ensure:
- Code follows Go best practices
- Cross-platform compatibility is maintained
- Security implications are considered
- Tests are included for new features

## Future Enhancements

- [ ] gRPC transport option
- [ ] End-to-end encryption for sensitive data
- [ ] Database for client history and logs
- [ ] Multi-tenancy support
- [ ] Role-based access control
- [ ] File transfer resume capability
- [x] ~~Real-time terminal session~~ (Completed)
- [x] ~~Web-based management interface~~ (Completed)
- [ ] Scheduled task execution
- [ ] Client grouping and bulk operations
