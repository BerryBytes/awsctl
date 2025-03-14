package aws

import (
	"awsctl/utils"
	"fmt"
	"os"
	"os/exec"
)

// Run AWS SSO login and fetch credentials
func SsoInit(refresh bool, noBrowser bool) error {

	awsProfile := os.Getenv("AWS_PROFILE")

	if awsProfile == "" {
		awsProfile = utils.PromptForProfile()
	}

	// If still no profile, run the SSO setup
	if awsProfile == "" {
		fmt.Println("No AWS profile found. Running SSO setup...")
		err := SetupSSO()
		if err != nil {
			return fmt.Errorf("failed to set up AWS SSO: %w", err)
		}

		// After setting up SSO, prompt the user to select a profile again
		awsProfile = utils.PromptForProfile()
	}

	validProfiles, err := utils.ValidProfiles()
	if err != nil {
		return fmt.Errorf("error verifying profiles: %w", err)
	}

	if !utils.Contains(validProfiles, awsProfile) {
		return fmt.Errorf("not a valid profile: %s", os.Getenv("AWS_PROFILE"))
	}

	if err := ssoLogin(awsProfile, refresh, noBrowser); err != nil {
		return fmt.Errorf("error during SSO login: %w", err)
	}

	// Retrieve Role Name & Account ID from AWS config
	roleName, err := utils.AwsConfigureGet("sso_role_name", awsProfile)
	if err != nil {
		return err
	}

	accountID, err := utils.AwsConfigureGet("sso_account_id", awsProfile)
	if err != nil {
		return err
	}

	accessToken, err := utils.GetCachedSsoAccessToken(awsProfile)
	if err != nil {
		return err
	}

	creds, err := utils.GetRoleCredentials(accessToken, roleName, accountID)
	if err != nil {
		return err
	}

	// Save credentials in AWS CLI awsProfiles
	if err := utils.SaveAWSCredentials(awsProfile, creds); err != nil {
		return fmt.Errorf("failed to save credentials for profile %s: %w", awsProfile, err)
	}

	if err := utils.SaveAWSCredentials("default", creds); err != nil {
		return fmt.Errorf("failed to save credentials for default profile: %w", err)
	}

	// Get role ARN
	roleARN, err := utils.AwsSTSGetCallerIdentity(awsProfile)
	if err != nil {
		return err
	}

	accountName, err := utils.GetSSOAccountName(accountID, awsProfile)
	if err != nil {
		return err
	}

	utils.PrintCurrentRole(awsProfile, accountID, accountName, roleName, roleARN, creds.Expiration)

	return nil
}

// Performs AWS SSO login if necessary
func ssoLogin(awsProfile string, refresh, noBrowser bool) error {
	if refresh || !utils.IsCallerIdentityValid(awsProfile) {
		args := []string{"sso", "login"}
		if noBrowser {
			args = append(args, "--no-browser")
		}
		args = append(args, "--profile", awsProfile)
		cmd := exec.Command("aws", args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("error during SSO login: %w", err)
		}
	}
	return nil
}
