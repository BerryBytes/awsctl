package sso

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"testing"

	mock_awsctl "github.com/BerryBytes/awsctl/tests/mock"

	"github.com/golang/mock/gomock"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testGeneralUtils struct {
	checkAWSCLIError error
}

func (t *testGeneralUtils) CheckAWSCLI() error {
	return t.checkAWSCLIError
}

func (t *testGeneralUtils) HandleSignals() context.Context {
	return context.Background()
}

func (t *testGeneralUtils) PrintCurrentRole(profile, accountID, accountName, roleName, roleARN, expiration string) {
}

func TestNewSSOCommands(t *testing.T) {
	tests := []struct {
		name           string
		generalUtils   *testGeneralUtils
		expectedErr    error
		expectedOutput string
	}{
		{
			name:           "successful AWS CLI check",
			generalUtils:   &testGeneralUtils{},
			expectedOutput: "AWS CLI is installed and available in PATH.",
		},
		{
			name: "AWS CLI not installed",
			generalUtils: &testGeneralUtils{
				checkAWSCLIError: errors.New("aws cli not found"),
			},
			expectedErr:    errors.New("aws cli not found"),
			expectedOutput: "Error: aws cli not found\nPlease install AWS CLI first: https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			var outBuf, errBuf bytes.Buffer

			cmd := &cobra.Command{
				Use: "test",
				PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
					if err := tt.generalUtils.CheckAWSCLI(); err != nil {
						if _, err1 := fmt.Fprintln(cmd.ErrOrStderr(), "Error:", err); err1 != nil {
							return err1
						}

						if _, err2 := fmt.Fprintln(cmd.ErrOrStderr(), "Please install AWS CLI first: https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html"); err2 != nil {
							return err2
						}

						return err
					}

					if _, err := fmt.Fprintln(cmd.OutOrStdout(), "AWS CLI is installed and available in PATH."); err != nil {
						return err
					}

					return nil
				},
			}
			cmd.SetOut(&outBuf)
			cmd.SetErr(&errBuf)

			err := cmd.PersistentPreRunE(cmd, []string{})

			combinedOutput := outBuf.String() + errBuf.String()

			if tt.expectedErr != nil {
				require.Error(t, err)
				assert.Equal(t, tt.expectedErr.Error(), err.Error())
			} else {
				require.NoError(t, err)
			}

			if tt.expectedOutput != "" {
				assert.Contains(t, combinedOutput, tt.expectedOutput)
			}
		})
	}
}

func TestSetupSSO_Interrupt(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSSO := mock_awsctl.NewMockSSOClient(ctrl)
	mockSSO.EXPECT().SetupSSO().Return(promptui.ErrInterrupt)

	cmd := SetupCmd(mockSSO)
	cmd.SetArgs([]string{})

	var outBuf bytes.Buffer
	cmd.SetOut(&outBuf)

	err := cmd.Execute()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "SSO initialization failed")
	assert.Contains(t, err.Error(), "^C")
}

func TestSetupCmd_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSSOClient := mock_awsctl.NewMockSSOClient(ctrl)
	mockSSOClient.EXPECT().SetupSSO().Return(nil)

	cmd := SetupCmd(mockSSOClient)

	var outBuf bytes.Buffer
	cmd.SetOut(&outBuf)

	err := cmd.Execute()

	assert.NoError(t, err)
	assert.Contains(t, outBuf.String(), "AWS SSO setup completed successfully.")
}
