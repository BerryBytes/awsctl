package sso

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateAccountID(t *testing.T) {
	tests := []struct {
		name          string
		accountID     string
		expectError   bool
		errorContains string
	}{
		{
			name:        "valid account ID",
			accountID:   "123456789012",
			expectError: false,
		},
		{
			name:          "invalid length",
			accountID:     "1234567890",
			expectError:   true,
			errorContains: "must be 12 digits",
		},
		{
			name:          "non-numeric",
			accountID:     "1234567890ab",
			expectError:   true,
			errorContains: "must be 12 digits",
		},
		{
			name:          "empty",
			accountID:     "",
			expectError:   true,
			errorContains: "must be 12 digits",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAccountID(tt.accountID)
			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateStartURL(t *testing.T) {
	tests := []struct {
		name          string
		startURL      string
		expectError   bool
		errorContains string
	}{
		{
			name:        "valid HTTPS URL",
			startURL:    "https://example.awsapps.com/start",
			expectError: false,
		},
		{
			name:          "invalid protocol",
			startURL:      "http://example.com",
			expectError:   true,
			errorContains: "must start with https://",
		},
		{
			name:          "no protocol",
			startURL:      "example.com",
			expectError:   true,
			errorContains: "must start with https://",
		},
		{
			name:          "empty",
			startURL:      "",
			expectError:   true,
			errorContains: "must start with https://",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateStartURL(tt.startURL)
			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestPrintSummary(t *testing.T) {
	t.Run("prints complete summary", func(t *testing.T) {
		printSummary(
			"my-profile",
			"my-session",
			"https://example.awsapps.com/start",
			"us-east-1",
			"123456789012",
			"AdminRole",
			"My Account",
			"arn:aws:iam::123456789012:role/AdminRole",
			"2023-01-01T00:00:00Z",
		)
	})

	t.Run("prints minimal summary", func(t *testing.T) {
		printSummary(
			"my-profile",
			"my-session",
			"https://example.awsapps.com/start",
			"us-east-1",
			"123456789012",
			"AdminRole",
			"",
			"",
			"",
		)
	})
}
