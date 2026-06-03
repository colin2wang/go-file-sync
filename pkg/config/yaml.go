package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// LoadYAML reads and parses a sync.yaml file.
// If the file does not exist, it returns DefaultConfig() instead of an error.
func LoadYAML(path string) (*GeneralConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultConfig(), nil
		}
		return nil, fmt.Errorf("read yaml config %s: %w", path, err)
	}

	cfg := DefaultConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse yaml config %s: %w", path, err)
	}

	return cfg, nil
}

// SaveYAML writes a GeneralConfig to a sync.yaml file.
func SaveYAML(path string, cfg *GeneralConfig) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal yaml: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write yaml %s: %w", path, err)
	}
	return nil
}
