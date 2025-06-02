package sso

import (
	"time"

	"github.com/BerryBytes/awsctl/models"
)

type SSOClient interface {
	SetupSSO() error
	InitSSO(refresh, noBrowser bool) error
	ConfigureSet(key, value, profile string) error
	ConfigureGet(key, profile string) (string, error)
	ValidProfiles() ([]string, error)
	ConfigureDefaultProfile(region, output string) error
	ConfigureSSOProfile(profile, region, accountID, role, ssoStartUrl, ssoSession string) error
	GetAWSRegion(profile string) (string, error)
	GetAWSOutput(profile string) (string, error)
	GetCachedSsoAccessToken(profile string) (string, time.Time, error)
	GetSSOAccountName(accountID, profile string) (string, error)
	SSOLogin(awsProfile string, refresh, noBrowser bool) error
	GetRoleCredentials(accessToken, roleName, accountID string) (*models.AWSCredentials, error)
	AwsSTSGetCallerIdentity(profile string) (string, error)
}

type Prompter interface {
	PromptWithDefault(label, defaultValue string) (string, error)
	PromptRequired(label string) (string, error)
	SelectFromList(label string, items []string) (string, error)
	PromptYesNo(label string, defaultValue bool) (bool, error)
}

type PromptRunner interface {
	RunPrompt(label, defaultValue string, validate func(string) error) (string, error)
	RunSelect(label string, items []string) (string, error)
}
