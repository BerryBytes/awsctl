package sso_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/BerryBytes/awsctl/internal/sso"
	"github.com/BerryBytes/awsctl/internal/sso/config"
	"github.com/BerryBytes/awsctl/models"
	mock_awsctl "github.com/BerryBytes/awsctl/tests/mock"
	mock_sso "github.com/BerryBytes/awsctl/tests/mock/sso"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockPrompt struct {
	method       string
	label        string
	defaultValue string
	response     string
	err          error
}

func TestLoadOrCreateSession(t *testing.T) {
	tests := []struct {
		name           string
		initialConfig  *models.Config
		nameParam      string
		startURLParam  string
		regionParam    string
		mockPrompts    []mockPrompt
		wantSession    *models.SSOSession
		wantConfigPath string
		wantErr        bool
		errContains    string
	}{
		{
			name: "Create new session with parameters",
			initialConfig: &models.Config{
				SSOSessions: []models.SSOSession{},
			},
			nameParam:     "test-session",
			startURLParam: "https://test.awsapps.com/start",
			regionParam:   "us-west-2",
			wantSession: &models.SSOSession{
				Name:     "test-session",
				StartURL: "https://test.awsapps.com/start",
				Region:   "us-west-2",
				Scopes:   "sso:account:access",
			},
		},
		{
			name: "Create new session interactively",
			initialConfig: &models.Config{
				SSOSessions: []models.SSOSession{},
			},
			mockPrompts: []mockPrompt{
				{"PromptWithDefault", "SSO session name", "default-sso", "test-session", nil},
				{"PromptRequired", "SSO start URL (e.g., https://my-sso-portal.awsapps.com/start)", "", "https://test.awsapps.com/start", nil},
				{"PromptForRegion", "us-east-1", "us-east-1", "us-west-2", nil},
			},
			wantSession: &models.SSOSession{
				Name:     "test-session",
				StartURL: "https://test.awsapps.com/start",
				Region:   "us-west-2",
				Scopes:   "sso:account:access",
			},
		},
		{
			name: "Region prompt error",
			initialConfig: &models.Config{
				SSOSessions: []models.SSOSession{},
			},
			mockPrompts: []mockPrompt{
				{"PromptWithDefault", "SSO session name", "default-sso", "test-session", nil},
				{"PromptRequired", "SSO start URL (e.g., https://my-sso-portal.awsapps.com/start)", "", "https://test.awsapps.com/start", nil},
				{"PromptForRegion", "us-east-1", "us-east-1", "", errors.New("invalid region")},
			},
			wantErr:     true,
			errContains: "failed to prompt for SSO region",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockPrompter := mock_sso.NewMockPrompter(ctrl)

			for _, mp := range tt.mockPrompts {
				switch mp.method {
				case "PromptWithDefault":
					mockPrompter.EXPECT().
						PromptWithDefault(mp.label, mp.defaultValue).
						Return(mp.response, mp.err)
				case "PromptRequired":
					mockPrompter.EXPECT().
						PromptRequired(mp.label).
						Return(mp.response, mp.err)
				case "PromptForRegion":
					mockPrompter.EXPECT().
						PromptForRegion(mp.defaultValue).
						Return(mp.response, mp.err)
				}
			}

			client := &sso.RealSSOClient{
				Prompter: mockPrompter,
				Config: config.Config{
					RawCustomConfig: tt.initialConfig,
				},
			}

			configPath, session, err := client.LoadOrCreateSession(tt.nameParam, tt.startURLParam, tt.regionParam)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				return
			}

			assert.NoError(t, err)
			if tt.wantConfigPath != "" {
				assert.Equal(t, tt.wantConfigPath, configPath)
			}
			if tt.wantSession != nil {
				assert.Equal(t, tt.wantSession.Name, session.Name)
				assert.Equal(t, tt.wantSession.StartURL, session.StartURL)
				assert.Equal(t, tt.wantSession.Region, session.Region)
				assert.Equal(t, tt.wantSession.Scopes, session.Scopes)
			}
		})
	}
}
func TestSelectSSOSession(t *testing.T) {
	tests := []struct {
		name          string
		initialConfig *models.Config
		mockPrompts   []mockPrompt
		wantSession   *models.SSOSession
		wantErr       bool
		errContains   string
	}{
		{
			name: "Select existing session from multiple",
			initialConfig: &models.Config{
				SSOSessions: []models.SSOSession{
					{
						Name:     "session1",
						StartURL: "https://session1.awsapps.com/start",
						Region:   "us-east-1",
					},
					{
						Name:     "session2",
						StartURL: "https://session2.awsapps.com/start",
						Region:   "us-west-2",
					},
				},
			},
			mockPrompts: []mockPrompt{
				{
					method:   "SelectFromList",
					label:    "Select an SSO session",
					response: "session1 (https://session1.awsapps.com/start)",
				},
			},
			wantSession: &models.SSOSession{
				Name:     "session1",
				StartURL: "https://session1.awsapps.com/start",
				Region:   "us-east-1",
				Scopes:   "sso:account:access",
			},
		},
		{
			name: "Choose to create new session",
			initialConfig: &models.Config{
				SSOSessions: []models.SSOSession{
					{
						Name:     "session1",
						StartURL: "https://session1.awsapps.com/start",
						Region:   "us-east-1",
					},
				},
			},
			mockPrompts: []mockPrompt{
				{
					method:   "SelectFromList",
					label:    "Select an SSO session",
					response: "Create new session",
				},
			},
			wantSession: nil,
		},
		{
			name: "Error selecting session",
			initialConfig: &models.Config{
				SSOSessions: []models.SSOSession{
					{
						Name:     "session1",
						StartURL: "https://session1.awsapps.com/start",
						Region:   "us-east-1",
					},
				},
			},
			mockPrompts: []mockPrompt{
				{
					method: "SelectFromList",
					label:  "Select an SSO session",
					err:    errors.New("selection error"),
				},
			},
			wantErr:     true,
			errContains: "failed to select SSO session",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockPrompter := mock_sso.NewMockPrompter(ctrl)

			for _, mp := range tt.mockPrompts {
				switch mp.method {
				case "SelectFromList":
					mockPrompter.EXPECT().
						SelectFromList(mp.label, gomock.Any()).
						Return(mp.response, mp.err)
				}
			}

			client := &sso.RealSSOClient{
				Prompter: mockPrompter,
				Config: config.Config{
					RawCustomConfig: tt.initialConfig,
				},
			}

			session, err := client.SelectSSOSession()

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				return
			}

			assert.NoError(t, err)
			if tt.wantSession == nil {
				assert.Nil(t, session)
			} else {
				assert.Equal(t, tt.wantSession.Name, session.Name)
				assert.Equal(t, tt.wantSession.StartURL, session.StartURL)
				assert.Equal(t, tt.wantSession.Region, session.Region)
				assert.Equal(t, tt.wantSession.Scopes, session.Scopes)
			}
		})
	}
}

