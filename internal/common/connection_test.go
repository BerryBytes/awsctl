package connection

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/BerryBytes/awsctl/models"
	mock_awsctl "github.com/BerryBytes/awsctl/tests/mock"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2instanceconnect"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

type mockFileInfo struct {
	name    string
	size    int64
	mode    os.FileMode
	modTime time.Time
	isDir   bool
}

func (m *mockFileInfo) Name() string       { return m.name }
func (m *mockFileInfo) Size() int64        { return m.size }
func (m *mockFileInfo) Mode() os.FileMode  { return m.mode }
func (m *mockFileInfo) ModTime() time.Time { return m.modTime }
func (m *mockFileInfo) IsDir() bool        { return m.isDir }
func (m *mockFileInfo) Sys() interface{}   { return nil }

type mocks struct {
	prompter     *mock_awsctl.MockConnectionPrompter
	fs           *mock_awsctl.MockFileSystemInterface
	ec2Client    *mock_awsctl.MockEC2ClientInterface
	instanceConn *mock_awsctl.MockEC2InstanceConnectInterface
	ssmClient    *mock_awsctl.MockSSMClientInterface
	configLoader *mock_awsctl.MockAWSConfigLoader
	ctrl         *gomock.Controller
}

func setupMocks(t *testing.T) mocks {
	ctrl := gomock.NewController(t)
	return mocks{
		prompter:     mock_awsctl.NewMockConnectionPrompter(ctrl),
		fs:           mock_awsctl.NewMockFileSystemInterface(ctrl),
		ec2Client:    mock_awsctl.NewMockEC2ClientInterface(ctrl),
		instanceConn: mock_awsctl.NewMockEC2InstanceConnectInterface(ctrl),
		ssmClient:    mock_awsctl.NewMockSSMClientInterface(ctrl),
		configLoader: mock_awsctl.NewMockAWSConfigLoader(ctrl),
		ctrl:         ctrl,
	}
}

func TestNewConnectionProvider(t *testing.T) {
	m := setupMocks(t)
	defer m.ctrl.Finish()

	credProvider := credentials.StaticCredentialsProvider{
		Value: aws.Credentials{
			AccessKeyID:     "mock-access-key",
			SecretAccessKey: "mock-secret-key",
			Source:          "test",
		},
	}
	awsConfig := aws.Config{
		Region:      "us-west-2",
		Credentials: credProvider,
	}

	provider := NewConnectionProvider(m.prompter, m.fs, awsConfig, m.ec2Client, m.ssmClient, m.instanceConn, nil)
	assert.NotNil(t, provider)
	assert.True(t, provider.awsConfigured, "awsConfigured should be true with valid credentials")
	assert.Equal(t, m.ec2Client, provider.ec2Client, "ec2Client should be set")
	assert.Equal(t, m.instanceConn, provider.instanceConn, "instanceConn should be set")

	provider = NewConnectionProvider(m.prompter, m.fs, aws.Config{}, nil, nil, nil, nil)
	assert.NotNil(t, provider)
	assert.False(t, provider.awsConfigured, "awsConfigured should be false with empty config")
	assert.Nil(t, provider.ec2Client, "ec2Client should be nil")
	assert.Nil(t, provider.instanceConn, "instanceConn should be nil")
}

