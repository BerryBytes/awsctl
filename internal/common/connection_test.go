package connection_test

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

	connection "github.com/BerryBytes/awsctl/internal/common"
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

	provider := connection.NewConnectionProvider(m.prompter, m.fs, awsConfig, m.ec2Client, m.ssmClient, m.instanceConn, nil)
	assert.NotNil(t, provider)
	assert.True(t, provider.AwsConfigured, "awsConfigured should be true with valid credentials")
	assert.Equal(t, m.ec2Client, provider.Ec2Client, "ec2Client should be set")
	assert.Equal(t, m.instanceConn, provider.InstanceConn, "instanceConn should be set")

	provider = connection.NewConnectionProvider(m.prompter, m.fs, aws.Config{}, nil, nil, nil, nil)
	assert.NotNil(t, provider)
	assert.False(t, provider.AwsConfigured, "awsConfigured should be false with empty config")
	assert.Nil(t, provider.Ec2Client, "ec2Client should be nil")
	assert.Nil(t, provider.InstanceConn, "instanceConn should be nil")
}

func TestGetConnectionDetails_UnsupportedMethod(t *testing.T) {
	m := setupMocks(t)
	defer m.ctrl.Finish()

	provider := connection.NewConnectionProvider(m.prompter, m.fs, aws.Config{}, m.ec2Client, m.ssmClient, m.instanceConn, nil)

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
	provider := &connection.ConnectionProvider{
		Prompter:      m.prompter,
		Fs:            m.fs,
		AwsConfig:     awsConfig,
		Ec2Client:     mockEC2Client,
		InstanceConn:  m.instanceConn,
		AwsConfigured: true,
		ConfigLoader:  m.configLoader,
		NewEC2Client: func(region string, loader connection.AWSConfigLoader) (connection.EC2ClientInterface, error) {
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

		m.prompter.EXPECT().PromptForBastionInstance(gomock.Any(), false).Return("i-1234567890abcdef0", nil)

		host, err := provider.GetBastionHost(ctx)
		assert.NoError(t, err)
		assert.Equal(t, "i-1234567890abcdef0", host)
	})

	t.Run("no bastions found", func(t *testing.T) {
		m.prompter.EXPECT().PromptForConfirmation("Look for bastion hosts in AWS?").Return(true, nil)
		m.prompter.EXPECT().PromptForRegion(expectedDefaultRegion).Return(expectedDefaultRegion, nil)
		mockEC2Client.EXPECT().ListBastionInstances(ctx).Return([]models.EC2Instance{}, nil)
		m.prompter.EXPECT().PromptForBastionHost().Return("bastion.example.com", nil)

		host, err := provider.GetBastionHost(ctx)
		assert.NoError(t, err)
		assert.Equal(t, "bastion.example.com", host)
	})
	t.Run("error loading default region", func(t *testing.T) {
		awsConfigNoRegion := aws.Config{
			Credentials: credProvider,
		}

		mockEC2Client := mock_awsctl.NewMockEC2ClientInterface(m.ctrl)

		provider := &connection.ConnectionProvider{
			Prompter:      m.prompter,
			Fs:            m.fs,
			AwsConfig:     awsConfigNoRegion,
			Ec2Client:     nil,
			InstanceConn:  m.instanceConn,
			AwsConfigured: true,
			ConfigLoader:  m.configLoader,
			NewEC2Client: func(region string, loader connection.AWSConfigLoader) (connection.EC2ClientInterface, error) {
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
		m.prompter.EXPECT().PromptForBastionInstance(gomock.Any(), false).Return("i-1234567890abcdef0", nil)

		host, err := provider.GetBastionHost(ctx)

		_ = w.Close()
		os.Stdout = old
		var buf bytes.Buffer
		if _, err := io.Copy(&buf, r); err != nil {
			t.Logf("failed to copy stdout: %v", err)
		}
		_ = r.Close()

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

		provider := connection.NewConnectionProvider(
			m.prompter,
			m.fs,
			awsConfig,
			nil,
			m.ssmClient,
			m.instanceConn,
			m.configLoader,
		)
		provider.NewEC2Client = func(region string, loader connection.AWSConfigLoader) (connection.EC2ClientInterface, error) {
			return nil, errors.New("client creation error")
		}

		m.prompter.EXPECT().PromptForConfirmation("Look for bastion hosts in AWS?").Return(true, nil)
		m.prompter.EXPECT().PromptForRegion("us-east-1").Return("us-east-1", nil)
		m.prompter.EXPECT().PromptForBastionHost().Return("bastion.example.com", nil)

		host, err := provider.GetBastionHost(ctx)
		assert.NoError(t, err)
		assert.Equal(t, "bastion.example.com", host)
		assert.Contains(t, logOutput.String(), "Failed to initialize EC2 client: client creation error")
	})
}

func TestGetBastionHost_NoAWSConfig(t *testing.T) {
	m := setupMocks(t)
	defer m.ctrl.Finish()

	provider := connection.NewConnectionProvider(m.prompter, m.fs, aws.Config{}, m.ec2Client, m.ssmClient, m.instanceConn, nil)

	m.prompter.EXPECT().PromptForBastionHost().Return("bastion.example.com", nil)

	host, err := provider.GetBastionHost(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, "bastion.example.com", host)
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
				m.prompter.EXPECT().ChooseConnectionMethod().Return(connection.MethodSSH, nil)
				m.prompter.EXPECT().PromptForBastionHost().Return("bastion.example.com", nil)
				m.prompter.EXPECT().PromptForSSHUser("ec2-user").Return("", errors.New("user error"))
			},
			expectedError: "failed to get user",
		},
		{
			name:          "Error getting SSH user - AWS configured",
			awsConfigured: true,
			setupMocks: func(m mocks) {
				m.prompter.EXPECT().ChooseConnectionMethod().Return(connection.MethodSSH, nil)
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
				m.prompter.EXPECT().ChooseConnectionMethod().Return(connection.MethodSSH, nil)
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

			var provider *connection.ConnectionProvider
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
				provider = connection.NewConnectionProvider(m.prompter, m.fs, awsConfig, m.ec2Client, m.ssmClient, m.instanceConn, nil)
			} else {
				provider = connection.NewConnectionProvider(m.prompter, m.fs, aws.Config{}, nil, nil, nil, nil)
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
	provider := connection.NewConnectionProvider(m.prompter, m.fs, awsConfig, m.ec2Client, m.ssmClient, m.instanceConn, nil)

	ctx := context.Background()

	m.prompter.EXPECT().ChooseConnectionMethod().Return(connection.MethodSSH, nil)
	m.prompter.EXPECT().PromptForConfirmation("Look for bastion hosts in AWS?").Return(false, nil)
	m.prompter.EXPECT().PromptForBastionHost().Return("i-1234567890abcdef0", nil)
	m.prompter.EXPECT().PromptForSSHUser("ec2-user").Return("ec2-user", nil)

	publicIP := "1.2.3.4"
	m.ec2Client.EXPECT().DescribeInstances(gomock.Any(), gomock.Any()).Return(&ec2.DescribeInstancesOutput{
		Reservations: []types.Reservation{
			{
				Instances: []types.Instance{
					{
						InstanceId:      aws.String("i-1234567890abcdef0"),
						PublicIpAddress: &publicIP,
						State: &types.InstanceState{
							Name: types.InstanceStateNameRunning,
						},
					},
				},
			},
		},
	}, nil)

	m.ec2Client.EXPECT().DescribeInstances(gomock.Any(), gomock.Any()).Return(&ec2.DescribeInstancesOutput{
		Reservations: []types.Reservation{
			{
				Instances: []types.Instance{
					{
						InstanceId:      aws.String("i-1234567890abcdef0"),
						Placement:       &types.Placement{AvailabilityZone: aws.String("us-west-2a")},
						PublicIpAddress: &publicIP,
						State:           &types.InstanceState{Name: types.InstanceStateNameRunning},
					},
				},
			},
		},
	}, nil)

	m.instanceConn.EXPECT().SendSSHPublicKey(gomock.Any(), gomock.Any()).Return(&ec2instanceconnect.SendSSHPublicKeyOutput{Success: true}, nil)

	details, err := provider.GetConnectionDetails(ctx)
	assert.NoError(t, err)
	assert.Equal(t, "1.2.3.4", details.Host)
	assert.True(t, details.UseInstanceConnect)
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

	provider := &connection.ConnectionProvider{
		Prompter:      m.prompter,
		Fs:            m.fs,
		AwsConfig:     awsConfig,
		Ec2Client:     nil,
		InstanceConn:  m.instanceConn,
		AwsConfigured: true,
		ConfigLoader:  m.configLoader,
		NewEC2Client: func(region string, loader connection.AWSConfigLoader) (connection.EC2ClientInterface, error) {
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

	host, err := provider.GetBastionHost(ctx)

	assert.NoError(t, err)
	assert.Equal(t, "bastion.example.com", host)

	logStr := logOutput.String()
	assert.Contains(t, logStr, "AWS lookup failed")
	assert.Contains(t, logStr, "aws error")
}

func TestGetSSHDetails_HomeDirError(t *testing.T) {
	m := setupMocks(t)
	defer m.ctrl.Finish()

	provider := &connection.ConnectionProvider{
		Prompter: m.prompter,
		Fs:       m.fs,
		HomeDir:  func() (string, error) { return "", errors.New("home dir error") },
	}

	m.prompter.EXPECT().ChooseConnectionMethod().Return(connection.MethodSSH, nil)
	m.prompter.EXPECT().PromptForBastionHost().Return("bastion.example.com", nil)
	m.prompter.EXPECT().PromptForSSHUser("ec2-user").Return("ec2-user", nil)
	m.prompter.EXPECT().PromptForSSHKeyPath("~/.ssh/id_ed25519").Return("~/.ssh/id_ed25519", nil)

	_, err := provider.GetConnectionDetails(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get home directory")
}

func TestGetSSHDetails_InvalidSSHKey(t *testing.T) {
	m := setupMocks(t)
	defer m.ctrl.Finish()

	provider := connection.NewConnectionProvider(m.prompter, m.fs, aws.Config{}, nil, nil, nil, nil)

	homeDir, _ := os.UserHomeDir()
	keyPath := filepath.Join(homeDir, ".ssh/id_ed25519")

	m.prompter.EXPECT().ChooseConnectionMethod().Return(connection.MethodSSH, nil)
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
	provider := connection.NewConnectionProvider(m.prompter, m.fs, awsConfig, m.ec2Client, m.ssmClient, m.instanceConn, nil)

	ctx := context.Background()

	var logOutput strings.Builder
	log.SetOutput(&logOutput)
	defer log.SetOutput(os.Stderr)

	stdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	defer func() {
		_ = w.Close()
		os.Stdout = stdout
		var stdoutOutput strings.Builder
		if _, err := io.Copy(&stdoutOutput, r); err != nil {
			t.Logf("error copying from pipe: %v", err)
		}
		_ = r.Close()
		assert.Contains(t, stdoutOutput.String(), "Failed to get region")
	}()

	m.prompter.EXPECT().PromptForConfirmation("Look for bastion hosts in AWS?").Return(true, nil)
	m.prompter.EXPECT().PromptForRegion("us-west-2").Return("", errors.New("region prompt failed"))
	m.prompter.EXPECT().PromptForBastionHost().Return("bastion.example.com", nil)

	host, err := provider.GetBastionHost(ctx)
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
			provider := connection.NewConnectionProvider(
				m.prompter,
				m.fs,
				tt.awsConfig,
				m.ec2Client,
				m.ssmClient,
				m.instanceConn,
				m.configLoader,
			)

			tt.mockSetup()

			region, err := provider.GetDefaultRegion()
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

func TestGetSSMDetails(t *testing.T) {
	m := setupMocks(t)
	defer m.ctrl.Finish()

	ctx := context.Background()

	tests := []struct {
		name          string
		awsConfigured bool
		setupMocks    func(*connection.ConnectionProvider)
		expected      *connection.ConnectionDetails
		expectedError string
	}{
		{
			name:          "Success with AWS lookup",
			awsConfigured: true,
			setupMocks: func(provider *connection.ConnectionProvider) {
				m.configLoader.EXPECT().LoadDefaultConfig(gomock.Any()).Return(aws.Config{Region: "us-west-2"}, nil).AnyTimes()
				m.prompter.EXPECT().PromptForRegion("us-west-2").Return("us-west-2", nil)
				provider.NewEC2Client = func(region string, loader connection.AWSConfigLoader) (connection.EC2ClientInterface, error) {
					return m.ec2Client, nil
				}
				m.ec2Client.EXPECT().ListBastionInstances(ctx).Return([]models.EC2Instance{
					{InstanceID: "i-1234567890abcdef0", Name: "bastion-1"},
				}, nil)
				m.prompter.EXPECT().PromptForBastionInstance(gomock.Any(), true).Return("i-1234567890abcdef0", nil)
			},
			expected: &connection.ConnectionDetails{
				InstanceID: "i-1234567890abcdef0",
				Method:     connection.MethodSSM,
				SSMClient:  m.ssmClient,
			},
		},
		{
			name:          "Success with manual instance ID",
			awsConfigured: true,
			setupMocks: func(provider *connection.ConnectionProvider) {
				m.configLoader.EXPECT().LoadDefaultConfig(gomock.Any()).Return(aws.Config{Region: "us-west-2"}, nil).AnyTimes()
				m.prompter.EXPECT().PromptForRegion("us-west-2").Return("us-west-2", nil)
				provider.NewEC2Client = func(region string, loader connection.AWSConfigLoader) (connection.EC2ClientInterface, error) {
					return m.ec2Client, nil
				}
				m.ec2Client.EXPECT().ListBastionInstances(ctx).Return(nil, errors.New("AWS error"))
				m.prompter.EXPECT().PromptForInstanceID().Return("i-manualinput", nil)
			},
			expected: &connection.ConnectionDetails{
				InstanceID: "i-manualinput",
				Method:     connection.MethodSSM,
				SSMClient:  m.ssmClient,
			},
		},
		{
			name:          "AWS not configured",
			awsConfigured: false,
			setupMocks:    func(provider *connection.ConnectionProvider) {},
			expectedError: "AWS configuration required for SSM access",
		},
		{
			name:          "Invalid manual instance ID format",
			awsConfigured: true,
			setupMocks: func(provider *connection.ConnectionProvider) {
				m.configLoader.EXPECT().LoadDefaultConfig(gomock.Any()).Return(aws.Config{Region: "us-west-2"}, nil).AnyTimes()
				m.prompter.EXPECT().PromptForRegion("us-west-2").Return("us-west-2", nil)
				provider.NewEC2Client = func(region string, loader connection.AWSConfigLoader) (connection.EC2ClientInterface, error) {
					return m.ec2Client, nil
				}
				m.ec2Client.EXPECT().ListBastionInstances(ctx).Return(nil, errors.New("AWS error"))
				m.prompter.EXPECT().PromptForInstanceID().Return("invalid-id", nil)
			},
			expectedError: "invalid instance ID format",
		},
		{
			name:          "Error getting manual instance ID",
			awsConfigured: true,
			setupMocks: func(provider *connection.ConnectionProvider) {
				m.configLoader.EXPECT().LoadDefaultConfig(gomock.Any()).Return(aws.Config{Region: "us-west-2"}, nil).AnyTimes()
				m.prompter.EXPECT().PromptForRegion("us-west-2").Return("us-west-2", nil)
				provider.NewEC2Client = func(region string, loader connection.AWSConfigLoader) (connection.EC2ClientInterface, error) {
					return m.ec2Client, nil
				}
				m.ec2Client.EXPECT().ListBastionInstances(ctx).Return(nil, errors.New("AWS error"))
				m.prompter.EXPECT().PromptForInstanceID().Return("", errors.New("user canceled"))
			},
			expectedError: "failed to get instance ID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var provider *connection.ConnectionProvider
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
				provider = connection.NewConnectionProvider(m.prompter, m.fs, awsConfig, m.ec2Client, m.ssmClient, m.instanceConn, m.configLoader)
			} else {
				provider = connection.NewConnectionProvider(m.prompter, m.fs, aws.Config{}, nil, nil, nil, m.configLoader)
			}

			tt.setupMocks(provider)

			details, err := provider.GetSSMDetails(ctx)
			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
				assert.Nil(t, details)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, details)
			}
		})
	}
}

func TestGetBastionInstanceID(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name           string
		isSSM          bool
		awsConfig      aws.Config
		setupMocks     func(*testing.T, mocks, *connection.ConnectionProvider)
		expected       string
		expectedError  string
		stdoutContains string
	}{
		{
			name:  "Success with SSM",
			isSSM: true,
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
			setupMocks: func(t *testing.T, m mocks, provider *connection.ConnectionProvider) {
				m.prompter.EXPECT().PromptForRegion("us-west-2").Return("us-west-2", nil)
				provider.NewEC2Client = func(region string, loader connection.AWSConfigLoader) (connection.EC2ClientInterface, error) {
					return m.ec2Client, nil
				}
				m.ec2Client.EXPECT().ListBastionInstances(ctx).Return([]models.EC2Instance{
					{InstanceID: "i-1234567890abcdef0", Name: "bastion-1"},
				}, nil)
				m.prompter.EXPECT().PromptForBastionInstance(gomock.Any(), true).Return("i-1234567890abcdef0", nil)
			},
			expected: "i-1234567890abcdef0",
		},
		{
			name:  "Success without SSM",
			isSSM: false,
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
			setupMocks: func(t *testing.T, m mocks, provider *connection.ConnectionProvider) {
				m.prompter.EXPECT().PromptForRegion("us-west-2").Return("us-west-2", nil)
				provider.NewEC2Client = func(region string, loader connection.AWSConfigLoader) (connection.EC2ClientInterface, error) {
					return m.ec2Client, nil
				}
				m.ec2Client.EXPECT().ListBastionInstances(ctx).Return([]models.EC2Instance{
					{InstanceID: "i-1234567890abcdef0", Name: "bastion-1"},
				}, nil)
				m.prompter.EXPECT().PromptForBastionInstance(gomock.Any(), false).Return("i-1234567890abcdef0", nil)
			},
			expected: "i-1234567890abcdef0",
		},
		{
			name:  "Error loading default region",
			isSSM: true,
			awsConfig: aws.Config{
				Credentials: credentials.StaticCredentialsProvider{
					Value: aws.Credentials{
						AccessKeyID:     "mock-access-key",
						SecretAccessKey: "mock-secret-key",
						Source:          "test",
					},
				},
			},
			setupMocks: func(t *testing.T, m mocks, provider *connection.ConnectionProvider) {
				m.configLoader.EXPECT().LoadDefaultConfig(gomock.Any()).Return(aws.Config{}, errors.New("config load error")).Do(func(_ context.Context) {
					t.Log("LoadDefaultConfig called, returning error")
				})
				m.prompter.EXPECT().PromptForRegion("").Return("us-west-2", nil)
				provider.NewEC2Client = func(region string, loader connection.AWSConfigLoader) (connection.EC2ClientInterface, error) {
					return m.ec2Client, nil
				}
				m.ec2Client.EXPECT().ListBastionInstances(ctx).Return([]models.EC2Instance{
					{InstanceID: "i-1234567890abcdef0", Name: "bastion-1"},
				}, nil)
				m.prompter.EXPECT().PromptForBastionInstance(gomock.Any(), true).Return("i-1234567890abcdef0", nil)
			},
			expected:       "i-1234567890abcdef0",
			stdoutContains: "Failed to load default region: failed to load AWS configuration: config load error",
		},
		{
			name:  "Error prompting for region",
			isSSM: true,
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
			setupMocks: func(t *testing.T, m mocks, provider *connection.ConnectionProvider) {
				m.prompter.EXPECT().PromptForRegion("us-west-2").Return("", errors.New("region prompt failed"))
			},
			expectedError: "failed to get region: region prompt failed",
		},
		{
			name:  "Error initializing EC2 client",
			isSSM: true,
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
			setupMocks: func(t *testing.T, m mocks, provider *connection.ConnectionProvider) {
				m.prompter.EXPECT().PromptForRegion("us-west-2").Return("us-west-2", nil)
				provider.NewEC2Client = func(region string, loader connection.AWSConfigLoader) (connection.EC2ClientInterface, error) {
					return nil, errors.New("client creation error")
				}
			},
			expectedError: "failed to initialize EC2 client: client creation error",
		},
		{
			name:  "AWS lookup failed",
			isSSM: true,
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
			setupMocks: func(t *testing.T, m mocks, provider *connection.ConnectionProvider) {
				m.prompter.EXPECT().PromptForRegion("us-west-2").Return("us-west-2", nil)
				provider.NewEC2Client = func(region string, loader connection.AWSConfigLoader) (connection.EC2ClientInterface, error) {
					return m.ec2Client, nil
				}
				m.ec2Client.EXPECT().ListBastionInstances(ctx).Return(nil, errors.New("aws error"))
			},
			expectedError: "AWS lookup failed: aws error",
		},
		{
			name:  "No bastion hosts found",
			isSSM: true,
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
			setupMocks: func(t *testing.T, m mocks, provider *connection.ConnectionProvider) {
				m.prompter.EXPECT().PromptForRegion("us-west-2").Return("us-west-2", nil)
				provider.NewEC2Client = func(region string, loader connection.AWSConfigLoader) (connection.EC2ClientInterface, error) {
					return m.ec2Client, nil
				}
				m.ec2Client.EXPECT().ListBastionInstances(ctx).Return([]models.EC2Instance{}, nil)
			},
			expectedError: "no bastion hosts found in AWS",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := setupMocks(t)
			defer m.ctrl.Finish()

			provider := connection.NewConnectionProvider(m.prompter, m.fs, tt.awsConfig, m.ec2Client, m.ssmClient, m.instanceConn, m.configLoader)

			if tt.stdoutContains != "" {
				r, w, err := os.Pipe()
				if err != nil {
					t.Fatalf("failed to create pipe: %v", err)
				}
				defer func() {
					if err := r.Close(); err != nil {
						t.Logf("failed to close pipe reader: %v", err)
					}
				}()

				oldStdout := os.Stdout
				os.Stdout = w
				defer func() { os.Stdout = oldStdout }()

				outputChan := make(chan string)

				go func() {
					var buf bytes.Buffer
					if _, err := io.Copy(&buf, r); err != nil {
						t.Logf("error copying from pipe: %v", err)
					}
					outputChan <- buf.String()
				}()

				defer func() {
					_ = w.Close()
					output := <-outputChan
					t.Logf("Captured stdout: %q", output)
					assert.Contains(t, output, tt.stdoutContains)
				}()
			}

			tt.setupMocks(t, m, provider)

			instanceID, err := provider.GetBastionInstanceID(ctx, tt.isSSM)
			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
				assert.Empty(t, instanceID)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, instanceID)
			}
		})
	}
}
func TestGetSSHDetails_GetAZError(t *testing.T) {
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
	provider := connection.NewConnectionProvider(m.prompter, m.fs, awsConfig, m.ec2Client, m.ssmClient, m.instanceConn, nil)

	ctx := context.Background()

	m.prompter.EXPECT().ChooseConnectionMethod().Return(connection.MethodSSH, nil)
	m.prompter.EXPECT().PromptForConfirmation("Look for bastion hosts in AWS?").Return(false, nil)
	m.prompter.EXPECT().PromptForBastionHost().Return("i-1234567890abcdef0", nil)
	m.prompter.EXPECT().PromptForSSHUser("ec2-user").Return("ec2-user", nil)

	publicIP := "1.2.3.4"
	m.ec2Client.EXPECT().DescribeInstances(gomock.Any(), gomock.Any()).Return(&ec2.DescribeInstancesOutput{
		Reservations: []types.Reservation{
			{
				Instances: []types.Instance{
					{
						InstanceId:      aws.String("i-1234567890abcdef0"),
						PublicIpAddress: &publicIP,
						State: &types.InstanceState{
							Name: types.InstanceStateNameRunning,
						},
					},
				},
			},
		},
	}, nil)

	m.ec2Client.EXPECT().DescribeInstances(gomock.Any(), gomock.Any()).Return(nil, errors.New("describe error"))

	_, err := provider.GetConnectionDetails(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get availability zone")
}

func TestGetSSHDetails_SendPublicKeyError(t *testing.T) {
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
	provider := connection.NewConnectionProvider(m.prompter, m.fs, awsConfig, m.ec2Client, m.ssmClient, m.instanceConn, nil)

	ctx := context.Background()

	m.prompter.EXPECT().ChooseConnectionMethod().Return(connection.MethodSSH, nil)
	m.prompter.EXPECT().PromptForConfirmation("Look for bastion hosts in AWS?").Return(false, nil)
	m.prompter.EXPECT().PromptForBastionHost().Return("i-1234567890abcdef0", nil)
	m.prompter.EXPECT().PromptForSSHUser("ec2-user").Return("ec2-user", nil)

	publicIP := "1.2.3.4"
	m.ec2Client.EXPECT().DescribeInstances(gomock.Any(), gomock.Any()).Return(&ec2.DescribeInstancesOutput{
		Reservations: []types.Reservation{
			{
				Instances: []types.Instance{
					{
						InstanceId:      aws.String("i-1234567890abcdef0"),
						PublicIpAddress: &publicIP,
						State: &types.InstanceState{
							Name: types.InstanceStateNameRunning,
						},
					},
				},
			},
		},
	}, nil)

	az := "us-west-2a"
	m.ec2Client.EXPECT().DescribeInstances(gomock.Any(), gomock.Any()).Return(&ec2.DescribeInstancesOutput{
		Reservations: []types.Reservation{
			{
				Instances: []types.Instance{
					{
						InstanceId:      aws.String("i-1234567890abcdef0"),
						Placement:       &types.Placement{AvailabilityZone: &az},
						PublicIpAddress: &publicIP,
						State:           &types.InstanceState{Name: types.InstanceStateNameRunning},
					},
				},
			},
		},
	}, nil)

	m.instanceConn.EXPECT().SendSSHPublicKey(gomock.Any(), gomock.Any()).Return(nil, errors.New("send key error"))

	_, err := provider.GetConnectionDetails(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to send SSH public key")
}

func TestGetInstanceAvailabilityZone_EC2ClientNotInitialized(t *testing.T) {
	m := setupMocks(t)
	defer m.ctrl.Finish()

	provider := connection.NewConnectionProvider(m.prompter, m.fs, aws.Config{}, nil, nil, nil, nil)

	_, err := provider.GetInstanceAvailabilityZone(context.Background(), "i-1234567890abcdef0")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "EC2 client not initialized")
}

func TestGetInstanceAvailabilityZone_InstanceNotFound(t *testing.T) {
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
	provider := connection.NewConnectionProvider(m.prompter, m.fs, awsConfig, m.ec2Client, m.ssmClient, m.instanceConn, nil)

	m.ec2Client.EXPECT().DescribeInstances(gomock.Any(), gomock.Any()).Return(&ec2.DescribeInstancesOutput{
		Reservations: []types.Reservation{},
	}, nil)

	_, err := provider.GetInstanceAvailabilityZone(context.Background(), "i-1234567890abcdef0")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "instance i-1234567890abcdef0 not found")
}

