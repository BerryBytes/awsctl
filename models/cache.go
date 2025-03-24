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
