package rds_test

import (
	"errors"
	"testing"

	"github.com/BerryBytes/awsctl/internal/rds"
	"github.com/BerryBytes/awsctl/models"
	mock_awsctl "github.com/BerryBytes/awsctl/tests/mock"
	mock_rds "github.com/BerryBytes/awsctl/tests/mock/rds"
	promptUtils "github.com/BerryBytes/awsctl/utils/prompt"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func setupTest(t *testing.T) (*rds.RDSService, *gomock.Controller, *mock_rds.MockRDSPromptInterface, *mock_rds.MockRDSAdapterInterface, *mock_awsctl.MockConnectionPrompter, *mock_awsctl.MockServicesInterface, *mock_awsctl.MockPrompter) {
	ctrl := gomock.NewController(t)
	mockRPrompter := mock_rds.NewMockRDSPromptInterface(ctrl)
	mockRDSClient := mock_rds.NewMockRDSAdapterInterface(ctrl)
	mockConnPrompter := mock_awsctl.NewMockConnectionPrompter(ctrl)
	mockConnServices := mock_awsctl.NewMockServicesInterface(ctrl)
	mockGPrompter := mock_awsctl.NewMockPrompter(ctrl)

	service := &rds.RDSService{
		RPrompter:    mockRPrompter,
		RDSClient:    mockRDSClient,
		CPrompter:    mockConnPrompter,
		ConnServices: mockConnServices,
		GPrompter:    mockGPrompter,
	}

	return service, ctrl, mockRPrompter, mockRDSClient, mockConnPrompter, mockConnServices, mockGPrompter
}

func TestRDSService_Run(t *testing.T) {
	svc, ctrl, mockRPrompter, _, _, _, _ := setupTest(t)
	defer ctrl.Finish()

	t.Run("ExitAction", func(t *testing.T) {
		mockRPrompter.EXPECT().SelectRDSAction().Return(rds.ExitRDS, nil)

		err := svc.Run()
		assert.NoError(t, err)
	})

	t.Run("Interrupted", func(t *testing.T) {
		mockRPrompter.EXPECT().SelectRDSAction().Return(rds.ExitRDS, promptUtils.ErrInterrupted)

		err := svc.Run()
		assert.Equal(t, promptUtils.ErrInterrupted, err)
	})
}

func TestHandleDirectConnection_Success(t *testing.T) {
	svc, ctrl, mockRPrompter, mockRDSAdapter, mockConnPrompter, mockConnServices, _ := setupTest(t)
	defer ctrl.Finish()

	mockConnServices.EXPECT().IsAWSConfigured().Return(true)
	mockConnPrompter.EXPECT().PromptForConfirmation("Look for RDS instances in AWS?").Return(false, nil)
	mockRPrompter.EXPECT().PromptForManualEndpoint().Return("localhost:5432", "admin", "us-west-2", nil)
	mockRDSAdapter.EXPECT().GenerateAuthToken("localhost:5432", "admin", "us-west-2").Return("mock-auth-token", nil)
	err := svc.HandleDirectConnection()
	assert.NoError(t, err)
}

func TestRDSService_HandleTunnelConnection(t *testing.T) {
	svc, ctrl, mockRPrompter, mockRDSAdapter, mockConnPrompter, mockConnServices, _ := setupTest(t)
	defer ctrl.Finish()

	t.Run("HappyPath", func(t *testing.T) {
		mockConnServices.EXPECT().IsAWSConfigured().Return(true)
		mockConnPrompter.EXPECT().PromptForConfirmation("Look for RDS instances in AWS?").Return(false, nil)
		mockRPrompter.EXPECT().PromptForManualEndpoint().Return("test-rds:3306", "test-user", "us-east-1", nil)
		mockConnPrompter.EXPECT().PromptForLocalPort("RDS", 3306).Return(3306, nil)
		mockRDSAdapter.EXPECT().GenerateAuthToken("test-rds:3306", "test-user", "us-east-1").Return("mock-auth-token", nil)
		mockConnServices.EXPECT().StartPortForwarding(gomock.Any(), 3306, "test-rds", 3306).Return(nil)

		err := svc.HandleTunnelConnection()
		assert.NoError(t, err)
	})

	t.Run("Error_InvalidPort", func(t *testing.T) {
		mockConnServices.EXPECT().IsAWSConfigured().Return(true)
		mockConnPrompter.EXPECT().PromptForConfirmation("Look for RDS instances in AWS?").Return(false, nil)
		mockRPrompter.EXPECT().PromptForManualEndpoint().Return("test-rds:invalid", "test-user", "us-east-1", nil)

		err := svc.HandleTunnelConnection()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid port")
	})

	t.Run("AWSNotConfigured", func(t *testing.T) {
		mockConnServices.EXPECT().IsAWSConfigured().Return(false)
		mockRPrompter.EXPECT().PromptForManualEndpoint().Return("test-rds:3306", "test-user", "us-east-1", nil)
		mockConnPrompter.EXPECT().PromptForLocalPort("RDS", 3306).Return(3306, nil)
		mockRDSAdapter.EXPECT().GenerateAuthToken("test-rds:3306", "test-user", "us-east-1").Return("mock-auth-token", nil)
		mockConnServices.EXPECT().StartPortForwarding(gomock.Any(), 3306, "test-rds", 3306).Return(nil)

		err := svc.HandleTunnelConnection()
		assert.NoError(t, err)
	})
}

func TestRDSService_CleanupSOCKS(t *testing.T) {
	svc, ctrl, _, _, _, _, _ := setupTest(t)
	defer ctrl.Finish()

	t.Run("NoSOCKSPort", func(t *testing.T) {
		err := svc.CleanupSOCKS()
		assert.NoError(t, err)
	})
}

func TestRDSService_HandleSOCKSConnection(t *testing.T) {
	svc, ctrl, mockRPrompter, _, mockConnPrompter, mockConnServices, _ := setupTest(t)
	defer ctrl.Finish()

	t.Run("HappyPath", func(t *testing.T) {
		mockConnServices.EXPECT().IsAWSConfigured().Return(true)
		mockConnPrompter.EXPECT().PromptForConfirmation("Look for RDS instances in AWS?").Return(false, nil)
		mockRPrompter.EXPECT().PromptForManualEndpoint().Return("test-rds:3306", "test-user", "us-east-1", nil)
		mockConnPrompter.EXPECT().PromptForSOCKSProxyPort(1080).Return(1080, nil)
		mockConnServices.EXPECT().StartSOCKSProxy(gomock.Any(), 1080).Return(nil)

		err := svc.HandleSOCKSConnection()
		assert.NoError(t, err)
		assert.Equal(t, 1080, svc.SOCKSPort())
	})

	t.Run("Error_SOCKSProxyFailure", func(t *testing.T) {
		mockConnServices.EXPECT().IsAWSConfigured().Return(true)
		mockConnPrompter.EXPECT().PromptForConfirmation("Look for RDS instances in AWS?").Return(false, nil)
		mockRPrompter.EXPECT().PromptForManualEndpoint().Return("test-rds:3306", "test-user", "us-east-1", nil)
		mockConnPrompter.EXPECT().PromptForSOCKSProxyPort(1080).Return(1080, nil)
		mockConnServices.EXPECT().StartSOCKSProxy(gomock.Any(), 1080).Return(errors.New("SOCKS failure"))

		err := svc.HandleSOCKSConnection()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "SOCKS proxy failed")
	})

	t.Run("AWSNotConfigured", func(t *testing.T) {
		mockConnServices.EXPECT().IsAWSConfigured().Return(false)
		mockRPrompter.EXPECT().PromptForManualEndpoint().Return("test-rds:3306", "test-user", "us-east-1", nil)
		mockConnPrompter.EXPECT().PromptForSOCKSProxyPort(1080).Return(1080, nil)
		mockConnServices.EXPECT().StartSOCKSProxy(gomock.Any(), 1080).Return(nil)

		err := svc.HandleSOCKSConnection()
		assert.NoError(t, err)
		assert.Equal(t, 1080, svc.SOCKSPort())
	})
}

