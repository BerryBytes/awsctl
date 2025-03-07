package aws

import (
	"awsctl/internal/config"
	"awsctl/utils"
	"fmt"
)

// SetupSSO guides the user through setting up AWS SSO and config file creation
func SetupSSO() error {

	configPath, err := config.FindConfigFile()

	// If no config file is found, prompt for configuration
	if err != nil {
		// If no config file is found, prompt for configuration
		fmt.Println("No custom configuration found on the project root directory. Please provide the following details:")

		// Step 1: Prompt user for AWS account and role
		account := utils.PromptForAccount()
		role := utils.PromptForRole()

		// Step 2: Map account name to AWS account ID
		accountID := utils.GetAccountID(account)
		if accountID == "" {
			return fmt.Errorf("unknown account selected: %s", account)
		}

		// Step 3: Prompt for AWS region
		region := utils.PromptForRegion() // Using the PromptForRegion function

		// Step 4: Configure Default Profile
		err := utils.AwsConfigureSet("region", region, "default")
		if err != nil {
			fmt.Println("Error setting region:", err)
			return err
		}

		err = utils.AwsConfigureSet("output", "json", "default")
		if err != nil {
			fmt.Println("Error setting output format:", err)
			return err
		}

		profile := fmt.Sprintf("sso-%s-%s", account, role)

		err = utils.AwsConfigureSet("sso_region", region, profile)
		if err != nil {
			fmt.Println("Error setting SSO region:", err)
			return err
		}

		err = utils.AwsConfigureSet("sso_account_id", accountID, profile)
		if err != nil {
			fmt.Println("Error setting SSO account ID:", err)
			return err
		}

		err = utils.AwsConfigureSet("sso_start_url", "https://osm.awsapps.com/start", profile)
		if err != nil {
			fmt.Println("Error setting SSO start URL:", err)
			return err
		}

		err = utils.AwsConfigureSet("sso_role_name", role, profile)
		if err != nil {
			fmt.Println("Error setting SSO role name:", err)
			return err
		}

		err = utils.AwsConfigureSet("region", region, profile)
		if err != nil {
			fmt.Println("Error setting region for profile:", err)
			return err
		}

		err = utils.AwsConfigureSet("output", "json", profile)
		if err != nil {
			fmt.Println("Error setting output format for profile:", err)
			return err
		}

	} else {
		fmt.Printf("📂 Loaded existing configuration from '%s':\n", configPath)

		// Load the config from YAML/JSON
		cfg, err := config.LoadConfig()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		fmt.Printf("Profile: %s\n", cfg.Aws.Profile)
		fmt.Printf("Region: %s\n", cfg.Aws.Region)
		fmt.Printf("Account ID: %s\n", cfg.Aws.AccountID)
		fmt.Printf("Role: %s\n", cfg.Aws.Role)
		fmt.Printf("SsoStartUrl: %s\n", cfg.Aws.SsoStartUrl)

		// Check if the provided accountID is valid
		account := utils.GetAccountName(cfg.Aws.AccountID)
		if account == "" {
			return fmt.Errorf("unknown accountID: %s", cfg.Aws.AccountID)
		}

		// Step 4: Configure Default Profile
		err = utils.AwsConfigureSet("region", cfg.Aws.Region, "default")
		if err != nil {
			fmt.Println("Error setting region:", err)
			return err
		}

		err = utils.AwsConfigureSet("output", "json", "default")
		if err != nil {
			fmt.Println("Error setting output format:", err)
			return err
		}
		// Step 4: Generate AWS profile name
		profile := fmt.Sprintf("sso-%s-%s", account, cfg.Aws.Role)

		// Step 5: Configure AWS SSO profile using awsConfigureSet from sso.go
		// utils.AwsConfigureSet("sso_start_url", "https://osm.awsapps.com/start", profile)
		// utils.AwsConfigureSet("sso_region", cfg.Aws.Region, profile)
		// utils.AwsConfigureSet("sso_account_id", cfg.Aws.AccountID, profile)
		// utils.AwsConfigureSet("sso_role_name", cfg.Aws.Role, profile)
		// utils.AwsConfigureSet("region", cfg.Aws.Region, profile)
		// utils.AwsConfigureSet("output", "json", profile)

		err = utils.AwsConfigureSet("sso_region", cfg.Aws.Region, profile)
		if err != nil {
			fmt.Println("Error setting SSO region:", err)
			return err
		}

		err = utils.AwsConfigureSet("sso_account_id", cfg.Aws.AccountID, profile)
		if err != nil {
			fmt.Println("Error setting SSO account ID:", err)
			return err
		}

		err = utils.AwsConfigureSet("sso_start_url", "https://osm.awsapps.com/start", profile)
		if err != nil {
			fmt.Println("Error setting SSO start URL:", err)
			return err
		}

		err = utils.AwsConfigureSet("sso_role_name", cfg.Aws.Role, profile)
		if err != nil {
			fmt.Println("Error setting SSO role name:", err)
			return err
		}

		err = utils.AwsConfigureSet("region", cfg.Aws.Region, profile)
		if err != nil {
			fmt.Println("Error setting region for profile:", err)
			return err
		}

		err = utils.AwsConfigureSet("output", "json", profile)
		if err != nil {
			fmt.Println("Error setting output format for profile:", err)
			return err
		}

		fmt.Printf("✅ Configuration updated and saved to ~/.aws/config.")
	}

	return nil
}
