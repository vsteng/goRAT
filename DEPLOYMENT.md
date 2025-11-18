# Production Deployment Guide

This guide covers deploying the Server Manager system in a production environment with nginx as a reverse proxy.

## Architecture Overview

```
┌─────────┐     HTTPS/WSS      ┌───────────┐     HTTP/WS      ┌────────────┐
│ Clients ├──────────────────>  │   Nginx   ├──────────────>   │   Server   │
└─────────┘   (Port 443)        │  (Proxy)  │  (Port 8080)     │   (HTTP)   │
                                 └───────────┘                  └────────────┘
                                 • TLS Termination              • No TLS
                                 • Load Balancing               • Auth Token
                                 • Rate Limiting                • WebSocket
                                 • HTTPS/WSS                    • API
```

**Key Design Decisions:**
- Nginx handles TLS encryption (certificates, ciphers, HTTPS)
- Server runs in HTTP mode (simpler, no cert management)
- Clients connect via HTTPS/WSS with certificate verification
- Server enforces unique machine IDs per client

## Prerequisites

- Ubuntu 20.04+ or similar Linux distribution
- Nginx 1.18+
- Go 1.21+ (for building from source)
- Domain name with DNS configured
- Root or sudo access

## Step 1: Build the Server

```bash
# Clone repository
git clone <repository-url>
cd servermanager

# Build server binary
make build

# Or manually
go build -o bin/server cmd/server/main.go

# Verify binary
./bin/server -h
```

## Step 2: Create System User

```bash
# Create dedicated user (no login shell)
sudo useradd -r -s /bin/false -d /opt/servermanager servermanager

# Create directories
sudo mkdir -p /opt/servermanager/{bin,logs}

# Copy binary
sudo cp bin/server /opt/servermanager/bin/

# Set ownership
sudo chown -R servermanager:servermanager /opt/servermanager

# Set permissions
sudo chmod 750 /opt/servermanager
sudo chmod 550 /opt/servermanager/bin/server
```

## Step 3: Configure Systemd Service

Create `/etc/systemd/system/servermanager.service`:

```ini
[Unit]
Description=Server Manager Service
After=network.target

[Service]
Type=simple
User=servermanager
Group=servermanager
WorkingDirectory=/opt/servermanager

# Generate token with: openssl rand -hex 32
Environment="AUTH_TOKEN=your-secure-token-here"
Environment="WEB_USERNAME=admin"
Environment="WEB_PASSWORD=your-secure-web-password"

ExecStart=/opt/servermanager/bin/server \
    -addr 127.0.0.1:8080 \
    -token ${AUTH_TOKEN} \
    -web-user ${WEB_USERNAME} \
    -web-pass ${WEB_PASSWORD}

# Restart policy
Restart=on-failure
RestartSec=5s

# Security hardening
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/opt/servermanager/logs

# Logging
StandardOutput=append:/opt/servermanager/logs/server.log
StandardError=append:/opt/servermanager/logs/server-error.log

[Install]
WantedBy=multi-user.target
```

**Start and enable service:**

```bash
# Reload systemd
sudo systemctl daemon-reload

# Start service
sudo systemctl start servermanager

# Enable on boot
sudo systemctl enable servermanager

# Check status
sudo systemctl status servermanager

# View logs
sudo journalctl -u servermanager -f
```

## Step 4: Install and Configure Nginx

```bash
# Install nginx
sudo apt update
sudo apt install nginx

# Stop nginx temporarily
sudo systemctl stop nginx
```

## Step 5: Obtain TLS Certificate

### Option A: Let's Encrypt (Recommended for Production)

```bash
# Install certbot
sudo apt install certbot python3-certbot-nginx

# Obtain certificate (interactive)
sudo certbot --nginx -d your-domain.com

# Certificate files will be at:
# /etc/letsencrypt/live/your-domain.com/fullchain.pem
# /etc/letsencrypt/live/your-domain.com/privkey.pem

# Auto-renewal is configured automatically
# Test renewal: sudo certbot renew --dry-run
```

### Option B: Self-Signed (Development/Testing Only)

```bash
# Generate self-signed certificate
sudo openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
    -keyout /etc/ssl/private/servermanager.key \
    -out /etc/ssl/certs/servermanager.crt \
    -subj "/CN=your-domain.com"

# Set permissions
sudo chmod 600 /etc/ssl/private/servermanager.key
sudo chmod 644 /etc/ssl/certs/servermanager.crt
```

## Step 6: Configure Nginx

Create `/etc/nginx/sites-available/servermanager`:

