package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// Init creates a .devtool/docker.yaml from discovered profiles.
// Returns the path to the created file.
func Init(dir string, cfg *Config) (string, error) {
	path := filepath.Join(dir, configPath)

	// Don't overwrite existing config
	if _, err := os.Stat(path); err == nil {
		return "", fmt.Errorf("config already exists: %s", path)
	}

	if err := Save(dir, cfg); err != nil {
		return "", err
	}

	return path, nil
}