func TestGetConnectionDetails_SSH(t *testing.T) {
	m := setupMocks(t)
	defer m.ctrl.Finish()

	credProvider := credentials.StaticCredentialsProvider{
		Value: aws.Credentials{
			AccessKeyID:     "mock-access-key",
			SecretAccessKey: "mock-secret-key",
			Source:          "test",
		},
	}
	awsConfig := aws.Config{
		Region:      "us-west-2",
		Credentials: credProvider,
	}
	provider := NewConnectionProvider(m.prompter, m.fs, awsConfig, m.ec2Client, m.ssmClient, m.instanceConn, nil)

	ctx := context.Background()
	homeDir, _ := os.UserHomeDir()
	keyPath := filepath.Join(homeDir, ".ssh/id_ed25519")

	m.prompter.EXPECT().ChooseConnectionMethod().Return(MethodSSH, nil)
	m.prompter.EXPECT().PromptForConfirmation("Look for bastion hosts in AWS?").Return(false, nil)
	m.prompter.EXPECT().PromptForBastionHost().Return("i-1234567890abcdef0", nil)
	m.prompter.EXPECT().PromptForSSHUser("ec2-user").Return("ec2-user", nil)
	m.prompter.EXPECT().PromptForSSHKeyPath("~/.ssh/id_ed25519").Return("~/.ssh/id_ed25519", nil)

	pubKeyInfo := &mockFileInfo{
		name:    "id_ed25519.pub",
		size:    68,
		mode:    0644,
		modTime: time.Now(),
		isDir:   false,
	}
	m.fs.EXPECT().Stat(keyPath+".pub").Return(pubKeyInfo, nil)
	m.fs.EXPECT().ReadFile(keyPath+".pub").Return([]byte("ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAA..."), nil)

	privateKeyInfo := &mockFileInfo{
		name:    "id_ed25519",
		size:    464,
		mode:    0600,
		modTime: time.Now(),
		isDir:   false,
	}
	m.fs.EXPECT().Stat(keyPath).Return(privateKeyInfo, nil)
	m.fs.EXPECT().ReadFile(keyPath).Return([]byte(`-----BEGIN OPENSSH PRIVATE KEY-----
b3BlbnNzaC1rZXktdjEAAAAABG5vbmUAAAAEbm9uZQAAAAAAAAABAAAAMwAAAAtzc2gtZW
QyNTUxOQAAACD6...example...
-----END OPENSSH PRIVATE KEY-----`), nil) // gitleaks:allow

	m.ec2Client.EXPECT().DescribeInstances(ctx, gomock.Any()).Return(&ec2.DescribeInstancesOutput{
		Reservations: []types.Reservation{
			{
				Instances: []types.Instance{
					{
						InstanceId:      aws.String("i-1234567890abcdef0"),
						PublicIpAddress: aws.String("203.0.113.1"),
						Placement:       &types.Placement{AvailabilityZone: aws.String("us-west-2a")},
					},
				},
			},
		},
	}, nil)

	m.instanceConn.EXPECT().SendSSHPublicKey(ctx, gomock.Any()).Return(&ec2instanceconnect.SendSSHPublicKeyOutput{}, nil)

	details, err := provider.GetConnectionDetails(ctx)
	assert.NoError(t, err)
	assert.Equal(t, &ConnectionDetails{
		Host:               "i-1234567890abcdef0",
		User:               "ec2-user",
		KeyPath:            keyPath,
		InstanceID:         "",
		UseInstanceConnect: true,
		Method:             MethodSSH,
	}, details)
}

