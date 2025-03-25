package sso

import (
	"awsctl/models"
	promptutils "awsctl/utils/prompt"
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

type AWSSSOClient interface {
	GetCachedSsoAccessToken() (string, error)
	ConfigureSSO() error
	GetSSOProfiles() ([]string, error)
	GetSSOAccountName(accountID, profile string) (string, error)
	GetSSORoles(profile, accountID string) ([]string, error)
	SSOLogin(awsProfile string, refresh, noBrowser bool) error
}

type RealAWSSSOClient struct {
	TokenCache        models.TokenCache
	CredentialsClient AWSCredentialsClient
}

func (c *RealSSOClient) configureProfile(profile *models.SSOProfile, account *models.SSOAccount, role string) error {
	fmt.Printf("Selected Profile: %s\n", profile.ProfileName)
	fmt.Printf("Selected Account: %s\n", account.AccountName)
	if role != "" {
		fmt.Printf("Selected Role: %s\n", role)
	}

	if err := c.AWSClient.ConfigClient.ConfigureDefaultProfile(profile.Region, "json"); err != nil {
		return fmt.Errorf("failed to configure default profile: %v", err)
	}

	ssoProfile := fmt.Sprintf("sso-%s-%s", account.AccountName, role)
	if err := c.AWSClient.ConfigClient.ConfigureSSOProfile(
		ssoProfile,
		profile.Region,
		account.AccountID,
		role,
		profile.SsoStartUrl,
	); err != nil {
		return fmt.Errorf("failed to configure SSO profile: %v", err)
	}

	fmt.Printf("Successfully configured profile: %s\n", ssoProfile)
	return nil
}

func getSsoAccessTokenFromCache() (string, time.Time, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", time.Time{}, fmt.Errorf("failed to get user home directory: %v", err)
	}
	cacheDir := filepath.Join(homeDir, ".aws", "sso", "cache")

	files, err := os.ReadDir(cacheDir)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("failed to read SSO cache directory: %v", err)
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
		return "", time.Time{}, fmt.Errorf("no SSO cache files found")
	}

	cacheFilePath := filepath.Join(cacheDir, latestFile)
	cacheFile, err := os.ReadFile(cacheFilePath)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("failed to read SSO cache file: %v", err)
	}

	var cache struct {
		AccessToken string `json:"accessToken"`
		ExpiresAt   string `json:"expiresAt"`
	}
	if err := json.Unmarshal(cacheFile, &cache); err != nil {
		return "", time.Time{}, fmt.Errorf("failed to unmarshal SSO cache file: %v", err)
	}

	if cache.AccessToken == "" {
		return "", time.Time{}, fmt.Errorf("no access token found in SSO cache file")
	}

	expiryTime, err := time.Parse(time.RFC3339, cache.ExpiresAt)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("invalid expiration time format: %v", err)
	}

	return cache.AccessToken, expiryTime, nil
}

func (c *RealAWSSSOClient) GetCachedSsoAccessToken() (string, error) {
	c.TokenCache.Mu.Lock()
	defer c.TokenCache.Mu.Unlock()

	if c.TokenCache.AccessToken != "" && time.Now().Before(c.TokenCache.Expiry) {
		return c.TokenCache.AccessToken, nil
	}

	accessToken, expiry, err := getSsoAccessTokenFromCache()
	if err != nil {
		return "", err
	}

	c.TokenCache.AccessToken = accessToken
	c.TokenCache.Expiry = expiry

	return accessToken, nil
}

func (c *RealAWSSSOClient) ConfigureSSO() error {
	cmd := exec.Command("aws", "configure", "sso")

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			if status, ok := exitError.Sys().(syscall.WaitStatus); ok {
				if status.ExitStatus() == 130 {
					fmt.Println("\nProcess terminated by user.")
					return promptutils.ErrInterrupted
				}
			}
		}
		return fmt.Errorf("failed to configure AWS SSO: %v", err)
	}
	return nil
}

func (c *RealAWSSSOClient) GetSSOProfiles() ([]string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get user home directory: %v", err)
	}
	configPath := filepath.Join(homeDir, ".aws", "config")

	file, err := os.Open(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open AWS config file: %v", err)
	}
	defer file.Close()

	var ssoProfiles []string
	scanner := bufio.NewScanner(file)
	var currentProfile string

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if strings.HasPrefix(line, "[profile ") {
			currentProfile = strings.TrimPrefix(line, "[profile ")
			currentProfile = strings.TrimSuffix(currentProfile, "]")
		}

		if strings.HasPrefix(line, "sso_session = ") && currentProfile != "" {
			ssoProfiles = append(ssoProfiles, currentProfile)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read AWS config file: %v", err)
	}

	return ssoProfiles, nil
}

func (c *RealAWSSSOClient) GetSSOAccountName(accountID, profile string) (string, error) {
	accessToken, err := c.GetCachedSsoAccessToken()
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

func (c *RealAWSSSOClient) GetSSORoles(profile, accountID string) ([]string, error) {
	accessToken, err := c.GetCachedSsoAccessToken()
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

	var roleNames []string
	for _, role := range roles.RoleList {
		roleNames = append(roleNames, role.RoleName)
	}

	if len(roleNames) == 0 {
		return nil, fmt.Errorf("no roles found for AWS account %s", accountID)
	}

	return roleNames, nil
}

func (c *RealAWSSSOClient) SSOLogin(awsProfile string, refresh, noBrowser bool) error {
	if refresh || !c.CredentialsClient.IsCallerIdentityValid(awsProfile) {
		args := []string{"sso", "login"}
		if noBrowser {
			args = append(args, "--no-browser")
		}
		args = append(args, "--profile", awsProfile)

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		cmd := exec.CommandContext(ctx, "aws", args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Start(); err != nil {
			return fmt.Errorf("error starting SSO login: %w", err)
		}

		if err := cmd.Wait(); err != nil {
			if ctx.Err() == context.DeadlineExceeded {
				return fmt.Errorf("SSO login timed out: the login flow was canceled or not completed")
			}
			// Check for specific AWS CLI errors
			if exitErr, ok := err.(*exec.ExitError); ok {
				return fmt.Errorf("SSO login failed: %s", string(exitErr.Stderr))
			}
			return fmt.Errorf("error during SSO login: %w", err)
		}
	}
	return nil
}
