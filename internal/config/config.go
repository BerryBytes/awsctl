package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/BerryBytes/awsctl/models"

	"gopkg.in/yaml.v3"
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
		if errors.Is(err, ErrNoConfigFile) {
			cfg.RawCustomConfig = nil
			return cfg, nil
		}
		return nil, err
	}
	cfg.RawCustomConfig = fileConfig
	return cfg, nil
}

func loadConfigFile(cfg *Config) (*models.Config, error) {
	configFilePath, err := FindConfigFile(cfg)
	if err != nil {
		return nil, err
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

var ErrNoConfigFile = errors.New("no config file found")

func FindConfigFile(cfg *Config) (string, error) {
	extensions := []string{"config.yml", "config.yaml", "config.json"}

	if _, err := os.Stat(cfg.AWSConfigDir); os.IsNotExist(err) {
		return "", ErrNoConfigFile
	} else if err != nil {
		return "", fmt.Errorf("failed to stat directory %s: %w", cfg.AWSConfigDir, err)
	}

	for _, ext := range extensions {
		possiblePath := filepath.Join(cfg.AWSConfigDir, ext)
		if _, err := os.Stat(possiblePath); err == nil {
			return possiblePath, nil
		}
	}

	return "", ErrNoConfigFile
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}
