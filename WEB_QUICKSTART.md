# Quick Start Guide - Web Interface & Terminal

This guide will help you quickly set up and use the new web-based management interface and real-time terminal features.

## Prerequisites

- Server Manager already built (`make build`)
- Server and client binaries available in `bin/` directory

## Step 1: Start the Server

```bash
# Start the server with web UI credentials
./bin/server -addr :8080 -token your-secret-token -web-user admin -web-pass mypassword
```

You should see output like:
```
Web UI will be available at http://localhost:8080/login
Web UI credentials - Username: admin, Password: mypassword
Server starting on :8080
```

## Step 2: Start a Client

In a new terminal window:

```bash
# Start a client
./bin/client -server ws://localhost:8080/ws -token your-secret-token
```

The client will connect and register with the server.

## Step 3: Access the Web Interface

1. Open your web browser and navigate to: `http://localhost:8080/login`

2. Log in with the credentials you set:
   - Username: `admin`
   - Password: `mypassword`

3. You'll see the dashboard with your connected clients

## Step 4: Use the Terminal

1. In the dashboard, find your connected client
2. Click the green "Terminal" button
3. A new window opens with an interactive terminal
4. Try some commands:
   ```bash
   # Linux/Mac
   ls -la
   pwd
   whoami
   
   # Windows (cmd)
   dir
   cd
   echo Hello
   ```

5. The terminal supports:
   - Real-time command output
   - Command history (Up/Down arrows)
   - Interrupt (Ctrl+C or click "Interrupt" button)
   - Clear screen

## Features Overview

### Dashboard
- **View all clients**: See status, hostname, OS, IP
- **Auto-refresh**: Updates every 10 seconds
- **Quick command**: Send one-off commands
- **Terminal access**: Full interactive shell

### Terminal
- **Interactive shell**: Full bash/sh/cmd access
- **Real-time output**: See output as it happens
- **Command history**: Navigate with arrow keys
- **Session isolation**: Each terminal is independent

### Security
- **Session-based auth**: Secure login required
- **24-hour sessions**: Auto-expire after inactivity
- **Separate credentials**: Web UI and client tokens are independent

## Production Deployment

For production use with HTTPS:

1. Set up nginx with SSL (see main README)
2. Start server in HTTP mode (nginx handles TLS)
3. Access via `https://your-domain.com/login`

## Troubleshooting

### Can't connect to web UI
- Check server is running: `ps aux | grep server`
- Verify port 8080 is accessible: `netstat -an | grep 8080`
- Check firewall settings

### Terminal not working
- Ensure client is connected (check dashboard)
- Check client logs for errors
- Verify WebSocket connection in browser console

### Invalid credentials
- Double-check username and password flags
- Restart server with correct credentials
- Sessions expire after 24 hours

## Next Steps

- Explore the API endpoints (see main README)
- Set up auto-start for clients
- Configure nginx for production HTTPS
- Implement additional security measures

For complete documentation, see the main README.md file.
