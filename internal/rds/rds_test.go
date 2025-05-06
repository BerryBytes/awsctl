package rds_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/BerryBytes/awsctl/internal/rds"
	"github.com/BerryBytes/awsctl/models"
	mock_awsctl "github.com/BerryBytes/awsctl/tests/mock"
	mock_rds "github.com/BerryBytes/awsctl/tests/mock/rds"
	promptUtils "github.com/BerryBytes/awsctl/utils/prompt"
	"github.com/aws/aws-sdk-go-v2/aws"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func setupTest(t *testing.T) (*rds.RDSService, *gomock.Controller, *mock_rds.MockRDSPromptInterface, *mock_rds.MockRDSAdapterInterface, *mock_awsctl.MockConnectionPrompter, *mock_awsctl.MockServicesInterface, *mock_awsctl.MockPrompter) {
	ctrl := gomock.NewController(t)
	mockRPrompter := mock_rds.NewMockRDSPromptInterface(ctrl)
	mockRDSClient := mock_rds.NewMockRDSAdapterInterface(ctrl)
	mockConnPrompter := mock_awsctl.NewMockConnectionPrompter(ctrl)
	mockConnServices := mock_awsctl.NewMockServicesInterface(ctrl)
	mockGPrompter := mock_awsctl.NewMockPrompter(ctrl)
	mockFs := mock_awsctl.NewMockFileSystemInterface(ctrl)
	mockSSHExecutor := mock_awsctl.NewMockSSHExecutorInterface(ctrl)
	mockOsDetector := mock_awsctl.NewMockOSDetector(ctrl)

	service := &rds.RDSService{
		RPrompter:    mockRPrompter,
		RDSClient:    mockRDSClient,
		CPrompter:    mockConnPrompter,
		ConnServices: mockConnServices,
		GPrompter:    mockGPrompter,
		Fs:           mockFs,
		SSHExecutor:  mockSSHExecutor,
		OsDetector:   mockOsDetector,
	}

	return service, ctrl, mockRPrompter, mockRDSClient, mockConnPrompter, mockConnServices, mockGPrompter
}

type mockFileInfo struct {
	name    string
	size    int64
	mode    os.FileMode
	modTime time.Time
	isDir   bool
}

func (m *mockFileInfo) Name() string       { return m.name }
func (m *mockFileInfo) Size() int64        { return m.size }
func (m *mockFileInfo) Mode() os.FileMode  { return m.mode }
func (m *mockFileInfo) ModTime() time.Time { return m.modTime }
func (m *mockFileInfo) IsDir() bool        { return m.isDir }
func (m *mockFileInfo) Sys() interface{}   { return nil }

type mockTransport struct {
	resp *http.Response
	err  error
}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return m.resp, m.err
}

type errorReader struct {
	err error
}

func (r *errorReader) Read(p []byte) (n int, err error) {
	return 0, r.err
}

func TestRDSService_Run(t *testing.T) {
	svc, ctrl, mockRPrompter, _, _, _, _ := setupTest(t)
	defer ctrl.Finish()

	t.Run("ExitAction", func(t *testing.T) {
		mockRPrompter.EXPECT().SelectRDSAction().Return(rds.ExitRDS, nil)

		err := svc.Run()
		assert.NoError(t, err)
	})

	t.Run("Interrupted", func(t *testing.T) {
		mockRPrompter.EXPECT().SelectRDSAction().Return(rds.ExitRDS, promptUtils.ErrInterrupted)

		err := svc.Run()
		assert.Equal(t, promptUtils.ErrInterrupted, err)
	})
}

func TestHandleDirectConnection_Success(t *testing.T) {
	svc, ctrl, mockRPrompter, mockRDSAdapter, mockConnPrompter, mockConnServices, _ := setupTest(t)
	defer ctrl.Finish()

	mockConnServices.EXPECT().IsAWSConfigured().Return(true)
	mockConnPrompter.EXPECT().PromptForConfirmation("Look for RDS instances in AWS?").Return(false, nil)
	mockRPrompter.EXPECT().PromptForManualEndpoint().Return("localhost:5432", "admin", "us-west-2", nil)
	mockRDSAdapter.EXPECT().GenerateAuthToken("localhost:5432", "admin", "us-west-2").Return("mock-auth-token", nil)
	err := svc.HandleDirectConnection()
	assert.NoError(t, err)
}

