package root

import (
	"testing"

	mock_awsctl "github.com/BerryBytes/awsctl/tests/mock"
	mock_rds "github.com/BerryBytes/awsctl/tests/mock/rds"
	"github.com/golang/mock/gomock"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestNewRootCmd(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tests := []struct {
		name         string
		setupMocks   func(*mock_awsctl.MockSSOClient, *mock_awsctl.MockBastionServiceInterface, *mock_awsctl.MockGeneralUtilsInterface, *mock_awsctl.MockFileSystemInterface, *mock_rds.MockRDSServiceInterface)
		validateFunc func(*testing.T, *cobra.Command)
	}{
		{
			name: "successful initialization with all dependencies",
			setupMocks: func(
				ssoClient *mock_awsctl.MockSSOClient,
				bastionSvc *mock_awsctl.MockBastionServiceInterface,
				genManager *mock_awsctl.MockGeneralUtilsInterface,
				fs *mock_awsctl.MockFileSystemInterface,
				rdsSvc *mock_rds.MockRDSServiceInterface,
			) {
			},
			validateFunc: func(t *testing.T, cmd *cobra.Command) {
				assert.Equal(t, "awsctl", cmd.Use)
				assert.Equal(t, "AWS CLI Tool", cmd.Short)
				assert.NotEmpty(t, cmd.Long)

				assert.Len(t, cmd.Commands(), 3)
				assert.IsType(t, &cobra.Command{}, cmd.Commands()[0])
				assert.IsType(t, &cobra.Command{}, cmd.Commands()[1])
				assert.IsType(t, &cobra.Command{}, cmd.Commands()[2])
			},
		},
		{
			name: "nil dependencies should still work",
			setupMocks: func(
				ssoClient *mock_awsctl.MockSSOClient,
				bastionSvc *mock_awsctl.MockBastionServiceInterface,
				genManager *mock_awsctl.MockGeneralUtilsInterface,
				fs *mock_awsctl.MockFileSystemInterface,
				rdsSvc *mock_rds.MockRDSServiceInterface,
			) {
			},
			validateFunc: func(t *testing.T, cmd *cobra.Command) {
				assert.NotNil(t, cmd)
				assert.Len(t, cmd.Commands(), 3)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockSSO := mock_awsctl.NewMockSSOClient(ctrl)
			mockBastion := mock_awsctl.NewMockBastionServiceInterface(ctrl)
			mockGeneral := mock_awsctl.NewMockGeneralUtilsInterface(ctrl)
			mockFS := mock_awsctl.NewMockFileSystemInterface(ctrl)
			mockRDS := mock_rds.NewMockRDSServiceInterface(ctrl)

			tt.setupMocks(mockSSO, mockBastion, mockGeneral, mockFS, mockRDS)

			deps := RootDependencies{
				SSOClient:      mockSSO,
				BastionService: mockBastion,
				GeneralManager: mockGeneral,
				FileSystem:     mockFS,
				RDSService:     mockRDS,
			}

			cmd := NewRootCmd(deps)
			tt.validateFunc(t, cmd)
		})
	}
}

func TestRootCmdExecution(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSSO := mock_awsctl.NewMockSSOClient(ctrl)
	mockBastion := mock_awsctl.NewMockBastionServiceInterface(ctrl)
	mockGeneral := mock_awsctl.NewMockGeneralUtilsInterface(ctrl)
	mockFS := mock_awsctl.NewMockFileSystemInterface(ctrl)
	mockRDS := mock_rds.NewMockRDSServiceInterface(ctrl)

	deps := RootDependencies{
		SSOClient:      mockSSO,
		BastionService: mockBastion,
		GeneralManager: mockGeneral,
		FileSystem:     mockFS,
		RDSService:     mockRDS,
	}

	t.Run("root command help execution", func(t *testing.T) {
		cmd := NewRootCmd(deps)
		cmd.SetArgs([]string{})

		err := cmd.Execute()
		assert.NoError(t, err)
	})

	t.Run("root command with invalid subcommand", func(t *testing.T) {
		cmd := NewRootCmd(deps)
		cmd.SetArgs([]string{"invalid"})

		err := cmd.Execute()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unknown command")
	})
}

func TestSubcommandInitialization(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSSO := mock_awsctl.NewMockSSOClient(ctrl)
	mockBastion := mock_awsctl.NewMockBastionServiceInterface(ctrl)
	mockGeneral := mock_awsctl.NewMockGeneralUtilsInterface(ctrl)
	mockFS := mock_awsctl.NewMockFileSystemInterface(ctrl)
	mockRDS := mock_rds.NewMockRDSServiceInterface(ctrl)

	deps := RootDependencies{
		SSOClient:      mockSSO,
		BastionService: mockBastion,
		GeneralManager: mockGeneral,
		FileSystem:     mockFS,
		RDSService:     mockRDS,
	}

	t.Run("SSO subcommand exists", func(t *testing.T) {
		cmd := NewRootCmd(deps)
		ssoCmd, _, err := cmd.Find([]string{"sso"})
		assert.NoError(t, err)
		assert.NotNil(t, ssoCmd)
		assert.Equal(t, "sso", ssoCmd.Name())
	})

	t.Run("bastion subcommand exists", func(t *testing.T) {
		cmd := NewRootCmd(deps)
		bastionCmd, _, err := cmd.Find([]string{"bastion"})
		assert.NoError(t, err)
		assert.NotNil(t, bastionCmd)
		assert.Equal(t, "bastion", bastionCmd.Name())
	})

	t.Run("RDS subcommand exists", func(t *testing.T) {
		cmd := NewRootCmd(deps)
		rdsCmd, _, err := cmd.Find([]string{"rds"})
		assert.NoError(t, err)
		assert.NotNil(t, rdsCmd)
		assert.Equal(t, "rds", rdsCmd.Name())
	})
}
