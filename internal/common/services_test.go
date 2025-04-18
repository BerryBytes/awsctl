package connection

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/BerryBytes/awsctl/models"
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
	ssmStarter   *mock_awsctl.MockSSMStarterInterface
	configLoader *mock_awsctl.MockAWSConfigLoader
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
		ssmStarter:   mock_awsctl.NewMockSSMStarterInterface(ctrl),
		configLoader: mock_awsctl.NewMockAWSConfigLoader(ctrl),
	}
}

func TestStartSOCKSProxy_SSM_Success(t *testing.T) {
	m := setupServiceMocks(t)
	defer m.ctrl.Finish()

	credProvider := credentials.StaticCredentialsProvider{
		Value: aws.Credentials{AccessKeyID: "mock-access-key", SecretAccessKey: "mock-secret-key", Source: "test"},
	}
	awsConfig := aws.Config{Region: "us-west-2", Credentials: credProvider}
	provider := NewConnectionProvider(m.prompter, m.fs, awsConfig, m.ec2Client, m.ssmClient, m.instanceConn, m.configLoader)
	provider.newEC2Client = func(region string, loader AWSConfigLoader) (EC2ClientInterface, error) {
		return m.ec2Client, nil
	}
	services := &Services{
		provider:   provider,
		executor:   m.executor,
		osDetector: m.osDetector,
		ssmStarter: m.ssmStarter,
	}

	ctx := context.Background()
	instanceID := "i-1234567890abcdef0"
	localPort := 1080

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("Failed to create pipe: %v", err)
	}
	os.Stdout = w

	var buf bytes.Buffer
	done := make(chan struct{})
	go func() {
		_, err := io.Copy(&buf, r)
		if err != nil {
			fmt.Fprintf(os.Stderr, "copy error: %v\n", err)
		}
		close(done)
	}()

	defer func() {
		_ = w.Close()
		os.Stdout = oldStdout
		<-done
	}()

	m.prompter.EXPECT().ChooseConnectionMethod().Return(MethodSSM, nil)
	m.prompter.EXPECT().PromptForRegion("us-west-2").Return("us-west-2", nil)
	m.ec2Client.EXPECT().ListBastionInstances(ctx).Return([]models.EC2Instance{
		{InstanceID: instanceID, Name: "bastion-1"},
	}, nil)
	m.prompter.EXPECT().PromptForBastionInstance(gomock.Any(), true).Return(instanceID, nil)
	m.ssmStarter.EXPECT().StartSOCKSProxy(ctx, instanceID, localPort).Return(nil)

	err = services.StartSOCKSProxy(ctx, localPort)
	assert.NoError(t, err)

	if err := w.Sync(); err != nil {
		fmt.Fprintf(os.Stderr, "sync error: %v\n", err)
	}
	_ = w.Close()
	<-done

	output := buf.String()
	t.Logf("Captured stdout: %q", output)
	assert.Contains(t, output, fmt.Sprintf("Setting up SSM SOCKS proxy on localhost:%d via instance %s...\n", localPort, instanceID))
	assert.Contains(t, output, "SOCKS proxy active. Press Ctrl+C to stop.\n")
}

