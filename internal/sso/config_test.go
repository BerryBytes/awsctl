package sso_test

import (
	"fmt"
	"testing"

	"github.com/BerryBytes/awsctl/internal/sso"
	mock_awsctl "github.com/BerryBytes/awsctl/tests/mock"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func TestRealCommandExecutor_RunCommand(t *testing.T) {
	executor := &sso.RealCommandExecutor{}

	output, err := executor.RunCommand("echo", "hello")
	assert.NoError(t, err)
	assert.Equal(t, "hello\n", string(output))

	_, err = executor.RunCommand("nonexistent-command")
	assert.Error(t, err)
}

func TestConfigureGet_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockExecutor := mock_awsctl.NewMockCommandExecutor(ctrl)
	client := &sso.RealAWSConfigClient{Executor: mockExecutor}

	expectedErr := fmt.Errorf("command failed")
	mockExecutor.EXPECT().
		RunCommand("aws", "configure", "get", "region", "--profile", "test-profile").
		Return([]byte{}, expectedErr).
		Times(1)

	result, err := client.ConfigureGet("region", "test-profile")

	assert.Error(t, err)
	assert.Equal(t, "", result)
	assert.Contains(t, err.Error(), "failed to get region")
	assert.ErrorIs(t, err, expectedErr)
}
func TestConfigureSet(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockExecutor := mock_awsctl.NewMockCommandExecutor(ctrl)

	mockExecutor.EXPECT().RunCommand("aws", "configure", "set", "region", "us-west-2", "--profile", "test-profile").Return([]byte{}, nil).Times(1)

	client := &sso.RealAWSConfigClient{Executor: mockExecutor}

	err := client.ConfigureSet("region", "us-west-2", "test-profile")
	assert.NoError(t, err)
}

func TestConfigureGet(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockExecutor := mock_awsctl.NewMockCommandExecutor(ctrl)

	mockExecutor.EXPECT().RunCommand("aws", "configure", "get", "region", "--profile", "test-profile").Return([]byte("us-west-2\n"), nil).Times(1)

	client := &sso.RealAWSConfigClient{Executor: mockExecutor}

	region, err := client.ConfigureGet("region", "test-profile")
	assert.NoError(t, err)
	assert.Equal(t, "us-west-2", region)
}

func TestValidProfiles(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockExecutor := mock_awsctl.NewMockCommandExecutor(ctrl)

	mockExecutor.EXPECT().RunCommand("aws", "configure", "list-profiles").Return([]byte("test-profile\nanother-profile\n"), nil).Times(1)

	client := &sso.RealAWSConfigClient{Executor: mockExecutor}

	profiles, err := client.ValidProfiles()
	assert.NoError(t, err)
	assert.Equal(t, []string{"test-profile", "another-profile"}, profiles)
}

func TestConfigureDefaultProfile(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockExecutor := mock_awsctl.NewMockCommandExecutor(ctrl)

	mockExecutor.EXPECT().RunCommand("aws", "configure", "set", "region", "us-west-2", "--profile", "default").Return([]byte{}, nil).Times(1)
	mockExecutor.EXPECT().RunCommand("aws", "configure", "set", "output", "json", "--profile", "default").Return([]byte{}, nil).Times(1)

	client := &sso.RealAWSConfigClient{Executor: mockExecutor}

	err := client.ConfigureDefaultProfile("us-west-2", "json")
	assert.NoError(t, err)
}

func TestConfigureSSOProfile(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockExecutor := mock_awsctl.NewMockCommandExecutor(ctrl)

	mockExecutor.EXPECT().RunCommand("aws", "configure", "set", "sso_region", "us-west-2", "--profile", "test-profile").Return([]byte{}, nil).Times(1)
	mockExecutor.EXPECT().RunCommand("aws", "configure", "set", "sso_account_id", "123456789012", "--profile", "test-profile").Return([]byte{}, nil).Times(1)
	mockExecutor.EXPECT().RunCommand("aws", "configure", "set", "sso_start_url", "https://my-sso-url", "--profile", "test-profile").Return([]byte{}, nil).Times(1)
	mockExecutor.EXPECT().RunCommand("aws", "configure", "set", "sso_role_name", "my-role", "--profile", "test-profile").Return([]byte{}, nil).Times(1)
	mockExecutor.EXPECT().RunCommand("aws", "configure", "set", "region", "us-west-2", "--profile", "test-profile").Return([]byte{}, nil).Times(1)
	mockExecutor.EXPECT().RunCommand("aws", "configure", "set", "output", "json", "--profile", "test-profile").Return([]byte{}, nil).Times(1)

	client := &sso.RealAWSConfigClient{Executor: mockExecutor}

	err := client.ConfigureSSOProfile("test-profile", "us-west-2", "123456789012", "my-role", "https://my-sso-url")
	assert.NoError(t, err)
}