func TestNewRDSService(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConnServices := mock_awsctl.NewMockServicesInterface(ctrl)

	t.Run("BasicInitialization", func(t *testing.T) {
		service := rds.NewRDSService(mockConnServices)

		assert.NotNil(t, service)
		assert.Equal(t, mockConnServices, service.ConnServices)
		assert.NotNil(t, service.RPrompter)
	})

	t.Run("WithOptions", func(t *testing.T) {
		mockRDSClient := mock_rds.NewMockRDSAdapterInterface(ctrl)
		mockPrompter := mock_awsctl.NewMockPrompter(ctrl)

		opts := []func(*rds.RDSService){
			func(s *rds.RDSService) { s.RDSClient = mockRDSClient },
			func(s *rds.RDSService) { s.GPrompter = mockPrompter },
		}

		service := rds.NewRDSService(mockConnServices, opts...)

		assert.Equal(t, mockRDSClient, service.RDSClient)
		assert.Equal(t, mockPrompter, service.GPrompter)
	})
}

func TestRDSService_Run_SwitchCases(t *testing.T) {
	svc, ctrl, mockRPrompter, mockRDSAdapter, mockConnPrompter, mockConnServices, mockGPrompter := setupTest(t)
	defer ctrl.Finish()

	t.Run("ConnectDirect", func(t *testing.T) {
		mockRPrompter.EXPECT().SelectRDSAction().Return(rds.ConnectDirect, nil)
		mockConnServices.EXPECT().IsAWSConfigured().Return(true)
		mockConnPrompter.EXPECT().PromptForConfirmation("Look for RDS instances in AWS?").Return(false, nil)
		mockRPrompter.EXPECT().PromptForManualEndpoint().Return("localhost:5432", "admin", "us-west-2", nil)
		mockRDSAdapter.EXPECT().GenerateAuthToken("localhost:5432", "admin", "us-west-2").Return("mock-auth-token", nil)
		err := svc.Run()
		assert.NoError(t, err)
	})

	t.Run("ConnectViaTunnel", func(t *testing.T) {
		mockRPrompter.EXPECT().SelectRDSAction().Return(rds.ConnectViaTunnel, nil)
		mockConnServices.EXPECT().IsAWSConfigured().Return(true)
		mockConnPrompter.EXPECT().PromptForConfirmation("Look for RDS instances in AWS?").Return(false, nil)
		mockRPrompter.EXPECT().PromptForManualEndpoint().Return("test-rds:3306", "test-user", "us-east-1", nil)
		mockConnPrompter.EXPECT().PromptForLocalPort("RDS", 3306).Return(3306, nil)
		mockRDSAdapter.EXPECT().GenerateAuthToken("test-rds:3306", "test-user", "us-east-1").Return("mock-auth-token", nil)
		mockConnServices.EXPECT().StartPortForwarding(gomock.Any(), 3306, "test-rds", 3306).Return(nil)

		err := svc.Run()
		assert.NoError(t, err)
	})

	t.Run("ConnectViaSOCKS", func(t *testing.T) {
		mockRPrompter.EXPECT().SelectRDSAction().Return(rds.ConnectViaSOCKS, nil)
		mockConnServices.EXPECT().IsAWSConfigured().Return(true)
		mockConnPrompter.EXPECT().PromptForConfirmation("Look for RDS instances in AWS?").Return(false, nil)
		mockRPrompter.EXPECT().PromptForManualEndpoint().Return("test-rds:3306", "test-user", "us-east-1", nil)
		mockConnPrompter.EXPECT().PromptForSOCKSProxyPort(1080).Return(1080, nil)
		mockConnServices.EXPECT().StartSOCKSProxy(gomock.Any(), 1080).Return(nil)
		mockRPrompter.EXPECT().SelectRDSAction().Return(rds.ExitRDS, nil)

		err := svc.Run()
		assert.NoError(t, err)
	})

	t.Run("ConnectViaSOCKS_WithCleanup", func(t *testing.T) {
		mockSSH := mock_awsctl.NewMockSSHExecutorInterface(ctrl)
		mockOS := mock_awsctl.NewMockOSDetector(ctrl)

		svc := &rds.RDSService{
			RPrompter:    mockRPrompter,
			CPrompter:    mockConnPrompter,
			ConnServices: mockConnServices,
			SSHExecutor:  mockSSH,
			OsDetector:   mockOS,
			GPrompter:    mockGPrompter,
		}

		mockRPrompter.EXPECT().SelectRDSAction().Return(rds.ConnectViaSOCKS, nil)
		mockConnServices.EXPECT().IsAWSConfigured().Return(true)
		mockConnPrompter.EXPECT().PromptForConfirmation("Look for RDS instances in AWS?").Return(false, nil)
		mockRPrompter.EXPECT().PromptForManualEndpoint().Return("test-rds:3306", "test-user", "us-east-1", nil)
		mockConnPrompter.EXPECT().PromptForSOCKSProxyPort(1080).Return(1080, nil)
		mockConnServices.EXPECT().StartSOCKSProxy(gomock.Any(), 1080).Return(nil)

		mockRPrompter.EXPECT().SelectRDSAction().Return(rds.ExitRDS, promptUtils.ErrInterrupted)

		mockOS.EXPECT().GetOS().Return("linux")
		mockSSH.EXPECT().Execute(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

		err := svc.Run()
		assert.Equal(t, promptUtils.ErrInterrupted, err)
		assert.Equal(t, 0, svc.SOCKSPort())
	})
	t.Run("ActionSelectionError", func(t *testing.T) {
		mockRPrompter.EXPECT().SelectRDSAction().Return(rds.ConnectDirect, errors.New("some error"))

		err := svc.Run()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "action selection aborted")
	})

}