func TestRDSService_HandleTunnelConnection(t *testing.T) {

	t.Run("HappyPath", func(t *testing.T) {
		svc, ctrl, mockRPrompter, mockRDSAdapter, mockConnPrompter, mockConnServices, mockGPrompter := setupTest(t)
		defer ctrl.Finish()

		mockFs := svc.Fs.(*mock_awsctl.MockFileSystemInterface)
		mockFs.EXPECT().UserHomeDir().Return("/home/test", nil)
		mockFs.EXPECT().MkdirAll(gomock.Any(), gomock.Any()).Return(nil)
		mockFs.EXPECT().WriteFile(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(path string, data []byte, perm os.FileMode) error {
			if strings.Contains(path, "mysql-config") {
				assert.Contains(t, string(data), "[client]")
				assert.Contains(t, string(data), "host=127.0.0.1")
				assert.Contains(t, string(data), "port=3306")
				assert.Contains(t, string(data), "user=test-user")
				assert.Contains(t, string(data), "password=mock-auth-token")
				assert.Contains(t, string(data), "ssl-ca=/home/test/.rds-certs/us-east-1-bundle.pem")
			}
			return nil
		}).Return(nil)

		transport := &mockTransport{
			resp: &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte("mock-cert"))),
			},
			err: nil,
		}
		client := &http.Client{Transport: transport}
		origClient := http.DefaultClient
		http.DefaultClient = client
		defer func() { http.DefaultClient = origClient }()

		mockRPrompter.EXPECT().PromptForAuthMethod("Select authentication method for RDS:", []string{"Token", "Native password"}).Return("Token", nil)
		mockConnServices.EXPECT().IsAWSConfigured().Return(true)
		mockConnPrompter.EXPECT().PromptForConfirmation("Look for RDS instances in AWS?").Return(false, nil)
		mockRPrompter.EXPECT().PromptForManualEndpoint().Return("test-rds:3306", "test-user", "us-east-1", nil)
		mockConnPrompter.EXPECT().PromptForLocalPort("RDS", 3306).Return(3306, nil)
		mockRDSAdapter.EXPECT().GenerateAuthToken("test-rds:3306", "test-user", "us-east-1").Return("mock-auth-token", nil)
		mockGPrompter.EXPECT().PromptForSelection("To connect securely, an RDS SSL certificate is required:", []string{"Download certificate automatically", "Provide custom certificate path"}).Return("Download certificate automatically", nil)
		mockConnServices.EXPECT().StartPortForwarding(gomock.Any(), 3306, "test-rds", 3306).Return(
			func() {},
			func() {},
			nil,
		)

		done := make(chan error)
		go func() {
			err := svc.HandleTunnelConnection()
			done <- err
		}()

		time.Sleep(100 * time.Millisecond)
		if err := syscall.Kill(syscall.Getpid(), syscall.SIGINT); err != nil {
			t.Fatalf("failed to send SIGINT: %v", err)
		}

		select {
		case err := <-done:
			assert.NoError(t, err, "HandleTunnelConnection should return nil on SIGINT")
		case <-time.After(2 * time.Second):
			t.Fatal("Test timed out waiting for HandleTunnelConnection to complete")
		}
	})

	t.Run("Error_InvalidPort", func(t *testing.T) {
		svc, ctrl, mockRPrompter, _, mockConnPrompter, mockConnServices, _ := setupTest(t)
		defer ctrl.Finish()

		mockRPrompter.EXPECT().PromptForAuthMethod("Select authentication method for RDS:", []string{"Token", "Native password"}).Return("Token", nil)
		mockConnServices.EXPECT().IsAWSConfigured().Return(true)
		mockConnPrompter.EXPECT().PromptForConfirmation("Look for RDS instances in AWS?").Return(false, nil)
		mockRPrompter.EXPECT().PromptForManualEndpoint().Return("test-rds:invalid", "test-user", "us-east-1", nil)

		err := svc.HandleTunnelConnection()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid port")
	})

	t.Run("AWSNotConfigured", func(t *testing.T) {
		svc, ctrl, mockRPrompter, mockRDSAdapter, mockConnPrompter, mockConnServices, mockGPrompter := setupTest(t)
		defer ctrl.Finish()

		mockFs := svc.Fs.(*mock_awsctl.MockFileSystemInterface)
		mockFs.EXPECT().UserHomeDir().Return("/home/test", nil)
		mockFs.EXPECT().MkdirAll(gomock.Any(), gomock.Any()).Return(nil)
		mockFs.EXPECT().WriteFile(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(path string, data []byte, perm os.FileMode) error {
			if strings.Contains(path, "mysql-config") {
				assert.Contains(t, string(data), "[client]")
				assert.Contains(t, string(data), "host=127.0.0.1")
				assert.Contains(t, string(data), "port=3306")
				assert.Contains(t, string(data), "user=test-user")
				assert.Contains(t, string(data), "password=mock-auth-token")
				assert.Contains(t, string(data), "ssl-ca=/home/test/.rds-certs/us-east-1-bundle.pem")
			}
			return nil
		}).Return(nil)

		transport := &mockTransport{
			resp: &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte("mock-cert"))),
			},
			err: nil,
		}
		client := &http.Client{Transport: transport}
		origClient := http.DefaultClient
		http.DefaultClient = client
		defer func() { http.DefaultClient = origClient }()

		mockRPrompter.EXPECT().PromptForAuthMethod("Select authentication method for RDS:", []string{"Token", "Native password"}).Return("Token", nil)
		mockConnServices.EXPECT().IsAWSConfigured().Return(false)
		mockRPrompter.EXPECT().PromptForManualEndpoint().Return("test-rds:3306", "test-user", "us-east-1", nil)
		mockConnPrompter.EXPECT().PromptForLocalPort("RDS", 3306).Return(3306, nil)
		mockRDSAdapter.EXPECT().GenerateAuthToken("test-rds:3306", "test-user", "us-east-1").Return("mock-auth-token", nil)
		mockGPrompter.EXPECT().PromptForSelection("To connect securely, an RDS SSL certificate is required:", []string{"Download certificate automatically", "Provide custom certificate path"}).Return("Download certificate automatically", nil)
		mockConnServices.EXPECT().StartPortForwarding(gomock.Any(), 3306, "test-rds", 3306).Return(
			func() {},
			func() {},
			nil,
		)

		done := make(chan error)
		go func() {
			err := svc.HandleTunnelConnection()
			done <- err
		}()

		time.Sleep(100 * time.Millisecond)
		if err := syscall.Kill(syscall.Getpid(), syscall.SIGINT); err != nil {
			t.Fatalf("failed to send SIGINT: %v", err)
		}

		select {
		case err := <-done:
			assert.NoError(t, err, "HandleTunnelConnection should return nil on SIGINT")
		case <-time.After(2 * time.Second):
			t.Fatal("Test timed out waiting for HandleTunnelConnection to complete")
		}
	})
}

func TestRDSService_CleanupSOCKS(t *testing.T) {
	svc, ctrl, _, _, _, _, _ := setupTest(t)
	defer ctrl.Finish()

	t.Run("NoSOCKSPort", func(t *testing.T) {
		err := svc.CleanupSOCKS()
		assert.NoError(t, err)
	})

	t.Run("SuccessfulTermination", func(t *testing.T) {
		svc.SetSOCKSPort(1080)

		if svc.SSHExecutor == nil {
			t.Fatal("SSHExecutor is nil in SuccessfulTermination")
		}
		mockSSHExecutor, ok := svc.SSHExecutor.(*mock_awsctl.MockSSHExecutorInterface)
		if !ok {
			t.Fatalf("SSHExecutor is not *mock_awsctl.MockSSHExecutorInterface, got %T", svc.SSHExecutor)
		}

		if svc.OsDetector == nil {
			t.Fatal("OsDetector is nil in SuccessfulTermination")
		}
		mockOsDetector, ok := svc.OsDetector.(*mock_awsctl.MockOSDetector)
		if !ok {
			t.Fatalf("OsDetector is not *mock_awsctl.MockOSDetector, got %T", svc.OsDetector)
		}

		mockSSHExecutor.EXPECT().Execute(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
		mockOsDetector.EXPECT().GetOS().Return("linux")

		r, w, _ := os.Pipe()
		origStdout := os.Stdout
		os.Stdout = w
		defer func() {
			os.Stdout = origStdout
			_ = w.Close()
		}()

		err := svc.CleanupSOCKS()
		assert.NoError(t, err)
		assert.Equal(t, 0, svc.SOCKSPort())

		_ = w.Close()
		var buf bytes.Buffer
		if _, err := io.Copy(&buf, r); err != nil {
			t.Fatalf("failed to read from pipe: %v", err)
		}
		output := buf.String()
		assert.Contains(t, output, "SOCKS proxy on port 1080 terminated.")
	})

	t.Run("TerminationError", func(t *testing.T) {
		svc.SetSOCKSPort(1080)

		if svc.SSHExecutor == nil {
			t.Fatal("SSHExecutor is nil in TerminationError")
		}
		mockSSHExecutor, ok := svc.SSHExecutor.(*mock_awsctl.MockSSHExecutorInterface)
		if !ok {
			t.Fatalf("SSHExecutor is not *mock_awsctl.MockSSHExecutorInterface, got %T", svc.SSHExecutor)
		}

		if svc.OsDetector == nil {
			t.Fatal("OsDetector is nil in TerminationError")
		}
		mockOsDetector, ok := svc.OsDetector.(*mock_awsctl.MockOSDetector)
		if !ok {
			t.Fatalf("OsDetector is not *mock_awsctl.MockOSDetector, got %T", svc.OsDetector)
		}

		mockSSHExecutor.EXPECT().Execute(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("termination error"))
		mockOsDetector.EXPECT().GetOS().Return("linux")

		err := svc.CleanupSOCKS()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "termination error")
		assert.Equal(t, 1080, svc.SOCKSPort())
	})
}

