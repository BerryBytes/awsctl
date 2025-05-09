package sso_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/BerryBytes/awsctl/internal/sso"
	"github.com/BerryBytes/awsctl/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type MockCommandExecutor struct {
	mock.Mock
}

func (m *MockCommandExecutor) RunCommand(name string, args ...string) ([]byte, error) {
	argsForCall := append([]interface{}{name}, interfaceSlice(args)...)
	ret := m.Called(argsForCall...)
	return ret.Get(0).([]byte), ret.Error(1)
}

func (m *MockCommandExecutor) RunInteractiveCommand(ctx context.Context, name string, args ...string) error {
	argsForCall := append([]interface{}{ctx, name}, interfaceSlice(args)...)
	ret := m.Called(argsForCall...)
	return ret.Error(0)
}

func (m *MockCommandExecutor) RunCommandWithInput(name, input string, args ...string) ([]byte, error) {
	argsForCall := append([]interface{}{name, input}, interfaceSlice(args)...)
	ret := m.Called(argsForCall...)
	return ret.Get(0).([]byte), ret.Error(1)
}

func (m *MockCommandExecutor) LookPath(file string) (string, error) {
	ret := m.Called(file)
	return ret.Get(0).(string), ret.Error(1)
}

type MockAWSCredentialsClient struct {
	mock.Mock
}

func (m *MockAWSCredentialsClient) GetRoleCredentials(accessToken, roleName, accountID string) (*models.AWSCredentials, error) {
	args := m.Called(accessToken, roleName, accountID)
	return args.Get(0).(*models.AWSCredentials), args.Error(1)
}

func (m *MockAWSCredentialsClient) SaveAWSCredentials(profile string, creds *models.AWSCredentials) error {
	args := m.Called(profile, creds)
	return args.Error(0)
}

func (m *MockAWSCredentialsClient) IsCallerIdentityValid(profile string) bool {
	args := m.Called(profile)
	return args.Bool(0)
}

func (m *MockAWSCredentialsClient) AwsSTSGetCallerIdentity(profile string) (string, error) {
	args := m.Called(profile)
	return args.String(0), args.Error(1)
}

type MockAWSConfigClient struct {
	mock.Mock
}

func (m *MockAWSConfigClient) ConfigureDefaultProfile(region, output string) error {
	args := m.Called(region, output)
	return args.Error(0)
}

func (m *MockAWSConfigClient) ConfigureSSOProfile(profile, region, accountID, role, startURL string) error {
	args := m.Called(profile, region, accountID, role, startURL)
	return args.Error(0)
}

func interfaceSlice(slice []string) []interface{} {
	result := make([]interface{}, len(slice))
	for i, v := range slice {
		result[i] = v
	}
	return result
}

func createTempAWSConfig(t *testing.T, content string) string {
	t.Helper()
	homeDir, err := os.MkdirTemp("", "aws_test")
	require.NoError(t, err)
	configPath := filepath.Join(homeDir, ".aws")
	err = os.MkdirAll(configPath, 0755)
	require.NoError(t, err)
	configFile := filepath.Join(configPath, "config")
	err = os.WriteFile(configFile, []byte(content), 0644)
	require.NoError(t, err)
	return homeDir
}

func createTempSSOCache(t *testing.T, cacheDir string, cache models.SSOCache) string {
	t.Helper()
	err := os.MkdirAll(cacheDir, 0755)
	require.NoError(t, err)
	cacheFile, err := os.CreateTemp(cacheDir, "*.json")
	require.NoError(t, err)
	defer func() {
		if err := cacheFile.Close(); err != nil {
			t.Logf("failed to close cache file: %v", err)
		}
	}()
	err = json.NewEncoder(cacheFile).Encode(cache)
	require.NoError(t, err)
	return cacheFile.Name()
}

func stringPtr(s string) *string {
	return &s
}

func TestNewRealAWSSSOClient(t *testing.T) {
	mockExecutor := &MockCommandExecutor{}
	client, err := sso.NewRealAWSSSOClient(mockExecutor)
	require.NoError(t, err)
	assert.NotNil(t, client)
	assert.Equal(t, mockExecutor, client.Executor)
	assert.NotNil(t, client.CredentialsClient)
	assert.NotNil(t, client.GetHomeDir)
}