func TestGetConnectionDetails_UnsupportedMethod(t *testing.T) {
	m := setupMocks(t)
	defer m.ctrl.Finish()

	provider := NewConnectionProvider(m.prompter, m.fs, aws.Config{}, m.ec2Client, m.ssmClient, m.instanceConn, nil)

	m.prompter.EXPECT().ChooseConnectionMethod().Return("INVALID", nil)

	_, err := provider.GetConnectionDetails(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported connection method: INVALID")
}

func TestGetBastionHost_AWSConfigured(t *testing.T) {
	m := setupMocks(t)
	defer m.ctrl.Finish()

	credProvider := credentials.StaticCredentialsProvider{
		Value: aws.Credentials{
			AccessKeyID:     "mock-access-key",
			SecretAccessKey: "mock-secret-key",
			Source:          "test",
		},
	}

	expectedDefaultRegion := "ap-south-1"
	awsConfig := aws.Config{
		Region:      expectedDefaultRegion,
		Credentials: credProvider,
	}

	mockEC2Client := mock_awsctl.NewMockEC2ClientInterface(m.ctrl)
	provider := &ConnectionProvider{
		prompter:      m.prompter,
		fs:            m.fs,
		awsConfig:     awsConfig,
		ec2Client:     mockEC2Client,
		instanceConn:  m.instanceConn,
		awsConfigured: true,
		configLoader:  m.configLoader,
		newEC2Client: func(region string, loader AWSConfigLoader) (EC2ClientInterface, error) {
			return mockEC2Client, nil
		},
	}

	ctx := context.Background()

	t.Run("successful bastion selection", func(t *testing.T) {
		m.prompter.EXPECT().PromptForConfirmation("Look for bastion hosts in AWS?").Return(true, nil)
		m.prompter.EXPECT().PromptForRegion(expectedDefaultRegion).Return(expectedDefaultRegion, nil)
		mockEC2Client.EXPECT().ListBastionInstances(ctx).Return([]models.EC2Instance{
			{InstanceID: "i-1234567890abcdef0", Name: "bastion-1"},
		}, nil)

		m.prompter.EXPECT().PromptForBastionInstance(gomock.Any()).Return("i-1234567890abcdef0", nil)

		host, err := provider.getBastionHost(ctx)
		assert.NoError(t, err)
		assert.Equal(t, "i-1234567890abcdef0", host)
	})

	t.Run("no bastions found", func(t *testing.T) {
		m.prompter.EXPECT().PromptForConfirmation("Look for bastion hosts in AWS?").Return(true, nil)
		m.prompter.EXPECT().PromptForRegion(expectedDefaultRegion).Return(expectedDefaultRegion, nil)
		mockEC2Client.EXPECT().ListBastionInstances(ctx).Return([]models.EC2Instance{}, nil)
		m.prompter.EXPECT().PromptForBastionHost().Return("bastion.example.com", nil)

		host, err := provider.getBastionHost(ctx)
		assert.NoError(t, err)
		assert.Equal(t, "bastion.example.com", host)
	})
	t.Run("error loading default region", func(t *testing.T) {
		awsConfigNoRegion := aws.Config{
			Credentials: credProvider,
		}

		mockEC2Client := mock_awsctl.NewMockEC2ClientInterface(m.ctrl)

		provider := &ConnectionProvider{
			prompter:      m.prompter,
			fs:            m.fs,
			awsConfig:     awsConfigNoRegion,
			ec2Client:     nil,
			instanceConn:  m.instanceConn,
			awsConfigured: true,
			configLoader:  m.configLoader,
			newEC2Client: func(region string, loader AWSConfigLoader) (EC2ClientInterface, error) {
				return mockEC2Client, nil
			},
		}

		old := os.Stdout
		r, w, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		os.Stdout = w

		m.configLoader.EXPECT().LoadDefaultConfig(gomock.Any()).Return(aws.Config{}, errors.New("config load error"))
		m.prompter.EXPECT().PromptForConfirmation("Look for bastion hosts in AWS?").Return(true, nil)
		m.prompter.EXPECT().PromptForRegion("").Return("us-east-1", nil)

		mockEC2Client.EXPECT().ListBastionInstances(ctx).Return([]models.EC2Instance{
			{InstanceID: "i-1234567890abcdef0", Name: "bastion-1"},
		}, nil)
		m.prompter.EXPECT().PromptForBastionInstance(gomock.Any()).Return("i-1234567890abcdef0", nil)

		host, err := provider.getBastionHost(ctx)

		w.Close()
		os.Stdout = old
		var buf bytes.Buffer
		if _, err := io.Copy(&buf, r); err != nil {
			t.Logf("failed to copy stdout: %v", err)
		}
		r.Close()

		assert.NoError(t, err)
		assert.Equal(t, "i-1234567890abcdef0", host)
		assert.Contains(t, buf.String(), "Failed to load default region: failed to load AWS configuration: config load error")
	})
	t.Run("nil ec2Client, error creating EC2 client", func(t *testing.T) {
		var logOutput strings.Builder
		log.SetOutput(&logOutput)
		defer log.SetOutput(os.Stderr)

		credProvider := credentials.StaticCredentialsProvider{
			Value: aws.Credentials{
				AccessKeyID:     "mock-access-key",
				SecretAccessKey: "mock-secret-key",
				Source:          "test",
			},
		}
		awsConfig := aws.Config{
			Region:      "us-east-1",
			Credentials: credProvider,
		}

		provider := NewConnectionProvider(
			m.prompter,
			m.fs,
			awsConfig,
			nil,
			m.ssmClient,
			m.instanceConn,
			m.configLoader,
		)
		provider.newEC2Client = func(region string, loader AWSConfigLoader) (EC2ClientInterface, error) {
			return nil, errors.New("client creation error")
		}

		m.prompter.EXPECT().PromptForConfirmation("Look for bastion hosts in AWS?").Return(true, nil)
		m.prompter.EXPECT().PromptForRegion("us-east-1").Return("us-east-1", nil)
		m.prompter.EXPECT().PromptForBastionHost().Return("bastion.example.com", nil)

		host, err := provider.getBastionHost(ctx)
		assert.NoError(t, err)
		assert.Equal(t, "bastion.example.com", host)
		assert.Contains(t, logOutput.String(), "Failed to initialize EC2 client: client creation error")
	})
}

func TestGetBastionHost_NoAWSConfig(t *testing.T) {
	m := setupMocks(t)
	defer m.ctrl.Finish()

	provider := NewConnectionProvider(m.prompter, m.fs, aws.Config{}, m.ec2Client, m.ssmClient, m.instanceConn, nil)

	m.prompter.EXPECT().PromptForBastionHost().Return("bastion.example.com", nil)

	host, err := provider.getBastionHost(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, "bastion.example.com", host)
}

func TestGetInstancePublicIP(t *testing.T) {
	m := setupMocks(t)
	defer m.ctrl.Finish()

	credProvider := credentials.StaticCredentialsProvider{
		Value: aws.Credentials{
			AccessKeyID:     "mock-access-key",
			SecretAccessKey: "mock-secret-key",
			Source:          "test",
		},
	}
	awsConfig := aws.Config{
		Region:      "us-west-2",
		Credentials: credProvider,
	}
	provider := NewConnectionProvider(m.prompter, m.fs, awsConfig, m.ec2Client, m.ssmClient, m.instanceConn, nil)

	ctx := context.Background()
	instanceID := "i-1234567890abcdef0"

	tests := []struct {
		name          string
		mockResponse  *ec2.DescribeInstancesOutput
		mockError     error
		expectedIP    string
		expectedError string
	}{
		{
			name: "Success with public IP",
			mockResponse: &ec2.DescribeInstancesOutput{
				Reservations: []types.Reservation{
					{
						Instances: []types.Instance{
							{
								InstanceId:      aws.String(instanceID),
								PublicIpAddress: aws.String("203.0.113.1"),
							},
						},
					},
				},
			},
			expectedIP: "203.0.113.1",
		},
		{
			name: "No instances found",
			mockResponse: &ec2.DescribeInstancesOutput{
				Reservations: []types.Reservation{},
			},
			expectedError: "no instance found with ID",
		},
		{
			name: "No public IP",
			mockResponse: &ec2.DescribeInstancesOutput{
				Reservations: []types.Reservation{
					{
						Instances: []types.Instance{
							{
								InstanceId: aws.String(instanceID),
							},
						},
					},
				},
			},
			expectedError: "has no public IP address",
		},
		{
			name:          "AWS error",
			mockError:     errors.New("aws error"),
			expectedError: "failed to describe instance",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m.ec2Client.EXPECT().DescribeInstances(ctx, gomock.Any()).Return(tt.mockResponse, tt.mockError)

			ip, err := provider.getInstancePublicIP(ctx, instanceID)
			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedIP, ip)
			}
		})
	}
}

