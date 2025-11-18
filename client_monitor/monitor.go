package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

// Monitor monitors and manages the client process
type Monitor struct {
	clientPath    string
	clientArgs    []string
	checkInterval time.Duration
	restartDelay  time.Duration
	maxRestarts   int
	restartCount  int
	lastRestart   time.Time
	running       bool
	stopChan      chan bool
}

// Config holds monitor configuration
type Config struct {
	ClientPath    string
	ClientArgs    []string
	CheckInterval time.Duration
	RestartDelay  time.Duration
	MaxRestarts   int
}

// NewMonitor creates a new monitor instance
func NewMonitor(config *Config) *Monitor {
	if config.CheckInterval == 0 {
		config.CheckInterval = 10 * time.Second
	}
	if config.RestartDelay == 0 {
		config.RestartDelay = 5 * time.Second
	}
	if config.MaxRestarts == 0 {
		config.MaxRestarts = -1 // Unlimited restarts
	}

	return &Monitor{
		clientPath:    config.ClientPath,
		clientArgs:    config.ClientArgs,
		checkInterval: config.CheckInterval,
		restartDelay:  config.RestartDelay,
		maxRestarts:   config.MaxRestarts,
		stopChan:      make(chan bool),
	}
}

// Start starts the monitor
func (m *Monitor) Start() error {
	log.Printf("Starting client monitor")
	log.Printf("Client path: %s", m.clientPath)
	log.Printf("Check interval: %v", m.checkInterval)

	// Check if client exists
	if _, err := os.Stat(m.clientPath); os.IsNotExist(err) {
		return fmt.Errorf("client not found at: %s", m.clientPath)
	}

	m.running = true

	// Initial client start
	if err := m.startClient(); err != nil {
		log.Printf("Failed to start client: %v", err)
	}

	// Start monitoring loop
	go m.monitorLoop()

	log.Printf("Monitor started successfully")
	return nil
}

// Stop stops the monitor
func (m *Monitor) Stop() {
	log.Printf("Stopping monitor...")
	m.running = false
	close(m.stopChan)
}

// monitorLoop continuously monitors the client
func (m *Monitor) monitorLoop() {
	ticker := time.NewTicker(m.checkInterval)
	defer ticker.Stop()

	for m.running {
		select {
		case <-ticker.C:
			if !m.isClientRunning() {
				log.Printf("Client is not running, attempting restart...")
				m.handleClientDown()
			}
		case <-m.stopChan:
			return
		}
	}
}

// isClientRunning checks if the client process is running
func (m *Monitor) isClientRunning() bool {
	clientName := filepath.Base(m.clientPath)

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		// Windows: use tasklist
		cmd = exec.Command("tasklist", "/FI", fmt.Sprintf("IMAGENAME eq %s", clientName), "/NH")
	} else {
		// Linux/Unix: use pgrep
		cmd = exec.Command("pgrep", "-f", clientName)
	}

	output, err := cmd.Output()
	if err != nil {
		return false
	}

	// If output is not empty, process is running
	return len(output) > 0
}

// handleClientDown handles when client is detected as down
func (m *Monitor) handleClientDown() {
	// Check max restarts
	if m.maxRestarts > 0 && m.restartCount >= m.maxRestarts {
		log.Printf("Maximum restart attempts (%d) reached, stopping monitor", m.maxRestarts)
		m.Stop()
		return
	}

	// Check restart delay
	if time.Since(m.lastRestart) < m.restartDelay {
		log.Printf("Waiting for restart delay...")
		time.Sleep(m.restartDelay - time.Since(m.lastRestart))
	}

	// Restart client
	if err := m.startClient(); err != nil {
		log.Printf("Failed to restart client: %v", err)
	} else {
		m.restartCount++
		m.lastRestart = time.Now()
		log.Printf("Client restarted (attempt %d)", m.restartCount)
	}
}

// startClient starts the client process
func (m *Monitor) startClient() error {
	log.Printf("Starting client: %s %v", m.clientPath, m.clientArgs)

	cmd := exec.Command(m.clientPath, m.clientArgs...)

	// Detach process
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil

	if runtime.GOOS != "windows" {
		// Unix: set process group to detach
		cmd.SysProcAttr = getSysProcAttr()
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start client: %v", err)
	}

	// Release process so it runs independently
	if err := cmd.Process.Release(); err != nil {
		return fmt.Errorf("failed to release process: %v", err)
	}

	log.Printf("Client started successfully (PID: %d)", cmd.Process.Pid)
	return nil
}

// installClient installs the client if not present
func (m *Monitor) InstallClient(sourcePath string) error {
	log.Printf("Installing client from %s to %s", sourcePath, m.clientPath)

	// Check if source exists
	if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
		return fmt.Errorf("source client not found: %s", sourcePath)
	}

	// Create destination directory
	destDir := filepath.Dir(m.clientPath)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %v", err)
	}

	// Copy file
	if err := copyFile(sourcePath, m.clientPath); err != nil {
		return fmt.Errorf("failed to copy client: %v", err)
	}

	// Make executable
	if err := os.Chmod(m.clientPath, 0755); err != nil {
		return fmt.Errorf("failed to set permissions: %v", err)
	}

	log.Printf("Client installed successfully")
	return nil
}

// GetStats returns monitor statistics
func (m *Monitor) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"running":        m.running,
		"restart_count":  m.restartCount,
		"last_restart":   m.lastRestart,
		"client_running": m.isClientRunning(),
	}
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer source.Close()

	destination, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destination.Close()

	_, err = destination.ReadFrom(source)
	return err
}