func TestGetCachedSsoAccessToken(t *testing.T) {
	tests := []struct {
		name          string
		setup         func(t *testing.T) *sso.RealAWSSSOClient
		profile       string
		wantToken     string
		wantErr       bool
		expectedError string
	}{
		{
			name: "valid cached token",
			setup: func(t *testing.T) *sso.RealAWSSSOClient {
				homeDir := createTempAWSConfig(t, `
[profile test-profile]
sso_session = test-session

[sso-session test-session]
sso_start_url = https://test-start-url
`)
				cacheDir := filepath.Join(homeDir, ".aws", "sso", "cache")
				expiry := time.Now().Add(1 * time.Hour).Format(time.RFC3339)
				cache := models.SSOCache{
					AccessToken: stringPtr("test-token"),
					ExpiresAt:   &expiry,
					StartURL:    stringPtr("https://test-start-url"),
				}
				createTempSSOCache(t, cacheDir, cache)
				return &sso.RealAWSSSOClient{
					TokenCache: models.TokenCache{},
					GetHomeDir: func() (string, error) { return homeDir, nil },
				}
			},
			profile:   "test-profile",
			wantToken: "test-token",
			wantErr:   false,
		},
		{
			name: "expired token with successful re-login",
			setup: func(t *testing.T) *sso.RealAWSSSOClient {
				homeDir := createTempAWSConfig(t, `
[profile test-profile]
sso_session = test-session

[sso-session test-session]
sso_start_url = https://test-start-url
`)
				cacheDir := filepath.Join(homeDir, ".aws", "sso", "cache")
				err := os.MkdirAll(cacheDir, 0755)
				require.NoError(t, err)

				pastExpiry := time.Now().Add(-1 * time.Hour).Format(time.RFC3339)
				expiredCache := models.SSOCache{
					AccessToken: stringPtr("expired-token"),
					ExpiresAt:   &pastExpiry,
					StartURL:    stringPtr("https://test-start-url"),
				}
				createTempSSOCache(t, cacheDir, expiredCache)

				time.Sleep(1 * time.Millisecond)

				futureExpiry := time.Now().Add(1 * time.Hour).Format(time.RFC3339)
				validCache := models.SSOCache{
					AccessToken: stringPtr("new-token"),
					ExpiresAt:   &futureExpiry,
					StartURL:    stringPtr("https://test-start-url"),
				}
				createTempSSOCache(t, cacheDir, validCache)

				mockExecutor := &MockCommandExecutor{}
				mockExecutor.On("RunInteractiveCommand", mock.Anything, "aws", "sso", "login", "--profile", "test-profile").Return(nil)
				mockCreds := &MockAWSCredentialsClient{}
				mockCreds.On("IsCallerIdentityValid", "test-profile").Return(false)
				return &sso.RealAWSSSOClient{
					Executor:          mockExecutor,
					CredentialsClient: mockCreds,
					GetHomeDir:        func() (string, error) { return homeDir, nil },
				}
			},
			profile:   "test-profile",
			wantToken: "new-token",
			wantErr:   false,
		},
		{
			name: "no cache file",
			setup: func(t *testing.T) *sso.RealAWSSSOClient {
				homeDir := createTempAWSConfig(t, `
[profile test-profile]
sso_session = test-session

[sso-session test-session]
sso_start_url = https://test-start-url
`)
				return &sso.RealAWSSSOClient{
					GetHomeDir: func() (string, error) { return homeDir, nil },
				}
			},
			profile:       "test-profile",
			wantErr:       true,
			expectedError: "no matching SSO cache file found",
		},
		{
			name: "failed to get home directory",
			setup: func(t *testing.T) *sso.RealAWSSSOClient {
				return &sso.RealAWSSSOClient{
					GetHomeDir: func() (string, error) { return "", errors.New("home dir error") },
				}
			},
			profile:       "test-profile",
			wantErr:       true,
			expectedError: "unable to find home directory",
		},
		{
			name: "invalid expiration time format",
			setup: func(t *testing.T) *sso.RealAWSSSOClient {
				homeDir := createTempAWSConfig(t, `
[profile test-profile]
sso_session = test-session

[sso-session test-session]
sso_start_url = https://test-start-url
`)
				cacheDir := filepath.Join(homeDir, ".aws", "sso", "cache")
				cache := models.SSOCache{
					AccessToken: stringPtr("test-token"),
					ExpiresAt:   stringPtr("invalid-time-format"),
					StartURL:    stringPtr("https://test-start-url"),
				}
				createTempSSOCache(t, cacheDir, cache)
				return &sso.RealAWSSSOClient{
					GetHomeDir: func() (string, error) { return homeDir, nil },
				}
			},
			profile:       "test-profile",
			wantErr:       true,
			expectedError: "invalid expiration time format",
		},
		{
			name: "no access token in cache",
			setup: func(t *testing.T) *sso.RealAWSSSOClient {
				homeDir := createTempAWSConfig(t, `
[profile test-profile]
sso_session = test-session

[sso-session test-session]
sso_start_url = https://test-start-url
`)
				cacheDir := filepath.Join(homeDir, ".aws", "sso", "cache")
				expiry := time.Now().Add(1 * time.Hour).Format(time.RFC3339)
				cache := models.SSOCache{
					AccessToken: nil,
					ExpiresAt:   &expiry,
					StartURL:    stringPtr("https://test-start-url"),
				}
				createTempSSOCache(t, cacheDir, cache)
				return &sso.RealAWSSSOClient{
					TokenCache: models.TokenCache{},
					GetHomeDir: func() (string, error) { return homeDir, nil },
				}
			},
			profile:       "test-profile",
			wantErr:       true,
			expectedError: "no access token found in cache",
		},
		{
			name: "token refresh failure",
			setup: func(t *testing.T) *sso.RealAWSSSOClient {
				homeDir := createTempAWSConfig(t, `
[profile test-profile]
sso_session = test-session

[sso-session test-session]
sso_start_url = https://test-start-url
`)
				cacheDir := filepath.Join(homeDir, ".aws", "sso", "cache")
				pastExpiry := time.Now().Add(-1 * time.Hour).Format(time.RFC3339)
				createTempSSOCache(t, cacheDir, models.SSOCache{
					AccessToken: stringPtr("expired-token"),
					ExpiresAt:   &pastExpiry,
					StartURL:    stringPtr("https://test-start-url"),
				})
				mockExecutor := &MockCommandExecutor{}
				mockExecutor.On("RunInteractiveCommand", mock.Anything, "aws", "sso", "login", "--profile", "test-profile").Return(errors.New("login failed"))
				mockCreds := &MockAWSCredentialsClient{}
				mockCreds.On("IsCallerIdentityValid", "test-profile").Return(false)
				return &sso.RealAWSSSOClient{
					Executor:          mockExecutor,
					CredentialsClient: mockCreds,
					GetHomeDir:        func() (string, error) { return homeDir, nil },
				}
			},
			profile:       "test-profile",
			wantErr:       true,
			expectedError: "SSO login failed",
		},
		{
			name: "failed_to_read_cache_directory",
			setup: func(t *testing.T) *sso.RealAWSSSOClient {
				homeDir := createTempAWSConfig(t, `
		[profile test-profile]
		sso_session = test-session

		[sso-session test-session]
		sso_start_url = https://test-start-url
		`)
				cacheDir := filepath.Join(homeDir, ".aws", "sso", "cache")
				err := os.MkdirAll(cacheDir, 0755)
				require.NoError(t, err)
				err = os.Chmod(cacheDir, 0000)
				require.NoError(t, err)
				t.Cleanup(func() {
					if err := os.Chmod(cacheDir, 0755); err != nil && !os.IsNotExist(err) {
						t.Errorf("failed to restore cacheDir permissions: %v", err)
					}
					if err := os.RemoveAll(homeDir); err != nil {
						t.Errorf("failed to remove temp directory %s: %v", homeDir, err)
					}
				})
				return &sso.RealAWSSSOClient{
					GetHomeDir: func() (string, error) { return homeDir, nil },
				}
			},
			profile:       "test-profile",
			wantErr:       true,
			expectedError: "failed to read SSO cache directory",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := tt.setup(t)
			token, err := client.GetCachedSsoAccessToken(tt.profile)
			if tt.wantErr {
				require.Error(t, err)
				if tt.expectedError != "" {
					assert.Contains(t, err.Error(), tt.expectedError)
				}
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantToken, token)
		})
	}
}