func TestGetConnectionDetails_ErrorCases(t *testing.T) {
	tests := []struct {
		name          string
		awsConfigured bool
		setupMocks    func(m mocks)
		expectedError string
	}{
		{
			name: "Error choosing connection method",
			setupMocks: func(m mocks) {
				m.prompter.EXPECT().ChooseConnectionMethod().Return("", errors.New("prompt error"))
			},
			expectedError: "failed to select connection method",
		},
		{
			name:          "Error getting SSH user - AWS not configured",
			awsConfigured: false,
			setupMocks: func(m mocks) {
				m.prompter.EXPECT().ChooseConnectionMethod().Return(MethodSSH, nil)
				m.prompter.EXPECT().PromptForBastionHost().Return("bastion.example.com", nil)
				m.prompter.EXPECT().PromptForSSHUser("ec2-user").Return("", errors.New("user error"))
			},
			expectedError: "failed to get user",
		},
		{
			name:          "Error getting SSH user - AWS configured",
			awsConfigured: true,
			setupMocks: func(m mocks) {
				m.prompter.EXPECT().ChooseConnectionMethod().Return(MethodSSH, nil)
				m.prompter.EXPECT().PromptForConfirmation("Look for bastion hosts in AWS?").Return(false, nil)
				m.prompter.EXPECT().PromptForBastionHost().Return("bastion.example.com", nil)
				m.prompter.EXPECT().PromptForSSHUser("ec2-user").Return("", errors.New("user error"))
			},
			expectedError: "failed to get user",
		},
		{
			name:          "Error getting SSH key path - AWS not configured",
			awsConfigured: false,
			setupMocks: func(m mocks) {
				m.prompter.EXPECT().ChooseConnectionMethod().Return(MethodSSH, nil)
				m.prompter.EXPECT().PromptForBastionHost().Return("bastion.example.com", nil)
				m.prompter.EXPECT().PromptForSSHUser("ec2-user").Return("ec2-user", nil)
				m.prompter.EXPECT().PromptForSSHKeyPath("~/.ssh/id_ed25519").Return("", errors.New("key error"))
			},
			expectedError: "failed to get key path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := setupMocks(t)
			defer m.ctrl.Finish()

			var provider *ConnectionProvider
			if tt.awsConfigured {
				credProvider := credentials.StaticCredentialsProvider{
					Value: aws.Credentials{
						AccessKeyID:     "mock-access-key",
						SecretAccessKey: "mock-secret-key",
						Source:          "test",
					},
				}
				awsConfig := aws.Config{
					Region:      "us-west-2",
					Credentials: credProvider,
				}
				provider = NewConnectionProvider(m.prompter, m.fs, awsConfig, m.ec2Client, m.ssmClient, m.instanceConn, nil)
			} else {
				provider = NewConnectionProvider(m.prompter, m.fs, aws.Config{}, nil, nil, nil, nil)
			}

			tt.setupMocks(m)

			_, err := provider.GetConnectionDetails(context.Background())
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedError)
		})
	}
}

