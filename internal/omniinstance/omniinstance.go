package omniinstance

// InstanceConfig holds the Omni instance connection settings persisted via the UI.
type InstanceConfig struct {
	Endpoint          string `json:"endpoint"`
	ServiceAccountKey string `json:"serviceAccountKey"`
}

// Store persists the Omni instance connection settings.
type Store interface {
	// LoadInstanceConfig returns nil, nil when nothing has been stored yet.
	LoadInstanceConfig() (*InstanceConfig, error)
	SaveInstanceConfig(cfg InstanceConfig) error
	// DeleteInstanceConfig removes the stored settings. Deleting when nothing
	// is stored is not an error.
	DeleteInstanceConfig() error
}
