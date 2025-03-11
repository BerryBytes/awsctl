package utils

import (
	"awsctl/models"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// findProfile searches for a profile by name in the configuration
func FindProfile(cfg *models.Config, profileName string) (*models.SSOProfile, error) {
	for _, profile := range cfg.Aws.Profiles {
		if profile.ProfileName == profileName {
			return &profile, nil
		}
	}
	return nil, fmt.Errorf("profile %s not found", profileName)
}

// findAccount searches for an account by name in the profile
func FindAccount(profile *models.SSOProfile, accountName string) (*models.SSOAccount, error) {
	for _, account := range profile.Accounts {
		if account.AccountName == accountName {
			return &account, nil
		}
	}
	return nil, fmt.Errorf("account %s not found in profile %s", accountName, profile.ProfileName)
}

// extractAccountNames returns the account names from a profile
func ExtractAccountNames(profile *models.SSOProfile) []string {
	accounts := []string{}
	for _, account := range profile.Accounts {
		accounts = append(accounts, account.AccountName)
	}
	return accounts
}

// getUniqueProfiles returns a list of unique profile names from the configuration
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

// Get AWS SSO profiles
func GetSSOProfiles() ([]string, error) {
	cmd := exec.Command("aws", "configure", "list-profiles")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list AWS profiles: %v", err)
	}

	profiles := strings.Split(strings.TrimSpace(string(output)), "\n")
	var ssoProfiles []string
	for _, profile := range profiles {
		checkCmd := exec.Command("aws", "configure", "get", "sso_start_url", "--profile", profile)
		checkOutput, _ := checkCmd.Output()
		if strings.TrimSpace(string(checkOutput)) != "" {
			ssoProfiles = append(ssoProfiles, profile)
		}
	}

	return ssoProfiles, nil
}

// Configure AWS SSO programmatically
func ConfigureSSO(ssoStartURL, ssoRegion, profileName string) error {
	cmd := exec.Command("aws", "configure", "sso", "--sso-start-url", ssoStartURL, "--sso-region", ssoRegion, "--profile", profileName)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to configure AWS SSO: %v", err)
	}
	return nil
}

// Get SSO start URL for a profile
func GetSSOStartURL(profile string) (string, error) {
	cmd := exec.Command("aws", "configure", "get", "sso_start_url", "--profile", profile)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get SSO start URL for profile %s: %v", profile, err)
	}
	return strings.TrimSpace(string(output)), nil
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

// Get available accounts
func GetSSOAccounts(profile string) ([]models.SSOAccount, error) {
	cmd := exec.Command("aws", "sso", "list-accounts", "--profile", profile, "--output", "json")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list AWS accounts: %v", err)
	}

	var accounts []models.SSOAccount
	if err := json.Unmarshal(output, &accounts); err != nil {
		return nil, fmt.Errorf("failed to unmarshal accounts: %v", err)
	}

	if len(accounts) == 0 {
		return nil, fmt.Errorf("no AWS accounts found for SSO")
	}

	return accounts, nil
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
	// Create a slice of account names from the SSOAccount structs
	accountNames := make([]string, len(accounts))
	for i, account := range accounts {
		accountNames[i] = account.AccountName
	}

	// Prompt the user to select an account from the list
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
	cmd := exec.Command("aws", "configure", "get", "sso_region", "--profile", profile)
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

// Get roles for the selected account
func GetSSORoles(profile, accountID string) ([]string, error) {
	cmd := exec.Command("aws", "sso", "list-account-roles", "--profile", profile, "--account-id", accountID, "--output", "json")
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

	var roleNames []string
	for _, role := range roles.RoleList {
		roleNames = append(roleNames, role.RoleName)
	}

	if len(roleNames) == 0 {
		return nil, fmt.Errorf("no roles found for AWS account %s", accountID)
	}

	return roleNames, nil
}
