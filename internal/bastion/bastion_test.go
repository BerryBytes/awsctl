package bastion

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"testing"
	"time"

	"path/filepath"

	"github.com/BerryBytes/awsctl/models"
	mock_awsctl "github.com/BerryBytes/awsctl/tests/mock"
	"github.com/BerryBytes/awsctl/utils/common"
	promptUtils "github.com/BerryBytes/awsctl/utils/prompt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockFileInfo struct {
	mode os.FileMode
}

func (m mockFileInfo) Name() string       { return "testfile" }
func (m mockFileInfo) Size() int64        { return 0 }
func (m mockFileInfo) Mode() os.FileMode  { return m.mode }
func (m mockFileInfo) ModTime() time.Time { return time.Time{} }
func (m mockFileInfo) IsDir() bool        { return false }
func (m mockFileInfo) Sys() interface{}   { return nil }

func TestBastionService_Run_SSHFlow(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPrompter := mock_awsctl.NewMockBastionPrompterInterface(ctrl)
	mockFS := mock_awsctl.NewMockFileSystemInterface(ctrl)
	mockSSH := mock_awsctl.NewMockSSHExecutorInterface(ctrl)

	mockPrompter.EXPECT().SelectAction().Return(SSHIntoBastion, nil)
	mockPrompter.EXPECT().PromptForConfirmation("Look for bastion hosts in AWS?").Return(false, nil)
	mockPrompter.EXPECT().PromptForBastionHost().Return("test.host", nil)
	mockPrompter.EXPECT().PromptForSSHUser("ubuntu").Return("testuser", nil)
	mockPrompter.EXPECT().PromptForSSHKeyPath("~/.ssh/id_ed25519").Return("/test/key", nil)

	mockFS.EXPECT().Stat("/test/key").Return(&mockFileInfo{mode: 0600}, nil)
	mockFS.EXPECT().ReadFile("/test/key").Return([]byte("-----BEGIN OPENSSH PRIVATE KEY-----\nfake-private-key-content\n-----END OPENSSH PRIVATE KEY-----"), nil)

	mockSSH.EXPECT().Execute(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

	service := &BastionService{
		BPrompter:     mockPrompter,
		Fs:            mockFS,
		SSHExecutor:   mockSSH,
		AwsConfigured: true,
	}

	err := service.Run()
	assert.NoError(t, err)
}

func TestBastionService_Run_SOCKSFlow(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPrompter := mock_awsctl.NewMockBastionPrompterInterface(ctrl)
	mockFS := mock_awsctl.NewMockFileSystemInterface(ctrl)
	mockSSH := mock_awsctl.NewMockSSHExecutorInterface(ctrl)

	mockPrompter.EXPECT().SelectAction().Return(StartSOCKSProxy, nil).Times(1)
	mockPrompter.EXPECT().SelectAction().Return(ExitBastion, nil).Times(1)

	mockPrompter.EXPECT().PromptForSOCKSProxyPort(9999).Return(9999, nil)
	mockPrompter.EXPECT().PromptForConfirmation("Look for bastion hosts in AWS?").Return(false, nil)
	mockPrompter.EXPECT().PromptForBastionHost().Return("test.host", nil)
	mockPrompter.EXPECT().PromptForSSHUser("ubuntu").Return("testuser", nil)
	mockPrompter.EXPECT().PromptForSSHKeyPath("~/.ssh/id_ed25519").Return("/test/key", nil)

	mockFS.EXPECT().Stat("/test/key").Return(&mockFileInfo{mode: 0600}, nil)
	mockFS.EXPECT().ReadFile("/test/key").Return([]byte("-----BEGIN OPENSSH PRIVATE KEY-----\nfake-private-key-content\n-----END OPENSSH PRIVATE KEY-----"), nil)

	mockSSH.EXPECT().Execute(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

	service := &BastionService{
		BPrompter:     mockPrompter,
		Fs:            mockFS,
		SSHExecutor:   mockSSH,
		AwsConfigured: true,
	}

	err := service.Run()
	assert.NoError(t, err)
}

func TestBastionService_Run_PortForwardFlow(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPrompter := mock_awsctl.NewMockBastionPrompterInterface(ctrl)
	mockFS := mock_awsctl.NewMockFileSystemInterface(ctrl)
	mockSSH := mock_awsctl.NewMockSSHExecutorInterface(ctrl)

	// Setup expectations
	mockPrompter.EXPECT().SelectAction().Return(PortForwarding, nil)
	mockPrompter.EXPECT().SelectAction().Return(ExitBastion, nil).Times(1)
	mockPrompter.EXPECT().PromptForLocalPort("port forwarding", 3500).Return(3500, nil)
	mockPrompter.EXPECT().PromptForRemoteHost().Return("remote.host", nil)
	mockPrompter.EXPECT().PromptForRemotePort("destination").Return(5432, nil)
	mockPrompter.EXPECT().PromptForConfirmation("Look for bastion hosts in AWS?").Return(false, nil)
	mockPrompter.EXPECT().PromptForBastionHost().Return("test.host", nil)
	mockPrompter.EXPECT().PromptForSSHUser("ubuntu").Return("testuser", nil)
	mockPrompter.EXPECT().PromptForSSHKeyPath("~/.ssh/id_ed25519").Return("/test/key", nil)

	tmpDir, err := os.MkdirTemp("", "ssh_test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	keyPath := filepath.Join(tmpDir, "test_key")
	keyContent := []byte("-----BEGIN OPENSSH PRIVATE KEY-----\nfake-private-key-content\n-----END OPENSSH PRIVATE KEY-----") // gitleaks:allow
	err = os.WriteFile(keyPath, keyContent, 0600)
	require.NoError(t, err)

	// Mock file system interactions with the temporary file
	mockFS.EXPECT().Stat("/test/key").Return(&mockFileInfo{mode: 0600}, nil)
	mockFS.EXPECT().ReadFile("/test/key").Return(keyContent, nil)

	mockSSH.EXPECT().Execute(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

	service := &BastionService{
		BPrompter:     mockPrompter,
		Fs:            mockFS,
		SSHExecutor:   mockSSH,
		AwsConfigured: true,
	}

	err = service.Run()
	assert.NoError(t, err)
}

func TestBastionService_getConnectionDetails_KeyPathExpansion(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPrompter := mock_awsctl.NewMockBastionPrompterInterface(ctrl)
	mockFS := mock_awsctl.NewMockFileSystemInterface(ctrl)

	mockPrompter.EXPECT().PromptForBastionHost().Return("test.host", nil)
	mockPrompter.EXPECT().PromptForSSHUser("ubuntu").Return("testuser", nil)
	mockPrompter.EXPECT().PromptForSSHKeyPath("~/.ssh/id_ed25519").Return("~/testkey", nil)

	mockFS.EXPECT().Stat(gomock.Any()).DoAndReturn(func(path string) (os.FileInfo, error) {
		assert.NotContains(t, path, "~", "tilde should have been expanded")
		return &mockFileInfo{mode: 0600}, nil
	})
	mockFS.EXPECT().ReadFile(gomock.Any()).Return([]byte("-----BEGIN PRIVATE KEY-----\n..."), nil)

	service := &BastionService{
		BPrompter:     mockPrompter,
		Fs:            mockFS,
		AwsConfigured: false,
	}

	_, _, _, err := service.getConnectionDetails()
	assert.NoError(t, err)
}

func TestBastionService_HandlePortForward_ErrorCases(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tests := []struct {
		name          string
		setupMocks    func(*mock_awsctl.MockBastionPrompterInterface, *mock_awsctl.MockFileSystemInterface)
		expectedError string
	}{
		{
			name: "local port prompt error",
			setupMocks: func(p *mock_awsctl.MockBastionPrompterInterface, _ *mock_awsctl.MockFileSystemInterface) {
				p.EXPECT().PromptForLocalPort("port forwarding", 3500).Return(0, fmt.Errorf("port error"))
			},
			expectedError: "port error",
		},
		{
			name: "remote host prompt error",
			setupMocks: func(p *mock_awsctl.MockBastionPrompterInterface, _ *mock_awsctl.MockFileSystemInterface) {
				p.EXPECT().PromptForLocalPort("port forwarding", 3500).Return(3500, nil)
				p.EXPECT().PromptForRemoteHost().Return("", fmt.Errorf("host error"))
			},
			expectedError: "host error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockPrompter := mock_awsctl.NewMockBastionPrompterInterface(ctrl)
			mockFS := mock_awsctl.NewMockFileSystemInterface(ctrl)

			tt.setupMocks(mockPrompter, mockFS)

			service := &BastionService{
				BPrompter:     mockPrompter,
				Fs:            mockFS,
				AwsConfigured: false,
			}

			err := service.HandlePortForward()
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedError)
		})
	}
}

func TestBastionService_HandleSSH(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPrompter := mock_awsctl.NewMockBastionPrompterInterface(ctrl)
	mockSSH := mock_awsctl.NewMockSSHExecutorInterface(ctrl)
	mockFS := mock_awsctl.NewMockFileSystemInterface(ctrl)

	mockPrompter.EXPECT().PromptForBastionHost().Return("bastion.example.com", nil)
	mockPrompter.EXPECT().PromptForSSHUser(gomock.Any()).Return("ubuntu", nil)
	mockPrompter.EXPECT().PromptForSSHKeyPath(gomock.Any()).Return("~/.ssh/id_ed25519", nil)

	mockFS.EXPECT().Stat(gomock.Any()).Return(&mockFileInfo{mode: 0600}, nil)
	mockFS.EXPECT().ReadFile(gomock.Any()).Return([]byte("-----BEGIN PRIVATE KEY-----\n..."), nil)

	mockSSH.EXPECT().Execute(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)

	mockService := &BastionService{
		BPrompter:     mockPrompter,
		SSHExecutor:   mockSSH,
		Fs:            mockFS,
		AwsConfigured: false,
	}

	err := mockService.HandleSSH()
	assert.NoError(t, err, "Expected SSH connection to succeed")
}

func TestBastionService_Run_ExitBastion(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPrompter := mock_awsctl.NewMockBastionPrompterInterface(ctrl)
	mockFS := mock_awsctl.NewMockFileSystemInterface(ctrl)
	mockSSH := mock_awsctl.NewMockSSHExecutorInterface(ctrl)

	mockPrompter.EXPECT().SelectAction().Return(ExitBastion, nil)

	service := &BastionService{
		BPrompter:   mockPrompter,
		Fs:          mockFS,
		SSHExecutor: mockSSH,
	}

	err := service.Run()
	assert.NoError(t, err)
}

func TestBastionService_GetBastionHost(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	testInstance := models.EC2Instance{
		InstanceID:      "i-123",
		PublicIPAddress: "1.2.3.4",
		Name:            "test-bastion",
	}

	tests := []struct {
		name          string
		awsConfigured bool
		setupMocks    func(*mock_awsctl.MockBastionPrompterInterface, *mock_awsctl.MockEC2ClientInterface)
		expectedHost  string
		expectedError error
	}{
		{
			name:          "AWS not configured",
			awsConfigured: false,
			setupMocks: func(p *mock_awsctl.MockBastionPrompterInterface, _ *mock_awsctl.MockEC2ClientInterface) {
				p.EXPECT().PromptForBastionHost().Return("manual.host", nil)
			},
			expectedHost:  "manual.host",
			expectedError: nil,
		},
		{
			name:          "AWS configured with instance found",
			awsConfigured: true,
			setupMocks: func(p *mock_awsctl.MockBastionPrompterInterface, ec2 *mock_awsctl.MockEC2ClientInterface) {
				p.EXPECT().PromptForConfirmation("Look for bastion hosts in AWS?").Return(true, nil)
				ec2.EXPECT().ListBastionInstances(gomock.Any()).Return([]models.EC2Instance{testInstance}, nil)
				p.EXPECT().PromptForBastionInstance([]models.EC2Instance{testInstance}).Return("1.2.3.4", nil)
			},
			expectedHost:  "1.2.3.4",
			expectedError: nil,
		},
		{
			name:          "AWS configured, confirmation interrupted",
			awsConfigured: true,
			setupMocks: func(p *mock_awsctl.MockBastionPrompterInterface, ec2 *mock_awsctl.MockEC2ClientInterface) {
				p.EXPECT().PromptForConfirmation("Look for bastion hosts in AWS?").Return(false, promptUtils.ErrInterrupted)
			},
			expectedHost:  "",
			expectedError: promptUtils.ErrInterrupted,
		},
		{
			name:          "AWS configured, confirmation error",
			awsConfigured: true,
			setupMocks: func(p *mock_awsctl.MockBastionPrompterInterface, ec2 *mock_awsctl.MockEC2ClientInterface) {
				p.EXPECT().PromptForConfirmation("Look for bastion hosts in AWS?").Return(false, fmt.Errorf("some prompt error"))
			},
			expectedHost:  "",
			expectedError: fmt.Errorf("bastion host selection aborted: some prompt error"),
		},
		{
			name:          "AWS configured, list instances fails",
			awsConfigured: true,
			setupMocks: func(p *mock_awsctl.MockBastionPrompterInterface, ec2 *mock_awsctl.MockEC2ClientInterface) {
				p.EXPECT().PromptForConfirmation("Look for bastion hosts in AWS?").Return(true, nil)
				ec2.EXPECT().ListBastionInstances(gomock.Any()).Return(nil, fmt.Errorf("AWS API error"))
				p.EXPECT().PromptForBastionHost().Return("manual.host", nil)
			},
			expectedHost:  "manual.host",
			expectedError: nil,
		},
		{
			name:          "AWS configured, no instances found",
			awsConfigured: true,
			setupMocks: func(p *mock_awsctl.MockBastionPrompterInterface, ec2 *mock_awsctl.MockEC2ClientInterface) {
				p.EXPECT().PromptForConfirmation("Look for bastion hosts in AWS?").Return(true, nil)
				ec2.EXPECT().ListBastionInstances(gomock.Any()).Return([]models.EC2Instance{}, nil)
				p.EXPECT().PromptForBastionHost().Return("manual.host", nil)
			},
			expectedHost:  "manual.host",
			expectedError: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockPrompter := mock_awsctl.NewMockBastionPrompterInterface(ctrl)
			mockEC2 := mock_awsctl.NewMockEC2ClientInterface(ctrl)

			if tt.setupMocks != nil {
				tt.setupMocks(mockPrompter, mockEC2)
			}

			service := &BastionService{
				BPrompter:     mockPrompter,
				EC2Client:     mockEC2,
				AwsConfigured: tt.awsConfigured,
			}

			host, err := service.GetBastionHost()

			if tt.expectedError == nil {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedHost, host)
			} else {
				assert.EqualError(t, err, tt.expectedError.Error())
				assert.Equal(t, tt.expectedHost, host)
			}
		})
	}
}

