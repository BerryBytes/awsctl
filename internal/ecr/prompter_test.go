package ecr_test

import (
	"errors"
	"os"
	"testing"

	"github.com/BerryBytes/awsctl/internal/ecr"
	mock_awsctl "github.com/BerryBytes/awsctl/tests/mock"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func TestNewEPrompter(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPrompt := mock_awsctl.NewMockPrompter(ctrl)
	mockConfigClient := mock_awsctl.NewMockAWSConfigClient(ctrl)

	prompter := ecr.NewEPrompter(mockPrompt, mockConfigClient)

	assert.NotNil(t, prompter)
	assert.Equal(t, mockPrompt, prompter.Prompt)
	assert.Equal(t, mockConfigClient, prompter.AWSConfigClient)
}

func TestSelectECRAction_Success(t *testing.T) {
	tests := []struct {
		name          string
		selected      string
		expected      ecr.ECRAction
		expectedError error
	}{
		{
			name:     "LoginECR selected",
			selected: "Login to ECR",
			expected: ecr.LoginECR,
		},
		{
			name:     "ExitECR selected",
			selected: "Exit",
			expected: ecr.ExitECR,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockPrompt := mock_awsctl.NewMockPrompter(ctrl)
			mockConfigClient := mock_awsctl.NewMockAWSConfigClient(ctrl)

			prompter := ecr.NewEPrompter(mockPrompt, mockConfigClient)

			actions := []string{"Login to ECR", "Exit"}
			mockPrompt.EXPECT().
				PromptForSelection("Select an ECR action:", actions).
				Return(tt.selected, nil)

			action, err := prompter.SelectECRAction()

			assert.Equal(t, tt.expected, action)
			assert.NoError(t, err)
		})
	}
}

func TestSelectECRAction_ErrorCases(t *testing.T) {
	tests := []struct {
		name          string
		promptError   error
		expectedError string
	}{
		{
			name:          "Prompt error",
			promptError:   errors.New("prompt error"),
			expectedError: "failed to select ECR action: prompt error",
		},
		{
			name:          "Invalid selection",
			promptError:   nil,
			expectedError: "invalid action selected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockPrompt := mock_awsctl.NewMockPrompter(ctrl)
			mockConfigClient := mock_awsctl.NewMockAWSConfigClient(ctrl)

			prompter := ecr.NewEPrompter(mockPrompt, mockConfigClient)

			actions := []string{"Login to ECR", "Exit"}
			var selected string
			if tt.promptError != nil {
				selected = "Login to ECR"
			} else {
				selected = "Invalid option"
			}
			mockPrompt.EXPECT().
				PromptForSelection("Select an ECR action:", actions).
				Return(selected, tt.promptError)

			action, err := prompter.SelectECRAction()

			assert.Equal(t, ecr.ExitECR, action)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedError)
		})
	}
}

func TestPromptForProfile_FromEnv(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPrompt := mock_awsctl.NewMockPrompter(ctrl)
	mockConfigClient := mock_awsctl.NewMockAWSConfigClient(ctrl)

	prompter := ecr.NewEPrompter(mockPrompt, mockConfigClient)

	expectedProfile := "test-profile"
	t.Setenv("AWS_PROFILE", expectedProfile)

	profile, err := prompter.PromptForProfile()

	assert.Equal(t, expectedProfile, profile)
	assert.NoError(t, err)
}

func TestPromptForProfile_FromConfig_SingleProfile(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPrompt := mock_awsctl.NewMockPrompter(ctrl)
	mockConfigClient := mock_awsctl.NewMockAWSConfigClient(ctrl)

	prompter := ecr.NewEPrompter(mockPrompt, mockConfigClient)

	os.Unsetenv("AWS_PROFILE")

	expectedProfiles := []string{"default"}
	mockConfigClient.EXPECT().
		ValidProfiles().
		Return(expectedProfiles, nil)

	profile, err := prompter.PromptForProfile()

	assert.Equal(t, "default", profile)
	assert.NoError(t, err)
}

func TestPromptForProfile_FromConfig_MultipleProfiles(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPrompt := mock_awsctl.NewMockPrompter(ctrl)
	mockConfigClient := mock_awsctl.NewMockAWSConfigClient(ctrl)

	prompter := ecr.NewEPrompter(mockPrompt, mockConfigClient)

	os.Unsetenv("AWS_PROFILE")

	expectedProfiles := []string{"profile1", "profile2", "profile3"}
	selectedProfile := "profile2"

	mockConfigClient.EXPECT().
		ValidProfiles().
		Return(expectedProfiles, nil)

	mockPrompt.EXPECT().
		PromptForSelection("Select an AWS profile:", expectedProfiles).
		Return(selectedProfile, nil)

	profile, err := prompter.PromptForProfile()

	assert.Equal(t, selectedProfile, profile)
	assert.NoError(t, err)
}

func TestPromptForProfile_ErrorCases(t *testing.T) {
	tests := []struct {
		name          string
		validProfiles []string
		configError   error
		promptError   error
		expectedError string
	}{
		{
			name:          "Config client error",
			validProfiles: nil,
			configError:   errors.New("config error"),
			expectedError: "failed to list valid profiles: config error",
		},
		{
			name:          "No profiles found",
			validProfiles: []string{},
			expectedError: "no valid AWS profiles found",
		},
		{
			name:          "Prompt selection error",
			validProfiles: []string{"p1", "p2"},
			promptError:   errors.New("prompt error"),
			expectedError: "prompt error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockPrompt := mock_awsctl.NewMockPrompter(ctrl)
			mockConfigClient := mock_awsctl.NewMockAWSConfigClient(ctrl)

			prompter := ecr.NewEPrompter(mockPrompt, mockConfigClient)

			os.Unsetenv("AWS_PROFILE")

			mockConfigClient.EXPECT().
				ValidProfiles().
				Return(tt.validProfiles, tt.configError)

			if len(tt.validProfiles) > 1 && tt.configError == nil {
				mockPrompt.EXPECT().
					PromptForSelection("Select an AWS profile:", tt.validProfiles).
					Return("", tt.promptError)
			}

			profile, err := prompter.PromptForProfile()

			assert.Empty(t, profile)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedError)
		})
	}
}
