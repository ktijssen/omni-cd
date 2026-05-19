package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestLoad_Defaults verifies that omitted env vars produce expected defaults.
func TestLoad_Defaults(t *testing.T) {
	clearAllEnv(t)

	cfg, err := Load(nil)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.RefreshInterval != 300*time.Second {
		t.Errorf("RefreshInterval: got %v, want 300s", cfg.RefreshInterval)
	}
	if cfg.WebPort != "8080" {
		t.Errorf("WebPort: got %q, want %q", cfg.WebPort, "8080")
	}
	if cfg.MetricsPort != "9090" {
		t.Errorf("MetricsPort: got %q, want %q", cfg.MetricsPort, "9090")
	}
	if cfg.LogRetentionDays != 7 {
		t.Errorf("LogRetentionDays: got %d, want 7", cfg.LogRetentionDays)
	}
	if cfg.AuditRetentionDays != 30 {
		t.Errorf("AuditRetentionDays: got %d, want 30", cfg.AuditRetentionDays)
	}
	if cfg.OIDC != nil {
		t.Error("OIDC should be nil when not configured")
	}
	if cfg.OmniEnvLocked {
		t.Error("OmniEnvLocked should be false when no credentials in env")
	}
}

func TestLoad_OmniEnvLocked(t *testing.T) {
	clearAllEnv(t)
	t.Setenv("OMNI_ENDPOINT", "https://omni.example.com")
	t.Setenv("OMNI_SERVICE_ACCOUNT_KEY", "my-key")

	cfg, err := Load(nil)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if !cfg.OmniEnvLocked {
		t.Error("OmniEnvLocked should be true when both credentials are in env")
	}
	if cfg.OmniEndpoint != "https://omni.example.com" {
		t.Errorf("OmniEndpoint: got %q", cfg.OmniEndpoint)
	}
}

func TestLoad_OmniEnvLocked_PartialNotLocked(t *testing.T) {
	clearAllEnv(t)
	t.Setenv("OMNI_ENDPOINT", "https://omni.example.com")

	cfg, err := Load(nil)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.OmniEnvLocked {
		t.Error("OmniEnvLocked should be false when only one credential is set")
	}
}

func TestLoad_OIDC_MissingIssuer(t *testing.T) {
	clearAllEnv(t)
	t.Setenv("OIDC_ENABLED", "true")
	t.Setenv("OIDC_CLIENT_ID", "my-client")

	cfg, err := Load(nil)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.OIDC != nil {
		t.Error("OIDC should be nil when OIDC_ISSUER_URL is missing")
	}
}

func TestLoad_OIDC_MissingClientID(t *testing.T) {
	clearAllEnv(t)
	t.Setenv("OIDC_ENABLED", "true")
	t.Setenv("OIDC_ISSUER_URL", "https://issuer.example.com")

	cfg, err := Load(nil)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.OIDC != nil {
		t.Error("OIDC should be nil when OIDC_CLIENT_ID is missing")
	}
}

func TestLoad_OIDC_Full(t *testing.T) {
	clearAllEnv(t)
	t.Setenv("OIDC_ENABLED", "true")
	t.Setenv("OIDC_ISSUER_URL", "https://issuer.example.com")
	t.Setenv("OIDC_CLIENT_ID", "my-client")
	t.Setenv("OIDC_CLIENT_SECRET", "my-secret")
	t.Setenv("OIDC_ADMIN_GROUPS", "admins,ops")
	t.Setenv("OIDC_VIEWER_EMAILS", "viewer@example.com")
	t.Setenv("OIDC_DEFAULT_ROLE", "viewer")

	cfg, err := Load(nil)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.OIDC == nil {
		t.Fatal("OIDC should not be nil when fully configured")
	}
	if cfg.OIDC.IssuerURL != "https://issuer.example.com" {
		t.Errorf("IssuerURL: got %q", cfg.OIDC.IssuerURL)
	}
	if cfg.OIDC.ClientID != "my-client" {
		t.Errorf("ClientID: got %q", cfg.OIDC.ClientID)
	}
	if cfg.OIDC.ClientSecret != "my-secret" {
		t.Errorf("ClientSecret: got %q", cfg.OIDC.ClientSecret)
	}
	if len(cfg.OIDC.AdminGroups) != 2 || cfg.OIDC.AdminGroups[0] != "admins" || cfg.OIDC.AdminGroups[1] != "ops" {
		t.Errorf("AdminGroups: got %v", cfg.OIDC.AdminGroups)
	}
	if len(cfg.OIDC.ViewerEmails) != 1 || cfg.OIDC.ViewerEmails[0] != "viewer@example.com" {
		t.Errorf("ViewerEmails: got %v", cfg.OIDC.ViewerEmails)
	}
	if cfg.OIDC.DefaultRole != "viewer" {
		t.Errorf("DefaultRole: got %q", cfg.OIDC.DefaultRole)
	}
}

