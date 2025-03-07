package utils

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/manifoldco/promptui"
)

// Select AWS profile interactively
func SelectAWSProfile() (string, error) {
	profiles, err := VerifyProfiles()
	if err != nil {
		return "", err
	}

	prompt := promptui.Select{
		Label: "✔ Choose AWS Profile:",
		Items: profiles,
	}

	index, result, err := prompt.Run()
	if err != nil {
		return "", err
	}

	fmt.Printf("✅ AWS Profile set: %s (Index: %d)\n", result, index)
	return result, nil
}

// Get available AWS CLI profiles
func VerifyProfiles() ([]string, error) {
	cmd := exec.Command("aws", "configure", "list-profiles")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	profiles := strings.Split(strings.TrimSpace(string(output)), "\n")
	return profiles, nil
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

// Read AWS SSO access token from cache
func GetSSOAccessToken() (string, error) {
	cachePath := filepath.Join(os.Getenv("HOME"), ".aws/sso/cache")

	files, err := os.ReadDir(cachePath)
	if err != nil {
		return "", fmt.Errorf("failed to read SSO cache directory: %w", err)
	}

	for _, file := range files {
		data, err := os.ReadFile(filepath.Join(cachePath, file.Name()))
		if err != nil {
			continue
		}

		var cache map[string]interface{}
		if err := json.Unmarshal(data, &cache); err != nil {
			continue
		}

		if token, exists := cache["accessToken"]; exists {
			return token.(string), nil
		}
	}

	return "", fmt.Errorf("no valid AWS SSO access token found")
}

// GetAccount returns the account name based on the account ID
// func GetAccount(accountID string) string {
// 	accountMap := map[string]string{
// 		"05882":  "Ls",
// 		"3252":   "Nood",
// 		"702":    "Prion",
// 		"913699": "ity",
// 		"8165":   "vices",
// 		"79144":  "berg",
// 	}

// 	if accountName, exists := accountMap[accountID]; exists {
// 		return accountName
// 	}

// 	fmt.Printf("Unknown Account Id: %s\n", accountID)
// 	return "Unknown"
// }

// Fetch AWS Role Credentials using SSO
func GetRoleCredentials(accessToken, roleName, accountID string) (*AWSCredentials, error) {
	cmd := exec.Command("aws", "sso", "get-role-credentials",
		"--access-token", accessToken,
		"--role-name", roleName,
		"--account-id", accountID)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get role credentials: %w", err)
	}

	var response struct {
		RoleCredentials struct {
			AccessKeyID     string `json:"accessKeyId"`
			SecretAccessKey string `json:"secretAccessKey"`
			SessionToken    string `json:"sessionToken"`
			Expiration      int64  `json:"expiration"`
		} `json:"roleCredentials"`
	}
	if err := json.Unmarshal(output, &response); err != nil {
		return nil, fmt.Errorf("failed to parse credentials JSON: %w", err)
	}

	return &AWSCredentials{
		AccessKeyID:     response.RoleCredentials.AccessKeyID,
		SecretAccessKey: response.RoleCredentials.SecretAccessKey,
		SessionToken:    response.RoleCredentials.SessionToken,
		Expiration:      time.Unix(response.RoleCredentials.Expiration/1000, 0).Format(time.RFC3339),
	}, nil
}

// Save AWS credentials in AWS config
func SaveAWSCredentials(profile string, creds *AWSCredentials) {
	errAccessKeyId := AwsConfigureSet("aws_access_key_id", creds.AccessKeyID, profile)
	if errAccessKeyId != nil {
		fmt.Println("Error setting AWS access key:", errAccessKeyId)
		// Handle the error (e.g., return, exit, or retry)
	}
	// AwsConfigureSet("aws_access_key_id", creds.AccessKeyID, profile)
	// AwsConfigureSet("aws_secret_access_key", creds.SecretAccessKey, profile)
	errSecretKey := AwsConfigureSet("aws_secret_access_key", creds.SecretAccessKey, profile)
	if errSecretKey != nil {
		fmt.Println("Error setting AWS access key:", errSecretKey)
		// Handle the error (e.g., return, exit, or retry)
	}
	// AwsConfigureSet("aws_session_token", creds.SessionToken, profile)
	errSessionToken := AwsConfigureSet("aws_session_token", creds.SessionToken, profile)
	if errSessionToken != nil {
		fmt.Println("Error setting AWS access key:", errSessionToken)
		// Handle the error (e.g., return, exit, or retry)
	}

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

// Get accountID with account
func GetAccountID(account string) string {
	accountMap := map[string]string{
		"Logs":            "058821512442",
		"NonProd":         "325313491802",
		"Security":        "915416153699",
		"Production":      "709693661542",
		"Shared Services": "811108753365",
		"MarcRosenberg":   "791543727244",
	}

	return accountMap[account]
}

// Get account with accountID
func GetAccountName(accountID string) string {
	accountMap := map[string]string{
		"058821512442": "Logs",
		"325313491802": "NonProd",
		"915416153699": "Security",
		"709693661542": "Production",
		"811108753365": "Shared Services",
		"791543727244": "MarcRosenberg",
	}

	return accountMap[accountID]
}

// AWS Credentials struct
type AWSCredentials struct {
	AccessKeyID     string
	SecretAccessKey string
	SessionToken    string
	Expiration      string
}
