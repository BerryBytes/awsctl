package rds_test

import (
	"errors"
	"os"
	"testing"

	"github.com/BerryBytes/awsctl/internal/rds"
	"github.com/BerryBytes/awsctl/models"
	mock_awsctl "github.com/BerryBytes/awsctl/tests/mock"
	promptUtils "github.com/BerryBytes/awsctl/utils/prompt"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func TestNewRPrompter(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPrompter := mock_awsctl.NewMockPrompter(ctrl)
	mockConfigClient := mock_awsctl.NewMockAWSConfigClient(ctrl)

	prompter := rds.NewRPrompter(mockPrompter, mockConfigClient)

	assert.NotNil(t, prompter)
	assert.Equal(t, mockPrompter, prompter.Prompt)
	assert.Equal(t, mockConfigClient, prompter.AWSConfigClient)
}

func TestPromptForRDSInstance_WithInstances(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPrompter := mock_awsctl.NewMockPrompter(ctrl)
	mockConfigClient := mock_awsctl.NewMockAWSConfigClient(ctrl)

	instances := []models.RDSInstance{
		{
			DBInstanceIdentifier: "db-1",
			Engine:               "postgres",
			Endpoint:             "db-1.abc123.us-east-1.rds.amazonaws.com:5432",
		},
		{
			DBInstanceIdentifier: "db-2",
			Engine:               "mysql",
			Endpoint:             "db-2.abc123.us-east-1.rds.amazonaws.com:3306",
		},
	}

	expectedItems := []string{
		"db-1 (postgres) - db-1.abc123.us-east-1.rds.amazonaws.com:5432",
		"db-2 (mysql) - db-2.abc123.us-east-1.rds.amazonaws.com:3306",
	}

	mockPrompter.EXPECT().PromptForSelection(
		"Select an RDS instance:",
		expectedItems,
	).Return(expectedItems[0], nil)

	prompter := rds.NewRPrompter(mockPrompter, mockConfigClient)
	selected, err := prompter.PromptForRDSInstance(instances)

	assert.NoError(t, err)
	assert.Equal(t, "db-1", selected)
}

func TestPromptForProfile_FromEnv(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPrompter := mock_awsctl.NewMockPrompter(ctrl)
	mockConfigClient := mock_awsctl.NewMockAWSConfigClient(ctrl)

	err := os.Setenv("AWS_PROFILE", "test-profile")
	if err != nil {
		t.Fatalf("Failed to set AWS_PROFILE: %v", err)
	}

	defer func() {
		if err := os.Unsetenv("AWS_PROFILE"); err != nil {
			t.Logf("Warning: failed to unset AWS_PROFILE: %v", err)
		}
	}()

	prompter := rds.NewRPrompter(mockPrompter, mockConfigClient)
	profile, err := prompter.PromptForProfile()

	assert.NoError(t, err)
	assert.Equal(t, "test-profile", profile)
}

func TestPromptForProfile_FromConfig(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPrompter := mock_awsctl.NewMockPrompter(ctrl)
	mockConfigClient := mock_awsctl.NewMockAWSConfigClient(ctrl)

	if err := os.Unsetenv("AWS_PROFILE"); err != nil {
		t.Logf("Warning: failed to unset AWS_PROFILE: %v", err)
	}

	expectedProfiles := []string{"profile1", "profile2"}
	mockConfigClient.EXPECT().ValidProfiles().Return(expectedProfiles, nil)
	mockPrompter.EXPECT().PromptForSelection(
		"Select an AWS profile:",
		expectedProfiles,
	).Return("profile2", nil)

	prompter := rds.NewRPrompter(mockPrompter, mockConfigClient)
	profile, err := prompter.PromptForProfile()

	assert.NoError(t, err)
	assert.Equal(t, "profile2", profile)
}

func TestPromptForProfile_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPrompter := mock_awsctl.NewMockPrompter(ctrl)
	mockConfigClient := mock_awsctl.NewMockAWSConfigClient(ctrl)

	if err := os.Unsetenv("AWS_PROFILE"); err != nil {
		t.Logf("Warning: failed to unset AWS_PROFILE: %v", err)
	}

	mockConfigClient.EXPECT().ValidProfiles().Return(nil, errors.New("config error"))

	prompter := rds.NewRPrompter(mockPrompter, mockConfigClient)
	profile, err := prompter.PromptForProfile()

	assert.Error(t, err)
	assert.Equal(t, "", profile)
	assert.Contains(t, err.Error(), "failed to list valid profiles")
}

