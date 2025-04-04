package sso

import (
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

	"github.com/BerryBytes/awsctl/models"
	promptUtils "github.com/BerryBytes/awsctl/utils/prompt"

	"gopkg.in/ini.v1"
)

type AWSSSOClient interface {
	GetCachedSsoAccessToken(string) (string, error)
	ConfigureSSO() error
	GetSSOProfiles() ([]string, error)
	GetSSOAccountName(accountID, profile string) (string, error)
	GetSSORoles(profile, accountID string) ([]string, error)
	SSOLogin(awsProfile string, refresh, noBrowser bool) error
}

type RealAWSSSOClient struct {
	TokenCache        models.TokenCache
	CredentialsClient AWSCredentialsClient
	Executor          CommandExecutor
	getHomeDir        func() (string, error)
	AWSClient         *AWSClient
}

func NewRealAWSSSOClient(executor CommandExecutor) (*RealAWSSSOClient, error) {

	credentialsClient := NewRealAWSCredentialsClient(&RealAWSConfigClient{Executor: executor}, executor)

	return &RealAWSSSOClient{
		CredentialsClient: credentialsClient,
		Executor:          executor,
		getHomeDir:        os.UserHomeDir,
	}, nil
}

func getSsoAccessTokenFromCache(profile string, client *RealAWSSSOClient) (*models.SSOCache, time.Time, error) {
	sessionName, err := client.getSessionName(profile)
	if err != nil {
		return nil, time.Time{}, err
	}

	homeDir, err := client.getHomeDir()
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("failed to get user home directory: %v", err)
	}
	cacheDir := filepath.Join(homeDir, ".aws", "sso", "cache")

	files, err := os.ReadDir(cacheDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, time.Time{}, fmt.Errorf("no matching SSO cache file found")
		}
		return nil, time.Time{}, fmt.Errorf("failed to read SSO cache directory: %v", err)
	}

	var selectedCache *models.SSOCache
	var latestModTime time.Time

	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".json") {
			cacheFilePath := filepath.Join(cacheDir, file.Name())

			fileInfo, err := os.Stat(cacheFilePath)
			if err != nil {
				continue
			}

			cacheFile, err := os.Open(cacheFilePath)
			if err != nil {
				continue
			}
			defer func() {
				if err := cacheFile.Close(); err != nil {
					fmt.Fprintf(os.Stderr, "Error closing cache file: %v\n", err)
				}
			}()

			var cache models.SSOCache
			if err := json.NewDecoder(cacheFile).Decode(&cache); err != nil {
				continue
			}

			if (cache.SessionName != nil && *cache.SessionName == sessionName) || (cache.StartURL != nil && strings.TrimSuffix(*cache.StartURL, "#") == sessionName) {
				if fileInfo.ModTime().After(latestModTime) {
					latestModTime = fileInfo.ModTime()
					selectedCache = &cache
				}
			}
		}
	}

	if selectedCache == nil {
		return nil, time.Time{}, fmt.Errorf("no matching SSO cache file found")
	}

	var expiryTime time.Time
	if selectedCache.ExpiresAt != nil {
		expiryTime, err = time.Parse(time.RFC3339, *selectedCache.ExpiresAt)
		if err != nil {
			return nil, time.Time{}, fmt.Errorf("invalid expiration time format: %v", err)
		}
	}

	if expiryTime.Before(time.Now()) {
		fmt.Println("Token Expired. Re-login")
		err := client.SSOLogin(profile, true, false)
		if err != nil {
			return nil, time.Time{}, fmt.Errorf("SSO login failed: %v", err)
		}

		return getSsoAccessTokenFromCache(profile, client)
	}

	return selectedCache, expiryTime, nil
}

func (c *RealAWSSSOClient) GetCachedSsoAccessToken(profile string) (string, error) {
	c.TokenCache.Mu.Lock()
	defer c.TokenCache.Mu.Unlock()

	if c.TokenCache.AccessToken != "" && time.Now().Before(c.TokenCache.Expiry) {
		return c.TokenCache.AccessToken, nil
	}

	cachedSSO, expiry, err := getSsoAccessTokenFromCache(profile, c)
	if err != nil {
		return "", err
	}

	if cachedSSO.AccessToken == nil {
		return "", fmt.Errorf("no access token found in cache")
	}

	c.TokenCache.AccessToken = *cachedSSO.AccessToken
	c.TokenCache.Expiry = expiry

	return *cachedSSO.AccessToken, nil
}