func TestBastionService_Run_SelectActionError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPrompter := mock_awsctl.NewMockBastionPrompterInterface(ctrl)
	mockFS := mock_awsctl.NewMockFileSystemInterface(ctrl)
	mockSSH := mock_awsctl.NewMockSSHExecutorInterface(ctrl)

	mockPrompter.EXPECT().SelectAction().Return("", fmt.Errorf("some error"))

	service := &BastionService{
		BPrompter:   mockPrompter,
		Fs:          mockFS,
		SSHExecutor: mockSSH,
	}

	err := service.Run()
	assert.Error(t, err)
	assert.Equal(t, "profile selection aborted: some error", err.Error())
}

func TestBastionService_Run_SSHExecuteError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPrompter := mock_awsctl.NewMockBastionPrompterInterface(ctrl)
	mockFS := mock_awsctl.NewMockFileSystemInterface(ctrl)
	mockSSH := mock_awsctl.NewMockSSHExecutorInterface(ctrl)

	mockPrompter.EXPECT().SelectAction().Return(SSHIntoBastion, nil)
	mockPrompter.EXPECT().PromptForBastionHost().Return("test.host", nil)
	mockPrompter.EXPECT().PromptForSSHUser("ubuntu").Return("testuser", nil)
	mockPrompter.EXPECT().PromptForSSHKeyPath("~/.ssh/id_ed25519").Return("/test/key", nil)

	mockFS.EXPECT().Stat("/test/key").Return(&mockFileInfo{mode: 0600}, nil)
	mockFS.EXPECT().ReadFile("/test/key").Return([]byte("-----BEGIN OPENSSH PRIVATE KEY-----\nfake-private-key-content\n-----END OPENSSH PRIVATE KEY-----"), nil)

	mockSSH.EXPECT().Execute(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(fmt.Errorf("ssh execution failed"))

	service := &BastionService{
		BPrompter:   mockPrompter,
		Fs:          mockFS,
		SSHExecutor: mockSSH,
	}

	err := service.Run()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "SSH into bastion failed")
}

