package config

import (
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
	// OmniEnvLocked is true when both Omni credentials are supplied via ENV
	// variables. When true, the UI config form is disabled and API writes
	// to /api/omni-instance return 403.
	OmniEnvLocked bool

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
	LogLevel         string // DEBUG, INFO, WARN, ERROR
	LogRetentionDays int    // How many days of log files to keep
}

// Load reads configuration from environment variables. Missing Omni credentials
// are not an error — the caller decides whether to load them from a file or
// wait for UI configuration.
func Load() (*Config, error) {
	endpoint := os.Getenv("OMNI_ENDPOINT")
	saKey := os.Getenv("OMNI_SERVICE_ACCOUNT_KEY")
	envLocked := endpoint != "" && saKey != ""

	refreshSec, _ := strconv.Atoi(getEnv("REFRESH_INTERVAL", "300"))
	retentionDays, _ := strconv.Atoi(getEnv("LOG_RETENTION_DAYS", "7"))

	authDisabled, _ := strconv.ParseBool(os.Getenv("AUTH_DISABLED"))
	adminPassword := os.Getenv("ADMIN_PASSWORD")

	// Repos starts empty; repos are managed at runtime via the web UI.
	return &Config{
		OmniEndpoint:          endpoint,
		OmniServiceAccountKey: saKey,
		OmniEnvLocked:         envLocked,
		RefreshInterval:       time.Duration(refreshSec) * time.Second,
		Repos:                 nil,
		WebPort:               getEnv("WEB_PORT", "8080"),
		AdminUsername:         getEnv("ADMIN_USERNAME", "admin"),
		AdminPassword:         adminPassword,
		AuthDisabled:          authDisabled,
		LogLevel:              getEnv("LOG_LEVEL", "INFO"),
		LogRetentionDays:      retentionDays,
	}, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