func TestNewRDSService(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConnServices := mock_awsctl.NewMockServicesInterface(ctrl)

	t.Run("BasicInitialization", func(t *testing.T) {
		service := rds.NewRDSService(mockConnServices)

		assert.NotNil(t, service)
		assert.Equal(t, mockConnServices, service.ConnServices)
		assert.NotNil(t, service.RPrompter)
	})

	t.Run("WithOptions", func(t *testing.T) {
		mockRDSClient := mock_rds.NewMockRDSAdapterInterface(ctrl)
		mockPrompter := mock_awsctl.NewMockPrompter(ctrl)

		opts := []func(*rds.RDSService){
			func(s *rds.RDSService) { s.RDSClient = mockRDSClient },
			func(s *rds.RDSService) { s.GPrompter = mockPrompter },
		}

		service := rds.NewRDSService(mockConnServices, opts...)

		assert.Equal(t, mockRDSClient, service.RDSClient)
		assert.Equal(t, mockPrompter, service.GPrompter)
	})
}

func TestRDSService_Run_SwitchCases(t *testing.T) {

	t.Run("ConnectDirect", func(t *testing.T) {
		svc, ctrl, mockRPrompter, mockRDSAdapter, mockConnPrompter, mockConnServices, _ := setupTest(t)
		defer ctrl.Finish()

		mockRPrompter.EXPECT().SelectRDSAction().Return(rds.ConnectDirect, nil)
		mockConnServices.EXPECT().IsAWSConfigured().Return(true)
		mockConnPrompter.EXPECT().PromptForConfirmation("Look for RDS instances in AWS?").Return(false, nil)
		mockRPrompter.EXPECT().PromptForManualEndpoint().Return("localhost:5432", "admin", "us-west-2", nil)
		mockRDSAdapter.EXPECT().GenerateAuthToken("localhost:5432", "admin", "us-west-2").Return("mock-auth-token", nil)

		err := svc.Run()
		assert.NoError(t, err)
	})

	t.Run("ConnectViaTunnel", func(t *testing.T) {
		svc, ctrl, mockRPrompter, mockRDSAdapter, mockConnPrompter, mockConnServices, mockGPrompter := setupTest(t)
		defer ctrl.Finish()

		mockFs := svc.Fs.(*mock_awsctl.MockFileSystemInterface)
		mockFs.EXPECT().UserHomeDir().Return("/home/test", nil)
		mockFs.EXPECT().MkdirAll(gomock.Any(), gomock.Any()).Return(nil)
		mockFs.EXPECT().WriteFile(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(path string, data []byte, perm os.FileMode) error {
			if strings.Contains(path, "mysql-config") {
				assert.Contains(t, string(data), "[client]")
				assert.Contains(t, string(data), "host=127.0.0.1")
				assert.Contains(t, string(data), "port=3306")
				assert.Contains(t, string(data), "user=test-user")
				assert.Contains(t, string(data), "password=mock-auth-token")
				assert.Contains(t, string(data), "ssl-ca=/home/test/.rds-certs/us-east-1-bundle.pem")
			}
			return nil
		}).Return(nil)

		mockRPrompter.EXPECT().SelectRDSAction().Return(rds.ConnectViaTunnel, nil)
		mockRPrompter.EXPECT().PromptForAuthMethod("Select authentication method for RDS:", []string{"Token", "Native password"}).Return("Token", nil)
		mockConnServices.EXPECT().IsAWSConfigured().Return(true)
		mockConnPrompter.EXPECT().PromptForConfirmation("Look for RDS instances in AWS?").Return(false, nil)
		mockRPrompter.EXPECT().PromptForManualEndpoint().Return("test-rds:3306", "test-user", "us-east-1", nil)
		mockConnPrompter.EXPECT().PromptForLocalPort("RDS", 3306).Return(3306, nil)
		mockRDSAdapter.EXPECT().GenerateAuthToken("test-rds:3306", "test-user", "us-east-1").Return("mock-auth-token", nil)
		mockGPrompter.EXPECT().PromptForSelection("To connect securely, an RDS SSL certificate is required:", []string{"Download certificate automatically", "Provide custom certificate path"}).Return("Download certificate automatically", nil)

		transport := &mockTransport{
			resp: &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte("mock-cert"))),
			},
			err: nil,
		}
		client := &http.Client{Transport: transport}
		origClient := http.DefaultClient
		http.DefaultClient = client
		defer func() { http.DefaultClient = origClient }()

		mockConnServices.EXPECT().StartPortForwarding(gomock.Any(), 3306, "test-rds", 3306).Return(
			func() {},
			func() {},
			nil,
		)

		done := make(chan error)
		go func() {
			err := svc.Run()
			done <- err
		}()

		time.Sleep(100 * time.Millisecond)
		if err := syscall.Kill(syscall.Getpid(), syscall.SIGINT); err != nil {
			t.Fatalf("failed to send SIGINT: %v", err)
		}

		select {
		case err := <-done:
			assert.NoError(t, err, "Run should return nil on SIGINT")
		case <-time.After(2 * time.Second):
			t.Fatal("Test timed out waiting for Run to complete")
		}
	})

	t.Run("ActionSelectionError", func(t *testing.T) {
		svc, ctrl, mockRPrompter, _, _, _, _ := setupTest(t)
		defer ctrl.Finish()

		mockRPrompter.EXPECT().SelectRDSAction().Return(rds.ConnectDirect, errors.New("some error"))

		err := svc.Run()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "action selection aborted")
	})
}

