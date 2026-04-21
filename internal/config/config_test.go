package config

import (
	"testing"
	"time"
)

// TestLoad_Defaults verifies that omitted env vars produce expected defaults.
func TestLoad_Defaults(t *testing.T) {
	clearOIDCEnv(t)
	t.Setenv("OMNI_ENDPOINT", "")
	t.Setenv("OMNI_SERVICE_ACCOUNT_KEY", "")
	t.Setenv("REFRESH_INTERVAL", "")
	t.Setenv("WEB_PORT", "")
	t.Setenv("METRICS_PORT", "")
	t.Setenv("LOG_RETENTION_DAYS", "")
	t.Setenv("AUDIT_RETENTION_DAYS", "")

	cfg, err := Load()
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
	clearOIDCEnv(t)
	t.Setenv("OMNI_ENDPOINT", "https://omni.example.com")
	t.Setenv("OMNI_SERVICE_ACCOUNT_KEY", "my-key")

	cfg, err := Load()
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
	clearOIDCEnv(t)
	t.Setenv("OMNI_ENDPOINT", "https://omni.example.com")
	t.Setenv("OMNI_SERVICE_ACCOUNT_KEY", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.OmniEnvLocked {
		t.Error("OmniEnvLocked should be false when only one credential is set")
	}
}

func TestLoad_OIDC_MissingIssuer(t *testing.T) {
	clearOIDCEnv(t)
	t.Setenv("OIDC_ENABLED", "true")
	t.Setenv("OIDC_ISSUER_URL", "")
	t.Setenv("OIDC_CLIENT_ID", "my-client")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.OIDC != nil {
		t.Error("OIDC should be nil when OIDC_ISSUER_URL is missing")
	}
}

func TestLoad_OIDC_MissingClientID(t *testing.T) {
	clearOIDCEnv(t)
	t.Setenv("OIDC_ENABLED", "true")
	t.Setenv("OIDC_ISSUER_URL", "https://issuer.example.com")
	t.Setenv("OIDC_CLIENT_ID", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.OIDC != nil {
		t.Error("OIDC should be nil when OIDC_CLIENT_ID is missing")
	}
}

func TestLoad_OIDC_Full(t *testing.T) {
	clearOIDCEnv(t)
	t.Setenv("OIDC_ENABLED", "true")
	t.Setenv("OIDC_ISSUER_URL", "https://issuer.example.com")
	t.Setenv("OIDC_CLIENT_ID", "my-client")
	t.Setenv("OIDC_CLIENT_SECRET", "my-secret")
	t.Setenv("OIDC_ADMIN_GROUPS", "admins,ops")
	t.Setenv("OIDC_VIEWER_EMAILS", "viewer@example.com")
	t.Setenv("OIDC_DEFAULT_ROLE", "viewer")

	cfg, err := Load()
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
	clearOIDCEnv(t)
	t.Setenv("OIDC_ENABLED", "true")
	t.Setenv("OIDC_ISSUER_URL", "https://issuer.example.com")
	t.Setenv("OIDC_CLIENT_ID", "client")
	t.Setenv("OIDC_SCOPES", "openid,email,profile")

	cfg, _ := Load()
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

// clearOIDCEnv resets all OIDC-related env vars so tests start from a clean state.
func clearOIDCEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"OIDC_ENABLED", "OIDC_ISSUER_URL", "OIDC_CLIENT_ID", "OIDC_CLIENT_SECRET",
		"OIDC_REDIRECT_URL", "OIDC_SCOPES", "OIDC_GROUPS_CLAIM",
		"OIDC_ADMIN_GROUPS", "OIDC_ADMIN_EMAILS",
		"OIDC_VIEWER_GROUPS", "OIDC_VIEWER_EMAILS",
		"OIDC_DEFAULT_ROLE", "OIDC_INSECURE",
	} {
		t.Setenv(key, "")
	}
}