func TestGetSSHDetails_InstanceConnectFallback(t *testing.T) {
	m := setupMocks(t)
	defer m.ctrl.Finish()

	credProvider := credentials.StaticCredentialsProvider{
		Value: aws.Credentials{
			AccessKeyID:     "mock-access-key",
			SecretAccessKey: "mock-secret-key",
			Source:          "test",
		},
	}
	awsConfig := aws.Config{
		Region:      "us-west-2",
		Credentials: credProvider,
	}
	provider := NewConnectionProvider(m.prompter, m.fs, awsConfig, m.ec2Client, m.ssmClient, m.instanceConn, nil)

	ctx := context.Background()
	homeDir, _ := os.UserHomeDir()
	keyPath := filepath.Join(homeDir, ".ssh/id_ed25519")

	m.prompter.EXPECT().ChooseConnectionMethod().Return(MethodSSH, nil)
	m.prompter.EXPECT().PromptForConfirmation("Look for bastion hosts in AWS?").Return(false, nil)
	m.prompter.EXPECT().PromptForBastionHost().Return("i-1234567890abcdef0", nil)
	m.prompter.EXPECT().PromptForSSHUser("ec2-user").Return("ec2-user", nil)
	m.prompter.EXPECT().PromptForSSHKeyPath("~/.ssh/id_ed25519").Return("~/.ssh/id_ed25519", nil)

	pubKeyInfo := &mockFileInfo{
		name:    "id_ed25519.pub",
		size:    68,
		mode:    0644,
		modTime: time.Now(),
		isDir:   false,
	}
	m.fs.EXPECT().Stat(keyPath+".pub").Return(pubKeyInfo, nil)
	m.fs.EXPECT().ReadFile(keyPath+".pub").Return([]byte("ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAA..."), nil)

	privateKeyInfo := &mockFileInfo{
		name:    "id_ed25519",
		size:    464,
		mode:    0600,
		modTime: time.Now(),
		isDir:   false,
	}
	m.fs.EXPECT().Stat(keyPath).Return(privateKeyInfo, nil)

	m.fs.EXPECT().ReadFile(keyPath).Return([]byte(`-----BEGIN OPENSSH PRIVATE KEY-----
b3BlbnNzaC1rZXktdjEAAAAABG5vbmUAAAAEbm9uZQAAAAAAAAABAAAAMwAAAAtzc2gtZW
QyNTUxOQAAACD6...example...
-----END OPENSSH PRIVATE KEY-----`), nil) // gitleaks:allow

	m.instanceConn.EXPECT().SendSSHPublicKey(ctx, gomock.Any()).Return(nil, errors.New("connect error"))

	m.ec2Client.EXPECT().DescribeInstances(ctx, gomock.Any()).Return(&ec2.DescribeInstancesOutput{
		Reservations: []types.Reservation{
			{
				Instances: []types.Instance{
					{
						InstanceId:      aws.String("i-1234567890abcdef0"),
						PublicIpAddress: aws.String("203.0.113.1"),
						Placement:       &types.Placement{AvailabilityZone: aws.String("us-west-2a")},
					},
				},
			},
		},
	}, nil).Times(2)

	details, err := provider.GetConnectionDetails(ctx)
	assert.NoError(t, err)
	assert.Equal(t, "203.0.113.1", details.Host)
	assert.False(t, details.UseInstanceConnect)
}

