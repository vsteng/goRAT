package server

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func Main() {
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
		defer func() {
			if r := recover(); r != nil {
				log.Printf("PANIC RECOVERED in server: %v", r)
				log.Println("Server will attempt to restart...")
				// Signal error to restart
				errorChan <- nil
			}
		}()

		if err := srv.Start(); err != nil {
			log.Printf("Server error: %v", err)
			errorChan <- err
		}
	}()

	log.Println("Server is running. Press Ctrl+C to stop.")

	// Wait for shutdown signal or error
	for {
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
				log.Printf("Server encountered error: %v", err)
				log.Println("Attempting to restart server in 5 seconds...")
				time.Sleep(5 * time.Second)

				// Restart server
				go func() {
					defer func() {
						if r := recover(); r != nil {
							log.Printf("PANIC RECOVERED in server restart: %v", r)
							errorChan <- nil
						}
					}()

					if err := srv.Start(); err != nil {
						log.Printf("Server restart error: %v", err)
						errorChan <- err
					}
				}()
			} else {
				log.Println("Server recovered from panic, restarting...")
				time.Sleep(2 * time.Second)

				// Restart after panic
				go func() {
					defer func() {
						if r := recover(); r != nil {
							log.Printf("PANIC RECOVERED in server restart: %v", r)
							errorChan <- nil
						}
					}()

					if err := srv.Start(); err != nil {
						log.Printf("Server restart error: %v", err)
						errorChan <- err
					}
				}()
			}
		}
	}
}
