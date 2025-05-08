package bastion_test

import (
	"errors"
	"testing"

	"github.com/BerryBytes/awsctl/cmd/bastion"
	mock_awsctl "github.com/BerryBytes/awsctl/tests/mock"
	promptutils "github.com/BerryBytes/awsctl/utils/prompt"
	"github.com/golang/mock/gomock"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func executeCommand(cmd *cobra.Command, args ...string) error {
	cmd.SetArgs(args)
	return cmd.Execute()
}

func TestBastionCmd_Run_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockService := mock_awsctl.NewMockBastionServiceInterface(ctrl)
	mockService.EXPECT().Run(gomock.Any()).Return(nil)

	cmd := bastion.NewBastionCmd(bastion.BastionDependencies{
		Service: mockService,
	})

	err := executeCommand(cmd)
	assert.NoError(t, err)
}

func TestBastionCmd_Run_Interrupted(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockService := mock_awsctl.NewMockBastionServiceInterface(ctrl)
	mockService.EXPECT().Run(gomock.Any()).Return(promptutils.ErrInterrupted)

	cmd := bastion.NewBastionCmd(bastion.BastionDependencies{
		Service: mockService,
	})

	err := executeCommand(cmd)
	assert.NoError(t, err)
}

func TestBastionCmd_Run_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	someErr := errors.New("unexpected error")

	mockService := mock_awsctl.NewMockBastionServiceInterface(ctrl)
	mockService.EXPECT().Run(gomock.Any()).Return(someErr)

	cmd := bastion.NewBastionCmd(bastion.BastionDependencies{
		Service: mockService,
	})

	err := executeCommand(cmd)
	assert.EqualError(t, err, "unexpected error")
}