func TestRDSService_GetConnectionDetails(t *testing.T) {
	tests := []struct {
		name         string
		prepareMocks func(
			*mock_awsctl.MockServicesInterface,
			*mock_awsctl.MockConnectionPrompter,
			*mock_rds.MockRDSPromptInterface,
			*mock_rds.MockRDSAdapterInterface,
			*mock_awsctl.MockPrompter,
		)
		expectEndpoint string
		expectUser     string
		expectRegion   string
		expectError    bool
	}{
		{
			name: "ManualConnection",
			prepareMocks: func(
				cs *mock_awsctl.MockServicesInterface,
				cp *mock_awsctl.MockConnectionPrompter,
				rp *mock_rds.MockRDSPromptInterface,
				rc *mock_rds.MockRDSAdapterInterface,
				gp *mock_awsctl.MockPrompter,
			) {
				cs.EXPECT().IsAWSConfigured().Return(true)
				cp.EXPECT().PromptForConfirmation("Look for RDS instances in AWS?").Return(false, nil)
				rp.EXPECT().PromptForManualEndpoint().Return("test-rds:3306", "test-user", "us-east-1", nil)
			},
			expectEndpoint: "test-rds:3306",
			expectUser:     "test-user",
			expectRegion:   "us-east-1",
			expectError:    false,
		},
		{
			name: "AWSConnection",
			prepareMocks: func(
				cs *mock_awsctl.MockServicesInterface,
				cp *mock_awsctl.MockConnectionPrompter,
				rp *mock_rds.MockRDSPromptInterface,
				rc *mock_rds.MockRDSAdapterInterface,
				gp *mock_awsctl.MockPrompter,
			) {
				cs.EXPECT().IsAWSConfigured().Return(true)
				cp.EXPECT().PromptForConfirmation("Look for RDS instances in AWS?").Return(true, nil)
				cp.EXPECT().PromptForRegion("").Return("us-east-1", nil)
				rp.EXPECT().PromptForProfile().Return("default", nil)
				rc.EXPECT().ListRDSResources(gomock.Any()).Return([]models.RDSInstance{{DBInstanceIdentifier: "test-rds"}}, nil)
				rp.EXPECT().PromptForRDSInstance([]models.RDSInstance{{DBInstanceIdentifier: "test-rds"}}).Return("test-rds", nil)
				rc.EXPECT().GetConnectionEndpoint(gomock.Any(), "test-rds").Return("test-rds:3306", nil)
				gp.EXPECT().PromptForInput("Enter database username:", "").Return("test-user", nil)
			},
			expectEndpoint: "test-rds:3306",
			expectUser:     "test-user",
			expectRegion:   "us-east-1",
			expectError:    false,
		},
		{
			name: "AWSNotConfigured",
			prepareMocks: func(
				cs *mock_awsctl.MockServicesInterface,
				cp *mock_awsctl.MockConnectionPrompter,
				rp *mock_rds.MockRDSPromptInterface,
				rc *mock_rds.MockRDSAdapterInterface,
				gp *mock_awsctl.MockPrompter,
			) {
				cs.EXPECT().IsAWSConfigured().Return(false)
				rp.EXPECT().PromptForManualEndpoint().Return("test-rds:3306", "test-user", "us-east-1", nil)
			},
			expectEndpoint: "test-rds:3306",
			expectUser:     "test-user",
			expectRegion:   "us-east-1",
			expectError:    false,
		},
		{
			name: "ProfilePromptError",
			prepareMocks: func(
				cs *mock_awsctl.MockServicesInterface,
				cp *mock_awsctl.MockConnectionPrompter,
				rp *mock_rds.MockRDSPromptInterface,
				rc *mock_rds.MockRDSAdapterInterface,
				gp *mock_awsctl.MockPrompter,
			) {
				cs.EXPECT().IsAWSConfigured().Return(true)
				cp.EXPECT().PromptForConfirmation("Look for RDS instances in AWS?").Return(true, nil)
				cp.EXPECT().PromptForRegion("").Return("us-east-1", nil)
				rp.EXPECT().PromptForProfile().Return("", errors.New("profile error"))
				rp.EXPECT().PromptForManualEndpoint().Return("test-rds:3306", "test-user", "us-east-1", nil)
			},
			expectEndpoint: "test-rds:3306",
			expectUser:     "test-user",
			expectRegion:   "us-east-1",
			expectError:    false,
		},
		{
			name: "AWSConfigError",
			prepareMocks: func(
				cs *mock_awsctl.MockServicesInterface,
				cp *mock_awsctl.MockConnectionPrompter,
				rp *mock_rds.MockRDSPromptInterface,
				rc *mock_rds.MockRDSAdapterInterface,
				gp *mock_awsctl.MockPrompter,
			) {
				cs.EXPECT().IsAWSConfigured().Return(true)
				cp.EXPECT().PromptForConfirmation("Look for RDS instances in AWS?").Return(true, nil)
				cp.EXPECT().PromptForRegion("").Return("us-east-1", nil)
				rp.EXPECT().PromptForProfile().Return("invalid-profile", nil)
				rp.EXPECT().PromptForManualEndpoint().Return("test-rds:3306", "test-user", "us-east-1", nil)
			},
			expectEndpoint: "test-rds:3306",
			expectUser:     "test-user",
			expectRegion:   "us-east-1",
			expectError:    false,
		},
		{
			name: "NoRDSResources",
			prepareMocks: func(
				cs *mock_awsctl.MockServicesInterface,
				cp *mock_awsctl.MockConnectionPrompter,
				rp *mock_rds.MockRDSPromptInterface,
				rc *mock_rds.MockRDSAdapterInterface,
				gp *mock_awsctl.MockPrompter,
			) {
				cs.EXPECT().IsAWSConfigured().Return(true)
				cp.EXPECT().PromptForConfirmation("Look for RDS instances in AWS?").Return(true, nil)
				cp.EXPECT().PromptForRegion("").Return("us-east-1", nil)
				rp.EXPECT().PromptForProfile().Return("default", nil)
				rc.EXPECT().ListRDSResources(gomock.Any()).Return([]models.RDSInstance{}, nil)
				rp.EXPECT().PromptForManualEndpoint().Return("test-rds:3306", "test-user", "us-east-1", nil)
			},
			expectEndpoint: "test-rds:3306",
			expectUser:     "test-user",
			expectRegion:   "us-east-1",
			expectError:    false,
		},
		{
			name: "RDSResourcesError",
			prepareMocks: func(
				cs *mock_awsctl.MockServicesInterface,
				cp *mock_awsctl.MockConnectionPrompter,
				rp *mock_rds.MockRDSPromptInterface,
				rc *mock_rds.MockRDSAdapterInterface,
				gp *mock_awsctl.MockPrompter,
			) {
				cs.EXPECT().IsAWSConfigured().Return(true)
				cp.EXPECT().PromptForConfirmation("Look for RDS instances in AWS?").Return(true, nil)
				cp.EXPECT().PromptForRegion("").Return("us-east-1", nil)
				rp.EXPECT().PromptForProfile().Return("default", nil)
				rc.EXPECT().ListRDSResources(gomock.Any()).Return(nil, errors.New("AWS error"))
				rp.EXPECT().PromptForManualEndpoint().Return("test-rds:3306", "test-user", "us-east-1", nil)
			},
			expectEndpoint: "test-rds:3306",
			expectUser:     "test-user",
			expectRegion:   "us-east-1",
			expectError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockConnServices := mock_awsctl.NewMockServicesInterface(ctrl)
			mockConnPrompter := mock_awsctl.NewMockConnectionPrompter(ctrl)
			mockRPrompter := mock_rds.NewMockRDSPromptInterface(ctrl)
			mockRDSClient := mock_rds.NewMockRDSAdapterInterface(ctrl)
			mockGPrompter := mock_awsctl.NewMockPrompter(ctrl)

			svc := &rds.RDSService{
				RPrompter:    mockRPrompter,
				RDSClient:    mockRDSClient,
				CPrompter:    mockConnPrompter,
				ConnServices: mockConnServices,
				GPrompter:    mockGPrompter,
			}

			tt.prepareMocks(mockConnServices, mockConnPrompter, mockRPrompter, mockRDSClient, mockGPrompter)

			endpoint, dbUser, region, err := svc.GetConnectionDetails()

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectEndpoint, endpoint)
				assert.Equal(t, tt.expectUser, dbUser)
				assert.Equal(t, tt.expectRegion, region)
			}
		})
	}
}