func TestLoad_OIDC_Scopes_CSV(t *testing.T) {
	clearAllEnv(t)
	t.Setenv("OIDC_ENABLED", "true")
	t.Setenv("OIDC_ISSUER_URL", "https://issuer.example.com")
	t.Setenv("OIDC_CLIENT_ID", "client")
	t.Setenv("OIDC_SCOPES", "openid,email,profile")

	cfg, _ := Load(nil)
	if cfg.OIDC == nil {
		t.Fatal("OIDC should not be nil")
	}
	if len(cfg.OIDC.Scopes) != 3 {
		t.Errorf("Scopes: expected 3, got %v", cfg.OIDC.Scopes)
	}
	if cfg.OIDC.Scopes[0] != "openid" || cfg.OIDC.Scopes[1] != "email" || cfg.OIDC.Scopes[2] != "profile" {
		t.Errorf("Scopes values: got %v", cfg.OIDC.Scopes)
	}
}

func TestSplitCSV(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"", nil},
		{"single", []string{"single"}},
		{"a,b,c", []string{"a", "b", "c"}},
		{" a , b , c ", []string{"a", "b", "c"}},
		{"a,,b", []string{"a", "b"}}, // empty segment skipped
		{",leading", []string{"leading"}},
		{"trailing,", []string{"trailing"}},
	}
	for _, tt := range tests {
		got := splitCSV(tt.input)
		if len(got) != len(tt.want) {
			t.Errorf("splitCSV(%q): got %v, want %v", tt.input, got, tt.want)
			continue
		}
		for i := range tt.want {
			if got[i] != tt.want[i] {
				t.Errorf("splitCSV(%q)[%d]: got %q, want %q", tt.input, i, got[i], tt.want[i])
			}
		}
	}
}

func TestGetEnv_Fallback(t *testing.T) {
	const key = "TEST_GETENV_FALLBACK_XYZ"
	t.Setenv(key, "")
	if got := getEnv(key, "default"); got != "default" {
		t.Errorf("missing key should return fallback, got %q", got)
	}
	t.Setenv(key, "set-value")
	if got := getEnv(key, "default"); got != "set-value" {
		t.Errorf("set key should return its value, got %q", got)
	}
}

