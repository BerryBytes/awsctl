package common_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"

	mock_awsctl "github.com/BerryBytes/awsctl/tests/mock"
	"github.com/BerryBytes/awsctl/utils/common"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

type mockFileInfo struct {
	mode os.FileMode
}

func (m mockFileInfo) Name() string       { return "testfile" }
func (m mockFileInfo) Size() int64        { return 0 }
func (m mockFileInfo) Mode() os.FileMode  { return m.mode }
func (m mockFileInfo) ModTime() time.Time { return time.Time{} }
func (m mockFileInfo) IsDir() bool        { return false }
func (m mockFileInfo) Sys() interface{}   { return nil }

func TestRuntimeOSDetector_GetOS(t *testing.T) {
	detector := common.RuntimeOSDetector{}
	os := detector.GetOS()
	assert.Equal(t, runtime.GOOS, os)
}
func TestRealSSHExecutor(t *testing.T) {
	t.Run("Execute success", func(t *testing.T) {
		executor := &common.RealSSHExecutor{}
		var stdout, stderr bytes.Buffer

		err := executor.Execute([]string{"echo", "hello"}, strings.NewReader(""), &stdout, &stderr)
		assert.NoError(t, err)
		assert.Equal(t, "hello\n", stdout.String())
	})

	t.Run("Execute failure", func(t *testing.T) {
		executor := &common.RealSSHExecutor{}
		var stdout, stderr bytes.Buffer

		err := executor.Execute([]string{"false"}, nil, &stdout, &stderr)
		assert.Error(t, err)
	})
}

