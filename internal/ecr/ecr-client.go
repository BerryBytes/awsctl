package ecr

import (
	"context"
	"encoding/base64"
	"fmt"
	"path/filepath"

	"github.com/BerryBytes/awsctl/internal/sso"
	"github.com/BerryBytes/awsctl/utils/common"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
)

type AwsECRAdapter struct {
	Client     ECRAPI
	Cfg        aws.Config
	FileSystem common.FileSystemInterface
	Executor   sso.CommandExecutor
}

func NewECRClient(cfg aws.Config, fs common.FileSystemInterface, executor sso.CommandExecutor) *AwsECRAdapter {
	return &AwsECRAdapter{
		Client:     ecr.NewFromConfig(cfg),
		Cfg:        cfg,
		FileSystem: fs,
		Executor:   executor,
	}
}

func (c *AwsECRAdapter) Login(ctx context.Context) error {
	if _, err := c.Executor.LookPath("docker"); err != nil {
		return fmt.Errorf("docker command not found: %w", err)
	}

	input := &ecr.GetAuthorizationTokenInput{}
	resp, err := c.Client.GetAuthorizationToken(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to get ECR authorization token: %w", err)
	}

	if len(resp.AuthorizationData) == 0 {
		return fmt.Errorf("no authorization data returned")
	}

	auth := resp.AuthorizationData[0]
	token, err := base64.StdEncoding.DecodeString(*auth.AuthorizationToken)
	if err != nil {
		return fmt.Errorf("failed to decode authorization token: %w", err)
	}

	usernamePassword := string(token)
	username := "AWS"
	password := usernamePassword[len(username)+1:]

	registry := *auth.ProxyEndpoint
	args := []string{"login", "--username", username, "--password-stdin", registry}
	output, err := c.Executor.RunCommandWithInput("docker", password, args...)
	if err != nil {
		return fmt.Errorf("failed to execute docker login: %w, output: %s", err, string(output))
	}

	homeDir, err := c.FileSystem.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}
	dockerConfigDir := filepath.Join(homeDir, ".docker")
	if err := c.FileSystem.MkdirAll(dockerConfigDir, 0755); err != nil {
		return fmt.Errorf("failed to create Docker config directory: %w", err)
	}

	return nil
}
