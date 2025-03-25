package sso

import (
	"errors"
	"testing"

	mock_sso "github.com/BerryBytes/awsctl/tests/mocks"
	promptUtils "github.com/BerryBytes/awsctl/utils/prompt"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func TestSetupCmd(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSSOClient := mock_sso.NewMockSSOClient(ctrl)

	tests := []struct {
		name          string
		mockSetup     func()
		expectedError string
	}{
		{
			name: "successful setup",
			mockSetup: func() {
				mockSSOClient.EXPECT().SetupSSO().Return(nil)
			},
			expectedError: "",
		},
		{
			name: "error during setup",
			mockSetup: func() {
				mockSSOClient.EXPECT().SetupSSO().Return(errors.New("setup error"))
			},
			expectedError: "setup error",
		},
		{
			name: "interrupted by user",
			mockSetup: func() {
				mockSSOClient.EXPECT().SetupSSO().Return(promptUtils.ErrInterrupted)
			},
			expectedError: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			cmd := SetupCmd(mockSSOClient)
			cmd.SetArgs([]string{})

			err := cmd.Execute()

			if tt.expectedError == "" {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			}
		})
	}
}
