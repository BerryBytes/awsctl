package sso

import (
	"errors"
	"fmt"
	"os"
	"slices"

	"github.com/BerryBytes/awsctl/internal/config"
	"github.com/BerryBytes/awsctl/models"
	promptUtils "github.com/BerryBytes/awsctl/utils/prompt"

	"github.com/manifoldco/promptui"
)

type SSOClient interface {
	InitSSO(refresh, noBrowser bool) error
	SetupSSO() error
}

type RealSSOClient struct {
	AWSClient AWSClient
	Config    config.Config
}

func NewSSOClient(awsClient AWSClient) (SSOClient, error) {
	cfg, err := config.NewConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	return &RealSSOClient{
		AWSClient: awsClient,
		Config:    *cfg,
	}, nil
}

func (c *RealSSOClient) InitSSO(refresh, noBrowser bool) error {

	fmt.Println("Initializing AWS SSO...")

	awsProfile := c.Config.AWSProfile
	if awsProfile == "" {
		profiles, err := c.AWSClient.ConfigClient.ValidProfiles()
		if err != nil {
			return fmt.Errorf("error getting profiles list: %w", err)
		}

		if len(profiles) == 0 {
			fmt.Println("No profiles found. Configuring SSO Setup...")
			if err := c.SetupSSO(); err != nil {
				if errors.Is(err, promptUtils.ErrInterrupted) {
					return promptUtils.ErrInterrupted
				}
				return fmt.Errorf("failed to set up AWS SSO: %w", err)
			}

			profiles, err = c.AWSClient.ConfigClient.ValidProfiles()
			if err != nil {
				return fmt.Errorf("error verifying profiles after SSO setup: %w", err)
			}
		}

		selectedProfile, err := c.AWSClient.SelectionClient.SelectProfile(profiles)
		if err != nil {
			if errors.Is(err, promptUtils.ErrInterrupted) {
				return promptUtils.ErrInterrupted
			}
			return fmt.Errorf("profile selection aborted: %v", err)
		}

		awsProfile = selectedProfile
	}

	validProfiles, err := c.AWSClient.ConfigClient.ValidProfiles()
	if err != nil {
		return fmt.Errorf("error verifying profiles: %w", err)
	}

	if !slices.Contains(validProfiles, awsProfile) {
		return fmt.Errorf("not a valid profile: %s", awsProfile)
	}

	if err := c.AWSClient.SSOClient.SSOLogin(awsProfile, refresh, noBrowser); err != nil {
		return fmt.Errorf("error during SSO login: %w", err)
	}

	roleName, err := c.AWSClient.ConfigClient.ConfigureGet("sso_role_name", awsProfile)
	if err != nil {
		return fmt.Errorf("failed to get role name: %w", err)
	}

	accountID, err := c.AWSClient.ConfigClient.ConfigureGet("sso_account_id", awsProfile)
	if err != nil {
		return fmt.Errorf("failed to get account ID: %w", err)
	}

	accessToken, err := c.AWSClient.SSOClient.GetCachedSsoAccessToken(awsProfile)
	if err != nil {
		return fmt.Errorf("failed to get cached SSO access token: %w", err)
	}

	creds, err := c.AWSClient.CredentialsClient.GetRoleCredentials(accessToken, roleName, accountID)
	if err != nil {
		return fmt.Errorf("failed to get role credentials: %w", err)
	}

	if err := c.AWSClient.CredentialsClient.SaveAWSCredentials(awsProfile, creds); err != nil {
		return fmt.Errorf("failed to save credentials for profile %s: %w", awsProfile, err)
	}
	if err := c.AWSClient.CredentialsClient.SaveAWSCredentials("default", creds); err != nil {
		return fmt.Errorf("failed to save credentials for default profile: %w", err)
	}

	roleARN, err := c.AWSClient.CredentialsClient.AwsSTSGetCallerIdentity(awsProfile)
	if err != nil {
		return fmt.Errorf("failed to get caller identity: %w", err)
	}

	accountName, err := c.AWSClient.SSOClient.GetSSOAccountName(accountID, awsProfile)
	if err != nil {
		return fmt.Errorf("failed to get account name: %w", err)
	}

	c.AWSClient.UtilityClient.PrintCurrentRole(awsProfile, accountID, accountName, roleName, roleARN, creds.Expiration)

	return nil
}