func TestValidateSSHKey_InvalidContent(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockFS := mock_awsctl.NewMockFileSystemInterface(ctrl)

	mockFS.EXPECT().Stat("/invalid/key").Return(&mockFileInfo{mode: 0600}, nil)
	mockFS.EXPECT().ReadFile("/invalid/key").Return([]byte("INVALID KEY CONTENT"), nil)

	err := common.ValidateSSHKey(mockFS, "/invalid/key")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "file does not appear to be a valid SSH private key")
}

func TestBastionService_getConnectionDetails_KeyPathExpansionError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPrompter := mock_awsctl.NewMockBastionPrompterInterface(ctrl)
	mockFS := mock_awsctl.NewMockFileSystemInterface(ctrl)

	mockPrompter.EXPECT().PromptForBastionHost().Return("test.host", nil)
	mockPrompter.EXPECT().PromptForSSHUser("ubuntu").Return("testuser", nil)
	mockPrompter.EXPECT().PromptForSSHKeyPath("~/.ssh/id_ed25519").Return("~/testkey", nil)

	mockFS.EXPECT().Stat(gomock.Any()).DoAndReturn(func(path string) (os.FileInfo, error) {
		if strings.Contains(path, "~") {
			return nil, fmt.Errorf("home dir not found")
		}
		return nil, fmt.Errorf("unexpected path")
	})

	service := &BastionService{
		BPrompter:     mockPrompter,
		Fs:            mockFS,
		AwsConfigured: false,
	}

	_, _, _, err := service.getConnectionDetails()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid SSH key: failed to access SSH key file: unexpected path")
}