func TestRDSService_Run_ErrorCases(t *testing.T) {
	svc, ctrl, mockRPrompter, _, _, mockConnServices, _ := setupTest(t)
	defer ctrl.Finish()

	t.Run("HandleDirectConnectionError", func(t *testing.T) {
		mockConnServices.EXPECT().IsAWSConfigured().Return(false)
		mockRPrompter.EXPECT().SelectRDSAction().Return(rds.ConnectDirect, nil)
		mockRPrompter.EXPECT().PromptForManualEndpoint().Return("", "", "", errors.New("manual endpoint error"))

		err := svc.Run()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "direct connection failed")
	})

	t.Run("HandleTunnelConnectionError", func(t *testing.T) {
		mockConnServices.EXPECT().IsAWSConfigured().Return(false)
		mockRPrompter.EXPECT().SelectRDSAction().Return(rds.ConnectViaTunnel, nil)
		mockRPrompter.EXPECT().PromptForManualEndpoint().Return("invalid-endpoint", "", "", nil)

		err := svc.Run()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "tunnel connection failed")
	})

	t.Run("HandleSOCKSConnectionError", func(t *testing.T) {
		mockConnServices.EXPECT().IsAWSConfigured().Return(false)
		mockRPrompter.EXPECT().SelectRDSAction().Return(rds.ConnectViaSOCKS, nil)
		mockRPrompter.EXPECT().PromptForManualEndpoint().Return("invalid-endpoint", "", "", nil)

		err := svc.Run()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "SOCKS connection failed")
	})
}

