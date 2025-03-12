package utils

import (
	"awsctl/models"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Get available AWS CLI profiles
func ValidProfiles() ([]string, error) {
	cmd := exec.Command("aws", "configure", "list-profiles")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	profiles := strings.Split(strings.TrimSpace(string(output)), "\n")
	return profiles, nil
}

// Set AWS CLI config values
func AwsConfigureSet(key, value, profile string) error {
	cmd := exec.Command("aws", "configure", "set", key, value, "--profile", profile)
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to set AWS config for profile %s: %w", profile, err)
	}
	return nil
}

// Get AWS configuration values
func AwsConfigureGet(key, profile string) (string, error) {
	cmd := exec.Command("aws", "configure", "get", key, "--profile", profile)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get %s: %w", key, err)
	}
	return strings.TrimSpace(string(output)), nil
}

// Save AWS credentials in AWS config
func SaveAWSCredentials(profile string, creds *models.AWSCredentials) error {
	saveCredential := func(name, value string) error {
		err := AwsConfigureSet(name, value, profile)
		if err != nil {
			return fmt.Errorf("failed to set %s for profile %s: %w", name, profile, err)
		}
		return nil
	}

	if err := saveCredential("aws_access_key_id", creds.AccessKeyID); err != nil {
		return err
	}

	if err := saveCredential("aws_secret_access_key", creds.SecretAccessKey); err != nil {
		return err
	}

	if err := saveCredential("aws_session_token", creds.SessionToken); err != nil {
		return err
	}

	return nil
}

// Retrieves the SSO access token from the cache.
func GetSsoAccessTokenFromCache(profile string) (string, error) {
	// Get the path to the SSO cache directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %v", err)
	}
	cacheDir := filepath.Join(homeDir, ".aws", "sso", "cache")

	// Find the latest cache file
	files, err := os.ReadDir(cacheDir)
	if err != nil {
		return "", fmt.Errorf("failed to read SSO cache directory: %v", err)
	}

	var latestFile string
	var latestModTime time.Time
	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".json") {
			info, err := file.Info()
			if err != nil {
				continue
			}
			if info.ModTime().After(latestModTime) {
				latestFile = file.Name()
				latestModTime = info.ModTime()
			}
		}
	}

	if latestFile == "" {
		return "", fmt.Errorf("no SSO cache files found")
	}

	// Read the cache file
	cacheFilePath := filepath.Join(cacheDir, latestFile)
	cacheFile, err := os.ReadFile(cacheFilePath)
	if err != nil {
		return "", fmt.Errorf("failed to read SSO cache file: %v", err)
	}

	// Parse the cache file
	var cache struct {
		AccessToken string `json:"accessToken"`
	}
	if err := json.Unmarshal(cacheFile, &cache); err != nil {
		return "", fmt.Errorf("failed to unmarshal SSO cache file: %v", err)
	}

	if cache.AccessToken == "" {
		return "", fmt.Errorf("no access token found in SSO cache file")
	}

	return cache.AccessToken, nil
}

// Fetch AWS Role Credentials using SSO
func GetRoleCredentials(accessToken, roleName, accountID string) (*models.AWSCredentials, error) {
	cmd := exec.Command("aws", "sso", "get-role-credentials",
		"--access-token", accessToken,
		"--role-name", roleName,
		"--account-id", accountID)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get role credentials: %w", err)
	}

	var response models.RoleCredentialsResponse
	if err := json.Unmarshal(output, &response); err != nil {
		return nil, fmt.Errorf("failed to parse credentials JSON: %w", err)
	}

	return &models.AWSCredentials{
		AccessKeyID:     response.RoleCredentials.AccessKeyID,
		SecretAccessKey: response.RoleCredentials.SecretAccessKey,
		SessionToken:    response.RoleCredentials.SessionToken,
		Expiration:      time.Unix(response.RoleCredentials.Expiration/1000, 0).Format(time.RFC3339),
	}, nil
}

// Fetch Role ARN
func AwsSTSGetCallerIdentity(profile string) (string, error) {
	cmd := exec.Command("aws", "sts", "get-caller-identity", "--profile", profile)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get caller identity: %w", err)
	}

	var identity struct {
		Arn string `json:"Arn"`
	}
	if err := json.Unmarshal(output, &identity); err != nil {
		return "", fmt.Errorf("failed to parse identity JSON: %w", err)
	}

	return identity.Arn, nil
}

// Print AWS Role details
func PrintCurrentRole(profile, accountID, accountName, roleName, roleARN, expiration string) {

	fmt.Printf(`
📌 AWS Session Details:
---------------------------------
✅ Profile      : %s
🏦 Account Id   : %s
🏷️  Account Name : %s
🔑 Role Name    : %s
🆔 Role ARN     : %s
⌛ Expiration   : %s
---------------------------------
`, profile, accountID, accountName, roleName, roleARN, expiration)
}

// Check if the caller identity is valid
func IsCallerIdentityValid(profile string) bool {
	cmd := exec.Command("aws", "sts", "get-caller-identity", "--profile", profile)
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	// Parse the output into a struct
	var identity struct {
		UserID string `json:"UserId"`
	}
	if err := json.Unmarshal(output, &identity); err != nil {
		return false
	}

	// Check if UserID is valid (non-empty)
	return identity.UserID != ""
}

func Contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// configures the default AWS profile
func ConfigureDefaultProfile(region string) error {
	if err := AwsConfigureSet("region", region, "default"); err != nil {
		fmt.Println("Error setting region:", err)
		return err
	}
	if err := AwsConfigureSet("output", "json", "default"); err != nil {
		fmt.Println("Error setting output format:", err)
		return err
	}
	return nil
}

// configures the AWS SSO profile
func ConfigureSSOProfile(profile, region, accountID, role, ssoStartUrl string) error {
	configs := map[string]string{
		"sso_region":     region,
		"sso_account_id": accountID,
		"sso_start_url":  ssoStartUrl,
		"sso_role_name":  role,
		"region":         region,
		"output":         "json",
	}

	for key, value := range configs {
		if err := AwsConfigureSet(key, value, profile); err != nil {
			fmt.Printf("Error setting %s: %v\n", key, err)
			return err
		}
	}
	return nil
}

// handles setup abortion
func AbortSetup(err error) error {
	fmt.Println("Setup aborted. No changes made.")
	return err
}
