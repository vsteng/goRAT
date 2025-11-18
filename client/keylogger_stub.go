//go:build !windows && !linux
// +build !windows,!linux

package client

import (
	"fmt"
	"log"
)

// startPlatformMonitor is a stub for unsupported platforms
func (kl *Keylogger) startPlatformMonitor() error {
	return fmt.Errorf("keylogger not supported on this platform")
}

// monitorRDP is a stub for unsupported platforms
func (kl *Keylogger) monitorRDP() {
	log.Printf("RDP monitoring not supported on this platform")
	kl.mu.Lock()
	kl.running = false
	kl.mu.Unlock()
}

// monitorGeneral is a stub for unsupported platforms
func (kl *Keylogger) monitorGeneral() {
	log.Printf("General keyboard monitoring not supported on this platform")
	kl.mu.Lock()
	kl.running = false
	kl.mu.Unlock()
}

// monitorSSH is a stub for unsupported platforms
func (kl *Keylogger) monitorSSH() {
	log.Printf("SSH monitoring not supported on this platform")
	kl.mu.Lock()
	kl.running = false
	kl.mu.Unlock()
}
