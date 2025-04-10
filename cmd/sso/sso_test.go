package sso

import (
	"bytes"
	"errors"
	"testing"

	mock_awsctl "github.com/BerryBytes/awsctl/tests/mock"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func TestNewSSOCommands(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSSOClient := mock_awsctl.NewMockSSOClient(ctrl)
	mockGeneralUtils := mock_awsctl.NewMockGeneralUtilsInterface(ctrl)

	cmd := NewSSOCommands(mockSSOClient, mockGeneralUtils)

	t.Run("Command Metadata", func(t *testing.T) {
		assert.Equal(t, "sso", cmd.Use, "Command use should be 'sso'")
		assert.Equal(t, "Manage AWS SSO configurations", cmd.Short, "Short description should match")
		assert.Equal(t, "A set of commands to manage and configure AWS SSO profiles.", cmd.Long, "Long description should match")
		assert.False(t, cmd.SilenceUsage, "SilenceUsage should be false initially")
	})

	t.Run("Command Structure", func(t *testing.T) {
		commands := cmd.Commands()
		assert.Len(t, commands, 2, "Should have exactly 2 subcommands")

		commandNames := make([]string, len(commands))
		for i, c := range commands {
			commandNames[i] = c.Use
		}
		assert.Contains(t, commandNames, "init", "Should have 'init' subcommand")
		assert.Contains(t, commandNames, "setup", "Should have 'setup' subcommand")
	})

	t.Run("PersistentPreRunE - Success", func(t *testing.T) {
		mockGeneralUtils.EXPECT().CheckAWSCLI().Return(nil)

		err := cmd.PersistentPreRunE(cmd, []string{})
		assert.NoError(t, err, "Should not return an error when CheckAWSCLI succeeds")
		assert.True(t, cmd.SilenceUsage, "SilenceUsage should be true after successful pre-run")
	})

	t.Run("PersistentPreRunE - AWS CLI Check Failure", func(t *testing.T) {
		cmd := NewSSOCommands(mockSSOClient, mockGeneralUtils)

		var outBuf bytes.Buffer
		cmd.SetOut(&outBuf)
		cmd.SetErr(&outBuf)

		mockGeneralUtils.EXPECT().CheckAWSCLI().Return(errors.New("AWS CLI not installed"))

		err := cmd.PersistentPreRunE(cmd, []string{})
		assert.Error(t, err, "Should return an error when CheckAWSCLI fails")
		assert.Equal(t, "AWS CLI not installed", err.Error(), "Error message should match")
		assert.Contains(t, outBuf.String(), "Please install AWS CLI first: https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html", "Should print installation instructions")
		assert.True(t, cmd.SilenceUsage, "SilenceUsage should be true even on failure")
	})
}
