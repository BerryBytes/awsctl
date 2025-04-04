package sso

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/BerryBytes/awsctl/models"
	mock_awsctl "github.com/BerryBytes/awsctl/tests/mock"
	promptUtils "github.com/BerryBytes/awsctl/utils/prompt"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func TestAWSSelectionClient(t *testing.T) {
	validConfig := &models.Config{
		Aws: struct {
			Profiles []models.SSOProfile `json:"profiles" yaml:"profiles"`
		}{
			Profiles: []models.SSOProfile{
				{
					ProfileName: "profile1",
					Accounts: []models.SSOAccount{
						{AccountName: "account1", Roles: []string{"role1"}},
					},
				},
			},
		},
	}

	emptyConfig := &models.Config{
		Aws: struct {
			Profiles []models.SSOProfile `json:"profiles" yaml:"profiles"`
		}{},
	}

	tests := []struct {
		name     string
		setup    func(*mock_awsctl.MockPrompter)
		testFunc func(*RealAWSSelectionClient) error
		wantErr  bool
		errType  error
	}{
		{
			name: "FindProfile success",
			testFunc: func(c *RealAWSSelectionClient) error {
				_, err := c.FindProfile(validConfig, "profile1")
				return err
			},
		},
		{
			name: "FindProfile not found",
			testFunc: func(c *RealAWSSelectionClient) error {
				_, err := c.FindProfile(validConfig, "missing")
				return err
			},
			wantErr: true,
		},

		{
			name: "SelectProfile success",
			setup: func(m *mock_awsctl.MockPrompter) {
				m.EXPECT().PromptForSelection("Select an AWS SSO Profile", []string{"p1"}).
					Return("p1", nil)
			},
			testFunc: func(c *RealAWSSelectionClient) error {
				_, err := c.SelectProfile([]string{"p1"})
				return err
			},
		},
		{
			name: "SelectProfile interrupted",
			setup: func(m *mock_awsctl.MockPrompter) {
				m.EXPECT().PromptForSelection(gomock.Any(), gomock.Any()).
					Return("", promptUtils.ErrInterrupted)
			},
			testFunc: func(c *RealAWSSelectionClient) error {
				_, err := c.SelectProfile([]string{"p1"})
				return err
			},
			wantErr: true,
			errType: promptUtils.ErrInterrupted,
		},

		{
			name: "SelectProfileFromConfig success",
			setup: func(m *mock_awsctl.MockPrompter) {
				m.EXPECT().PromptForSelection(gomock.Any(), gomock.Any()).
					Return("profile1", nil)
			},
			testFunc: func(c *RealAWSSelectionClient) error {
				_, err := c.SelectProfileFromConfig(validConfig)
				return err
			},
		},
		{
			name: "SelectProfileFromConfig no profiles",
			testFunc: func(c *RealAWSSelectionClient) error {
				_, err := c.SelectProfileFromConfig(emptyConfig)
				return err
			},
			wantErr: true,
		},

		{
			name: "SelectRoleFromAccount success",
			setup: func(m *mock_awsctl.MockPrompter) {
				m.EXPECT().PromptForSelection("Select an AWS Role", []string{"role1"}).
					Return("role1", nil)
			},
			testFunc: func(c *RealAWSSelectionClient) error {
				_, err := c.SelectRoleFromAccount(&models.SSOAccount{Roles: []string{"role1"}})
				return err
			},
		},
		{
			name: "SelectRoleFromAccount no roles",
			testFunc: func(c *RealAWSSelectionClient) error {
				_, err := c.SelectRoleFromAccount(&models.SSOAccount{Roles: []string{}})
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockPrompter := mock_awsctl.NewMockPrompter(ctrl)
			if tt.setup != nil {
				tt.setup(mockPrompter)
			}

			client := &RealAWSSelectionClient{Prompter: mockPrompter}
			err := tt.testFunc(client)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errType != nil {
					assert.True(t, errors.Is(err, tt.errType),
						"expected error type %v, got %v", tt.errType, err)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestExtractAccountNames(t *testing.T) {
	profile := &models.SSOProfile{
		Accounts: []models.SSOAccount{
			{AccountName: "a1"},
			{AccountName: "a2"},
		},
	}

	client := &RealAWSSelectionClient{}
	names := client.ExtractAccountNames(profile)

	assert.Equal(t, []string{"a1", "a2"}, names)
}

func TestGetUniqueProfiles(t *testing.T) {
	cfg := &models.Config{
		Aws: struct {
			Profiles []models.SSOProfile `json:"profiles" yaml:"profiles"`
		}{
			Profiles: []models.SSOProfile{
				{ProfileName: "p1"},
				{ProfileName: "p1"},
				{ProfileName: "p2"},
			},
		},
	}

	client := &RealAWSSelectionClient{}
	profiles, err := client.GetUniqueProfiles(cfg)

	assert.NoError(t, err)
	assert.Equal(t, []string{"p1", "p2"}, profiles)
}

func TestFindAccount(t *testing.T) {
	tests := []struct {
		name        string
		profile     *models.SSOProfile
		accountName string
		wantErr     bool
	}{
		{
			name: "account found",
			profile: &models.SSOProfile{
				ProfileName: "profile1",
				Accounts: []models.SSOAccount{
					{AccountName: "account1"},
				},
			},
			accountName: "account1",
		},
		{
			name: "account not found",
			profile: &models.SSOProfile{
				ProfileName: "profile1",
				Accounts: []models.SSOAccount{
					{AccountName: "account1"},
				},
			},
			accountName: "missing",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &RealAWSSelectionClient{}
			account, err := client.FindAccount(tt.profile, tt.accountName)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), fmt.Sprintf("account %s not found in profile %s",
					tt.accountName, tt.profile.ProfileName))
				assert.Nil(t, account)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.accountName, account.AccountName)
			}
		})
	}
}

