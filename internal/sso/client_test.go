package sso_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/BerryBytes/awsctl/internal/sso"
	"github.com/BerryBytes/awsctl/models"
	mock_awsctl "github.com/BerryBytes/awsctl/tests/mock"
	mock_sso "github.com/BerryBytes/awsctl/tests/mock/sso"
	"github.com/BerryBytes/awsctl/utils/common"
	promptUtils "github.com/BerryBytes/awsctl/utils/prompt"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSSOClient(t *testing.T) {
	t.Run("successful creation", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockPrompter := mock_sso.NewMockPrompter(ctrl)
		mockExecutor := mock_awsctl.NewMockCommandExecutor(ctrl)

		client, err := sso.NewSSOClient(mockPrompter, mockExecutor)
		assert.NoError(t, err)
		assert.NotNil(t, client)
	})

	t.Run("nil prompter returns error", func(t *testing.T) {
		client, err := sso.NewSSOClient(nil, nil)
		assert.Error(t, err)
		assert.Nil(t, client)
		assert.Equal(t, "prompter cannot be nil", err.Error())
	})
}
func TestNewSSOClient_NilExecutor(t *testing.T) {
	t.Run("nil executor gets default", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockPrompter := mock_sso.NewMockPrompter(ctrl)

		client, err := sso.NewSSOClient(mockPrompter, nil)
		assert.NoError(t, err)
		assert.NotNil(t, client)
		assert.IsType(t, &common.RealCommandExecutor{}, client.(*sso.RealSSOClient).Executor)
	})
}

func TestGetCachedSsoAccessToken(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPrompter := mock_sso.NewMockPrompter(ctrl)
	mockExecutor := mock_awsctl.NewMockCommandExecutor(ctrl)

	t.Run("returns cached token if valid", func(t *testing.T) {
		client := &sso.RealSSOClient{
			Prompter: mockPrompter,
			Executor: mockExecutor,
			TokenCache: models.TokenCache{
				AccessToken: "cached-token",
				Expiry:      time.Now().Add(1 * time.Hour),
			},
		}

		token, expiry, err := client.GetCachedSsoAccessToken("profile")
		assert.NoError(t, err)
		assert.Equal(t, "cached-token", token)
		assert.False(t, expiry.IsZero())
	})

	t.Run("fetches new token if cache expired", func(t *testing.T) {
		tempDir := t.TempDir()
		cacheDir := filepath.Join(tempDir, ".aws", "sso", "cache")
		err := os.MkdirAll(cacheDir, 0755)
		require.NoError(t, err)

		cacheFile := filepath.Join(cacheDir, "test.json")
		cacheData := models.SSOCache{
			StartURL:    stringPtr("https://example.awsapps.com/start"),
			AccessToken: stringPtr("new-token"),
			ExpiresAt:   stringPtr(time.Now().Add(1 * time.Hour).Format(time.RFC3339)),
		}
		data, err := json.Marshal(cacheData)
		require.NoError(t, err)
		err = os.WriteFile(cacheFile, data, 0644)
		require.NoError(t, err)

		mockExecutor.EXPECT().RunCommand("aws", "configure", "get", "sso_start_url", "--profile", "test-profile").
			Return([]byte("https://example.awsapps.com/start"), nil)

		client := &sso.RealSSOClient{
			Prompter: mockPrompter,
			Executor: mockExecutor,
			TokenCache: models.TokenCache{
				AccessToken: "expired-token",
				Expiry:      time.Now().Add(-1 * time.Hour),
			},
		}

		oldHome := os.Getenv("HOME")
		t.Cleanup(func() {
			_ = os.Setenv("HOME", oldHome)
		})
		if err := os.Setenv("HOME", tempDir); err != nil {
			t.Fatalf("failed to set HOME to tempDir: %v", err)
		}

		token, _, err := client.GetCachedSsoAccessToken("test-profile")
		assert.NoError(t, err)
		assert.Equal(t, "new-token", token)
	})
}

