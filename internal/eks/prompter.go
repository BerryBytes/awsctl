package eks

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/BerryBytes/awsctl/internal/sso"
	"github.com/BerryBytes/awsctl/models"
	promptUtils "github.com/BerryBytes/awsctl/utils/prompt"
)

type EPrompter struct {
	Prompt          promptUtils.Prompter
	AWSConfigClient sso.AWSConfigClient
}

type EKSAction int

const (
	UpdateKubeConfig EKSAction = iota
	ExitEKS
)

func NewEPrompter(
	prompt promptUtils.Prompter,
	configClient sso.AWSConfigClient,
) *EPrompter {
	return &EPrompter{
		Prompt:          prompt,
		AWSConfigClient: configClient,
	}
}

func (p *EPrompter) PromptForRegion() (string, error) {
	region, err := p.Prompt.PromptForInput("Enter AWS region (e.g., us-east-1):", "")
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

func (p *EPrompter) PromptForEKSCluster(clusters []models.EKSCluster) (string, error) {
	if len(clusters) == 0 {
		clusterName, _, _, _, err := p.PromptForManualCluster()
		if err != nil {
			return "", err
		}
		return clusterName, nil
	}

	items := make([]string, len(clusters))
	for i, cluster := range clusters {
		items[i] = fmt.Sprintf("%s (%s)", cluster.ClusterName, cluster.Region)
	}

	selected, err := p.Prompt.PromptForSelection("Select an EKS cluster:", items)
	if err != nil {
		if errors.Is(err, promptUtils.ErrInterrupted) {
			return "", promptUtils.ErrInterrupted
		}
		return "", fmt.Errorf("failed to select EKS cluster: %w", err)
	}

	for _, cluster := range clusters {
		if selected == fmt.Sprintf("%s (%s)", cluster.ClusterName, cluster.Region) {
			return cluster.ClusterName, nil
		}
	}

	return "", errors.New("invalid selection")
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

func (p *EPrompter) SelectEKSAction() (EKSAction, error) {
	actions := []string{
		"Update kubeconfig",
		"Exit",
	}

	selected, err := p.Prompt.PromptForSelection("Select an EKS action:", actions)
	if err != nil {
		return ExitEKS, fmt.Errorf("failed to select EKS action: %w", err)
	}

	switch selected {
	case actions[UpdateKubeConfig]:
		return UpdateKubeConfig, nil
	case actions[ExitEKS]:
		return ExitEKS, nil
	default:
		return ExitEKS, fmt.Errorf("invalid action selected")
	}
}

func (p *EPrompter) PromptForManualCluster() (clusterName, endpoint, caData, region string, err error) {
	clusterName, err = p.Prompt.PromptForInput("Enter EKS cluster name:", "")
	if err != nil {
		return "", "", "", "", fmt.Errorf("failed to input cluster name: %w", err)
	}

	endpoint, err = p.Prompt.PromptForInput("Enter EKS cluster endpoint (e.g., https://<endpoint>):", "")
	if err != nil {
		return "", "", "", "", fmt.Errorf("failed to input endpoint: %w", err)
	}

	if !strings.HasPrefix(endpoint, "https://") {
		return "", "", "", "", fmt.Errorf("invalid endpoint format; must start with https://")
	}

	caData, err = p.Prompt.PromptForInput("Enter Certificate Authority data (base64):", "")
	if err != nil {
		return "", "", "", "", fmt.Errorf("failed to input CA data: %w", err)
	}

	region, err = p.PromptForRegion()
	if err != nil {
		return "", "", "", "", fmt.Errorf("failed to input region: %w", err)
	}

	return clusterName, endpoint, caData, region, nil
}

func (p *EPrompter) GetAWSConfig() (profile, region string, err error) {
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
