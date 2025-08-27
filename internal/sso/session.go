package sso

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/BerryBytes/awsctl/internal/sso/config"
	"github.com/BerryBytes/awsctl/models"
	promptUtils "github.com/BerryBytes/awsctl/utils/prompt"
)

func (c *RealSSOClient) LoadOrCreateSession(name, startURL, region string) (string, *models.SSOSession, error) {
	configPath, err := config.FindConfigFile(&c.Config)
	if err != nil && !errors.Is(err, config.ErrNoConfigFile) {
		return "", nil, fmt.Errorf("failed to check config file: %w", err)
	}

	if name != "" && startURL != "" && region != "" {
		ssoSession := &models.SSOSession{
			Name:     name,
			StartURL: strings.TrimSuffix(startURL, "#"),
			Region:   region,
			Scopes:   "sso:account:access",
		}

		c.Config.RawCustomConfig.SSOSessions = append(c.Config.RawCustomConfig.SSOSessions, *ssoSession)
		return configPath, ssoSession, nil
	}

	if configPath != "" && len(c.Config.RawCustomConfig.SSOSessions) > 0 {
		fmt.Printf("Loaded existing configuration from '%s'\n", configPath)

		if len(c.Config.RawCustomConfig.SSOSessions) == 1 && name == "" && startURL == "" && region == "" {
			ssoSession := &c.Config.RawCustomConfig.SSOSessions[0]
			ssoSession.StartURL = strings.TrimSuffix(ssoSession.StartURL, "#")
			if ssoSession.Scopes == "" {
				ssoSession.Scopes = "sso:account:access"
			}
			fmt.Printf("Using SSO session: %s (Start URL: %s, Region: %s)\n",
				ssoSession.Name, ssoSession.StartURL, ssoSession.Region)
			return configPath, ssoSession, nil
		}

		ssoSession, err := c.SelectSSOSession()
		if err != nil {
			if errors.Is(err, promptUtils.ErrInterrupted) {
				return "", nil, promptUtils.ErrInterrupted
			}
			return "", nil, fmt.Errorf("failed to select SSO session: %w", err)
		}
		if ssoSession != nil {
			return configPath, ssoSession, nil
		}
	}

	fmt.Println("Setting up a new AWS SSO configuration...")

	name, err = c.Prompter.PromptWithDefault("SSO session name", "default-sso")
	if err != nil {
		return "", nil, fmt.Errorf("failed to prompt for SSO session name: %w", err)
	}

	startURL, err = c.Prompter.PromptRequired("SSO start URL (e.g., https://my-sso-portal.awsapps.com/start)")
	if err != nil {
		return "", nil, fmt.Errorf("failed to prompt for SSO start URL: %w", err)
	}

	region, err = c.Prompter.PromptForRegion("us-east-1")
	if err != nil {
		return "", nil, fmt.Errorf("failed to prompt for SSO region: %w", err)
	}

	ssoSession := &models.SSOSession{
		Name:     name,
		StartURL: strings.TrimSuffix(startURL, "#"),
		Region:   region,
		Scopes:   "sso:account:access",
	}

	c.Config.RawCustomConfig.SSOSessions = append(c.Config.RawCustomConfig.SSOSessions, *ssoSession)
	return configPath, ssoSession, nil
}

func (c *RealSSOClient) SelectSSOSession() (*models.SSOSession, error) {
	options := make([]string, 0, len(c.Config.RawCustomConfig.SSOSessions)+1)
	sessionMap := make(map[string]*models.SSOSession)
	for i := range c.Config.RawCustomConfig.SSOSessions {
		session := &c.Config.RawCustomConfig.SSOSessions[i]
		startURL := strings.TrimRight(session.StartURL, "#/")
		display := fmt.Sprintf("%s (%s)", session.Name, startURL)
		options = append(options, display)
		sessionMap[display] = session
	}
	options = append(options, "Create new session")

	selected, err := c.Prompter.SelectFromList("Select an SSO session", options)
	if err != nil {
		if errors.Is(err, promptUtils.ErrInterrupted) {
			return nil, promptUtils.ErrInterrupted
		}
		return nil, fmt.Errorf("failed to select SSO session: %w", err)
	}
	if selected == "Create new session" {
		return nil, nil
	}

	ssoSession, exists := sessionMap[selected]
	if !exists {
		return nil, fmt.Errorf("selected session not found")
	}

	if ssoSession.Name == "" || ssoSession.StartURL == "" || ssoSession.Region == "" {
		return nil, fmt.Errorf("selected session '%s' has missing or invalid fields", ssoSession.Name)
	}

	ssoSession.StartURL = strings.TrimSuffix(ssoSession.StartURL, "#")
	if ssoSession.Scopes == "" {
		ssoSession.Scopes = "sso:account:access"
	}

	fmt.Printf("Using SSO session: %s (Start URL: %s, Region: %s)\n",
		ssoSession.Name, ssoSession.StartURL, ssoSession.Region)
	return ssoSession, nil
}