func TestConfigureSSO(t *testing.T) {
	tests := []struct {
		name          string
		mockSetup     func(*MockCommandExecutor)
		wantErr       bool
		expectedError string
	}{
		{
			name: "successful configuration",
			mockSetup: func(m *MockCommandExecutor) {
				m.On("RunInteractiveCommand", mock.Anything, "aws", "configure", "sso").Return(nil)
			},
			wantErr: false,
		},
		{
			name: "user interrupt",
			mockSetup: func(m *MockCommandExecutor) {
				exitErr := &exec.ExitError{
					Stderr:       []byte("interrupted"),
					ProcessState: &os.ProcessState{},
				}
				m.On("RunInteractiveCommand", mock.Anything, "aws", "configure", "sso").
					Return(exitErr)
			},
			wantErr:       true,
			expectedError: "failed to configure AWS SSO:",
		},
		{
			name: "command failure",
			mockSetup: func(m *MockCommandExecutor) {
				m.On("RunInteractiveCommand", mock.Anything, "aws", "configure", "sso").
					Return(errors.New("command failed"))
			},
			wantErr:       true,
			expectedError: "failed to configure AWS SSO",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExecutor := &MockCommandExecutor{}
			tt.mockSetup(mockExecutor)
			client := &sso.RealAWSSSOClient{
				Executor: mockExecutor,
			}
			err := client.ConfigureSSO()
			if tt.wantErr {
				require.Error(t, err)
				if tt.expectedError != "" {
					assert.Contains(t, err.Error(), tt.expectedError)
				}
				return
			}
			require.NoError(t, err)
			mockExecutor.AssertExpectations(t)
		})
	}
}

