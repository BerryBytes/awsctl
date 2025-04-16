package connection

import (
	"context"
	"fmt"

	"github.com/BerryBytes/awsctl/utils/common"
)

type Services struct {
	provider   *ConnectionProvider
	executor   common.SSHExecutorInterface
	osDetector common.OSDetector
}

func NewServices(
	provider *ConnectionProvider,
) *Services {
	return &Services{
		provider:   provider,
		executor:   &common.RealSSHExecutor{},
		osDetector: common.RuntimeOSDetector{},
	}
}

func (s *Services) SSHIntoBastion(ctx context.Context) error {
	details, err := s.provider.GetConnectionDetails(ctx)
	if err != nil {
		return fmt.Errorf("failed to get connection details: %w", err)
	}

	builder := common.NewSSHCommandBuilder(
		details.Host,
		details.User,
		details.KeyPath,
		details.UseInstanceConnect,
	)

	cmd := builder.Build()

	if details.UseInstanceConnect {
		fmt.Println("Using EC2 Instance Connect for authentication")
	} else {
		fmt.Println("Using traditional SSH authentication")
	}

	fmt.Printf("Connecting to %s@%s...\n", details.User, details.Host)
	return common.ExecuteSSHCommand(s.executor, cmd)
}

func (s *Services) StartSOCKSProxy(ctx context.Context, localPort int) error {
	details, err := s.provider.GetConnectionDetails(ctx)
	if err != nil {
		return fmt.Errorf("failed to get connection details: %w", err)
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

	fmt.Printf("Setting up SOCKS proxy on localhost:%d via %s...\n", localPort, details.Host)
	fmt.Println("SOCKS proxy active. Press Ctrl+C to stop.")

	return common.ExecuteSSHCommand(s.executor, cmd)
}

func (s *Services) StartPortForwarding(ctx context.Context, localPort int, remoteHost string, remotePort int) error {
	details, err := s.provider.GetConnectionDetails(ctx)
	if err != nil {
		return fmt.Errorf("failed to get connection details: %w", err)
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

	fmt.Printf("Setting up port forwarding from localhost:%d to %s:%d via %s...\n", localPort, remoteHost, remotePort, details.Host)
	fmt.Println("Port forwarding active. Press Ctrl+C to stop.")

	return common.ExecuteSSHCommand(s.executor, cmd)
}
