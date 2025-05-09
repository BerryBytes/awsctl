package ecr_test

import (
	"errors"
	"testing"

	"github.com/BerryBytes/awsctl/cmd/ecr"
	mock_ecr "github.com/BerryBytes/awsctl/tests/mock/ecr"
	promptutils "github.com/BerryBytes/awsctl/utils/prompt"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func TestNewECRCmd(t *testing.T) {
	tests := []struct {
		name          string
		mockSetup     func(*mock_ecr.MockECRServiceInterface)
		args          []string
		expectedError error
	}{
		{
			name: "successful execution",
			mockSetup: func(mockSvc *mock_ecr.MockECRServiceInterface) {
				mockSvc.EXPECT().Run().Return(nil)
			},
			args:          []string{},
			expectedError: nil,
		},
		{
			name: "user interruption",
			mockSetup: func(mockSvc *mock_ecr.MockECRServiceInterface) {
				mockSvc.EXPECT().Run().Return(promptutils.ErrInterrupted)
			},
			args:          []string{},
			expectedError: nil,
		},
		{
			name: "service error",
			mockSetup: func(mockSvc *mock_ecr.MockECRServiceInterface) {
				mockSvc.EXPECT().Run().Return(errors.New("ecr login failed"))
			},
			args:          []string{},
			expectedError: errors.New("ecr login failed"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockService := mock_ecr.NewMockECRServiceInterface(ctrl)
			if tt.mockSetup != nil {
				tt.mockSetup(mockService)
			}

			deps := ecr.ECRDependencies{
				Service: mockService,
			}

			cmd := ecr.NewECRCmd(deps)
			cmd.SetArgs(tt.args)

			err := cmd.Execute()
			if tt.expectedError != nil {
				assert.EqualError(t, err, tt.expectedError.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestECRCmdConfiguration(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockService := mock_ecr.NewMockECRServiceInterface(ctrl)
	deps := ecr.ECRDependencies{
		Service: mockService,
	}

	cmd := ecr.NewECRCmd(deps)

	assert.Equal(t, "ecr", cmd.Use)
	assert.Equal(t, "Interactive AWS ECR login manager", cmd.Short)
	assert.Contains(t, cmd.Long, "Interactive menu for logging into AWS Elastic Container Registry (ECR)")
	assert.True(t, cmd.SilenceUsage)
	assert.NotNil(t, cmd.RunE)
}

func TestECRCmdArgsHandling(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockService := mock_ecr.NewMockECRServiceInterface(ctrl)
	mockService.EXPECT().Run().Return(nil).AnyTimes()

	deps := ecr.ECRDependencies{
		Service: mockService,
	}

	t.Run("no arguments", func(t *testing.T) {
		cmd := ecr.NewECRCmd(deps)
		cmd.SetArgs([]string{})
		err := cmd.Execute()
		assert.NoError(t, err)
	})

	t.Run("with arguments (should ignore)", func(t *testing.T) {
		cmd := ecr.NewECRCmd(deps)
		cmd.SetArgs([]string{"extra", "args"})
		err := cmd.Execute()
		assert.NoError(t, err)
	})
}
