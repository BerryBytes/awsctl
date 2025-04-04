package common

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

type SSHExecutorInterface interface {
	Execute(args []string, stdin io.Reader, stdout, stderr io.Writer) error
}

type RealSSHExecutor struct{}

func (e *RealSSHExecutor) Execute(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	cmd := exec.Command("ssh", args...)
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}

type SSHCommandBuilder struct {
	host     string
	user     string
	keyPath  string
	baseArgs []string
}

func NewSSHCommandBuilder(host, user, keyPath string) *SSHCommandBuilder {
	return &SSHCommandBuilder{
		host:    host,
		user:    user,
		keyPath: keyPath,
		baseArgs: []string{
			"-i", keyPath,
			"-o", "ConnectTimeout=15",
			"-o", "BatchMode=yes",
			"-o", "StrictHostKeyChecking=ask",
		},
	}
}

func (b *SSHCommandBuilder) WithForwarding(localPort int, remoteHost string, remotePort int) *SSHCommandBuilder {
	b.baseArgs = append(b.baseArgs, "-L", fmt.Sprintf("%d:%s:%d", localPort, remoteHost, remotePort))
	return b
}

func (b *SSHCommandBuilder) WithSOCKS(localPort int) *SSHCommandBuilder {
	b.baseArgs = append(b.baseArgs, "-D", strconv.Itoa(localPort))
	return b
}

func (b *SSHCommandBuilder) WithBackground() *SSHCommandBuilder {
	b.baseArgs = append(b.baseArgs, "-N", "-f")
	return b
}

func (b *SSHCommandBuilder) Build() []string {
	target := fmt.Sprintf("%s@%s", b.user, b.host)
	return append(b.baseArgs, target)
}

func ExecuteSSHCommand(executor SSHExecutorInterface, args []string) error {
	var stderrBuf bytes.Buffer
	err := executor.Execute(args, os.Stdin, os.Stdout, &stderrBuf)

	if err != nil {
		errorOutput := stderrBuf.String()
		return interpretSSHError(err, errorOutput, args)
	}
	return nil
}

func interpretSSHError(err error, errorOutput string, args []string) error {
	var keyPath, user, host string

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-i":
			if i+1 < len(args) {
				keyPath = args[i+1]
			}
		}
		if strings.Contains(args[i], "@") {
			parts := strings.Split(args[i], "@")
			if len(parts) == 2 {
				user = parts[0]
				host = parts[1]
			}
		}
	}

	switch {
	case strings.Contains(errorOutput, "Permission denied"):
		if strings.Contains(errorOutput, "publickey") {
			return fmt.Errorf("SSH authentication failed: invalid SSH key at %s or key not authorized on server", keyPath)
		}
		return fmt.Errorf("SSH authentication failed: invalid credentials for user %s", user)

	case strings.Contains(errorOutput, "Connection timed out"), strings.Contains(errorOutput, "No route to host"):
		return fmt.Errorf("network connection failed: cannot reach host %s (check IP/network connectivity)", host)

	case strings.Contains(errorOutput, "Could not resolve hostname"):
		return fmt.Errorf("invalid hostname: %s cannot be resolved", host)

	case strings.Contains(errorOutput, "Host key verification failed"):
		return fmt.Errorf("host key verification failed for %s (try removing the host from known_hosts file)", host)

	default:
		return fmt.Errorf("SSH connection failed: %w\nCommand: ssh %s\nError output: %s",
			err, strings.Join(args, " "), errorOutput)
	}
}

func ValidateSSHKey(fs FileSystemInterface, keyPath string) error {
	fileInfo, err := fs.Stat(keyPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("SSH key file does not exist at %s", keyPath)
		}
		return fmt.Errorf("failed to access SSH key file: %w", err)
	}

	mode := fileInfo.Mode()
	if mode.Perm()&0077 != 0 {
		return fmt.Errorf("insecure SSH key permissions %#o (should be 600 or 400)", mode.Perm())
	}

	content, err := fs.ReadFile(keyPath)
	if err != nil {
		return fmt.Errorf("failed to read SSH key file: %w", err)
	}

	if !strings.Contains(string(content), "PRIVATE KEY") {
		return fmt.Errorf("file does not appear to be a valid SSH private key")
	}

	return nil
}