func TestGetBastionHost_AWSFailure(t *testing.T) {
	m := setupMocks(t)
	defer m.ctrl.Finish()

	credProvider := credentials.StaticCredentialsProvider{
		Value: aws.Credentials{
			AccessKeyID:     "mock-access-key",
			SecretAccessKey: "mock-secret-key",
			Source:          "test",
		},
	}
	awsConfig := aws.Config{
		Region:      "ap-south-1",
		Credentials: credProvider,
	}

	provider := &ConnectionProvider{
		prompter:      m.prompter,
		fs:            m.fs,
		awsConfig:     awsConfig,
		ec2Client:     nil,
		instanceConn:  m.instanceConn,
		awsConfigured: true,
		configLoader:  m.configLoader,
		newEC2Client: func(region string, loader AWSConfigLoader) (EC2ClientInterface, error) {
			return m.ec2Client, nil
		},
	}

	ctx := context.Background()

	var logOutput bytes.Buffer
	log.SetOutput(&logOutput)
	defer log.SetOutput(os.Stderr)

	m.prompter.EXPECT().PromptForConfirmation("Look for bastion hosts in AWS?").Return(true, nil)
	m.prompter.EXPECT().PromptForRegion("ap-south-1").Return("ap-south-1", nil)
	m.ec2Client.EXPECT().ListBastionInstances(ctx).Return(nil, errors.New("aws error"))
	m.prompter.EXPECT().PromptForBastionHost().Return("bastion.example.com", nil)

	host, err := provider.getBastionHost(ctx)

	assert.NoError(t, err)
	assert.Equal(t, "bastion.example.com", host)

	logStr := logOutput.String()
	assert.Contains(t, logStr, "AWS lookup failed")
	assert.Contains(t, logStr, "aws error")
}

