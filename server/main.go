package server

import (
	"flag"
	"log"
)

func Main() {
	// Parse command line flags
	addr := flag.String("addr", ":8080", "Server address")
	certFile := flag.String("cert", "", "TLS certificate file (leave empty for HTTP behind nginx)")
	keyFile := flag.String("key", "", "TLS key file (leave empty for HTTP behind nginx)")
	authToken := flag.String("token", "your-secret-token", "Authentication token")
	useTLS := flag.Bool("tls", false, "Enable TLS (use false when behind nginx)")
	webUsername := flag.String("web-user", "admin", "Web UI username")
	webPassword := flag.String("web-pass", "admin", "Web UI password")
	flag.Parse()

	// Create server configuration
	config := &Config{
		Address:     *addr,
		CertFile:    *certFile,
		KeyFile:     *keyFile,
		AuthToken:   *authToken,
		UseTLS:      *useTLS,
		WebUsername: *webUsername,
		WebPassword: *webPassword,
	}

	// Create and start server
	srv := NewServer(config)
	if config.UseTLS {
		log.Printf("Starting server with TLS on %s", *addr)
	} else {
		log.Printf("Starting server (HTTP) on %s - ensure nginx handles TLS", *addr)
	}
	log.Printf("Web UI will be available at http://localhost%s/login", *addr)
	log.Printf("Web UI credentials - Username: %s, Password: %s", *webUsername, *webPassword)

	if err := srv.Start(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