func TestBastionService_getConnectionDetails_InvalidKeyPermissions(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPrompter := mock_awsctl.NewMockBastionPrompterInterface(ctrl)
	mockFS := mock_awsctl.NewMockFileSystemInterface(ctrl)

	mockPrompter.EXPECT().PromptForBastionHost().Return("test.host", nil)
	mockPrompter.EXPECT().PromptForSSHUser("ubuntu").Return("testuser", nil)
	mockPrompter.EXPECT().PromptForSSHKeyPath("~/.ssh/id_ed25519").Return("/test/key", nil)

	mockFS.EXPECT().Stat("/test/key").Return(&mockFileInfo{mode: 0644}, nil)

	service := &BastionService{
		BPrompter:     mockPrompter,
		Fs:            mockFS,
		AwsConfigured: false,
	}

	_, _, _, err := service.getConnectionDetails()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid SSH key: insecure SSH key permissions 0644 (should be 600 or 400)")
}

func TestBastionService_HandleSOCKS_InvalidPort(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPrompter := mock_awsctl.NewMockBastionPrompterInterface(ctrl)

	mockPrompter.EXPECT().PromptForSOCKSProxyPort(9999).Return(0, fmt.Errorf("invalid port"))

	service := &BastionService{
		BPrompter: mockPrompter,
	}

	err := service.HandleSOCKS()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid port")
}

