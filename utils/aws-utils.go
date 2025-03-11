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
	// Define a helper function to avoid code repetition for error handling
	saveCredential := func(name, value string) error {
		err := AwsConfigureSet(name, value, profile)
		if err != nil {
			return fmt.Errorf("failed to set %s for profile %s: %w", name, profile, err)
		}
		return nil
	}

	// Save each credential and handle errors
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

// Read AWS SSO access token from cache
func GetSsoAccessTokenFromCache() (string, error) {
	// Compute the cache filename using shasum
	cacheFilename := shasum("https://osm.awsapps.com/start")

	// Construct the full path to the cache file
	cachePath := filepath.Join(os.Getenv("HOME"), ".aws/sso/cache", cacheFilename+".json")

	// Read the cache file
	data, err := os.ReadFile(cachePath)
	if err != nil {
		return "", fmt.Errorf("failed to read SSO cache file: %w", err)
	}

	// Parse the cache file to extract the access token
	var cache map[string]interface{}
	if err := json.Unmarshal(data, &cache); err != nil {
		return "", fmt.Errorf("failed to parse SSO cache file: %w", err)
	}

	accessToken, exists := cache["accessToken"]
	if !exists {
		return "", fmt.Errorf("access token not found in SSO cache file")
	}

	return accessToken.(string), nil
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
func PrintCurrentRole(profile, accountID, roleName, roleARN, expiration string) {
	// accountName := GetAccountName(accountID) // Get the friendly account name
	accountName := "Development" // Get the friendly account name

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

	var identity struct {
		UserID string `json:"UserId"`
	}
	if err := json.Unmarshal(output, &identity); err != nil {
		return false
	}

	return strings.Contains(identity.UserID, "mbm")
}

func Contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func shasum(input string) string {
	cmd := exec.Command("shasum")
	cmd.Stdin = strings.NewReader(input)
	output, err := cmd.Output()
	if err != nil {
		fmt.Println("Error calculating shasum:", err)
		os.Exit(1)
	}
	return strings.Split(string(output), " ")[0]
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

// func SaveSSOProfile(profileName string, account models.SSOAccount) error {
// 	// Step 1: Find the config file
// 	configFilePath, err := config.FindConfigFile()
// 	if err != nil {
// 		return fmt.Errorf("config file not found: %w", err)
// 	}

// 	// Step 2: Load the current configuration
// 	var cfg models.Config
// 	fileData, err := os.ReadFile(configFilePath)
// 	if err != nil {
// 		return fmt.Errorf("failed to read config file: %w", err)
// 	}

// 	// Unmarshal the YAML or JSON config
// 	if err := yaml.Unmarshal(fileData, &cfg); err != nil {
// 		// Try JSON unmarshalling if YAML fails
// 		if err := json.Unmarshal(fileData, &cfg); err != nil {
// 			return fmt.Errorf("failed to parse config file: %w", err)
// 		}
// 	}

// 	// Step 3: Find the AWS section for the given profile
// 	if cfg.Aws.Profile != profileName {
// 		return fmt.Errorf("profile %s not found in the config", profileName)
// 	}

// 	// Step 4: Check if the account exists, and update if it does
// 	accountExists := false
// 	for i, existingAccount := range cfg.Aws.Accounts.AccountList {
// 		if existingAccount.AccountID == account.AccountID {
// 			// Update the existing account details
// 			cfg.Aws.Accounts.AccountList[i] = account
// 			accountExists = true
// 			break
// 		}
// 	}

// 	// If the account doesn't exist, append the new account to the list
// 	if !accountExists {
// 		cfg.Aws.Accounts.AccountList = append(cfg.Aws.Accounts.AccountList, account)
// 	}

// 	// Step 5: Marshal the updated config back to YAML or JSON
// 	updatedData, err := yaml.Marshal(cfg)
// 	if err != nil {
// 		return fmt.Errorf("failed to marshal updated config: %w", err)
// 	}

// 	// Step 6: Save the updated config back to the file
// 	err = os.WriteFile(configFilePath, updatedData, 0644)
// 	if err != nil {
// 		return fmt.Errorf("failed to save updated config file: %w", err)
// 	}

// 	return nil
// }
