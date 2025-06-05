package promptUtils

import (
	"errors"
	"fmt"

	"github.com/manifoldco/promptui"
)

type Prompter interface {
	PromptForSelection(label string, items []string) (string, error)
	PromptForInput(label, defaultValue string) (string, error)
	PromptForInputWithValidation(prompt, defaultValue string, validate func(string) error) (string, error)
}

type RealPrompter struct{}

var ErrInterrupted = errors.New("operation interrupted")

func handlePromptError(err error) error {
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

	err = handlePromptError(err)
	if err != nil {
		if errors.Is(err, ErrInterrupted) {
			return "", ErrInterrupted
		}
		return "", err
	}

	return selected, nil
}

func (p *RealPrompter) PromptForInput(label, defaultValue string) (string, error) {
	prompt := promptui.Prompt{
		Label:   label,
		Default: defaultValue,
	}

	result, err := prompt.Run()

	err = handlePromptError(err)
	if err != nil {
		if errors.Is(err, ErrInterrupted) {
			return "", ErrInterrupted
		}
		return "", fmt.Errorf("input prompt failed: %w", err)
	}

	return result, nil
}

func (p *RealPrompter) PromptForInputWithValidation(prompt, defaultValue string, validate func(string) error) (string, error) {
	for {
		input, err := p.PromptForInput(prompt, defaultValue)
		if err != nil {
			return "", err
		}

		if err := validate(input); err != nil {
			fmt.Printf("Invalid input: %v\n", err)
			continue
		}

		return input, nil
	}
}

func NewPrompt() Prompter {
	return &RealPrompter{}
}