func TestBastionService_Run_UnknownAction(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPrompter := mock_awsctl.NewMockBastionPrompterInterface(ctrl)

	mockPrompter.EXPECT().SelectAction().Return("unknown-action", nil)
	mockPrompter.EXPECT().SelectAction().Return(ExitBastion, nil)

	service := &BastionService{
		BPrompter: mockPrompter,
	}

	err := service.Run()
	assert.NoError(t, err)
}

func TestNewBastionService_Default(t *testing.T) {
	service := NewBastionService()

	if service == nil {
		t.Error("Expected service instance, got nil")
		return
	}

	if _, ok := service.BPrompter.(*BastionPrompter); !ok {
		t.Error("Expected default BastionPrompter implementation")
	}

	if _, ok := service.Fs.(*common.RealFileSystem); !ok {
		t.Error("Expected default FileSystem implementation")
	}

	if service.AwsConfigured {
		t.Error("Expected AWS to not be configured by default")
	}
}

func TestNewBastionService_WithOptions(t *testing.T) {
	mockPrompter := &mock_awsctl.MockBastionPrompterInterface{}
	mockEC2Client := &mock_awsctl.MockEC2ClientInterface{}

	service := NewBastionService(
		func(s *BastionService) {
			s.BPrompter = mockPrompter
		},
		func(s *BastionService) {
			s.EC2Client = mockEC2Client
			s.AwsConfigured = true
		},
	)

	if service.BPrompter != mockPrompter {
		t.Error("Custom prompter not set")
	}

	if service.EC2Client != mockEC2Client {
		t.Error("Custom EC2Client not set")
	}

	if !service.AwsConfigured {
		t.Error("Expected AWS to be configured")
	}
}

