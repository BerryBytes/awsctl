package sso_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/BerryBytes/awsctl/internal/sso"
	mock_awsctl "github.com/BerryBytes/awsctl/tests/mock"
	mock_sso "github.com/BerryBytes/awsctl/tests/mock/sso"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigureSSOProfile(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPrompter := mock_sso.NewMockPrompter(ctrl)
	mockExecutor := mock_awsctl.NewMockCommandExecutor(ctrl)

	t.Run("successful configuration", func(t *testing.T) {
		mockExecutor.EXPECT().RunCommand("aws", "configure", "set", "sso_session", "test-session", "--profile", "test-profile").Return(nil, nil)
		mockExecutor.EXPECT().RunCommand("aws", "configure", "set", "sso_region", "us-west-2", "--profile", "test-profile").Return(nil, nil)
		mockExecutor.EXPECT().RunCommand("aws", "configure", "set", "sso_account_id", "123456789012", "--profile", "test-profile").Return(nil, nil)
		mockExecutor.EXPECT().RunCommand("aws", "configure", "set", "sso_start_url", "https://example.awsapps.com/start", "--profile", "test-profile").Return(nil, nil)
		mockExecutor.EXPECT().RunCommand("aws", "configure", "set", "sso_role_name", "Admin", "--profile", "test-profile").Return(nil, nil)
		mockExecutor.EXPECT().RunCommand("aws", "configure", "set", "region", "us-west-2", "--profile", "test-profile").Return(nil, nil)
		mockExecutor.EXPECT().RunCommand("aws", "configure", "set", "output", "json", "--profile", "test-profile").Return(nil, nil)

		client := &sso.RealSSOClient{
			Prompter: mockPrompter,
			Executor: mockExecutor,
		}

		err := client.ConfigureSSOProfile("test-profile", "us-west-2", "123456789012", "Admin", "https://example.awsapps.com/start", "test-session")
		assert.NoError(t, err)
	})

	t.Run("error setting first config value", func(t *testing.T) {
		mockExecutor.EXPECT().
			RunCommand("aws", "configure", "set", "sso_session", "test-session", "--profile", "test-profile").
			Return(nil, errors.New("config error"))

		mockExecutor.EXPECT().RunCommand(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

		client := &sso.RealSSOClient{
			Prompter: mockPrompter,
			Executor: mockExecutor,
		}

		err := client.ConfigureSSOProfile("test-profile", "us-west-2", "123456789012", "Admin", "https://example.awsapps.com/start", "test-session")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "config error")
	})
}
func TestConfigureAWSProfile(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPrompter := mock_sso.NewMockPrompter(ctrl)
	mockExecutor := mock_awsctl.NewMockCommandExecutor(ctrl)

	tests := []struct {
		name           string
		profileName    string
		sessionName    string
		ssoRegion      string
		ssoStartURL    string
		accountID      string
		roleName       string
		region         string
		setup          func()
		expectError    bool
		errorContains  string
		expectedOutput string
	}{
		{
			name:        "successful non-default profile configuration",
			profileName: "test-profile",
			sessionName: "test-session",
			ssoRegion:   "us-west-2",
			ssoStartURL: "https://example.awsapps.com/start",
			accountID:   "123456789012",
			roleName:    "Admin",
			region:      "us-west-2",
			setup: func() {
				mockExecutor.EXPECT().RunCommand("aws", "configure", "set", "sso_session", "test-session", "--profile", "test-profile").Return(nil, nil)
				mockExecutor.EXPECT().RunCommand("aws", "configure", "set", "sso_region", "us-west-2", "--profile", "test-profile").Return(nil, nil)
				mockExecutor.EXPECT().RunCommand("aws", "configure", "set", "sso_account_id", "123456789012", "--profile", "test-profile").Return(nil, nil)
				mockExecutor.EXPECT().RunCommand("aws", "configure", "set", "sso_start_url", "https://example.awsapps.com/start", "--profile", "test-profile").Return(nil, nil)
				mockExecutor.EXPECT().RunCommand("aws", "configure", "set", "sso_role_name", "Admin", "--profile", "test-profile").Return(nil, nil)
				mockExecutor.EXPECT().RunCommand("aws", "configure", "set", "region", "us-west-2", "--profile", "test-profile").Return(nil, nil)
				mockExecutor.EXPECT().RunCommand("aws", "configure", "set", "output", "json", "--profile", "test-profile").Return(nil, nil)
			},
			expectError:    false,
			expectedOutput: "Configured AWS profile 'test-profile'",
		},
		{
			name:        "successful default profile configuration",
			profileName: "default",
			sessionName: "test-session",
			ssoRegion:   "us-west-2",
			ssoStartURL: "https://example.awsapps.com/start",
			accountID:   "123456789012",
			roleName:    "Admin",
			region:      "us-west-2",
			setup: func() {
				tempDir := t.TempDir()
				oldHome := os.Getenv("HOME")
				if err := os.Setenv("HOME", tempDir); err != nil {
					t.Fatalf("failed to set HOME env: %v", err)
				}
				t.Cleanup(func() {
					if err := os.Setenv("HOME", oldHome); err != nil {
						t.Logf("failed to reset HOME env: %v", err)
					}
				})
			},
			expectError:    false,
			expectedOutput: "Configured AWS default profile",
		},
		{
			name:          "invalid start URL",
			profileName:   "test-profile",
			sessionName:   "test-session",
			ssoRegion:     "us-west-2",
			ssoStartURL:   "invalid-url",
			accountID:     "123456789012",
			roleName:      "Admin",
			region:        "us-west-2",
			setup:         func() {},
			expectError:   true,
			errorContains: "invalid start URL",
		},
		{
			name:          "invalid account ID",
			profileName:   "test-profile",
			sessionName:   "test-session",
			ssoRegion:     "us-west-2",
			ssoStartURL:   "https://example.awsapps.com/start",
			accountID:     "invalid",
			roleName:      "Admin",
			region:        "us-west-2",
			setup:         func() {},
			expectError:   true,
			errorContains: "invalid account ID",
		},
		{
			name:        "error creating config directory",
			profileName: "default",
			sessionName: "test-session",
			ssoRegion:   "us-west-2",
			ssoStartURL: "https://example.awsapps.com/start",
			accountID:   "123456789012",
			roleName:    "Admin",
			region:      "us-west-2",
			setup: func() {
				tempDir := t.TempDir()
				awsDir := filepath.Join(tempDir, ".aws")

				require.NoError(t, os.WriteFile(awsDir, []byte("not a directory"), 0600))

				oldHome := os.Getenv("HOME")
				if err := os.Setenv("HOME", tempDir); err != nil {
					t.Fatalf("failed to set HOME env: %v", err)
				}
				t.Cleanup(func() {
					if err := os.Setenv("HOME", oldHome); err != nil {
						t.Logf("failed to reset HOME env: %v", err)
					}
				})
			},
			expectError:   true,
			errorContains: "failed to create directory",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			client := &sso.RealSSOClient{
				Prompter: mockPrompter,
				Executor: mockExecutor,
			}

			err := client.ConfigureAWSProfile(tt.profileName, tt.sessionName, tt.ssoRegion, tt.ssoStartURL, tt.accountID, tt.roleName, tt.region)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				require.NoError(t, err)

			}
		})
	}
}
func TestPromptProfileDetails(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPrompter := mock_sso.NewMockPrompter(ctrl)

	tests := []struct {
		name           string
		ssoRegion      string
		mockResponses  []interface{}
		expectedName   string
		expectedRegion string
		expectError    bool
	}{
		{
			name:      "successful prompts",
			ssoRegion: "us-west-2",
			mockResponses: []interface{}{
				"test-profile", nil,
				"us-east-1", nil,
			},
			expectedName:   "test-profile",
			expectedRegion: "us-east-1",
			expectError:    false,
		},
		{
			name:      "error prompting for profile name",
			ssoRegion: "us-west-2",
			mockResponses: []interface{}{
				"", errors.New("prompt error"),
			},
			expectError: true,
		},
		{
			name:      "error prompting for region",
			ssoRegion: "us-west-2",
			mockResponses: []interface{}{
				"test-profile", nil,
				"", errors.New("prompt error"),
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := mockPrompter
			if len(tt.mockResponses) > 0 {
				mock.EXPECT().
					PromptWithDefault("Enter profile name to configure", "sso-profile").
					Return(tt.mockResponses[0], tt.mockResponses[1])
			}
			if len(tt.mockResponses) > 2 {
				mock.EXPECT().
					PromptWithDefault("AWS region for this profile", tt.ssoRegion).
					Return(tt.mockResponses[2], tt.mockResponses[3])
			}

			client := &sso.RealSSOClient{
				Prompter: mockPrompter,
			}

			name, region, err := client.PromptProfileDetails(tt.ssoRegion)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedName, name)
				assert.Equal(t, tt.expectedRegion, region)
			}
		})
	}
}