func TestSelectRDSAction(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPrompter := mock_awsctl.NewMockPrompter(ctrl)
	mockConfigClient := mock_awsctl.NewMockAWSConfigClient(ctrl)

	testCases := []struct {
		name           string
		selectedAction string
		expectedAction rds.RDSAction
		expectError    bool
	}{
		{
			name:           "Connect Direct",
			selectedAction: "Connect Direct (Just show RDS endpoint)",
			expectedAction: rds.ConnectDirect,
			expectError:    false,
		},
		{
			name:           "Connect Via Tunnel",
			selectedAction: "Connect Via Tunnel (SSH port forwarding)",
			expectedAction: rds.ConnectViaTunnel,
			expectError:    false,
		},

		{
			name:           "Exit",
			selectedAction: "Exit",
			expectedAction: rds.ExitRDS,
			expectError:    false,
		},
		{
			name:           "Invalid Action",
			selectedAction: "Invalid Action",
			expectedAction: rds.ExitRDS,
			expectError:    true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actions := []string{
				"Connect Direct (Just show RDS endpoint)",
				"Connect Via Tunnel (SSH port forwarding)",
				"Exit",
			}

			mockPrompter.EXPECT().PromptForSelection(
				"Select an RDS action:",
				actions,
			).Return(tc.selectedAction, nil)

			prompter := rds.NewRPrompter(mockPrompter, mockConfigClient)
			action, err := prompter.SelectRDSAction()

			if tc.expectError {
				assert.Error(t, err)
				assert.Equal(t, rds.ExitRDS, action)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedAction, action)
			}
		})
	}
}
func TestPromptForManualEndpoint_Invalid(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPrompter := mock_awsctl.NewMockPrompter(ctrl)
	mockConfigClient := mock_awsctl.NewMockAWSConfigClient(ctrl)

	mockPrompter.EXPECT().PromptForInput(
		"Enter RDS endpoint (hostname:port):",
		"",
	).Return("invalid-endpoint", nil)

	prompter := rds.NewRPrompter(mockPrompter, mockConfigClient)
	_, _, _, err := prompter.PromptForManualEndpoint()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid endpoint format")
}

func TestGetAWSConfig_FromEnv(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPrompter := mock_awsctl.NewMockPrompter(ctrl)
	mockConfigClient := mock_awsctl.NewMockAWSConfigClient(ctrl)

	if err := os.Setenv("AWS_PROFILE", "test-profile"); err != nil {
		t.Fatalf("failed to set AWS_PROFILE: %v", err)
	}
	if err := os.Setenv("AWS_REGION", "us-east-1"); err != nil {
		t.Fatalf("failed to set AWS_REGION: %v", err)
	}

	defer func() {
		if err := os.Unsetenv("AWS_PROFILE"); err != nil {
			t.Logf("Warning: failed to unset AWS_PROFILE: %v", err)
		}
		if err := os.Unsetenv("AWS_REGION"); err != nil {
			t.Logf("Warning: failed to unset AWS_REGION: %v", err)
		}
	}()

	prompter := rds.NewRPrompter(mockPrompter, mockConfigClient)
	profile, region, err := prompter.GetAWSConfig()

	assert.NoError(t, err)
	assert.Equal(t, "test-profile", profile)
	assert.Equal(t, "us-east-1", region)
}

