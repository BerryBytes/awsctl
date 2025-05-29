package sso_test

import (
	"errors"
	"testing"

	"github.com/BerryBytes/awsctl/cmd/sso"
	mock_awsctl "github.com/BerryBytes/awsctl/tests/mock"
	mock_sso "github.com/BerryBytes/awsctl/tests/mock/sso"

	"github.com/golang/mock/gomock"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func executePersistentPreRunE(cmd *cobra.Command) error {
	cmd.SetArgs([]string{})
	return cmd.PersistentPreRunE(cmd, []string{})
}

func TestSSOCmd_PersistentPreRun_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockGeneral := mock_awsctl.NewMockGeneralUtilsInterface(ctrl)
	mockGeneral.EXPECT().CheckAWSCLI().Return(nil)

	mockSSO := mock_sso.NewMockSSOClient(ctrl)

	cmd := sso.NewSSOCommands(sso.SSODependencies{
		SetupClient:    mockSSO,
		GeneralManager: mockGeneral,
	})

	err := executePersistentPreRunE(cmd)
	assert.NoError(t, err)
}

func TestSSOCmd_PersistentPreRun_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockGeneral := mock_awsctl.NewMockGeneralUtilsInterface(ctrl)
	mockGeneral.EXPECT().CheckAWSCLI().Return(errors.New("missing aws cli"))

	mockSSO := mock_sso.NewMockSSOClient(ctrl)

	cmd := sso.NewSSOCommands(sso.SSODependencies{
		SetupClient:    mockSSO,
		GeneralManager: mockGeneral,
	})

	err := executePersistentPreRunE(cmd)
	assert.EqualError(t, err, "missing aws cli")
}

func TestSSOCmd_HasSubcommands(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockGeneral := mock_awsctl.NewMockGeneralUtilsInterface(ctrl)
	mockGeneral.EXPECT().CheckAWSCLI().Return(nil).AnyTimes()

	mockSSO := mock_sso.NewMockSSOClient(ctrl)

	cmd := sso.NewSSOCommands(sso.SSODependencies{
		SetupClient:    mockSSO,
		GeneralManager: mockGeneral,
	})

	subcommands := cmd.Commands()
	var names []string
	for _, c := range subcommands {
		names = append(names, c.Name())
	}

	assert.Contains(t, names, "init")
	assert.Contains(t, names, "setup")
}
