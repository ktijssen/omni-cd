package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds all configuration for omni-cd.
type Config struct {
	// Omni connection settings
	OmniEndpoint          string
	OmniServiceAccountKey string

	// Git repository settings
	GitRepo   string
	GitBranch string
	GitToken  string

	// Sync behaviour
	RefreshInterval time.Duration // How often to check for new git commits (refresh mode)
	SyncInterval    time.Duration // How often to force a full reconcile (sync mode)

	// Resource paths within the Git repo
	MCPath       string
	ClustersPath string

	// Feature toggles
	ClustersEnabled bool

	// Web UI
	WebPort string

	// Logging
	LogLevel string // DEBUG, INFO, WARN, ERROR
}

// Load reads configuration from environment variables and validates
// that all required values are present.
func Load() (*Config, error) {
	endpoint := os.Getenv("OMNI_ENDPOINT")
	if endpoint == "" {
		return nil, fmt.Errorf("OMNI_ENDPOINT is required")
	}

	saKey := os.Getenv("OMNI_SERVICE_ACCOUNT_KEY")
	if saKey == "" {
		return nil, fmt.Errorf("OMNI_SERVICE_ACCOUNT_KEY is required")
	}

	gitRepo := os.Getenv("GIT_REPO")
	if gitRepo == "" {
		return nil, fmt.Errorf("GIT_REPO is required")
	}

	refreshSec, _ := strconv.Atoi(getEnv("REFRESH_INTERVAL", "300"))
	syncSec, _ := strconv.Atoi(getEnv("SYNC_INTERVAL", "3600"))
	clustersEnabled, _ := strconv.ParseBool(getEnv("CLUSTERS_ENABLED", "true"))

	return &Config{
		OmniEndpoint:          endpoint,
		OmniServiceAccountKey: saKey,
		GitRepo:               gitRepo,
		GitBranch:             getEnv("GIT_BRANCH", "main"),
		GitToken:              os.Getenv("GIT_TOKEN"),
		RefreshInterval:       time.Duration(refreshSec) * time.Second,
		SyncInterval:          time.Duration(syncSec) * time.Second,
		MCPath:                getEnv("MC_PATH", "machine-classes"),
		ClustersPath:          getEnv("CLUSTERS_PATH", "clusters"),
		ClustersEnabled:       clustersEnabled,
		WebPort:               getEnv("WEB_PORT", "8080"),
		LogLevel:              getEnv("LOG_LEVEL", "INFO"),
	}, nil
}

// getEnv returns the value of an environment variable or a default.
func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
