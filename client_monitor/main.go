package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

func main() {
	// Parse command line flags
	clientPath := flag.String("client", "./client", "Path to client executable")
	installFrom := flag.String("install", "", "Install client from this path")
	checkInterval := flag.Duration("interval", 10*time.Second, "Health check interval")
	restartDelay := flag.Duration("delay", 5*time.Second, "Delay between restarts")
	maxRestarts := flag.Int("max-restarts", -1, "Maximum restart attempts (-1 for unlimited)")
	flag.Parse()

	// Get absolute path to client
	absClientPath, err := filepath.Abs(*clientPath)
	if err != nil {
		log.Fatalf("Failed to get absolute path: %v", err)
	}

	// Collect remaining args for client
	clientArgs := flag.Args()

	config := &Config{
		ClientPath:    absClientPath,
		ClientArgs:    clientArgs,
		CheckInterval: *checkInterval,
		RestartDelay:  *restartDelay,
		MaxRestarts:   *maxRestarts,
	}

	log.Printf("Client Monitor Starting...")
	log.Printf("Monitoring client: %s", config.ClientPath)

	// Create monitor
	mon := NewMonitor(config)

	// Install client if requested
	if *installFrom != "" {
		log.Printf("Installing client from: %s", *installFrom)
		if err := mon.InstallClient(*installFrom); err != nil {
			log.Fatalf("Failed to install client: %v", err)
		}
		log.Printf("Client installed successfully")
	}

	// Start monitor
	if err := mon.Start(); err != nil {
		log.Fatalf("Failed to start monitor: %v", err)
	}

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Wait for termination signal
	<-sigChan
	log.Printf("Received termination signal")

	// Stop monitor
	mon.Stop()

	// Print final stats
	stats := mon.GetStats()
	log.Printf("Final statistics: %+v", stats)

	log.Printf("Client monitor stopped")
}