// TestLoad_File_Basic verifies that a YAML file populates Config when env is empty.
func TestLoad_File_Basic(t *testing.T) {
	clearAllEnv(t)
	path := writeConfigFile(t, `
omni:
  endpoint: https://omni.from-file.example.com
  serviceAccountKey: file-key
refreshInterval: 60
webPort: "9000"
metricsPort: "9100"
webhookSecret: file-secret
adminPassword: file-admin
authDisabled: true
logLevel: DEBUG
logRetentionDays: 14
auditRetentionDays: 60
`)

	cfg, err := Load([]string{path})
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.OmniEndpoint != "https://omni.from-file.example.com" {
		t.Errorf("OmniEndpoint: got %q", cfg.OmniEndpoint)
	}
	if cfg.OmniServiceAccountKey != "file-key" {
		t.Errorf("OmniServiceAccountKey: got %q", cfg.OmniServiceAccountKey)
	}
	if !cfg.OmniEnvLocked {
		t.Error("OmniEnvLocked should be true when both Omni creds come from file")
	}
	if cfg.RefreshInterval != 60*time.Second {
		t.Errorf("RefreshInterval: got %v", cfg.RefreshInterval)
	}
	if cfg.WebPort != "9000" {
		t.Errorf("WebPort: got %q", cfg.WebPort)
	}
	if cfg.MetricsPort != "9100" {
		t.Errorf("MetricsPort: got %q", cfg.MetricsPort)
	}
	if cfg.WebhookSecret != "file-secret" {
		t.Errorf("WebhookSecret: got %q", cfg.WebhookSecret)
	}
	if cfg.AdminPassword != "file-admin" {
		t.Errorf("AdminPassword: got %q", cfg.AdminPassword)
	}
	if !cfg.AuthDisabled {
		t.Error("AuthDisabled: should be true from file")
	}
	if cfg.LogLevel != "DEBUG" {
		t.Errorf("LogLevel: got %q", cfg.LogLevel)
	}
	if cfg.LogRetentionDays != 14 {
		t.Errorf("LogRetentionDays: got %d", cfg.LogRetentionDays)
	}
	if cfg.AuditRetentionDays != 60 {
		t.Errorf("AuditRetentionDays: got %d", cfg.AuditRetentionDays)
	}
}

// TestLoad_File_EnvOverrides verifies env vars take precedence over file values.
func TestLoad_File_EnvOverrides(t *testing.T) {
	clearAllEnv(t)
	t.Setenv("OMNI_ENDPOINT", "https://env-wins.example.com")
	t.Setenv("WEB_PORT", "7777")
	t.Setenv("LOG_RETENTION_DAYS", "21")
	t.Setenv("AUTH_DISABLED", "false")

	path := writeConfigFile(t, `
omni:
  endpoint: https://file-loses.example.com
webPort: "9000"
logRetentionDays: 14
authDisabled: true
`)

	cfg, err := Load([]string{path})
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.OmniEndpoint != "https://env-wins.example.com" {
		t.Errorf("OmniEndpoint: env should win, got %q", cfg.OmniEndpoint)
	}
	if cfg.WebPort != "7777" {
		t.Errorf("WebPort: env should win, got %q", cfg.WebPort)
	}
	if cfg.LogRetentionDays != 21 {
		t.Errorf("LogRetentionDays: env should win, got %d", cfg.LogRetentionDays)
	}
	if cfg.AuthDisabled {
		t.Error("AuthDisabled: env AUTH_DISABLED=false should override file authDisabled=true")
	}
}

// TestLoad_File_OIDC verifies OIDC settings load from file and env overrides apply.
func TestLoad_File_OIDC(t *testing.T) {
	clearAllEnv(t)
	path := writeConfigFile(t, `
oidc:
  enabled: true
  issuerUrl: https://issuer.from-file.example.com
  clientId: file-client
  clientSecret: file-secret
  scopes: [openid, email, groups]
  adminGroups: [admins, sre]
  viewerEmails: [viewer@example.com]
  defaultRole: viewer
  insecure: true
`)

	cfg, err := Load([]string{path})
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.OIDC == nil {
		t.Fatal("OIDC should be configured from file")
	}
	if cfg.OIDC.IssuerURL != "https://issuer.from-file.example.com" {
		t.Errorf("IssuerURL: got %q", cfg.OIDC.IssuerURL)
	}
	if cfg.OIDC.ClientID != "file-client" {
		t.Errorf("ClientID: got %q", cfg.OIDC.ClientID)
	}
	if cfg.OIDC.ClientSecret != "file-secret" {
		t.Errorf("ClientSecret: got %q", cfg.OIDC.ClientSecret)
	}
	if len(cfg.OIDC.Scopes) != 3 || cfg.OIDC.Scopes[2] != "groups" {
		t.Errorf("Scopes: got %v", cfg.OIDC.Scopes)
	}
	if len(cfg.OIDC.AdminGroups) != 2 || cfg.OIDC.AdminGroups[1] != "sre" {
		t.Errorf("AdminGroups: got %v", cfg.OIDC.AdminGroups)
	}
	if !cfg.OIDC.Insecure {
		t.Error("Insecure should be true from file")
	}

	// Now override one OIDC value via env and re-load.
	t.Setenv("OIDC_CLIENT_SECRET", "env-secret")
	cfg, err = Load([]string{path})
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.OIDC.ClientSecret != "env-secret" {
		t.Errorf("OIDC ClientSecret: env should override file, got %q", cfg.OIDC.ClientSecret)
	}
}

