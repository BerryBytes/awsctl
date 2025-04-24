package connection

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/BerryBytes/awsctl/internal/sso"
	"github.com/BerryBytes/awsctl/utils/common"
)

type Services struct {
	provider        *ConnectionProvider
	executor        common.SSHExecutorInterface
	osDetector      common.OSDetector
	ssmStarter      SSMStarterInterface
	commandExecutor sso.CommandExecutor
}

func NewServices(
	provider *ConnectionProvider,
) *Services {
	return &Services{
		provider:        provider,
		executor:        &common.RealSSHExecutor{},
		osDetector:      common.RuntimeOSDetector{},
		ssmStarter:      NewRealSSMStarter(provider.ssmClient, provider.awsConfig.Region),
		commandExecutor: &sso.RealCommandExecutor{},
	}
}

func (s *Services) SSHIntoBastion(ctx context.Context) error {
	details, err := s.provider.GetConnectionDetails(ctx)
	if err != nil {
		return fmt.Errorf("failed to get connection details: %w", err)
	}

	if details.Method == MethodSSM {
		fmt.Printf("Initiating SSM session with instance %s...\n", details.InstanceID)
		return s.ssmStarter.StartSession(ctx, details.InstanceID)
	}

	fmt.Printf("Details here: %+v \n", details)

	if details.UseInstanceConnect {
		fmt.Println("Using EC2 Instance Connect Endpoint (EIC-E) for authentication")
		fmt.Printf("Connecting to %s@%s using EC2 Instance Connect...\n", details.User, details.InstanceID)

		args := []string{
			"ec2-instance-connect", "ssh",
			"--instance-id", details.InstanceID,
			"--connection-type", "eice",
		}

		return s.commandExecutor.RunInteractiveCommand(ctx, "aws", args...)
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
	return common.ExecuteSSHCommand(s.executor, cmd)
}

func (s *Services) StartSOCKSProxy(ctx context.Context, localPort int) error {
	details, err := s.provider.GetConnectionDetails(ctx)
	if err != nil {
		return fmt.Errorf("failed to get connection details: %w", err)
	}

	cleanupKey := func() {
		if details.UseInstanceConnect && details.KeyPath != "" {
			if _, err := os.Stat(details.KeyPath); os.IsNotExist(err) {
				fmt.Printf("Temporary key %s already removed or never existed\n", details.KeyPath)
				return
			}
			if err := os.Remove(details.KeyPath); err != nil {
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
		return s.ssmStarter.StartSOCKSProxy(ctx, details.InstanceID, localPort)
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

	err = common.ExecuteSSHCommand(s.executor, cmd)
	return err
}

func (s *Services) StartPortForwarding(ctx context.Context, localPort int, remoteHost string, remotePort int) error {
	details, err := s.provider.GetConnectionDetails(ctx)
	if err != nil {
		return fmt.Errorf("failed to get connection details: %w", err)
	}
	cleanupKey := func() {
		if details.UseInstanceConnect && details.KeyPath != "" {
			if _, err := os.Stat(details.KeyPath); os.IsNotExist(err) {
				fmt.Printf("Temporary key %s already removed or never existed\n", details.KeyPath)
				return
			}
			if err := os.Remove(details.KeyPath); err != nil {
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
		return s.ssmStarter.StartPortForwarding(ctx, details.InstanceID, localPort, remoteHost, remotePort)
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

	fmt.Printf("command here: %v", cmd)
	via := details.Host
	if details.UseInstanceConnect {
		via = fmt.Sprintf("EC2 Instance Connect for instance %s", details.Host)
	}

	fmt.Printf("Setting up port forwarding from localhost:%d to %s:%d via %s...\n", localPort, remoteHost, remotePort, via)
	fmt.Println("Port forwarding active. Press Ctrl+C to stop.")

	err = common.ExecuteSSHCommand(s.executor, cmd)
	if details.UseInstanceConnect && details.KeyPath != "" {
		os.Remove(details.KeyPath)
	}
	return err
}