func TestSSOLogin(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPrompter := mock_sso.NewMockPrompter(ctrl)
	mockExecutor := mock_awsctl.NewMockCommandExecutor(ctrl)

	tests := []struct {
		name        string
		setup       func()
		profile     string
		refresh     bool
		noBrowser   bool
		expectError bool
	}{
		{
			name: "successful login with browser",
			setup: func() {
				mockExecutor.EXPECT().RunInteractiveCommand(gomock.Any(), "aws", "sso", "login", "--profile", "test-profile").
					Return(nil)
			},
			profile:     "test-profile",
			expectError: false,
		},
		{
			name: "successful login without browser",
			setup: func() {
				mockExecutor.EXPECT().RunInteractiveCommand(gomock.Any(), "aws", "sso", "login", "--no-browser", "--profile", "test-profile").
					Return(nil)
			},
			profile:     "test-profile",
			noBrowser:   true,
			expectError: false,
		},
		{
			name: "login timeout",
			setup: func() {
				mockExecutor.EXPECT().RunInteractiveCommand(gomock.Any(), "aws", "sso", "login", "--profile", "test-profile").
					Return(context.DeadlineExceeded)
			},
			profile:     "test-profile",
			expectError: true,
		},
		{
			name: "login interrupted",
			setup: func() {
				mockExecutor.EXPECT().RunInteractiveCommand(gomock.Any(), "aws", "sso", "login", "--profile", "test-profile").
					Return(promptUtils.ErrInterrupted)
			},
			profile:     "test-profile",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			client := &sso.RealSSOClient{
				Prompter: mockPrompter,
				Executor: mockExecutor,
			}

			err := client.SSOLogin(tt.profile, tt.refresh, tt.noBrowser)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGetSSOAccountName(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPrompter := mock_sso.NewMockPrompter(ctrl)
	mockExecutor := mock_awsctl.NewMockCommandExecutor(ctrl)

	tests := []struct {
		name          string
		setup         func(client *sso.RealSSOClient)
		accountID     string
		profile       string
		expectedName  string
		expectError   bool
		errorContains string
	}{
		{
			name: "successful account name retrieval with cache hit",
			setup: func(client *sso.RealSSOClient) {
				client.TokenCache.AccessToken = "test-token"
				client.TokenCache.Expiry = time.Now().Add(1 * time.Hour)

				accountList := models.AccountNameResponse{
					AccountList: []models.Account{
						{AccountID: "123456789012", AccountName: "test-account"},
					},
				}
				output, _ := json.Marshal(accountList)
				mockExecutor.EXPECT().
					RunCommand("aws", "sso", "list-accounts", "--access-token", "test-token", "--output", "json").
					Return(output, nil)
			},
			accountID:    "123456789012",
			profile:      "test-profile",
			expectedName: "test-account",
			expectError:  false,
		},
		{
			name: "successful account name retrieval with cache miss",
			setup: func(client *sso.RealSSOClient) {
				tempDir := t.TempDir()
				cacheDir := filepath.Join(tempDir, ".aws", "sso", "cache")
				require.NoError(t, os.MkdirAll(cacheDir, 0755))

				cache := models.SSOCache{
					StartURL:    stringPtr("https://example.awsapps.com/start"),
					AccessToken: stringPtr("test-token"),
					ExpiresAt:   stringPtr(time.Now().Add(1 * time.Hour).Format(time.RFC3339)),
				}
				data, _ := json.Marshal(cache)
				err := os.WriteFile(filepath.Join(cacheDir, "test.json"), data, 0644)
				require.NoError(t, err)

				oldHome := os.Getenv("HOME")
				t.Setenv("HOME", tempDir)
				t.Cleanup(func() {
					_ = os.Setenv("HOME", oldHome)
				})

				mockExecutor.EXPECT().
					RunCommand("aws", "configure", "get", "sso_start_url", "--profile", "test-profile").
					Return([]byte("https://example.awsapps.com/start"), nil)

				accountList := models.AccountNameResponse{
					AccountList: []models.Account{
						{AccountID: "123456789012", AccountName: "test-account"},
					},
				}
				output, _ := json.Marshal(accountList)
				mockExecutor.EXPECT().
					RunCommand("aws", "sso", "list-accounts", "--access-token", "test-token", "--output", "json").
					Return(output, nil)

				client.TokenCache = models.TokenCache{}
			},
			accountID:    "123456789012",
			profile:      "test-profile",
			expectedName: "test-account",
			expectError:  false,
		},
		{
			name: "account not found",
			setup: func(client *sso.RealSSOClient) {
				tempDir := t.TempDir()
				cacheDir := filepath.Join(tempDir, ".aws", "sso", "cache")
				require.NoError(t, os.MkdirAll(cacheDir, 0755))

				cache := models.SSOCache{
					StartURL:    stringPtr("https://example.awsapps.com/start"),
					AccessToken: stringPtr("test-token"),
					ExpiresAt:   stringPtr(time.Now().Add(1 * time.Hour).Format(time.RFC3339)),
				}
				data, _ := json.Marshal(cache)
				require.NoError(t, os.WriteFile(filepath.Join(cacheDir, "cache.json"), data, 0644))

				oldHome := os.Getenv("HOME")
				t.Setenv("HOME", tempDir)
				t.Cleanup(func() {
					_ = os.Setenv("HOME", oldHome)
				})

				mockExecutor.EXPECT().
					RunCommand("aws", "configure", "get", "sso_start_url", "--profile", "test-profile").
					Return([]byte("https://example.awsapps.com/start"), nil)

				accountList := models.AccountNameResponse{
					AccountList: []models.Account{},
				}
				output, _ := json.Marshal(accountList)
				mockExecutor.EXPECT().
					RunCommand("aws", "sso", "list-accounts", "--access-token", "test-token", "--output", "json").
					Return(output, nil)

				client.TokenCache = models.TokenCache{}
			},
			accountID:     "123456789012",
			profile:       "test-profile",
			expectedName:  "",
			expectError:   true,
			errorContains: "account ID 123456789012 not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &sso.RealSSOClient{
				Executor: mockExecutor,
				Prompter: mockPrompter,
			}
			tt.setup(client)

			name, err := client.GetSSOAccountName(tt.accountID, tt.profile)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					require.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedName, name)
			}
		})
	}
}