func TestPromptForDBUser(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPrompter := mock_awsctl.NewMockPrompter(ctrl)
	mockConfigClient := mock_awsctl.NewMockAWSConfigClient(ctrl)

	mockPrompter.EXPECT().PromptForInput(
		"Enter database username:",
		"",
	).Return("admin", nil)

	prompter := rds.NewRPrompter(mockPrompter, mockConfigClient)
	user, err := prompter.PromptForDBUser()

	assert.NoError(t, err)
	assert.Equal(t, "admin", user)
}

func TestPromptForRegion_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPrompter := mock_awsctl.NewMockPrompter(ctrl)
	mockConfigClient := mock_awsctl.NewMockAWSConfigClient(ctrl)

	mockPrompter.EXPECT().PromptForInput(
		"Enter AWS region (e.g. us-east-1):",
		"",
	).Return("us-east-1", nil)

	prompter := rds.NewRPrompter(mockPrompter, mockConfigClient)
	region, err := prompter.PromptForRegion()

	assert.NoError(t, err)
	assert.Equal(t, "us-east-1", region)
}

func TestPromptForRegion_EmptyInput(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPrompter := mock_awsctl.NewMockPrompter(ctrl)
	mockConfigClient := mock_awsctl.NewMockAWSConfigClient(ctrl)

	mockPrompter.EXPECT().PromptForInput(
		"Enter AWS region (e.g. us-east-1):",
		"",
	).Return("", nil)

	prompter := rds.NewRPrompter(mockPrompter, mockConfigClient)
	region, err := prompter.PromptForRegion()

	assert.Error(t, err)
	assert.Equal(t, "", region)
	assert.Equal(t, "AWS region cannot be empty", err.Error())
}

func TestPromptForRegion_Interrupted(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPrompter := mock_awsctl.NewMockPrompter(ctrl)
	mockConfigClient := mock_awsctl.NewMockAWSConfigClient(ctrl)

	mockPrompter.EXPECT().PromptForInput(
		"Enter AWS region (e.g. us-east-1):",
		"",
	).Return("", promptUtils.ErrInterrupted)

	prompter := rds.NewRPrompter(mockPrompter, mockConfigClient)
	region, err := prompter.PromptForRegion()

	assert.Error(t, err)
	assert.Equal(t, "", region)
	assert.True(t, errors.Is(err, promptUtils.ErrInterrupted))
}

func TestPromptForRegion_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPrompter := mock_awsctl.NewMockPrompter(ctrl)
	mockConfigClient := mock_awsctl.NewMockAWSConfigClient(ctrl)

	testErr := errors.New("input error")
	mockPrompter.EXPECT().PromptForInput(
		"Enter AWS region (e.g. us-east-1):",
		"",
	).Return("", testErr)

	prompter := rds.NewRPrompter(mockPrompter, mockConfigClient)
	region, err := prompter.PromptForRegion()

	assert.Error(t, err)
	assert.Equal(t, "", region)
	assert.Contains(t, err.Error(), "failed to get AWS region")
	assert.ErrorContains(t, err, testErr.Error())
}

func TestPromptForRDSInstance_Interrupted(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPrompter := mock_awsctl.NewMockPrompter(ctrl)
	mockConfigClient := mock_awsctl.NewMockAWSConfigClient(ctrl)

	instances := []models.RDSInstance{
		{
			DBInstanceIdentifier: "db-1",
			Engine:               "postgres",
			Endpoint:             "db-1.abc123.us-east-1.rds.amazonaws.com:5432",
		},
	}

	mockPrompter.EXPECT().PromptForSelection(
		"Select an RDS instance:",
		gomock.Any(),
	).Return("", promptUtils.ErrInterrupted)

	prompter := rds.NewRPrompter(mockPrompter, mockConfigClient)
	_, err := prompter.PromptForRDSInstance(instances)

	assert.Error(t, err)
	assert.True(t, errors.Is(err, promptUtils.ErrInterrupted))
}

