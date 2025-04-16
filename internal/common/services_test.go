package connection

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	mock_awsctl "github.com/BerryBytes/awsctl/tests/mock"
	"github.com/BerryBytes/awsctl/utils/common"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

type serviceMocks struct {
	ctrl         *gomock.Controller
	prompter     *mock_awsctl.MockConnectionPrompter
	fs           *mock_awsctl.MockFileSystemInterface
	ec2Client    *mock_awsctl.MockEC2ClientInterface
	instanceConn *mock_awsctl.MockEC2InstanceConnectInterface
	ssmClient    *mock_awsctl.MockSSMClientInterface
	executor     *mock_awsctl.MockSSHExecutorInterface
	osDetector   *mock_awsctl.MockOSDetector
}

func setupServiceMocks(t *testing.T) serviceMocks {
	ctrl := gomock.NewController(t)
	return serviceMocks{
		ctrl:         ctrl,
		prompter:     mock_awsctl.NewMockConnectionPrompter(ctrl),
		fs:           mock_awsctl.NewMockFileSystemInterface(ctrl),
		ec2Client:    mock_awsctl.NewMockEC2ClientInterface(ctrl),
		instanceConn: mock_awsctl.NewMockEC2InstanceConnectInterface(ctrl),
		ssmClient:    mock_awsctl.NewMockSSMClientInterface(ctrl),
		executor:     mock_awsctl.NewMockSSHExecutorInterface(ctrl),
		osDetector:   mock_awsctl.NewMockOSDetector(ctrl),
	}
}

func TestNewServices(t *testing.T) {
	m := setupServiceMocks(t)
	defer m.ctrl.Finish()

	credProvider := credentials.StaticCredentialsProvider{
		Value: aws.Credentials{AccessKeyID: "mock-access-key", SecretAccessKey: "mock-secret-key", Source: "test"},
	}
	awsConfig := aws.Config{Region: "us-west-2", Credentials: credProvider}
	provider := NewConnectionProvider(m.prompter, m.fs, awsConfig, m.ec2Client, m.ssmClient, m.instanceConn, nil)

	services := NewServices(provider)
	assert.NotNil(t, services)
	assert.Equal(t, provider, services.provider)
	assert.IsType(t, &common.RealSSHExecutor{}, services.executor)
	assert.IsType(t, common.RuntimeOSDetector{}, services.osDetector)
}

func TestSSHIntoBastion_Success(t *testing.T) {
	m := setupServiceMocks(t)
	defer m.ctrl.Finish()

	credProvider := credentials.StaticCredentialsProvider{
		Value: aws.Credentials{AccessKeyID: "mock-access-key", SecretAccessKey: "mock-secret-key", Source: "test"},
	}
	awsConfig := aws.Config{Region: "us-west-2", Credentials: credProvider}
	provider := NewConnectionProvider(m.prompter, m.fs, awsConfig, m.ec2Client, m.ssmClient, m.instanceConn, nil)
	services := &Services{provider: provider, executor: m.executor, osDetector: m.osDetector}

	ctx := context.Background()
	homeDir, _ := os.UserHomeDir()
	keyPath := filepath.Join(homeDir, ".ssh/id_ed25519")

	m.prompter.EXPECT().ChooseConnectionMethod().Return(MethodSSH, nil)
	m.prompter.EXPECT().PromptForConfirmation("Look for bastion hosts in AWS?").Return(false, nil)
	m.prompter.EXPECT().PromptForBastionHost().Return("bastion.example.com", nil)
	m.prompter.EXPECT().PromptForSSHUser("ec2-user").Return("ec2-user", nil)
	m.prompter.EXPECT().PromptForSSHKeyPath("~/.ssh/id_ed25519").Return("~/.ssh/id_ed25519", nil)
	pubKeyInfo := &mockFileInfo{name: "id_ed25519.pub", size: 68, mode: 0644, modTime: time.Now(), isDir: false}
	m.fs.EXPECT().Stat(keyPath+".pub").Return(pubKeyInfo, nil).AnyTimes()
	privateKeyInfo := &mockFileInfo{name: "id_ed25519", size: 464, mode: 0600, modTime: time.Now(), isDir: false}
	m.fs.EXPECT().Stat(keyPath).Return(privateKeyInfo, nil)
	m.fs.EXPECT().ReadFile(keyPath).Return([]byte("-----BEGIN OPENSSH PRIVATE KEY-----\n..."), nil)

	m.executor.EXPECT().Execute(
		[]string{
			"ssh",
			"-i", keyPath,
			"-o", "BatchMode=no",
			"-o", "ConnectTimeout=30",
			"-o", "StrictHostKeyChecking=yes",
			"-o", "ServerAliveInterval=60",
			"ec2-user@bastion.example.com",
		},
		gomock.Any(), gomock.Any(), gomock.Any(),
	).Return(nil)

	err := services.SSHIntoBastion(ctx)
	assert.NoError(t, err)
}

