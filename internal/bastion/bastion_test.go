package bastion

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/BerryBytes/awsctl/models"
	mock_awsctl "github.com/BerryBytes/awsctl/tests/mock"
	"github.com/BerryBytes/awsctl/utils/common"
	promptUtils "github.com/BerryBytes/awsctl/utils/prompt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2instanceconnect"
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

func TestSendSSHPublicKey(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-key-*.pub")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	if _, err := tmpFile.WriteString("ssh-rsa AAAAB3NzaC1yc2E... test-key"); err != nil {
		t.Fatalf("failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	tests := []struct {
		name          string
		instanceID    string
		user          string
		publicKeyPath string
		setupMocks    func(
			*mock_awsctl.MockEC2ClientInterface,
			*mock_awsctl.MockEC2InstanceConnectClient,
		)
		wantErr     bool
		expectedErr string
	}{
		{
			name:          "successful key send",
			instanceID:    "i-1234567890abcdef0",
			user:          "ec2-user",
			publicKeyPath: tmpFile.Name(),
			setupMocks: func(mockEC2 *mock_awsctl.MockEC2ClientInterface, mockIC *mock_awsctl.MockEC2InstanceConnectClient) {
				mockEC2.EXPECT().DescribeInstances(gomock.Any(), gomock.Any()).Return(&ec2.DescribeInstancesOutput{
					Reservations: []types.Reservation{
						{
							Instances: []types.Instance{
								{
									InstanceId: aws.String("i-1234567890abcdef0"),
									Placement: &types.Placement{
										AvailabilityZone: aws.String("us-west-2a"),
									},
								},
							},
						},
					},
				}, nil)

				mockIC.EXPECT().SendSSHPublicKey(gomock.Any(), &ec2instanceconnect.SendSSHPublicKeyInput{
					InstanceId:       aws.String("i-1234567890abcdef0"),
					InstanceOSUser:   aws.String("ec2-user"),
					SSHPublicKey:     aws.String("ssh-rsa AAAAB3NzaC1yc2E... test-key"),
					AvailabilityZone: aws.String("us-west-2a"),
				}).Return(&ec2instanceconnect.SendSSHPublicKeyOutput{}, nil)
			},
			wantErr: false,
		},
		{
			name:          "failed to read public key",
			instanceID:    "i-1234567890abcdef0",
			user:          "ec2-user",
			publicKeyPath: "nonexistent-key.pub",
			setupMocks: func(mockEC2 *mock_awsctl.MockEC2ClientInterface, mockIC *mock_awsctl.MockEC2InstanceConnectClient) {
			},
			wantErr:     true,
			expectedErr: "failed to read public key",
		},
		{
			name:          "failed to describe instance",
			instanceID:    "i-1234567890abcdef0",
			user:          "ec2-user",
			publicKeyPath: tmpFile.Name(),
			setupMocks: func(mockEC2 *mock_awsctl.MockEC2ClientInterface, mockIC *mock_awsctl.MockEC2InstanceConnectClient) {
				mockEC2.EXPECT().DescribeInstances(gomock.Any(), gomock.Any()).Return(nil, errors.New("api error"))
			},
			wantErr:     true,
			expectedErr: "failed to describe instance",
		},
		{
			name:          "no instances found",
			instanceID:    "i-1234567890abcdef0",
			user:          "ec2-user",
			publicKeyPath: tmpFile.Name(),
			setupMocks: func(mockEC2 *mock_awsctl.MockEC2ClientInterface, mockIC *mock_awsctl.MockEC2InstanceConnectClient) {
				mockEC2.EXPECT().DescribeInstances(gomock.Any(), gomock.Any()).Return(&ec2.DescribeInstancesOutput{
					Reservations: []types.Reservation{},
				}, nil)
			},
			wantErr:     true,
			expectedErr: "no instance found with ID",
		},
		{
			name:          "no availability zone",
			instanceID:    "i-1234567890abcdef0",
			user:          "ec2-user",
			publicKeyPath: tmpFile.Name(),
			setupMocks: func(mockEC2 *mock_awsctl.MockEC2ClientInterface, mockIC *mock_awsctl.MockEC2InstanceConnectClient) {
				mockEC2.EXPECT().DescribeInstances(gomock.Any(), gomock.Any()).Return(&ec2.DescribeInstancesOutput{
					Reservations: []types.Reservation{
						{
							Instances: []types.Instance{
								{
									InstanceId: aws.String("i-1234567890abcdef0"),
									Placement:  &types.Placement{},
								},
							},
						},
					},
				}, nil)
			},
			wantErr:     true,
			expectedErr: "has no availability zone",
		},
		{
			name:          "failed to send SSH public key",
			instanceID:    "i-1234567890abcdef0",
			user:          "ec2-user",
			publicKeyPath: tmpFile.Name(),
			setupMocks: func(mockEC2 *mock_awsctl.MockEC2ClientInterface, mockIC *mock_awsctl.MockEC2InstanceConnectClient) {
				mockEC2.EXPECT().DescribeInstances(gomock.Any(), gomock.Any()).Return(&ec2.DescribeInstancesOutput{
					Reservations: []types.Reservation{
						{
							Instances: []types.Instance{
								{
									InstanceId: aws.String("i-1234567890abcdef0"),
									Placement: &types.Placement{
										AvailabilityZone: aws.String("us-west-2a"),
									},
								},
							},
						},
					},
				}, nil)

				mockIC.EXPECT().SendSSHPublicKey(gomock.Any(), gomock.Any()).Return(nil, errors.New("api error"))
			},
			wantErr:     true,
			expectedErr: "failed to send SSH public key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockEC2 := mock_awsctl.NewMockEC2ClientInterface(ctrl)
			mockIC := mock_awsctl.NewMockEC2InstanceConnectClient(ctrl)

			if tt.setupMocks != nil {
				tt.setupMocks(mockEC2, mockIC)
			}

			service := &BastionService{
				EC2Client:             mockEC2,
				InstanceConnectClient: mockIC,
				Fs:                    &common.RealFileSystem{},
			}

			err := service.SendSSHPublicKey(context.Background(), tt.instanceID, tt.user, tt.publicKeyPath)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.expectedErr != "" {
					assert.Contains(t, err.Error(), tt.expectedErr)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestBastionService_Run_Flows(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tests := []struct {
		name        string
		action      string
		setupMocks  func(*mock_awsctl.MockBastionPrompterInterface, *mock_awsctl.MockFileSystemInterface, *mock_awsctl.MockSSHExecutorInterface)
		wantErr     bool
		expectedErr string
	}{
		{
			name:   "SSH flow",
			action: SSHIntoBastion,
			setupMocks: func(p *mock_awsctl.MockBastionPrompterInterface, fs *mock_awsctl.MockFileSystemInterface, ssh *mock_awsctl.MockSSHExecutorInterface) {
				p.EXPECT().SelectAction().Return(SSHIntoBastion, nil)
				p.EXPECT().PromptForConfirmation("Look for bastion hosts in AWS?").Return(false, nil)
				p.EXPECT().PromptForBastionHost().Return("test.host", nil)
				p.EXPECT().PromptForSSHUser("ec2-user").Return("testuser", nil)
				p.EXPECT().PromptForSSHKeyPath("~/.ssh/id_ed25519").Return("/test/key", nil)

				fs.EXPECT().Stat("/test/key").Return(&mockFileInfo{mode: 0600}, nil)
				fs.EXPECT().ReadFile("/test/key").Return([]byte("-----BEGIN OPENSSH PRIVATE KEY-----\nfake-private-key-content\n-----END OPENSSH PRIVATE KEY-----"), nil)

				ssh.EXPECT().Execute(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			},
			wantErr: false,
		},
		{
			name:   "SOCKS flow",
			action: StartSOCKSProxy,
			setupMocks: func(p *mock_awsctl.MockBastionPrompterInterface, fs *mock_awsctl.MockFileSystemInterface, ssh *mock_awsctl.MockSSHExecutorInterface) {
				p.EXPECT().SelectAction().Return(StartSOCKSProxy, nil)
				p.EXPECT().SelectAction().Return(ExitBastion, nil)
				p.EXPECT().PromptForSOCKSProxyPort(9999).Return(9999, nil)
				p.EXPECT().PromptForConfirmation("Look for bastion hosts in AWS?").Return(false, nil)
				p.EXPECT().PromptForBastionHost().Return("test.host", nil)
				p.EXPECT().PromptForSSHUser("ec2-user").Return("testuser", nil)
				p.EXPECT().PromptForSSHKeyPath("~/.ssh/id_ed25519").Return("/test/key", nil)

				fs.EXPECT().Stat("/test/key").Return(&mockFileInfo{mode: 0600}, nil)
				fs.EXPECT().ReadFile("/test/key").Return([]byte("-----BEGIN OPENSSH PRIVATE KEY-----\nfake-private-key-content\n-----END OPENSSH PRIVATE KEY-----"), nil)

				expectedCmd := []string{
					"ssh",
					"-i", "/test/key",
					"-o", "BatchMode=yes",
					"-o", "ConnectTimeout=30",
					"-o", "StrictHostKeyChecking=ask",
					"-D", "9999",
					"testuser@test.host",
				}
				ssh.EXPECT().Execute(expectedCmd, gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			},
			wantErr: false,
		},
		{
			name:   "SOCKS flow - error in getConnectionDetails",
			action: StartSOCKSProxy,
			setupMocks: func(p *mock_awsctl.MockBastionPrompterInterface, fs *mock_awsctl.MockFileSystemInterface, ssh *mock_awsctl.MockSSHExecutorInterface) {
				p.EXPECT().SelectAction().Return(StartSOCKSProxy, nil)
				p.EXPECT().PromptForSOCKSProxyPort(9999).Return(9999, nil)
				p.EXPECT().PromptForConfirmation("Look for bastion hosts in AWS?").Return(false, nil)
				p.EXPECT().PromptForBastionHost().Return("", fmt.Errorf("failed to prompt for host"))
			},
			wantErr:     true,
			expectedErr: "failed to prompt for host",
		},
		{
			name:   "SOCKS flow - SSH execution fails",
			action: StartSOCKSProxy,
			setupMocks: func(p *mock_awsctl.MockBastionPrompterInterface, fs *mock_awsctl.MockFileSystemInterface, ssh *mock_awsctl.MockSSHExecutorInterface) {
				p.EXPECT().SelectAction().Return(StartSOCKSProxy, nil)
				p.EXPECT().PromptForSOCKSProxyPort(9999).Return(9999, nil)
				p.EXPECT().PromptForConfirmation("Look for bastion hosts in AWS?").Return(false, nil)
				p.EXPECT().PromptForBastionHost().Return("test.host", nil)
				p.EXPECT().PromptForSSHUser("ec2-user").Return("testuser", nil)
				p.EXPECT().PromptForSSHKeyPath("~/.ssh/id_ed25519").Return("/test/key", nil)

				fs.EXPECT().Stat("/test/key").Return(&mockFileInfo{mode: 0600}, nil)
				fs.EXPECT().ReadFile("/test/key").Return([]byte("-----BEGIN OPENSSH PRIVATE KEY-----\nfake-private-key-content\n-----END OPENSSH PRIVATE KEY-----"), nil)

				expectedCmd := []string{
					"ssh",
					"-i", "/test/key",
					"-o", "BatchMode=yes",
					"-o", "ConnectTimeout=30",
					"-o", "StrictHostKeyChecking=ask",
					"-D", "9999",
					"testuser@test.host",
				}
				ssh.EXPECT().Execute(expectedCmd, gomock.Any(), gomock.Any(), gomock.Any()).Return(fmt.Errorf("SSH failed"))
			},
			wantErr:     true,
			expectedErr: "SOCKS proxy setup failed: failed to start SOCKS proxy: SSH connection failed: SSH failed",
		},
		{
			name:   "Port forwarding flow",
			action: PortForwarding,
			setupMocks: func(p *mock_awsctl.MockBastionPrompterInterface, fs *mock_awsctl.MockFileSystemInterface, ssh *mock_awsctl.MockSSHExecutorInterface) {
				p.EXPECT().SelectAction().Return(PortForwarding, nil)
				p.EXPECT().SelectAction().Return(ExitBastion, nil)
				p.EXPECT().PromptForLocalPort("port forwarding", 3500).Return(3500, nil)
				p.EXPECT().PromptForRemoteHost().Return("remote.host", nil)
				p.EXPECT().PromptForRemotePort("destination").Return(5432, nil)
				p.EXPECT().PromptForConfirmation("Look for bastion hosts in AWS?").Return(false, nil)
				p.EXPECT().PromptForBastionHost().Return("test.host", nil)
				p.EXPECT().PromptForSSHUser("ec2-user").Return("testuser", nil)
				p.EXPECT().PromptForSSHKeyPath("~/.ssh/id_ed25519").Return("/test/key", nil)

				fs.EXPECT().Stat("/test/key").Return(&mockFileInfo{mode: 0600}, nil)
				fs.EXPECT().ReadFile("/test/key").Return([]byte("-----BEGIN OPENSSH PRIVATE KEY-----\nfake-private-key-content\n-----END OPENSSH PRIVATE KEY-----"), nil)

				ssh.EXPECT().Execute(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockPrompter := mock_awsctl.NewMockBastionPrompterInterface(ctrl)
			mockFS := mock_awsctl.NewMockFileSystemInterface(ctrl)
			mockSSH := mock_awsctl.NewMockSSHExecutorInterface(ctrl)

			if tt.setupMocks != nil {
				tt.setupMocks(mockPrompter, mockFS, mockSSH)
			}

			service := &BastionService{
				BPrompter:     mockPrompter,
				Fs:            mockFS,
				SSHExecutor:   mockSSH,
				AwsConfigured: true,
			}

			err := service.Run()

			if tt.wantErr {
				assert.Error(t, err)
				if tt.expectedErr != "" {
					assert.Contains(t, err.Error(), tt.expectedErr)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
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

func TestBastionService_Run(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tests := []struct {
		name                string
		action              string
		expectedError       error
		mockSetup           func(*mock_awsctl.MockBastionPrompterInterface, *mock_awsctl.MockSSHExecutorInterface)
		expectedOutput      string
		sshExecuteReturnErr error
		checkOutput         bool
	}{
		{
			name:          "Interrupted with SOCKS",
			action:        "",
			expectedError: promptUtils.ErrInterrupted,
			mockSetup: func(p *mock_awsctl.MockBastionPrompterInterface, s *mock_awsctl.MockSSHExecutorInterface) {
				p.EXPECT().SelectAction().Return("", promptUtils.ErrInterrupted)
				s.EXPECT().Execute(
					[]string{"sh", "-c", "pkill -f 'ssh.*-D.*9999'"},
					gomock.Any(), nil, nil,
				).Return(nil)
			},
			expectedOutput: "SOCKS proxy on port 9999 terminated.",
		},
		{
			name:          "Interrupted with SOCKS Cleanup Error",
			action:        "",
			expectedError: promptUtils.ErrInterrupted,
			mockSetup: func(p *mock_awsctl.MockBastionPrompterInterface, s *mock_awsctl.MockSSHExecutorInterface) {
				p.EXPECT().SelectAction().Return("", promptUtils.ErrInterrupted)
				s.EXPECT().Execute(
					[]string{"sh", "-c", "pkill -f 'ssh.*-D.*9999'"},
					gomock.Any(), nil, nil,
				).Return(fmt.Errorf("cleanup failed"))
			},
			expectedOutput: "Failed to cleanup SOCKS proxy: failed to terminate SOCKS proxy on port 9999: cleanup failed",
		},
		{
			name:          "Select Action Error",
			action:        "",
			expectedError: fmt.Errorf("action selection aborted: some error"),
			mockSetup: func(p *mock_awsctl.MockBastionPrompterInterface, s *mock_awsctl.MockSSHExecutorInterface) {
				p.EXPECT().SelectAction().Return("", fmt.Errorf("some error"))
			},
			checkOutput: false,
		},

		{
			name:          "Error Propagation for SOCKS proxy setup",
			action:        StartSOCKSProxy,
			expectedError: fmt.Errorf("SOCKS proxy setup failed: socks error"),
			mockSetup: func(p *mock_awsctl.MockBastionPrompterInterface, s *mock_awsctl.MockSSHExecutorInterface) {
				p.EXPECT().SelectAction().Return(StartSOCKSProxy, nil)
				p.EXPECT().PromptForSOCKSProxyPort(9999).Return(0, fmt.Errorf("socks error"))
			},
			checkOutput: false,
		},
		{
			name:          "Error Propagation for Port forwarding setup",
			action:        PortForwarding,
			expectedError: fmt.Errorf("port forwarding setup failed: port error"),
			mockSetup: func(p *mock_awsctl.MockBastionPrompterInterface, s *mock_awsctl.MockSSHExecutorInterface) {
				p.EXPECT().SelectAction().Return(PortForwarding, nil)
				p.EXPECT().PromptForLocalPort("port forwarding", 3500).Return(0, fmt.Errorf("port error"))
			},
			checkOutput: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockPrompter := mock_awsctl.NewMockBastionPrompterInterface(ctrl)
			mockSSH := mock_awsctl.NewMockSSHExecutorInterface(ctrl)

			tt.mockSetup(mockPrompter, mockSSH)

			service := &BastionService{
				BPrompter:   mockPrompter,
				SSHExecutor: mockSSH,
				socksPort:   9999,
			}

			var buf bytes.Buffer
			var oldStdout *os.File
			if tt.checkOutput {
				oldStdout = os.Stdout
				r, w, _ := os.Pipe()
				os.Stdout = w
				defer func() {
					w.Close()
					os.Stdout = oldStdout
					if _, err := io.Copy(&buf, r); err != nil {
						t.Logf("failed to copy stdout: %v", err)
					}
					r.Close()
				}()
			}

			err := service.Run()

			if tt.expectedError != nil {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError.Error())
			} else {
				assert.NoError(t, err)
			}

			if tt.checkOutput && tt.expectedOutput != "" {
				assert.Contains(t, buf.String(), tt.expectedOutput)
			}
		})
	}
}

func TestBastionService_Run_ExitWithSOCKSCleanupError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPrompter := mock_awsctl.NewMockBastionPrompterInterface(ctrl)
	mockSSH := mock_awsctl.NewMockSSHExecutorInterface(ctrl)

	mockPrompter.EXPECT().SelectAction().Return(ExitBastion, nil)

	mockSSH.EXPECT().Execute(
		[]string{"sh", "-c", "pkill -f 'ssh.*-D.*9999'"},
		gomock.Any(), nil, nil,
	).Return(fmt.Errorf("cleanup failed"))

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	defer func() {
		w.Close()
		os.Stdout = oldStdout
	}()

	service := &BastionService{
		BPrompter:   mockPrompter,
		SSHExecutor: mockSSH,
		socksPort:   9999,
	}

	err := service.Run()

	w.Close()
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("failed to copy stdout: %v", err)
	}

	assert.NoError(t, err)
	assert.Contains(t, buf.String(), "Failed to cleanup SOCKS proxy: failed to terminate SOCKS proxy on port 9999: cleanup failed")
}

func TestBastionService_Run_SSHExecuteError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPrompter := mock_awsctl.NewMockBastionPrompterInterface(ctrl)
	mockFS := mock_awsctl.NewMockFileSystemInterface(ctrl)
	mockSSH := mock_awsctl.NewMockSSHExecutorInterface(ctrl)

	mockPrompter.EXPECT().SelectAction().Return(SSHIntoBastion, nil)
	mockPrompter.EXPECT().PromptForBastionHost().Return("test.host", nil)
	mockPrompter.EXPECT().PromptForSSHUser("ec2-user").Return("testuser", nil)
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

func TestBastionService_getConnectionDetails_GeneralCases(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tests := []struct {
		name          string
		mockSetup     func(p *mock_awsctl.MockBastionPrompterInterface, fs *mock_awsctl.MockFileSystemInterface)
		homeDirFunc   func() (string, error)
		expectedError string
	}{
		{
			name: "Key path with tilde that fails to expand",
			mockSetup: func(p *mock_awsctl.MockBastionPrompterInterface, fs *mock_awsctl.MockFileSystemInterface) {
				p.EXPECT().PromptForBastionHost().Return("test.host", nil)
				p.EXPECT().PromptForSSHUser("ec2-user").Return("testuser", nil)
				p.EXPECT().PromptForSSHKeyPath("~/.ssh/id_ed25519").Return("~/testkey", nil)

				fs.EXPECT().Stat(gomock.Any()).DoAndReturn(func(path string) (os.FileInfo, error) {
					if strings.Contains(path, "~") {
						return nil, fmt.Errorf("home dir not found")
					}
					return nil, fmt.Errorf("unexpected path")
				})
			},
			expectedError: "invalid SSH key: failed to access SSH key file: unexpected path",
		},
		{
			name: "Invalid key file permissions",
			mockSetup: func(p *mock_awsctl.MockBastionPrompterInterface, fs *mock_awsctl.MockFileSystemInterface) {
				p.EXPECT().PromptForBastionHost().Return("test.host", nil)
				p.EXPECT().PromptForSSHUser("ec2-user").Return("testuser", nil)
				p.EXPECT().PromptForSSHKeyPath("~/.ssh/id_ed25519").Return("/test/key", nil)

				fs.EXPECT().Stat("/test/key").Return(&mockFileInfo{mode: 0644}, nil)
			},
			expectedError: "invalid SSH key: insecure SSH key permissions 0644",
		},
		{
			name: "Key path with tilde expansion works",
			mockSetup: func(p *mock_awsctl.MockBastionPrompterInterface, fs *mock_awsctl.MockFileSystemInterface) {
				p.EXPECT().PromptForBastionHost().Return("test.host", nil)
				p.EXPECT().PromptForSSHUser("ec2-user").Return("testuser", nil)
				p.EXPECT().PromptForSSHKeyPath("~/.ssh/id_ed25519").Return("~/testkey", nil)

				fs.EXPECT().Stat(gomock.Any()).DoAndReturn(func(path string) (os.FileInfo, error) {
					assert.NotContains(t, path, "~", "tilde should have been expanded")
					return &mockFileInfo{mode: 0600}, nil
				})
				fs.EXPECT().ReadFile(gomock.Any()).Return([]byte("-----BEGIN PRIVATE KEY-----"), nil)
			},
			expectedError: "",
		},
		{
			name: "PromptForBastionHost fails",
			mockSetup: func(p *mock_awsctl.MockBastionPrompterInterface, _ *mock_awsctl.MockFileSystemInterface) {
				p.EXPECT().PromptForBastionHost().Return("", fmt.Errorf("host error"))
			},
			expectedError: "failed to get host",
		},
		{
			name: "PromptForSSHUser fails",
			mockSetup: func(p *mock_awsctl.MockBastionPrompterInterface, _ *mock_awsctl.MockFileSystemInterface) {
				p.EXPECT().PromptForBastionHost().Return("host", nil)
				p.EXPECT().PromptForSSHUser("ec2-user").Return("", fmt.Errorf("user error"))
			},
			expectedError: "failed to get user",
		},
		{
			name: "PromptForSSHKeyPath fails",
			mockSetup: func(p *mock_awsctl.MockBastionPrompterInterface, _ *mock_awsctl.MockFileSystemInterface) {
				p.EXPECT().PromptForBastionHost().Return("host", nil)
				p.EXPECT().PromptForSSHUser("ec2-user").Return("user", nil)
				p.EXPECT().PromptForSSHKeyPath("~/.ssh/id_ed25519").Return("", fmt.Errorf("key error"))
			},
			expectedError: "failed to get key path",
		},
		{
			name: "homeDir fails to return path",
			mockSetup: func(p *mock_awsctl.MockBastionPrompterInterface, _ *mock_awsctl.MockFileSystemInterface) {
				p.EXPECT().PromptForBastionHost().Return("host", nil)
				p.EXPECT().PromptForSSHUser("ec2-user").Return("user", nil)
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

			_, _, _, _, err := service.getConnectionDetails()

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestBastionService_getConnectionDetails_InstanceConnect(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tests := []struct {
		name                       string
		mockSetup                  func(mockPrompter *mock_awsctl.MockBastionPrompterInterface, mockFS *mock_awsctl.MockFileSystemInterface, mockIC *mock_awsctl.MockEC2InstanceConnectClient, mockEC2 *mock_awsctl.MockEC2ClientInterface)
		expectedHost               string
		expectedUser               string
		expectedKeyPath            string
		expectedUseInstanceConnect bool
		expectedError              string
	}{
		{
			name: "InstanceConnect Success",
			mockSetup: func(mockPrompter *mock_awsctl.MockBastionPrompterInterface, mockFS *mock_awsctl.MockFileSystemInterface, mockIC *mock_awsctl.MockEC2InstanceConnectClient, mockEC2 *mock_awsctl.MockEC2ClientInterface) {
				mockPrompter.EXPECT().PromptForConfirmation(gomock.Any()).Return(false, nil)
				mockPrompter.EXPECT().PromptForBastionHost().Return("i-1234567890", nil)
				mockPrompter.EXPECT().PromptForSSHUser("ec2-user").Return("testuser", nil)
				mockPrompter.EXPECT().PromptForSSHKeyPath("~/.ssh/id_ed25519").Return("/test/key", nil)

				mockFS.EXPECT().Stat("/test/key").Return(&mockFileInfo{mode: 0600}, nil)
				mockFS.EXPECT().ReadFile("/test/key").Return([]byte("-----BEGIN OPENSSH PRIVATE KEY-----\n..."), nil)
				mockFS.EXPECT().Stat("/test/key.pub").Return(&mockFileInfo{}, nil)
				mockFS.EXPECT().ReadFile("/test/key.pub").Return([]byte("public-key"), nil)

				mockEC2.EXPECT().DescribeInstances(gomock.Any(), gomock.Any()).Return(&ec2.DescribeInstancesOutput{
					Reservations: []types.Reservation{
						{
							Instances: []types.Instance{
								{
									InstanceId: aws.String("i-1234567890"),
									Placement: &types.Placement{
										AvailabilityZone: aws.String("us-west-2a"),
									},
								},
							},
						},
					},
				}, nil)

				mockIC.EXPECT().SendSSHPublicKey(gomock.Any(), gomock.Any()).
					Return(&ec2instanceconnect.SendSSHPublicKeyOutput{Success: true}, nil)
			},
			expectedHost:               "i-1234567890",
			expectedUser:               "testuser",
			expectedKeyPath:            "/test/key",
			expectedUseInstanceConnect: true,
			expectedError:              "",
		},
		{
			name: "InstanceConnect Fallback to Public IP",
			mockSetup: func(mockPrompter *mock_awsctl.MockBastionPrompterInterface, mockFS *mock_awsctl.MockFileSystemInterface, mockIC *mock_awsctl.MockEC2InstanceConnectClient, mockEC2 *mock_awsctl.MockEC2ClientInterface) {
				mockPrompter.EXPECT().PromptForConfirmation(gomock.Any()).Return(false, nil)
				mockPrompter.EXPECT().PromptForBastionHost().Return("i-1234567890", nil)
				mockPrompter.EXPECT().PromptForSSHUser("ec2-user").Return("testuser", nil)
				mockPrompter.EXPECT().PromptForSSHKeyPath("~/.ssh/id_ed25519").Return("/test/key", nil)

				mockFS.EXPECT().Stat("/test/key").Return(&mockFileInfo{mode: 0600}, nil)
				mockFS.EXPECT().ReadFile("/test/key").Return([]byte("-----BEGIN OPENSSH PRIVATE KEY-----\n..."), nil)
				mockFS.EXPECT().Stat("/test/key.pub").Return(&mockFileInfo{}, nil)
				mockFS.EXPECT().ReadFile("/test/key.pub").Return([]byte("public-key"), nil)

				mockEC2.EXPECT().DescribeInstances(gomock.Any(), gomock.Any()).Return(&ec2.DescribeInstancesOutput{
					Reservations: []types.Reservation{
						{
							Instances: []types.Instance{
								{
									InstanceId: aws.String("i-1234567890"),
									Placement: &types.Placement{
										AvailabilityZone: aws.String("us-west-2a"),
									},
								},
							},
						},
					},
				}, nil)

				mockIC.EXPECT().SendSSHPublicKey(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("instance connect failed"))

				mockEC2.EXPECT().DescribeInstances(gomock.Any(), gomock.Any()).Return(&ec2.DescribeInstancesOutput{
					Reservations: []types.Reservation{
						{
							Instances: []types.Instance{
								{
									InstanceId: aws.String("i-1234567890"),
									Placement: &types.Placement{
										AvailabilityZone: aws.String("us-west-2a"),
									},
									PublicIpAddress: aws.String("203.0.113.10"),
								},
							},
						},
					},
				}, nil)
			},
			expectedHost:               "203.0.113.10",
			expectedUser:               "testuser",
			expectedKeyPath:            "/test/key",
			expectedUseInstanceConnect: false,
			expectedError:              "",
		},
		{
			name: "InstanceConnect Fallback Error",
			mockSetup: func(mockPrompter *mock_awsctl.MockBastionPrompterInterface, mockFS *mock_awsctl.MockFileSystemInterface, mockIC *mock_awsctl.MockEC2InstanceConnectClient, mockEC2 *mock_awsctl.MockEC2ClientInterface) {
				mockPrompter.EXPECT().PromptForConfirmation(gomock.Any()).Return(false, nil)
				mockPrompter.EXPECT().PromptForBastionHost().Return("i-1234567890", nil)
				mockPrompter.EXPECT().PromptForSSHUser("ec2-user").Return("testuser", nil)
				mockPrompter.EXPECT().PromptForSSHKeyPath("~/.ssh/id_ed25519").Return("/test/key", nil)

				mockFS.EXPECT().Stat("/test/key").Return(&mockFileInfo{mode: 0600}, nil).AnyTimes()
				mockFS.EXPECT().ReadFile("/test/key").Return([]byte("private-key"), nil).AnyTimes()
				mockFS.EXPECT().Stat("/test/key.pub").Return(&mockFileInfo{}, nil).AnyTimes()
				mockFS.EXPECT().ReadFile("/test/key.pub").Return([]byte("public-key"), nil).AnyTimes()

				mockIC.EXPECT().SendSSHPublicKey(gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, input *ec2instanceconnect.SendSSHPublicKeyInput, _ ...func(*ec2instanceconnect.Options)) (*ec2instanceconnect.SendSSHPublicKeyOutput, error) {
						t.Logf("SendSSHPublicKey called with: %+v", input)
						return &ec2instanceconnect.SendSSHPublicKeyOutput{Success: false}, fmt.Errorf("failed to describe instance: no public IP found")
					}).Times(1)

				mockEC2.EXPECT().DescribeInstances(gomock.Any(), gomock.Any()).Return(&ec2.DescribeInstancesOutput{
					Reservations: []types.Reservation{
						{
							Instances: []types.Instance{
								{
									InstanceId: aws.String("i-1234567890"),
									Placement: &types.Placement{
										AvailabilityZone: aws.String("us-west-2a"),
									},
									NetworkInterfaces: []types.InstanceNetworkInterface{
										{
											PrivateIpAddress: aws.String("10.0.0.1"),
										},
									},
								},
							},
						},
					},
				}, nil).Times(1)

				mockEC2.EXPECT().DescribeInstances(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("no public IP found")).Times(1)
			},
			expectedHost:               "",
			expectedUser:               "",
			expectedKeyPath:            "",
			expectedUseInstanceConnect: false,
			expectedError:              "failed to get public IP for fallback",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockPrompter := mock_awsctl.NewMockBastionPrompterInterface(ctrl)
			mockFS := mock_awsctl.NewMockFileSystemInterface(ctrl)
			mockIC := mock_awsctl.NewMockEC2InstanceConnectClient(ctrl)
			mockEC2 := mock_awsctl.NewMockEC2ClientInterface(ctrl)

			if tt.mockSetup != nil {
				tt.mockSetup(mockPrompter, mockFS, mockIC, mockEC2)
			}

			service := &BastionService{
				BPrompter:             mockPrompter,
				Fs:                    mockFS,
				AwsConfigured:         true,
				InstanceConnectClient: mockIC,
				EC2Client:             mockEC2,
			}

			host, user, keyPath, useInstanceConnect, err := service.getConnectionDetails()

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tt.expectedHost, host)
			assert.Equal(t, tt.expectedUser, user)
			assert.Equal(t, tt.expectedKeyPath, keyPath)
			assert.Equal(t, tt.expectedUseInstanceConnect, useInstanceConnect)
		})
	}
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

		assert.False(t, service.AwsConfigured)
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

func TestBastionService_CleanupSOCKS(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tests := []struct {
		name          string
		socksPort     int
		mockSetup     func(*mock_awsctl.MockSSHExecutorInterface)
		expectedPort  int
		expectedError error
	}{
		{
			name:      "no socks port set",
			socksPort: 0,
			mockSetup: func(mock *mock_awsctl.MockSSHExecutorInterface) {

			},
			expectedPort: 0,
		},
		{
			name:      "successful cleanup",
			socksPort: 9999,
			mockSetup: func(mock *mock_awsctl.MockSSHExecutorInterface) {
				mock.EXPECT().Execute(
					[]string{"sh", "-c", "pkill -f 'ssh.*-D.*9999'"},
					gomock.Any(), nil, nil,
				).Return(nil)
			},
			expectedPort: 0,
		},
		{
			name:      "cleanup fails",
			socksPort: 9999,
			mockSetup: func(mock *mock_awsctl.MockSSHExecutorInterface) {
				mock.EXPECT().Execute(
					[]string{"sh", "-c", "pkill -f 'ssh.*-D.*9999'"},
					gomock.Any(), nil, nil,
				).Return(fmt.Errorf("pkill failed"))
			},
			expectedPort:  9999,
			expectedError: fmt.Errorf("failed to terminate SOCKS proxy on port 9999: pkill failed"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockSSH := mock_awsctl.NewMockSSHExecutorInterface(ctrl)
			if tt.mockSetup != nil {
				tt.mockSetup(mockSSH)
			}

			service := &BastionService{
				SSHExecutor: mockSSH,
				socksPort:   tt.socksPort,
			}

			err := service.CleanupSOCKS()
			if tt.expectedError != nil {
				assert.EqualError(t, err, tt.expectedError.Error())
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.expectedPort, service.socksPort)
		})
	}
}

func TestBastionService_getInstancePublicIP(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.TODO()
	instanceID := "i-1234567890abcdef0"
	publicIP := "203.0.113.42"

	tests := []struct {
		name           string
		awsConfigured  bool
		mockSetup      func(*mock_awsctl.MockEC2ClientInterface)
		expectedResult string
		expectedError  error
	}{
		{
			name:          "AWS not configured",
			awsConfigured: false,
			mockSetup:     nil,
			expectedError: errors.New("AWS not configured"),
		},
		{
			name:          "successful get public IP",
			awsConfigured: true,
			mockSetup: func(mockEC2 *mock_awsctl.MockEC2ClientInterface) {
				mockEC2.EXPECT().DescribeInstances(ctx, &ec2.DescribeInstancesInput{
					InstanceIds: []string{instanceID},
				}).Return(&ec2.DescribeInstancesOutput{
					Reservations: []types.Reservation{
						{
							Instances: []types.Instance{
								{
									PublicIpAddress: &publicIP,
								},
							},
						},
					},
				}, nil)
			},
			expectedResult: publicIP,
		},
		{
			name:          "instance not found",
			awsConfigured: true,
			mockSetup: func(mockEC2 *mock_awsctl.MockEC2ClientInterface) {
				mockEC2.EXPECT().DescribeInstances(ctx, gomock.Any()).Return(&ec2.DescribeInstancesOutput{
					Reservations: []types.Reservation{},
				}, nil)
			},
			expectedError: fmt.Errorf("no instance found with ID %s", instanceID),
		},
		{
			name:          "no public IP",
			awsConfigured: true,
			mockSetup: func(mockEC2 *mock_awsctl.MockEC2ClientInterface) {
				mockEC2.EXPECT().DescribeInstances(ctx, gomock.Any()).Return(&ec2.DescribeInstancesOutput{
					Reservations: []types.Reservation{
						{
							Instances: []types.Instance{
								{
									PublicIpAddress: nil,
								},
							},
						},
					},
				}, nil)
			},
			expectedError: fmt.Errorf("instance %s has no public IP address", instanceID),
		},
		{
			name:          "describe instances fails",
			awsConfigured: true,
			mockSetup: func(mockEC2 *mock_awsctl.MockEC2ClientInterface) {
				mockEC2.EXPECT().DescribeInstances(ctx, gomock.Any()).Return(nil, fmt.Errorf("throttling error"))
			},
			expectedError: fmt.Errorf("failed to describe instance: throttling error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var mockEC2 *mock_awsctl.MockEC2ClientInterface
			if tt.awsConfigured {
				mockEC2 = mock_awsctl.NewMockEC2ClientInterface(ctrl)
				if tt.mockSetup != nil {
					tt.mockSetup(mockEC2)
				}
			}

			service := &BastionService{
				EC2Client:     mockEC2,
				AwsConfigured: tt.awsConfigured,
			}

			result, err := service.getInstancePublicIP(instanceID)
			if tt.expectedError != nil {
				assert.EqualError(t, err, tt.expectedError.Error())
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedResult, result)
			}
		})
	}
}