func (c *RealSSOClient) ConfigureSSOSession(sessionName, startURL, region, scopes string) error {
	fmt.Println("\nConfiguring AWS SSO session in ~/.aws/config...")
	startURL = strings.TrimSuffix(startURL, "#")

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get user home directory: %w", err)
	}
	configFile := filepath.Join(homeDir, ".aws", "config")
	configDir := filepath.Dir(configFile)

	if err := os.MkdirAll(configDir, 0700); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", configDir, err)
	}

	existingSession := make(map[string]string)
	fileExists := false
	if data, err := os.ReadFile(configFile); err == nil {
		fileExists = true
		lines := strings.Split(string(data), "\n")
		inSession := false
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == fmt.Sprintf("[sso-session %s]", sessionName) {
				inSession = true
				continue
			}
			if inSession && (strings.HasPrefix(line, "[") || line == "") {
				inSession = false
				continue
			}
			if inSession && strings.Contains(line, "=") {
				parts := strings.SplitN(line, "=", 2)
				if len(parts) == 2 {
					key := strings.TrimSpace(parts[0])
					value := strings.TrimSpace(parts[1])
					existingSession[key] = value
				}
			}
		}
	}

	newConfig := map[string]string{
		"sso_start_url":           startURL,
		"sso_region":              region,
		"sso_registration_scopes": scopes,
	}
	matches := true
	for key, newValue := range newConfig {
		if existingValue, exists := existingSession[key]; !exists || existingValue != newValue {
			matches = false
			break
		}
	}

	if len(existingSession) > 0 && matches {
		fmt.Printf("sso-session %s already configured with identical values, skipping write\n", sessionName)
		return nil
	}

	var newContent strings.Builder

	if fileExists {
		data, err := os.ReadFile(configFile)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", configFile, err)
		}

		lines := strings.Split(string(data), "\n")
		inSession := false
		for _, line := range lines {
			if strings.HasPrefix(line, fmt.Sprintf("[sso-session %s]", sessionName)) {
				inSession = true
				continue
			}
			if inSession && (strings.HasPrefix(line, "[") || line == "") {
				inSession = false
			}
			if !inSession {
				newContent.WriteString(line)
				newContent.WriteString("\n")
			}
		}
	}

	newContent.WriteString(fmt.Sprintf("[sso-session %s]\n", sessionName))
	newContent.WriteString(fmt.Sprintf("sso_start_url = %s\n", startURL))
	newContent.WriteString(fmt.Sprintf("sso_region = %s\n", region))
	newContent.WriteString(fmt.Sprintf("sso_registration_scopes = %s\n", scopes))
	newContent.WriteString("\n")

	if err := writeConfigFile(configFile, newContent.String()); err != nil {
		return fmt.Errorf("failed to write %s: %w", configFile, err)
	}

	fmt.Printf("Set sso-session.%s.sso_start_url = %s\n", sessionName, startURL)
	fmt.Printf("Set sso-session.%s.sso_region = %s\n", sessionName, region)
	fmt.Printf("Set sso-session.%s.sso_registration_scopes = %s\n", sessionName, scopes)

	return nil
}

func (c *RealSSOClient) RunSSOLogin(sessionName string) error {
	if err := c.validateAWSConfig(sessionName); err != nil {
		return fmt.Errorf("invalid SSO configuration: %w", err)
	}

	fmt.Println("\nInitiating AWS SSO login... (this may open a browser window)")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	if err := c.Executor.RunInteractiveCommand(ctx, "aws", "sso", "login", "--sso-session", sessionName); err != nil {
		return fmt.Errorf("error during SSO login: %w; ensure AWS CLI is updated (run 'aws --version'), verify the SSO Start URL, region, and network connectivity", err)
	}

	fmt.Println("AWS SSO login successful")
	return nil
}

func (c *RealSSOClient) validateAWSConfig(sessionName string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get user home directory: %w", err)
	}
	configFile := filepath.Join(homeDir, ".aws", "config")
	data, err := os.ReadFile(configFile)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", configFile, err)
	}
	if !strings.Contains(string(data), fmt.Sprintf("[sso-session %s]", sessionName)) {
		return fmt.Errorf("sso-session %s not found in %s; check the configuration", sessionName, configFile)
	}
	return nil
}

func (c *RealSSOClient) GetAccessToken(startURL string) (string, error) {
	startURL = strings.TrimSuffix(startURL, "#")

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}
	cacheDir := filepath.Join(homeDir, ".aws", "sso", "cache")
	files, err := os.ReadDir(cacheDir)
	if err != nil {
		return "", fmt.Errorf("failed to read SSO cache directory: %w", err)
	}

	var latestToken string
	var latestExpiry time.Time
	foundExpired := false

	for _, file := range files {
		if file.IsDir() {
			continue
		}
		cacheFile := filepath.Join(cacheDir, file.Name())
		data, err := os.ReadFile(cacheFile)
		if err != nil {
			continue
		}

		var cache struct {
			StartURL    string `json:"startUrl"`
			AccessToken string `json:"accessToken"`
			ExpiresAt   string `json:"expiresAt"`
		}
		if err := json.Unmarshal(data, &cache); err != nil {
			continue
		}

		if cache.StartURL == startURL && cache.AccessToken != "" {
			expireStr := strings.Replace(cache.ExpiresAt, "UTC", "Z", 1)
			expireTime, err := time.Parse(time.RFC3339, expireStr)
			if err != nil {
				return "", fmt.Errorf("invalid expiration time: %w", err)
			}

			if time.Now().After(expireTime) {
				foundExpired = true
				continue
			}

			// keep the latest valid token
			if expireTime.After(latestExpiry) {
				latestExpiry = expireTime
				latestToken = cache.AccessToken
			}
		}
	}

	if latestToken != "" {
		return latestToken, nil
	}

	if foundExpired {
		return "", fmt.Errorf("access token expired for start URL: %s", startURL)
	}

	return "", fmt.Errorf("no valid access token found for start URL: %s", startURL)
}