```nginx
# Map for WebSocket upgrade header
map $http_upgrade $connection_upgrade {
    default upgrade;
    '' close;
}

upstream servermanager_backend {
    server 127.0.0.1:8080;
    keepalive 64;
}

# Redirect HTTP to HTTPS
server {
    listen 80;
    listen [::]:80;
    server_name your-domain.com;

    location /.well-known/acme-challenge/ {
        root /var/www/html;
    }

    location / {
        return 301 https://$server_name$request_uri;
    }
}

# HTTPS server
server {
    listen 443 ssl http2;
    listen [::]:443 ssl http2;
    server_name your-domain.com;

    # TLS Configuration (Let's Encrypt)
    ssl_certificate /etc/letsencrypt/live/your-domain.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/your-domain.com/privkey.pem;

    # TLS Configuration (Self-Signed - uncomment if using)
    # ssl_certificate /etc/ssl/certs/servermanager.crt;
    # ssl_certificate_key /etc/ssl/private/servermanager.key;

    # Modern TLS configuration
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers 'ECDHE-ECDSA-AES128-GCM-SHA256:ECDHE-RSA-AES128-GCM-SHA256:ECDHE-ECDSA-AES256-GCM-SHA384:ECDHE-RSA-AES256-GCM-SHA384';
    ssl_prefer_server_ciphers off;
    
    ssl_session_cache shared:SSL:50m;
    ssl_session_timeout 1d;
    ssl_session_tickets off;

    # OCSP stapling
    ssl_stapling on;
    ssl_stapling_verify on;
    ssl_trusted_certificate /etc/letsencrypt/live/your-domain.com/chain.pem;
    resolver 8.8.8.8 8.8.4.4 valid=300s;
    resolver_timeout 5s;

    # Security headers
    add_header Strict-Transport-Security "max-age=63072000; includeSubDomains; preload" always;
    add_header X-Frame-Options DENY always;
    add_header X-Content-Type-Options nosniff always;
    add_header X-XSS-Protection "1; mode=block" always;

    # Logging
    access_log /var/log/nginx/servermanager_access.log;
    error_log /var/log/nginx/servermanager_error.log;

    # Rate limiting (optional)
    limit_req_zone $binary_remote_addr zone=api_limit:10m rate=10r/s;
    limit_req_zone $binary_remote_addr zone=ws_limit:10m rate=5r/s;

    # Root location - web UI
    location / {
        proxy_pass http://servermanager_backend;
        
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        
        # Cookie support for sessions
        proxy_set_header Cookie $http_cookie;
        
        # Timeouts
        proxy_read_timeout 60s;
        proxy_send_timeout 60s;
        proxy_connect_timeout 10s;
    }

    # Client WebSocket endpoint
    location /ws {
        limit_req zone=ws_limit burst=10 nodelay;

        proxy_pass http://servermanager_backend;
        
        # WebSocket headers
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        
        # Standard headers
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        
        # Timeouts
        proxy_read_timeout 86400s;
        proxy_send_timeout 86400s;
        proxy_connect_timeout 10s;
        
        proxy_buffering off;
    }

    # API endpoints (including terminal WebSocket)
    location /api/ {
        limit_req zone=api_limit burst=20 nodelay;

        proxy_pass http://servermanager_backend;
        
        # WebSocket support for terminal
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection $connection_upgrade;
        
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        
        # Cookie support for authentication
        proxy_set_header Cookie $http_cookie;
        
        # Timeouts - longer for terminal WebSocket
        proxy_read_timeout 86400s;
        proxy_send_timeout 86400s;
        proxy_connect_timeout 10s;
        
        proxy_buffering off;
    }

    # Web UI pages
    location ~ ^/(login|dashboard|terminal|files) {
        proxy_pass http://servermanager_backend;
        
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        
        # Cookie support for sessions
        proxy_set_header Cookie $http_cookie;
        
        proxy_read_timeout 60s;
        proxy_send_timeout 60s;
        proxy_connect_timeout 10s;
    }

    # Health check
    location /health {
        access_log off;
        return 200 "healthy\n";
        add_header Content-Type text/plain;
    }
}
```

**Enable site and restart nginx:**

```bash
# Create symlink
sudo ln -s /etc/nginx/sites-available/servermanager /etc/nginx/sites-enabled/

# Remove default site (optional)
sudo rm -f /etc/nginx/sites-enabled/default

# Test configuration
sudo nginx -t

# Start nginx
sudo systemctl start nginx
sudo systemctl enable nginx

# Check status
sudo systemctl status nginx
```

## Step 7: Firewall Configuration