func TestPromptForRDSInstance_InvalidSelection(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPrompter := mock_awsctl.NewMockPrompter(ctrl)
	mockConfigClient := mock_awsctl.NewMockAWSConfigClient(ctrl)

	instances := []models.RDSInstance{
		{
			DBInstanceIdentifier: "db-1",
			Engine:               "postgres",
			Endpoint:             "db-1.abc123.us-east-1.rds.amazonaws.com:5432",
		},
	}

	mockPrompter.EXPECT().PromptForSelection(
		"Select an RDS instance:",
		gomock.Any(),
	).Return("invalid-selection", nil)

	prompter := rds.NewRPrompter(mockPrompter, mockConfigClient)
	_, err := prompter.PromptForRDSInstance(instances)

	assert.Error(t, err)
	assert.Equal(t, "invalid selection", err.Error())
}

func TestPromptForManualEndpoint_ErrorInUsername(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPrompter := mock_awsctl.NewMockPrompter(ctrl)
	mockConfigClient := mock_awsctl.NewMockAWSConfigClient(ctrl)

	mockPrompter.EXPECT().PromptForInput(
		"Enter RDS endpoint (hostname:port):",
		"",
	).Return("valid.endpoint:5432", nil)

	mockPrompter.EXPECT().PromptForInput(
		"Enter database username:",
		"",
	).Return("", errors.New("username error"))

	prompter := rds.NewRPrompter(mockPrompter, mockConfigClient)
	_, _, _, err := prompter.PromptForManualEndpoint()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to input username")
}

func TestPromptForManualEndpoint_ErrorInRegion(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPrompter := mock_awsctl.NewMockPrompter(ctrl)
	mockConfigClient := mock_awsctl.NewMockAWSConfigClient(ctrl)

	mockPrompter.EXPECT().PromptForInput(
		"Enter RDS endpoint (hostname:port):",
		"",
	).Return("valid.endpoint:5432", nil)

	mockPrompter.EXPECT().PromptForInput(
		"Enter database username:",
		"",
	).Return("testuser", nil)

	mockPrompter.EXPECT().PromptForInput(
		"Enter AWS region (e.g. us-east-1):",
		"",
	).Return("", errors.New("region error"))

	prompter := rds.NewRPrompter(mockPrompter, mockConfigClient)
	_, _, _, err := prompter.PromptForManualEndpoint()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to input region")
}

func TestGetAWSConfig_FromPrompts(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPrompter := mock_awsctl.NewMockPrompter(ctrl)
	mockConfigClient := mock_awsctl.NewMockAWSConfigClient(ctrl)

	if err := os.Unsetenv("AWS_PROFILE"); err != nil {
		t.Logf("Warning: failed to unset AWS_PROFILE: %v", err)
	}
	if err := os.Unsetenv("AWS_REGION"); err != nil {
		t.Logf("Warning: failed to unset AWS_REGION: %v", err)
	}

	profiles := []string{"profile1", "profile2"}
	mockConfigClient.EXPECT().ValidProfiles().Return(profiles, nil)
	mockPrompter.EXPECT().PromptForSelection(
		"Select AWS profile:",
		profiles,
	).Return("profile2", nil)

	mockPrompter.EXPECT().PromptForInput(
		"Enter AWS region (e.g. us-east-1):",
		"",
	).Return("us-west-2", nil)

	prompter := rds.NewRPrompter(mockPrompter, mockConfigClient)
	profile, region, err := prompter.GetAWSConfig()

	assert.NoError(t, err)
	assert.Equal(t, "profile2", profile)
	assert.Equal(t, "us-west-2", region)
}