func TestGetAWSRegion(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockExecutor := mock_awsctl.NewMockCommandExecutor(ctrl)

	mockExecutor.EXPECT().RunCommand("aws", "configure", "get", "region", "--profile", "test-profile").Return([]byte("us-west-2\n"), nil).Times(1)

	client := &sso.RealAWSConfigClient{Executor: mockExecutor}

	region, err := client.GetAWSRegion("test-profile")
	assert.NoError(t, err)
	assert.Equal(t, "us-west-2", region)
}

func TestGetAWSOutput(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockExecutor := mock_awsctl.NewMockCommandExecutor(ctrl)

	mockExecutor.EXPECT().RunCommand("aws", "configure", "get", "output", "--profile", "test-profile").Return([]byte("json\n"), nil).Times(1)

	client := &sso.RealAWSConfigClient{Executor: mockExecutor}

	output, err := client.GetAWSOutput("test-profile")
	assert.NoError(t, err)
	assert.Equal(t, "json", output)
}

func TestGetAWSRegion_ErrorCases(t *testing.T) {
	tests := []struct {
		name        string
		mockOutput  []byte
		mockError   error
		expectError bool
		errorMsg    string
	}{
		{
			name:        "command fails",
			mockError:   fmt.Errorf("command error"),
			expectError: true,
			errorMsg:    "failed to get AWS region",
		},
		{
			name:        "empty region",
			mockOutput:  []byte("\n"),
			expectError: true,
			errorMsg:    "AWS region not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockExecutor := mock_awsctl.NewMockCommandExecutor(ctrl)
			client := &sso.RealAWSConfigClient{Executor: mockExecutor}

			mockExecutor.EXPECT().
				RunCommand("aws", "configure", "get", "region", "--profile", "test-profile").
				Return(tt.mockOutput, tt.mockError)

			_, err := client.GetAWSRegion("test-profile")
			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGetAWSOutput_ErrorCases(t *testing.T) {
	tests := []struct {
		name        string
		mockOutput  []byte
		mockError   error
		expectError bool
		errorMsg    string
	}{
		{
			name:        "command fails",
			mockError:   fmt.Errorf("command error"),
			expectError: true,
			errorMsg:    "failed to get AWS output",
		},
		{
			name:        "empty output format",
			mockOutput:  []byte("\n"),
			expectError: true,
			errorMsg:    "AWS output not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockExecutor := mock_awsctl.NewMockCommandExecutor(ctrl)
			client := &sso.RealAWSConfigClient{Executor: mockExecutor}

			mockExecutor.EXPECT().
				RunCommand("aws", "configure", "get", "output", "--profile", "test-profile").
				Return(tt.mockOutput, tt.mockError)

			_, err := client.GetAWSOutput("test-profile")
			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConfigureDefaultProfile_ErrorCases(t *testing.T) {
	tests := []struct {
		name        string
		regionErr   error
		outputErr   error
		expectError bool
	}{
		{
			name:        "region set fails",
			regionErr:   fmt.Errorf("Error setting region"),
			expectError: true,
		},
		{
			name:        "output set fails",
			outputErr:   fmt.Errorf("Error setting output format"),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockExecutor := mock_awsctl.NewMockCommandExecutor(ctrl)
			client := &sso.RealAWSConfigClient{Executor: mockExecutor}

			mockExecutor.EXPECT().
				RunCommand("aws", "configure", "set", "region", "us-west-2", "--profile", "default").
				Return([]byte{}, tt.regionErr)

			if tt.regionErr == nil {
				mockExecutor.EXPECT().
					RunCommand("aws", "configure", "set", "output", "json", "--profile", "default").
					Return([]byte{}, tt.outputErr)
			}

			err := client.ConfigureDefaultProfile("us-west-2", "json")
			if tt.expectError {
				assert.Error(t, err)
				if tt.regionErr != nil {
					assert.Contains(t, err.Error(), "Error setting region")
				} else {
					assert.Contains(t, err.Error(), "Error setting output format")
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
