package rds_test

import (
	"errors"
	"testing"

	"github.com/BerryBytes/awsctl/cmd/rds"
	mock_rds "github.com/BerryBytes/awsctl/tests/mock/rds"
	promptutils "github.com/BerryBytes/awsctl/utils/prompt"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func TestNewRDSCmd(t *testing.T) {
	tests := []struct {
		name          string
		mockSetup     func(*mock_rds.MockRDSServiceInterface)
		args          []string
		expectedError error
	}{
		{
			name: "successful execution",
			mockSetup: func(mockSvc *mock_rds.MockRDSServiceInterface) {
				mockSvc.EXPECT().Run().Return(nil)
			},
			args:          []string{},
			expectedError: nil,
		},
		{
			name: "user interruption",
			mockSetup: func(mockSvc *mock_rds.MockRDSServiceInterface) {
				mockSvc.EXPECT().Run().Return(promptutils.ErrInterrupted)
			},
			args:          []string{},
			expectedError: nil, // Should convert interruption to nil
		},
		{
			name: "service error",
			mockSetup: func(mockSvc *mock_rds.MockRDSServiceInterface) {
				mockSvc.EXPECT().Run().Return(errors.New("connection failed"))
			},
			args:          []string{},
			expectedError: errors.New("connection failed"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockService := mock_rds.NewMockRDSServiceInterface(ctrl)
			if tt.mockSetup != nil {
				tt.mockSetup(mockService)
			}

			deps := rds.RDSDependencies{
				Service: mockService,
			}

			cmd := rds.NewRDSCmd(deps)
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

func TestRDSCmdConfiguration(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockService := mock_rds.NewMockRDSServiceInterface(ctrl)
	deps := rds.RDSDependencies{
		Service: mockService,
	}

	cmd := rds.NewRDSCmd(deps)

	// Test command metadata
	assert.Equal(t, "rds", cmd.Use)
	assert.Equal(t, "Interactive RDS connection manager", cmd.Short)
	assert.Contains(t, cmd.Long, "Interactive menu for managing RDS database connections")
	assert.True(t, cmd.SilenceUsage)

	// Test command hierarchy
	assert.NotNil(t, cmd.RunE)
}

func TestRDSCmdRunBehavior(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	t.Run("normal execution path", func(t *testing.T) {
		mockService := mock_rds.NewMockRDSServiceInterface(ctrl)
		mockService.EXPECT().Run().Return(nil)

		deps := rds.RDSDependencies{
			Service: mockService,
		}

		cmd := rds.NewRDSCmd(deps)
		err := cmd.Execute()
		assert.NoError(t, err)
	})

	t.Run("error propagation", func(t *testing.T) {
		expectedErr := errors.New("database error")
		mockService := mock_rds.NewMockRDSServiceInterface(ctrl)
		mockService.EXPECT().Run().Return(expectedErr)

		deps := rds.RDSDependencies{
			Service: mockService,
		}

		cmd := rds.NewRDSCmd(deps)
		err := cmd.Execute()
		assert.Equal(t, expectedErr, err)
	})

	t.Run("user interruption handling", func(t *testing.T) {
		mockService := mock_rds.NewMockRDSServiceInterface(ctrl)
		mockService.EXPECT().Run().Return(promptutils.ErrInterrupted)

		deps := rds.RDSDependencies{
			Service: mockService,
		}

		cmd := rds.NewRDSCmd(deps)
		err := cmd.Execute()
		assert.NoError(t, err)
	})
}

func TestRDSCmdArgsHandling(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockService := mock_rds.NewMockRDSServiceInterface(ctrl)
	mockService.EXPECT().Run().Return(nil).AnyTimes() // Allow any number of calls

	deps := rds.RDSDependencies{
		Service: mockService,
	}

	t.Run("no arguments", func(t *testing.T) {
		cmd := rds.NewRDSCmd(deps)
		cmd.SetArgs([]string{})
		err := cmd.Execute()
		assert.NoError(t, err)
	})

	t.Run("with arguments (should ignore)", func(t *testing.T) {
		cmd := rds.NewRDSCmd(deps)
		cmd.SetArgs([]string{"extra", "args"})
		err := cmd.Execute()
		assert.NoError(t, err)
	})
}