func TestGetAWSConfig_NoProfiles(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPrompter := mock_awsctl.NewMockPrompter(ctrl)
	mockConfigClient := mock_awsctl.NewMockAWSConfigClient(ctrl)

	if err := os.Unsetenv("AWS_PROFILE"); err != nil {
		t.Logf("Warning: failed to unset AWS_PROFILE: %v", err)
	}
	if err := os.Unsetenv("AWS_REGION"); err != nil {
		t.Logf("Warning: failed to unset AWS_REGION: %v", err)
	}

	mockConfigClient.EXPECT().ValidProfiles().Return([]string{}, nil)

	prompter := rds.NewRPrompter(mockPrompter, mockConfigClient)
	_, _, err := prompter.GetAWSConfig()

	assert.Error(t, err)
	assert.Equal(t, "no AWS profiles found", err.Error())
}

func TestGetAWSConfig_ProfileSelectionError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPrompter := mock_awsctl.NewMockPrompter(ctrl)
	mockConfigClient := mock_awsctl.NewMockAWSConfigClient(ctrl)

	if err := os.Unsetenv("AWS_PROFILE"); err != nil {
		t.Logf("Warning: failed to unset AWS_PROFILE: %v", err)
	}
	if err := os.Unsetenv("AWS_REGION"); err != nil {
		t.Logf("Warning: failed to unset AWS_REGION: %v", err)
	}

	profiles := []string{"profile1", "profile2"}
	mockConfigClient.EXPECT().ValidProfiles().Return(profiles, nil)
	mockPrompter.EXPECT().PromptForSelection(
		"Select AWS profile:",
		profiles,
	).Return("", errors.New("selection error"))

	prompter := rds.NewRPrompter(mockPrompter, mockConfigClient)
	_, _, err := prompter.GetAWSConfig()

	assert.Error(t, err)
	assert.Equal(t, "selection error", err.Error())
}

func TestPromptForRDSInstance_NoInstances(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPrompter := mock_awsctl.NewMockPrompter(ctrl)
	mockConfigClient := mock_awsctl.NewMockAWSConfigClient(ctrl)

	validEndpoint := "valid.hostname:1234"
	mockPrompter.EXPECT().PromptForInput(
		"Enter RDS endpoint (hostname:port):",
		"",
	).Return(validEndpoint, nil)

	mockPrompter.EXPECT().PromptForInput(
		"Enter database username:",
		"",
	).Return("manual-user", nil)

	mockPrompter.EXPECT().PromptForInput(
		"Enter AWS region (e.g. us-east-1):",
		"",
	).Return("us-west-2", nil)

	prompter := rds.NewRPrompter(mockPrompter, mockConfigClient)
	endpoint, err := prompter.PromptForRDSInstance([]models.RDSInstance{})

	assert.NoError(t, err)
	assert.Equal(t, validEndpoint, endpoint)
}

func TestPromptForRDSInstance_SelectionError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPrompter := mock_awsctl.NewMockPrompter(ctrl)
	mockConfigClient := mock_awsctl.NewMockAWSConfigClient(ctrl)

	instances := []models.RDSInstance{
		{
			DBInstanceIdentifier: "db-1",
			Engine:               "postgres",
			Endpoint:             "db-1.abc123.us-east-1.rds.amazonaws.com:5432",
		},
	}

	testErr := errors.New("selection error")
	mockPrompter.EXPECT().PromptForSelection(
		"Select an RDS instance:",
		gomock.Any(),
	).Return("", testErr)

	prompter := rds.NewRPrompter(mockPrompter, mockConfigClient)
	_, err := prompter.PromptForRDSInstance(instances)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to select RDS instance")
	assert.ErrorContains(t, err, testErr.Error())
}

func TestPromptForProfile_SingleProfile(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPrompter := mock_awsctl.NewMockPrompter(ctrl)
	mockConfigClient := mock_awsctl.NewMockAWSConfigClient(ctrl)

	if err := os.Unsetenv("AWS_PROFILE"); err != nil {
		t.Logf("Warning: failed to unset AWS_PROFILE: %v", err)
	}

	singleProfile := []string{"profile1"}
	mockConfigClient.EXPECT().ValidProfiles().Return(singleProfile, nil)

	prompter := rds.NewRPrompter(mockPrompter, mockConfigClient)
	profile, err := prompter.PromptForProfile()

	assert.NoError(t, err)
	assert.Equal(t, "profile1", profile)
}

