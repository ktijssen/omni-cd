package config

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"go.yaml.in/yaml/v4"
)

// RepoConfig holds settings for a single git repository.
type RepoConfig struct {
	Name         string `yaml:"name"`
	URL          string `yaml:"url"`
	Branch       string `yaml:"branch"`
	ClustersPath string `yaml:"clusters_path"`
	MCPath       string `yaml:"mc_path"`
	Token        string `yaml:"token,omitempty"`
	// FromConfig is true when this repo originated from the config file.
	// UI mutations (delete/update) are rejected for these. Recomputed at
	// startup based on the live config; never persisted.
	FromConfig bool `yaml:"-" json:"-"`
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
	// OmniEnvLocked is true when both Omni credentials are supplied via
	// external configuration (environment variables or YAML config file).
	// When true, the UI config form is disabled and API writes to
	// /api/omni-instance return 403.
	OmniEnvLocked bool

	// Sync behaviour
	RefreshInterval time.Duration // How often to check for new git commits (refresh mode)

	// Repo list — managed at runtime via the web UI; YAML config can seed
	// the initial list, starts empty otherwise.
	Repos []RepoConfig

	// Web UI
	WebPort       string
	MetricsPort   string // port for the /metrics endpoint (Prometheus)
	WebhookSecret string // optional HMAC secret for POST /api/webhook

	// Authentication
	AdminPassword string
	AuthDisabled  bool // AUTH_DISABLED=true skips login entirely

	// OIDC — nil when not configured.
	OIDC *OIDCConfig

	// Logging
	LogLevel           string // DEBUG, INFO, WARN, ERROR
	LogRetentionDays   int    // How many days of log files to keep (LOG_RETENTION_DAYS, default 7)
	AuditRetentionDays int    // How many days of audit files to keep (AUDIT_RETENTION_DAYS, default 30)
}

// fileConfig is the YAML representation of Config. Loaded when a config path
// is supplied; env vars override matching file values.
type fileConfig struct {
	Omni struct {
		Endpoint          string `yaml:"endpoint"`
		ServiceAccountKey string `yaml:"serviceAccountKey"`
	} `yaml:"omni"`

	RefreshInterval int `yaml:"refreshInterval"` // seconds

	WebPort       string `yaml:"webPort"`
	MetricsPort   string `yaml:"metricsPort"`
	WebhookSecret string `yaml:"webhookSecret"`

	AdminPassword string `yaml:"adminPassword"`
	AuthDisabled  bool   `yaml:"authDisabled"`

	LogLevel           string `yaml:"logLevel"`
	LogRetentionDays   int    `yaml:"logRetentionDays"`
	AuditRetentionDays int    `yaml:"auditRetentionDays"`

	OIDC struct {
		Enabled      bool     `yaml:"enabled"`
		IssuerURL    string   `yaml:"issuerUrl"`
		ClientID     string   `yaml:"clientId"`
		ClientSecret string   `yaml:"clientSecret"`
		RedirectURL  string   `yaml:"redirectUrl"`
		Scopes       []string `yaml:"scopes"`
		GroupsClaim  string   `yaml:"groupsClaim"`
		AdminGroups  []string `yaml:"adminGroups"`
		AdminEmails  []string `yaml:"adminEmails"`
		ViewerGroups []string `yaml:"viewerGroups"`
		ViewerEmails []string `yaml:"viewerEmails"`
		DefaultRole  string   `yaml:"defaultRole"`
		Insecure     bool     `yaml:"insecure"`
	} `yaml:"oidc"`

	Repos []RepoConfig `yaml:"repos"`
}