func TestStartPortForwarding_SSM_Success(t *testing.T) {
	m := setupServiceMocks(t)
	defer m.ctrl.Finish()

	credProvider := credentials.StaticCredentialsProvider{
		Value: aws.Credentials{AccessKeyID: "mock-access-key", SecretAccessKey: "mock-secret-key", Source: "test"},
	}
	awsConfig := aws.Config{Region: "us-west-2", Credentials: credProvider}
	provider := NewConnectionProvider(m.prompter, m.fs, awsConfig, m.ec2Client, m.ssmClient, m.instanceConn, m.configLoader)
	provider.newEC2Client = func(region string, loader AWSConfigLoader) (EC2ClientInterface, error) {
		return m.ec2Client, nil
	}
	services := &Services{
		provider:   provider,
		executor:   m.executor,
		osDetector: m.osDetector,
		ssmStarter: m.ssmStarter,
	}

	ctx := context.Background()
	instanceID := "i-1234567890abcdef0"
	localPort := 8080
	remoteHost := "database.internal"
	remotePort := 5432

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("Failed to create pipe: %v", err)
	}
	os.Stdout = w

	var buf bytes.Buffer
	done := make(chan struct{})
	go func() {
		_, err := io.Copy(&buf, r)
		if err != nil {
			fmt.Fprintf(os.Stderr, "copy error: %v\n", err)
		}
		close(done)
	}()

	defer func() {
		_ = w.Close()
		os.Stdout = oldStdout
		<-done
	}()

	m.prompter.EXPECT().ChooseConnectionMethod().Return(MethodSSM, nil)
	m.prompter.EXPECT().PromptForRegion("us-west-2").Return("us-west-2", nil)
	m.ec2Client.EXPECT().ListBastionInstances(ctx).Return([]models.EC2Instance{
		{InstanceID: instanceID, Name: "bastion-1"},
	}, nil)
	m.prompter.EXPECT().PromptForBastionInstance(gomock.Any(), true).Return(instanceID, nil)
	m.ssmStarter.EXPECT().StartPortForwarding(ctx, instanceID, localPort, remoteHost, remotePort).Return(nil)

	err = services.StartPortForwarding(ctx, localPort, remoteHost, remotePort)
	assert.NoError(t, err)

	_ = w.Sync()
	_ = w.Close()
	<-done

	output := buf.String()
	t.Logf("Captured stdout: %q", output)
	assert.Contains(t, output, fmt.Sprintf("Setting up SSM port forwarding from localhost:%d to %s:%d via instance %s...\n",
		localPort, remoteHost, remotePort, instanceID))
	assert.Contains(t, output, "Port forwarding active. Press Ctrl+C to stop.\n")
}

func TestSSHIntoBastion_SSM_Success(t *testing.T) {
	m := setupServiceMocks(t)
	defer m.ctrl.Finish()

	credProvider := credentials.StaticCredentialsProvider{
		Value: aws.Credentials{AccessKeyID: "mock-access-key", SecretAccessKey: "mock-secret-key", Source: "test"},
	}
	awsConfig := aws.Config{Region: "us-west-2", Credentials: credProvider}
	provider := NewConnectionProvider(m.prompter, m.fs, awsConfig, m.ec2Client, m.ssmClient, m.instanceConn, m.configLoader)
	provider.newEC2Client = func(region string, loader AWSConfigLoader) (EC2ClientInterface, error) {
		return m.ec2Client, nil
	}
	services := &Services{
		provider:   provider,
		executor:   m.executor,
		osDetector: m.osDetector,
		ssmStarter: m.ssmStarter,
	}

	ctx := context.Background()
	instanceID := "i-1234567890abcdef0"

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("Failed to create pipe: %v", err)
	}
	os.Stdout = w

	var buf bytes.Buffer
	done := make(chan struct{})
	go func() {
		_, err := io.Copy(&buf, r)
		if err != nil {
			fmt.Fprintf(os.Stderr, "copy error: %v\n", err)
		}
		close(done)
	}()

	defer func() {
		_ = w.Close()
		os.Stdout = oldStdout
		<-done
	}()

	m.prompter.EXPECT().ChooseConnectionMethod().Return(MethodSSM, nil)
	m.prompter.EXPECT().PromptForRegion("us-west-2").Return("us-west-2", nil)
	m.ec2Client.EXPECT().ListBastionInstances(ctx).Return([]models.EC2Instance{
		{InstanceID: instanceID, Name: "bastion-1"},
	}, nil)
	m.prompter.EXPECT().PromptForBastionInstance(gomock.Any(), true).Return(instanceID, nil)
	m.ssmStarter.EXPECT().StartSession(ctx, instanceID).Return(nil)

	err = services.SSHIntoBastion(ctx)
	assert.NoError(t, err)

	_ = w.Sync()
	_ = w.Close()
	<-done

	output := buf.String()
	t.Logf("Captured stdout: %q", output)
	assert.Contains(t, output, fmt.Sprintf("Initiating SSM session with instance %s...\n", instanceID))
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
			"-o", "StrictHostKeyChecking=ask",
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
			"-o", "StrictHostKeyChecking=ask",
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
			"-o", "StrictHostKeyChecking=ask",
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
			"-o", "StrictHostKeyChecking=ask",
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
			"-o", "StrictHostKeyChecking=ask",
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
			"-o", "StrictHostKeyChecking=ask",
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