func TestGetSSOProfiles(t *testing.T) {
	tests := []struct {
		name          string
		setup         func(t *testing.T) *sso.RealAWSSSOClient
		wantProfiles  []string
		wantErr       bool
		expectedError string
	}{
		{
			name: "valid profiles",
			setup: func(t *testing.T) *sso.RealAWSSSOClient {
				homeDir := createTempAWSConfig(t, `
[profile test-profile]
sso_session = test-session

[sso-session test-session]
sso_start_url = https://test-start-url
`)
				return &sso.RealAWSSSOClient{
					GetHomeDir: func() (string, error) { return homeDir, nil },
				}
			},
			wantProfiles: []string{"test-profile"},
			wantErr:      false,
		},
		{
			name: "no config file",
			setup: func(t *testing.T) *sso.RealAWSSSOClient {
				homeDir, err := os.MkdirTemp("", "aws_test")
				require.NoError(t, err)
				return &sso.RealAWSSSOClient{
					GetHomeDir: func() (string, error) { return homeDir, nil },
				}
			},
			wantErr:       true,
			expectedError: "failed to open AWS config file",
		},
		{
			name: "failed to get home directory",
			setup: func(t *testing.T) *sso.RealAWSSSOClient {
				return &sso.RealAWSSSOClient{
					GetHomeDir: func() (string, error) { return "", errors.New("home dir error") },
				}
			},
			wantErr:       true,
			expectedError: "failed to get user home directory",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := tt.setup(t)
			homeDir, err := client.GetHomeDir()
			if err == nil && homeDir != "" {
				defer func() {
					if err := os.RemoveAll(homeDir); err != nil {
						t.Logf("failed to remove temp home dir: %v", err)
					}
				}()
			}
			profiles, err := client.GetSSOProfiles()
			if tt.wantErr {
				require.Error(t, err)
				if tt.expectedError != "" {
					assert.Contains(t, err.Error(), tt.expectedError)
				}
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantProfiles, profiles)
		})
	}
}

