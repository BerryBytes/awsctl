package common

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"

	"github.com/BerryBytes/awsctl/internal/sso"
)

type SSHExecutorInterface interface {
	Execute(args []string, stdin io.Reader, stdout, stderr io.Writer) error
}

type OSDetector interface {
	GetOS() string
}

type RuntimeOSDetector struct{}

func (r RuntimeOSDetector) GetOS() string {
	return runtime.GOOS
}

type RealSSHExecutor struct {
	commandExecutor sso.CommandExecutor
}

func NewRealSSHExecutor(commandExecutor sso.CommandExecutor) *RealSSHExecutor {
	return &RealSSHExecutor{commandExecutor: commandExecutor}
}

type SOCKSProxyConfig struct {
	Executor    SSHExecutorInterface
	Host        string
	User        string
	KeyPath     string
	DefaultPort int
}

type SSHCommandBuilder struct {
	host     string
	user     string
	keyPath  string
	baseArgs []string
}

func NewSSHCommandBuilder(host, user, keyPath string, useInstanceConnect bool) *SSHCommandBuilder {
	args := []string{
		"-i", keyPath,
	}

	if useInstanceConnect {
		args = append(args,
			"-o", "BatchMode=yes",
			"-o", "ConnectTimeout=10",
			"-o", "ServerAliveInterval=15",
			"-o", "StrictHostKeyChecking=no",
			"-o", "UserKnownHostsFile=/dev/null",
		)

		if strings.HasPrefix(host, "i-") {
			args = append(args,
				"-o", fmt.Sprintf("ProxyCommand=aws ec2-instance-connect open-tunnel --instance-id %s", host),
			)
		}
	} else {
		args = append(args,
			"-o", "BatchMode=no",
			"-o", "ConnectTimeout=30",
			"-o", "StrictHostKeyChecking=ask",
			"-o", "ServerAliveInterval=60",
		)
	}

	return &SSHCommandBuilder{
		host:     host,
		user:     user,
		keyPath:  keyPath,
		baseArgs: args,
	}
}

func (e *RealSSHExecutor) Execute(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("no command provided")
	}
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}

func (b *SSHCommandBuilder) WithForwarding(localPort int, remoteHost string, remotePort int) *SSHCommandBuilder {
	b.baseArgs = append(b.baseArgs, "-N", "-T", "-L", fmt.Sprintf("%d:%s:%d", localPort, remoteHost, remotePort))
	return b
}

func (b *SSHCommandBuilder) WithSOCKS(localPort int) *SSHCommandBuilder {
	b.baseArgs = append(b.baseArgs, "-N", "-T", "-D", strconv.Itoa(localPort))
	return b
}

func (b *SSHCommandBuilder) WithBackground() *SSHCommandBuilder {
	b.baseArgs = append(b.baseArgs, "-N", "-f")
	return b
}

func (b *SSHCommandBuilder) Build() []string {
	target := b.host
	if strings.HasPrefix(b.host, "i-") && containsProxyCommand(b.baseArgs) {
		target = "127.0.0.1"
	}

	target = fmt.Sprintf("%s@%s", b.user, target)
	cmd := []string{"ssh"}
	cmd = append(cmd, b.baseArgs...)
	cmd = append(cmd, target)
	return cmd
}

func ExecuteSSHCommand(executor SSHExecutorInterface, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no command provided")
	}

	var stderrBuf bytes.Buffer
	err := executor.Execute(args, os.Stdin, os.Stdout, &stderrBuf)

	if err != nil {
		errorOutput := stderrBuf.String()
		return interpretSSHError(err, errorOutput, args)
	}
	return nil
}

func interpretSSHError(err error, errorOutput string, args []string) error {
	if exitErr, ok := err.(*exec.ExitError); ok {
		switch exitErr.ExitCode() {
		case 0, 1, 2, 130, 143:
			return nil
		}
	}
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
		return fmt.Errorf("SSH connection failed: %w\nCommand: %s\nError output: %s",
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

func TerminateSOCKSProxy(executor SSHExecutorInterface, port int, osDetector OSDetector) error {
	if executor == nil {
		return fmt.Errorf("executor cannot be nil")
	}
	if port < 1 || port > 65535 {
		return fmt.Errorf("invalid port number: %d", port)
	}

	var cmd []string
	switch osDetector.GetOS() {
	case "linux", "darwin":
		cmd = []string{
			"sh", "-c",
			fmt.Sprintf("pkill -f 'ssh.*-D.*%d'", port),
		}
	case "windows":
		return terminateSOCKSProxyWindows(executor, port)
	default:
		return fmt.Errorf("unsupported operating system: %s", osDetector.GetOS())
	}

	err := executor.Execute(cmd, nil, nil, nil)
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return nil
		}
		return fmt.Errorf("failed to terminate SOCKS proxy on port %d: %w", port, err)
	}

	return nil
}

func terminateSOCKSProxyWindows(executor SSHExecutorInterface, port int) error {
	netstatCmd := []string{
		"cmd", "/c",
		fmt.Sprintf("netstat -aon | findstr :%d", port),
	}

	var stdout, stderr strings.Builder
	err := executor.Execute(netstatCmd, nil, &stdout, &stderr)
	if err != nil {
		if stdout.Len() == 0 && stderr.Len() > 0 {
			return fmt.Errorf("failed to find SOCKS proxy process on port %d: %s", port, stderr.String())
		}
		return nil
	}

	output := stdout.String()
	if output == "" {
		return nil
	}

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) >= 5 && strings.Contains(fields[1], ":"+strconv.Itoa(port)) {
			pidStr := fields[4]
			pid, err := strconv.Atoi(pidStr)
			if err != nil {
				return fmt.Errorf("invalid PID in netstat output: %s", pidStr)
			}

			killCmd := []string{
				"taskkill", "/PID", strconv.Itoa(pid), "/F",
			}
			err = executor.Execute(killCmd, nil, nil, nil)
			if err != nil {
				return fmt.Errorf("failed to kill SOCKS proxy process (PID %d) on port %d: %w", pid, port, err)
			}
			return nil
		}
	}

	return nil
}

func containsProxyCommand(args []string) bool {
	for i, arg := range args {
		if arg == "-o" && i+1 < len(args) && strings.Contains(args[i+1], "ProxyCommand") {
			return true
		}
	}
	return false
}
