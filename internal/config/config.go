package config

import (
	"awsctl/models"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v2"
)

type Config struct {
	AWSProfile      string
	AWSConfigDir    string
	RawCustomConfig *models.Config
}

func NewConfig() (*Config, error) {
	userHome, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get user home directory: %w", err)
	}

	cfg := &Config{
		AWSProfile:   getEnv("AWS_PROFILE", ""),
		AWSConfigDir: filepath.Join(userHome, ".config", "awsctl"),
	}

	fileConfig, err := loadConfigFile(cfg)
	if err != nil {
		return nil, err
	}
	cfg.RawCustomConfig = fileConfig

	return cfg, nil
}

func loadConfigFile(cfg *Config) (*models.Config, error) {
	configFilePath, err := FindConfigFile(cfg)
	if err != nil {
		return nil, fmt.Errorf("config file not found: %w", err)
	}

	fileData, err := os.ReadFile(configFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var parsedConfig models.Config
	if err := yaml.Unmarshal(fileData, &parsedConfig); err != nil {
		fmt.Println("YAML parsing failed, trying JSON...")
		if err := json.Unmarshal(fileData, &parsedConfig); err != nil {
			return nil, fmt.Errorf("failed to parse config file: %w", err)
		}
	}

	return &parsedConfig, nil
}

func FindConfigFile(cfg *Config) (string, error) {
	extensions := []string{"config.yml", "config.yaml", "config.json"}

	if _, err := os.Stat(cfg.AWSConfigDir); os.IsNotExist(err) {
		return "", fmt.Errorf("directory %s does not exist", cfg.AWSConfigDir)
	}

	for _, ext := range extensions {
		possiblePath := filepath.Join(cfg.AWSConfigDir, ext)
		if _, err := os.Stat(possiblePath); err == nil {
			return possiblePath, nil
		}
	}

	return "", fmt.Errorf("no config file found in %s", cfg.AWSConfigDir)
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}
