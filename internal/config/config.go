package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// RepoConfig holds settings for a single git repository.
type RepoConfig struct {
	Name         string `yaml:"name"`
	URL          string `yaml:"url"`
	Branch       string `yaml:"branch"`
	ClustersPath string `yaml:"clusters_path"`
	MCPath       string `yaml:"mc_path"`
	Token        string `yaml:"-"` // not serialised to disk in plaintext
}

// Config holds all configuration for omni-cd.
type Config struct {
	// Omni connection settings
	OmniEndpoint          string
	OmniServiceAccountKey string

	// Sync behaviour
	RefreshInterval time.Duration // How often to check for new git commits (refresh mode)

	// Repo list — managed at runtime via the web UI; starts empty on first boot.
	Repos []RepoConfig

	// Web UI
	WebPort string

	// Authentication
	AdminUsername string
	AdminPassword string
	AuthDisabled  bool // AUTH_DISABLED=true skips login entirely

	// Logging
	LogLevel string // DEBUG, INFO, WARN, ERROR
}

// Load reads configuration from environment variables and validates that all
// required values are present.
func Load() (*Config, error) {
	endpoint := os.Getenv("OMNI_ENDPOINT")
	if endpoint == "" {
		return nil, fmt.Errorf("OMNI_ENDPOINT is required")
	}

	saKey := os.Getenv("OMNI_SERVICE_ACCOUNT_KEY")
	if saKey == "" {
		return nil, fmt.Errorf("OMNI_SERVICE_ACCOUNT_KEY is required")
	}

	refreshSec, _ := strconv.Atoi(getEnv("REFRESH_INTERVAL", "300"))

	authDisabled, _ := strconv.ParseBool(os.Getenv("AUTH_DISABLED"))
	adminPassword := os.Getenv("ADMIN_PASSWORD")
	if adminPassword == "" && !authDisabled {
		return nil, fmt.Errorf("ADMIN_PASSWORD is required (or set AUTH_DISABLED=true)")
	}

	// Repos starts empty; repos are managed at runtime via the web UI.
	return &Config{
		OmniEndpoint:          endpoint,
		OmniServiceAccountKey: saKey,
		RefreshInterval:       time.Duration(refreshSec) * time.Second,
		Repos:                 nil,
		WebPort:               getEnv("WEB_PORT", "8080"),
		AdminUsername:         getEnv("ADMIN_USERNAME", "admin"),
		AdminPassword:         adminPassword,
		AuthDisabled:          authDisabled,
		LogLevel:              getEnv("LOG_LEVEL", "INFO"),
	}, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
