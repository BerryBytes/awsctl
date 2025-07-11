package sso

import (
	"errors"
	"testing"

	"github.com/BerryBytes/awsctl/internal/sso"
	mock_sso "github.com/BerryBytes/awsctl/tests/mock/sso"
	promptUtils "github.com/BerryBytes/awsctl/utils/prompt"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func TestSetupCmd(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSSOClient := mock_sso.NewMockSSOClient(ctrl)

	tests := []struct {
		name           string
		args           []string
		mockSetup      func()
		expectedError  string
		expectedOutput string
	}{
		{
			name: "successful setup with no flags",
			args: []string{},
			mockSetup: func() {
				mockSSOClient.EXPECT().SetupSSO(sso.SSOFlagOptions{}).Return(nil)
			},
		},
		{
			name: "successful setup with all flags",
			args: []string{"--name=test-session", "--start-url=https://test.awsapps.com/start", "--region=us-east-1"},
			mockSetup: func() {
				mockSSOClient.EXPECT().SetupSSO(sso.SSOFlagOptions{
					Name:     "test-session",
					StartURL: "https://test.awsapps.com/start",
					Region:   "us-east-1",
				}).Return(nil)
			},
		},
		{
			name: "error during setup",
			args: []string{},
			mockSetup: func() {
				mockSSOClient.EXPECT().SetupSSO(sso.SSOFlagOptions{}).Return(errors.New("setup error"))
			},
			expectedError: "SSO initialization failed: setup error",
		},
		{
			name: "interrupted by user",
			args: []string{},
			mockSetup: func() {
				mockSSOClient.EXPECT().SetupSSO(sso.SSOFlagOptions{}).Return(promptUtils.ErrInterrupted)
			},
		},
		{
			name: "invalid start URL format",
			args: []string{"--start-url=invalid-url"},
			mockSetup: func() {

			},
			expectedError: "invalid start URL: must begin with https://",
		},
		{
			name: "invalid region",
			args: []string{"--region=invalid-region"},
			mockSetup: func() {

			},
			expectedError: "invalid AWS region: invalid-region",
		},
		{
			name: "invalid session name",
			args: []string{"--name=invalid-name-"},
			mockSetup: func() {

			},
			expectedError: "invalid session name: must only contain letters, numbers, dashes, or underscores, and cannot start or end with a dash/underscore",
		},
		{
			name: "partial flags - only name provided",
			args: []string{"--name=valid-name"},
			mockSetup: func() {
				mockSSOClient.EXPECT().SetupSSO(sso.SSOFlagOptions{
					Name: "valid-name",
				}).Return(nil)
			},
		},
		{
			name: "partial flags - only region provided",
			args: []string{"--region=us-west-2"},
			mockSetup: func() {
				mockSSOClient.EXPECT().SetupSSO(sso.SSOFlagOptions{
					Region: "us-west-2",
				}).Return(nil)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			cmd := SetupCmd(mockSSOClient)
			cmd.SetArgs(tt.args)

			err := cmd.Execute()

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSetupCmd_FlagValidation(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSSOClient := mock_sso.NewMockSSOClient(ctrl)

	tests := []struct {
		name          string
		args          []string
		expectCall    bool
		expectedError string
	}{
		{
			name:       "valid start URL",
			args:       []string{"--start-url=https://valid.awsapps.com/start"},
			expectCall: true,
		},
		{
			name:          "invalid start URL missing https",
			args:          []string{"--start-url=http://invalid.awsapps.com/start"},
			expectCall:    false,
			expectedError: "invalid start URL: must begin with https://",
		},
		{
			name:          "invalid start URL format",
			args:          []string{"--start-url=invalid-format"},
			expectCall:    false,
			expectedError: "invalid start URL: must begin with https://",
		},
		{
			name:       "valid region",
			args:       []string{"--region=eu-west-1"},
			expectCall: true,
		},
		{
			name:          "invalid region",
			args:          []string{"--region=invalid-region"},
			expectCall:    false,
			expectedError: "invalid AWS region: invalid-region",
		},
		{
			name:       "valid session name",
			args:       []string{"--name=valid_name-123"},
			expectCall: true,
		},
		{
			name:          "invalid session name starts with dash",
			args:          []string{"--name=-invalid"},
			expectCall:    false,
			expectedError: "invalid session name: must only contain letters, numbers, dashes, or underscores, and cannot start or end with a dash/underscore",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.expectCall {
				mockSSOClient.EXPECT().SetupSSO(gomock.Any()).Return(nil)
			}

			cmd := SetupCmd(mockSSOClient)
			cmd.SetArgs(tt.args)
			cmd.SilenceErrors = true
			cmd.SilenceUsage = true

			err := cmd.Execute()

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