// TestLoad_File_Repos verifies the YAML repos block seeds Config.Repos.
func TestLoad_File_Repos(t *testing.T) {
	clearAllEnv(t)
	path := writeConfigFile(t, `
repos:
  - name: prod
    url: https://github.com/example/prod.git
    branch: main
    clusters_path: clusters
    mc_path: machineclasses
  - name: staging
    url: https://github.com/example/staging.git
    branch: develop
    clusters_path: env/staging/clusters
    mc_path: env/staging/mc
`)

	cfg, err := Load([]string{path})
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if len(cfg.Repos) != 2 {
		t.Fatalf("Repos: expected 2, got %d", len(cfg.Repos))
	}
	if cfg.Repos[0].Name != "prod" || cfg.Repos[0].Branch != "main" {
		t.Errorf("Repos[0]: got %+v", cfg.Repos[0])
	}
	if cfg.Repos[1].MCPath != "env/staging/mc" {
		t.Errorf("Repos[1].MCPath: got %q", cfg.Repos[1].MCPath)
	}
}

// TestLoad_File_Token verifies that a token in YAML is read into RepoConfig.
// The config file is expected to be supplied via a Kubernetes Secret (or
// equivalent), so storing tokens in it is the supported pattern.
func TestLoad_File_Token(t *testing.T) {
	clearAllEnv(t)
	path := writeConfigFile(t, `
repos:
  - name: r
    url: https://github.com/example/r.git
    branch: main
    clusters_path: c
    mc_path: m
    token: ghp_secret
`)

	cfg, err := Load([]string{path})
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if len(cfg.Repos) != 1 {
		t.Fatalf("Repos: expected 1, got %d", len(cfg.Repos))
	}
	if cfg.Repos[0].Token != "ghp_secret" {
		t.Errorf("Token: got %q, want %q", cfg.Repos[0].Token, "ghp_secret")
	}
}

// TestLoad_File_Layering verifies that multiple paths layer correctly:
// later files override earlier ones; absent fields are preserved; list fields
// are replaced when present in a layer. This exercises the external-secret
// use case where one source provides the bulk of the config and another
// provides just the sensitive subtree (e.g. omni credentials).
func TestLoad_File_Layering(t *testing.T) {
	clearAllEnv(t)

	base := writeConfigFile(t, `
omni:
  endpoint: https://omni.base.example.com
  serviceAccountKey: base-key
refreshInterval: 60
logLevel: INFO
oidc:
  enabled: true
  issuerUrl: https://issuer.base.example.com
  clientId: base-client
  clientSecret: base-secret
  scopes: [openid, email]
repos:
  - name: base-repo
    url: https://github.com/example/base.git
    branch: main
    clusters_path: c
    mc_path: m
`)
	overlay := writeConfigFile(t, `
omni:
  endpoint: https://omni.overlay.example.com
  serviceAccountKey: overlay-key
oidc:
  clientSecret: overlay-secret
`)

	cfg, err := Load([]string{base, overlay})
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Overlay overrode the omni block.
	if cfg.OmniEndpoint != "https://omni.overlay.example.com" {
		t.Errorf("OmniEndpoint: got %q, want overlay value", cfg.OmniEndpoint)
	}
	if cfg.OmniServiceAccountKey != "overlay-key" {
		t.Errorf("OmniServiceAccountKey: got %q, want overlay value", cfg.OmniServiceAccountKey)
	}
	// Base fields not touched by overlay remained.
	if cfg.RefreshInterval != 60*time.Second {
		t.Errorf("RefreshInterval: got %v, base should be preserved", cfg.RefreshInterval)
	}
	if cfg.LogLevel != "INFO" {
		t.Errorf("LogLevel: got %q, base should be preserved", cfg.LogLevel)
	}
	// OIDC block: clientSecret overridden, others from base.
	if cfg.OIDC == nil {
		t.Fatal("OIDC should be configured")
	}
	if cfg.OIDC.IssuerURL != "https://issuer.base.example.com" {
		t.Errorf("OIDC IssuerURL: got %q, base should be preserved", cfg.OIDC.IssuerURL)
	}
	if cfg.OIDC.ClientID != "base-client" {
		t.Errorf("OIDC ClientID: got %q, base should be preserved", cfg.OIDC.ClientID)
	}
	if cfg.OIDC.ClientSecret != "overlay-secret" {
		t.Errorf("OIDC ClientSecret: got %q, want overlay value", cfg.OIDC.ClientSecret)
	}
	if len(cfg.OIDC.Scopes) != 2 || cfg.OIDC.Scopes[0] != "openid" {
		t.Errorf("OIDC Scopes: got %v, base list should be preserved", cfg.OIDC.Scopes)
	}
	// Repos list from base is preserved since the overlay didn't set it.
	if len(cfg.Repos) != 1 || cfg.Repos[0].Name != "base-repo" {
		t.Errorf("Repos: got %v, base list should be preserved", cfg.Repos)
	}
}

