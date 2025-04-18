package sso

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type CommandExecutor interface {
	RunCommand(name string, args ...string) ([]byte, error)
	RunInteractiveCommand(ctx context.Context, name string, args ...string) error
	LookPath(file string) (string, error)
}

type RealCommandExecutor struct{}

func (e *RealCommandExecutor) RunCommand(name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	return cmd.Output()
}

func (e *RealCommandExecutor) RunInteractiveCommand(ctx context.Context, name string, args ...string) error {

	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (e *RealCommandExecutor) LookPath(file string) (string, error) {
	return exec.LookPath(file)
}

type AWSConfigClient interface {
	ConfigureSet(key, value, profile string) error
	ConfigureGet(key, profile string) (string, error)
	ValidProfiles() ([]string, error)
	ConfigureDefaultProfile(region, output string) error
	ConfigureSSOProfile(profile, region, accountID, role, ssoStartUrl string) error
	GetAWSRegion(profile string) (string, error)
	GetAWSOutput(profile string) (string, error)
}

type RealAWSConfigClient struct {
	Executor CommandExecutor
}

func (c *RealAWSConfigClient) ConfigureSet(key, value, profile string) error {
	_, err := c.Executor.RunCommand("aws", "configure", "set", key, value, "--profile", profile)
	return err
}

func (c *RealAWSConfigClient) ConfigureGet(key, profile string) (string, error) {
	output, err := c.Executor.RunCommand("aws", "configure", "get", key, "--profile", profile)
	if err != nil {
		return "", fmt.Errorf("failed to get %s: %w", key, err)
	}
	return strings.TrimSpace(string(output)), nil
}

func (c *RealAWSConfigClient) ValidProfiles() ([]string, error) {
	output, err := c.Executor.RunCommand("aws", "configure", "list-profiles")
	if err != nil {
		return nil, err
	}
	profiles := strings.Split(strings.TrimSpace(string(output)), "\n")

	var validProfiles []string
	for _, profile := range profiles {
		if profile != "" && profile != "default" {
			validProfiles = append(validProfiles, profile)
		}
	}

	return validProfiles, nil
}

func (c *RealAWSConfigClient) ConfigureDefaultProfile(region, output string) error {
	fmt.Println("Default CLI profile added")
	if err := c.ConfigureSet("region", region, "default"); err != nil {
		fmt.Println("Error setting region:", err)
		return err
	}
	if err := c.ConfigureSet("output", output, "default"); err != nil {
		fmt.Println("Error setting output format:", err)
		return err
	}
	return nil
}

func (c *RealAWSConfigClient) ConfigureSSOProfile(profile, region, accountID, role, ssoStartUrl string) error {
	configs := map[string]string{
		"sso_region":     region,
		"sso_account_id": accountID,
		"sso_start_url":  ssoStartUrl,
		"sso_role_name":  role,
		"region":         region,
		"output":         "json",
	}

	for key, value := range configs {
		if err := c.ConfigureSet(key, value, profile); err != nil {
			fmt.Printf("Error setting %s: %v\n", key, err)
			return err
		}
	}
	return nil
}

func (c *RealAWSConfigClient) GetAWSRegion(profile string) (string, error) {
	output, err := c.Executor.RunCommand("aws", "configure", "get", "region", "--profile", profile)
	if err != nil {
		return "", fmt.Errorf("failed to get AWS region: %v", err)
	}

	region := strings.TrimSpace(string(output))
	if region == "" {
		return "", fmt.Errorf("AWS region not found in profile %s", profile)
	}

	return region, nil
}

func (c *RealAWSConfigClient) GetAWSOutput(profile string) (string, error) {
	output, err := c.Executor.RunCommand("aws", "configure", "get", "output", "--profile", profile)
	if err != nil {
		return "", fmt.Errorf("failed to get AWS output: %v", err)
	}

	outputFormat := strings.TrimSpace(string(output))
	if outputFormat == "" {
		return "", fmt.Errorf("AWS output not found in profile %s", profile)
	}

	return outputFormat, nil
}
