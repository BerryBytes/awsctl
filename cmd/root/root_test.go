package root

import (
	"bytes"
	"errors"
	"testing"

	cmdSSO "github.com/BerryBytes/awsctl/cmd/sso"
	mock_awsctl "github.com/BerryBytes/awsctl/tests/mock"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRootCmd(t *testing.T) {
	tests := []struct {
		name          string
		expectedUse   string
		expectedShort string
		expectedLong  string
	}{
		{
			name:          "root command metadata",
			expectedUse:   "awsctl",
			expectedShort: "AWS CLI Tool",
			expectedLong:  "A CLI tool for managing AWS services and configurations.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockSSOClient := mock_awsctl.NewMockSSOClient(ctrl)
			mockBastionService := mock_awsctl.NewMockBastionServiceInterface(ctrl)
			mockGeneralManager := mock_awsctl.NewMockGeneralUtilsInterface(ctrl)

			rootCmd := NewRootCmd(mockSSOClient, mockBastionService, mockGeneralManager)

			assert.Equal(t, tt.expectedUse, rootCmd.Use)
			assert.Equal(t, tt.expectedShort, rootCmd.Short)
			assert.Contains(t, rootCmd.Long, tt.expectedLong)
		})
	}
}

func TestRootCommandStructure(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSSOClient := mock_awsctl.NewMockSSOClient(ctrl)
	mockBastionService := mock_awsctl.NewMockBastionServiceInterface(ctrl)
	mockGeneralManager := mock_awsctl.NewMockGeneralUtilsInterface(ctrl)

	rootCmd := NewRootCmd(mockSSOClient, mockBastionService, mockGeneralManager)

	ssoCmd := cmdSSO.NewSSOCommands(mockSSOClient, mockGeneralManager)
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == ssoCmd.Use {
			found = true
			break
		}
	}
	assert.True(t, found, "SSO command should be registered under root")

	assert.GreaterOrEqual(t, len(rootCmd.Commands()), 1, "Root command should have at least one subcommand")
}

func TestRootCmd_Execution(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		expectedOutput string
		expectedErr    error
	}{
		{
			name:           "help command",
			args:           []string{"help"},
			expectedOutput: "Usage:",
			expectedErr:    nil,
		},
		{
			name:           "no args shows help",
			args:           []string{},
			expectedOutput: "Usage:",
			expectedErr:    nil,
		},
		{
			name:           "invalid command",
			args:           []string{"invalid"},
			expectedOutput: "unknown command",
			expectedErr:    errors.New("unknown command"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockSSOClient := mock_awsctl.NewMockSSOClient(ctrl)
			mockBastionService := mock_awsctl.NewMockBastionServiceInterface(ctrl)
			mockGeneralManager := mock_awsctl.NewMockGeneralUtilsInterface(ctrl)

			rootCmd := NewRootCmd(mockSSOClient, mockBastionService, mockGeneralManager)

			var outBuf bytes.Buffer
			rootCmd.SetOut(&outBuf)
			rootCmd.SetErr(&outBuf)

			rootCmd.SetArgs(tt.args)
			err := rootCmd.Execute()

			if tt.expectedErr != nil {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr.Error())
			} else {
				require.NoError(t, err)
			}

			if tt.expectedOutput != "" {
				assert.Contains(t, outBuf.String(), tt.expectedOutput)
			}
		})
	}
}

func TestRootCmd_SubcommandExecution(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSSOClient := mock_awsctl.NewMockSSOClient(ctrl)
	mockBastionService := mock_awsctl.NewMockBastionServiceInterface(ctrl)
	mockGeneralUtils := mock_awsctl.NewMockGeneralUtilsInterface(ctrl)

	// Expect CheckAWSCLI to be called and return nil for successful execution
	mockGeneralUtils.EXPECT().CheckAWSCLI().Return(nil)
	mockSSOClient.EXPECT().SetupSSO().Return(nil)

	rootCmd := NewRootCmd(mockSSOClient, mockBastionService, mockGeneralUtils)

	rootCmd.SetArgs([]string{"sso", "setup"})

	var outBuf bytes.Buffer
	rootCmd.SetOut(&outBuf)

	err := rootCmd.Execute()
	assert.NoError(t, err)

	assert.Contains(t, outBuf.String(), "AWS SSO setup completed successfully")
}
