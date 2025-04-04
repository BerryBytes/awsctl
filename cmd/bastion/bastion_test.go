package bastion

import (
	"errors"
	"testing"

	mock_sso "github.com/BerryBytes/awsctl/tests/mock"
	promptUtils "github.com/BerryBytes/awsctl/utils/prompt"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func TestNewBastionCmd_Run_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockBastionSvc := mock_sso.NewMockBastionServiceInterface(ctrl)
	mockBastionSvc.EXPECT().Run().Return(nil)

	cmd := NewBastionCmd(mockBastionSvc)
	err := cmd.Execute()

	assert.NoError(t, err)
}

func TestNewBastionCmd_Run_WithErrInterrupted(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockBastionSvc := mock_sso.NewMockBastionServiceInterface(ctrl)
	mockBastionSvc.EXPECT().Run().Return(promptUtils.ErrInterrupted)

	cmd := NewBastionCmd(mockBastionSvc)
	err := cmd.Execute()

	assert.NoError(t, err)
}

func TestNewBastionCmd_Run_WithUnexpectedError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockBastionSvc := mock_sso.NewMockBastionServiceInterface(ctrl)
	mockBastionSvc.EXPECT().Run().Return(errors.New("unexpected error"))

	cmd := NewBastionCmd(mockBastionSvc)
	err := cmd.Execute()

	assert.Error(t, err)
	assert.Equal(t, "unexpected error", err.Error())
}
