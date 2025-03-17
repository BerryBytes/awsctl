package utils

import (
	"awsctl/models"
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

var tokenCache = &models.TokenCache{}

func GetCachedSsoAccessToken(profile string) (string, error) {
	tokenCache.Mu.Lock()
	defer tokenCache.Mu.Unlock()

	// Check if the token is still valid
	if tokenCache.AccessToken != "" && time.Now().Before(tokenCache.Expiry) {
		return tokenCache.AccessToken, nil
	}

	// Fetch a new token from the cache
	accessToken, expiry, err := GetSsoAccessTokenFromCache(profile)
	if err != nil {
		return "", err
	}

	// Update the cache
	tokenCache.AccessToken = accessToken
	tokenCache.Expiry = expiry

	return accessToken, nil
}

// Searches for a profile by name in the configuration
func FindProfile(cfg *models.Config, profileName string) (*models.SSOProfile, error) {
	for _, profile := range cfg.Aws.Profiles {
		if profile.ProfileName == profileName {
			return &profile, nil
		}
	}
	return nil, fmt.Errorf("profile %s not found", profileName)
}

// Searches for an account by name in the profile
func FindAccount(profile *models.SSOProfile, accountName string) (*models.SSOAccount, error) {
	for _, account := range profile.Accounts {
		if account.AccountName == accountName {
			return &account, nil
		}
	}
	return nil, fmt.Errorf("account %s not found in profile %s", accountName, profile.ProfileName)
}

// Returns the account names from a profile
func ExtractAccountNames(profile *models.SSOProfile) []string {
	accounts := []string{}
	for _, account := range profile.Accounts {
		accounts = append(accounts, account.AccountName)
	}
	return accounts
}

// Returns a list of unique profile names from the configuration
func GetUniqueProfiles(cfg *models.Config) ([]string, error) {
	profiles := []string{}
	for _, profile := range cfg.Aws.Profiles {
		if !Contains(profiles, profile.ProfileName) {
			profiles = append(profiles, profile.ProfileName)
		}
	}
	if len(profiles) == 0 {
		return nil, fmt.Errorf("no profiles found in configuration")
	}
	return profiles, nil
}

// GetSSOProfiles lists all profiles that are configured with SSO sessions.
func GetSSOProfiles() ([]string, error) {
	// Get the path to the AWS config file
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get user home directory: %v", err)
	}
	configPath := filepath.Join(homeDir, ".aws", "config")

	// Open the AWS config file
	file, err := os.Open(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open AWS config file: %v", err)
	}
	defer file.Close()

	// Parse the config file
	var ssoProfiles []string
	scanner := bufio.NewScanner(file)
	var currentProfile string

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Check for profile sections
		if strings.HasPrefix(line, "[profile ") {
			currentProfile = strings.TrimPrefix(line, "[profile ")
			currentProfile = strings.TrimSuffix(currentProfile, "]")
		}

		// Check for sso_session key
		if strings.HasPrefix(line, "sso_session = ") && currentProfile != "" {
			ssoProfiles = append(ssoProfiles, currentProfile)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read AWS config file: %v", err)
	}

	return ssoProfiles, nil
}

func ConfigureSSO() error {
	cmd := exec.Command("aws", "configure", "sso")

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to configure AWS SSO: %v", err)
	}

	return nil
}

// Ensure SSO login
func EnsureSSOLogin(profile, region string) error {
	cmd := exec.Command("aws", "sts", "get-caller-identity", "--profile", profile)
	if err := cmd.Run(); err != nil {
		fmt.Println("AWS SSO session not found. Logging in...")
		loginCmd := exec.Command("aws", "sso", "login", "--profile", profile, "--region", region)
		loginCmd.Stdout = os.Stdout
		loginCmd.Stderr = os.Stderr
		if err := loginCmd.Run(); err != nil {
			return fmt.Errorf("failed to log in to AWS SSO: %v", err)
		}
	}
	return nil
}

// Get account name with account ID
func GetSSOAccountName(accountID, profile string) (string, error) {
	accessToken, err := GetCachedSsoAccessToken(profile)
	if err != nil {
		return "", fmt.Errorf("failed to retrieve SSO access token: %v", err)
	}

	cmd := exec.Command("aws", "sso", "list-accounts", "--access-token", accessToken, "--output", "json")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to list AWS accounts: %v", err)
	}

	var response struct {
		AccountList []struct {
			AccountID   string `json:"accountId"`
			AccountName string `json:"accountName"`
		} `json:"accountList"`
	}

	if err := json.Unmarshal(output, &response); err != nil {
		return "", fmt.Errorf("failed to unmarshal accounts: %v", err)
	}

	for _, account := range response.AccountList {
		if account.AccountID == accountID {
			return account.AccountName, nil
		}
	}

	return "", fmt.Errorf("account ID %s not found", accountID)
}

// Select a profile using the generic PromptForSelection utility
func SelectProfile(profiles []string) (string, error) {
	profile, err := PromptForSelection("Select an AWS SSO Profile", profiles)
	if err != nil {
		return "", fmt.Errorf("profile selection aborted: %v", err)
	}
	return profile, nil
}

func SelectAccount(accounts []models.SSOAccount) (string, error) {
	accountNames := make([]string, len(accounts))
	for i, account := range accounts {
		accountNames[i] = account.AccountName
	}

	accountName, err := PromptForSelection("Select an AWS Account", accountNames)
	if err != nil {
		return "", fmt.Errorf("account selection aborted: %v", err)
	}
	return accountName, nil
}

// Select role from list
func SelectRole(roles []string) (string, error) {
	role, err := PromptForSelection("Select an AWS Role", roles)
	if err != nil {
		return "", fmt.Errorf("role selection aborted: %v", err)
	}
	return role, nil

}

// Get region for selected profile
func GetAWSRegion(profile string) (string, error) {
	cmd := exec.Command("aws", "configure", "get", "region", "--profile", profile)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get AWS region: %v", err)
	}

	region := strings.TrimSpace(string(output))
	if region == "" {
		return "", fmt.Errorf("AWS region not found in profile %s", profile)
	}

	return region, nil
}

// Get region for selected profile
func GetAWSOutput(profile string) (string, error) {
	cmd := exec.Command("aws", "configure", "get", "output", "--profile", profile)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get AWS output: %v", err)
	}

	outputFormat := strings.TrimSpace(string(output))
	if outputFormat == "" {
		return "", fmt.Errorf("AWS output not found in profile %s", profile)
	}

	return outputFormat, nil
}

// Get roles for the selected account
func GetSSORoles(profile, accountID string) ([]string, error) {
	accessToken, err := GetCachedSsoAccessToken(profile)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve SSO access token: %v", err)
	}
	cmd := exec.Command("aws", "sso", "list-account-roles", "--profile", profile, "--account-id", accountID, "--access-token", accessToken, "--output", "json")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list AWS SSO roles: %v", err)
	}

	var roles struct {
		RoleList []struct {
			RoleName string `json:"roleName"`
		} `json:"roleList"`
	}

	if err := json.Unmarshal(output, &roles); err != nil {
		return nil, fmt.Errorf("failed to unmarshal roles: %v", err)
	}

	fmt.Printf("Parsed JSON Struct: %+v\n", roles)

	var roleNames []string
	for _, role := range roles.RoleList {
		roleNames = append(roleNames, role.RoleName)
	}

	if len(roleNames) == 0 {
		return nil, fmt.Errorf("no roles found for AWS account %s", accountID)
	}

	return roleNames, nil
}
