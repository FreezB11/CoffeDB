package config

import (
	"encoding/json"
	"os"
)

// Config represents the application configuration
type Config struct {
	Server  ServerConfig  `json:"server"`
	Storage StorageConfig `json:"storage"`
	Logging LoggingConfig `json:"logging"`
}

// ServerConfig contains HTTP server configuration
type ServerConfig struct {
	Port         string `json:"port"`
	Debug        bool   `json:"debug"`
	ReadTimeout  int    `json:"read_timeout"`
	WriteTimeout int    `json:"write_timeout"`
	IdleTimeout  int    `json:"idle_timeout"`
}

// StorageConfig contains storage engine configuration
type StorageConfig struct {
	DataDir             string `json:"data_dir"`
	MemtableSize        int64  `json:"memtable_size"`
	CompactionInterval  int    `json:"compaction_interval"`
	WALSyncInterval     int    `json:"wal_sync_interval"`
	EnableCompression   bool   `json:"enable_compression"`
	MaxOpenFiles        int    `json:"max_open_files"`
}

// LoggingConfig contains logging configuration
type LoggingConfig struct {
	Level  string `json:"level"`
	Format string `json:"format"`
	File   string `json:"file"`
}

// Load loads configuration from file
func Load(filename string) (*Config, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var config Config
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&config); err != nil {
		return nil, err
	}

	// Apply environment variable overrides
	config.applyEnvOverrides()

	return &config, nil
}

// Default returns default configuration
func Default() *Config {
	config := &Config{
		Server: ServerConfig{
			Port:         "8080",
			Debug:        false,
			ReadTimeout:  30,
			WriteTimeout: 30,
			IdleTimeout:  120,
		},
		Storage: StorageConfig{
			DataDir:            "./data",
			// MemtableSize:       64 * 1024 * 1024, // 64MB
			MemtableSize:       1024,
			CompactionInterval: 3600,             // 1 hour
			WALSyncInterval:    1,                // 1 second
			EnableCompression:  false,
			MaxOpenFiles:       1000,
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
			File:   "",
		},
	}

	config.applyEnvOverrides()
	return config
}

// applyEnvOverrides applies environment variable overrides
func (c *Config) applyEnvOverrides() {
	if port := os.Getenv("coffedb_PORT"); port != "" {
		c.Server.Port = port
	}
	
	if dataDir := os.Getenv("coffedb_DATA_DIR"); dataDir != "" {
		c.Storage.DataDir = dataDir
	}
	
	if debug := os.Getenv("coffedb_DEBUG"); debug == "true" {
		c.Server.Debug = true
	}
	
	if compression := os.Getenv("coffedb_COMPRESSION"); compression == "true" {
		c.Storage.EnableCompression = true
	}
}

// Save saves configuration to file
func (c *Config) Save(filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(c)
}
