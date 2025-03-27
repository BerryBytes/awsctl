package promptutils

import (
	"errors"
	"fmt"
	"strings"

	"github.com/manifoldco/promptui"
)

type Prompter interface {
	PromptForSelection(label string, items []string) (string, error)
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
