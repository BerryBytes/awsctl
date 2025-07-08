package sso

import (
	"errors"
	"fmt"
	"strings"

	generalutils "github.com/BerryBytes/awsctl/utils/general"
	promptUtils "github.com/BerryBytes/awsctl/utils/prompt"
	"github.com/manifoldco/promptui"
)

type RealPromptRunner struct{}

func (r *RealPromptRunner) RunPrompt(label, defaultValue string, validate func(string) error) (string, error) {
	prompt := promptui.Prompt{
		Label:    label,
		Default:  defaultValue,
		Validate: validate,
	}
	return prompt.Run()
}

func (r *RealPromptRunner) RunSelect(label string, items []string) (string, error) {
	selectPrompt := promptui.Select{
		Label: label,
		Items: items,
	}
	_, result, err := selectPrompt.Run()
	return result, err
}

var ValidateStartURLFunc = func(input string) error {
	if !strings.HasPrefix(input, "http://") && !strings.HasPrefix(input, "https://") {
		return fmt.Errorf("invalid URL format")
	}
	return nil
}

type PromptUI struct {
	Prompt promptUtils.Prompter
	Runner PromptRunner
}

func handlePromptError(err error) error {
	if err != nil {
		if errors.Is(err, promptui.ErrInterrupt) || errors.Is(err, promptui.ErrEOF) {
			fmt.Println("\nReceived termination signal. Exiting.")
			return promptUtils.ErrInterrupted
		}
		return fmt.Errorf("prompt failed: %w", err)
	}
	return nil
}

func (p *PromptUI) PromptWithDefault(label, defaultValue string) (string, error) {
	validate := func(input string) error {
		if strings.TrimSpace(input) == "" && defaultValue == "" {
			return fmt.Errorf("input cannot be empty")
		}
		return nil
	}
	result, err := p.Runner.RunPrompt(label, defaultValue, validate)
	err = handlePromptError(err)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(result) == "" {
		return defaultValue, nil
	}
	return result, nil
}

func (b *PromptUI) PromptForRegion(defaultRegion string) (string, error) {
	return b.Prompt.PromptForInputWithValidation(
		fmt.Sprintf("SSO region (Default: %s):", defaultRegion),
		defaultRegion,
		func(input string) error {
			if !generalutils.IsRegionValid(input) {
				return fmt.Errorf("invalid AWS region format or unrecognized region: %s", input)
			}
			return nil
		},
	)
}

func (p *PromptUI) PromptRequired(label string) (string, error) {
	validate := func(input string) error {
		if strings.TrimSpace(input) == "" {
			return fmt.Errorf("input is required")
		}
		return ValidateStartURLFunc(input)
	}
	result, err := p.Runner.RunPrompt(label, "", validate)
	err = handlePromptError(err)
	if err != nil {
		return "", err
	}
	return result, nil
}

func (p *PromptUI) SelectFromList(label string, items []string) (string, error) {
	result, err := p.Runner.RunSelect(label, items)
	err = handlePromptError(err)
	if err != nil {
		return "", err
	}
	return result, nil
}

func (p *PromptUI) PromptYesNo(label string, defaultValue bool) (bool, error) {
	defaultStr := "n"
	if defaultValue {
		defaultStr = "y"
	}
	validate := func(input string) error {
		input = strings.ToLower(strings.TrimSpace(input))
		if input != "" && input != "y" && input != "n" {
			return fmt.Errorf("input must be 'y' or 'n'")
		}
		return nil
	}
	result, err := p.Runner.RunPrompt(label, defaultStr, validate)
	err = handlePromptError(err)
	if err != nil {
		return false, err
	}
	result = strings.ToLower(strings.TrimSpace(result))
	if result == "" {
		result = defaultStr
	}
	return result == "y", nil
}

func NewPrompter() Prompter {
	return &PromptUI{Runner: &RealPromptRunner{}, Prompt: promptUtils.NewPrompt()}
}