func TestGetSSHDetails_HomeDirError(t *testing.T) {
	m := setupMocks(t)
	defer m.ctrl.Finish()

	provider := &ConnectionProvider{
		prompter: m.prompter,
		fs:       m.fs,
		homeDir:  func() (string, error) { return "", errors.New("home dir error") },
	}

	m.prompter.EXPECT().ChooseConnectionMethod().Return(MethodSSH, nil)
	m.prompter.EXPECT().PromptForBastionHost().Return("bastion.example.com", nil)
	m.prompter.EXPECT().PromptForSSHUser("ec2-user").Return("ec2-user", nil)
	m.prompter.EXPECT().PromptForSSHKeyPath("~/.ssh/id_ed25519").Return("~/.ssh/id_ed25519", nil)

	_, err := provider.GetConnectionDetails(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get home directory")
}

func TestSendSSHPublicKey_ErrorCases(t *testing.T) {
	m := setupMocks(t)
	defer m.ctrl.Finish()

	credProvider := credentials.StaticCredentialsProvider{
		Value: aws.Credentials{
			AccessKeyID:     "mock-access-key",
			SecretAccessKey: "mock-secret-key",
			Source:          "test",
		},
	}
	awsConfig := aws.Config{
		Region:      "us-west-2",
		Credentials: credProvider,
	}
	provider := NewConnectionProvider(m.prompter, m.fs, awsConfig, m.ec2Client, m.ssmClient, m.instanceConn, nil)

	ctx := context.Background()
	instanceID := "i-1234567890abcdef0"
	user := "ec2-user"
	pubKeyPath := "/path/to/key.pub"

	tests := []struct {
		name          string
		setupMocks    func()
		expectedError string
	}{
		{
			name: "Failed to read public key",
			setupMocks: func() {
				m.fs.EXPECT().ReadFile(pubKeyPath).Return(nil, errors.New("read error"))
			},
			expectedError: "failed to read public key",
		},
		{
			name: "Failed to describe instance",
			setupMocks: func() {
				m.fs.EXPECT().ReadFile(pubKeyPath).Return([]byte("public-key"), nil)
				m.ec2Client.EXPECT().DescribeInstances(ctx, gomock.Any()).Return(nil, errors.New("describe error"))
			},
			expectedError: "failed to describe instance",
		},
		{
			name: "No instance found",
			setupMocks: func() {
				m.fs.EXPECT().ReadFile(pubKeyPath).Return([]byte("public-key"), nil)
				m.ec2Client.EXPECT().DescribeInstances(ctx, gomock.Any()).Return(&ec2.DescribeInstancesOutput{
					Reservations: []types.Reservation{},
				}, nil)
			},
			expectedError: "no instance found with ID",
		},
		{
			name: "No availability zone",
			setupMocks: func() {
				m.fs.EXPECT().ReadFile(pubKeyPath).Return([]byte("public-key"), nil)
				m.ec2Client.EXPECT().DescribeInstances(ctx, gomock.Any()).Return(&ec2.DescribeInstancesOutput{
					Reservations: []types.Reservation{
						{
							Instances: []types.Instance{
								{
									InstanceId: aws.String(instanceID),
									Placement:  &types.Placement{},
								},
							},
						},
					},
				}, nil)
			},
			expectedError: "has no availability zone",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMocks()
			err := provider.sendSSHPublicKey(ctx, instanceID, user, pubKeyPath)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedError)
		})
	}
}

