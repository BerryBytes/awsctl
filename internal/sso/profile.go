package sso

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func (c *RealSSOClient) ConfigureSSOProfile(profile, region, accountID, role, ssoStartUrl, ssoSession string) error {
	configs := map[string]string{
		"sso_session":    ssoSession,
		"sso_region":     region,
		"sso_account_id": accountID,
		"sso_start_url":  ssoStartUrl,
		"sso_role_name":  role,
		"region":         region,
		"output":         "json",
	}

	for key, value := range configs {
		if err := c.ConfigureSet(key, value, profile); err != nil {
			fmt.Printf("Error setting %s: %v\n", key, err)
			return err
		}
	}
	return nil
}

func (c *RealSSOClient) configureAWSProfile(profileName, sessionName, ssoRegion, ssoStartURL, accountID, roleName, region string) error {
	ssoStartURL = strings.TrimSuffix(ssoStartURL, "#")
	if err := validateStartURL(ssoStartURL); err != nil {
		return fmt.Errorf("invalid start URL: %w", err)
	}
	if err := validateAccountID(accountID); err != nil {
		return fmt.Errorf("invalid account ID: %w", err)
	}

	if profileName != "default" {
		if err := c.ConfigureSSOProfile(profileName, ssoRegion, accountID, roleName, ssoStartURL, sessionName); err != nil {
			return fmt.Errorf("failed to configure SSO profile %s: %w", profileName, err)
		}
		fmt.Printf("Configured AWS profile '%s'\n", profileName)
		return nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get user home directory: %w", err)
	}
	configFile := filepath.Join(homeDir, ".aws", "config")
	configDir := filepath.Dir(configFile)

	if err := os.MkdirAll(configDir, 0700); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", configDir, err)
	}

	var configContent strings.Builder
	if _, err := os.Stat(configFile); err == nil {
		data, err := os.ReadFile(configFile)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", configFile, err)
		}
		configContent.Write(data)
		if len(data) > 0 && data[len(data)-1] != '\n' {
			configContent.WriteString("\n")
		}
	}

	lines := strings.Split(configContent.String(), "\n")
	var newLines []string
	inDefault := false
	for _, line := range lines {
		if strings.HasPrefix(line, "[default]") {
			inDefault = true
			fmt.Println("Existing default profile found, overwriting...")
			continue
		}
		if inDefault && (strings.HasPrefix(line, "[") || line == "") {
			inDefault = false
		}
		if !inDefault {
			newLines = append(newLines, line)
		}
	}
	configContent.Reset()
	configContent.WriteString(strings.Join(newLines, "\n"))
	configContent.WriteString("\n")

	configContent.WriteString("[default]\n")
	configContent.WriteString(fmt.Sprintf("sso_session = %s\n", sessionName))
	configContent.WriteString(fmt.Sprintf("sso_account_id = %s\n", accountID))
	configContent.WriteString(fmt.Sprintf("sso_role_name = %s\n", roleName))
	configContent.WriteString(fmt.Sprintf("region = %s\n", region))
	configContent.WriteString(fmt.Sprintf("sso_region = %s\n", ssoRegion))
	configContent.WriteString(fmt.Sprintf("sso_start_url = %s\n", ssoStartURL))
	configContent.WriteString("output = json\n")

	if err := writeConfigFile(configFile, configContent.String()); err != nil {
		return fmt.Errorf("failed to write %s: %w", configFile, err)
	}

	fmt.Println("Configured AWS default profile")

	data, err := os.ReadFile(configFile)
	if err != nil {
		return fmt.Errorf("failed to verify %s: %w", configFile, err)
	}
	configText := string(data)
	if !strings.Contains(configText, "[default]") ||
		!strings.Contains(configText, fmt.Sprintf("sso_session = %s", sessionName)) ||
		!strings.Contains(configText, fmt.Sprintf("sso_account_id = %s", accountID)) ||
		!strings.Contains(configText, fmt.Sprintf("sso_role_name = %s", roleName)) {
		return fmt.Errorf("failed to configure default profile in %s: missing required fields", configFile)
	}

	return nil
}

func (c *RealSSOClient) promptProfileDetails(ssoRegion string) (string, string, error) {
	profileName, err := c.Prompter.PromptWithDefault("Enter profile name to configure", "sso-profile")
	if err != nil {
		return "", "", fmt.Errorf("failed to prompt for profile name: %w", err)
	}
	region, err := c.Prompter.PromptWithDefault("AWS region for this profile", ssoRegion)
	if err != nil {
		return "", "", fmt.Errorf("failed to prompt for region: %w", err)
	}
	return profileName, region, nil
}
