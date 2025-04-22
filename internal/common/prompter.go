package connection

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
	SSHIntoBastion  = "1) SSH/SSM into bastion"
	StartSOCKSProxy = "2) Start SOCKS proxy"
	PortForwarding  = "3) Port forwarding"
	ExitBastion     = "4) Exit"
)

type ConnectionPrompterStruct struct {
	Prompter promptUtils.Prompter
	Prompt   promptui.Prompt
}

func NewConnectionPrompter() *ConnectionPrompterStruct {
	return &ConnectionPrompterStruct{
		Prompter: promptUtils.NewPrompt(),
	}
}

func (b *ConnectionPrompterStruct) SelectAction() (string, error) {
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

func (b *ConnectionPrompterStruct) PromptForSOCKSProxyPort(defaultPort int) (int, error) {
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

func (b *ConnectionPrompterStruct) PromptForBastionHost() (string, error) {
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

func (b *ConnectionPrompterStruct) PromptForSSHUser(defaultUser string) (string, error) {
	user, err := b.Prompter.PromptForInput(fmt.Sprintf("Enter SSH user (default: %s)", defaultUser), defaultUser)
	if err != nil {
		if errors.Is(err, promptUtils.ErrInterrupted) {
			return "", promptUtils.ErrInterrupted
		}
		return "", fmt.Errorf("failed to get SSH user: %w", err)
	}
	return user, nil
}

func (b *ConnectionPrompterStruct) PromptForLocalPort(purpose string, defaultPort int) (int, error) {
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

func (b *ConnectionPrompterStruct) PromptForRemoteHost() (string, error) {
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

func (b *ConnectionPrompterStruct) PromptForRemotePort(service string) (int, error) {
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

func (b *ConnectionPrompterStruct) PromptForSSHKeyPath(defaultPath string) (string, error) {
	path, err := b.Prompter.PromptForInput(fmt.Sprintf("Enter SSH key path (default: %s)", defaultPath), defaultPath)
	if err != nil {
		if errors.Is(err, promptUtils.ErrInterrupted) {
			return "", promptUtils.ErrInterrupted
		}
		return "", fmt.Errorf("failed to get SSH key path: %w", err)
	}
	return path, nil
}

func (b *ConnectionPrompterStruct) PromptForBastionInstance(instances []models.EC2Instance, isSSM bool) (string, error) {
	if len(instances) == 0 {
		return "", errors.New("no instances available")
	}

	if isSSM {
		items := make([]string, len(instances))
		for i, inst := range instances {
			name := inst.Name
			if name == "" {
				name = inst.InstanceID
			}
			items[i] = fmt.Sprintf("%s (%s) - %s", name, inst.InstanceID,
				func() string {
					if inst.PublicIPAddress == "" {
						return "No Public IP"
					}
					return inst.PublicIPAddress
				}())
		}

		selected, err := b.Prompter.PromptForSelection("Select bastion instance for SSM:", items)
		if err != nil {
			if errors.Is(err, promptUtils.ErrInterrupted) {
				return "", promptUtils.ErrInterrupted
			}
			return "", fmt.Errorf("failed to select bastion instance: %w", err)
		}

		for _, inst := range instances {
			name := inst.Name
			if name == "" {
				name = inst.InstanceID
			}
			if strings.Contains(selected, fmt.Sprintf("%s (%s)", name, inst.InstanceID)) {
				return inst.InstanceID, nil
			}
		}
		return "", errors.New("invalid selection")
	}

	connectionMethods := []string{
		"Public IP (direct SSH)",
		"Instance ID (EC2 Instance Connect)",
	}
	method, err := b.Prompter.PromptForSelection("Select connection method:", connectionMethods)
	if err != nil {
		if errors.Is(err, promptUtils.ErrInterrupted) {
			return "", promptUtils.ErrInterrupted
		}
		return "", fmt.Errorf("failed to select connection method: %w", err)
	}

	var filteredInstances []models.EC2Instance
	var items []string

	if method == "Public IP (direct SSH)" {
		for _, inst := range instances {
			if inst.PublicIPAddress != "" {
				filteredInstances = append(filteredInstances, inst)
			}
		}
		if len(filteredInstances) == 0 {
			return "", errors.New("no bastion instances with public IP available")
		}

		items = make([]string, len(filteredInstances))
		for i, inst := range filteredInstances {
			name := inst.Name
			if name == "" {
				name = inst.InstanceID
			}
			items[i] = fmt.Sprintf("%s (%s) - %s", name, inst.InstanceID, inst.PublicIPAddress)
		}
	} else {
		filteredInstances = instances
		items = make([]string, len(filteredInstances))
		for i, inst := range filteredInstances {
			name := inst.Name
			if name == "" {
				name = inst.InstanceID
			}
			items[i] = fmt.Sprintf("%s (%s) - %s", name, inst.InstanceID,
				func() string {
					if inst.PublicIPAddress == "" {
						return "No Public IP (EC2 Connect only)"
					}
					return inst.PublicIPAddress
				}())
		}
	}

	selected, err := b.Prompter.PromptForSelection("Select bastion instance:", items)
	if err != nil {
		if errors.Is(err, promptUtils.ErrInterrupted) {
			return "", promptUtils.ErrInterrupted
		}
		return "", fmt.Errorf("failed to select bastion instance: %w", err)
	}

	for _, inst := range filteredInstances {
		name := inst.Name
		if name == "" {
			name = inst.InstanceID
		}
		if strings.Contains(selected, fmt.Sprintf("%s (%s)", name, inst.InstanceID)) {
			if method == "Public IP (direct SSH)" {
				return inst.PublicIPAddress, nil
			}
			return inst.InstanceID, nil
		}
	}

	return "", errors.New("invalid selection")
}

func (p *ConnectionPrompterStruct) PromptForConfirmation(prompt string) (bool, error) {
	for {
		fullPrompt := fmt.Sprintf("%s (y/N)", prompt)
		result, err := p.Prompter.PromptForInput(fullPrompt, "n")
		if err != nil {
			if errors.Is(err, promptUtils.ErrInterrupted) {
				return false, promptUtils.ErrInterrupted
			}
			return false, fmt.Errorf("confirmation failed: %w", err)
		}

		normalized := strings.ToLower(strings.TrimSpace(result))
		switch normalized {
		case "y", "yes":
			return true, nil
		case "n", "no", "":
			return false, nil
		default:
			fmt.Printf("Invalid input %q - please enter 'y' or 'n'\n", result)
			continue
		}
	}
}

func (b *ConnectionPrompterStruct) PromptForInstanceID() (string, error) {
	instanceID, err := b.Prompter.PromptForInput("Enter EC2 instance ID:", "")
	if err != nil {
		if errors.Is(err, promptUtils.ErrInterrupted) {
			return "", promptUtils.ErrInterrupted
		}
		return "", fmt.Errorf("failed to get instance ID: %w", err)
	}

	if instanceID == "" {
		return "", errors.New("instance ID cannot be empty")
	}

	return instanceID, nil
}

func (b *ConnectionPrompterStruct) ChooseConnectionMethod() (string, error) {
	options := []string{
		MethodSSH,
		MethodSSM,
	}

	selected, err := b.Prompter.PromptForSelection("Select connection method:", options)
	if err != nil {
		if errors.Is(err, promptUtils.ErrInterrupted) {
			return "", promptUtils.ErrInterrupted
		}
		return "", fmt.Errorf("failed to choose connection method: %w", err)
	}

	switch selected {
	case MethodSSH:
		return MethodSSH, nil
	case MethodSSM:
		return MethodSSM, nil
	default:
		return "", fmt.Errorf("unexpected selection: %s", selected)
	}
}

func (b *ConnectionPrompterStruct) PromptForRegion(defaultRegion string) (string, error) {
	promptMessage := "Enter AWS region:"
	if defaultRegion != "" {
		promptMessage = fmt.Sprintf("Enter AWS region (Default: %s):", defaultRegion)
	}

	region, err := b.Prompter.PromptForInput(promptMessage, defaultRegion)
	if err != nil {
		if errors.Is(err, promptUtils.ErrInterrupted) {
			return "", promptUtils.ErrInterrupted
		}
		return "", fmt.Errorf("failed to get region: %w", err)
	}
	if region == "" {
		return "", errors.New("region cannot be empty")
	}
	return region, nil
}
