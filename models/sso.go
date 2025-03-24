package models

// SSOAccount represents an AWS account in an SSO profile.
type SSOAccount struct {
	AccountID   string   `json:"accountId" yaml:"accountId"`
	AccountName string   `json:"accountName" yaml:"accountName"`
	SSORegion   string   `json:"ssoRegion,omitempty" yaml:"ssoRegion,omitempty"`
	Email       string   `json:"emailAddress" yaml:"emailAddress"`
	Roles       []string `json:"roles,omitempty" yaml:"roles,omitempty"`
}

// SSOProfile represents a profile configuration, which can contain multiple accounts.
type SSOProfile struct {
	ProfileName string       `json:"profileName" yaml:"profileName"`
	Region      string       `json:"region" yaml:"region"`
	AccountID   string       `json:"accountId" yaml:"accountId"`
	Role        string       `json:"role" yaml:"role"`
	SsoStartUrl string       `json:"ssoStartUrl" yaml:"ssoStartUrl"`
	Accounts    []SSOAccount `json:"accountList" yaml:"accountList"`
}

// Config represents the root configuration containing all profiles.
type Config struct {
	Aws struct {
		Profiles []SSOProfile `json:"profiles" yaml:"profiles"`
	} `json:"aws" yaml:"aws"`
}