func (c *RealAWSSSOClient) ConfigureSSO() error {
	err := c.Executor.RunInteractiveCommand(context.Background(), "aws", "configure", "sso")
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			if status, ok := exitError.Sys().(syscall.WaitStatus); ok {
				if status.ExitStatus() == 130 {
					fmt.Println("\nProcess terminated by user.")
					return promptUtils.ErrInterrupted
				}
			}
		}
		return fmt.Errorf("failed to configure AWS SSO: %v", err)
	}
	return nil
}

func (c *RealAWSSSOClient) GetSSOProfiles() ([]string, error) {

	homeDir, err := c.getHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get user home directory: %v", err)
	}
	configPath := filepath.Join(homeDir, ".aws", "config")

	file, err := os.Open(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open AWS config file: %v", err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "Error closing file: %v\n", err)
		}
	}()

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
	accessToken, err := c.GetCachedSsoAccessToken(profile)
	if err != nil {
		return "", fmt.Errorf("failed to retrieve SSO access token: %v", err)
	}

	output, err := c.Executor.RunCommand("aws", "sso", "list-accounts", "--access-token", accessToken, "--output", "json")
	if err != nil {
		return "", fmt.Errorf("failed to list AWS accounts: %v", err)
	}

	var response models.AccountNameResponse

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
	accessToken, err := c.GetCachedSsoAccessToken(profile)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve SSO access token: %v", err)
	}
	output, err := c.Executor.RunCommand("aws", "sso", "list-account-roles", "--profile", profile, "--account-id", accountID, "--access-token", accessToken, "--output", "json")
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
		if c.Executor == nil {
			return fmt.Errorf("executor is not initialized")
		}
		err := c.Executor.RunInteractiveCommand(ctx, "aws", args...)
		if err != nil {
			if ctx.Err() == context.DeadlineExceeded {
				return fmt.Errorf("SSO login timed out: the login flow was canceled or not completed")
			}
			if exitErr, ok := err.(*exec.ExitError); ok {
				return fmt.Errorf("SSO login failed: %v", exitErr)
			}
			return fmt.Errorf("error during SSO login: %v", err)
		}
	}
	return nil
}

func (c *RealAWSSSOClient) getSessionName(profile string) (string, error) {

	if c.getHomeDir == nil {
		return "", fmt.Errorf("home dir  is nil in RealAWSSSOClient")
	}
	homeDir, err := c.getHomeDir()
	if err != nil {
		return "", fmt.Errorf("unable to find home directory: %v", err)
	}

	configFilePath := fmt.Sprintf("%s/.aws/config", homeDir)

	cfg, err := ini.Load(configFilePath)
	if err != nil {
		return "", fmt.Errorf("unable to load AWS config file: %v", err)
	}

	sectionName := fmt.Sprintf("profile %s", profile)
	section, err := cfg.GetSection(sectionName)
	if err != nil {
		return "", fmt.Errorf("profile '%s' not found in the config file", profile)
	}

	sessionName := section.Key("sso_session").String()

	if sessionName != "" {
		ssoSessionSectionName := fmt.Sprintf("sso-session %s", sessionName)
		ssoSessionSection, err := cfg.GetSection(ssoSessionSectionName)
		if err != nil {
			return "", fmt.Errorf("sso-session section '%s' not found in the config file", ssoSessionSectionName)
		}

		ssoStartURL := ssoSessionSection.Key("sso_start_url").String()
		if ssoStartURL == "" {
			return "", fmt.Errorf("sso_start_url not found in sso-session section '%s'", ssoSessionSectionName)
		}

		return ssoStartURL, nil
	}

	startURL := section.Key("sso_start_url").String()
	if startURL == "" {
		return "", fmt.Errorf("no sso_session or sso_start_url found for profile '%s'", profile)
	}

	return startURL, nil
}
