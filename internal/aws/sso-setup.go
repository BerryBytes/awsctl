package aws

import (
	"awsctl/internal/config"
	"awsctl/utils"
	"fmt"
)

// SetupSSO guides the user through setting up AWS SSO and config file creation
func SetupSSO() error {
	handleSignals()

	configPath, err := config.FindConfigFile()
	if err != nil {
		return setupNewConfiguration()
	}

	return updateCustomConfiguration(configPath)
}

// handleManualSetup handles the case where no config file is found
func setupNewConfiguration() error {
	fmt.Println("No custom configuration found on the project root directory. Please provide the following details:")

	// Step 1: Prompt user for AWS account and role
	account, err := utils.PromptForAccount()
	if err != nil {
		return abortSetup(err)
	}

	role, err := utils.PromptForRole()
	if err != nil {
		return abortSetup(err)
	}

	// Step 2: Map account name to AWS account ID
	accountID := utils.GetAccountID(account)
	if accountID == "" {
		return fmt.Errorf("unknown account selected: %s", account)
	}

	// Step 3: Prompt for AWS region
	region, err := utils.PromptForRegion()
	if err != nil {
		return abortSetup(err)
	}

	// Step 4: Configure AWS profiles
	profile := fmt.Sprintf("sso-%s-%s", account, role)
	if err := configureDefaultProfile(region); err != nil {
		return err
	}
	if err := configureSSOProfile(profile, region, accountID, role); err != nil {
		return err
	}

	return nil
}

// handles custom config in the case where an existing config file is found on ~/.config/aws/
func updateCustomConfiguration(configPath string) error {
	fmt.Printf("📂 Loaded existing configuration from '%s':\n", configPath)

	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	fmt.Printf("Profile: %s\n", cfg.Aws.Profile)
	fmt.Printf("Region: %s\n", cfg.Aws.Region)
	fmt.Printf("Account ID: %s\n", cfg.Aws.AccountID)
	fmt.Printf("Role: %s\n", cfg.Aws.Role)
	fmt.Printf("SsoStartUrl: %s\n", cfg.Aws.SsoStartUrl)

	// Validate account ID
	account := utils.GetAccountName(cfg.Aws.AccountID)
	if account == "" {
		return fmt.Errorf("unknown accountID: %s", cfg.Aws.AccountID)
	}

	// Configure AWS profiles
	if err := configureDefaultProfile(cfg.Aws.Region); err != nil {
		return err
	}

	profile := fmt.Sprintf("sso-%s-%s", account, cfg.Aws.Role)
	if err := configureSSOProfile(profile, cfg.Aws.Region, cfg.Aws.AccountID, cfg.Aws.Role); err != nil {
		return err
	}

	fmt.Printf("✅ Configuration updated and saved to ~/.aws/config.")
	return nil
}

// configures the default AWS profile
func configureDefaultProfile(region string) error {
	if err := utils.AwsConfigureSet("region", region, "default"); err != nil {
		fmt.Println("Error setting region:", err)
		return err
	}
	if err := utils.AwsConfigureSet("output", "json", "default"); err != nil {
		fmt.Println("Error setting output format:", err)
		return err
	}
	return nil
}

// configures the AWS SSO profile
func configureSSOProfile(profile, region, accountID, role string) error {
	configs := map[string]string{
		"sso_region":     region,
		"sso_account_id": accountID,
		"sso_start_url":  "https://osm.awsapps.com/start",
		"sso_role_name":  role,
		"region":         region,
		"output":         "json",
	}

	for key, value := range configs {
		if err := utils.AwsConfigureSet(key, value, profile); err != nil {
			fmt.Printf("Error setting %s: %v\n", key, err)
			return err
		}
	}
	return nil
}

// handles setup abortion
func abortSetup(err error) error {
	fmt.Println("Setup aborted. No changes made.")
	return err
}
