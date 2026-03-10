package oidcconfig

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config holds OIDC provider configuration persisted via the UI.
type Config struct {
	IssuerURL    string `json:"issuerURL"`
	ClientID     string `json:"clientID"`
	ClientSecret string `json:"clientSecret"`
	RedirectURL  string `json:"redirectURL"`

	// Scopes requested from the provider. Defaults to ["openid", "email", "profile"].
	Scopes []string `json:"scopes,omitempty"`

	// GroupsClaim is the JWT/userinfo claim that contains a list of group names.
	// Defaults to "groups" if empty.
	GroupsClaim string `json:"groupsClaim,omitempty"`

	// Role mapping — checked in order: adminEmails > adminGroups > viewerEmails > viewerGroups > defaultRole.
	AdminGroups  []string `json:"adminGroups,omitempty"`
	AdminEmails  []string `json:"adminEmails,omitempty"`
	ViewerGroups []string `json:"viewerGroups,omitempty"`
	ViewerEmails []string `json:"viewerEmails,omitempty"`

	// DefaultRole is assigned when the user does not match any email or group rule.
	// Valid values: "admin", "viewer", "none". Defaults to "viewer".
	DefaultRole string `json:"defaultRole,omitempty"`

	// Insecure disables TLS certificate verification for the OIDC provider.
	// Only use this for development or internal providers with self-signed certs.
	Insecure bool `json:"insecure,omitempty"`
}

// GetScopes returns the configured scopes, falling back to a sensible default.
func (c *Config) GetScopes() []string {
	if len(c.Scopes) == 0 {
		return []string{"openid", "email", "profile"}
	}
	return c.Scopes
}

// GetGroupsClaim returns the configured groups claim name, defaulting to "groups".
func (c *Config) GetGroupsClaim() string {
	if c.GroupsClaim == "" {
		return "groups"
	}
	return c.GroupsClaim
}

// GetDefaultRole returns the default role, normalising an empty value to "viewer".
func (c *Config) GetDefaultRole() string {
	switch c.DefaultRole {
	case "admin", "viewer", "none":
		return c.DefaultRole
	default:
		return "viewer"
	}
}

// DefaultPath is the default storage location for the OIDC config file.
const DefaultPath = "/data/config/oidc.json"

// Load reads OIDC config from path. Returns nil, nil if the file does not exist.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// Save writes cfg to path with restricted (0600) permissions.
func Save(path string, cfg Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

// Delete removes the OIDC config file. A missing file is not an error.
func Delete(path string) error {
	err := os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