func TestGetInstanceAvailabilityZone_AZNotFound(t *testing.T) {
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
	provider := connection.NewConnectionProvider(m.prompter, m.fs, awsConfig, m.ec2Client, m.ssmClient, m.instanceConn, nil)

	m.ec2Client.EXPECT().DescribeInstances(gomock.Any(), gomock.Any()).Return(&ec2.DescribeInstancesOutput{
		Reservations: []types.Reservation{
			{
				Instances: []types.Instance{
					{
						InstanceId: aws.String("i-1234567890abcdef0"),
						Placement:  nil,
					},
				},
			},
		},
	}, nil)

	_, err := provider.GetInstanceAvailabilityZone(context.Background(), "i-1234567890abcdef0")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "availability zone not found")
}

func TestGetInstanceDetails_EC2ClientNotInitialized(t *testing.T) {
	m := setupMocks(t)
	defer m.ctrl.Finish()

	provider := connection.NewConnectionProvider(m.prompter, m.fs, aws.Config{}, nil, nil, nil, nil)

	_, err := provider.GetInstanceDetails(context.Background(), "i-1234567890abcdef0")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "EC2 client not initialized")
}

func TestGetInstanceDetails_DescribeError(t *testing.T) {
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
	provider := connection.NewConnectionProvider(m.prompter, m.fs, awsConfig, m.ec2Client, m.ssmClient, m.instanceConn, nil)

	m.ec2Client.EXPECT().DescribeInstances(gomock.Any(), gomock.Any()).Return(nil, errors.New("describe error"))

	_, err := provider.GetInstanceDetails(context.Background(), "i-1234567890abcdef0")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to describe instance")
}

func TestGetInstanceDetails_InstanceNotFound(t *testing.T) {
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
	provider := connection.NewConnectionProvider(m.prompter, m.fs, awsConfig, m.ec2Client, m.ssmClient, m.instanceConn, nil)

	m.ec2Client.EXPECT().DescribeInstances(gomock.Any(), gomock.Any()).Return(&ec2.DescribeInstancesOutput{
		Reservations: []types.Reservation{},
	}, nil)

	_, err := provider.GetInstanceDetails(context.Background(), "i-1234567890abcdef0")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "instance i-1234567890abcdef0 not found")
}