func TestGetSSOAccountName(t *testing.T) {
	tests := []struct {
		name          string
		setup         func(t *testing.T) *sso.RealAWSSSOClient
		accountID     string
		profile       string
		wantName      string
		wantErr       bool
		expectedError string
	}{
		{
			name: "valid account name",
			setup: func(t *testing.T) *sso.RealAWSSSOClient {
				mockExecutor := &MockCommandExecutor{}
				response := struct {
					AccountList []models.SSOAccount `json:"accountList"`
				}{
					AccountList: []models.SSOAccount{
						{AccountID: "123456789012", AccountName: "test-account"},
					},
				}
				jsonResponse, _ := json.Marshal(response)
				mockExecutor.On("RunCommand", "aws", "sso", "list-accounts", "--access-token", "test-token", "--output", "json").
					Return(jsonResponse, nil)
				client := &sso.RealAWSSSOClient{
					Executor: mockExecutor,
					TokenCache: models.TokenCache{
						AccessToken: "test-token",
						Expiry:      time.Now().Add(1 * time.Hour),
					},
				}
				return client
			},
			accountID: "123456789012",
			profile:   "test-profile",
			wantName:  "test-account",
			wantErr:   false,
		},
		{
			name: "account not found",
			setup: func(t *testing.T) *sso.RealAWSSSOClient {
				mockExecutor := &MockCommandExecutor{}
				response := struct {
					AccountList []models.SSOAccount `json:"accountList"`
				}{
					AccountList: []models.SSOAccount{},
				}
				jsonResponse, _ := json.Marshal(response)
				mockExecutor.On("RunCommand", "aws", "sso", "list-accounts", "--access-token", "test-token", "--output", "json").
					Return(jsonResponse, nil)
				client := &sso.RealAWSSSOClient{
					Executor: mockExecutor,
					TokenCache: models.TokenCache{
						AccessToken: "test-token",
						Expiry:      time.Now().Add(1 * time.Hour),
					},
				}
				return client
			},
			accountID:     "123456789012",
			profile:       "test-profile",
			wantErr:       true,
			expectedError: "account ID 123456789012 not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := tt.setup(t)
			name, err := client.GetSSOAccountName(tt.accountID, tt.profile)
			if tt.wantErr {
				require.Error(t, err)
				if tt.expectedError != "" {
					assert.Contains(t, err.Error(), tt.expectedError)
				}
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantName, name)
		})
	}
}

func TestGetSSORoles(t *testing.T) {
	tests := []struct {
		name          string
		setup         func(t *testing.T) *sso.RealAWSSSOClient
		profile       string
		accountID     string
		wantRoles     []string
		wantErr       bool
		expectedError string
	}{
		{
			name: "valid roles",
			setup: func(t *testing.T) *sso.RealAWSSSOClient {
				mockExecutor := &MockCommandExecutor{}
				response := struct {
					RoleList []struct {
						RoleName string `json:"roleName"`
					} `json:"roleList"`
				}{
					RoleList: []struct {
						RoleName string `json:"roleName"`
					}{{RoleName: "Admin"}},
				}
				jsonResponse, _ := json.Marshal(response)

				mockExecutor.On("RunCommand", "aws", "sso", "list-account-roles", "--profile", "test-profile", "--account-id", "123456789012", "--access-token", "test-token", "--output", "json").
					Return(jsonResponse, nil)

				client := &sso.RealAWSSSOClient{
					Executor: mockExecutor,
					TokenCache: models.TokenCache{
						AccessToken: "test-token",
						Expiry:      time.Now().Add(1 * time.Hour),
					},
				}
				return client
			},
			profile:   "test-profile",
			accountID: "123456789012",
			wantRoles: []string{"Admin"},
			wantErr:   false,
		},
		{
			name: "no roles found",
			setup: func(t *testing.T) *sso.RealAWSSSOClient {
				mockExecutor := &MockCommandExecutor{}
				response := struct {
					RoleList []struct {
						RoleName string `json:"roleName"`
					} `json:"roleList"`
				}{
					RoleList: []struct {
						RoleName string `json:"roleName"`
					}{},
				}
				jsonResponse, _ := json.Marshal(response)

				mockExecutor.On("RunCommand", "aws", "sso", "list-account-roles", "--profile", "test-profile", "--account-id", "123456789012", "--access-token", "test-token", "--output", "json").
					Return(jsonResponse, nil)

				client := &sso.RealAWSSSOClient{
					Executor: mockExecutor,
					TokenCache: models.TokenCache{
						AccessToken: "test-token",
						Expiry:      time.Now().Add(1 * time.Hour),
					},
				}
				return client
			},
			profile:       "test-profile",
			accountID:     "123456789012",
			wantErr:       true,
			expectedError: "no roles found for AWS account 123456789012",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := tt.setup(t)
			roles, err := client.GetSSORoles(tt.profile, tt.accountID)
			if tt.wantErr {
				require.Error(t, err)
				if tt.expectedError != "" {
					assert.Contains(t, err.Error(), tt.expectedError)
				}
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantRoles, roles)
		})
	}
}

