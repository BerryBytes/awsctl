package bastion

import (
	"context"
	"errors"
	"fmt"

	connection "github.com/BerryBytes/awsctl/internal/common"
	promptUtils "github.com/BerryBytes/awsctl/utils/prompt"
)

type BastionService struct {
	services connection.ServicesInterface
	prompter connection.ConnectionPrompter
}

func NewBastionService(
	services connection.ServicesInterface,
	prompter connection.ConnectionPrompter,
) *BastionService {
	return &BastionService{
		services: services,
		prompter: prompter,
	}
}

func (b *BastionService) Run(ctx context.Context) error {
	for {
		action, err := b.prompter.SelectAction()
		if err != nil {
			return b.handleSelectionError(err)
		}

		switch action {
		case connection.SSHIntoBastion:
			return b.handleSSHIntoBastion(ctx)
		case connection.StartSOCKSProxy:
			return b.handleStartSOCKSProxy(ctx)
		case connection.PortForwarding:
			return b.handlePortForwarding(ctx)
		case connection.ExitBastion:
			return b.handleExitBastion()
		default:
			return fmt.Errorf("unknown action: %s", action)
		}
	}
}

func (b *BastionService) handleSelectionError(err error) error {
	if errors.Is(err, promptUtils.ErrInterrupted) {
		return nil
	}
	return fmt.Errorf("failed to select action: %w", err)
}

func (b *BastionService) handleSSHIntoBastion(ctx context.Context) error {
	if err := b.services.SSHIntoBastion(ctx); err != nil {
		if errors.Is(err, promptUtils.ErrInterrupted) {
			return nil
		}
		return fmt.Errorf("SSH/SSM failed: %v", err)
	}
	fmt.Println("SSH/SSM session closed. Exiting.")
	return nil
}

func (b *BastionService) handleStartSOCKSProxy(ctx context.Context) error {
	port, err := b.prompter.PromptForSOCKSProxyPort(1080)
	if err != nil {
		return b.handlePromptError(err, "port")
	}

	if err := b.services.StartSOCKSProxy(ctx, port); err != nil {
		return fmt.Errorf("SOCKS proxy error: %v", err)
	}
	fmt.Println("SOCKS proxy session closed. Exiting.")
	return nil
}

func (b *BastionService) handlePortForwarding(ctx context.Context) error {
	localPort, err := b.prompter.PromptForLocalPort("forwarding", 8080)
	if err != nil {
		return b.handlePromptError(err, "local port")
	}

	remoteHost, err := b.prompter.PromptForRemoteHost()
	if err != nil {
		return b.handlePromptError(err, "remote host")
	}

	remotePort, err := b.prompter.PromptForRemotePort("remote service")
	if err != nil {
		return b.handlePromptError(err, "remote port")
	}

	if err := b.services.StartPortForwarding(ctx, localPort, remoteHost, remotePort); err != nil {
		if errors.Is(err, promptUtils.ErrInterrupted) {
			return nil
		}
		return fmt.Errorf("port forwarding error: %v", err)
	}
	fmt.Println("Port forwarding session closed. Exiting.")
	return nil
}

func (b *BastionService) handlePromptError(err error, field string) error {
	if errors.Is(err, promptUtils.ErrInterrupted) {
		return nil
	}
	return fmt.Errorf("failed to get %s: %v", field, err)
}

func (b *BastionService) handleExitBastion() error {
	fmt.Println("Exiting bastion tool.")
	return nil
}