func TestHandleTunnelConnection_ErrorCases(t *testing.T) {
	svc, ctrl, mockRPrompter, mockRDSAdapter, mockConnPrompter, mockConnServices, _ := setupTest(t)
	defer ctrl.Finish()

	t.Run("InvalidEndpointFormat", func(t *testing.T) {
		mockConnServices.EXPECT().IsAWSConfigured().Return(true)
		mockConnPrompter.EXPECT().PromptForConfirmation("Look for RDS instances in AWS?").Return(false, nil)
		mockRPrompter.EXPECT().PromptForManualEndpoint().Return("invalid-endpoint", "user", "region", nil)

		err := svc.HandleTunnelConnection()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid RDS endpoint format")
	})

	t.Run("InvalidPort", func(t *testing.T) {
		mockConnServices.EXPECT().IsAWSConfigured().Return(true)
		mockConnPrompter.EXPECT().PromptForConfirmation("Look for RDS instances in AWS?").Return(false, nil)
		mockRPrompter.EXPECT().PromptForManualEndpoint().Return("host:invalid", "user", "region", nil)

		err := svc.HandleTunnelConnection()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid port in RDS endpoint")
	})

	t.Run("LocalPortPromptError", func(t *testing.T) {
		mockConnServices.EXPECT().IsAWSConfigured().Return(true)
		mockConnPrompter.EXPECT().PromptForConfirmation("Look for RDS instances in AWS?").Return(false, nil)
		mockRPrompter.EXPECT().PromptForManualEndpoint().Return("host:3306", "user", "region", nil)
		mockConnPrompter.EXPECT().PromptForLocalPort("RDS", 3306).Return(0, errors.New("port error"))

		err := svc.HandleTunnelConnection()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get local port")
	})

	t.Run("AuthTokenError", func(t *testing.T) {
		mockConnServices.EXPECT().IsAWSConfigured().Return(true)
		mockConnPrompter.EXPECT().PromptForConfirmation("Look for RDS instances in AWS?").Return(false, nil)
		mockRPrompter.EXPECT().PromptForManualEndpoint().Return("host:3306", "user", "region", nil)
		mockConnPrompter.EXPECT().PromptForLocalPort("RDS", 3306).Return(3306, nil)
		mockRDSAdapter.EXPECT().GenerateAuthToken("host:3306", "user", "region").Return("", errors.New("auth error"))

		err := svc.HandleTunnelConnection()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to generate RDS auth token")
	})
}

