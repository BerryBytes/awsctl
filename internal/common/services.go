package connection

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/BerryBytes/awsctl/utils/common"
	"github.com/spf13/afero"
)

type Services struct {
	Provider        *ConnectionProvider
	Executor        common.SSHExecutorInterface
	OsDetector      common.OSDetector
	SsmStarter      SSMStarterInterface
	CommandExecutor common.CommandExecutor
}

func NewServices(
	provider *ConnectionProvider,
) *Services {
	return &Services{
		Provider:        provider,
		Executor:        &common.RealSSHExecutor{},
		OsDetector:      common.RuntimeOSDetector{},
		SsmStarter:      NewRealSSMStarter(provider.SsmClient, provider.AwsConfig.Region),
		CommandExecutor: &common.RealCommandExecutor{},
	}
}

func (s *Services) SSHIntoBastion(ctx context.Context) error {
	details, err := s.Provider.GetConnectionDetails(ctx)
	if err != nil {
		return fmt.Errorf("failed to get connection details: %w", err)
	}

	tempFiles := []common.TempFile{}
	if details.KeyPath != "" && details.UseInstanceConnect {
		tempFiles = append(tempFiles, common.TempFile{
			Path: details.KeyPath,
			Desc: "temporary SSH key",
		})
	}

	cleanup := common.SetupCleanup(afero.NewOsFs(), tempFiles)
	defer cleanup()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		cleanup()
		os.Exit(1)
	}()

	if details.Method == MethodSSM {
		fmt.Printf("Initiating SSM session with instance %s...\n", details.InstanceID)
		return s.SsmStarter.StartSession(ctx, details.InstanceID)
	}

	if details.UseInstanceConnect {
		fmt.Println("Using EC2 Instance Connect Endpoint (EIC-E) for authentication")
		fmt.Printf("Connecting to %s@%s using EC2 Instance Connect...\n", details.User, details.InstanceID)

		args := []string{
			"ec2-instance-connect", "ssh",
			"--instance-id", details.InstanceID,
			"--connection-type", "eice",
		}

		return s.CommandExecutor.RunInteractiveCommand(ctx, "aws", args...)
	}

	builder := common.NewSSHCommandBuilder(
		details.Host,
		details.User,
		details.KeyPath,
		details.UseInstanceConnect,
	)

	cmd := builder.Build()

	fmt.Println("Using traditional SSH authentication")

	fmt.Printf("Connecting to %s@%s...\n", details.User, details.Host)
	err = common.ExecuteSSHCommand(s.Executor, cmd)
	if err != nil {
		fmt.Printf("SSH session failed with error: %v\n", err)
	}

	return err
}

func (s *Services) StartSOCKSProxy(ctx context.Context, localPort int) error {
	details, err := s.Provider.GetConnectionDetails(ctx)
	if err != nil {
		return fmt.Errorf("failed to get connection details: %w", err)
	}

	tempFiles := []common.TempFile{}
	if details.KeyPath != "" && details.UseInstanceConnect {
		tempFiles = append(tempFiles, common.TempFile{
			Path: details.KeyPath,
			Desc: "temporary SSH key",
		})
	}

	cleanup := common.SetupCleanup(afero.NewOsFs(), tempFiles)
	defer cleanup()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		cleanup()
		os.Exit(1)
	}()

	if details.Method == MethodSSM {
		fmt.Printf("Setting up SSM SOCKS proxy on localhost:%d via instance %s...\n", localPort, details.InstanceID)
		fmt.Println("SOCKS proxy active. Press Ctrl+C to stop.")
		return s.SsmStarter.StartSOCKSProxy(ctx, details.InstanceID, localPort)
	}

	builder := common.NewSSHCommandBuilder(
		details.Host,
		details.User,
		details.KeyPath,
		details.UseInstanceConnect,
	)

	cmd := builder.
		WithSOCKS(localPort).
		Build()

	via := details.Host
	if details.UseInstanceConnect {
		via = fmt.Sprintf("EC2 Instance Connect for instance %s", details.Host)
	}
	fmt.Printf("Setting up SOCKS proxy on localhost:%d via %s...\n", localPort, via)
	fmt.Println("SOCKS proxy active. Press Ctrl+C to stop.")

	err = common.ExecuteSSHCommand(s.Executor, cmd)
	return err
}

func (s *Services) StartPortForwarding(ctx context.Context, localPort int, remoteHost string, remotePort int) (cleanup func(), stop func(), err error) {
	details, err := s.Provider.GetConnectionDetails(ctx)
	if err != nil {
		return func() {}, func() {}, fmt.Errorf("failed to get connection details: %w", err)
	}

	tempFiles := []common.TempFile{}
	if details.KeyPath != "" && details.UseInstanceConnect {
		tempFiles = append(tempFiles, common.TempFile{
			Path: details.KeyPath,
			Desc: "temporary SSH key",
		})
	}

	cleanup = func() {
		common.SetupCleanup(afero.NewOsFs(), tempFiles)()
	}

	if details.Method == MethodSSM {
		fmt.Printf("Setting up SSM port forwarding from localhost:%d to %s:%d via instance %s...\n",
			localPort, remoteHost, remotePort, details.InstanceID)
		fmt.Println("Port forwarding active. Press Ctrl+C to stop.")
		err := s.SsmStarter.StartPortForwarding(ctx, details.InstanceID, localPort, remoteHost, remotePort)
		return cleanup, func() {}, err
	}

	builder := common.NewSSHCommandBuilder(
		details.Host,
		details.User,
		details.KeyPath,
		details.UseInstanceConnect,
	)

	cmd := builder.
		WithForwarding(localPort, remoteHost, remotePort).
		Build()

	via := details.Host
	if details.UseInstanceConnect {
		via = fmt.Sprintf("EC2 Instance Connect for instance %s", details.Host)
	}

	fmt.Printf("Setting up port forwarding from localhost:%d to %s:%d via %s...\n",
		localPort, remoteHost, remotePort, via)
	fmt.Println("Port forwarding active. Press Ctrl+C to stop.")

	cmdErr := make(chan error, 1)
	stopChan := make(chan struct{})
	go func() {
		err := common.ExecuteSSHCommand(s.Executor, cmd)
		select {
		case <-stopChan:
		default:
			cmdErr <- err
		}
	}()

	stop = func() {
		close(stopChan)
	}

	return cleanup, stop, nil
}

func (s *Services) IsAWSConfigured() bool {
	if s == nil || s.Provider == nil {
		return false
	}

	if s.Provider.AwsConfig.Credentials == nil {
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	creds, err := s.Provider.AwsConfig.Credentials.Retrieve(ctx)
	if err != nil {
		return false
	}

	return creds.HasKeys() &&
		creds.AccessKeyID != "" &&
		creds.SecretAccessKey != ""
}
