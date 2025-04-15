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

func (s *Services) StartSOCKSProxy(ctx context.Context, port int) error {
	details, err := s.provider.GetConnectionDetails(ctx)
	if err != nil {
		return fmt.Errorf("failed to get connection details: %w", err)
	}

	if err := common.TerminateSOCKSProxy(s.executor, port, s.osDetector); err != nil {
		return fmt.Errorf("failed to terminate existing proxy: %w", err)
	}

	builder := common.NewSSHCommandBuilder(
		details.Host,
		details.User,
		details.KeyPath,
		details.UseInstanceConnect,
	)

	cmd := builder.
		WithSOCKS(port).
		WithBackground().
		Build()

	fmt.Printf("Starting SOCKS proxy on port %d...\n", port)
	if err := common.ExecuteSSHCommand(s.executor, cmd); err != nil {
		return fmt.Errorf("failed to start SOCKS proxy: %w", err)
	}

	fmt.Printf("SOCKS proxy started on port %d\n", port)
	return nil
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

	fmt.Printf("Setting up port forwarding from localhost:%d to %s:%d via %s...\n",
		localPort, remoteHost, remotePort, details.Host)

	return common.ExecuteSSHCommand(s.executor, cmd)
}