func TestGetRDSConnectionDetails_ErrorCases(t *testing.T) {
	svc, ctrl, mockRPrompter, _, mockConnPrompter, mockConnServices, _ := setupTest(t)
	defer ctrl.Finish()

	t.Run("RegionPromptError", func(t *testing.T) {
		mockConnServices.EXPECT().IsAWSConfigured().Return(true)
		mockConnPrompter.EXPECT().PromptForConfirmation("Look for RDS instances in AWS?").Return(true, nil)
		mockConnPrompter.EXPECT().PromptForRegion("").Return("", errors.New("region error"))
		mockRPrompter.EXPECT().PromptForManualEndpoint().Return("host:3306", "user", "region", nil)

		_, _, _, err := svc.GetConnectionDetails()
		assert.NoError(t, err)
	})

}

func TestHandleDirectConnection_AuthTokenError(t *testing.T) {
	svc, ctrl, mockRPrompter, mockRDSAdapter, mockConnPrompter, mockConnServices, _ := setupTest(t)
	defer ctrl.Finish()

	t.Run("GenerateAuthTokenError", func(t *testing.T) {
		mockConnServices.EXPECT().IsAWSConfigured().Return(true)
		mockConnPrompter.EXPECT().PromptForConfirmation("Look for RDS instances in AWS?").Return(false, nil)
		mockRPrompter.EXPECT().PromptForManualEndpoint().Return("localhost:5432", "admin", "us-west-2", nil)

		mockRDSAdapter.EXPECT().GenerateAuthToken("localhost:5432", "admin", "us-west-2").
			Return("", errors.New("token generation failed"))

		err := svc.HandleDirectConnection()

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to generate RDS auth token")
		assert.Contains(t, err.Error(), "token generation failed")
	})
}
