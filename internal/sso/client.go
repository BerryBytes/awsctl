package sso

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/BerryBytes/awsctl/internal/sso/config"
	"github.com/BerryBytes/awsctl/models"
	"github.com/BerryBytes/awsctl/utils/common"
	promptUtils "github.com/BerryBytes/awsctl/utils/prompt"
)

type RealSSOClient struct {
	TokenCache models.TokenCache
	Config     config.Config
	Prompter   Prompter
	Executor   common.CommandExecutor
}

func NewSSOClient(prompter Prompter, executor common.CommandExecutor) (SSOClient, error) {
	if prompter == nil {
		return nil, fmt.Errorf("prompter cannot be nil")
	}
	if executor == nil {
		executor = &common.RealCommandExecutor{}
	}
	cfg, err := config.NewConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}
	return &RealSSOClient{
		Config:   *cfg,
		Prompter: prompter,
		Executor: executor,
	}, nil
}

func (c *RealSSOClient) getSsoAccessTokenFromCache(profile string) (*models.SSOCache, time.Time, error) {
	startURL, err := c.ConfigureGet("sso_start_url", profile)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("failed to get sso_start_url for profile %s: %v", profile, err)
	}
	startURL = strings.TrimSuffix(startURL, "#")

	homeDir, err := os.UserHomeDir()
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

			data, err := os.ReadFile(cacheFilePath)
			if err != nil {
				continue
			}

			var cache models.SSOCache
			if err := json.Unmarshal(data, &cache); err != nil {
				continue
			}

			if cache.StartURL != nil && strings.TrimSuffix(*cache.StartURL, "#") == startURL {
				if fileInfo.ModTime().After(latestModTime) {
					latestModTime = fileInfo.ModTime()
					selectedCache = &cache
				}
			}
		}
	}

	if selectedCache == nil {
		return nil, time.Time{}, fmt.Errorf("no matching SSO cache file found for profile %s, start URL %s", profile, startURL)
	}

	var expiryTime time.Time
	if selectedCache.ExpiresAt != nil {
		expiryTime, err = time.Parse(time.RFC3339, *selectedCache.ExpiresAt)
		if err != nil {
			return nil, time.Time{}, fmt.Errorf("invalid expiration time format: %v", err)
		}
	}

	if expiryTime.Before(time.Now()) {
		fmt.Println("Token expired. Re-login required.")
		err := c.SSOLogin(profile, true, false)
		if err != nil {
			return nil, time.Time{}, fmt.Errorf("SSO login failed: %v", err)
		}
		return c.getSsoAccessTokenFromCache(profile)
	}

	return selectedCache, expiryTime, nil
}

func (c *RealSSOClient) GetCachedSsoAccessToken(profile string) (string, time.Time, error) {
	c.TokenCache.Mu.Lock()
	defer c.TokenCache.Mu.Unlock()

	if c.TokenCache.AccessToken != "" && time.Now().Before(c.TokenCache.Expiry) {
		return c.TokenCache.AccessToken, c.TokenCache.Expiry, nil
	}

	cachedSSO, expiry, err := c.getSsoAccessTokenFromCache(profile)
	if err != nil {
		return "", time.Time{}, err
	}

	if cachedSSO.AccessToken == nil {
		return "", time.Time{}, fmt.Errorf("no access token found in cache for profile %s", profile)
	}

	c.TokenCache.AccessToken = *cachedSSO.AccessToken
	c.TokenCache.Expiry = expiry

	return *cachedSSO.AccessToken, expiry, nil
}

func (c *RealSSOClient) GetSSOAccountName(accountID, profile string) (string, error) {
	accessToken, _, err := c.GetCachedSsoAccessToken(profile)
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

func (c *RealSSOClient) SSOLogin(awsProfile string, refresh, noBrowser bool) error {
	args := []string{"sso", "login"}
	if noBrowser {
		args = append(args, "--no-browser")
	}
	args = append(args, "--profile", awsProfile)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := c.Executor.RunInteractiveCommand(ctx, "aws", args...)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("SSO login timed out: the login flow was canceled or not completed")
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok && status.ExitStatus() == 130 {
				return promptUtils.ErrInterrupted
			}
			return fmt.Errorf("SSO login failed: %v", exitErr)
		}
		return fmt.Errorf("error during SSO login: %v", err)
	}
	return nil
}

func (c *RealSSOClient) GetRoleCredentials(accessToken, roleName, accountID string) (*models.AWSCredentials, error) {
	output, err := c.Executor.RunCommand("aws", "sso", "get-role-credentials",
		"--access-token", accessToken,
		"--role-name", roleName,
		"--account-id", accountID)
	if err != nil {
		return nil, fmt.Errorf("failed to get role credentials: %w, output: %s", err, string(output))
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

func (c *RealSSOClient) AwsSTSGetCallerIdentity(profile string) (string, error) {
	output, err := c.Executor.RunCommand("aws", "sts", "get-caller-identity", "--profile", profile)
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