func TestGetRoleCredentials(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPrompter := mock_sso.NewMockPrompter(ctrl)
	mockExecutor := mock_awsctl.NewMockCommandExecutor(ctrl)

	t.Run("successful credentials retrieval", func(t *testing.T) {
		client := &sso.RealSSOClient{
			Prompter: mockPrompter,
			Executor: mockExecutor,
		}

		expectedCreds := &models.AWSCredentials{
			AccessKeyID:     "AKIA...",
			SecretAccessKey: "secret...",
			SessionToken:    "token...",
			Expiration:      time.Now().Add(1 * time.Hour).Format(time.RFC3339),
		}

		response := models.RoleCredentialsResponse{
			RoleCredentials: models.RoleCredentials{
				AccessKeyID:     expectedCreds.AccessKeyID,
				SecretAccessKey: expectedCreds.SecretAccessKey,
				SessionToken:    expectedCreds.SessionToken,
				Expiration:      time.Now().Add(1*time.Hour).Unix() * 1000,
			},
		}
		responseData, err := json.Marshal(response)
		require.NoError(t, err)

		mockExecutor.EXPECT().RunCommand(
			"aws", "sso", "get-role-credentials",
			"--access-token", "test-token",
			"--role-name", "test-role",
			"--account-id", "123456789012",
		).Return(responseData, nil)

		creds, err := client.GetRoleCredentials("test-token", "test-role", "123456789012")
		assert.NoError(t, err)
		assert.Equal(t, expectedCreds, creds)
	})

	t.Run("command failure", func(t *testing.T) {
		client := &sso.RealSSOClient{
			Prompter: mockPrompter,
			Executor: mockExecutor,
		}

		mockExecutor.EXPECT().RunCommand(
			"aws", "sso", "get-role-credentials",
			"--access-token", "test-token",
			"--role-name", "test-role",
			"--account-id", "123456789012",
		).Return(nil, errors.New("command failed"))

		_, err := client.GetRoleCredentials("test-token", "test-role", "123456789012")
		assert.Error(t, err)
	})

	t.Run("invalid JSON response", func(t *testing.T) {
		client := &sso.RealSSOClient{
			Prompter: mockPrompter,
			Executor: mockExecutor,
		}

		mockExecutor.EXPECT().RunCommand(
			"aws", "sso", "get-role-credentials",
			"--access-token", "test-token",
			"--role-name", "test-role",
			"--account-id", "123456789012",
		).Return([]byte("invalid json"), nil)

		_, err := client.GetRoleCredentials("test-token", "test-role", "123456789012")
		assert.Error(t, err)
	})
}

