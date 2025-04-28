package connection

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/BerryBytes/awsctl/internal/sso"
	"github.com/BerryBytes/awsctl/utils/common"
)

type Services struct {
	Provider        *ConnectionProvider
	Executor        common.SSHExecutorInterface
	OsDetector      common.OSDetector
	SsmStarter      SSMStarterInterface
	CommandExecutor sso.CommandExecutor
}

func NewServices(
	provider *ConnectionProvider,
) *Services {
	return &Services{
		Provider:        provider,
		Executor:        &common.RealSSHExecutor{},
		OsDetector:      common.RuntimeOSDetector{},
		SsmStarter:      NewRealSSMStarter(provider.SsmClient, provider.AwsConfig.Region),
		CommandExecutor: &sso.RealCommandExecutor{},
	}
}

func (s *Services) SSHIntoBastion(ctx context.Context) error {
	details, err := s.Provider.GetConnectionDetails(ctx)
	if err != nil {
		return fmt.Errorf("failed to get connection details: %w", err)
	}

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
	return common.ExecuteSSHCommand(s.Executor, cmd)
}

func (s *Services) StartSOCKSProxy(ctx context.Context, localPort int) error {
	details, err := s.Provider.GetConnectionDetails(ctx)
	if err != nil {
		return fmt.Errorf("failed to get connection details: %w", err)
	}

	cleanupKey := func() {
		if details.UseInstanceConnect && details.KeyPath != "" {
			if _, err := s.Provider.Fs.Stat(details.KeyPath); os.IsNotExist(err) {
				fmt.Printf("Temporary key %s already removed or never existed\n", details.KeyPath)
				return
			}
			if err := s.Provider.Fs.Remove(details.KeyPath); err != nil {
				fmt.Printf("Warning: failed to remove temporary key %s: %v\n", details.KeyPath, err)
			} else {
				fmt.Printf("Successfully removed temporary key %s\n", details.KeyPath)
			}
		}
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	defer cleanupKey()
	go func() {
		<-sigChan
		cleanupKey()
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

func (s *Services) StartPortForwarding(ctx context.Context, localPort int, remoteHost string, remotePort int) error {
	details, err := s.Provider.GetConnectionDetails(ctx)
	if err != nil {
		return fmt.Errorf("failed to get connection details: %w", err)
	}
	cleanupKey := func() {
		if details.UseInstanceConnect && details.KeyPath != "" {
			if _, err := s.Provider.Fs.Stat(details.KeyPath); os.IsNotExist(err) {
				fmt.Printf("Temporary key %s already removed or never existed\n", details.KeyPath)
				return
			}
			if err := s.Provider.Fs.Remove(details.KeyPath); err != nil {
				fmt.Printf("Warning: failed to remove temporary key %s: %v\n", details.KeyPath, err)
			} else {
				fmt.Printf("Successfully removed temporary key %s\n", details.KeyPath)
			}
		}
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	defer cleanupKey()
	go func() {
		<-sigChan
		cleanupKey()
		os.Exit(1)
	}()

	if details.Method == MethodSSM {
		fmt.Printf("Setting up SSM port forwarding from localhost:%d to %s:%d via instance %s...\n", localPort, remoteHost, remotePort, details.InstanceID)
		fmt.Println("Port forwarding active. Press Ctrl+C to stop.")
		return s.SsmStarter.StartPortForwarding(ctx, details.InstanceID, localPort, remoteHost, remotePort)
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

	fmt.Printf("Setting up port forwarding from localhost:%d to %s:%d via %s...\n", localPort, remoteHost, remotePort, via)
	fmt.Println("Port forwarding active. Press Ctrl+C to stop.")

	err = common.ExecuteSSHCommand(s.Executor, cmd)
	if details.UseInstanceConnect && details.KeyPath != "" {
		_ = s.Provider.Fs.Remove(details.KeyPath)
	}
	return err
}

func (s *Services) IsAWSConfigured() bool {
	if s == nil || s.Provider == nil {
		return false
	}

	if s.Provider.AwsConfig.Credentials == nil {
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	creds, err := s.Provider.AwsConfig.Credentials.Retrieve(ctx)
	if err != nil {
		return false
	}

	return creds.HasKeys() &&
		creds.AccessKeyID != "" &&
		creds.SecretAccessKey != ""
}
