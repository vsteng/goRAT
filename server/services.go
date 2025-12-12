package server

import (
	"time"

	"gorat/pkg/api"
	"gorat/pkg/auth"
	"gorat/pkg/clients"
	"gorat/pkg/config"
	"gorat/pkg/logger"
	"gorat/pkg/storage"
)

// Services holds all major application services for dependency injection
type Services struct {
	Config       *config.ServerConfig
	Logger       *logger.Logger
	Storage      storage.Store
	ClientMgr    clients.Manager
	ProxyMgr     *ProxyManager
	SessionMgr   auth.SessionManager
	TermProxy    *TerminalProxy
	Auth         auth.Authenticator
	APIHandler   *api.Handler
	AdminHandler *api.AdminHandler
}

// NewServices creates and initializes all services
func NewServices(cfg *config.ServerConfig) (*Services, error) {
	log := logger.Get()

	log.InfoWith("initializing services", "config", cfg.String())

	// Initialize storage layer via factory (supports sqlite, postgres)
	store, err := storage.NewStore(cfg.Database)
	if err != nil {
		log.ErrorWithErr("failed to initialize storage", err)
		return nil, err
	}

	// Initialize client manager
	clientMgr := clients.NewManager()
	clientMgr.Start()

	// Initialize other services
	sessionMgr := auth.NewSessionManager(24 * time.Hour)
	termProxy := NewTerminalProxy(clientMgr, sessionMgr)
	proxyMgr := NewProxyManager(clientMgr, store)
	authenticator := auth.NewAuthenticator("")

	// Initialize API handlers
	apiHandler, err := api.NewHandler(sessionMgr, clientMgr, store, cfg.WebUI.Username, cfg.WebUI.Password)
	if err != nil {
		log.ErrorWithErr("failed to initialize API handler", err)
		return nil, err
	}

	adminHandler := api.NewAdminHandler(clientMgr, store)

	log.InfoWith("services initialized successfully")

	return &Services{
		Config:       cfg,
		Logger:       log,
		Storage:      store,
		ClientMgr:    clientMgr,
		ProxyMgr:     proxyMgr,
		SessionMgr:   sessionMgr,
		TermProxy:    termProxy,
		Auth:         authenticator,
		APIHandler:   apiHandler,
		AdminHandler: adminHandler,
	}, nil
}