```bash
# UFW (Ubuntu)
sudo ufw allow 80/tcp    # HTTP (for Let's Encrypt)
sudo ufw allow 443/tcp   # HTTPS
sudo ufw enable

# Or iptables
sudo iptables -A INPUT -p tcp --dport 80 -j ACCEPT
sudo iptables -A INPUT -p tcp --dport 443 -j ACCEPT
sudo iptables-save | sudo tee /etc/iptables/rules.v4
```

## Step 8: Deploy Clients

### Build Client Binary

```bash
# Linux client
GOOS=linux GOARCH=amd64 go build -o bin/client-linux cmd/client/main.go

# Windows client
GOOS=windows GOARCH=amd64 go build -o bin/client-windows.exe cmd/client/main.go
```

### Create Client Deployment Script

**Linux (`deploy-client-linux.sh`):**

```bash
#!/bin/bash
set -e

SERVER_URL="wss://your-domain.com/ws"
AUTH_TOKEN="your-secure-token-here"

# Download client binary
curl -o /tmp/client https://your-server.com/downloads/client-linux
chmod +x /tmp/client

# Install
sudo mv /tmp/client /usr/local/bin/servermanager-client

# Run with auto-start
/usr/local/bin/servermanager-client \
    -server "$SERVER_URL" \
    -token "$AUTH_TOKEN" \
    -autostart

echo "Client installed and started"
```

**Windows (`deploy-client-windows.ps1`):**

```powershell
$ServerUrl = "wss://your-domain.com/ws"
$AuthToken = "your-secure-token-here"

# Download client
$clientPath = "$env:ProgramFiles\ServerManager\client.exe"
New-Item -ItemType Directory -Force -Path (Split-Path $clientPath)
Invoke-WebRequest -Uri "https://your-server.com/downloads/client-windows.exe" -OutFile $clientPath

# Run with auto-start
& $clientPath -server $ServerUrl -token $AuthToken -autostart

Write-Host "Client installed and started"
```

## Step 9: Monitoring and Maintenance

### Monitor Server Logs

```bash
# Server application logs
sudo tail -f /opt/servermanager/logs/server.log

# Systemd logs
sudo journalctl -u servermanager -f

# Nginx logs
sudo tail -f /var/log/nginx/servermanager_access.log
sudo tail -f /var/log/nginx/servermanager_error.log
```

### Monitor Nginx Performance

```bash
# Connection statistics
sudo watch -n 1 'ss -s'

# Active connections
sudo netstat -an | grep :443 | wc -l

# Nginx status (if compiled with --with-http_stub_status_module)
curl http://localhost/nginx_status
```

### Log Rotation

Create `/etc/logrotate.d/servermanager`:

```
/opt/servermanager/logs/*.log {
    daily
    missingok
    rotate 14
    compress
    delaycompress
    notifempty
    create 0640 servermanager servermanager
    sharedscripts
    postrotate
        systemctl reload servermanager > /dev/null 2>&1 || true
    endscript
}
```

### Backup Strategy

```bash
# Create backup script
cat > /opt/servermanager/backup.sh << 'EOF'
#!/bin/bash
BACKUP_DIR="/var/backups/servermanager"
DATE=$(date +%Y%m%d_%H%M%S)

mkdir -p "$BACKUP_DIR"

# Backup logs
tar -czf "$BACKUP_DIR/logs-$DATE.tar.gz" /opt/servermanager/logs/

# Backup nginx config
tar -czf "$BACKUP_DIR/nginx-$DATE.tar.gz" /etc/nginx/sites-available/servermanager

# Keep only last 7 days
find "$BACKUP_DIR" -name "*.tar.gz" -mtime +7 -delete

echo "Backup completed: $DATE"
EOF

chmod +x /opt/servermanager/backup.sh

# Add to cron (daily at 2 AM)
echo "0 2 * * * /opt/servermanager/backup.sh" | sudo crontab -
```

## Step 10: Security Hardening

### 1. Secure Authentication Token

```bash
# Generate strong token
openssl rand -hex 32

# Store in environment file (not in systemd service directly)
sudo mkdir -p /opt/servermanager/config
sudo chmod 700 /opt/servermanager/config
echo "AUTH_TOKEN=$(openssl rand -hex 32)" | sudo tee /opt/servermanager/config/env
sudo chmod 600 /opt/servermanager/config/env
sudo chown servermanager:servermanager /opt/servermanager/config/env

# Update systemd service to use environment file
sudo systemctl edit servermanager
# Add: EnvironmentFile=/opt/servermanager/config/env
```

