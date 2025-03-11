package models

// AWSCredentials holds the credentials returned by AWS SSO
type AWSCredentials struct {
	AccessKeyID     string
	SecretAccessKey string
	SessionToken    string
	Expiration      string
}

// RoleCredentialsResponse is the structure to parse the role credentials response from AWS SSO
type RoleCredentialsResponse struct {
	RoleCredentials struct {
		AccessKeyID     string `json:"accessKeyId"`
		SecretAccessKey string `json:"secretAccessKey"`
		SessionToken    string `json:"sessionToken"`
		Expiration      int64  `json:"expiration"`
	} `json:"roleCredentials"`
}

type SSOAccount struct {
	AccountID   string   `json:"accountId" yaml:"accountId"`
	AccountName string   `json:"accountName" yaml:"accountName"`
	SSORegion   string   `json:"ssoRegion" yaml:"ssoRegion"`
	Email       string   `json:"email" yaml:"email"`
	Roles       []string `json:"roles" yaml:"roles"`
}

// SSOProfile represents a profile configuration, which can contain multiple accounts
type SSOProfile struct {
	ProfileName string       `json:"profileName" yaml:"profileName"`
	Region      string       `json:"region" yaml:"region"`
	AccountID   string       `json:"accountId" yaml:"accountId"`
	Role        string       `json:"role" yaml:"role"`
	SsoStartUrl string       `json:"ssoStartUrl" yaml:"ssoStartUrl"`
	Accounts    []SSOAccount `json:"accounts" yaml:"accounts"` // List of accounts under this profile
}

// Config represents the root configuration containing all profiles
type Config struct {
	Aws struct {
		Profiles []SSOProfile `json:"profiles" yaml:"profiles"` // List of profiles
	} `json:"aws" yaml:"aws"`
}
