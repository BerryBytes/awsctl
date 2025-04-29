package connection_test

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

	connection "github.com/BerryBytes/awsctl/internal/common"
	"github.com/BerryBytes/awsctl/models"
	mock_awsctl "github.com/BerryBytes/awsctl/tests/mock"
	"github.com/BerryBytes/awsctl/utils/common"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2instanceconnect"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

type serviceMocks struct {
	ctrl            *gomock.Controller
	prompter        *mock_awsctl.MockConnectionPrompter
	fs              *mock_awsctl.MockFileSystemInterface
	ec2Client       *mock_awsctl.MockEC2ClientInterface
	instanceConn    *mock_awsctl.MockEC2InstanceConnectInterface
	ssmClient       *mock_awsctl.MockSSMClientInterface
	executor        *mock_awsctl.MockSSHExecutorInterface
	osDetector      *mock_awsctl.MockOSDetector
	ssmStarter      *mock_awsctl.MockSSMStarterInterface
	configLoader    *mock_awsctl.MockAWSConfigLoader
	commandExecutor *mock_awsctl.MockCommandExecutor
}

func setupServiceMocks(t *testing.T) serviceMocks {
	ctrl := gomock.NewController(t)
	return serviceMocks{
		ctrl:            ctrl,
		prompter:        mock_awsctl.NewMockConnectionPrompter(ctrl),
		fs:              mock_awsctl.NewMockFileSystemInterface(ctrl),
		ec2Client:       mock_awsctl.NewMockEC2ClientInterface(ctrl),
		instanceConn:    mock_awsctl.NewMockEC2InstanceConnectInterface(ctrl),
		ssmClient:       mock_awsctl.NewMockSSMClientInterface(ctrl),
		executor:        mock_awsctl.NewMockSSHExecutorInterface(ctrl),
		osDetector:      mock_awsctl.NewMockOSDetector(ctrl),
		ssmStarter:      mock_awsctl.NewMockSSMStarterInterface(ctrl),
		configLoader:    mock_awsctl.NewMockAWSConfigLoader(ctrl),
		commandExecutor: mock_awsctl.NewMockCommandExecutor(ctrl),
	}
}

