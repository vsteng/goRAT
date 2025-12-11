package server

import (
	"time"

	"gorat/pkg/auth"
	"gorat/pkg/config"
	"gorat/pkg/logger"
	"gorat/pkg/storage"
)

// Services holds all major application services for dependency injection
type Services struct {
	Config     *config.ServerConfig
	Logger     *logger.Logger
	Storage    storage.Store
	ClientMgr  *ClientManager
	ProxyMgr   *ProxyManager
	SessionMgr auth.SessionManager
	TermProxy  *TerminalProxy
	Auth       auth.Authenticator
}

// NewServices creates and initializes all services
func NewServices(cfg *config.ServerConfig) (*Services, error) {
	log := logger.Get()

	log.InfoWith("initializing services", "config", cfg.String())

	// Initialize storage layer
	store, err := storage.NewSQLiteStore(cfg.Database.Path)
	if err != nil {
		log.ErrorWithErr("failed to initialize storage", err)
		return nil, err
	}

	// Initialize client manager
	clientMgr := NewClientManager()
	clientMgr.SetStore(store)

	// Initialize other services
	sessionMgr := auth.NewSessionManager(24 * time.Hour)
	termProxy := NewTerminalProxy(clientMgr, sessionMgr)
	proxyMgr := NewProxyManager(clientMgr, store)
	authenticator := auth.NewAuthenticator("")

	log.InfoWith("services initialized successfully")

	return &Services{
		Config:     cfg,
		Logger:     log,
		Storage:    store,
		ClientMgr:  clientMgr,
		ProxyMgr:   proxyMgr,
		SessionMgr: sessionMgr,
		TermProxy:  termProxy,
		Auth:       authenticator,
	}, nil
}
