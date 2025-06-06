package sso

import (
	"errors"
	"fmt"
	"slices"
	"time"

	promptUtils "github.com/BerryBytes/awsctl/utils/prompt"
)

func (c *RealSSOClient) SetupSSO() error {
	fmt.Println("AWS SSO Configuration Tool")
	fmt.Println("-------------------------")

	_, ssoSession, err := c.loadOrCreateSession()
	if err != nil {
		if errors.Is(err, promptUtils.ErrInterrupted) {
			return nil
		}
		return fmt.Errorf("failed to load or create session: %w", err)
	}

	if err := c.configureSSOSession(ssoSession.Name, ssoSession.StartURL, ssoSession.Region, ssoSession.Scopes); err != nil {
		return fmt.Errorf("failed to configure SSO session: %w", err)
	}

	if err := c.runSSOLogin(ssoSession.Name); err != nil {
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

	profileName, region, err := c.promptProfileDetails(ssoSession.Region)
	if err != nil {
		if errors.Is(err, promptUtils.ErrInterrupted) {
			return nil
		}
		return fmt.Errorf("failed to prompt profile details: %w", err)
	}

	if err := c.configureAWSProfile(profileName, ssoSession.Name, ssoSession.Region, ssoSession.StartURL, accountID, role, region); err != nil {
		return fmt.Errorf("failed to configure AWS profile: %w", err)
	}

	defaultConfigured := profileName == "default"
	if !defaultConfigured {
		setDefault, err := c.Prompter.PromptYesNo("Set this as the default profile? [Y/n]", true)
		if err != nil {
			if errors.Is(err, promptUtils.ErrInterrupted) {
				return nil
			}
			return fmt.Errorf("failed to prompt for default profile: %w", err)
		}
		if setDefault {
			if err := c.configureAWSProfile("default", ssoSession.Name, ssoSession.Region, ssoSession.StartURL, accountID, role, region); err != nil {
				return fmt.Errorf("failed to configure AWS default profile: %w", err)
			}
			defaultConfigured = true
		}
	}

	printSummary(profileName, ssoSession.Name, ssoSession.StartURL, ssoSession.Region, accountID, role, "", "", "")
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
		if len(profiles) == 0 {
			fmt.Println("No profiles found. Configuring SSO...")
			if err := c.SetupSSO(); err != nil {
				if errors.Is(err, promptUtils.ErrInterrupted) {
					return promptUtils.ErrInterrupted
				}
				return fmt.Errorf("failed to set up SSO: %w", err)
			}
			profiles, err = c.ValidProfiles()
			if err != nil {
				return fmt.Errorf("failed to verify profiles after setup: %w", err)
			}
		}

		if len(profiles) == 0 {
			return fmt.Errorf("no valid profiles found after setup")
		}

		awsProfile, err = c.Prompter.SelectFromList("Select AWS profile", profiles)
		if err != nil {
			return fmt.Errorf("failed to select profile: %w", err)
		}
	}

	if !slices.Contains(profiles, awsProfile) {
		return fmt.Errorf("invalid profile: %s", awsProfile)
	}

	var expiration string
	_, expiry, err := c.GetCachedSsoAccessToken(awsProfile)
	if err != nil {
		fmt.Printf("SSO token expired or missing for profile %s. Logging in...\n", awsProfile)
		if err := c.SSOLogin(awsProfile, refresh, noBrowser); err != nil {
			return fmt.Errorf("failed to login: %w", err)
		}
		_, expiry, err = c.GetCachedSsoAccessToken(awsProfile)
		if err != nil {
			return fmt.Errorf("failed to get SSO token after login: %w", err)
		}
	}
	if !expiry.IsZero() {
		expiration = expiry.Format(time.RFC3339)
	}
	fmt.Printf("SSO token validated for profile %s\n", awsProfile)

	sessionName, err := c.ConfigureGet("sso_session", awsProfile)
	if err != nil {
		return fmt.Errorf("failed to get sso_session: %w", err)
	}

	ssoStartURL, err := c.ConfigureGet("sso_start_url", awsProfile)
	if err != nil {
		return fmt.Errorf("failed to get sso_start_url: %w", err)
	}

	ssoRegion, err := c.ConfigureGet("sso_region", awsProfile)
	if err != nil {
		return fmt.Errorf("failed to get sso_region: %w", err)
	}

	accountID, err := c.ConfigureGet("sso_account_id", awsProfile)
	if err != nil {
		return fmt.Errorf("failed to get account ID: %w", err)
	}

	roleName, err := c.ConfigureGet("sso_role_name", awsProfile)
	if err != nil {
		return fmt.Errorf("failed to get role name: %w", err)
	}

	roleARN, err := c.AwsSTSGetCallerIdentity(awsProfile)
	if err != nil {
		return fmt.Errorf("failed to get role ARN: %w", err)
	}

	accountName, err := c.GetSSOAccountName(accountID, awsProfile)
	if err != nil {
		accountName = "Unknown"
		fmt.Printf("Warning: Failed to get account name: %v\n", err)
	}

	printSummary(awsProfile, sessionName, ssoStartURL, ssoRegion, accountID, roleName, accountName, roleARN, expiration)
	return nil
}
