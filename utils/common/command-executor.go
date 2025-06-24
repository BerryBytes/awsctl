package common

import (
	"context"
	"os"
	"os/exec"
	"strings"
)

type RealCommandExecutor struct{}

func (e *RealCommandExecutor) RunCommand(name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	return cmd.Output()
}

func (e *RealCommandExecutor) RunInteractiveCommand(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (e *RealCommandExecutor) RunCommandWithInput(name string, input string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	cmd.Stdin = strings.NewReader(input)
	return cmd.Output()
}

func (e *RealCommandExecutor) LookPath(file string) (string, error) {
	return exec.LookPath(file)
}