// Load reads configuration from zero or more YAML files and environment
// variables. Precedence is env vars > later file > earlier file > defaults.
// Layering works because yaml.Unmarshal only touches fields present in each
// document — so a partial file (e.g. just `omni:`) only overrides that subtree
// while leaving the rest intact. Lists (repos, OIDC groups, …) are replaced
// when present in a layer; layers that omit a list leave the prior value.
func Load(configPaths []string) (*Config, error) {
	var file fileConfig
	for _, p := range configPaths {
		if err := loadFileInto(&file, p); err != nil {
			return nil, err
		}
	}

	endpoint := stringValue("OMNI_ENDPOINT", file.Omni.Endpoint, "")
	saKey := stringValue("OMNI_SERVICE_ACCOUNT_KEY", file.Omni.ServiceAccountKey, "")
	envLocked := endpoint != "" && saKey != ""

	refreshSec := intValue("REFRESH_INTERVAL", file.RefreshInterval, 300)
	retentionDays := intValue("LOG_RETENTION_DAYS", file.LogRetentionDays, 7)
	auditRetentionDays := intValue("AUDIT_RETENTION_DAYS", file.AuditRetentionDays, 30)

	authDisabled := boolValue("AUTH_DISABLED", file.AuthDisabled)
	adminPassword := stringValue("ADMIN_PASSWORD", file.AdminPassword, "")

	oidcEnabled := boolValue("OIDC_ENABLED", file.OIDC.Enabled)

	var oidcCfg *OIDCConfig
	if oidcEnabled {
		issuer := stringValue("OIDC_ISSUER_URL", file.OIDC.IssuerURL, "")
		clientID := stringValue("OIDC_CLIENT_ID", file.OIDC.ClientID, "")
		switch {
		case issuer == "":
			slog.Warn("OIDC enabled but issuer URL is not set — OIDC disabled")
		case clientID == "":
			slog.Warn("OIDC enabled but client ID is not set — OIDC disabled")
		default:
			oidcCfg = &OIDCConfig{
				IssuerURL:    issuer,
				ClientID:     clientID,
				ClientSecret: stringValue("OIDC_CLIENT_SECRET", file.OIDC.ClientSecret, ""),
				RedirectURL:  stringValue("OIDC_REDIRECT_URL", file.OIDC.RedirectURL, ""),
				Scopes:       stringSliceValue("OIDC_SCOPES", file.OIDC.Scopes),
				GroupsClaim:  stringValue("OIDC_GROUPS_CLAIM", file.OIDC.GroupsClaim, ""),
				AdminGroups:  stringSliceValue("OIDC_ADMIN_GROUPS", file.OIDC.AdminGroups),
				AdminEmails:  stringSliceValue("OIDC_ADMIN_EMAILS", file.OIDC.AdminEmails),
				ViewerGroups: stringSliceValue("OIDC_VIEWER_GROUPS", file.OIDC.ViewerGroups),
				ViewerEmails: stringSliceValue("OIDC_VIEWER_EMAILS", file.OIDC.ViewerEmails),
				DefaultRole:  stringValue("OIDC_DEFAULT_ROLE", file.OIDC.DefaultRole, ""),
				Insecure:     boolValue("OIDC_INSECURE", file.OIDC.Insecure),
			}
		}
	}

	return &Config{
		OmniEndpoint:          endpoint,
		OmniServiceAccountKey: saKey,
		OmniEnvLocked:         envLocked,
		RefreshInterval:       time.Duration(refreshSec) * time.Second,
		Repos:                 file.Repos,
		WebPort:               stringValue("WEB_PORT", file.WebPort, "8080"),
		MetricsPort:           stringValue("METRICS_PORT", file.MetricsPort, "9090"),
		WebhookSecret:         stringValue("WEBHOOK_SECRET", file.WebhookSecret, ""),
		AdminPassword:         adminPassword,
		AuthDisabled:          authDisabled,
		OIDC:                  oidcCfg,
		LogLevel:              stringValue("LOG_LEVEL", file.LogLevel, "INFO"),
		LogRetentionDays:      retentionDays,
		AuditRetentionDays:    auditRetentionDays,
	}, nil
}

// loadFileInto reads a YAML file and merges it into the given fileConfig.
// Empty paths are ignored. Scalar and struct fields present in the YAML
// overwrite the corresponding fields in cfg; fields absent from the YAML are
// left untouched. Lists present in the YAML replace any prior value.
func loadFileInto(cfg *fileConfig, path string) error {
	if path == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read config file %q: %w", path, err)
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return fmt.Errorf("parse config file %q: %w", path, err)
	}
	return nil
}

// stringValue returns the env var if set and non-empty, then the file value if
// non-empty, then the fallback.
func stringValue(envKey, fileValue, fallback string) string {
	if v := os.Getenv(envKey); v != "" {
		return v
	}
	if fileValue != "" {
		return fileValue
	}
	return fallback
}

// intValue returns the env var if set and parseable, then the file value if
// non-zero, then the fallback. Treats a YAML zero as "unset" so explicit
// fallback defaults still apply.
func intValue(envKey string, fileValue, fallback int) int {
	if v := os.Getenv(envKey); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	if fileValue != 0 {
		return fileValue
	}
	return fallback
}

// boolValue returns the parsed env var if set and non-empty, else the file
// value. There is no fallback parameter — bool defaults are always false, which
// is also the zero value of the file field, so the file value alone covers both.
func boolValue(envKey string, fileValue bool) bool {
	if v, ok := os.LookupEnv(envKey); ok && v != "" {
		b, _ := strconv.ParseBool(v)
		return b
	}
	return fileValue
}

// stringSliceValue returns the env var split on commas if set, else the file value.
func stringSliceValue(envKey string, fileValue []string) []string {
	if v := os.Getenv(envKey); v != "" {
		return splitCSV(v)
	}
	return fileValue
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