func TestRunSSOLogin(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(t *testing.T) string
		sessionName string
		mockExec    func(*mock_awsctl.MockCommandExecutor)
		wantErr     bool
		errContains string
	}{
		{
			name: "Successful login",
			setup: func(t *testing.T) string {
				dir, err := os.MkdirTemp("", "awsctl-test")
				require.NoError(t, err)

				awsDir := filepath.Join(dir, ".aws")
				err = os.Mkdir(awsDir, 0700)
				require.NoError(t, err)

				configPath := filepath.Join(awsDir, "config")
				content := `[sso-session test-session]
sso_start_url = https://test.awsapps.com/start
sso_region = us-west-2
`
				err = os.WriteFile(configPath, []byte(content), 0600)
				require.NoError(t, err)

				return dir
			},
			sessionName: "test-session",
			mockExec: func(m *mock_awsctl.MockCommandExecutor) {
				m.EXPECT().
					RunInteractiveCommand(gomock.Any(), "aws", "sso", "login", "--sso-session", "test-session").
					Return(nil)
			},
		},
		{
			name: "Invalid configuration",
			setup: func(t *testing.T) string {
				dir, err := os.MkdirTemp("", "awsctl-test")
				require.NoError(t, err)
				return dir
			},
			sessionName: "missing-session",
			wantErr:     true,
			errContains: "invalid SSO configuration",
		},
		{
			name: "Login command fails",
			setup: func(t *testing.T) string {
				dir, err := os.MkdirTemp("", "awsctl-test")
				require.NoError(t, err)

				awsDir := filepath.Join(dir, ".aws")
				err = os.Mkdir(awsDir, 0700)
				require.NoError(t, err)

				configPath := filepath.Join(awsDir, "config")
				content := `[sso-session test-session]
sso_start_url = https://test.awsapps.com/start
sso_region = us-west-2
`
				err = os.WriteFile(configPath, []byte(content), 0600)
				require.NoError(t, err)

				return dir
			},
			sessionName: "test-session",
			mockExec: func(m *mock_awsctl.MockCommandExecutor) {
				m.EXPECT().
					RunInteractiveCommand(gomock.Any(), "aws", "sso", "login", "--sso-session", "test-session").
					Return(errors.New("login failed"))
			},
			wantErr:     true,
			errContains: "error during SSO login",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			tempHome := tt.setup(t)
			defer func() {
				if err := os.RemoveAll(tempHome); err != nil {
					t.Logf("failed to remove temp home: %v", err)
				}
			}()

			originalHome := os.Getenv("HOME")
			if err := os.Setenv("HOME", tempHome); err != nil {
				t.Fatalf("failed to set HOME to tempDir: %v", err)
			}
			defer func() {
				if err := os.Setenv("HOME", originalHome); err != nil {
					t.Logf("warning: failed to reset HOME environment variable: %v", err)
				}
			}()

			mockExecutor := mock_awsctl.NewMockCommandExecutor(ctrl)
			if tt.mockExec != nil {
				tt.mockExec(mockExecutor)
			}

			client := &sso.RealSSOClient{
				Executor: mockExecutor,
			}

			err := client.RunSSOLogin(tt.sessionName)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				return
			}

			assert.NoError(t, err)
		})
	}
}

