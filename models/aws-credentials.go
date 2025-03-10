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
