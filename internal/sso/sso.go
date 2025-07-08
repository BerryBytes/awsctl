package sso

import (
	"errors"
	"fmt"
	"slices"

	promptUtils "github.com/BerryBytes/awsctl/utils/prompt"
)

func (c *RealSSOClient) SetupSSO(opts SSOFlagOptions) error {
	fmt.Println("AWS SSO Configuration Tool")
	fmt.Println("-------------------------")

	_, ssoSession, err := c.LoadOrCreateSession(opts.Name, opts.StartURL, opts.Region)
	if err != nil {
		if errors.Is(err, promptUtils.ErrInterrupted) {
			return nil
		}
		return fmt.Errorf("failed to load or create session: %w", err)
	}

	if err := c.ConfigureSSOSession(ssoSession.Name, ssoSession.StartURL, ssoSession.Region, ssoSession.Scopes); err != nil {
		return fmt.Errorf("failed to configure SSO session: %w", err)
	}

	if err := c.RunSSOLogin(ssoSession.Name); err != nil {
		return fmt.Errorf("failed to run SSO login: %w", err)
	}

	accountID, err := c.selectAccount(ssoSession.Region, ssoSession.StartURL)
	if err != nil {
		if errors.Is(err, promptUtils.ErrInterrupted) {
			return nil
		}
		return fmt.Errorf("failed to select account: %w", err)
	}

	role, err := c.selectRole(ssoSession.Region, ssoSession.StartURL, accountID)
	if err != nil {
		if errors.Is(err, promptUtils.ErrInterrupted) {
			return nil
		}
		return fmt.Errorf("failed to select role: %w", err)
	}

	profileName := ssoSession.Name + "-profile"

	if err := c.ConfigureAWSProfile(profileName, ssoSession.Name, ssoSession.Region, ssoSession.StartURL, accountID, role, ssoSession.Region); err != nil {
		return fmt.Errorf("failed to configure AWS profile: %w", err)
	}

	defaultConfigured := profileName == "default"

	if !defaultConfigured {
		if err := c.ConfigureAWSProfile("default", ssoSession.Name, ssoSession.Region, ssoSession.StartURL, accountID, role, ssoSession.Region); err != nil {
			return fmt.Errorf("failed to configure AWS default profile: %w", err)
		}
		defaultConfigured = true
	}

	PrintSummary(profileName, ssoSession.Name, ssoSession.StartURL, ssoSession.Region, accountID, role, "", "", "")
	fmt.Printf("\nSuccessfully configured AWS profile '%s'!\n", profileName)

	if defaultConfigured {
		fmt.Println("You can now use AWS CLI commands without specifying --profile")
	} else {
		fmt.Printf("You can now use this profile with AWS CLI commands using: --profile %s\n", profileName)
	}

	return nil
}

func (c *RealSSOClient) InitSSO(refresh, noBrowser bool) error {
	fmt.Println("Initializing AWS SSO...")

	profiles, err := c.ValidProfiles()
	if err != nil {
		return fmt.Errorf("failed to get profiles list: %w", err)
	}

	awsProfile := c.Config.AWSProfile
	if awsProfile == "" {
		// if len(profiles) == 0 {
		// 	fmt.Println("No profiles found. Configuring SSO...")
		// 	if err := c.SetupSSO(); err != nil {
		// 		if errors.Is(err, promptUtils.ErrInterrupted) {
		// 			return promptUtils.ErrInterrupted
		// 		}
		// 		return fmt.Errorf("failed to set up SSO: %w", err)
		// 	}
		// 	return nil
		// }

		awsProfile, err = c.Prompter.SelectFromList("Select AWS profile", profiles)
		if err != nil {
			return fmt.Errorf("failed to select profile: %w", err)
		}

		if awsProfile != "default" {
			if err := c.setProfileAsDefault(awsProfile); err != nil {
				return err
			}
		}
	}

	if !slices.Contains(profiles, awsProfile) {
		return fmt.Errorf("invalid profile: %s", awsProfile)
	}

	if _, _, err := c.GetCachedSsoAccessToken(awsProfile); err != nil {
		fmt.Printf("SSO token expired or missing for profile %s. Logging in...\n", awsProfile)
		if err := c.SSOLogin(awsProfile, refresh, noBrowser); err != nil {
			return fmt.Errorf("failed to login: %w", err)
		}
		if _, _, err = c.GetCachedSsoAccessToken(awsProfile); err != nil {
			return fmt.Errorf("failed to get SSO token after login: %w", err)
		}
	}
	fmt.Printf("SSO token validated for profile %s\n", awsProfile)

	return c.printProfileSummary(awsProfile)
}
