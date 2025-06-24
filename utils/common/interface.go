package common

import (
	"context"
	"io"
	"os"
)

type FileSystemInterface interface {
	Stat(name string) (os.FileInfo, error)
	ReadFile(name string) ([]byte, error)
	UserHomeDir() (string, error)
	Remove(name string) error
	MkdirAll(path string, perm os.FileMode) error
	WriteFile(name string, data []byte, perm os.FileMode) error
}

type CommandExecutor interface {
	RunCommand(name string, args ...string) ([]byte, error)
	RunInteractiveCommand(ctx context.Context, name string, args ...string) error
	LookPath(file string) (string, error)
	RunCommandWithInput(name string, input string, args ...string) ([]byte, error)
}

type SSHExecutorInterface interface {
	Execute(args []string, stdin io.Reader, stdout, stderr io.Writer) error
}

type OSDetector interface {
	GetOS() string
}
