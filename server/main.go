package server

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gorat/pkg/config"
	"gorat/pkg/logger"
)

func Main() {
	// Check for help flag early before instance check
	if len(os.Args) > 1 && (os.Args[len(os.Args)-1] == "-h" || os.Args[len(os.Args)-1] == "--help") {
		// Parse flags to display help
		fs := flag.NewFlagSet("server", flag.ContinueOnError)
		fs.String("addr", ":8080", "Server address")
		fs.String("config", "", "Config file path (optional)")
		fs.String("cert", "", "TLS certificate file (leave empty for HTTP behind nginx)")
		fs.String("key", "", "TLS key file (leave empty for HTTP behind nginx)")
		fs.Bool("tls", false, "Enable TLS (use false when behind nginx)")
		fs.String("web-user", "admin", "Web UI username")
		fs.String("web-pass", "admin", "Web UI password")
		fs.String("log-level", "info", "Log level: debug, info, warn, error")
		fs.String("log-format", "text", "Log format: text or json")
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
	configPath := flag.String("config", "", "Config file path (optional)")
	certFile := flag.String("cert", "", "TLS certificate file (leave empty for HTTP behind nginx)")
	keyFile := flag.String("key", "", "TLS key file (leave empty for HTTP behind nginx)")
	useTLS := flag.Bool("tls", false, "Enable TLS (use false when behind nginx)")
	webUsername := flag.String("web-user", "admin", "Web UI username")
	webPassword := flag.String("web-pass", "admin", "Web UI password")
	logLevel := flag.String("log-level", "info", "Log level: debug, info, warn, error")
	logFormat := flag.String("log-format", "text", "Log format: text or json")
	flag.Parse()

	// Initialize structured logger
	logger.Init(logger.LogLevel(*logLevel), *logFormat)
	log := logger.Get()

	log.InfoWith("server starting", "version", "1.0.0")

	// Load configuration (from file or defaults)
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.ErrorWithErr("failed to load configuration", err)
		return
	}

	// Override config with command-line flags if provided
	if *addr != ":8080" {
		cfg.Address = *addr
	}
	if *webUsername != "admin" {
		cfg.WebUI.Username = *webUsername
	}
	if *webPassword != "admin" {
		cfg.WebUI.Password = *webPassword
	}
	if *certFile != "" {
		cfg.TLS.CertFile = *certFile
	}
	if *keyFile != "" {
		cfg.TLS.KeyFile = *keyFile
	}
	if *useTLS {
		cfg.TLS.Enabled = true
	}

	log.InfoWith("configuration loaded", "address", cfg.Address, "tls", cfg.TLS.Enabled)

	// Initialize services (dependency injection container)
	// Future: Pass services to Server instead of creating multiple instances
	_, err = NewServices(cfg)
	if err != nil {
		log.ErrorWithErr("failed to initialize services", err)
		return
	}

	// Create server instance (legacy approach for now)
	srv, err := NewServerWithRecovery(&Config{
		Address:     cfg.Address,
		CertFile:    cfg.TLS.CertFile,
		KeyFile:     cfg.TLS.KeyFile,
		AuthToken:   "",
		UseTLS:      cfg.TLS.Enabled,
		WebUsername: cfg.WebUI.Username,
		WebPassword: cfg.WebUI.Password,
	})
	if err != nil {
		log.ErrorWithErr("failed to create server", err)
		return
	}

	// Write PID file for instance management
	if err := instanceMgr.WritePID(); err != nil {
		log.WarnWith("failed to write PID file", "error", err)
	}
	defer instanceMgr.RemovePID()

	if cfg.TLS.Enabled {
		log.InfoWith("starting server with TLS", "address", cfg.Address)
	} else {
		log.InfoWith("starting server with HTTP", "address", cfg.Address, "note", "ensure nginx handles TLS")
	}
	log.InfoWith("web UI available", "url", fmt.Sprintf("http://localhost%s/login", cfg.Address))
	log.InfoWith("web UI credentials", "username", cfg.WebUI.Username)
	log.InfoWith("authentication method", "type", "machine ID")

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)

	// Start server in a goroutine
	errorChan := make(chan error, 1)
	go func() {
		if err := srv.Start(); err != nil {
			log.ErrorWithErr("server error", err)
			errorChan <- err
		}
	}()

	log.InfoWith("server is running", "press", "Ctrl+C to stop")

	// Wait for shutdown signal or error
	select {
	case sig := <-sigChan:
		log.InfoWith("received signal", "signal", sig.String())
		log.InfoWith("shutting down server gracefully")

		// Create shutdown context with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := srv.Shutdown(ctx); err != nil {
			log.ErrorWithErr("error during shutdown", err)
		}
		log.InfoWith("server stopped")
		return

	case err := <-errorChan:
		if err != nil {
			log.ErrorWithErr("server encountered fatal error", err)
		}
		log.InfoWith("server stopped")
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
