package aws

import (
	"awsctl/utils"
	"fmt"
	"os"
	"os/exec"
)

// Run AWS SSO login and fetch credentials
func SsoRun(refresh bool, noBrowser bool) error {

	awsProfile := os.Getenv("AWS_PROFILE")

	if awsProfile == "" {
		awsProfile = utils.PromptForProfile()
	}

	// Verify Profiles
	validProfiles, err := utils.ValidProfiles()
	if err != nil {
		return fmt.Errorf("error verifying profiles: %w", err)
	}

	if !utils.Contains(validProfiles, awsProfile) {
		return fmt.Errorf("not a valid profile: %s", os.Getenv("AWS_PROFILE"))
	}

	// Perform SSO Login
	if err := ssoLogin(awsProfile, refresh, noBrowser); err != nil {
		return fmt.Errorf("error during SSO login: %w", err)
	}

	// Step 3: Retrieve Role Name & Account ID from AWS config
	roleName, err := utils.AwsConfigureGet("sso_role_name", awsProfile)
	if err != nil {
		return err
	}
	accountID, err := utils.AwsConfigureGet("sso_account_id", awsProfile)
	if err != nil {
		return err
	}

	// Step 4: Read AWS SSO Access Token
	accessToken, err := utils.GetSsoAccessTokenFromCache()
	if err != nil {
		return err
	}

	// Step 5: Fetch AWS Credentials using SSO
	creds, err := utils.GetRoleCredentials(accessToken, roleName, accountID)
	if err != nil {
		return err
	}

	// Step 6: Save credentials in AWS CLI awsProfiles
	utils.SaveAWSCredentials(awsProfile, creds)
	utils.SaveAWSCredentials("default", creds)

	// Step 7: Get role ARN
	roleARN, err := utils.AwsSTSGetCallerIdentity(awsProfile)
	if err != nil {
		return err
	}

	// Step 8: Print Role Details
	utils.PrintCurrentRole(awsProfile, accountID, roleName, roleARN, creds.Expiration)

	return nil
}

// ssoLogin performs AWS SSO login if necessary
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