func TestGetCachedSsoAccessToken_ErrorCases(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPrompter := mock_sso.NewMockPrompter(ctrl)
	mockExecutor := mock_awsctl.NewMockCommandExecutor(ctrl)

	t.Run("error from getSsoAccessTokenFromCache", func(t *testing.T) {
		mockExecutor.EXPECT().
			RunCommand("aws", "configure", "get", "sso_start_url", "--profile", "bad-profile").
			Return(nil, errors.New("config error"))

		client := &sso.RealSSOClient{
			Prompter: mockPrompter,
			Executor: mockExecutor,
		}

		_, _, err := client.GetCachedSsoAccessToken("bad-profile")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get sso_start_url for profile bad-profile")
	})

	t.Run("nil access token in cache", func(t *testing.T) {
		tempDir := t.TempDir()
		cacheDir := filepath.Join(tempDir, ".aws", "sso", "cache")
		require.NoError(t, os.MkdirAll(cacheDir, 0755))

		cacheFile := filepath.Join(cacheDir, "test.json")
		cacheData := models.SSOCache{
			StartURL:    stringPtr("https://example.com"),
			AccessToken: nil,
			ExpiresAt:   stringPtr(time.Now().Add(1 * time.Hour).Format(time.RFC3339)),
		}
		data, _ := json.Marshal(cacheData)
		require.NoError(t, os.WriteFile(cacheFile, data, 0644))

		mockExecutor.EXPECT().
			RunCommand("aws", "configure", "get", "sso_start_url", "--profile", "test-profile").
			Return([]byte("https://example.com"), nil)

		oldHome := os.Getenv("HOME")
		if err := os.Setenv("HOME", tempDir); err != nil {
			t.Fatalf("failed to set HOME to tempDir: %v", err)
		}
		t.Cleanup(func() {
			_ = os.Setenv("HOME", oldHome)
		})

		client := &sso.RealSSOClient{
			Prompter: mockPrompter,
			Executor: mockExecutor,
		}

		_, _, err := client.GetCachedSsoAccessToken("test-profile")
		assert.Error(t, err)
		assert.Equal(t, "no access token found in cache for profile test-profile", err.Error())
	})
}

func TestGetSSOAccountName_ErrorCases(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPrompter := mock_sso.NewMockPrompter(ctrl)
	mockExecutor := mock_awsctl.NewMockCommandExecutor(ctrl)

	t.Run("error getting access token", func(t *testing.T) {
		client := &sso.RealSSOClient{
			Prompter: mockPrompter,
			Executor: mockExecutor,
		}

		mockExecutor.EXPECT().
			RunCommand("aws", "configure", "get", "sso_start_url", "--profile", "bad-profile").
			Return(nil, errors.New("config error"))

		_, err := client.GetSSOAccountName("123456789012", "bad-profile")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to retrieve SSO access token")
	})

	t.Run("error listing accounts", func(t *testing.T) {
		client := &sso.RealSSOClient{
			Prompter: mockPrompter,
			Executor: mockExecutor,
			TokenCache: models.TokenCache{
				AccessToken: "test-token",
				Expiry:      time.Now().Add(1 * time.Hour),
			},
		}

		mockExecutor.EXPECT().
			RunCommand("aws", "sso", "list-accounts", "--access-token", "test-token", "--output", "json").
			Return(nil, errors.New("command failed"))

		_, err := client.GetSSOAccountName("123456789012", "test-profile")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to list AWS accounts")
	})

	t.Run("invalid accounts JSON", func(t *testing.T) {
		client := &sso.RealSSOClient{
			Prompter: mockPrompter,
			Executor: mockExecutor,
			TokenCache: models.TokenCache{
				AccessToken: "test-token",
				Expiry:      time.Now().Add(1 * time.Hour),
			},
		}

		mockExecutor.EXPECT().
			RunCommand("aws", "sso", "list-accounts", "--access-token", "test-token", "--output", "json").
			Return([]byte("invalid json"), nil)

		_, err := client.GetSSOAccountName("123456789012", "test-profile")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to unmarshal accounts")
	})
}

