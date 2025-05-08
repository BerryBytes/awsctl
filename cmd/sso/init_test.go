package sso

import (
	"bytes"
	"errors"
	"testing"

	mock_awsctl "github.com/BerryBytes/awsctl/tests/mock"
	promptUtils "github.com/BerryBytes/awsctl/utils/prompt"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func TestInitCmd(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSSOClient := mock_awsctl.NewMockSSOClient(ctrl)

	tests := []struct {
		name          string
		args          []string
		refreshFlag   bool
		noBrowserFlag bool
		mockSetup     func()
		expectedError string
	}{
		{
			name:          "successful initialization",
			args:          []string{},
			refreshFlag:   false,
			noBrowserFlag: false,
			mockSetup: func() {
				mockSSOClient.EXPECT().InitSSO(false, false).Return(nil)
			},
			expectedError: "",
		},
		{
			name:          "successful initialization with refresh flag",
			args:          []string{"--refresh"},
			refreshFlag:   true,
			noBrowserFlag: false,
			mockSetup: func() {
				mockSSOClient.EXPECT().InitSSO(true, false).Return(nil)
			},
			expectedError: "",
		},
		{
			name:          "successful initialization with no-browser flag",
			args:          []string{"--no-browser"},
			refreshFlag:   false,
			noBrowserFlag: true,
			mockSetup: func() {
				mockSSOClient.EXPECT().InitSSO(false, true).Return(nil)
			},
			expectedError: "",
		},
		{
			name:          "error during initialization",
			args:          []string{},
			refreshFlag:   false,
			noBrowserFlag: false,
			mockSetup: func() {
				mockSSOClient.EXPECT().InitSSO(false, false).Return(errors.New("initialization error"))
			},
			expectedError: "SSO initialization failed: initialization error",
		},
		{
			name:          "interrupted by user",
			args:          []string{},
			refreshFlag:   false,
			noBrowserFlag: false,
			mockSetup: func() {
				mockSSOClient.EXPECT().InitSSO(false, false).Return(promptUtils.ErrInterrupted)
			},
			expectedError: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.mockSetup == nil {
				t.Fatalf("mockSetup function is nil for test case: %s", tt.name)
			}

			tt.mockSetup()

			if mockSSOClient == nil {
				t.Fatal("mockSSOClient is nil, ensure it is properly initialized")
			}

			cmd := InitCmd(mockSSOClient)
			cmd.SetArgs(tt.args)

			err := cmd.Execute()

			if tt.expectedError == "" {
				assert.NoError(t, err, "Expected no error but got one in test case: %s", tt.name)
			} else {
				assert.Error(t, err, "Expected an error but got nil in test case: %s", tt.name)
				assert.Contains(t, err.Error(), tt.expectedError, "Unexpected error message in test case: %s", tt.name)
			}
		})
	}
}

func TestInitCmd_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSSOClient := mock_awsctl.NewMockSSOClient(ctrl)
	mockSSOClient.EXPECT().InitSSO(false, false).Return(nil)

	cmd := InitCmd(mockSSOClient)

	var outBuf bytes.Buffer
	cmd.SetOut(&outBuf)

	cmd.SetArgs([]string{})
	err := cmd.Execute()

	assert.NoError(t, err)
	assert.Contains(t, outBuf.String(), "AWS SSO init completed successfully")
}
