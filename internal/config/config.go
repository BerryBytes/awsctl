package config

import (
	"awsctl/models"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v2"
)

// LoadConfig loads the configuration from a file (either .yaml or .json)
func LoadConfig() (*models.Config, error) {
	// Step 1: Find the config file in the current directory
	configFilePath, err := FindConfigFile()
	if err != nil {
		return nil, fmt.Errorf("config file not found: %w", err)
	}

	// Step 2: Read the configuration file
	fileData, err := os.ReadFile(configFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Try to unmarshal YAML first
	var cfg models.Config
	if err := yaml.Unmarshal(fileData, &cfg); err != nil {
		// If YAML parsing fails, try JSON
		fmt.Println("YAML parsing failed, trying JSON...")
		if err := json.Unmarshal(fileData, &cfg); err != nil {
			return nil, fmt.Errorf("failed to parse config file: %w", err)
		}
	}

	return &cfg, nil
}

// FindConfigFile looks for config files (config.yml, config.yaml, or config.json) in the ~/.config/aws directory
func FindConfigFile() (string, error) {
	extensions := []string{"config.yml", "config.yaml", "config.json"}

	// Get the path to the ~/.config/aws directory
	configDir := filepath.Join(os.Getenv("HOME"), ".config", "aws")

	// Check if the ~/.config/aws directory exists
	if _, err := os.Stat(configDir); os.IsNotExist(err) {
		return "", fmt.Errorf("directory ~/.config/aws does not exist")
	}

	// Check for the config files in the ~/.config/aws directory
	for _, ext := range extensions {
		possiblePath := filepath.Join(configDir, ext)
		if _, err := os.Stat(possiblePath); err == nil {
			return possiblePath, nil
		}
	}

	return "", fmt.Errorf("no config file found in ~/.config/aws directory")
}