func TestRDSService_GetConnectionDetails(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRDSClient := mock_rds.NewMockRDSAdapterInterface(ctrl)
	mockRPrompter := mock_rds.NewMockRDSPromptInterface(ctrl)
	mockConnPrompter := mock_awsctl.NewMockConnectionPrompter(ctrl)
	mockConnServices := mock_awsctl.NewMockServicesInterface(ctrl)
	mockGPrompter := mock_awsctl.NewMockPrompter(ctrl)
	mockConfigLoader := mock_rds.NewMockConfigLoader(ctrl)
	mockRDSClientFactory := mock_rds.NewMockRDSClientFactory(ctrl)

	newService := func(rdsClient rds.RDSAdapterInterface) *rds.RDSService {
		return &rds.RDSService{
			RPrompter:        mockRPrompter,
			RDSClient:        rdsClient,
			CPrompter:        mockConnPrompter,
			ConnServices:     mockConnServices,
			GPrompter:        mockGPrompter,
			ConnProvider:     nil,
			ConfigLoader:     mockConfigLoader,
			RDSClientFactory: mockRDSClientFactory,
		}
	}

	t.Run("ManualConnection", func(t *testing.T) {
		svc := newService(nil)

		mockConnServices.EXPECT().IsAWSConfigured().Return(true)
		mockConnPrompter.EXPECT().PromptForConfirmation("Look for RDS instances in AWS?").Return(false, nil)
		mockRPrompter.EXPECT().PromptForManualEndpoint().Return("test-rds:3306", "test-user", "us-east-1", nil)

		endpoint, dbUser, region, err := svc.GetConnectionDetails()
		assert.NoError(t, err)
		assert.Equal(t, "test-rds:3306", endpoint)
		assert.Equal(t, "test-user", dbUser)
		assert.Equal(t, "us-east-1", region)
	})

	t.Run("AWSConnection", func(t *testing.T) {
		svc := newService(mockRDSClient)

		mockConnServices.EXPECT().IsAWSConfigured().Return(true)
		mockConnPrompter.EXPECT().PromptForConfirmation("Look for RDS instances in AWS?").Return(true, nil)
		mockConnPrompter.EXPECT().PromptForRegion("").Return("us-east-1", nil)
		mockRPrompter.EXPECT().PromptForProfile().Return("default", nil)

		mockRDSClient.EXPECT().ListRDSResources(gomock.Any()).Return([]models.RDSInstance{
			{DBInstanceIdentifier: "test-rds"},
		}, nil)

		mockRPrompter.EXPECT().PromptForRDSInstance([]models.RDSInstance{
			{DBInstanceIdentifier: "test-rds"},
		}).Return("test-rds", nil)

		mockRDSClient.EXPECT().GetConnectionEndpoint(gomock.Any(), "test-rds").Return("test-rds:3306", nil)
		mockGPrompter.EXPECT().PromptForInput("Enter database username:", "").Return("test-user", nil)

		endpoint, dbUser, region, err := svc.GetConnectionDetails()
		assert.NoError(t, err)
		assert.Equal(t, "test-rds:3306", endpoint)
		assert.Equal(t, "test-user", dbUser)
		assert.Equal(t, "us-east-1", region)
	})

	t.Run("AWSNotConfigured", func(t *testing.T) {
		svc := newService(nil)

		mockConnServices.EXPECT().IsAWSConfigured().Return(false)
		mockRPrompter.EXPECT().PromptForManualEndpoint().Return("test-rds:3306", "test-user", "us-east-1", nil)

		endpoint, dbUser, region, err := svc.GetConnectionDetails()
		assert.NoError(t, err)
		assert.Equal(t, "test-rds:3306", endpoint)
		assert.Equal(t, "test-user", dbUser)
		assert.Equal(t, "us-east-1", region)
	})

	t.Run("ProfilePromptError", func(t *testing.T) {
		svc := newService(nil)

		mockConnServices.EXPECT().IsAWSConfigured().Return(true)
		mockConnPrompter.EXPECT().PromptForConfirmation("Look for RDS instances in AWS?").Return(true, nil)
		mockConnPrompter.EXPECT().PromptForRegion("").Return("us-east-1", nil)
		mockRPrompter.EXPECT().PromptForProfile().Return("", errors.New("profile error"))
		mockRPrompter.EXPECT().PromptForManualEndpoint().Return("test-rds:3306", "test-user", "us-east-1", nil)

		endpoint, dbUser, region, err := svc.GetConnectionDetails()
		assert.NoError(t, err)
		assert.Equal(t, "test-rds:3306", endpoint)
		assert.Equal(t, "test-user", dbUser)
		assert.Equal(t, "us-east-1", region)
	})

	t.Run("AWSConfigError", func(t *testing.T) {
		svc := newService(mockRDSClient)

		mockConnServices.EXPECT().IsAWSConfigured().Return(true)
		mockConnPrompter.EXPECT().PromptForConfirmation("Look for RDS instances in AWS?").Return(true, nil)
		mockConnPrompter.EXPECT().PromptForRegion("").Return("us-east-1", nil)
		mockRPrompter.EXPECT().PromptForProfile().Return("invalid-profile", nil)

		mockRDSClient.EXPECT().ListRDSResources(gomock.Any()).Return(nil, errors.New("failed to load config"))
		mockRPrompter.EXPECT().PromptForManualEndpoint().Return("test-rds:3306", "test-user", "us-east-1", nil)

		endpoint, dbUser, region, err := svc.GetConnectionDetails()
		assert.NoError(t, err)
		assert.Equal(t, "test-rds:3306", endpoint)
		assert.Equal(t, "test-user", dbUser)
		assert.Equal(t, "us-east-1", region)
	})

	t.Run("NoRDSResources", func(t *testing.T) {
		svc := newService(mockRDSClient)

		mockConnServices.EXPECT().IsAWSConfigured().Return(true)
		mockConnPrompter.EXPECT().PromptForConfirmation("Look for RDS instances in AWS?").Return(true, nil)
		mockConnPrompter.EXPECT().PromptForRegion("").Return("us-east-1", nil)
		mockRPrompter.EXPECT().PromptForProfile().Return("default", nil)
		mockRDSClient.EXPECT().ListRDSResources(gomock.Any()).Return([]models.RDSInstance{}, nil)
		mockRPrompter.EXPECT().PromptForManualEndpoint().Return("test-rds:3306", "test-user", "us-east-1", nil)

		endpoint, dbUser, region, err := svc.GetConnectionDetails()
		assert.NoError(t, err)
		assert.Equal(t, "test-rds:3306", endpoint)
		assert.Equal(t, "test-user", dbUser)
		assert.Equal(t, "us-east-1", region)
	})

	t.Run("RDSResourcesError", func(t *testing.T) {
		svc := newService(mockRDSClient)

		mockConnServices.EXPECT().IsAWSConfigured().Return(true)
		mockConnPrompter.EXPECT().PromptForConfirmation("Look for RDS instances in AWS?").Return(true, nil)
		mockConnPrompter.EXPECT().PromptForRegion("").Return("us-east-1", nil)
		mockRPrompter.EXPECT().PromptForProfile().Return("default", nil)
		mockRDSClient.EXPECT().ListRDSResources(gomock.Any()).Return(nil, errors.New("AWS error"))
		mockRPrompter.EXPECT().PromptForManualEndpoint().Return("test-rds:3306", "test-user", "us-east-1", nil)

		endpoint, dbUser, region, err := svc.GetConnectionDetails()
		assert.NoError(t, err)
		assert.Equal(t, "test-rds:3306", endpoint)
		assert.Equal(t, "test-user", dbUser)
		assert.Equal(t, "us-east-1", region)
	})

	t.Run("RDSClientInitialization", func(t *testing.T) {
		svc := newService(nil)

		mockConnServices.EXPECT().IsAWSConfigured().Return(true)
		mockConnPrompter.EXPECT().PromptForConfirmation("Look for RDS instances in AWS?").Return(true, nil)
		mockConnPrompter.EXPECT().PromptForRegion("").Return("us-east-1", nil)
		mockRPrompter.EXPECT().PromptForProfile().Return("default", nil)

		mockConfigLoader.EXPECT().LoadDefaultConfig(gomock.Any(), gomock.Any()).Return(aws.Config{
			Region: "us-east-1",
		}, nil)

		mockRDSClientFactory.EXPECT().NewRDSClient(gomock.Any(), gomock.Any()).Return(mockRDSClient)

		mockRDSClient.EXPECT().ListRDSResources(gomock.Any()).Return([]models.RDSInstance{
			{DBInstanceIdentifier: "test-rds"},
		}, nil)

		mockRPrompter.EXPECT().PromptForRDSInstance([]models.RDSInstance{
			{DBInstanceIdentifier: "test-rds"},
		}).Return("test-rds", nil)

		mockRDSClient.EXPECT().GetConnectionEndpoint(gomock.Any(), "test-rds").Return("test-rds:3306", nil)
		mockGPrompter.EXPECT().PromptForInput("Enter database username:", "").Return("test-user", nil)

		endpoint, dbUser, region, err := svc.GetConnectionDetails()
		assert.NoError(t, err)
		assert.Equal(t, "test-rds:3306", endpoint)
		assert.Equal(t, "test-user", dbUser)
		assert.Equal(t, "us-east-1", region)
	})

	t.Run("RDSClientInitializationConfigError", func(t *testing.T) {
		svc := newService(nil)

		mockConnServices.EXPECT().IsAWSConfigured().Return(true)
		mockConnPrompter.EXPECT().PromptForConfirmation("Look for RDS instances in AWS?").Return(true, nil)
		mockConnPrompter.EXPECT().PromptForRegion("").Return("us-east-1", nil)
		mockRPrompter.EXPECT().PromptForProfile().Return("default", nil)

		mockConfigLoader.EXPECT().LoadDefaultConfig(gomock.Any(), gomock.Any()).Return(aws.Config{}, errors.New("config error"))
		mockRPrompter.EXPECT().PromptForManualEndpoint().Return("test-rds:3306", "test-user", "us-east-1", nil)

		endpoint, dbUser, region, err := svc.GetConnectionDetails()
		assert.NoError(t, err)
		assert.Equal(t, "test-rds:3306", endpoint)
		assert.Equal(t, "test-user", dbUser)
		assert.Equal(t, "us-east-1", region)
	})
}