func TestExecuteSSHCommand_EmptyCommand(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockExecutor := mock_awsctl.NewMockSSHExecutorInterface(ctrl)
	err := common.ExecuteSSHCommand(mockExecutor, []string{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no command provided")
}

func TestSSHCommandBuilder(t *testing.T) {
	t.Run("Basic command", func(t *testing.T) {
		builder := common.NewSSHCommandBuilder("example.com", "user", "/path/to/key", false)
		cmd := builder.Build()

		expected := []string{
			"ssh",
			"-i", "/path/to/key",
			"-o", "BatchMode=no",
			"-o", "ConnectTimeout=30",
			"-o", "StrictHostKeyChecking=ask",
			"-o", "ServerAliveInterval=60",
			"user@example.com",
		}
		assert.Equal(t, expected, cmd)
	})

	t.Run("With instance connect", func(t *testing.T) {
		builder := common.NewSSHCommandBuilder("example.com", "user", "/path/to/key", true)
		cmd := builder.Build()

		expected := []string{
			"ssh", "-i", "/path/to/key", "-o", "BatchMode=yes", "-o", "ConnectTimeout=10",
			"-o", "ServerAliveInterval=15", "-o", "StrictHostKeyChecking=no",
			"-o", "UserKnownHostsFile=/dev/null", "user@example.com",
		}

		assert.Equal(t, expected, cmd)
	})

	t.Run("With port forwarding", func(t *testing.T) {
		builder := common.NewSSHCommandBuilder("example.com", "user", "/path/to/key", false)
		cmd := builder.WithForwarding(8080, "localhost", 80).Build()

		expected := []string{"ssh", "-i", "/path/to/key", "-o", "BatchMode=no", "-o", "ConnectTimeout=30",
			"-o", "StrictHostKeyChecking=ask", "-o", "ServerAliveInterval=60",
			"-N", "-T", "-L", "8080:localhost:80", "user@example.com"}

		assert.Equal(t, expected, cmd)

	})

	t.Run("With SOCKS proxy", func(t *testing.T) {
		builder := common.NewSSHCommandBuilder("example.com", "user", "/path/to/key", false)
		cmd := builder.WithSOCKS(1080).Build()

		expected := []string{"ssh", "-i", "/path/to/key", "-o", "BatchMode=no", "-o",
			"ConnectTimeout=30", "-o", "StrictHostKeyChecking=ask", "-o", "ServerAliveInterval=60", "-N", "-T",
			"-D", "1080", "user@example.com"}

		assert.Equal(t, expected, cmd)
	})

	t.Run("With background", func(t *testing.T) {
		builder := common.NewSSHCommandBuilder("example.com", "user", "/path/to/key", false)
		cmd := builder.WithBackground().Build()

		expected := []string{"ssh", "-i", "/path/to/key", "-o", "BatchMode=no", "-o",
			"ConnectTimeout=30", "-o", "StrictHostKeyChecking=ask",
			"-o", "ServerAliveInterval=60",
			"-N", "-f", "user@example.com"}

		assert.Equal(t, expected, cmd)
	})
}

func TestExecuteSSHCommand(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tests := []struct {
		name          string
		mockSetup     func(*mock_awsctl.MockSSHExecutorInterface)
		cmd           []string
		expectedError string
	}{
		{
			name: "success",
			mockSetup: func(m *mock_awsctl.MockSSHExecutorInterface) {
				m.EXPECT().Execute(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			},
			cmd: []string{"ssh", "-i", "/test/key", "user@host"},
		},
		{
			name: "permission denied - publickey",
			mockSetup: func(m *mock_awsctl.MockSSHExecutorInterface) {
				m.EXPECT().Execute(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
						_, _ = stderr.Write([]byte("Permission denied (publickey)"))
						return &exec.ExitError{}
					})
			},
			cmd:           []string{"ssh", "-i", "/test/key", "user@host"},
			expectedError: "SSH authentication failed: invalid SSH key at /test/key",
		},
		{
			name: "permission denied - password",
			mockSetup: func(m *mock_awsctl.MockSSHExecutorInterface) {
				m.EXPECT().Execute(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
						_, _ = stderr.Write([]byte("Permission denied (password)"))
						return &exec.ExitError{}
					})
			},
			cmd:           []string{"ssh", "-i", "/test/key", "user@host"},
			expectedError: "SSH authentication failed: invalid credentials for user user",
		},
		{
			name: "connection timed out",
			mockSetup: func(m *mock_awsctl.MockSSHExecutorInterface) {
				m.EXPECT().Execute(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
						_, _ = stderr.Write([]byte("Connection timed out"))
						return &exec.ExitError{}
					})
			},
			cmd:           []string{"ssh", "-i", "/test/key", "user@host"},
			expectedError: "network connection failed: cannot reach host host",
		},
		{
			name: "no route to host",
			mockSetup: func(m *mock_awsctl.MockSSHExecutorInterface) {
				m.EXPECT().Execute(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
						_, _ = stderr.Write([]byte("No route to host"))
						return &exec.ExitError{}
					})
			},
			cmd:           []string{"ssh", "-i", "/test/key", "user@host"},
			expectedError: "network connection failed: cannot reach host host",
		},
		{
			name: "could not resolve hostname",
			mockSetup: func(m *mock_awsctl.MockSSHExecutorInterface) {
				m.EXPECT().Execute(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
						_, _ = stderr.Write([]byte("Could not resolve hostname"))
						return &exec.ExitError{}
					})
			},
			cmd:           []string{"ssh", "-i", "/test/key", "user@invalidhost"},
			expectedError: "invalid hostname: invalidhost cannot be resolved",
		},
		{
			name: "host key verification failed",
			mockSetup: func(m *mock_awsctl.MockSSHExecutorInterface) {
				m.EXPECT().Execute(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
						_, _ = stderr.Write([]byte("Host key verification failed"))
						return &exec.ExitError{}
					})
			},
			cmd:           []string{"ssh", "-i", "/test/key", "user@host"},
			expectedError: "host key verification failed for host",
		},
		{
			name: "unknown error",
			mockSetup: func(m *mock_awsctl.MockSSHExecutorInterface) {
				m.EXPECT().Execute(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
						_, _ = stderr.Write([]byte("Some unknown error"))
						return fmt.Errorf("unknown error")
					})
			},
			cmd:           []string{"ssh", "-i", "/test/key", "user@host"},
			expectedError: "SSH connection failed: unknown error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExecutor := mock_awsctl.NewMockSSHExecutorInterface(ctrl)
			tt.mockSetup(mockExecutor)

			err := common.ExecuteSSHCommand(mockExecutor, tt.cmd)

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)

				if tt.name == "unknown error" {
					assert.Contains(t, err.Error(), "Command: ssh -i /test/key user@host")
					assert.Contains(t, err.Error(), "Error output: Some unknown error")
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
func TestValidateSSHKey(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tests := []struct {
		name          string
		mockSetup     func(*mock_awsctl.MockFileSystemInterface)
		keyPath       string
		expectedError string
	}{
		{
			name: "valid key",
			mockSetup: func(m *mock_awsctl.MockFileSystemInterface) {
				m.EXPECT().Stat("/test/key").Return(&mockFileInfo{mode: 0600}, nil)
				m.EXPECT().ReadFile("/test/key").Return([]byte("-----BEGIN OPENSSH PRIVATE KEY-----\nFAKE\n-----END OPENSSH PRIVATE KEY-----"), nil)
			},
			keyPath: "/test/key",
		},
		{
			name: "file not found",
			mockSetup: func(m *mock_awsctl.MockFileSystemInterface) {
				m.EXPECT().Stat("/missing/key").Return(nil, os.ErrNotExist)
			},
			keyPath:       "/missing/key",
			expectedError: "SSH key file does not exist",
		},
		{
			name: "insecure permissions",
			mockSetup: func(m *mock_awsctl.MockFileSystemInterface) {
				m.EXPECT().Stat("/insecure/key").Return(&mockFileInfo{mode: 0644}, nil)
			},
			keyPath:       "/insecure/key",
			expectedError: "insecure SSH key permissions",
		},
		{
			name: "invalid key format",
			mockSetup: func(m *mock_awsctl.MockFileSystemInterface) {
				m.EXPECT().Stat("/invalid/key").Return(&mockFileInfo{mode: 0600}, nil)
				m.EXPECT().ReadFile("/invalid/key").Return([]byte("NOT A VALID KEY"), nil)
			},
			keyPath:       "/invalid/key",
			expectedError: "file does not appear to be a valid SSH private key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockFS := mock_awsctl.NewMockFileSystemInterface(ctrl)
			tt.mockSetup(mockFS)

			err := common.ValidateSSHKey(mockFS, tt.keyPath)

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateSSHKey_AccessError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockFS := mock_awsctl.NewMockFileSystemInterface(ctrl)
	mockFS.EXPECT().Stat("/test/key").Return(nil, errors.New("permission denied"))

	err := common.ValidateSSHKey(mockFS, "/test/key")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to access SSH key file")
}

func TestValidateSSHKey_ReadError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockFS := mock_awsctl.NewMockFileSystemInterface(ctrl)
	mockFS.EXPECT().Stat("/test/key").Return(&mockFileInfo{mode: 0600}, nil)
	mockFS.EXPECT().ReadFile("/test/key").Return(nil, errors.New("read error"))

	err := common.ValidateSSHKey(mockFS, "/test/key")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read SSH key file")
}
