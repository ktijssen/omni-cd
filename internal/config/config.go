package config

import (
	"log/slog"
	"os"
	"strconv"
	"strings"
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

// OIDCConfig holds OIDC provider settings loaded from environment variables.
// OIDC is enabled when OIDC_ISSUER_URL and OIDC_CLIENT_ID are both set.
type OIDCConfig struct {
	IssuerURL    string
	ClientID     string
	ClientSecret string
	RedirectURL  string   // optional: auto-derived from the login request when empty
	Scopes       []string // defaults to ["openid", "email", "profile"]
	GroupsClaim  string   // defaults to "groups"
	AdminGroups  []string
	AdminEmails  []string
	ViewerGroups []string
	ViewerEmails []string
	DefaultRole  string // "admin", "viewer", "none" — defaults to "viewer"
	Insecure     bool   // skip TLS verification (self-signed certs)
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
	WebPort       string
	MetricsPort   string // port for the /metrics endpoint (Prometheus)
	WebhookSecret string // optional HMAC secret for POST /api/webhook

	// Authentication
	AdminPassword string
	AuthDisabled  bool // AUTH_DISABLED=true skips login entirely

	// OIDC — nil when not configured via environment variables.
	OIDC *OIDCConfig

	// Logging
	LogLevel           string // DEBUG, INFO, WARN, ERROR
	LogRetentionDays   int    // How many days of log files to keep (LOG_RETENTION_DAYS, default 7)
	AuditRetentionDays int    // How many days of audit files to keep (AUDIT_RETENTION_DAYS, default 30)
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
	auditRetentionDays, _ := strconv.Atoi(getEnv("AUDIT_RETENTION_DAYS", "30"))

	authDisabled, _ := strconv.ParseBool(os.Getenv("AUTH_DISABLED"))
	adminPassword := os.Getenv("ADMIN_PASSWORD")

	oidcEnabled, _ := strconv.ParseBool(getEnv("OIDC_ENABLED", "false"))

	var oidcCfg *OIDCConfig
	if oidcEnabled {
		issuer := os.Getenv("OIDC_ISSUER_URL")
		clientID := os.Getenv("OIDC_CLIENT_ID")
		switch {
		case issuer == "":
			slog.Warn("OIDC_ENABLED=true but OIDC_ISSUER_URL is not set — OIDC disabled")
		case clientID == "":
			slog.Warn("OIDC_ENABLED=true but OIDC_CLIENT_ID is not set — OIDC disabled")
		default:
			insecure, _ := strconv.ParseBool(os.Getenv("OIDC_INSECURE"))
			oidcCfg = &OIDCConfig{
				IssuerURL:    issuer,
				ClientID:     clientID,
				ClientSecret: os.Getenv("OIDC_CLIENT_SECRET"),
				RedirectURL:  os.Getenv("OIDC_REDIRECT_URL"),
				Scopes:       splitCSV(os.Getenv("OIDC_SCOPES")),
				GroupsClaim:  os.Getenv("OIDC_GROUPS_CLAIM"),
				AdminGroups:  splitCSV(os.Getenv("OIDC_ADMIN_GROUPS")),
				AdminEmails:  splitCSV(os.Getenv("OIDC_ADMIN_EMAILS")),
				ViewerGroups: splitCSV(os.Getenv("OIDC_VIEWER_GROUPS")),
				ViewerEmails: splitCSV(os.Getenv("OIDC_VIEWER_EMAILS")),
				DefaultRole:  os.Getenv("OIDC_DEFAULT_ROLE"),
				Insecure:     insecure,
			}
		}
	}

	// Repos starts empty; repos are managed at runtime via the web UI.
	return &Config{
		OmniEndpoint:          endpoint,
		OmniServiceAccountKey: saKey,
		OmniEnvLocked:         envLocked,
		RefreshInterval:       time.Duration(refreshSec) * time.Second,
		Repos:                 nil,
		WebPort:               getEnv("WEB_PORT", "8080"),
		MetricsPort:           getEnv("METRICS_PORT", "9090"),
		WebhookSecret:         os.Getenv("WEBHOOK_SECRET"),
		AdminPassword:         adminPassword,
		AuthDisabled:          authDisabled,
		OIDC:                  oidcCfg,
		LogLevel:              getEnv("LOG_LEVEL", "INFO"),
		LogRetentionDays:      retentionDays,
		AuditRetentionDays:    auditRetentionDays,
	}, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}
