//go:build debug
// +build debug

package client

import (
	"io"
	"log"
	"os"
)

const (
	// BuildMode indicates the build configuration
	BuildMode = "debug"

	// Default configuration for debug builds
	DefaultDaemon    = false
	DefaultAutoStart = true
	DefaultEnableLog = true
)

func init() {
	// Debug mode: Enable detailed logging
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.SetOutput(os.Stderr)
}

// SetupLogging configures logging for debug mode
func SetupLogging(daemon bool) io.WriteCloser {
	if daemon {
		// If running as daemon in debug mode, write to file
		logFile, err := os.OpenFile("client_debug.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err == nil {
			log.SetOutput(logFile)
			log.Printf("Debug mode: Logging to client_debug.log")
			return logFile
		}
		log.Printf("Warning: Failed to open log file: %v", err)
	}
	// Non-daemon or file open failed: log to stderr
	log.SetOutput(os.Stderr)
	return nil
}

// ShouldLog returns whether logging is enabled
func ShouldLog() bool {
	return true
}
