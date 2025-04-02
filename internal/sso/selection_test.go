package sso

import (
	"errors"
	"fmt"
	"testing"

	"github.com/BerryBytes/awsctl/models"
	mock_sso "github.com/BerryBytes/awsctl/tests/mocks"
	promptUtils "github.com/BerryBytes/awsctl/utils/prompt"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func TestFindProfile(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAWSSelectionClient := mock_sso.NewMockAWSSelectionClient(ctrl)

	profileName := "profile1"
	cfg := &models.Config{
		Aws: struct {
			Profiles []models.SSOProfile `json:"profiles" yaml:"profiles"`
		}{
			Profiles: []models.SSOProfile{
				{
					ProfileName: "profile1",
					Region:      "us-west-1",
					AccountID:   "123456789012",
					Role:        "Admin",
					SsoStartUrl: "https://example.com/start",
					Accounts: []models.SSOAccount{
						{
							AccountID:   "123456789012",
							AccountName: "Account 1",
							SSORegion:   "us-west-1",
							Email:       "user1@example.com",
							Roles:       []string{"Admin", "User"},
						},
					},
				},
				{
					ProfileName: "profile2",
					Region:      "us-east-1",
					AccountID:   "987654321098",
					Role:        "ReadOnly",
					SsoStartUrl: "https://example.com/start",
					Accounts: []models.SSOAccount{
						{
							AccountID:   "987654321098",
							AccountName: "Account 2",
							SSORegion:   "us-east-1",
							Email:       "user2@example.com",
							Roles:       []string{"ReadOnly"},
						},
					},
				},
			},
		},
	}

	expectedProfile := &models.SSOProfile{
		ProfileName: profileName,
		Region:      "us-west-1",
		AccountID:   "123456789012",
		Role:        "Admin",
		SsoStartUrl: "https://example.com/start",
	}

	mockAWSSelectionClient.EXPECT().
		FindProfile(cfg, profileName).
		Return(expectedProfile, nil)

	profile, err := mockAWSSelectionClient.FindProfile(cfg, profileName)

	assert.NoError(t, err)
	assert.NotNil(t, profile)
	assert.Equal(t, profileName, profile.ProfileName)
	assert.Equal(t, "us-west-1", profile.Region)
}

func TestSelectProfileFromConfig(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPrompter := mock_sso.NewMockPrompter(ctrl)
	mockAWSSelectionClient := &RealAWSSelectionClient{
		Prompter: mockPrompter,
	}

	cfg := &models.Config{
		Aws: struct {
			Profiles []models.SSOProfile `json:"profiles" yaml:"profiles"`
		}{
			Profiles: []models.SSOProfile{
				{
					ProfileName: "profile1",
					Region:      "us-west-1",
					AccountID:   "123456789012",
					Role:        "Admin",
					SsoStartUrl: "https://example.com/start",
					Accounts: []models.SSOAccount{
						{
							AccountID:   "123456789012",
							AccountName: "Account 1",
							SSORegion:   "us-west-1",
							Email:       "user1@example.com",
							Roles:       []string{"Admin", "User"},
						},
					},
				},
				{
					ProfileName: "profile2",
					Region:      "us-east-1",
					AccountID:   "987654321098",
					Role:        "ReadOnly",
					SsoStartUrl: "https://example.com/start",
					Accounts: []models.SSOAccount{
						{
							AccountID:   "987654321098",
							AccountName: "Account 2",
							SSORegion:   "us-east-1",
							Email:       "user2@example.com",
							Roles:       []string{"ReadOnly"},
						},
					},
				},
			},
		},
	}

	mockPrompter.EXPECT().PromptForSelection("Select an AWS SSO Profile", []string{"profile1", "profile2"}).
		Return("profile1", nil)

	profile, err := mockAWSSelectionClient.SelectProfileFromConfig(cfg)

	assert.NoError(t, err)
	assert.NotNil(t, profile)
	assert.Equal(t, profile.ProfileName, "profile1")
}

func TestSelectAccountFromProfile(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPrompter := mock_sso.NewMockPrompter(ctrl)
	mockAWSSelectionClient := &RealAWSSelectionClient{
		Prompter: mockPrompter,
	}

	profile := &models.SSOProfile{
		ProfileName: "profile1",
		Accounts: []models.SSOAccount{
			{AccountName: "account1"},
			{AccountName: "account2"},
		},
	}

	mockPrompter.EXPECT().PromptForSelection("Select AWS Account", []string{"account1", "account2"}).
		Return("account1", nil)

	account, err := mockAWSSelectionClient.SelectAccountFromProfile(profile)

	assert.NoError(t, err)
	assert.NotNil(t, account)
	assert.Equal(t, account.AccountName, "account1")
}

func TestSelectRoleFromAccount(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPrompter := mock_sso.NewMockPrompter(ctrl)
	mockAWSSelectionClient := &RealAWSSelectionClient{
		Prompter: mockPrompter,
	}

	account := &models.SSOAccount{
		AccountName: "account1",
		Roles:       []string{"role1", "role2"},
	}

	mockPrompter.EXPECT().PromptForSelection("Select an AWS Role", []string{"role1", "role2"}).
		Return("role1", nil)

	role, err := mockAWSSelectionClient.SelectRoleFromAccount(account)

	assert.NoError(t, err)
	assert.Equal(t, role, "role1")
}

func TestFindAccount_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAWSSelectionClient := mock_sso.NewMockAWSSelectionClient(ctrl)

	profile := &models.SSOProfile{
		ProfileName: "profile1",
		Accounts: []models.SSOAccount{
			{AccountName: "account1"},
		},
	}

	accountName := "account2"

	mockAWSSelectionClient.EXPECT().FindAccount(profile, accountName).
		Return(nil, fmt.Errorf("account %s not found in profile %s", accountName, profile.ProfileName))

	account, err := mockAWSSelectionClient.FindAccount(profile, accountName)

	assert.Error(t, err)
	assert.Equal(t, err.Error(), "account account2 not found in profile profile1")
	assert.Nil(t, account)
}

func TestSelectProfile_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPrompter := mock_sso.NewMockPrompter(ctrl)
	mockAWSSelectionClient := &RealAWSSelectionClient{
		Prompter: mockPrompter,
	}

	profiles := []string{"profile1", "profile2"}

	mockPrompter.EXPECT().PromptForSelection("Select an AWS SSO Profile", profiles).
		Return("", promptUtils.ErrInterrupted)

	_, err := mockAWSSelectionClient.SelectProfile(profiles)

	assert.Error(t, err)
	assert.True(t, errors.Is(err, promptUtils.ErrInterrupted))
}