### 2. IP Whitelisting (Optional)

In nginx config, add to `/ws` location:

```nginx
location /ws {
    # Allow only specific IPs
    allow 203.0.113.0/24;
    allow 198.51.100.5;
    deny all;
    
    # ... rest of config
}
```

### 3. Fail2Ban Protection

Create `/etc/fail2ban/filter.d/servermanager.conf`:

```ini
[Definition]
failregex = ^<HOST> .* "GET /ws HTTP/.*" 401
            ^<HOST> .* "POST /api/.* HTTP/.*" 401
ignoreregex =
```

Create `/etc/fail2ban/jail.d/servermanager.conf`:

```ini
[servermanager]
enabled = true
port = 443
filter = servermanager
logpath = /var/log/nginx/servermanager_access.log
maxretry = 5
bantime = 3600
findtime = 600
```

Restart fail2ban:

```bash
sudo systemctl restart fail2ban
sudo fail2ban-client status servermanager
```

### 4. Regular Updates

```bash
# Create update script
cat > /opt/servermanager/update.sh << 'EOF'
#!/bin/bash
set -e

echo "Stopping service..."
sudo systemctl stop servermanager

echo "Backing up current binary..."
sudo cp /opt/servermanager/bin/server /opt/servermanager/bin/server.bak

echo "Downloading new version..."
# Replace with your actual download URL
curl -o /tmp/server https://your-server.com/downloads/server-latest

echo "Installing new binary..."
sudo mv /tmp/server /opt/servermanager/bin/server
sudo chown servermanager:servermanager /opt/servermanager/bin/server
sudo chmod 550 /opt/servermanager/bin/server

echo "Starting service..."
sudo systemctl start servermanager

echo "Update completed!"
sudo systemctl status servermanager
EOF

chmod +x /opt/servermanager/update.sh
```

## Troubleshooting

### Check Service Status

```bash
# Is service running?
sudo systemctl status servermanager

# View recent logs
sudo journalctl -u servermanager -n 100 --no-pager

# Check if listening on port
sudo ss -tlnp | grep 8080
```

### Test Backend Directly

```bash
# Test HTTP endpoint
curl http://localhost:8080/health

# Test WebSocket (requires wscat: npm install -g wscat)
wscat -c ws://localhost:8080/ws
```

### Test Nginx Proxy

```bash
# Test HTTPS
curl -I https://your-domain.com/health

# Test WebSocket through nginx
wscat -c wss://your-domain.com/ws
```

### Debug Nginx Issues

```bash
# Check nginx config
sudo nginx -t

# View error log
sudo tail -f /var/log/nginx/servermanager_error.log

# Enable debug logging (temporarily)
# Add to nginx.conf: error_log /var/log/nginx/error.log debug;
sudo systemctl reload nginx
```

### Performance Tuning

```bash
# Increase nginx worker connections
# Edit /etc/nginx/nginx.conf:
events {
    worker_connections 4096;
}

# Increase file descriptors
echo "fs.file-max = 65536" | sudo tee -a /etc/sysctl.conf
sudo sysctl -p

# For servermanager user
sudo -u servermanager bash -c 'ulimit -n'
# If low, add to /etc/security/limits.conf:
# servermanager soft nofile 65536
# servermanager hard nofile 65536
```

## Conclusion

Your Server Manager system is now deployed with:
- ✅ Nginx reverse proxy with TLS termination
- ✅ Systemd service management
- ✅ Automatic certificate renewal (Let's Encrypt)
- ✅ Security hardening (rate limiting, fail2ban)
- ✅ Monitoring and logging
- ✅ Backup strategy
- ✅ Web-based management interface with authentication
- ✅ Real-time terminal sessions
- ✅ File manager (basic implementation)
- ✅ Dual IP tracking (private and public)

### Accessing the Web Interface

1. Open your browser and navigate to: `https://your-domain.com/login`
2. Log in with your configured credentials
3. Features available:
   - **Dashboard**: View all connected clients with real-time status
   - **Terminal**: Interactive shell access to any client
   - **Command Execution**: Send commands and view output
   - **File Manager**: Browse client file systems (basic support)
   - **Dual IP Display**: View both private (local) and public IP addresses

### Client Information Displayed

The web dashboard now shows:
- Client ID and hostname
- Operating system and architecture
- **Private IP**: Client's local network IP address
- **Public IP**: Client's external/internet-facing IP address
- Connection status and last seen time
- Quick action buttons for Terminal, Command, and Files

Clients can now connect securely via `wss://your-domain.com/ws` with automatic machine ID generation and TLS verification.
