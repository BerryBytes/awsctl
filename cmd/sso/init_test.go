package sso

import (
	"errors"
	"testing"

	mock_sso "awsctl/tests/mocks"
	promptutils "awsctl/utils/prompt"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func TestInitCmd(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSSOClient := mock_sso.NewMockSSOClient(ctrl)

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
			expectedError: "failed to set up AWS SSO: initialization error",
		},
		{
			name:          "interrupted by user",
			args:          []string{},
			refreshFlag:   false,
			noBrowserFlag: false,
			mockSetup: func() {
				mockSSOClient.EXPECT().InitSSO(false, false).Return(promptutils.ErrInterrupted)
			},
			expectedError: "",
		},
		{
			name:          "error parsing refresh flag",
			args:          []string{"--refresh=invalid"},
			refreshFlag:   false,
			noBrowserFlag: false,
			mockSetup:     func() {},
			expectedError: `invalid argument "invalid" for "-r, --refresh" flag: strconv.ParseBool: parsing "invalid": invalid syntax`,
		},
		{
			name:          "error parsing no-browser flag",
			args:          []string{"--no-browser=invalid"},
			refreshFlag:   false,
			noBrowserFlag: false,
			mockSetup:     func() {},
			expectedError: `invalid argument "invalid" for "--no-browser" flag: strconv.ParseBool: parsing "invalid": invalid syntax`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			cmd := InitCmd(mockSSOClient)
			cmd.SetArgs(tt.args)

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