func TestGetSsoAccessTokenFromCache_ErrorCases(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPrompter := mock_sso.NewMockPrompter(ctrl)
	mockExecutor := mock_awsctl.NewMockCommandExecutor(ctrl)

	tests := []struct {
		name          string
		setup         func(*sso.RealSSOClient)
		profile       string
		expectedError string
		expectError   bool
	}{
		{
			name: "error getting start URL",
			setup: func(client *sso.RealSSOClient) {
				mockExecutor.EXPECT().
					RunCommand("aws", "configure", "get", "sso_start_url", "--profile", "bad-profile").
					Return(nil, errors.New("config error"))
			},
			profile:       "bad-profile",
			expectedError: "failed to get sso_start_url",
			expectError:   true,
		},
		{
			name: "error getting home directory",
			setup: func(client *sso.RealSSOClient) {
				mockExecutor.EXPECT().
					RunCommand("aws", "configure", "get", "sso_start_url", "--profile", "test-profile").
					Return([]byte("https://example.com"), nil)

				oldHome := os.Getenv("HOME")
				if err := os.Unsetenv("HOME"); err != nil {
					t.Logf("failed to unset HOME: %v", err)
				}
				t.Cleanup(func() {
					_ = os.Setenv("HOME", oldHome)
				})
			},
			profile:       "test-profile",
			expectedError: "failed to get user home directory",
			expectError:   true,
		},
		{
			name: "cache directory not exists",
			setup: func(client *sso.RealSSOClient) {
				mockExecutor.EXPECT().
					RunCommand("aws", "configure", "get", "sso_start_url", "--profile", "test-profile").
					Return([]byte("https://example.com"), nil)

				tempDir := t.TempDir()
				oldHome := os.Getenv("HOME")
				if err := os.Setenv("HOME", tempDir); err != nil {
					t.Fatalf("failed to set HOME to tempDir: %v", err)
				}
				t.Cleanup(func() {
					_ = os.Setenv("HOME", oldHome)
				})
			},
			profile:       "test-profile",
			expectedError: "no matching SSO cache file found",
			expectError:   true,
		},
		{
			name: "no matching cache file",
			setup: func(client *sso.RealSSOClient) {
				mockExecutor.EXPECT().
					RunCommand("aws", "configure", "get", "sso_start_url", "--profile", "test-profile").
					Return([]byte("https://example.com"), nil)

				tempDir := t.TempDir()
				cacheDir := filepath.Join(tempDir, ".aws", "sso", "cache")
				require.NoError(t, os.MkdirAll(cacheDir, 0755))

				cacheFile := filepath.Join(cacheDir, "test.json")
				cacheData := models.SSOCache{
					StartURL:    stringPtr("https://different.com"),
					AccessToken: stringPtr("token"),
					ExpiresAt:   stringPtr(time.Now().Add(1 * time.Hour).Format(time.RFC3339)),
				}
				data, _ := json.Marshal(cacheData)
				require.NoError(t, os.WriteFile(cacheFile, data, 0644))

				oldHome := os.Getenv("HOME")
				if err := os.Setenv("HOME", tempDir); err != nil {
					t.Fatalf("failed to set HOME to tempDir: %v", err)
				}
				t.Cleanup(func() {
					_ = os.Setenv("HOME", oldHome)
				})
			},
			profile:       "test-profile",
			expectedError: "no matching SSO cache file found",
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &sso.RealSSOClient{
				Prompter: mockPrompter,
				Executor: mockExecutor,
			}
			tt.setup(client)

			cache, _, err := client.GetSsoAccessTokenFromCache(tt.profile)

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, cache)
				if tt.name == "expired token triggers login" {
					assert.Equal(t, "new-token", *cache.AccessToken)
				}
			}
		})
	}
}

func stringPtr(s string) *string {
	return &s
}
