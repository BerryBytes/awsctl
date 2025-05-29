package ecr

import (
	"errors"
	"fmt"
	"os"

	"github.com/BerryBytes/awsctl/internal/sso"
	promptUtils "github.com/BerryBytes/awsctl/utils/prompt"
)

type EPrompter struct {
	Prompt          promptUtils.Prompter
	AWSConfigClient sso.SSOClient
}

type ECRAction int

const (
	LoginECR ECRAction = iota
	ExitECR
)

func NewEPrompter(
	prompt promptUtils.Prompter,
	configClient sso.SSOClient,
) *EPrompter {
	return &EPrompter{
		Prompt:          prompt,
		AWSConfigClient: configClient,
	}
}

func (p *EPrompter) SelectECRAction() (ECRAction, error) {
	actions := []string{
		"Login to ECR",
		"Exit",
	}

	selected, err := p.Prompt.PromptForSelection("Select an ECR action:", actions)
	if err != nil {
		return ExitECR, fmt.Errorf("failed to select ECR action: %w", err)
	}

	switch selected {
	case actions[LoginECR]:
		return LoginECR, nil
	case actions[ExitECR]:
		return ExitECR, nil
	default:
		return ExitECR, fmt.Errorf("invalid action selected")
	}
}

func (p *EPrompter) PromptForProfile() (string, error) {
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
