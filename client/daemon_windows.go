//go:build windows
// +build windows

package client

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"syscall"
)

// Daemonize runs the client as a background service on Windows
func Daemonize() error {
	// Check if we're already running detached
	if isRunningDetached() {
		return nil
	}

	// Get the executable path
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %v", err)
	}

	// Prepare command to run ourselves detached
	cmd := exec.Command(execPath, os.Args[1:]...)

	// Set process attributes to run detached
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000, // CREATE_NO_WINDOW
	}

	// Don't inherit stdio
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil

	// Start the detached process
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start detached process: %v", err)
	}

	log.Printf("Started detached process with PID: %d", cmd.Process.Pid)

	// Parent process exits, detached process continues
	os.Exit(0)
	return nil
}

// isRunningDetached checks if the process is running detached
func isRunningDetached() bool {
	// Check if we have a console window
	// This is a simplified check - in production you might want more sophisticated detection
	return os.Getenv("DAEMON_MODE") == "1"
}

// IsDaemon checks if the process is running as a service/daemon
func IsDaemon() bool {
	return isRunningDetached()
}
