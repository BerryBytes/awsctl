package rds

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/BerryBytes/awsctl/internal/sso"
	"github.com/BerryBytes/awsctl/models"
	promptUtils "github.com/BerryBytes/awsctl/utils/prompt"
)

type RPrompter struct {
	Prompt          promptUtils.Prompter
	AWSConfigClient sso.SSOClient
}

type RDSAction int

const (
	ConnectDirect RDSAction = iota
	ConnectViaTunnel
	ExitRDS
)

func NewRPrompter(
	prompt promptUtils.Prompter,
	configClient sso.SSOClient,
) *RPrompter {
	return &RPrompter{
		Prompt:          prompt,
		AWSConfigClient: configClient,
	}
}

func (p *RPrompter) PromptForRegion() (string, error) {
	region, err := p.Prompt.PromptForInput("Enter AWS region (e.g. us-east-1):", "")
	if err != nil {
		if errors.Is(err, promptUtils.ErrInterrupted) {
			return "", promptUtils.ErrInterrupted
		}
		return "", fmt.Errorf("failed to get AWS region: %w", err)
	}

	if region == "" {
		return "", errors.New("AWS region cannot be empty")
	}

	return region, nil
}

func (p *RPrompter) PromptForRDSInstance(instances []models.RDSInstance) (string, error) {
	if len(instances) == 0 {
		endpoint, dbUser, _, err := p.PromptForManualEndpoint()
		if err != nil {
			return "", err
		}
		fmt.Printf("User: %s\n", dbUser)
		return endpoint, nil
	}

	items := make([]string, len(instances))
	for i, inst := range instances {
		displayEndpoint := inst.Endpoint
		maxEndpointLength := 40

		if len(displayEndpoint) > maxEndpointLength {
			domainParts := strings.Split(displayEndpoint, ".")
			if len(domainParts) > 2 {
				displayEndpoint = domainParts[0] + "..." + strings.Join(domainParts[len(domainParts)-2:], ".")
			} else {
				displayEndpoint = displayEndpoint[:maxEndpointLength-3] + "..."
			}
		}

		items[i] = fmt.Sprintf("%s (%s) - %s",
			inst.DBInstanceIdentifier,
			inst.Engine,
			displayEndpoint)
	}

	selected, err := p.Prompt.PromptForSelection("Select an RDS instance:", items)
	if err != nil {
		if errors.Is(err, promptUtils.ErrInterrupted) {
			return "", promptUtils.ErrInterrupted
		}
		return "", fmt.Errorf("failed to select RDS instance: %w", err)
	}

	for _, inst := range instances {
		expectedFormat := fmt.Sprintf("%s (%s) - %s", inst.DBInstanceIdentifier, inst.Engine, inst.Endpoint)
		if selected == expectedFormat ||
			strings.HasPrefix(selected, fmt.Sprintf("%s (%s) -", inst.DBInstanceIdentifier, inst.Engine)) {
			return inst.DBInstanceIdentifier, nil
		}
	}

	return "", errors.New("invalid selection")
}

func (p *RPrompter) PromptForProfile() (string, error) {
	awsProfile := os.Getenv("AWS_PROFILE")
	if awsProfile != "" {
		return awsProfile, nil
	}

	validProfiles, err := p.AWSConfigClient.ValidProfiles()
	if err != nil {
		return "", fmt.Errorf("failed to list valid profiles: %w", err)
	}

	if len(validProfiles) == 0 {
		return "", errors.New("no valid AWS profiles found")
	}

	if len(validProfiles) == 1 {
		return validProfiles[0], nil
	}

	selectedProfile, err := p.Prompt.PromptForSelection("Select an AWS profile:", validProfiles)
	if err != nil {
		return "", err
	}

	return selectedProfile, nil
}

func (p *RPrompter) SelectRDSAction() (RDSAction, error) {
	actions := []string{
		"Connect Direct (Just show RDS endpoint)",
		"Connect Via Tunnel (SSH port forwarding)",
		"Exit",
	}

	selected, err := p.Prompt.PromptForSelection("Select an RDS action:", actions)
	if err != nil {
		return ExitRDS, fmt.Errorf("failed to select RDS action: %w", err)
	}

	switch selected {
	case actions[ConnectDirect]:
		return ConnectDirect, nil
	case actions[ConnectViaTunnel]:
		return ConnectViaTunnel, nil
	case actions[ExitRDS]:
		return ExitRDS, nil
	default:
		return ExitRDS, fmt.Errorf("invalid action selected")
	}
}

func (p *RPrompter) PromptForManualEndpoint() (endpoint, dbUser, region string, err error) {
	endpoint, err = p.Prompt.PromptForInput("Enter RDS endpoint (hostname:port):", "")
	if err != nil {
		return "", "", "", fmt.Errorf("failed to input endpoint: %w", err)
	}

	if !isValidEndpoint(endpoint) {
		return "", "", "", fmt.Errorf("invalid endpoint format")
	}

	dbUser, err = p.Prompt.PromptForInput("Enter database username:", "")
	if err != nil {
		return "", "", "", fmt.Errorf("failed to input username: %w", err)
	}

	region, err = p.PromptForRegion()
	if err != nil {
		return "", "", "", fmt.Errorf("failed to input region: %w", err)
	}

	return endpoint, dbUser, region, nil
}

func (p *RPrompter) GetAWSConfig() (profile, region string, err error) {
	profile = os.Getenv("AWS_PROFILE")
	if profile == "" {
		profiles, err := p.AWSConfigClient.ValidProfiles()
		if err != nil {
			return "", "", fmt.Errorf("failed to retrieve AWS profiles: %w", err)
		}

		if len(profiles) == 0 {
			return "", "", errors.New("no AWS profiles found")
		}

		profile, err = p.Prompt.PromptForSelection("Select AWS profile:", profiles)
		if err != nil {
			return "", "", err
		}
	}

	region = os.Getenv("AWS_REGION")
	if region == "" {
		region, err = p.PromptForRegion()
		if err != nil {
			return "", "", err
		}
	}

	return profile, region, nil
}

func (p *RPrompter) PromptForAuthMethod(message string, options []string) (string, error) {
	selected, err := p.Prompt.PromptForSelection(message, options)
	if err != nil {
		if errors.Is(err, promptUtils.ErrInterrupted) {
			return "", promptUtils.ErrInterrupted
		}
		return "", fmt.Errorf("failed to select authentication method: %w", err)
	}
	return selected, nil
}

func (p *RPrompter) PromptForDBUser() (string, error) {
	return p.Prompt.PromptForInput("Enter database username:", "")
}

func isValidEndpoint(endpoint string) bool {
	return strings.Contains(endpoint, ".") && strings.Contains(endpoint, ":")
}