func TestSelectAccount(t *testing.T) {
	tests := []struct {
		name        string
		accounts    []models.SSOAccount
		mockReturn  string
		mockError   error
		wantErr     bool
		wantErrType error
		errContains string
	}{
		{
			name: "successful selection",
			accounts: []models.SSOAccount{
				{AccountName: "account1"},
				{AccountName: "account2"},
			},
			mockReturn: "account1",
		},
		{
			name: "interrupted selection",
			accounts: []models.SSOAccount{
				{AccountName: "account1"},
			},
			mockError:   promptUtils.ErrInterrupted,
			wantErr:     true,
			wantErrType: promptUtils.ErrInterrupted,
			errContains: "",
		},
		{
			name: "other error",
			accounts: []models.SSOAccount{
				{AccountName: "account1"},
			},
			mockError:   fmt.Errorf("prompt error"),
			wantErr:     true,
			errContains: "account selection aborted: prompt error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockPrompter := mock_awsctl.NewMockPrompter(ctrl)

			expectedAccountNames := make([]string, len(tt.accounts))
			for i, acc := range tt.accounts {
				expectedAccountNames[i] = acc.AccountName
			}

			mockPrompter.EXPECT().
				PromptForSelection("Select an AWS Account", expectedAccountNames).
				Return(tt.mockReturn, tt.mockError)

			client := &RealAWSSelectionClient{Prompter: mockPrompter}
			_, err := client.SelectAccount(tt.accounts)

			if tt.wantErr {
				assert.Error(t, err)

				if tt.wantErrType != nil {
					assert.True(t, errors.Is(err, tt.wantErrType),
						"Expected error type %v, got %v", tt.wantErrType, err)
				}

				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
func TestSelectAccountFromProfile_ErrorCases(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(*mock_awsctl.MockPrompter)
		profile     *models.SSOProfile
		wantErr     bool
		errContains string
	}{
		{
			name: "interrupted prompt",
			profile: &models.SSOProfile{
				ProfileName: "profile1",
				Accounts: []models.SSOAccount{
					{AccountName: "account1"},
				},
			},
			setup: func(m *mock_awsctl.MockPrompter) {
				m.EXPECT().PromptForSelection(gomock.Any(), gomock.Any()).
					Return("", promptUtils.ErrInterrupted)
			},
			wantErr:     true,
			errContains: "interrupted",
		},
		{
			name: "account not found",
			profile: &models.SSOProfile{
				ProfileName: "profile1",
				Accounts: []models.SSOAccount{
					{AccountName: "account1"},
				},
			},
			setup: func(m *mock_awsctl.MockPrompter) {
				m.EXPECT().PromptForSelection(gomock.Any(), gomock.Any()).
					Return("missing", nil)
			},
			wantErr:     true,
			errContains: "failed to find account",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockPrompter := mock_awsctl.NewMockPrompter(ctrl)
			if tt.setup != nil {
				tt.setup(mockPrompter)
			}

			client := &RealAWSSelectionClient{Prompter: mockPrompter}
			_, err := client.SelectAccountFromProfile(tt.profile)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
				if strings.Contains(tt.name, "interrupted") {
					assert.True(t, errors.Is(err, promptUtils.ErrInterrupted))
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSelectRoleFromAccount_ErrorCases(t *testing.T) {
	tests := []struct {
		name       string
		account    *models.SSOAccount
		mockError  error
		wantErr    bool
		errType    error
		errMessage string
	}{
		{
			name:      "interrupted selection",
			account:   &models.SSOAccount{Roles: []string{"role1"}},
			mockError: promptUtils.ErrInterrupted,
			wantErr:   true,
			errType:   promptUtils.ErrInterrupted,
		},
		{
			name:       "other error",
			account:    &models.SSOAccount{Roles: []string{"role1"}},
			mockError:  fmt.Errorf("some error"),
			wantErr:    true,
			errMessage: "role selection aborted: role selection aborted: some error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockPrompter := mock_awsctl.NewMockPrompter(ctrl)
			mockPrompter.EXPECT().
				PromptForSelection("Select an AWS Role", tt.account.Roles).
				Return("", tt.mockError)

			client := &RealAWSSelectionClient{Prompter: mockPrompter}
			_, err := client.SelectRoleFromAccount(tt.account)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errType != nil {
					assert.True(t, errors.Is(err, tt.errType))
				}
				if tt.errMessage != "" {
					assert.Equal(t, tt.errMessage, err.Error(),
						"Error message mismatch. Check for double-wrapping of errors.")
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSelectProfileFromConfig_Interrupt(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPrompter := mock_awsctl.NewMockPrompter(ctrl)
	client := &RealAWSSelectionClient{Prompter: mockPrompter}

	cfg := &models.Config{
		Aws: struct {
			Profiles []models.SSOProfile `json:"profiles" yaml:"profiles"`
		}{
			Profiles: []models.SSOProfile{
				{ProfileName: "profile1"},
			},
		},
	}

	mockPrompter.EXPECT().
		PromptForSelection("Select an AWS SSO Profile", []string{"profile1"}).
		Return("", promptUtils.ErrInterrupted)

	_, err := client.SelectProfileFromConfig(cfg)

	assert.Error(t, err)
	assert.True(t, errors.Is(err, promptUtils.ErrInterrupted),
		"Expected ErrInterrupted, got: %v", err)
}