func (c *RealSSOClient) SetupSSO() error {

	configPath, err := config.FindConfigFile(&c.Config)

	if err != nil {
		if errors.Is(err, config.ErrNoConfigFile) {
			fmt.Println("No configuration file found. Setting up a new configuration...")
			return c.setupNewConfiguration()
		}
		return err
	}

	fileInfo, err := os.Stat(configPath)
	if err != nil {
		return fmt.Errorf("failed to check config file: %w", err)
	}

	if fileInfo.Size() == 0 {
		fmt.Println("Configuration file is empty. Setting up a new configuration...")
		return c.setupNewConfiguration()
	}

	prompt := promptui.Select{
		Label: "Configuration Found! How would you like to proceed?",
		Items: []string{"Use custom configuration from $HOME/.config/awsctl", "Set up a new configuration"},
	}

	_, userChoice, err := prompt.Run()
	if err != nil {
		if errors.Is(err, promptui.ErrInterrupt) {
			fmt.Println("\nReceived termination signal. Exiting.")
			return promptUtils.ErrInterrupted
		}
		return fmt.Errorf("failed to select an option: %w", err)
	}

	if userChoice == "Set up a new configuration" {
		return c.setupNewConfiguration()
	}

	err = c.updateCustomConfiguration(configPath)
	if errors.Is(err, promptUtils.ErrInterrupted) {
		return promptUtils.ErrInterrupted
	}

	return err
}

func (c *RealSSOClient) setupNewConfiguration() error {
	profiles, err := c.AWSClient.SSOClient.GetSSOProfiles()
	if err != nil || len(profiles) == 0 {
		fmt.Println("No AWS SSO profiles found. Configuring SSO...")

		// Configure AWS SSO interactively
		err := c.AWSClient.SSOClient.ConfigureSSO()
		if err != nil {
			if errors.Is(err, promptUtils.ErrInterrupted) {
				return promptUtils.ErrInterrupted
			}
			return fmt.Errorf("failed to configure AWS SSO: %v", err)
		}

		profiles, err = c.AWSClient.SSOClient.GetSSOProfiles()
		if err != nil {
			return fmt.Errorf("failed to refresh AWS SSO profiles: %v", err)
		}
	}

	profile, err := c.AWSClient.SelectionClient.SelectProfile(profiles)
	if err != nil {
		return fmt.Errorf("profile selection aborted: %v", err)
	}

	region, err := c.AWSClient.ConfigClient.GetAWSRegion(profile)
	if err != nil {
		return fmt.Errorf("failed to get AWS region: %v", err)
	}

	output, err := c.AWSClient.ConfigClient.GetAWSOutput(profile)
	if err != nil {
		return fmt.Errorf("failed to get AWS output format: %v", err)
	}

	if err := c.AWSClient.ConfigClient.ConfigureDefaultProfile(region, output); err != nil {
		return fmt.Errorf("failed to configure default profile: %v", err)
	}

	return nil
}

func (c *RealSSOClient) updateCustomConfiguration(configPath string) error {
	fmt.Printf("Loaded existing configuration from '%s':\n", configPath)

	cfg, err := config.NewConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %v", err)
	}

	profile, err := c.AWSClient.SelectionClient.SelectProfileFromConfig(cfg.RawCustomConfig)
	if err != nil {
		if errors.Is(err, promptUtils.ErrInterrupted) {
			return promptUtils.ErrInterrupted
		}
		return fmt.Errorf("profile selection aborted: %v", err)
	}

	account, err := c.AWSClient.SelectionClient.SelectAccountFromProfile(profile)

	if err != nil {
		if errors.Is(err, promptUtils.ErrInterrupted) {
			return promptUtils.ErrInterrupted
		}
		return fmt.Errorf("failed to select account: %v", err)
	}

	role, err := c.AWSClient.SelectionClient.SelectRoleFromAccount(account)

	if err != nil {
		if errors.Is(err, promptUtils.ErrInterrupted) {
			return promptUtils.ErrInterrupted
		}
		return fmt.Errorf("failed to select role: %v", err)
	}
	if err := c.configureProfile(profile, account, role); err != nil {
		return fmt.Errorf("failed to configure profile: %v", err)
	}

	return nil
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