func TestStartSOCKSProxy_SSM_Success(t *testing.T) {
	m := setupServiceMocks(t)
	defer m.ctrl.Finish()

	credProvider := credentials.StaticCredentialsProvider{
		Value: aws.Credentials{AccessKeyID: "mock-access-key", SecretAccessKey: "mock-secret-key", Source: "test"},
	}
	awsConfig := aws.Config{Region: "us-west-2", Credentials: credProvider}
	provider := connection.NewConnectionProvider(m.prompter, m.fs, awsConfig, m.ec2Client, m.ssmClient, m.instanceConn, m.configLoader)
	provider.NewEC2Client = func(region string, loader connection.AWSConfigLoader) (connection.EC2ClientInterface, error) {
		return m.ec2Client, nil
	}
	services := &connection.Services{
		Provider:   provider,
		Executor:   m.executor,
		OsDetector: m.osDetector,
		SsmStarter: m.ssmStarter,
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

	m.prompter.EXPECT().ChooseConnectionMethod().Return(connection.MethodSSM, nil)
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
	provider := connection.NewConnectionProvider(m.prompter, m.fs, awsConfig, m.ec2Client, m.ssmClient, m.instanceConn, m.configLoader)
	provider.NewEC2Client = func(region string, loader connection.AWSConfigLoader) (connection.EC2ClientInterface, error) {
		return m.ec2Client, nil
	}
	services := &connection.Services{
		Provider:   provider,
		Executor:   m.executor,
		OsDetector: m.osDetector,
		SsmStarter: m.ssmStarter,
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

	m.prompter.EXPECT().ChooseConnectionMethod().Return(connection.MethodSSM, nil)
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
	provider := connection.NewConnectionProvider(m.prompter, m.fs, awsConfig, m.ec2Client, m.ssmClient, m.instanceConn, m.configLoader)
	provider.NewEC2Client = func(region string, loader connection.AWSConfigLoader) (connection.EC2ClientInterface, error) {
		return m.ec2Client, nil
	}
	services := &connection.Services{
		Provider:   provider,
		Executor:   m.executor,
		OsDetector: m.osDetector,
		SsmStarter: m.ssmStarter,
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

	m.prompter.EXPECT().ChooseConnectionMethod().Return(connection.MethodSSM, nil)
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
	provider := connection.NewConnectionProvider(m.prompter, m.fs, awsConfig, m.ec2Client, m.ssmClient, m.instanceConn, nil)

	services := connection.NewServices(provider)
	assert.NotNil(t, services)
	assert.Equal(t, provider, services.Provider)
	assert.IsType(t, &common.RealSSHExecutor{}, services.Executor)
	assert.IsType(t, common.RuntimeOSDetector{}, services.OsDetector)
}

func TestSSHIntoBastion_Success(t *testing.T) {
	m := setupServiceMocks(t)
	defer m.ctrl.Finish()

	credProvider := credentials.StaticCredentialsProvider{
		Value: aws.Credentials{AccessKeyID: "mock-access-key", SecretAccessKey: "mock-secret-key", Source: "test"},
	}
	awsConfig := aws.Config{Region: "us-west-2", Credentials: credProvider}
	provider := connection.NewConnectionProvider(m.prompter, m.fs, awsConfig, m.ec2Client, m.ssmClient, m.instanceConn, nil)
	services := &connection.Services{
		Provider:   provider,
		Executor:   m.executor,
		OsDetector: m.osDetector,
		SsmStarter: m.ssmStarter,
	}

	ctx := context.Background()
	homeDir, _ := os.UserHomeDir()
	keyPath := filepath.Join(homeDir, ".ssh/id_ed25519")

	m.prompter.EXPECT().ChooseConnectionMethod().Return(connection.MethodSSH, nil)
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

	provider := connection.NewConnectionProvider(m.prompter, m.fs, aws.Config{}, m.ec2Client, m.ssmClient, m.instanceConn, nil)
	services := &connection.Services{
		Provider:   provider,
		Executor:   m.executor,
		OsDetector: m.osDetector,
		SsmStarter: m.ssmStarter}

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
	provider := connection.NewConnectionProvider(m.prompter, m.fs, awsConfig, m.ec2Client, m.ssmClient, m.instanceConn, nil)
	services := &connection.Services{
		Provider:   provider,
		Executor:   m.executor,
		OsDetector: m.osDetector,
		SsmStarter: m.ssmStarter,
	}

	ctx := context.Background()
	homeDir, _ := os.UserHomeDir()
	keyPath := filepath.Join(homeDir, ".ssh/id_ed25519")

	m.prompter.EXPECT().ChooseConnectionMethod().Return(connection.MethodSSH, nil)
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

	provider := connection.NewConnectionProvider(m.prompter, m.fs, aws.Config{}, m.ec2Client, m.ssmClient, m.instanceConn, nil)
	services := &connection.Services{
		Provider:   provider,
		Executor:   m.executor,
		OsDetector: m.osDetector,
		SsmStarter: m.ssmStarter,
	}

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
	provider := connection.NewConnectionProvider(m.prompter, m.fs, awsConfig, m.ec2Client, m.ssmClient, m.instanceConn, nil)
	services := &connection.Services{
		Provider:   provider,
		Executor:   m.executor,
		OsDetector: m.osDetector,
		SsmStarter: m.ssmStarter,
	}

	ctx := context.Background()
	homeDir, _ := os.UserHomeDir()
	keyPath := filepath.Join(homeDir, ".ssh/id_ed25519")
	port := 1080

	m.prompter.EXPECT().ChooseConnectionMethod().Return(connection.MethodSSH, nil)
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
	provider := connection.NewConnectionProvider(m.prompter, m.fs, awsConfig, m.ec2Client, m.ssmClient, m.instanceConn, nil)
	services := &connection.Services{
		Provider:   provider,
		Executor:   m.executor,
		OsDetector: m.osDetector,
		SsmStarter: m.ssmStarter,
	}

	ctx := context.Background()
	homeDir, _ := os.UserHomeDir()
	keyPath := filepath.Join(homeDir, ".ssh/id_ed25519")
	port := 1080

	m.prompter.EXPECT().ChooseConnectionMethod().Return(connection.MethodSSH, nil)
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
	provider := connection.NewConnectionProvider(m.prompter, m.fs, awsConfig, m.ec2Client, m.ssmClient, m.instanceConn, nil)
	services := &connection.Services{
		Provider:   provider,
		Executor:   m.executor,
		OsDetector: m.osDetector,
		SsmStarter: m.ssmStarter,
	}

	ctx := context.Background()
	homeDir, _ := os.UserHomeDir()
	keyPath := filepath.Join(homeDir, ".ssh/id_ed25519")
	localPort := 8080
	remoteHost := "internal.example.com"
	remotePort := 80

	m.prompter.EXPECT().ChooseConnectionMethod().Return(connection.MethodSSH, nil)
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

	provider := connection.NewConnectionProvider(m.prompter, m.fs, aws.Config{}, m.ec2Client, m.ssmClient, m.instanceConn, nil)
	services := &connection.Services{
		Provider:   provider,
		Executor:   m.executor,
		OsDetector: m.osDetector,
		SsmStarter: m.ssmStarter,
	}

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
	provider := connection.NewConnectionProvider(m.prompter, m.fs, awsConfig, m.ec2Client, m.ssmClient, m.instanceConn, nil)
	services := &connection.Services{
		Provider:   provider,
		Executor:   m.executor,
		OsDetector: m.osDetector,
		SsmStarter: m.ssmStarter,
	}

	ctx := context.Background()
	homeDir, _ := os.UserHomeDir()
	keyPath := filepath.Join(homeDir, ".ssh/id_ed25519")
	localPort := 8080
	remoteHost := "internal.example.com"
	remotePort := 80

	m.prompter.EXPECT().ChooseConnectionMethod().Return(connection.MethodSSH, nil)
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

func TestSSHIntoBastion_EC2InstanceConnect_Success(t *testing.T) {
	m := setupServiceMocks(t)
	defer m.ctrl.Finish()
	m.fs.EXPECT().Stat(gomock.Any()).Return(nil, os.ErrNotExist).AnyTimes()
	credProvider := credentials.StaticCredentialsProvider{
		Value: aws.Credentials{AccessKeyID: "mock-access-key", SecretAccessKey: "mock-secret-key", Source: "test"},
	}
	awsConfig := aws.Config{Region: "us-west-2", Credentials: credProvider}
	provider := connection.NewConnectionProvider(m.prompter, m.fs, awsConfig, m.ec2Client, m.ssmClient, m.instanceConn, m.configLoader)
	provider.NewEC2Client = func(region string, loader connection.AWSConfigLoader) (connection.EC2ClientInterface, error) {
		return m.ec2Client, nil
	}
	services := &connection.Services{
		Provider:        provider,
		Executor:        m.executor,
		OsDetector:      m.osDetector,
		SsmStarter:      m.ssmStarter,
		CommandExecutor: m.commandExecutor,
	}

	ctx := context.Background()
	instanceID := "i-1234567890abcdef0"
	user := "ec2-user"

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

	m.prompter.EXPECT().ChooseConnectionMethod().Return(connection.MethodSSH, nil)
	m.prompter.EXPECT().PromptForConfirmation("Look for bastion hosts in AWS?").Return(true, nil)
	m.prompter.EXPECT().PromptForRegion("us-west-2").Return("us-west-2", nil)
	m.ec2Client.EXPECT().ListBastionInstances(ctx).Return([]models.EC2Instance{
		{InstanceID: instanceID, Name: "bastion-1"},
	}, nil)
	m.prompter.EXPECT().PromptForBastionInstance(gomock.Any(), false).Return(instanceID, nil)
	m.prompter.EXPECT().PromptForSSHUser("ec2-user").Return(user, nil)

	m.ec2Client.EXPECT().DescribeInstances(ctx, gomock.Any()).Return(&ec2.DescribeInstancesOutput{
		Reservations: []types.Reservation{
			{
				Instances: []types.Instance{
					{
						InstanceId:      aws.String(instanceID),
						PublicIpAddress: aws.String("1.2.3.4"),
						State:           &types.InstanceState{Name: types.InstanceStateNameRunning},
						Tags:            []types.Tag{{Key: aws.String("Name"), Value: aws.String("bastion-1")}},
						MetadataOptions: &types.InstanceMetadataOptionsResponse{HttpTokens: types.HttpTokensStateRequired},
						Placement:       &types.Placement{AvailabilityZone: aws.String("us-west-2a")},
					},
				},
			},
		},
	}, nil).AnyTimes()

	m.instanceConn.EXPECT().SendSSHPublicKey(ctx, gomock.Any()).Return(&ec2instanceconnect.SendSSHPublicKeyOutput{
		Success: true,
	}, nil)

	m.commandExecutor.EXPECT().RunInteractiveCommand(
		ctx,
		"aws",
		[]string{
			"ec2-instance-connect", "ssh",
			"--instance-id", instanceID,
			"--connection-type", "eice",
		},
	).Return(nil)

	err = services.SSHIntoBastion(ctx)
	assert.NoError(t, err)

	_ = w.Sync()
	_ = w.Close()
	<-done

	output := buf.String()
	t.Logf("Captured stdout: %q", output)
	assert.Contains(t, output, "Using EC2 Instance Connect Endpoint (EIC-E) for authentication")
	assert.Contains(t, output, fmt.Sprintf("Connecting to %s@%s using EC2 Instance Connect...\n", user, instanceID))
}

func TestIsAWSConfigured(t *testing.T) {
	tests := []struct {
		name           string
		setupProvider  func(*connection.ConnectionProvider)
		expectedResult bool
	}{
		{
			name: "fully configured AWS credentials",
			setupProvider: func(p *connection.ConnectionProvider) {
				p.AwsConfig = aws.Config{
					Credentials: credentials.StaticCredentialsProvider{
						Value: aws.Credentials{
							AccessKeyID:     "AKIAEXAMPLE",
							SecretAccessKey: "secret",
							Source:          "test",
						},
					},
				}
			},
			expectedResult: true,
		},
		{
			name: "nil provider",
			setupProvider: func(p *connection.ConnectionProvider) {
			},
			expectedResult: false,
		},
		{
			name: "nil credentials",
			setupProvider: func(p *connection.ConnectionProvider) {
				p.AwsConfig = aws.Config{
					Credentials: nil,
				}
			},
			expectedResult: false,
		},
		{
			name: "empty credentials",
			setupProvider: func(p *connection.ConnectionProvider) {
				p.AwsConfig = aws.Config{
					Credentials: credentials.StaticCredentialsProvider{
						Value: aws.Credentials{},
					},
				}
			},
			expectedResult: false,
		},

		{
			name: "missing access key",
			setupProvider: func(p *connection.ConnectionProvider) {
				p.AwsConfig = aws.Config{
					Credentials: credentials.StaticCredentialsProvider{
						Value: aws.Credentials{
							SecretAccessKey: "secret",
							Source:          "test",
						},
					},
				}
			},
			expectedResult: false,
		},
		{
			name: "missing secret key",
			setupProvider: func(p *connection.ConnectionProvider) {
				p.AwsConfig = aws.Config{
					Credentials: credentials.StaticCredentialsProvider{
						Value: aws.Credentials{
							AccessKeyID: "AKIAEXAMPLE",
							Source:      "test",
						},
					},
				}
			},
			expectedResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			var provider *connection.ConnectionProvider
			if tt.name != "nil provider" {
				m := setupServiceMocks(t)
				defer m.ctrl.Finish()

				provider = connection.NewConnectionProvider(
					m.prompter,
					m.fs,
					aws.Config{},
					m.ec2Client,
					m.ssmClient,
					m.instanceConn,
					m.configLoader,
				)

				if tt.setupProvider != nil {
					tt.setupProvider(provider)
				}
			}

			var service *connection.Services
			if tt.name != "nil services" {
				service = &connection.Services{
					Provider: provider,
				}
			}

			result := service.IsAWSConfigured()
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}
