package promptutils

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/manifoldco/promptui"
)

type Prompter interface {
	PromptForSelection(label string, items []string) (string, error)
	PromptSSOConfiguration() (string, string, string, error)
	PromptForRegion() (string, error)
	PromptForRole() (string, error)
	PromptForAccount() (string, error)
	PromptForProfile() string
	PromptForConfirmation(prompt string) bool
}

type RealPrompter struct{}

var ErrInterrupted = errors.New("operation interrupted")

func (p *RealPrompter) HandlePromptError(err error) error {
	if err != nil {
		if errors.Is(err, promptui.ErrInterrupt) {
			fmt.Println("\nReceived termination signal. Exiting.")
			return ErrInterrupted
		}
		return fmt.Errorf("failed to select an option: %w", err)
	}
	return nil
}

func (p *RealPrompter) PromptForSelection(label string, items []string) (string, error) {
	prompt := promptui.Select{
		Label: label,
		Items: items,
	}
	_, selected, err := prompt.Run()

	err = p.HandlePromptError(err)
	if err != nil {
		if errors.Is(err, ErrInterrupted) {
			return "", ErrInterrupted
		}
		return "", err
	}

	return selected, nil
}

func (p *RealPrompter) PromptSSOConfiguration() (string, string, string, error) {
	prompt := promptui.Prompt{
		Label: "Enter Profile Name",
	}
	profileName, err := prompt.Run()
	if err != nil {
		return "", "", "", fmt.Errorf("failed to get profile name: %v", err)
	}

	prompt = promptui.Prompt{
		Label: "Enter SSO Start URL",
	}
	ssoStartURL, err := prompt.Run()
	if err != nil {
		return "", "", "", fmt.Errorf("failed to get SSO start URL: %v", err)
	}

	prompt = promptui.Prompt{
		Label: "Enter SSO Region",
	}
	ssoRegion, err := prompt.Run()
	if err != nil {
		return "", "", "", fmt.Errorf("failed to get SSO region: %v", err)
	}

	return profileName, ssoStartURL, ssoRegion, nil
}

func (p *RealPrompter) PromptForRegion() (string, error) {
	regions := []string{"us-east-1", "us-west-2", "eu-central-1"}
	prompt := promptui.Select{
		Label: "Choose region",
		Items: regions,
		Size:  len(regions),
	}
	_, selectedRegion, err := prompt.Run()
	if err != nil {
		return "", fmt.Errorf("selection aborted")
	}
	return selectedRegion, nil
}

func (p *RealPrompter) PromptForRole() (string, error) {
	roles := []string{"AdministratorAccess", "Billing", "PowerUserAccess", "ViewOnlyAccess", "LogsReadOnlyPermissionSet", "S3FullAccess"}
	prompt := promptui.Select{
		Label: "Choose role",
		Items: roles,
		Size:  len(roles),
	}
	_, selectedRole, err := prompt.Run()
	if err != nil {
		return "", fmt.Errorf("selection aborted")
	}
	return selectedRole, nil
}

func (p *RealPrompter) PromptForAccount() (string, error) {
	accounts := []string{"Logs", "NonProd", "Security", "Production", "Shared Services", "MarcRosenberg"}
	prompt := promptui.Select{
		Label: "Choose account",
		Items: accounts,
		Size:  len(accounts),
	}
	_, selected, err := prompt.Run()
	if err != nil {
		return "", fmt.Errorf("selection aborted")
	}
	return selected, nil
}

func (p *RealPrompter) PromptForProfile() string {
	cmd := exec.Command("aws", "configure", "list-profiles")
	output, err := cmd.Output()
	if err != nil {
		fmt.Println("Error: AWS CLI not found or misconfigured.")
		os.Exit(1)
	}

	profiles := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(profiles) == 0 || profiles[0] == "" {
		return ""
	}

	prompt := promptui.Select{
		Label: "Choose AWS Profile",
		Items: profiles,
		Size:  15,
	}

	_, selectedProfile, err := prompt.Run()
	if err != nil {
		fmt.Println("Profile selection cancelled.")
		os.Exit(1)
	}

	os.Setenv("AWS_PROFILE", selectedProfile)
	fmt.Println("Run below command in your shell to persist the profile:")
	fmt.Printf("export AWS_PROFILE=%s\n", selectedProfile)

	return selectedProfile
}

func (p *RealPrompter) PromptForConfirmation(prompt string) bool {
	promptInstance := promptui.Prompt{
		Label:     prompt,
		IsConfirm: true,
	}
	result, err := promptInstance.Run()
	if err != nil {
		return false
	}
	return strings.HasPrefix(strings.ToLower(result), "y")
}

func NewPrompt() Prompter {
	return &RealPrompter{}
}
