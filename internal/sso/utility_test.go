package sso_test

import (
	"errors"
	"testing"

	"github.com/BerryBytes/awsctl/internal/sso"
	mock_awsctl "github.com/BerryBytes/awsctl/tests/mock"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func TestRealAWSUtilityClient_AbortSetup(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tests := []struct {
		name        string
		inputErr    error
		expectedErr error
	}{
		{
			name:        "with error",
			inputErr:    errors.New("test error"),
			expectedErr: errors.New("test error"),
		},
		{
			name:        "without error",
			inputErr:    nil,
			expectedErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &sso.RealAWSUtilityClient{
				GeneralManager: nil,
			}

			err := client.AbortSetup(tt.inputErr)

			if tt.expectedErr != nil {
				assert.EqualError(t, err, tt.expectedErr.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestRealAWSUtilityClient_PrintCurrentRole(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	profile := "myProfile"
	accountID := "123456789012"
	accountName := "myAccount"
	roleName := "myRole"
	roleARN := "arn:aws:iam::123456789012:role/myRole"
	expiration := "2025-03-25T00:00:00Z"

	mockGeneralManager := mock_awsctl.NewMockGeneralUtilsInterface(ctrl)
	mockGeneralManager.EXPECT().
		PrintCurrentRole(profile, accountID, accountName, roleName, roleARN, expiration).
		Times(1)

	client := &sso.RealAWSUtilityClient{
		GeneralManager: mockGeneralManager,
	}

	client.PrintCurrentRole(profile, accountID, accountName, roleName, roleARN, expiration)
}
