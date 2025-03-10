package utils

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// AWS Credentials struct
type AWSCredentials struct {
	AccessKeyID     string
	SecretAccessKey string
	SessionToken    string
	Expiration      string
}

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
	accountName := GetAccountName(accountID) // Get the friendly account name

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
