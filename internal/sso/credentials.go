package sso

import (
	"awsctl/models"
	"encoding/json"
	"fmt"
	"time"
)

type AWSCredentialsClient interface {
	GetRoleCredentials(accessToken, roleName, accountID string) (*models.AWSCredentials, error)
	SaveAWSCredentials(profile string, creds *models.AWSCredentials) error
	IsCallerIdentityValid(profile string) bool
	AwsSTSGetCallerIdentity(profile string) (string, error)
}

type RealAWSCredentialsClient struct {
	configClient AWSConfigClient
	executor     CommandExecutor
}

func NewRealAWSCredentialsClient(configClient AWSConfigClient, executor CommandExecutor) *RealAWSCredentialsClient {
	return &RealAWSCredentialsClient{
		configClient: configClient,
		executor:     executor,
	}
}

func (c *RealAWSCredentialsClient) GetRoleCredentials(accessToken, roleName, accountID string) (*models.AWSCredentials, error) {
	output, err := c.executor.RunCommand("aws", "sso", "get-role-credentials",
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

func (c *RealAWSCredentialsClient) SaveAWSCredentials(profile string, creds *models.AWSCredentials) error {
	saveCredential := func(name, value string) error {
		err := c.configClient.ConfigureSet(name, value, profile)
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

func (c *RealAWSCredentialsClient) IsCallerIdentityValid(profile string) bool {
	output, err := c.executor.RunCommand("aws", "sts", "get-caller-identity", "--profile", profile)
	if err != nil {
		return false
	}

	var identity struct {
		UserID string `json:"UserId"`
	}
	if err := json.Unmarshal(output, &identity); err != nil {
		return false
	}
	return identity.UserID != ""
}

func (c *RealAWSCredentialsClient) AwsSTSGetCallerIdentity(profile string) (string, error) {
	output, err := c.executor.RunCommand("aws", "sts", "get-caller-identity", "--profile", profile)
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
