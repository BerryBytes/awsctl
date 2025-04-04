package sso

import (
	"testing"

	mock_awsctl "github.com/BerryBytes/awsctl/tests/mock"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func TestAbortSetup(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mock_awsctl.NewMockAWSUtilityClient(ctrl)

	mockClient.EXPECT().AbortSetup(gomock.Any()).Return(nil).Times(1)

	err := mockClient.AbortSetup(nil)

	assert.NoError(t, err, "AbortSetup should not return an error")
}

func TestPrintCurrentRole(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mock_awsctl.NewMockAWSUtilityClient(ctrl)

	profile := "myProfile"
	accountID := "123456789012"
	accountName := "myAccount"
	roleName := "myRole"
	roleARN := "arn:aws:iam::123456789012:role/myRole"
	expiration := "2025-03-25T00:00:00Z"

	mockClient.EXPECT().PrintCurrentRole(profile, accountID, accountName, roleName, roleARN, expiration).Times(1)

	mockClient.PrintCurrentRole(profile, accountID, accountName, roleName, roleARN, expiration)
}
