package eks

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"

	"github.com/BerryBytes/awsctl/internal/sso"
	"github.com/BerryBytes/awsctl/models"
	generalutils "github.com/BerryBytes/awsctl/utils/general"
	promptUtils "github.com/BerryBytes/awsctl/utils/prompt"
)

type EPrompter struct {
	Prompt          promptUtils.Prompter
	AWSConfigClient sso.SSOClient
}

type EKSAction int

const (
	UpdateKubeConfig EKSAction = iota
	ExitEKS
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

func (p *EPrompter) PromptForRegion() (string, error) {
	return p.Prompt.PromptForInputWithValidation(
		"Enter AWS region:",
		"",
		func(input string) error {
			if !generalutils.IsRegionValid(input) {
				return fmt.Errorf("invalid AWS region format or unrecognized region: %s", input)
			}
			return nil
		},
	)
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
	clusterName, err = p.Prompt.PromptForInputWithValidation(
		"Enter EKS cluster name:",
		"",
		func(input string) error {
			if input == "" {
				return fmt.Errorf("cluster name cannot be empty")
			}
			if len(input) > 40 {
				return fmt.Errorf("cluster name must be 40 characters or less (recommended for usability)")
			}
			if !regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9\-_]*[a-zA-Z0-9]$`).MatchString(input) {
				return fmt.Errorf("invalid format. Must:\n- Start with a letter\n- Contain only [a-z, A-Z, 0-9, -, _]\n- Not end with '-' or '_'")
			}
			return nil
		},
	)
	if err != nil {
		return "", "", "", "", fmt.Errorf("failed to input cluster name: %w", err)
	}

	endpoint, err = p.Prompt.PromptForInputWithValidation(
		"Enter EKS cluster endpoint (e.g., https://<endpoint>):",
		"",
		func(input string) error {
			if !strings.HasPrefix(input, "https://") {
				return fmt.Errorf("endpoint must start with https://")
			}

			parsedURL, err := url.ParseRequestURI(input)
			if err != nil {
				return fmt.Errorf("invalid URL format: %v", err)
			}

			host := parsedURL.Hostname()
			if host == "" {
				return fmt.Errorf("missing hostname in endpoint")
			}

			patterns := []*regexp.Regexp{
				// IPv4 patterns
				regexp.MustCompile(`^[A-Z0-9]+\.gr7\.[a-z0-9-]+\.eks\.amazonaws\.com$`),
				regexp.MustCompile(`^eks-cluster\.[a-z0-9-]+\.eks\.amazonaws\.com$`),
				regexp.MustCompile(`^eks-cluster\.[a-z0-9-]+\.amazonwebservices\.com\.cn$`),
				regexp.MustCompile(`^eks-cluster\.[a-z0-9-]+\.amazonwebservices\.gov$`),

				// IPv6 patterns
				regexp.MustCompile(`^eks-cluster\.[a-z0-9-]+\.api\.aws$`),
				regexp.MustCompile(`^eks-cluster\.[a-z0-9-]+\.api\.amazonwebservices\.com\.cn$`),
			}

			for _, pattern := range patterns {
				if pattern.MatchString(host) {
					return nil
				}
			}

			return fmt.Errorf(`invalid EKS endpoint format. Valid examples:
- IPv4: https://ABCD123.gr7.us-west-2.eks.amazonaws.com
- IPv4: https://eks-cluster.cn-north-1.amazonwebservices.com.cn
- IPv6: https://eks-cluster.us-east-1.api.aws`)
		},
	)
	if err != nil {
		return "", "", "", "", fmt.Errorf("failed to input endpoint: %w", err)
	}

	caData, err = p.Prompt.PromptForInputWithValidation(
		"Enter Certificate Authority data (base64):",
		"",
		func(input string) error {
			if input == "" {
				return fmt.Errorf("CA data cannot be empty")
			}
			decoded, err := base64.StdEncoding.DecodeString(input)
			if err != nil {
				return fmt.Errorf("invalid base64 data: %v", err)
			}
			if !bytes.Contains(decoded, []byte("-----BEGIN CERTIFICATE-----")) {
				return fmt.Errorf("CA data should be a PEM certificate in base64 format")
			}
			return nil
		},
	)
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
