package sso

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func writeConfigFile(path, content string) error {
	tmpFile, err := os.CreateTemp(filepath.Dir(path), "aws-config-*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer func() {
		if err := os.Remove(tmpFile.Name()); err != nil {
			fmt.Printf("failed to remove temp file: %v\n", err)
		}
	}()

	if _, err := tmpFile.WriteString(content); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}
	if err := tmpFile.Chmod(0600); err != nil {
		return fmt.Errorf("failed to set permissions: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}
	return os.Rename(tmpFile.Name(), path)
}

func (c *RealSSOClient) ConfigureSet(key, value, profile string) error {
	_, err := c.Executor.RunCommand("aws", "configure", "set", key, value, "--profile", profile)
	return err
}

func (c *RealSSOClient) ConfigureGet(key, profile string) (string, error) {
	output, err := c.Executor.RunCommand("aws", "configure", "get", key, "--profile", profile)
	if err != nil {
		return "", fmt.Errorf("failed to get %s: %w", key, err)
	}
	return strings.TrimSpace(string(output)), nil
}

func (c *RealSSOClient) ValidProfiles() ([]string, error) {
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

func (c *RealSSOClient) ConfigureDefaultProfile(region, output string) error {
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

func (c *RealSSOClient) GetAWSRegion(profile string) (string, error) {
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

func (c *RealSSOClient) GetAWSOutput(profile string) (string, error) {
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