func TestGetSSHDetails_InvalidSSHKey(t *testing.T) {
	m := setupMocks(t)
	defer m.ctrl.Finish()

	provider := NewConnectionProvider(m.prompter, m.fs, aws.Config{}, nil, nil, nil, nil)

	homeDir, _ := os.UserHomeDir()
	keyPath := filepath.Join(homeDir, ".ssh/id_ed25519")

	m.prompter.EXPECT().ChooseConnectionMethod().Return(MethodSSH, nil)
	m.prompter.EXPECT().PromptForBastionHost().Return("bastion.example.com", nil)
	m.prompter.EXPECT().PromptForSSHUser("ec2-user").Return("ec2-user", nil)
	m.prompter.EXPECT().PromptForSSHKeyPath("~/.ssh/id_ed25519").Return("~/.ssh/id_ed25519", nil)

	keyInfo := &mockFileInfo{
		name:    "id_ed25519",
		size:    464,
		mode:    0600,
		modTime: time.Now(),
		isDir:   false,
	}
	m.fs.EXPECT().Stat(keyPath).Return(keyInfo, nil)
	m.fs.EXPECT().ReadFile(keyPath).Return([]byte("invalid-key"), nil)

	_, err := provider.GetConnectionDetails(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid SSH key")
}

func TestGetBastionHost_RegionPromptError(t *testing.T) {
	m := setupMocks(t)
	defer m.ctrl.Finish()

	credProvider := credentials.StaticCredentialsProvider{
		Value: aws.Credentials{
			AccessKeyID:     "mock-access-key",
			SecretAccessKey: "mock-secret-key",
			Source:          "test",
		},
	}
	awsConfig := aws.Config{
		Region:      "us-west-2",
		Credentials: credProvider,
	}
	provider := NewConnectionProvider(m.prompter, m.fs, awsConfig, m.ec2Client, m.ssmClient, m.instanceConn, nil)

	ctx := context.Background()

	var logOutput strings.Builder
	log.SetOutput(&logOutput)
	defer log.SetOutput(os.Stderr)

	stdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	defer func() {
		w.Close()
		os.Stdout = stdout
		var stdoutOutput strings.Builder
		if _, err := io.Copy(&stdoutOutput, r); err != nil {
			t.Logf("error copying from pipe: %v", err)
		}
		r.Close()
		assert.Contains(t, stdoutOutput.String(), "Failed to get region")
	}()

	m.prompter.EXPECT().PromptForConfirmation("Look for bastion hosts in AWS?").Return(true, nil)
	m.prompter.EXPECT().PromptForRegion("us-west-2").Return("", errors.New("region prompt failed"))
	m.prompter.EXPECT().PromptForBastionHost().Return("bastion.example.com", nil)

	host, err := provider.getBastionHost(ctx)
	assert.NoError(t, err)
	assert.Equal(t, "bastion.example.com", host)
}

func TestGetDefaultRegion(t *testing.T) {
	m := setupMocks(t)
	defer m.ctrl.Finish()

	tests := []struct {
		name           string
		awsConfig      aws.Config
		mockSetup      func()
		expectedRegion string
		expectedError  string
	}{
		{
			name: "Non-empty awsConfig Region",
			awsConfig: aws.Config{
				Region: "us-west-2",
				Credentials: credentials.StaticCredentialsProvider{
					Value: aws.Credentials{
						AccessKeyID:     "mock-access-key",
						SecretAccessKey: "mock-secret-key",
						Source:          "test",
					},
				},
			},
			mockSetup:      func() {},
			expectedRegion: "us-west-2",
			expectedError:  "",
		},
		{
			name: "Empty awsConfig Region, valid default config",
			awsConfig: aws.Config{
				Credentials: credentials.StaticCredentialsProvider{
					Value: aws.Credentials{
						AccessKeyID:     "mock-access-key",
						SecretAccessKey: "mock-secret-key",
						Source:          "test",
					},
				},
			},
			mockSetup: func() {
				m.configLoader.EXPECT().LoadDefaultConfig(gomock.Any()).Return(aws.Config{Region: "us-east-1"}, nil)
			},
			expectedRegion: "us-east-1",
			expectedError:  "",
		},
		{
			name: "Empty awsConfig Region, no region in default config",
			awsConfig: aws.Config{
				Credentials: credentials.StaticCredentialsProvider{
					Value: aws.Credentials{
						AccessKeyID:     "mock-access-key",
						SecretAccessKey: "mock-secret-key",
						Source:          "test",
					},
				},
			},
			mockSetup: func() {
				m.configLoader.EXPECT().LoadDefaultConfig(gomock.Any()).Return(aws.Config{}, nil)
			},
			expectedRegion: "",
			expectedError:  "no region configured in AWS config",
		},
		{
			name: "Empty awsConfig Region, error loading config",
			awsConfig: aws.Config{
				Credentials: credentials.StaticCredentialsProvider{
					Value: aws.Credentials{
						AccessKeyID:     "mock-access-key",
						SecretAccessKey: "mock-secret-key",
						Source:          "test",
					},
				},
			},
			mockSetup: func() {
				m.configLoader.EXPECT().LoadDefaultConfig(gomock.Any()).Return(aws.Config{}, errors.New("config load error"))
			},
			expectedRegion: "",
			expectedError:  "failed to load AWS configuration: config load error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := NewConnectionProvider(
				m.prompter,
				m.fs,
				tt.awsConfig,
				m.ec2Client,
				m.ssmClient,
				m.instanceConn,
				m.configLoader,
			)

			tt.mockSetup()

			region, err := provider.getDefaultRegion()
			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
				assert.Equal(t, tt.expectedRegion, region)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedRegion, region)
			}
		})
	}
}