func TestNewBastionService_WithAWSConfig(t *testing.T) {
	service := NewBastionService(WithAWSConfig(context.TODO()))

	if service == nil {
		t.Error("Expected service instance, got nil")
		return
	}

	if _, ok := service.BPrompter.(*BastionPrompter); !ok {
		t.Error("Expected default BastionPrompter implementation")
	}

	if _, ok := service.Fs.(*common.RealFileSystem); !ok {
		t.Error("Expected default FileSystem implementation")
	}

	if service.AwsConfigured {
		t.Log("AWS configured successfully in test environment")
		if service.EC2Client == nil {
			t.Error("Expected EC2Client to be set when AwsConfigured is true")
		}
	} else {
		t.Log("AWS not configured in test environment (no credentials?)")
	}
}

func TestWithAWSConfig_ErrorCases(t *testing.T) {
	t.Run("failed to load AWS config", func(t *testing.T) {
		var buf bytes.Buffer
		log.SetOutput(&buf)
		defer func() {
			log.SetOutput(os.Stderr)
		}()

		mockLoader := func(ctx context.Context, optFns ...func(*config.LoadOptions) error) (aws.Config, error) {
			return aws.Config{}, fmt.Errorf("config error")
		}

		service := NewBastionService(func(s *BastionService) {
			s.configLoader = mockLoader
		}, WithAWSConfig(context.Background()))

		// No AWS config loaded successfully
		assert.False(t, service.AwsConfigured)
		// If no logging happens, we check the service's configuration directly
		// Adjust according to how your service is behaving with errors
	})

	t.Run("AWS region not set", func(t *testing.T) {
		var buf bytes.Buffer
		log.SetOutput(&buf)
		defer func() {
			log.SetOutput(os.Stderr)
		}()

		mockLoader := func(ctx context.Context, optFns ...func(*config.LoadOptions) error) (aws.Config, error) {
			return aws.Config{}, nil
		}

		service := NewBastionService(func(s *BastionService) {
			s.configLoader = mockLoader
		}, WithAWSConfig(context.Background()))

		assert.False(t, service.AwsConfigured)
		// Since the region is empty, the service should not configure successfully
	})

	t.Run("failed to retrieve credentials", func(t *testing.T) {
		var buf bytes.Buffer
		log.SetOutput(&buf)
		defer func() {
			log.SetOutput(os.Stderr)
		}()

		mockLoader := func(ctx context.Context, optFns ...func(*config.LoadOptions) error) (aws.Config, error) {
			return aws.Config{
				Region: "us-east-1",
				Credentials: aws.CredentialsProviderFunc(func(ctx context.Context) (aws.Credentials, error) {
					return aws.Credentials{}, fmt.Errorf("cred error")
				}),
			}, nil
		}

		service := NewBastionService(func(s *BastionService) {
			s.configLoader = mockLoader
		}, WithAWSConfig(context.Background()))

		assert.False(t, service.AwsConfigured)
		// Check if credentials retrieval failed, no EC2Client will be created
		assert.Nil(t, service.EC2Client)
	})

	t.Run("successful configuration", func(t *testing.T) {
		mockLoader := func(ctx context.Context, optFns ...func(*config.LoadOptions) error) (aws.Config, error) {
			return aws.Config{
				Region: "us-east-1",
				Credentials: aws.CredentialsProviderFunc(func(ctx context.Context) (aws.Credentials, error) {
					return aws.Credentials{
						AccessKeyID:     "test",
						SecretAccessKey: "test",
					}, nil
				}),
			}, nil
		}

		service := NewBastionService(func(s *BastionService) {
			s.configLoader = mockLoader
		}, WithAWSConfig(context.Background()))

		assert.True(t, service.AwsConfigured)
		assert.NotNil(t, service.EC2Client)
	})
}

