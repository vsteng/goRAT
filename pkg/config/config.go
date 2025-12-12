package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// ServerConfig represents server configuration
type ServerConfig struct {
	Address        string         `yaml:"address"`
	TLS            TLSConfig      `yaml:"tls"`
	WebUI          WebUIConfig    `yaml:"webui"`
	Database       DatabaseConfig `yaml:"database"`
	Logging        LoggingConfig  `yaml:"logging"`
	ConnectionPool PoolConfig     `yaml:"connection_pool"`
}

// TLSConfig represents TLS settings
type TLSConfig struct {
	Enabled     bool   `yaml:"enabled"`
	CertFile    string `yaml:"cert_file"`
	KeyFile     string `yaml:"key_file"`
	BehindProxy bool   `yaml:"behind_proxy"`
}

// WebUIConfig represents web UI settings
type WebUIConfig struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	Port     int    `yaml:"port"`
}

// DatabaseConfig represents database settings
type DatabaseConfig struct {
	Type              string `yaml:"type"` // sqlite | postgres
	Path              string `yaml:"path"`
	MaxConnections    int    `yaml:"max_connections"`
	ConnectionTimeout int    `yaml:"connection_timeout"`
}

// LoggingConfig represents logging settings
type LoggingConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

// PoolConfig represents connection pool settings
type PoolConfig struct {
	MaxPooledConns   int `yaml:"max_pooled_conns"`
	PoolConnIdleTime int `yaml:"pool_conn_idle_time_seconds"`
	PoolConnLifetime int `yaml:"pool_conn_lifetime_seconds"`
}

// DefaultConfig returns default configuration
func DefaultConfig() *ServerConfig {
	return &ServerConfig{
		Address: ":8080",
		TLS: TLSConfig{
			Enabled:     false,
			CertFile:    "",
			KeyFile:     "",
			BehindProxy: false,
		},
		WebUI: WebUIConfig{
			Username: "admin",
			Password: "admin",
			Port:     8080,
		},
		Database: DatabaseConfig{
			Type:              "sqlite",
			Path:              "./clients.db",
			MaxConnections:    25,
			ConnectionTimeout: 30,
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "text",
		},
		ConnectionPool: PoolConfig{
			MaxPooledConns:   10,
			PoolConnIdleTime: 300,
			PoolConnLifetime: 1800,
		},
	}
}

// LoadConfig loads configuration from file and environment variables
func LoadConfig(configPath string) (*ServerConfig, error) {
	config := DefaultConfig()

	// Load from file if provided
	if configPath != "" {
		if err := loadFromFile(configPath, config); err != nil {
			return nil, fmt.Errorf("failed to load config file: %w", err)
		}
	}

	// Override with environment variables
	applyEnvOverrides(config)

	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return config, nil
}

// loadFromFile loads configuration from a YAML file
func loadFromFile(path string, config *ServerConfig) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	if err := yaml.Unmarshal(data, config); err != nil {
		return err
	}

	return nil
}

// applyEnvOverrides applies environment variable overrides
func applyEnvOverrides(config *ServerConfig) {
	if addr := os.Getenv("SERVER_ADDR"); addr != "" {
		config.Address = addr
	}

	if username := os.Getenv("WEB_USERNAME"); username != "" {
		config.WebUI.Username = username
	}

	if password := os.Getenv("WEB_PASSWORD"); password != "" {
		config.WebUI.Password = password
	}

	if dbPath := os.Getenv("DB_PATH"); dbPath != "" {
		config.Database.Path = dbPath
	}

	if logLevel := os.Getenv("LOG_LEVEL"); logLevel != "" {
		config.Logging.Level = logLevel
	}

	if logFormat := os.Getenv("LOG_FORMAT"); logFormat != "" {
		config.Logging.Format = logFormat
	}

	if tlsEnabled := os.Getenv("TLS_ENABLED"); tlsEnabled != "" {
		config.TLS.Enabled = tlsEnabled == "true"
	}

	if certFile := os.Getenv("TLS_CERT_FILE"); certFile != "" {
		config.TLS.CertFile = certFile
	}

	if keyFile := os.Getenv("TLS_KEY_FILE"); keyFile != "" {
		config.TLS.KeyFile = keyFile
	}

	if maxConns := os.Getenv("DB_MAX_CONNECTIONS"); maxConns != "" {
		if val, err := strconv.Atoi(maxConns); err == nil {
			config.Database.MaxConnections = val
		}
	}
}

// Validate validates the configuration
func (c *ServerConfig) Validate() error {
	if c.Address == "" {
		return fmt.Errorf("server address cannot be empty")
	}

	if c.WebUI.Username == "" {
		return fmt.Errorf("web UI username cannot be empty")
	}

	if c.WebUI.Password == "" {
		return fmt.Errorf("web UI password cannot be empty")
	}

	if c.TLS.Enabled {
		if c.TLS.CertFile == "" || c.TLS.KeyFile == "" {
			return fmt.Errorf("TLS enabled but cert/key files not provided")
		}

		if _, err := os.Stat(c.TLS.CertFile); err != nil {
			return fmt.Errorf("certificate file not found: %w", err)
		}

		if _, err := os.Stat(c.TLS.KeyFile); err != nil {
			return fmt.Errorf("key file not found: %w", err)
		}
	}

	if c.Database.MaxConnections < 1 {
		return fmt.Errorf("database max connections must be at least 1")
	}

	if !isValidLogLevel(c.Logging.Level) {
		return fmt.Errorf("invalid log level: %s", c.Logging.Level)
	}

	return nil
}

// isValidLogLevel checks if the log level is valid
func isValidLogLevel(level string) bool {
	valid := []string{"debug", "info", "warn", "error"}
	level = strings.ToLower(level)
	for _, v := range valid {
		if level == v {
			return true
		}
	}
	return false
}

// GetDatabasePath returns the absolute database path
func (c *ServerConfig) GetDatabasePath() string {
	if filepath.IsAbs(c.Database.Path) {
		return c.Database.Path
	}
	return filepath.Join(os.Getenv("PWD"), c.Database.Path)
}

// String returns a string representation of the configuration (for logging)
func (c *ServerConfig) String() string {
	return fmt.Sprintf("Config{Address: %s, DB: %s, TLS: %v, LogLevel: %s}",
		c.Address, c.Database.Path, c.TLS.Enabled, c.Logging.Level)
}
