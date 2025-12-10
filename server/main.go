package server

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func Main() {
	// Check for help flag early before instance check
	if len(os.Args) > 1 && (os.Args[len(os.Args)-1] == "-h" || os.Args[len(os.Args)-1] == "--help") {
		// Parse flags to display help
		fs := flag.NewFlagSet("server", flag.ContinueOnError)
		fs.String("addr", ":8080", "Server address")
		fs.String("cert", "", "TLS certificate file (leave empty for HTTP behind nginx)")
		fs.String("key", "", "TLS key file (leave empty for HTTP behind nginx)")
		fs.Bool("tls", false, "Enable TLS (use false when behind nginx)")
		fs.String("web-user", "admin", "Web UI username")
		fs.String("web-pass", "admin", "Web UI password")
		printHelp(fs)
		return
	}

	// Handle subcommands: start|stop|restart|status (default: start)
	command := "start"
	if len(os.Args) > 1 {
		first := os.Args[1]
		if first == "start" || first == "stop" || first == "restart" || first == "status" {
			command = first
			// Remove subcommand from args before flag parsing
			os.Args = append([]string{os.Args[0]}, os.Args[2:]...)
		}
	}

	instanceMgr := NewServerInstanceManager()

	// Handle subcommands
	if command != "start" {
		switch command {
		case "status":
			if running, pid := instanceMgr.IsRunning(); running {
				fmt.Printf("Server running (PID %d)\n", pid)
			} else {
				fmt.Println("Server not running")
			}
			return
		case "stop":
			if err := instanceMgr.Kill(); err != nil {
				fmt.Printf("Stop failed: %v\n", err)
			} else {
				fmt.Println("Server stopped")
			}
			return
		case "restart":
			_ = instanceMgr.Kill() // Ignore error; may not be running
			// Continue to start below.
			fmt.Println("Restarting server...")
		}
	}

	// Enforce single instance before starting
	if command == "start" {
		if running, pid := instanceMgr.IsRunning(); running {
			fmt.Printf("Server already running (PID %d)\n", pid)
			return
		}
	}

	// Parse command line flags
	addr := flag.String("addr", ":8080", "Server address")
	certFile := flag.String("cert", "", "TLS certificate file (leave empty for HTTP behind nginx)")
	keyFile := flag.String("key", "", "TLS key file (leave empty for HTTP behind nginx)")
	useTLS := flag.Bool("tls", false, "Enable TLS (use false when behind nginx)")
	webUsername := flag.String("web-user", "admin", "Web UI username")
	webPassword := flag.String("web-pass", "admin", "Web UI password")
	flag.Parse()

	// Create server configuration
	config := &Config{
		Address:     *addr,
		CertFile:    *certFile,
		KeyFile:     *keyFile,
		AuthToken:   "", // No longer used - machine ID is the token
		UseTLS:      *useTLS,
		WebUsername: *webUsername,
		WebPassword: *webPassword,
	}

	// Create server with error recovery
	srv, err := NewServerWithRecovery(config)
	if err != nil {
		log.Printf("WARNING: Server initialization error: %v", err)
		log.Println("Attempting to continue with limited functionality...")
		return
	}

	// Write PID file for instance management
	if err := instanceMgr.WritePID(); err != nil {
		log.Printf("Warning: Failed to write PID file: %v", err)
	}
	defer instanceMgr.RemovePID()

	if config.UseTLS {
		log.Printf("Starting server with TLS on %s", *addr)
	} else {
		log.Printf("Starting server (HTTP) on %s - ensure nginx handles TLS", *addr)
	}
	log.Printf("Web UI will be available at http://localhost%s/login", *addr)
	log.Printf("Web UI credentials - Username: %s, Password: %s", *webUsername, *webPassword)
	log.Printf("Authentication: Clients use machine ID (no token required)")

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)

	// Start server in a goroutine
	errorChan := make(chan error, 1)
	go func() {
		if err := srv.Start(); err != nil {
			log.Printf("Server error: %v", err)
			errorChan <- err
		}
	}()

	log.Println("Server is running. Press Ctrl+C to stop.")

	// Wait for shutdown signal or error
	select {
	case sig := <-sigChan:
		log.Printf("Received signal: %v", sig)
		log.Println("Shutting down server gracefully...")

		// Create shutdown context with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := srv.Shutdown(ctx); err != nil {
			log.Printf("Error during shutdown: %v", err)
		}
		log.Println("Server stopped.")
		return

	case err := <-errorChan:
		if err != nil {
			log.Printf("Server encountered fatal error: %v", err)
			log.Println("Server stopped.")
		}
		return
	}
}

// printHelp displays help information for the server
func printHelp(fs *flag.FlagSet) {
	fmt.Print(`Server Manager - Usage:

Commands:
  start              Start the server (default if no command given)
  stop               Stop the running server
  restart            Restart the server
  status             Show server status

Flags:
`)
	fs.PrintDefaults()
	fmt.Print(`
Examples:
  ./bin/server                                    # Start on default port 8080
  ./bin/server -addr 127.0.0.1:8081              # Start on custom port
  ./bin/server -addr :8080 -tls                  # Start with TLS
  ./bin/server stop                              # Stop the server
  ./bin/server restart                           # Restart the server
  ./bin/server status                            # Check if server is running
`)
}