func TestBastionService_Run_ErrorPropagation(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tests := []struct {
		name          string
		action        string
		mockSetup     func(*mock_awsctl.MockBastionPrompterInterface)
		expectedError string
	}{
		{
			name:   "SOCKS proxy setup error",
			action: StartSOCKSProxy,
			mockSetup: func(p *mock_awsctl.MockBastionPrompterInterface) {
				p.EXPECT().PromptForSOCKSProxyPort(9999).Return(0, fmt.Errorf("socks error"))
			},
			expectedError: "SOCKS proxy setup failed: socks error",
		},
		{
			name:   "Port forwarding setup error",
			action: PortForwarding,
			mockSetup: func(p *mock_awsctl.MockBastionPrompterInterface) {
				p.EXPECT().PromptForLocalPort("port forwarding", 3500).Return(0, fmt.Errorf("port error"))
			},
			expectedError: "port forwarding setup failed: port error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockPrompter := mock_awsctl.NewMockBastionPrompterInterface(ctrl)

			mockPrompter.EXPECT().SelectAction().Return(tt.action, nil)
			if tt.mockSetup != nil {
				tt.mockSetup(mockPrompter)
			}

			service := &BastionService{
				BPrompter: mockPrompter,
			}

			err := service.Run()
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedError)
		})
	}
}

func TestBastionService_getConnectionDetails_ErrorCases(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tests := []struct {
		name          string
		mockSetup     func(*mock_awsctl.MockBastionPrompterInterface, *mock_awsctl.MockFileSystemInterface)
		homeDirFunc   func() (string, error)
		expectedError string
	}{
		{
			name: "host prompt error",
			mockSetup: func(p *mock_awsctl.MockBastionPrompterInterface, _ *mock_awsctl.MockFileSystemInterface) {
				p.EXPECT().PromptForBastionHost().Return("", fmt.Errorf("host error"))
			},
			expectedError: "failed to get host",
		},
		{
			name: "user prompt error",
			mockSetup: func(p *mock_awsctl.MockBastionPrompterInterface, _ *mock_awsctl.MockFileSystemInterface) {
				p.EXPECT().PromptForBastionHost().Return("host", nil)
				p.EXPECT().PromptForSSHUser("ubuntu").Return("", fmt.Errorf("user error"))
			},
			expectedError: "failed to get user",
		},
		{
			name: "key path prompt error",
			mockSetup: func(p *mock_awsctl.MockBastionPrompterInterface, _ *mock_awsctl.MockFileSystemInterface) {
				p.EXPECT().PromptForBastionHost().Return("host", nil)
				p.EXPECT().PromptForSSHUser("ubuntu").Return("user", nil)
				p.EXPECT().PromptForSSHKeyPath("~/.ssh/id_ed25519").Return("", fmt.Errorf("key error"))
			},
			expectedError: "failed to get key path",
		},
		{
			name: "home dir error",
			mockSetup: func(p *mock_awsctl.MockBastionPrompterInterface, _ *mock_awsctl.MockFileSystemInterface) {
				p.EXPECT().PromptForBastionHost().Return("host", nil)
				p.EXPECT().PromptForSSHUser("ubuntu").Return("user", nil)
				p.EXPECT().PromptForSSHKeyPath("~/.ssh/id_ed25519").Return("~/key", nil)
			},
			homeDirFunc: func() (string, error) {
				return "", fmt.Errorf("home dir error")
			},
			expectedError: "failed to get home directory",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockPrompter := mock_awsctl.NewMockBastionPrompterInterface(ctrl)
			mockFS := mock_awsctl.NewMockFileSystemInterface(ctrl)

			service := &BastionService{
				BPrompter: mockPrompter,
				Fs:        mockFS,
				homeDir:   tt.homeDirFunc,
			}

			if tt.mockSetup != nil {
				tt.mockSetup(mockPrompter, mockFS)
			}

			_, _, _, err := service.getConnectionDetails()
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedError)
		})
	}
}