func TestGetAccessToken(t *testing.T) {
	tests := []struct {
		name        string
		setup       func() string
		startURL    string
		wantToken   string
		wantErr     bool
		errContains string
	}{
		{
			name: "Valid token found",
			setup: func() string {
				dir, _ := os.MkdirTemp("", "awsctl-test")
				ssoDir := filepath.Join(dir, ".aws", "sso", "cache")
				require.NoError(t, os.MkdirAll(ssoDir, 0700))

				tokenFile := filepath.Join(ssoDir, "token.json")
				expiresAt := time.Now().Add(1 * time.Hour).Format(time.RFC3339)
				content := fmt.Sprintf(`{
					"startUrl": "https://test.awsapps.com/start",
					"accessToken": "test-token",
					"expiresAt": "%s"
				}`, expiresAt)
				require.NoError(t, os.WriteFile(tokenFile, []byte(content), 0600))

				return dir
			},
			startURL:  "https://test.awsapps.com/start",
			wantToken: "test-token",
		},
		{
			name: "Expired token",
			setup: func() string {
				dir, _ := os.MkdirTemp("", "awsctl-test")
				ssoDir := filepath.Join(dir, ".aws", "sso", "cache")
				require.NoError(t, os.MkdirAll(ssoDir, 0700))

				tokenFile := filepath.Join(ssoDir, "token.json")
				expiresAt := time.Now().Add(-1 * time.Hour).Format(time.RFC3339)
				content := fmt.Sprintf(`{
					"startUrl": "https://test.awsapps.com/start",
					"accessToken": "test-token",
					"expiresAt": "%s"
				}`, expiresAt)
				require.NoError(t, os.WriteFile(tokenFile, []byte(content), 0600))

				return dir
			},
			startURL:    "https://test.awsapps.com/start",
			wantErr:     true,
			errContains: "access token expired",
		},
		{
			name: "No token found",
			setup: func() string {
				dir, _ := os.MkdirTemp("", "awsctl-test")
				require.NoError(t, os.MkdirAll(filepath.Join(dir, ".aws"), 0700))
				return dir
			},
			startURL:    "https://test.awsapps.com/start",
			wantErr:     true,
			errContains: "failed to read SSO cache directory:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := tt.setup()
			defer func() {
				if err := os.RemoveAll(tempDir); err != nil {
					t.Logf("failed to remove temp dir: %v", err)
				}
			}()
			originalHome := os.Getenv("HOME")
			if err := os.Setenv("HOME", tempDir); err != nil {
				t.Fatalf("failed to set HOME env: %v", err)
			}
			defer func() {
				if err := os.Setenv("HOME", originalHome); err != nil {
					t.Logf("warning: failed to reset HOME environment variable: %v", err)
				}
			}()

			client := &sso.RealSSOClient{}

			token, err := client.GetAccessToken(tt.startURL)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.wantToken, token)
		})
	}
}

