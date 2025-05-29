package sso

import (
	"errors"
	"fmt"
	"strings"

	promptUtils "github.com/BerryBytes/awsctl/utils/prompt"
	"github.com/manifoldco/promptui"
)

type PromptUI struct{}

func handlePromptError(err error) error {
	if err != nil {
		if errors.Is(err, promptui.ErrInterrupt) {
			fmt.Println("\nReceived termination signal. Exiting.")
			return promptUtils.ErrInterrupted
		}
		return fmt.Errorf("prompt failed: %w", err)
	}
	return nil
}

func (p *PromptUI) PromptWithDefault(label, defaultValue string) (string, error) {
	prompt := promptui.Prompt{
		Label:   label,
		Default: defaultValue,
		Validate: func(input string) error {
			if strings.TrimSpace(input) == "" {
				return fmt.Errorf("input cannot be empty")
			}
			return nil
		},
	}
	result, err := prompt.Run()
	err = handlePromptError(err)
	if err != nil {
		if errors.Is(err, promptUtils.ErrInterrupted) {
			return "", promptUtils.ErrInterrupted
		}
		return "", err
	}
	return result, nil
}

func (p *PromptUI) PromptRequired(label string) (string, error) {
	prompt := promptui.Prompt{
		Label: label,
		Validate: func(input string) error {
			if strings.TrimSpace(input) == "" {
				return fmt.Errorf("input is required")
			}
			return validateStartURL(input)
		},
	}
	result, err := prompt.Run()
	err = handlePromptError(err)
	if err != nil {
		if errors.Is(err, promptUtils.ErrInterrupted) {
			return "", promptUtils.ErrInterrupted
		}
		return "", err
	}
	return result, nil
}

func (p *PromptUI) SelectFromList(label string, items []string) (string, error) {
	selectPrompt := promptui.Select{
		Label: label,
		Items: items,
	}
	_, result, err := selectPrompt.Run()
	err = handlePromptError(err)
	if err != nil {
		if errors.Is(err, promptUtils.ErrInterrupted) {
			return "", promptUtils.ErrInterrupted
		}
		return "", err
	}
	return result, nil
}

func (p *PromptUI) PromptYesNo(label string, defaultValue bool) (bool, error) {
	defaultStr := "n"
	if defaultValue {
		defaultStr = "y"
	}
	prompt := promptui.Prompt{
		Label:   label,
		Default: defaultStr,
		Validate: func(input string) error {
			input = strings.ToLower(strings.TrimSpace(input))
			if input != "" && input != "y" && input != "n" {
				return fmt.Errorf("input must be 'y' or 'n'")
			}
			return nil
		},
	}
	result, err := prompt.Run()
	err = handlePromptError(err)
	if err != nil {
		if errors.Is(err, promptUtils.ErrInterrupted) {
			return false, promptUtils.ErrInterrupted
		}
		return false, err
	}
	result = strings.ToLower(strings.TrimSpace(result))
	if result == "" {
		result = defaultStr
	}
	return result == "y", nil
}

func NewPrompter() Prompter {
	return &PromptUI{}
}