func TestPromptForProfile_NoValidProfiles(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPrompter := mock_awsctl.NewMockPrompter(ctrl)
	mockConfigClient := mock_awsctl.NewMockAWSConfigClient(ctrl)

	if err := os.Unsetenv("AWS_PROFILE"); err != nil {
		t.Logf("Warning: failed to unset AWS_PROFILE: %v", err)
	}

	mockConfigClient.EXPECT().ValidProfiles().Return([]string{}, nil)

	prompter := rds.NewRPrompter(mockPrompter, mockConfigClient)
	_, err := prompter.PromptForProfile()

	assert.Error(t, err)
	assert.Equal(t, "no valid AWS profiles found", err.Error())
}

func TestSelectRDSAction_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPrompter := mock_awsctl.NewMockPrompter(ctrl)
	mockConfigClient := mock_awsctl.NewMockAWSConfigClient(ctrl)

	testErr := errors.New("selection error")
	mockPrompter.EXPECT().PromptForSelection(
		"Select an RDS action:",
		gomock.Any(),
	).Return("", testErr)

	prompter := rds.NewRPrompter(mockPrompter, mockConfigClient)
	_, err := prompter.SelectRDSAction()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to select RDS action")
	assert.ErrorContains(t, err, testErr.Error())
}

func TestPromptForManualEndpoint_EndpointError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPrompter := mock_awsctl.NewMockPrompter(ctrl)
	mockConfigClient := mock_awsctl.NewMockAWSConfigClient(ctrl)

	testErr := errors.New("endpoint error")
	mockPrompter.EXPECT().PromptForInput(
		"Enter RDS endpoint (hostname:port):",
		"",
	).Return("", testErr)

	prompter := rds.NewRPrompter(mockPrompter, mockConfigClient)
	_, _, _, err := prompter.PromptForManualEndpoint()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to input endpoint")
	assert.ErrorContains(t, err, testErr.Error())
}

func TestPromptForAuthMethod(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPrompter := mock_awsctl.NewMockPrompter(ctrl)
	mockConfigClient := mock_awsctl.NewMockAWSConfigClient(ctrl)

	testCases := []struct {
		name           string
		message        string
		options        []string
		mockSelection  string
		mockError      error
		expectedMethod string
		expectedError  string
		isInterrupted  bool
	}{
		{
			name:           "Successful selection",
			message:        "Select authentication method for RDS:",
			options:        []string{"Token", "Native password"},
			mockSelection:  "Token",
			mockError:      nil,
			expectedMethod: "Token",
			expectedError:  "",
		},
		{
			name:           "Interrupted prompt",
			message:        "Select authentication method for RDS:",
			options:        []string{"Token", "Native password"},
			mockSelection:  "",
			mockError:      promptUtils.ErrInterrupted,
			expectedMethod: "",
			isInterrupted:  true,
		},
		{
			name:           "Selection error",
			message:        "Select authentication method for RDS:",
			options:        []string{"Token", "Native password"},
			mockSelection:  "",
			mockError:      errors.New("selection error"),
			expectedMethod: "",
			expectedError:  "failed to select authentication method: selection error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockPrompter.EXPECT().PromptForSelection(
				tc.message,
				tc.options,
			).Return(tc.mockSelection, tc.mockError)

			prompter := rds.NewRPrompter(mockPrompter, mockConfigClient)
			method, err := prompter.PromptForAuthMethod(tc.message, tc.options)

			if tc.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedError)
			} else if tc.isInterrupted {
				assert.Error(t, err)
				assert.True(t, errors.Is(err, promptUtils.ErrInterrupted))
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tc.expectedMethod, method)
		})
	}
}