func TestRDSService_Run_ErrorCases(t *testing.T) {
	svc, ctrl, mockRPrompter, _, _, mockConnServices, _ := setupTest(t)
	defer ctrl.Finish()

	t.Run("HandleDirectConnectionError", func(t *testing.T) {
		mockRPrompter.EXPECT().SelectRDSAction().Return(rds.ConnectDirect, nil)
		mockConnServices.EXPECT().IsAWSConfigured().Return(false)
		mockRPrompter.EXPECT().PromptForManualEndpoint().Return("", "", "", errors.New("manual endpoint error"))

		err := svc.Run()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "direct connection failed")
	})

	t.Run("HandleTunnelConnectionError", func(t *testing.T) {
		mockRPrompter.EXPECT().SelectRDSAction().Return(rds.ConnectViaTunnel, nil)
		mockRPrompter.EXPECT().PromptForAuthMethod("Select authentication method for RDS:", []string{"Token", "Native password"}).Return("Token", nil)
		mockConnServices.EXPECT().IsAWSConfigured().Return(false)
		mockRPrompter.EXPECT().PromptForManualEndpoint().Return("invalid-endpoint", "", "", nil)

		err := svc.Run()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "tunnel connection failed")
	})

}

func TestHandleTunnelConnection_ErrorCases(t *testing.T) {
	svc, ctrl, mockRPrompter, mockRDSAdapter, mockConnPrompter, mockConnServices, _ := setupTest(t)
	defer ctrl.Finish()

	t.Run("InvalidEndpointFormat", func(t *testing.T) {
		mockRPrompter.EXPECT().PromptForAuthMethod("Select authentication method for RDS:", []string{"Token", "Native password"}).Return("Token", nil)
		mockConnServices.EXPECT().IsAWSConfigured().Return(true)
		mockConnPrompter.EXPECT().PromptForConfirmation("Look for RDS instances in AWS?").Return(false, nil)
		mockRPrompter.EXPECT().PromptForManualEndpoint().Return("invalid-endpoint", "user", "region", nil)

		err := svc.HandleTunnelConnection()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid RDS endpoint format")
	})

	t.Run("InvalidPort", func(t *testing.T) {
		mockRPrompter.EXPECT().PromptForAuthMethod("Select authentication method for RDS:", []string{"Token", "Native password"}).Return("Token", nil)
		mockConnServices.EXPECT().IsAWSConfigured().Return(true)
		mockConnPrompter.EXPECT().PromptForConfirmation("Look for RDS instances in AWS?").Return(false, nil)
		mockRPrompter.EXPECT().PromptForManualEndpoint().Return("host:invalid", "user", "region", nil)

		err := svc.HandleTunnelConnection()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid port in RDS endpoint")
	})

	t.Run("LocalPortPromptError", func(t *testing.T) {
		mockRPrompter.EXPECT().PromptForAuthMethod("Select authentication method for RDS:", []string{"Token", "Native password"}).Return("Token", nil)
		mockConnServices.EXPECT().IsAWSConfigured().Return(true)
		mockConnPrompter.EXPECT().PromptForConfirmation("Look for RDS instances in AWS?").Return(false, nil)
		mockRPrompter.EXPECT().PromptForManualEndpoint().Return("host:3306", "user", "region", nil)
		mockConnPrompter.EXPECT().PromptForLocalPort("RDS", 3306).Return(0, errors.New("port error"))

		err := svc.HandleTunnelConnection()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get local port")
	})

	t.Run("AuthTokenError", func(t *testing.T) {
		mockRPrompter.EXPECT().PromptForAuthMethod("Select authentication method for RDS:", []string{"Token", "Native password"}).Return("Token", nil)
		mockConnServices.EXPECT().IsAWSConfigured().Return(true)
		mockConnPrompter.EXPECT().PromptForConfirmation("Look for RDS instances in AWS?").Return(false, nil)
		mockRPrompter.EXPECT().PromptForManualEndpoint().Return("host:3306", "user", "region", nil)
		mockConnPrompter.EXPECT().PromptForLocalPort("RDS", 3306).Return(3306, nil)
		mockRDSAdapter.EXPECT().GenerateAuthToken("host:3306", "user", "region").Return("", errors.New("auth error"))

		err := svc.HandleTunnelConnection()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to generate RDS auth token")
	})
}