// TestLoad_File_Layering_ListReplaced verifies that a list specified in a
// later layer replaces the earlier value (it does not append).
func TestLoad_File_Layering_ListReplaced(t *testing.T) {
	clearAllEnv(t)
	base := writeConfigFile(t, `
repos:
  - name: a
    url: https://github.com/example/a.git
    branch: main
    clusters_path: c
    mc_path: m
  - name: b
    url: https://github.com/example/b.git
    branch: main
    clusters_path: c
    mc_path: m
`)
	overlay := writeConfigFile(t, `
repos:
  - name: c
    url: https://github.com/example/c.git
    branch: main
    clusters_path: c
    mc_path: m
`)

	cfg, err := Load([]string{base, overlay})
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if len(cfg.Repos) != 1 || cfg.Repos[0].Name != "c" {
		t.Errorf("Repos should be replaced by overlay, got %v", cfg.Repos)
	}
}

// TestLoad_File_Missing returns an error when the path is set but the file is absent.
func TestLoad_File_Missing(t *testing.T) {
	clearAllEnv(t)
	_, err := Load([]string{"/nonexistent/path/to/config.yaml"})
	if err == nil {
		t.Error("expected error when config file does not exist, got nil")
	}
}

// TestLoad_File_Malformed returns an error when the file is not valid YAML.
func TestLoad_File_Malformed(t *testing.T) {
	clearAllEnv(t)
	path := writeConfigFile(t, "this is: not: valid: yaml: [")
	if _, err := Load([]string{path}); err == nil {
		t.Error("expected error when config file is malformed, got nil")
	}
}

// writeConfigFile creates a temp config file and returns its path.
func writeConfigFile(t *testing.T, contents string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("writeConfigFile: %v", err)
	}
	return path
}

// clearAllEnv resets every env var Load reads so tests start from a clean slate.
func clearAllEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"OMNI_ENDPOINT", "OMNI_SERVICE_ACCOUNT_KEY",
		"REFRESH_INTERVAL", "WEB_PORT", "METRICS_PORT", "WEBHOOK_SECRET",
		"ADMIN_PASSWORD", "AUTH_DISABLED",
		"LOG_LEVEL", "LOG_RETENTION_DAYS", "AUDIT_RETENTION_DAYS",
		"OIDC_ENABLED", "OIDC_ISSUER_URL", "OIDC_CLIENT_ID", "OIDC_CLIENT_SECRET",
		"OIDC_REDIRECT_URL", "OIDC_SCOPES", "OIDC_GROUPS_CLAIM",
		"OIDC_ADMIN_GROUPS", "OIDC_ADMIN_EMAILS",
		"OIDC_VIEWER_GROUPS", "OIDC_VIEWER_EMAILS",
		"OIDC_DEFAULT_ROLE", "OIDC_INSECURE",
	} {
		t.Setenv(key, "")
	}
}