func TestSSHIntoBastion_GetDetailsFails(t *testing.T) {
	m := setupServiceMocks(t)
	defer m.ctrl.Finish()

	provider := NewConnectionProvider(m.prompter, m.fs, aws.Config{}, m.ec2Client, m.ssmClient, m.instanceConn, nil)
	services := &Services{provider: provider, executor: m.executor, osDetector: m.osDetector}

	ctx := context.Background()

	m.prompter.EXPECT().ChooseConnectionMethod().Return("", errors.New("method failed"))

	err := services.SSHIntoBastion(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get connection details: failed to select connection method: method failed")
}

func TestSSHIntoBastion_ExecuteFails(t *testing.T) {
	m := setupServiceMocks(t)
	defer m.ctrl.Finish()

	credProvider := credentials.StaticCredentialsProvider{
		Value: aws.Credentials{AccessKeyID: "mock-access-key", SecretAccessKey: "mock-secret-key", Source: "test"},
	}
	awsConfig := aws.Config{Region: "us-west-2", Credentials: credProvider}
	provider := NewConnectionProvider(m.prompter, m.fs, awsConfig, m.ec2Client, m.ssmClient, m.instanceConn, nil)
	services := &Services{provider: provider, executor: m.executor, osDetector: m.osDetector}

	ctx := context.Background()
	homeDir, _ := os.UserHomeDir()
	keyPath := filepath.Join(homeDir, ".ssh/id_ed25519")

	m.prompter.EXPECT().ChooseConnectionMethod().Return(MethodSSH, nil)
	m.prompter.EXPECT().PromptForConfirmation("Look for bastion hosts in AWS?").Return(false, nil)
	m.prompter.EXPECT().PromptForBastionHost().Return("bastion.example.com", nil)
	m.prompter.EXPECT().PromptForSSHUser("ec2-user").Return("ec2-user", nil)
	m.prompter.EXPECT().PromptForSSHKeyPath("~/.ssh/id_ed25519").Return("~/.ssh/id_ed25519", nil)
	pubKeyInfo := &mockFileInfo{name: "id_ed25519.pub", size: 68, mode: 0644, modTime: time.Now(), isDir: false}
	m.fs.EXPECT().Stat(keyPath+".pub").Return(pubKeyInfo, nil).AnyTimes()
	privateKeyInfo := &mockFileInfo{name: "id_ed25519", size: 464, mode: 0600, modTime: time.Now(), isDir: false}
	m.fs.EXPECT().Stat(keyPath).Return(privateKeyInfo, nil)
	m.fs.EXPECT().ReadFile(keyPath).Return([]byte("-----BEGIN OPENSSH PRIVATE KEY-----\n..."), nil)

	m.executor.EXPECT().Execute(
		[]string{
			"ssh",
			"-i", keyPath,
			"-o", "BatchMode=no",
			"-o", "ConnectTimeout=30",
			"-o", "StrictHostKeyChecking=yes",
			"-o", "ServerAliveInterval=60",
			"ec2-user@bastion.example.com",
		},
		gomock.Any(), gomock.Any(), gomock.Any(),
	).Return(errors.New("ssh failed"))

	err := services.SSHIntoBastion(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ssh failed")
}

func TestStartSOCKSProxy_GetDetailsFails(t *testing.T) {
	m := setupServiceMocks(t)
	defer m.ctrl.Finish()

	provider := NewConnectionProvider(m.prompter, m.fs, aws.Config{}, m.ec2Client, m.ssmClient, m.instanceConn, nil)
	services := &Services{provider: provider, executor: m.executor, osDetector: m.osDetector}

	ctx := context.Background()
	port := 1080

	m.prompter.EXPECT().ChooseConnectionMethod().Return("", errors.New("method failed"))

	err := services.StartSOCKSProxy(ctx, port)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get connection details: failed to select connection method: method failed")
}

func TestStartSOCKSProxy_Success(t *testing.T) {
	m := setupServiceMocks(t)
	defer m.ctrl.Finish()

	credProvider := credentials.StaticCredentialsProvider{
		Value: aws.Credentials{AccessKeyID: "mock-access-key", SecretAccessKey: "mock-secret-key", Source: "test"},
	}
	awsConfig := aws.Config{Region: "us-west-2", Credentials: credProvider}
	provider := NewConnectionProvider(m.prompter, m.fs, awsConfig, m.ec2Client, m.ssmClient, m.instanceConn, nil)
	services := &Services{provider: provider, executor: m.executor, osDetector: m.osDetector}

	ctx := context.Background()
	homeDir, _ := os.UserHomeDir()
	keyPath := filepath.Join(homeDir, ".ssh/id_ed25519")
	port := 1080

	m.prompter.EXPECT().ChooseConnectionMethod().Return(MethodSSH, nil)
	m.prompter.EXPECT().PromptForConfirmation("Look for bastion hosts in AWS?").Return(false, nil)
	m.prompter.EXPECT().PromptForBastionHost().Return("bastion.example.com", nil)
	m.prompter.EXPECT().PromptForSSHUser("ec2-user").Return("ec2-user", nil)
	m.prompter.EXPECT().PromptForSSHKeyPath("~/.ssh/id_ed25519").Return("~/.ssh/id_ed25519", nil)
	pubKeyInfo := &mockFileInfo{name: "id_ed25519.pub", size: 68, mode: 0644, modTime: time.Now(), isDir: false}
	m.fs.EXPECT().Stat(keyPath+".pub").Return(pubKeyInfo, nil).AnyTimes()
	privateKeyInfo := &mockFileInfo{name: "id_ed25519", size: 464, mode: 0600, modTime: time.Now(), isDir: false}
	m.fs.EXPECT().Stat(keyPath).Return(privateKeyInfo, nil)
	m.fs.EXPECT().ReadFile(keyPath).Return([]byte("-----BEGIN OPENSSH PRIVATE KEY-----\n..."), nil)

	m.executor.EXPECT().Execute(
		[]string{
			"ssh",
			"-i", keyPath,
			"-o", "BatchMode=no",
			"-o", "ConnectTimeout=30",
			"-o", "StrictHostKeyChecking=yes",
			"-o", "ServerAliveInterval=60",
			"-N", "-T", "-D", "1080",
			"ec2-user@bastion.example.com",
		},
		gomock.Any(), gomock.Any(), gomock.Any(),
	).Return(nil)

	err := services.StartSOCKSProxy(ctx, port)
	assert.NoError(t, err)
}

func TestStartSOCKSProxy_ExecuteFails(t *testing.T) {
	m := setupServiceMocks(t)
	defer m.ctrl.Finish()

	credProvider := credentials.StaticCredentialsProvider{
		Value: aws.Credentials{AccessKeyID: "mock-access-key", SecretAccessKey: "mock-secret-key", Source: "test"},
	}
	awsConfig := aws.Config{Region: "us-west-2", Credentials: credProvider}
	provider := NewConnectionProvider(m.prompter, m.fs, awsConfig, m.ec2Client, m.ssmClient, m.instanceConn, nil)
	services := &Services{provider: provider, executor: m.executor, osDetector: m.osDetector}

	ctx := context.Background()
	homeDir, _ := os.UserHomeDir()
	keyPath := filepath.Join(homeDir, ".ssh/id_ed25519")
	port := 1080

	m.prompter.EXPECT().ChooseConnectionMethod().Return(MethodSSH, nil)
	m.prompter.EXPECT().PromptForConfirmation("Look for bastion hosts in AWS?").Return(false, nil)
	m.prompter.EXPECT().PromptForBastionHost().Return("bastion.example.com", nil)
	m.prompter.EXPECT().PromptForSSHUser("ec2-user").Return("ec2-user", nil)
	m.prompter.EXPECT().PromptForSSHKeyPath("~/.ssh/id_ed25519").Return("~/.ssh/id_ed25519", nil)
	pubKeyInfo := &mockFileInfo{name: "id_ed25519.pub", size: 68, mode: 0644, modTime: time.Now(), isDir: false}
	m.fs.EXPECT().Stat(keyPath+".pub").Return(pubKeyInfo, nil).AnyTimes()
	privateKeyInfo := &mockFileInfo{name: "id_ed25519", size: 464, mode: 0600, modTime: time.Now(), isDir: false}
	m.fs.EXPECT().Stat(keyPath).Return(privateKeyInfo, nil)
	m.fs.EXPECT().ReadFile(keyPath).Return([]byte("-----BEGIN OPENSSH PRIVATE KEY-----\n..."), nil)

	m.executor.EXPECT().Execute(
		[]string{
			"ssh",
			"-i", keyPath,
			"-o", "BatchMode=no",
			"-o", "ConnectTimeout=30",
			"-o", "StrictHostKeyChecking=yes",
			"-o", "ServerAliveInterval=60",
			"-N", "-T",
			"-D", "1080",

			"ec2-user@bastion.example.com",
		},
		gomock.Any(), gomock.Any(), gomock.Any(),
	).Return(errors.New("proxy failed"))

	err := services.StartSOCKSProxy(ctx, port)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "proxy failed")
}

func TestStartPortForwarding_Success(t *testing.T) {
	m := setupServiceMocks(t)
	defer m.ctrl.Finish()

	credProvider := credentials.StaticCredentialsProvider{
		Value: aws.Credentials{AccessKeyID: "mock-access-key", SecretAccessKey: "mock-secret-key", Source: "test"},
	}
	awsConfig := aws.Config{Region: "us-west-2", Credentials: credProvider}
	provider := NewConnectionProvider(m.prompter, m.fs, awsConfig, m.ec2Client, m.ssmClient, m.instanceConn, nil)
	services := &Services{provider: provider, executor: m.executor, osDetector: m.osDetector}

	ctx := context.Background()
	homeDir, _ := os.UserHomeDir()
	keyPath := filepath.Join(homeDir, ".ssh/id_ed25519")
	localPort := 8080
	remoteHost := "internal.example.com"
	remotePort := 80

	m.prompter.EXPECT().ChooseConnectionMethod().Return(MethodSSH, nil)
	m.prompter.EXPECT().PromptForConfirmation("Look for bastion hosts in AWS?").Return(false, nil)
	m.prompter.EXPECT().PromptForBastionHost().Return("bastion.example.com", nil)
	m.prompter.EXPECT().PromptForSSHUser("ec2-user").Return("ec2-user", nil)
	m.prompter.EXPECT().PromptForSSHKeyPath("~/.ssh/id_ed25519").Return("~/.ssh/id_ed25519", nil)
	pubKeyInfo := &mockFileInfo{name: "id_ed25519.pub", size: 68, mode: 0644, modTime: time.Now(), isDir: false}
	m.fs.EXPECT().Stat(keyPath+".pub").Return(pubKeyInfo, nil).AnyTimes()
	privateKeyInfo := &mockFileInfo{name: "id_ed25519", size: 464, mode: 0600, modTime: time.Now(), isDir: false}
	m.fs.EXPECT().Stat(keyPath).Return(privateKeyInfo, nil)
	m.fs.EXPECT().ReadFile(keyPath).Return([]byte("-----BEGIN OPENSSH PRIVATE KEY-----\n..."), nil)

	m.executor.EXPECT().Execute(
		[]string{
			"ssh",
			"-i", keyPath,
			"-o", "BatchMode=no",
			"-o", "ConnectTimeout=30",
			"-o", "StrictHostKeyChecking=yes",
			"-o", "ServerAliveInterval=60",
			"-N", "-T",
			"-L", "8080:internal.example.com:80",
			"ec2-user@bastion.example.com",
		},
		gomock.Any(), gomock.Any(), gomock.Any(),
	).Return(nil)

	err := services.StartPortForwarding(ctx, localPort, remoteHost, remotePort)
	assert.NoError(t, err)
}

func TestStartPortForwarding_GetDetailsFails(t *testing.T) {
	m := setupServiceMocks(t)
	defer m.ctrl.Finish()

	provider := NewConnectionProvider(m.prompter, m.fs, aws.Config{}, m.ec2Client, m.ssmClient, m.instanceConn, nil)
	services := &Services{provider: provider, executor: m.executor, osDetector: m.osDetector}

	ctx := context.Background()
	localPort := 8080
	remoteHost := "internal.example.com"
	remotePort := 80

	m.prompter.EXPECT().ChooseConnectionMethod().Return("", errors.New("method failed"))

	err := services.StartPortForwarding(ctx, localPort, remoteHost, remotePort)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get connection details: failed to select connection method: method failed")
}

func TestStartPortForwarding_ExecuteFails(t *testing.T) {
	m := setupServiceMocks(t)
	defer m.ctrl.Finish()

	credProvider := credentials.StaticCredentialsProvider{
		Value: aws.Credentials{AccessKeyID: "mock-access-key", SecretAccessKey: "mock-secret-key", Source: "test"},
	}
	awsConfig := aws.Config{Region: "us-west-2", Credentials: credProvider}
	provider := NewConnectionProvider(m.prompter, m.fs, awsConfig, m.ec2Client, m.ssmClient, m.instanceConn, nil)
	services := &Services{provider: provider, executor: m.executor, osDetector: m.osDetector}

	ctx := context.Background()
	homeDir, _ := os.UserHomeDir()
	keyPath := filepath.Join(homeDir, ".ssh/id_ed25519")
	localPort := 8080
	remoteHost := "internal.example.com"
	remotePort := 80

	m.prompter.EXPECT().ChooseConnectionMethod().Return(MethodSSH, nil)
	m.prompter.EXPECT().PromptForConfirmation("Look for bastion hosts in AWS?").Return(false, nil)
	m.prompter.EXPECT().PromptForBastionHost().Return("bastion.example.com", nil)
	m.prompter.EXPECT().PromptForSSHUser("ec2-user").Return("ec2-user", nil)
	m.prompter.EXPECT().PromptForSSHKeyPath("~/.ssh/id_ed25519").Return("~/.ssh/id_ed25519", nil)
	pubKeyInfo := &mockFileInfo{name: "id_ed25519.pub", size: 68, mode: 0644, modTime: time.Now(), isDir: false}
	m.fs.EXPECT().Stat(keyPath+".pub").Return(pubKeyInfo, nil).AnyTimes()
	privateKeyInfo := &mockFileInfo{name: "id_ed25519", size: 464, mode: 0600, modTime: time.Now(), isDir: false}
	m.fs.EXPECT().Stat(keyPath).Return(privateKeyInfo, nil)
	m.fs.EXPECT().ReadFile(keyPath).Return([]byte("-----BEGIN OPENSSH PRIVATE KEY-----\n..."), nil)

	m.executor.EXPECT().Execute(
		[]string{
			"ssh",
			"-i", keyPath,
			"-o", "BatchMode=no",
			"-o", "ConnectTimeout=30",
			"-o", "StrictHostKeyChecking=yes",
			"-o", "ServerAliveInterval=60",
			"-N", "-T",
			"-L", "8080:internal.example.com:80",
			"ec2-user@bastion.example.com",
		},
		gomock.Any(), gomock.Any(), gomock.Any(),
	).Return(errors.New("forwarding failed"))

	err := services.StartPortForwarding(ctx, localPort, remoteHost, remotePort)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "forwarding failed")
}
