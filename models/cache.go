package models

import (
	"sync"
	"time"
)

// TokenCache holds a cached SSO access token and its expiration time.
type TokenCache struct {
	AccessToken string    `json:"accessToken" yaml:"accessToken"`
	Expiry      time.Time `json:"expiry" yaml:"expiry"`
	Mu          sync.Mutex
}

type SSOCache struct {
	AccessToken           *string `json:"accessToken"`
	ExpiresAt             *string `json:"expiresAt"`
	StartURL              *string `json:"startUrl"`
	SessionName           *string `json:"sessionName,omitempty"`
	AccountID             *string `json:"accountId,omitempty"`
	Region                *string `json:"region"`
	ClientID              *string `json:"clientId,omitempty"`
	ClientSecret          *string `json:"clientSecret,omitempty"`
	RegistrationExpiresAt *string `json:"registrationExpiresAt,omitempty"`
}