func TestGetSessionName(t *testing.T) {
	tests := []struct {
		name          string
		setup         func(t *testing.T) *sso.RealAWSSSOClient
		profile       string
		wantSession   string
		wantErr       bool
		expectedError string
	}{
		{
			name: "valid session from sso_session",
			setup: func(t *testing.T) *sso.RealAWSSSOClient {
				homeDir := createTempAWSConfig(t, `
[profile test-profile]
sso_session = test-session

[sso-session test-session]
sso_start_url = https://test-start-url
`)
				return &sso.RealAWSSSOClient{
					GetHomeDir: func() (string, error) { return homeDir, nil },
				}
			},
			profile:     "test-profile",
			wantSession: "https://test-start-url",
			wantErr:     false,
		},
		{
			name: "valid session from sso_start_url",
			setup: func(t *testing.T) *sso.RealAWSSSOClient {
				homeDir := createTempAWSConfig(t, `
[profile test-profile]
sso_start_url = https://test-start-url
`)
				return &sso.RealAWSSSOClient{
					GetHomeDir: func() (string, error) { return homeDir, nil },
				}
			},
			profile:     "test-profile",
			wantSession: "https://test-start-url",
			wantErr:     false,
		},
		{
			name: "profile not found",
			setup: func(t *testing.T) *sso.RealAWSSSOClient {
				homeDir := createTempAWSConfig(t, `
[profile other-profile]
sso_start_url = https://test-start-url
`)
				return &sso.RealAWSSSOClient{
					GetHomeDir: func() (string, error) { return homeDir, nil },
				}
			},
			profile:       "test-profile",
			wantErr:       true,
			expectedError: "profile 'test-profile' not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := tt.setup(t)
			homeDir, err := client.GetHomeDir()
			if err == nil && homeDir != "" {
				defer func() {
					if err := os.RemoveAll(homeDir); err != nil {
						t.Logf("failed to remove temp home dir: %v", err)
					}
				}()
			}
			session, err := client.GetSessionName(tt.profile)
			if tt.wantErr {
				require.Error(t, err)
				if tt.expectedError != "" {
					assert.Contains(t, err.Error(), tt.expectedError)
				}
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantSession, session)
		})
	}
}

func TestSSOLogin(t *testing.T) {
	tests := []struct {
		name          string
		setup         func(t *testing.T) *sso.RealAWSSSOClient
		profile       string
		refresh       bool
		noBrowser     bool
		wantErr       bool
		expectedError string
	}{
		{
			name: "successful login",
			setup: func(t *testing.T) *sso.RealAWSSSOClient {
				mockExecutor := &MockCommandExecutor{}
				mockExecutor.On("RunInteractiveCommand", mock.Anything, "aws", "sso", "login", "--profile", "test-profile").Return(nil)
				mockCreds := &MockAWSCredentialsClient{}
				mockCreds.On("IsCallerIdentityValid", "test-profile").Return(false)
				return &sso.RealAWSSSOClient{
					Executor:          mockExecutor,
					CredentialsClient: mockCreds,
				}
			},
			profile:   "test-profile",
			refresh:   false,
			noBrowser: false,
			wantErr:   false,
		},
		{
			name: "login timeout",
			setup: func(t *testing.T) *sso.RealAWSSSOClient {
				mockExecutor := &MockCommandExecutor{}
				mockExecutor.On("RunInteractiveCommand", mock.Anything, "aws", "sso", "login", "--profile", "test-profile").
					Return(context.DeadlineExceeded)
				mockCreds := &MockAWSCredentialsClient{}
				mockCreds.On("IsCallerIdentityValid", "test-profile").Return(false)
				return &sso.RealAWSSSOClient{
					Executor:          mockExecutor,
					CredentialsClient: mockCreds,
				}
			},
			profile:       "test-profile",
			refresh:       false,
			noBrowser:     false,
			wantErr:       true,
			expectedError: "error during SSO login: context deadline exceeded",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := tt.setup(t)
			err := client.SSOLogin(tt.profile, tt.refresh, tt.noBrowser)
			if tt.wantErr {
				require.Error(t, err)
				if tt.expectedError != "" {
					assert.Contains(t, err.Error(), tt.expectedError)
				}
				return
			}
			require.NoError(t, err)
			if mockExecutor, ok := client.Executor.(*MockCommandExecutor); ok {
				mockExecutor.AssertExpectations(t)
			}
		})
	}
}
