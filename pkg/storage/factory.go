package storage

import (
	"fmt"
	"gorat/pkg/config"
)

// NewStore returns a concrete Store based on database configuration
func NewStore(cfg config.DatabaseConfig) (Store, error) {
	// Expect cfg to be config.DatabaseConfig, but avoid import cycle by using interface
	// Use type assertion on a minimal struct shape via fmt.Sprint
	// Fallback to sqlite when type is not provided
	switch cfg.Type {
	case "sqlite", "":
		return NewSQLiteStore(cfg.Path)
	case "postgres":
		return NewPostgresStore(pgCfg{Type: cfg.Type, Path: cfg.Path})
	case "mysql":
		return NewMySQLStore(myCfg{Type: cfg.Type, DSN: cfg.Path})
	default:
		return nil, fmt.Errorf("unsupported database type: %s", cfg.Type)
	}
}