func TestGetRDSConnectionDetails_ErrorCases(t *testing.T) {
	svc, ctrl, mockRPrompter, _, mockConnPrompter, mockConnServices, _ := setupTest(t)
	defer ctrl.Finish()

	t.Run("RegionPromptError", func(t *testing.T) {
		mockConnServices.EXPECT().IsAWSConfigured().Return(true)
		mockConnPrompter.EXPECT().PromptForConfirmation("Look for RDS instances in AWS?").Return(true, nil)
		mockConnPrompter.EXPECT().PromptForRegion("").Return("", errors.New("region error"))
		mockRPrompter.EXPECT().PromptForManualEndpoint().Return("host:3306", "user", "region", nil)

		_, _, _, err := svc.GetConnectionDetails()
		assert.NoError(t, err)
	})

}

func TestHandleDirectConnection_AuthTokenError(t *testing.T) {
	svc, ctrl, mockRPrompter, mockRDSAdapter, mockConnPrompter, mockConnServices, _ := setupTest(t)
	defer ctrl.Finish()

	t.Run("GenerateAuthTokenError", func(t *testing.T) {
		mockConnServices.EXPECT().IsAWSConfigured().Return(true)
		mockConnPrompter.EXPECT().PromptForConfirmation("Look for RDS instances in AWS?").Return(false, nil)
		mockRPrompter.EXPECT().PromptForManualEndpoint().Return("localhost:5432", "admin", "us-west-2", nil)

		mockRDSAdapter.EXPECT().GenerateAuthToken("localhost:5432", "admin", "us-west-2").
			Return("", errors.New("token generation failed"))

		err := svc.HandleDirectConnection()

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to generate RDS auth token")
		assert.Contains(t, err.Error(), "token generation failed")
	})
}

func TestHandleSSLCertificate(t *testing.T) {
	svc, ctrl, _, _, _, _, mockGPrompter := setupTest(t)
	defer ctrl.Finish()

	mockFs := svc.Fs.(*mock_awsctl.MockFileSystemInterface)
	region := "us-east-1"
	defaultCertPath := filepath.Join("/home/test", fmt.Sprintf(".rds-certs/%s-bundle.pem", region))

	transport := &mockTransport{}
	client := &http.Client{
		Transport: transport,
	}

	origClient := http.DefaultClient
	http.DefaultClient = client
	defer func() { http.DefaultClient = origClient }()

	t.Run("DownloadCertificateSuccess", func(t *testing.T) {
		transport.resp = &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader([]byte("mock-cert"))),
		}
		transport.err = nil

		mockFs.EXPECT().UserHomeDir().Return("/home/test", nil)
		mockFs.EXPECT().MkdirAll(filepath.Join("/home/test", ".rds-certs"), os.FileMode(0700)).Return(nil)
		certPath := filepath.Join("/home/test", ".rds-certs", fmt.Sprintf("%s-bundle.pem", region))
		mockFs.EXPECT().WriteFile(certPath, []byte("mock-cert"), os.FileMode(0600)).Return(nil)

		mockGPrompter.EXPECT().PromptForSelection(
			"To connect securely, an RDS SSL certificate is required:",
			[]string{"Download certificate automatically", "Provide custom certificate path"},
		).Return("Download certificate automatically", nil)

		path, err := svc.HandleSSLCertificate(region)
		assert.NoError(t, err)
		assert.Equal(t, certPath, path)
	})

	t.Run("CustomCertificateSuccess", func(t *testing.T) {
		certPath := "/tmp/rds-cert.pem"
		mockFs.EXPECT().UserHomeDir().Return("/home/test", nil)
		mockFs.EXPECT().Stat(certPath).Return(&mockFileInfo{name: "rds-cert.pem"}, nil)

		mockGPrompter.EXPECT().PromptForSelection(
			"To connect securely, an RDS SSL certificate is required:",
			[]string{"Download certificate automatically", "Provide custom certificate path"},
		).Return("Provide custom certificate path", nil)

		mockGPrompter.EXPECT().PromptForInput(
			"Enter path to your RDS SSL CA certificate file",
			defaultCertPath,
		).Return(certPath, nil)

		path, err := svc.HandleSSLCertificate(region)
		assert.NoError(t, err)
		assert.Equal(t, certPath, path)
	})

	t.Run("CustomCertificateNotFound", func(t *testing.T) {
		certPath := "/tmp/nonexistent-cert.pem"
		mockFs.EXPECT().UserHomeDir().Return("/home/test", nil)
		mockFs.EXPECT().Stat(certPath).Return(nil, os.ErrNotExist)

		mockGPrompter.EXPECT().PromptForSelection(
			"To connect securely, an RDS SSL certificate is required:",
			[]string{"Download certificate automatically", "Provide custom certificate path"},
		).Return("Provide custom certificate path", nil)

		mockGPrompter.EXPECT().PromptForInput(
			"Enter path to your RDS SSL CA certificate file",
			defaultCertPath,
		).Return(certPath, nil)

		path, err := svc.HandleSSLCertificate(region)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), fmt.Sprintf("certificate not found at %s", certPath))
		assert.Empty(t, path)
	})

	t.Run("PromptForInputInterrupted", func(t *testing.T) {
		mockFs.EXPECT().UserHomeDir().Return("/home/test", nil)

		mockGPrompter.EXPECT().PromptForSelection(
			"To connect securely, an RDS SSL certificate is required:",
			[]string{"Download certificate automatically", "Provide custom certificate path"},
		).Return("Provide custom certificate path", nil)

		mockGPrompter.EXPECT().PromptForInput(
			"Enter path to your RDS SSL CA certificate file",
			defaultCertPath,
		).Return("", promptUtils.ErrInterrupted)

		path, err := svc.HandleSSLCertificate(region)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "operation cancelled by user")
		assert.Empty(t, path)
	})

	t.Run("PromptForInputError", func(t *testing.T) {
		mockFs.EXPECT().UserHomeDir().Return("/home/test", nil)

		mockGPrompter.EXPECT().PromptForSelection(
			"To connect securely, an RDS SSL certificate is required:",
			[]string{"Download certificate automatically", "Provide custom certificate path"},
		).Return("Provide custom certificate path", nil)

		mockGPrompter.EXPECT().PromptForInput(
			"Enter path to your RDS SSL CA certificate file",
			defaultCertPath,
		).Return("", errors.New("input error"))

		path, err := svc.HandleSSLCertificate(region)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "certificate path input failed: input error")
		assert.Empty(t, path)
	})

	t.Run("PromptForSelectionInterrupted", func(t *testing.T) {
		mockGPrompter.EXPECT().PromptForSelection(
			"To connect securely, an RDS SSL certificate is required:",
			[]string{"Download certificate automatically", "Provide custom certificate path"},
		).Return("", promptUtils.ErrInterrupted)

		path, err := svc.HandleSSLCertificate(region)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "operation cancelled by user")
		assert.Empty(t, path)
	})

	t.Run("PromptForSelectionError", func(t *testing.T) {
		mockGPrompter.EXPECT().PromptForSelection(
			"To connect securely, an RDS SSL certificate is required:",
			[]string{"Download certificate automatically", "Provide custom certificate path"},
		).Return("", errors.New("selection error"))

		path, err := svc.HandleSSLCertificate(region)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "certificate selection failed: selection error")
		assert.Empty(t, path)
	})

	t.Run("UserHomeDirError", func(t *testing.T) {
		certPath := "/tmp/rds-cert.pem"
		defaultCertPathWithEnv := filepath.Join(os.Getenv("HOME"), fmt.Sprintf(".rds-certs/%s-bundle.pem", region))
		mockFs.EXPECT().UserHomeDir().Return("", errors.New("home dir error"))
		mockFs.EXPECT().Stat(certPath).Return(&mockFileInfo{name: "rds-cert.pem"}, nil)

		mockGPrompter.EXPECT().PromptForSelection(
			"To connect securely, an RDS SSL certificate is required:",
			[]string{"Download certificate automatically", "Provide custom certificate path"},
		).Return("Provide custom certificate path", nil)

		mockGPrompter.EXPECT().PromptForInput(
			"Enter path to your RDS SSL CA certificate file",
			defaultCertPathWithEnv,
		).Return(certPath, nil)

		path, err := svc.HandleSSLCertificate(region)
		assert.NoError(t, err)
		assert.Equal(t, certPath, path)
	})
}

