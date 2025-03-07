package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v2"
)

// Config struct to hold AWS configuration
type Config struct {
	Aws struct {
		Profile     string `yaml:"profile" json:"profile"`
		Region      string `yaml:"region" json:"region"`
		AccountID   string `yaml:"account_id" json:"account_id"`
		Role        string `yaml:"role" json:"role"`
		SsoStartUrl string `yaml:"sso_start_url" json:"sso_start_url"`
	} `yaml:"aws" json:"aws"`
}

// LoadConfig loads the configuration from a file (either .yaml or .json)
func LoadConfig() (*Config, error) {
	// Step 1: Find the config file in the current directory
	configFilePath, err := FindConfigFile()
	if err != nil {
		return nil, fmt.Errorf("config file not found: %w", err)
	}

	// Step 2: Load the configuration from the found config file
	fileData, err := os.ReadFile(configFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Try to unmarshal YAML first
	var cfg Config
	if err := yaml.Unmarshal(fileData, &cfg); err != nil {
		// If YAML fails, try JSON
		if err := json.Unmarshal(fileData, &cfg); err != nil {
			return nil, fmt.Errorf("failed to parse config file: %w", err)
		}
	}

	return &cfg, nil
}

// FindConfigFile looks for config files (config.yml, config.yaml, or config.json) in the current directory
func FindConfigFile() (string, error) {
	// List of config file extensions
	extensions := []string{"config.yml", "config.yaml", "config.json"}

	// Get the project root directory
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	// Check for the config files in the project root directory
	for _, ext := range extensions {
		possiblePath := filepath.Join(dir, ext)
		if _, err := os.Stat(possiblePath); err == nil {
			return possiblePath, nil
		}
	}

	return "", fmt.Errorf("no config file found in the project root directory")
}
