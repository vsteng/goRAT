package config

import (
	"os"
	"testing"
)

// TestLoadConfig tests loading default config
func TestLoadConfig(t *testing.T) {
	os.Unsetenv("GORAT_CONFIG")
	cfg, err := LoadConfig("")
	if err != nil {
		t.Fatalf("Failed to load default config: %v", err)
	}
	if cfg == nil {
		t.Fatal("Config is nil")
	}
}

// TestLoadConfigDefaults tests default values are set
func TestLoadConfigDefaults(t *testing.T) {
	cfg, err := LoadConfig("")
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if cfg.Address == "" {
		t.Error("Address should not be empty")
	}
	if cfg.Database.Path == "" {
		t.Error("Database path should not be empty")
	}
	if cfg.WebUI.Username == "" {
		t.Error("WebUI username should not be empty")
	}
}

// TestConfigString tests String() method
func TestConfigString(t *testing.T) {
	cfg := &ServerConfig{
		Address: ":8080",
		Database: DatabaseConfig{
			Path: "clients.db",
		},
	}
	s := cfg.String()
	if s == "" {
		t.Error("String() should not return empty string")
	}
}