func TestDownloadSSLCertificate(t *testing.T) {
	svc, ctrl, _, _, _, _, _ := setupTest(t)
	defer ctrl.Finish()

	mockFs := svc.Fs.(*mock_awsctl.MockFileSystemInterface)
	region := "us-east-1"
	certURL := fmt.Sprintf("https://truststore.pki.rds.amazonaws.com/%s/%s-bundle.pem", region, region)

	stdout := os.Stdout
	defer func() { os.Stdout = stdout }()
	_, w, _ := os.Pipe()
	os.Stdout = w
	defer func() {
		if err := w.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to close write pipe: %v\n", err)
		}
	}()

	t.Run("Success", func(t *testing.T) {
		transport := &mockTransport{
			resp: &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte("mock-cert"))),
			},
			err: nil,
		}
		client := &http.Client{Transport: transport}
		origClient := http.DefaultClient
		http.DefaultClient = client
		defer func() { http.DefaultClient = origClient }()

		mockFs.EXPECT().UserHomeDir().Return("/home/test", nil)
		mockFs.EXPECT().MkdirAll(filepath.Join("/home/test", ".rds-certs"), os.FileMode(0700)).Return(nil)
		certPath := filepath.Join("/home/test", ".rds-certs", fmt.Sprintf("%s-bundle.pem", region))
		mockFs.EXPECT().WriteFile(certPath, []byte("mock-cert"), os.FileMode(0600)).Return(nil)

		path, err := rds.DownloadSSLCertificate(svc, region)
		assert.NoError(t, err)
		assert.Equal(t, certPath, path)
	})

	t.Run("HTTPGetError", func(t *testing.T) {
		transport := &mockTransport{
			resp: nil,
			err:  errors.New("network error"),
		}
		client := &http.Client{Transport: transport}
		origClient := http.DefaultClient
		http.DefaultClient = client
		defer func() { http.DefaultClient = origClient }()

		path, err := rds.DownloadSSLCertificate(svc, region)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), fmt.Sprintf("Get \"%s\": network error", certURL))
		assert.Empty(t, path)
	})

	t.Run("NonOKStatusCode", func(t *testing.T) {
		transport := &mockTransport{
			resp: &http.Response{
				StatusCode: http.StatusNotFound,
				Body:       io.NopCloser(bytes.NewReader([]byte(""))),
			},
			err: nil,
		}
		client := &http.Client{Transport: transport}
		origClient := http.DefaultClient
		http.DefaultClient = client
		defer func() { http.DefaultClient = origClient }()

		path, err := rds.DownloadSSLCertificate(svc, region)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to download certificate: HTTP 404")
		assert.Empty(t, path)
	})

	t.Run("ReadBodyError", func(t *testing.T) {
		transport := &mockTransport{
			resp: &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(&errorReader{err: errors.New("read error")}),
			},
			err: nil,
		}
		client := &http.Client{Transport: transport}
		origClient := http.DefaultClient
		http.DefaultClient = client
		defer func() { http.DefaultClient = origClient }()

		path, err := rds.DownloadSSLCertificate(svc, region)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to read certificate: read error")
		assert.Empty(t, path)
	})

	t.Run("MkdirAllError", func(t *testing.T) {
		transport := &mockTransport{
			resp: &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte("mock-cert"))),
			},
			err: nil,
		}
		client := &http.Client{Transport: transport}
		origClient := http.DefaultClient
		http.DefaultClient = client
		defer func() { http.DefaultClient = origClient }()

		mockFs.EXPECT().UserHomeDir().Return("/home/test", nil)
		mockFs.EXPECT().MkdirAll(filepath.Join("/home/test", ".rds-certs"), os.FileMode(0700)).Return(errors.New("mkdir error"))

		path, err := rds.DownloadSSLCertificate(svc, region)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create cert directory: mkdir error")
		assert.Empty(t, path)
	})

	t.Run("WriteFileError", func(t *testing.T) {
		transport := &mockTransport{
			resp: &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte("mock-cert"))),
			},
			err: nil,
		}
		client := &http.Client{Transport: transport}
		origClient := http.DefaultClient
		http.DefaultClient = client
		defer func() { http.DefaultClient = origClient }()

		mockFs.EXPECT().UserHomeDir().Return("/home/test", nil)
		mockFs.EXPECT().MkdirAll(filepath.Join("/home/test", ".rds-certs"), os.FileMode(0700)).Return(nil)
		certPath := filepath.Join("/home/test", ".rds-certs", fmt.Sprintf("%s-bundle.pem", region))
		mockFs.EXPECT().WriteFile(certPath, []byte("mock-cert"), os.FileMode(0600)).Return(errors.New("write error"))

		path, err := rds.DownloadSSLCertificate(svc, region)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to write certificate: write error")
		assert.Empty(t, path)
	})

	t.Run("UserHomeDirError", func(t *testing.T) {
		transport := &mockTransport{
			resp: &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte("mock-cert"))),
			},
			err: nil,
		}
		client := &http.Client{Transport: transport}
		origClient := http.DefaultClient
		http.DefaultClient = client
		defer func() { http.DefaultClient = origClient }()

		mockFs.EXPECT().UserHomeDir().Return("", errors.New("home dir error"))
		mockFs.EXPECT().MkdirAll(filepath.Join(os.Getenv("HOME"), ".rds-certs"), os.FileMode(0700)).Return(nil)
		certPath := filepath.Join(os.Getenv("HOME"), ".rds-certs", fmt.Sprintf("%s-bundle.pem", region))
		mockFs.EXPECT().WriteFile(certPath, []byte("mock-cert"), os.FileMode(0600)).Return(nil)

		path, err := rds.DownloadSSLCertificate(svc, region)
		assert.NoError(t, err)
		assert.Equal(t, certPath, path)
	})
}

