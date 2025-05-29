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

type Config struct {
	SSOSessions []SSOSession `yaml:"ssoSessions" json:"ssoSessions"`
}

// SSOSession represents an AWS SSO session configuration.
type SSOSession struct {
	Name     string `yaml:"name" json:"name"`
	StartURL string `yaml:"startUrl" json:"startUrl"`
	Region   string `yaml:"region" json:"region"`
	Scopes   string `yaml:"scopes,omitempty" json:"scopes,omitempty"`
}

type RoleCredentials struct {
	AccessKeyID     string `json:"accessKeyId"`
	SecretAccessKey string `json:"secretAccessKey"`
	SessionToken    string `json:"sessionToken"`
	Expiration      int64  `json:"expiration"`
}
