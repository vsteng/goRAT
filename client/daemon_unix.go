//go:build !windows
// +build !windows

package client

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"syscall"
)

// Daemonize runs the client as a background daemon on Unix systems
func Daemonize() error {
	// Check if we're already running as a daemon
	if os.Getppid() == 1 {
		// We're already a daemon (parent PID is 1 - init/systemd)
		return nil
	}

	// Get the executable path
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %v", err)
	}

	// Prepare command to run ourselves as daemon
	cmd := exec.Command(execPath, os.Args[1:]...)

	// Set process attributes to detach from terminal
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true, // Create new session
	}

	// Redirect stdout/stderr to /dev/null or log file
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil

	// Start the daemon process
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start daemon: %v", err)
	}

	log.Printf("Started daemon process with PID: %d", cmd.Process.Pid)

	// Parent process exits, daemon continues
	os.Exit(0)
	return nil
}

// IsDaemon checks if the process is running as a daemon
func IsDaemon() bool {
	return os.Getppid() == 1
}
