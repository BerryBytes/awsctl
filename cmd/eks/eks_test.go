package eks_test

import (
	"errors"
	"testing"

	"github.com/BerryBytes/awsctl/cmd/eks"
	mock_eks "github.com/BerryBytes/awsctl/tests/mock/eks"
	promptutils "github.com/BerryBytes/awsctl/utils/prompt"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func TestNewEKSCmd(t *testing.T) {
	tests := []struct {
		name          string
		mockSetup     func(*mock_eks.MockEKSServiceInterface)
		args          []string
		expectedError error
	}{
		{
			name: "successful execution",
			mockSetup: func(mockSvc *mock_eks.MockEKSServiceInterface) {
				mockSvc.EXPECT().Run().Return(nil)
			},
			args:          []string{},
			expectedError: nil,
		},
		{
			name: "user interruption",
			mockSetup: func(mockSvc *mock_eks.MockEKSServiceInterface) {
				mockSvc.EXPECT().Run().Return(promptutils.ErrInterrupted)
			},
			args:          []string{},
			expectedError: nil,
		},
		{
			name: "service error",
			mockSetup: func(mockSvc *mock_eks.MockEKSServiceInterface) {
				mockSvc.EXPECT().Run().Return(errors.New("cluster access failed"))
			},
			args:          []string{},
			expectedError: errors.New("cluster access failed"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockService := mock_eks.NewMockEKSServiceInterface(ctrl)
			if tt.mockSetup != nil {
				tt.mockSetup(mockService)
			}

			deps := eks.EKSDependencies{
				Service: mockService,
			}

			cmd := eks.NewEKSCmd(deps)
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

func TestEKSCmdConfiguration(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockService := mock_eks.NewMockEKSServiceInterface(ctrl)
	deps := eks.EKSDependencies{
		Service: mockService,
	}

	cmd := eks.NewEKSCmd(deps)

	assert.Equal(t, "eks", cmd.Use)
	assert.Equal(t, "Interactive EKS cluster manager", cmd.Short)
	assert.Contains(t, cmd.Long, "Interactive menu for managing EKS cluster configurations")
	assert.True(t, cmd.SilenceUsage)
	assert.NotNil(t, cmd.RunE)
}

func TestEKSCmdRunBehavior(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	t.Run("normal execution path", func(t *testing.T) {
		mockService := mock_eks.NewMockEKSServiceInterface(ctrl)
		mockService.EXPECT().Run().Return(nil)

		deps := eks.EKSDependencies{
			Service: mockService,
		}

		cmd := eks.NewEKSCmd(deps)
		err := cmd.Execute()
		assert.NoError(t, err)
	})

	t.Run("error propagation", func(t *testing.T) {
		expectedErr := errors.New("database error")
		mockService := mock_eks.NewMockEKSServiceInterface(ctrl)
		mockService.EXPECT().Run().Return(expectedErr)

		deps := eks.EKSDependencies{
			Service: mockService,
		}

		cmd := eks.NewEKSCmd(deps)
		err := cmd.Execute()
		assert.Equal(t, expectedErr, err)
	})

	t.Run("user interruption handling", func(t *testing.T) {
		mockService := mock_eks.NewMockEKSServiceInterface(ctrl)
		mockService.EXPECT().Run().Return(promptutils.ErrInterrupted)

		deps := eks.EKSDependencies{
			Service: mockService,
		}

		cmd := eks.NewEKSCmd(deps)
		err := cmd.Execute()
		assert.NoError(t, err)
	})
}

func TestEKSCmdArgsHandling(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockService := mock_eks.NewMockEKSServiceInterface(ctrl)
	mockService.EXPECT().Run().Return(nil).AnyTimes()

	deps := eks.EKSDependencies{
		Service: mockService,
	}

	t.Run("no arguments", func(t *testing.T) {
		cmd := eks.NewEKSCmd(deps)
		cmd.SetArgs([]string{})
		err := cmd.Execute()
		assert.NoError(t, err)
	})

	t.Run("with arguments (should ignore)", func(t *testing.T) {
		cmd := eks.NewEKSCmd(deps)
		cmd.SetArgs([]string{"extra", "args"})
		err := cmd.Execute()
		assert.NoError(t, err)
	})
}
