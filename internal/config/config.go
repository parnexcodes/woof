package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

// Config holds the application configuration
type Config struct {
	Concurrency int             `mapstructure:"concurrency"`
	Verbose     bool            `mapstructure:"verbose"`
	Output      string          `mapstructure:"output"`
	Providers   []ProviderConfig `mapstructure:"providers"`
	Upload      UploadConfig    `mapstructure:"upload"`
}

// ProviderConfig holds configuration for a file hosting provider
type ProviderConfig struct {
	Name     string                 `mapstructure:"name"`
	Enabled  bool                   `mapstructure:"enabled"`
	Settings map[string]interface{} `mapstructure:"settings"`
}

// UploadConfig holds upload-specific configuration
type UploadConfig struct {
	RetryAttempts int           `mapstructure:"retry_attempts"`
	RetryDelay    time.Duration `mapstructure:"retry_delay"`
	ChunkSize     int64         `mapstructure:"chunk_size"`
	Timeout       time.Duration `mapstructure:"timeout"`
}

// LoadConfig loads configuration from file and environment
func LoadConfig() (*Config, error) {
	config := &Config{}

	// Set default values
	setDefaults()

	// Read configuration
	if err := viper.Unmarshal(config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return config, nil
}

func setDefaults() {
	// Global defaults
	viper.SetDefault("concurrency", 5)
	viper.SetDefault("verbose", false)
	viper.SetDefault("output", "text")

	// Upload defaults
	viper.SetDefault("upload.retry_attempts", 3)
	viper.SetDefault("upload.retry_delay", "2s")
	viper.SetDefault("upload.chunk_size", 1024*1024) // 1MB
	viper.SetDefault("upload.timeout", "30m")

	// Provider defaults
	viper.SetDefault("providers", []ProviderConfig{
		{
			Name:    "buzzheavier",
			Enabled: false,
			Settings: map[string]interface{}{
				"upload_url":        "https://w.buzzheavier.com",
				"download_base_url": "https://buzzheavier.com",
				"timeout":           "10m",
			},
		},
	})
}

// GetEnabledProviders returns a list of enabled provider configurations
func (c *Config) GetEnabledProviders() []ProviderConfig {
	var enabled []ProviderConfig
	for _, provider := range c.Providers {
		if provider.Enabled {
			enabled = append(enabled, provider)
		}
	}
	return enabled
}