package config

import (
	"encoding/json"
	"os"

	"orbit/internal/model"
)

// Load reads the config.json file and returns a Config struct
func Load(path string) (*model.Config, error) {
	configFile, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var config model.Config
	if err := json.Unmarshal(configFile, &config); err != nil {
		return nil, err
	}
	return &config, nil
}

// Save writes the Config struct back to the file
func Save(path string, cfg *model.Config) error {
	data, err := json.MarshalIndent(cfg, "", "    ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