func TestRDSService_HandleTunnelConnection_ErrorCases(t *testing.T) {
	t.Parallel()

	t.Run("AuthMethodPromptError", func(t *testing.T) {
		t.Parallel()
		svc, ctrl, mockRPrompter, _, _, _, _ := setupTest(t)
		defer ctrl.Finish()

		mockRPrompter.EXPECT().PromptForAuthMethod("Select authentication method for RDS:", []string{"Token", "Native password"}).
			Return("", errors.New("prompt error"))

		err := svc.HandleTunnelConnection()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get authentication method")
	})

	t.Run("RDSConnectionDetailsError", func(t *testing.T) {
		t.Parallel()
		svc, ctrl, mockRPrompter, _, mockConnPrompter, mockConnServices, _ := setupTest(t)
		defer ctrl.Finish()

		mockRPrompter.EXPECT().PromptForAuthMethod("Select authentication method for RDS:", []string{"Token", "Native password"}).
			Return("Token", nil)
		mockConnServices.EXPECT().IsAWSConfigured().Return(true)
		mockConnPrompter.EXPECT().PromptForConfirmation("Look for RDS instances in AWS?").Return(false, nil)
		mockRPrompter.EXPECT().PromptForManualEndpoint().Return("", "", "", errors.New("connection details error"))

		err := svc.HandleTunnelConnection()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get RDS connection details")
	})

	t.Run("SSLCertificateError", func(t *testing.T) {
		t.Parallel()
		svc, ctrl, mockRPrompter, mockRDSAdapter, mockConnPrompter, mockConnServices, mockGPrompter := setupTest(t)
		defer ctrl.Finish()

		mockRPrompter.EXPECT().PromptForAuthMethod("Select authentication method for RDS:", []string{"Token", "Native password"}).
			Return("Token", nil)
		mockConnServices.EXPECT().IsAWSConfigured().Return(true)
		mockConnPrompter.EXPECT().PromptForConfirmation("Look for RDS instances in AWS?").Return(false, nil)
		mockRPrompter.EXPECT().PromptForManualEndpoint().Return("test-rds:3306", "test-user", "us-east-1", nil)
		mockConnPrompter.EXPECT().PromptForLocalPort("RDS", 3306).Return(3306, nil)
		mockRDSAdapter.EXPECT().GenerateAuthToken("test-rds:3306", "test-user", "us-east-1").Return("mock-auth-token", nil)
		mockGPrompter.EXPECT().PromptForSelection("To connect securely, an RDS SSL certificate is required:",
			[]string{"Download certificate automatically", "Provide custom certificate path"}).
			Return("", errors.New("ssl cert error"))

		err := svc.HandleTunnelConnection()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to handle SSL certificate")
	})

	t.Run("NativePasswordAuthMethod", func(t *testing.T) {
		t.Parallel()
		svc, ctrl, mockRPrompter, _, mockConnPrompter, mockConnServices, _ := setupTest(t)
		defer ctrl.Finish()

		mockRPrompter.EXPECT().PromptForAuthMethod("Select authentication method for RDS:", []string{"Token", "Native password"}).
			Return("Native password", nil)
		mockConnServices.EXPECT().IsAWSConfigured().Return(true)
		mockConnPrompter.EXPECT().PromptForConfirmation("Look for RDS instances in AWS?").Return(false, nil)
		mockRPrompter.EXPECT().PromptForManualEndpoint().Return("test-rds:3306", "test-user", "us-east-1", nil)
		mockConnPrompter.EXPECT().PromptForLocalPort("RDS", 3306).Return(3306, nil)
		mockConnServices.EXPECT().StartPortForwarding(gomock.Any(), 3306, "test-rds", 3306).Return(
			func() {},
			func() {},
			nil,
		)

		done := make(chan error)
		go func() {
			err := svc.HandleTunnelConnection()
			done <- err
		}()

		time.Sleep(100 * time.Millisecond)
		if err := syscall.Kill(syscall.Getpid(), syscall.SIGINT); err != nil {
			t.Fatalf("failed to send SIGINT: %v", err)
		}

		select {
		case err := <-done:
			assert.NoError(t, err, "HandleTunnelConnection should return nil on SIGINT")
		case <-time.After(2 * time.Second):
			t.Fatal("Test timed out waiting for HandleTunnelConnection to complete")
		}
	})

	t.Run("PortForwardingError", func(t *testing.T) {
		t.Parallel()
		svc, ctrl, mockRPrompter, mockRDSAdapter, mockConnPrompter, mockConnServices, mockGPrompter := setupTest(t)
		defer ctrl.Finish()

		mockFs := svc.Fs.(*mock_awsctl.MockFileSystemInterface)
		mockFs.EXPECT().UserHomeDir().Return("/home/test", nil)
		mockFs.EXPECT().MkdirAll(gomock.Any(), gomock.Any()).Return(nil)
		mockFs.EXPECT().WriteFile(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(path string, data []byte, perm os.FileMode) error {
			if strings.Contains(path, "mysql-config") {
				assert.Contains(t, string(data), "[client]")
				assert.Contains(t, string(data), "host=127.0.0.1")
				assert.Contains(t, string(data), "port=3306")
				assert.Contains(t, string(data), "user=test-user")
				assert.Contains(t, string(data), "password=mock-auth-token")
				assert.Contains(t, string(data), "ssl-ca=/home/test/.rds-certs/us-east-1-bundle.pem")
			}
			return nil
		}).Return(nil)

		transport := &mockTransport{
			resp: &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte("mock-cert"))),
			},
			err: nil,
		}
		client := &http.Client{Transport: transport}
		origClient := http.DefaultClient
		http.DefaultClient = client
		defer func() { http.DefaultClient = origClient }()

		mockRPrompter.EXPECT().PromptForAuthMethod("Select authentication method for RDS:", []string{"Token", "Native password"}).
			Return("Token", nil)
		mockConnServices.EXPECT().IsAWSConfigured().Return(true)
		mockConnPrompter.EXPECT().PromptForConfirmation("Look for RDS instances in AWS?").Return(false, nil)
		mockRPrompter.EXPECT().PromptForManualEndpoint().Return("test-rds:3306", "test-user", "us-east-1", nil)
		mockConnPrompter.EXPECT().PromptForLocalPort("RDS", 3306).Return(3306, nil)
		mockRDSAdapter.EXPECT().GenerateAuthToken("test-rds:3306", "test-user", "us-east-1").Return("mock-auth-token", nil)
		mockGPrompter.EXPECT().PromptForSelection("To connect securely, an RDS SSL certificate is required:",
			[]string{"Download certificate automatically", "Provide custom certificate path"}).
			Return("Download certificate automatically", nil)
		mockConnServices.EXPECT().StartPortForwarding(gomock.Any(), 3306, "test-rds", 3306).Return(
			func() {},
			func() {},
			errors.New("port forwarding error"),
		)

		err := svc.HandleTunnelConnection()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "tunnel connection failed: port forwarding failed")
	})
}
