package omniinstance

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// InstanceConfig holds the Omni instance connection settings persisted via the UI.
type InstanceConfig struct {
	Endpoint          string `json:"endpoint"`
	ServiceAccountKey string `json:"serviceAccountKey"`
}

// DefaultPath is the default location for the persisted instance config.
const DefaultPath = "/data/config/instances.json"

// Load reads instance config from path. Returns nil, nil if the file does not exist.
func Load(path string) (*InstanceConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var cfg InstanceConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// Save writes instance config to path with 0600 permissions.
func Save(path string, cfg InstanceConfig) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}