func TestConfigureSSOSession(t *testing.T) {
	tests := []struct {
		name        string
		setup       func() string
		sessionName string
		startURL    string
		region      string
		scopes      string
		wantErr     bool
		errContains string
	}{
		{
			name: "Create new config file",
			setup: func() string {
				dir, _ := os.MkdirTemp("", "awsctl-test")
				return dir
			},
			sessionName: "test-session",
			startURL:    "https://test.awsapps.com/start",
			region:      "us-west-2",
			scopes:      "sso:account:access",
		},
		{
			name: "Update existing config file",
			setup: func() string {
				dir, _ := os.MkdirTemp("", "awsctl-test")
				configPath := filepath.Join(dir, "config")
				require.NoError(t, os.WriteFile(configPath, []byte("[sso-session existing]\nsso_start_url = old\n"), 0600))

				return dir
			},
			sessionName: "test-session",
			startURL:    "https://test.awsapps.com/start",
			region:      "us-west-2",
			scopes:      "sso:account:access",
		},
		{
			name: "Skip identical configuration",
			setup: func() string {
				dir, _ := os.MkdirTemp("", "awsctl-test")
				configPath := filepath.Join(dir, "config")
				content := `[sso-session test-session]
sso_start_url = https://test.awsapps.com/start
sso_region = us-west-2
sso_registration_scopes = sso:account:access
`
				require.NoError(t, os.WriteFile(configPath, []byte(content), 0600))

				return dir
			},
			sessionName: "test-session",
			startURL:    "https://test.awsapps.com/start",
			region:      "us-west-2",
			scopes:      "sso:account:access",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempAwsDir := tt.setup()
			defer func() {
				if err := os.RemoveAll(tempAwsDir); err != nil {
					t.Logf("warning: failed to remove temp AWS dir: %v", err)
				}
			}()
			defer func() {
				if err := os.RemoveAll(tempAwsDir); err != nil {
					t.Logf("failed to remove temp dir: %v", err)
				}
			}()

			originalHome := os.Getenv("HOME")
			tempHome, _ := os.MkdirTemp("", "awsctl-test-home")
			defer func() {
				if err := os.RemoveAll(tempHome); err != nil {
					t.Logf("warning: failed to remove temp AWS dir: %v", err)
				}
			}()

			require.NoError(t, os.Symlink(tempAwsDir, filepath.Join(tempHome, ".aws")))
			if err := os.Setenv("HOME", tempHome); err != nil {
				t.Fatalf("failed to set HOME to tempDir: %v", err)
			}
			defer func() {
				if err := os.Setenv("HOME", originalHome); err != nil {
					t.Logf("warning: failed to reset HOME environment variable: %v", err)
				}
			}()

			client := &sso.RealSSOClient{}

			err := client.ConfigureSSOSession(tt.sessionName, tt.startURL, tt.region, tt.scopes)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				return
			}

			assert.NoError(t, err)

			configPath := filepath.Join(tempAwsDir, "config")
			content, err := os.ReadFile(configPath)
			assert.NoError(t, err)

			if tt.sessionName != "" {
				assert.Contains(t, string(content), fmt.Sprintf("[sso-session %s]", tt.sessionName))
				assert.Contains(t, string(content), fmt.Sprintf("sso_start_url = %s", tt.startURL))
				assert.Contains(t, string(content), fmt.Sprintf("sso_region = %s", tt.region))
				assert.Contains(t, string(content), fmt.Sprintf("sso_registration_scopes = %s", tt.scopes))
			}
		})
	}
}
