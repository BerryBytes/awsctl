package aws

import (
	"awsctl/internal/config"
	"awsctl/models"
	"awsctl/utils"
	"fmt"

	"github.com/manifoldco/promptui"
)

// SetupSSO guides the user through setting up AWS SSO and config file creation
func SetupSSO() error {
	handleSignals()

	configPath, err := config.FindConfigFile()
	if err != nil {
		return setupNewConfiguration()
	}

	// Prompt the user to decide whether to use the existing custom configuration or set up a new one
	prompt := promptui.Select{
		Label: "Configuration Found! How would you like to proceed?",
		Items: []string{"Use custom configuration from $HOME/.config/aws", "Set up a new configuration"},
	}

	_, userChoice, err := prompt.Run()
	if err != nil {
		return utils.AbortSetup(fmt.Errorf("failed to select an option: %v", err))
	}

	if userChoice == "Set up a new configuration" {
		return setupNewConfiguration()
	}

	return updateCustomConfiguration(configPath)
}

// setupNewConfiguration handles the case where no config file is found
func setupNewConfiguration() error {
	profiles, err := utils.GetSSOProfiles()
	if err != nil || len(profiles) == 0 {
		// If no profiles found, prompt for SSO configuration
		fmt.Println("No AWS SSO profiles found. Configuring SSO...")

		// Configure AWS SSO
		if err := utils.ConfigureSSO(); err != nil {
			return utils.AbortSetup(fmt.Errorf("failed to configure AWS SSO: %v", err))
		}

		// Refresh profiles after configuration
		profiles, err = utils.GetSSOProfiles()
		if err != nil {
			return utils.AbortSetup(fmt.Errorf("failed to refresh AWS SSO profiles: %v", err))
		}
	}
	// Let user select a profile
	profile, err := utils.SelectProfile(profiles)
	if err != nil {
		return utils.AbortSetup(fmt.Errorf("profile selection aborted: %v", err))
	}
	// Get region for selected profile
	region, err := utils.GetAWSRegion(profile)
	if err != nil {
		return utils.AbortSetup(fmt.Errorf("failed to get AWS Region: %v", err))
	}

	if err := utils.ConfigureDefaultProfile(region); err != nil {
		return utils.AbortSetup(fmt.Errorf("failed to configure default profile: %v", err))
	}

	return nil
}

// handles custom config in the case where an existing config file is found on ~/.config/aws/

func updateCustomConfiguration(configPath string) error {
	fmt.Printf("📂 Loaded existing configuration from '%s':\n", configPath)
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %v", err)
	}
	profile, err := selectProfile(cfg)
	if err != nil {
		return err
	}

	account, err := selectAccount(profile)
	if err != nil {
		return err
	}

	role, err := selectRole(account)
	if err != nil {
		return err
	}

	if err := configureProfile(profile, account, role); err != nil {
		return err
	}

	return nil
}

// Get profiles from custom config and prompt for selection
func selectProfile(cfg *models.Config) (*models.SSOProfile, error) {
	profiles, err := utils.GetUniqueProfiles(cfg)
	if err != nil {
		return nil, err
	}

	selectedProfile, err := utils.PromptForSelection("Select AWS Profile", profiles)
	if err != nil {
		return nil, err
	}

	selectedProfileObj, err := utils.FindProfile(cfg, selectedProfile)
	if err != nil {
		return nil, err
	}

	return selectedProfileObj, nil
}

// Retrieves and displays the available AWS accounts for the given profile
func selectAccount(profile *models.SSOProfile) (*models.SSOAccount, error) {
	accounts := utils.ExtractAccountNames(profile)
	selectedAccount, err := utils.PromptForSelection("Select AWS Account", accounts)
	if err != nil {
		return nil, err
	}

	selectedAccountObj, err := utils.FindAccount(profile, selectedAccount)
	if err != nil {
		return nil, err
	}

	return selectedAccountObj, nil
}

// Retrieves and displays the available role on the selected account
func selectRole(account *models.SSOAccount) (string, error) {
	return utils.SelectRole(account.Roles)
}

// configure sso profile based on the custom config
func configureProfile(profile *models.SSOProfile, account *models.SSOAccount, role string) error {
	fmt.Printf("Selected Profile: %s\n", profile.ProfileName)
	fmt.Printf("Selected Account: %s\n", account.AccountName)
	fmt.Printf("Selected Role: %s\n", role)

	if err := utils.ConfigureDefaultProfile(profile.Region); err != nil {
		return err
	}

	ssoProfile := fmt.Sprintf("sso-%s-%s", account.AccountName, profile.Role)
	if err := utils.ConfigureSSOProfile(
		ssoProfile,
		profile.Region,
		account.AccountID,
		role,
		profile.SsoStartUrl,
	); err != nil {
		return err
	}

	fmt.Printf("Successfully configured profile: %s\n", ssoProfile)
	return nil
}
