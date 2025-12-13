//go:build !debug
// +build !debug

package client

import (
	"io"
	"log"
	"os"
)

const (
	// BuildMode indicates the build configuration
	BuildMode = "release"

	// Default configuration for release builds
	DefaultDaemon    = true
	DefaultAutoStart = true
	DefaultEnableLog = false
	// ShowHelp controls whether to show help/usage information
	ShowHelp = false
)

func init() {
	// Release mode: Disable logging by default
	if !DefaultEnableLog {
		log.SetOutput(io.Discard)
	}
}

// SetupLogging configures logging for release mode
func SetupLogging(daemon bool) io.WriteCloser {
	// Release mode: disable logging unless explicitly enabled
	// Could be extended to check environment variable if needed
	logEnv := os.Getenv("CLIENT_ENABLE_LOG")
	if logEnv == "1" || logEnv == "true" {
		if daemon {
			logFile, err := os.OpenFile("client.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
			if err == nil {
				log.SetOutput(logFile)
				return logFile
			}
		} else {
			log.SetOutput(os.Stderr)
		}
		return nil
	}

	// Disable logging
	log.SetOutput(io.Discard)
	return nil
}

// ShouldLog returns whether logging is enabled
func ShouldLog() bool {
	logEnv := os.Getenv("CLIENT_ENABLE_LOG")
	return logEnv == "1" || logEnv == "true"
}
