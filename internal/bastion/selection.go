package bastion

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/BerryBytes/awsctl/models"
	promptUtils "github.com/BerryBytes/awsctl/utils/prompt"
	"github.com/manifoldco/promptui"
)

const (
	SSHIntoBastion  = "1) SSH into bastion"
	StartSOCKSProxy = "2) Start SOCKS proxy"
	PortForwarding  = "3) Port forwarding"
	ExitBastion     = "4) Exit"
)

type BastionPrompter struct {
	Prompter promptUtils.Prompter
	Prompt   promptui.Prompt
}

func NewBastionPrompter() *BastionPrompter {
	return &BastionPrompter{
		Prompter: promptUtils.NewPrompt(),
	}
}

func (b *BastionPrompter) SelectAction() (string, error) {
	options := []string{
		SSHIntoBastion,
		StartSOCKSProxy,
		PortForwarding,
		ExitBastion,
	}

	selected, err := b.Prompter.PromptForSelection("What would you like to do?", options)
	if err != nil {
		if errors.Is(err, promptUtils.ErrInterrupted) {
			return "", promptUtils.ErrInterrupted
		}
		return "", fmt.Errorf("failed to select action: %v", err)
	}
	return selected, nil
}

func (b *BastionPrompter) PromptForSOCKSProxyPort(defaultPort int) (int, error) {
	prompt := fmt.Sprintf("Enter SOCKS proxy port (default: %d)", defaultPort)
	result, err := b.Prompter.PromptForInput(prompt, strconv.Itoa(defaultPort))
	if err != nil {
		if errors.Is(err, promptUtils.ErrInterrupted) {
			return 0, promptUtils.ErrInterrupted
		}
		return 0, fmt.Errorf("failed to get SOCKS proxy port: %w", err)
	}

	port, err := strconv.Atoi(result)
	if err != nil {
		return 0, fmt.Errorf("invalid port number: %w", err)
	}

	if port < 1 || port > 65535 {
		return 0, errors.New("port must be between 1 and 65535")
	}

	return port, nil
}

func (b *BastionPrompter) PromptForBastionHost() (string, error) {
	host, err := b.Prompter.PromptForInput("Enter bastion host IP or DNS name:", "")
	if err != nil {
		if errors.Is(err, promptUtils.ErrInterrupted) {
			return "", promptUtils.ErrInterrupted
		}
		return "", fmt.Errorf("failed to get bastion host: %w", err)
	}

	if host == "" {
		return "", errors.New("bastion host cannot be empty")
	}
	return host, nil
}

func (b *BastionPrompter) PromptForSSHUser(defaultUser string) (string, error) {
	user, err := b.Prompter.PromptForInput(fmt.Sprintf("Enter SSH user (default: %s)", defaultUser), defaultUser)
	if err != nil {
		if errors.Is(err, promptUtils.ErrInterrupted) {
			return "", promptUtils.ErrInterrupted
		}
		return "", fmt.Errorf("failed to get SSH user: %w", err)
	}
	return user, nil
}

func (b *BastionPrompter) PromptForLocalPort(purpose string, defaultPort int) (int, error) {
	if defaultPort < 1 || defaultPort > 65535 {
		return 0, fmt.Errorf("invalid default port number")
	}

	promptMsg := fmt.Sprintf("Enter local port for %s [default: %d]:", purpose, defaultPort)
	result, err := b.Prompter.PromptForInput(promptMsg, strconv.Itoa(defaultPort))
	if err != nil {
		if errors.Is(err, promptUtils.ErrInterrupted) {
			return 0, promptUtils.ErrInterrupted
		}
		return 0, fmt.Errorf("failed to get local port: %w", err)
	}

	port, err := strconv.Atoi(result)
	if err != nil || port < 1 || port > 65535 {
		return 0, fmt.Errorf("invalid port number")
	}

	finalPort := findAvailablePort(port)
	if finalPort != port {
		fmt.Printf("Port %d is already in use. Choosing port %d instead.\n", port, finalPort)
	}

	return finalPort, nil
}

func findAvailablePort(port int) int {
	for p := port; p <= 65535; p++ {
		if isPortAvailable(p) {
			return p
		}
	}
	return port
}

func isPortAvailable(port int) bool {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return false
	}
	_ = listener.Close()
	return true
}

func (b *BastionPrompter) PromptForRemoteHost() (string, error) {
	host, err := b.Prompter.PromptForInput("Enter remote host IP or DNS name:", "")
	if err != nil {
		if errors.Is(err, promptUtils.ErrInterrupted) {
			return "", promptUtils.ErrInterrupted
		}
		return "", fmt.Errorf("failed to get remote host: %w", err)
	}
	if host == "" {
		return "", errors.New("remote host cannot be empty")
	}
	return host, nil
}

func (b *BastionPrompter) PromptForRemotePort(service string) (int, error) {
	prompt := fmt.Sprintf("Enter remote %s port", service)
	result, err := b.Prompter.PromptForInput(prompt, "")
	if err != nil {
		if errors.Is(err, promptUtils.ErrInterrupted) {
			return 0, promptUtils.ErrInterrupted
		}
		return 0, fmt.Errorf("failed to get remote port: %w", err)
	}
	port, err := strconv.Atoi(result)
	if err != nil {
		return 0, fmt.Errorf("invalid port number: %w", err)
	}

	return port, nil
}

func (b *BastionPrompter) PromptForSSHKeyPath(defaultPath string) (string, error) {
	path, err := b.Prompter.PromptForInput(fmt.Sprintf("Enter SSH key path (default: %s)", defaultPath), defaultPath)
	if err != nil {
		if errors.Is(err, promptUtils.ErrInterrupted) {
			return "", promptUtils.ErrInterrupted
		}
		return "", fmt.Errorf("failed to get SSH key path: %w", err)
	}
	return path, nil
}

func (b *BastionPrompter) PromptForBastionInstance(instances []models.EC2Instance) (string, error) {
	if len(instances) == 0 {
		return "", errors.New("no instances available")
	}

	items := make([]string, len(instances))
	for i, inst := range instances {
		name := inst.Name
		if name == "" {
			name = inst.InstanceID
		}
		items[i] = fmt.Sprintf("%s (%s) - %s", name, inst.InstanceID, inst.PublicIPAddress)
	}

	selected, err := b.Prompter.PromptForSelection("Select bastion instance:", items)

	if err != nil {
		if errors.Is(err, promptUtils.ErrInterrupted) {
			return "", promptUtils.ErrInterrupted
		}
		return "", fmt.Errorf("failed to select bastion host: %w", err)
	}

	for _, inst := range instances {
		name := inst.Name
		if name == "" {
			name = inst.InstanceID
		}
		if strings.Contains(selected, fmt.Sprintf("%s (%s)", name, inst.InstanceID)) {
			if inst.PublicIPAddress == "" {
				return "", errors.New("selected instance has no public IP")
			}
			return inst.PublicIPAddress, nil
		}
	}

	return "", errors.New("invalid selection")
}

func (p *BastionPrompter) PromptForConfirmation(prompt string) (bool, error) {

	result, err := p.Prompter.PromptForSelection(prompt, []string{"y", "n"})
	if err != nil {
		if errors.Is(err, promptUtils.ErrInterrupted) {
			fmt.Println("\nReceived termination signal. Exiting.")
			return false, promptUtils.ErrInterrupted
		}
		return false, fmt.Errorf("confirmation failed: %w", err)
	}

	return strings.ToLower(result) == "y", nil
}
