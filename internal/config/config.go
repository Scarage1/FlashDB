// Package config provides configuration management for FlashDB.
package config

import (
	"encoding/json"
	"os"
	"time"
)

// Config holds the FlashDB server configuration.
type Config struct {
	// Server settings
	Addr    string `json:"addr"`
	DataDir string `json:"data_dir"`

	// Logging
	LogLevel string `json:"log_level"`

	// Performance
	MaxClients   int           `json:"max_clients"`
	ReadTimeout  time.Duration `json:"read_timeout"`
	WriteTimeout time.Duration `json:"write_timeout"`

	// Persistence
	SyncWrites bool `json:"sync_writes"`
}

// DefaultConfig returns the default configuration.
func DefaultConfig() *Config {
	return &Config{
		Addr:         ":6379",
		DataDir:      "data",
		LogLevel:     "info",
		MaxClients:   10000,
		ReadTimeout:  0, // No timeout
		WriteTimeout: 0, // No timeout
		SyncWrites:   true,
	}
}

// Load loads configuration from a JSON file.
func Load(path string) (*Config, error) {
	cfg := DefaultConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, err
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Save saves the configuration to a JSON file.
func (c *Config) Save(path string) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
