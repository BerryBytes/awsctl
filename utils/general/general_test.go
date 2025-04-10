package generalutils

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCheckAWSCLI_NotFound(t *testing.T) {
	originalPath := os.Getenv("PATH")
	defer os.Setenv("PATH", originalPath)

	os.Setenv("PATH", "/nonexistent")
	manager := &DefaultGeneralUtilsManager{}
	err := manager.CheckAWSCLI()

	assert.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "AWS CLI not found"))
}
func TestCheckAWSCLI(t *testing.T) {
	t.Run("AWS CLI found", func(t *testing.T) {
		oldLookPath := execLookPath
		defer func() { execLookPath = oldLookPath }()
		execLookPath = func(name string) (string, error) {
			return "/usr/bin/aws", nil
		}

		manager := &DefaultGeneralUtilsManager{}
		err := manager.CheckAWSCLI()
		assert.NoError(t, err)
	})

}

var execLookPath = exec.LookPath

func TestHandleSignals(t *testing.T) {
	manager := &DefaultGeneralUtilsManager{}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	ctx := manager.HandleSignals()

	err := syscall.Kill(syscall.Getpid(), syscall.SIGINT)
	if err != nil {
		t.Fatalf("Failed to send signal: %v", err)
	}

	select {
	case <-ctx.Done():
		assert.Error(t, ctx.Err(), "context should be cancelled")
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for signal handling")
	}

	w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	_, err = io.Copy(&buf, r)
	if err != nil {
		t.Fatalf("Failed to copy output: %v", err)
	}

	assert.Contains(t, buf.String(), "Received termination signal")

	signal.Reset()
}

func TestPrintCurrentRole(t *testing.T) {
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	manager := &DefaultGeneralUtilsManager{}
	manager.PrintCurrentRole(
		"test-profile",
		"123456789012",
		"Test Account",
		"TestRole",
		"arn:aws:iam::123456789012:role/TestRole",
		"2023-12-31T23:59:59Z",
	)

	w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	_, err := io.Copy(&buf, r)
	if err != nil {
		t.Fatalf("Failed to copy output: %v", err)
	}

	output := buf.String()
	assert.Contains(t, output, "AWS Session Details")
	assert.Contains(t, output, "Profile      : test-profile")
	assert.Contains(t, output, "Account Id   : 123456789012")
	assert.Contains(t, output, "Account Name : Test Account")
	assert.Contains(t, output, "Role Name    : TestRole")
	assert.Contains(t, output, "Role ARN     : arn:aws:iam::123456789012:role/TestRole")
	assert.Contains(t, output, "Expiration   : 2023-12-31T23:59:59Z")
}

func TestNewGeneralUtilsManager(t *testing.T) {
	manager := NewGeneralUtilsManager()
	assert.NotNil(t, manager)
	_, ok := manager.(*DefaultGeneralUtilsManager)
	assert.True(t, ok, "should return DefaultGeneralUtilsManager")
}
