package ecr_test

import (
	"context"
	"encoding/base64"
	"errors"
	"testing"

	internalECR "github.com/BerryBytes/awsctl/internal/ecr"
	mock_awsctl "github.com/BerryBytes/awsctl/tests/mock"
	mock_ecr "github.com/BerryBytes/awsctl/tests/mock/ecr"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/aws/aws-sdk-go-v2/service/ecr/types"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func TestNewECRClient(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := aws.Config{Region: "us-east-1"}
	mockFileSystem := mock_awsctl.NewMockFileSystemInterface(ctrl)
	mockExecutor := mock_awsctl.NewMockCommandExecutor(ctrl)

	client := internalECR.NewECRClient(cfg, mockFileSystem, mockExecutor)

	assert.NotNil(t, client, "Client should be created")
	assert.NotNil(t, client.Client, "ECR API client should be initialized")
	assert.Equal(t, cfg, client.Cfg, "Config should be set")
	assert.Equal(t, mockFileSystem, client.FileSystem, "FileSystem should be set")
	assert.Equal(t, mockExecutor, client.Executor, "Executor should be set")
	assert.Implements(t, (*internalECR.ECRAdapterInterface)(nil), client, "Client should implement ECRAdapterInterface")
}

func TestAwsECRAdapter_Login(t *testing.T) {
	tests := []struct {
		name              string
		lookPathErr       error
		getAuthTokenOut   *ecr.GetAuthorizationTokenOutput
		getAuthTokenErr   error
		dockerLoginOutput []byte
		dockerLoginErr    error
		homeDir           string
		homeDirErr        error
		mkdirErr          error
		expectedError     string
		setupMocks        func(*mock_awsctl.MockCommandExecutor, *mock_ecr.MockECRAPI, *mock_awsctl.MockFileSystemInterface)
	}{
		{
			name: "Successful login",
			getAuthTokenOut: &ecr.GetAuthorizationTokenOutput{
				AuthorizationData: []types.AuthorizationData{
					{
						AuthorizationToken: aws.String(base64.StdEncoding.EncodeToString([]byte("AWS:secretpassword"))),
						ProxyEndpoint:      aws.String("https://123456789012.dkr.ecr.us-east-1.amazonaws.com"),
					},
				},
			},
			dockerLoginOutput: []byte("Login Succeeded"),
			homeDir:           "/home/user",
			setupMocks: func(exec *mock_awsctl.MockCommandExecutor, api *mock_ecr.MockECRAPI, fs *mock_awsctl.MockFileSystemInterface) {
				exec.EXPECT().LookPath("docker").Return("/usr/bin/docker", nil)
				api.EXPECT().GetAuthorizationToken(gomock.Any(), gomock.Any()).Return(&ecr.GetAuthorizationTokenOutput{
					AuthorizationData: []types.AuthorizationData{
						{
							AuthorizationToken: aws.String(base64.StdEncoding.EncodeToString([]byte("AWS:secretpassword"))),
							ProxyEndpoint:      aws.String("https://123456789012.dkr.ecr.us-east-1.amazonaws.com"),
						},
					},
				}, nil)
				exec.EXPECT().RunCommandWithInput(
					"docker",
					"secretpassword",
					[]string{"login", "--username", "AWS", "--password-stdin", "https://123456789012.dkr.ecr.us-east-1.amazonaws.com"},
				).Return([]byte("Login Succeeded"), nil)
				fs.EXPECT().UserHomeDir().Return("/home/user", nil)
				fs.EXPECT().MkdirAll("/home/user/.docker", gomock.Any()).Return(nil)
			},
		},
		{
			name: "Docker not found",
			setupMocks: func(exec *mock_awsctl.MockCommandExecutor, api *mock_ecr.MockECRAPI, fs *mock_awsctl.MockFileSystemInterface) {
				exec.EXPECT().LookPath("docker").Return("", errors.New("docker not found"))
			},
			expectedError: "docker command not found: docker not found",
		},
		{
			name: "GetAuthorizationToken fails",
			setupMocks: func(exec *mock_awsctl.MockCommandExecutor, api *mock_ecr.MockECRAPI, fs *mock_awsctl.MockFileSystemInterface) {
				exec.EXPECT().LookPath("docker").Return("/usr/bin/docker", nil)
				api.EXPECT().GetAuthorizationToken(gomock.Any(), gomock.Any()).Return(nil, errors.New("api error"))
			},
			expectedError: "failed to get ECR authorization token: api error",
		},
		{
			name: "Empty authorization data",
			setupMocks: func(exec *mock_awsctl.MockCommandExecutor, api *mock_ecr.MockECRAPI, fs *mock_awsctl.MockFileSystemInterface) {
				exec.EXPECT().LookPath("docker").Return("/usr/bin/docker", nil)
				api.EXPECT().GetAuthorizationToken(gomock.Any(), gomock.Any()).Return(&ecr.GetAuthorizationTokenOutput{
					AuthorizationData: []types.AuthorizationData{},
				}, nil)
			},
			expectedError: "no authorization data returned",
		},
		{
			name: "Invalid base64 token",
			setupMocks: func(exec *mock_awsctl.MockCommandExecutor, api *mock_ecr.MockECRAPI, fs *mock_awsctl.MockFileSystemInterface) {
				exec.EXPECT().LookPath("docker").Return("/usr/bin/docker", nil)
				api.EXPECT().GetAuthorizationToken(gomock.Any(), gomock.Any()).Return(&ecr.GetAuthorizationTokenOutput{
					AuthorizationData: []types.AuthorizationData{
						{
							AuthorizationToken: aws.String("invalid-base64"),
							ProxyEndpoint:      aws.String("https://123456789012.dkr.ecr.us-east-1.amazonaws.com"),
						},
					},
				}, nil)
			},
			expectedError: "failed to decode authorization token",
		},
		{
			name: "Docker login fails",
			setupMocks: func(exec *mock_awsctl.MockCommandExecutor, api *mock_ecr.MockECRAPI, fs *mock_awsctl.MockFileSystemInterface) {
				exec.EXPECT().LookPath("docker").Return("/usr/bin/docker", nil)
				api.EXPECT().GetAuthorizationToken(gomock.Any(), gomock.Any()).Return(&ecr.GetAuthorizationTokenOutput{
					AuthorizationData: []types.AuthorizationData{
						{
							AuthorizationToken: aws.String(base64.StdEncoding.EncodeToString([]byte("AWS:secretpassword"))),
							ProxyEndpoint:      aws.String("https://123456789012.dkr.ecr.us-east-1.amazonaws.com"),
						},
					},
				}, nil)
				exec.EXPECT().RunCommandWithInput(
					"docker",
					"secretpassword",
					[]string{"login", "--username", "AWS", "--password-stdin", "https://123456789012.dkr.ecr.us-east-1.amazonaws.com"},
				).Return([]byte("Login Failed"), errors.New("login error"))
			},
			expectedError: "failed to execute docker login: login error",
		},
		{
			name: "UserHomeDir fails",
			setupMocks: func(exec *mock_awsctl.MockCommandExecutor, api *mock_ecr.MockECRAPI, fs *mock_awsctl.MockFileSystemInterface) {
				exec.EXPECT().LookPath("docker").Return("/usr/bin/docker", nil)
				api.EXPECT().GetAuthorizationToken(gomock.Any(), gomock.Any()).Return(&ecr.GetAuthorizationTokenOutput{
					AuthorizationData: []types.AuthorizationData{
						{
							AuthorizationToken: aws.String(base64.StdEncoding.EncodeToString([]byte("AWS:secretpassword"))),
							ProxyEndpoint:      aws.String("https://123456789012.dkr.ecr.us-east-1.amazonaws.com"),
						},
					},
				}, nil)
				exec.EXPECT().RunCommandWithInput(
					"docker",
					"secretpassword",
					[]string{"login", "--username", "AWS", "--password-stdin", "https://123456789012.dkr.ecr.us-east-1.amazonaws.com"},
				).Return([]byte("Login Succeeded"), nil)
				fs.EXPECT().UserHomeDir().Return("", errors.New("home dir error"))
			},
			expectedError: "failed to get home directory: home dir error",
		},
		{
			name: "MkdirAll fails",
			setupMocks: func(exec *mock_awsctl.MockCommandExecutor, api *mock_ecr.MockECRAPI, fs *mock_awsctl.MockFileSystemInterface) {
				exec.EXPECT().LookPath("docker").Return("/usr/bin/docker", nil)
				api.EXPECT().GetAuthorizationToken(gomock.Any(), gomock.Any()).Return(&ecr.GetAuthorizationTokenOutput{
					AuthorizationData: []types.AuthorizationData{
						{
							AuthorizationToken: aws.String(base64.StdEncoding.EncodeToString([]byte("AWS:secretpassword"))),
							ProxyEndpoint:      aws.String("https://123456789012.dkr.ecr.us-east-1.amazonaws.com"),
						},
					},
				}, nil)
				exec.EXPECT().RunCommandWithInput(
					"docker",
					"secretpassword",
					[]string{"login", "--username", "AWS", "--password-stdin", "https://123456789012.dkr.ecr.us-east-1.amazonaws.com"},
				).Return([]byte("Login Succeeded"), nil)
				fs.EXPECT().UserHomeDir().Return("/home/user", nil)
				fs.EXPECT().MkdirAll("/home/user/.docker", gomock.Any()).Return(errors.New("mkdir error"))
			},
			expectedError: "failed to create Docker config directory: mkdir error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockECRAPI := mock_ecr.NewMockECRAPI(ctrl)
			mockFileSystem := mock_awsctl.NewMockFileSystemInterface(ctrl)
			mockExecutor := mock_awsctl.NewMockCommandExecutor(ctrl)

			if tt.setupMocks != nil {
				tt.setupMocks(mockExecutor, mockECRAPI, mockFileSystem)
			}

			adapter := &internalECR.AwsECRAdapter{
				Client:     mockECRAPI,
				Cfg:        aws.Config{Region: "us-east-1"},
				FileSystem: mockFileSystem,
				Executor:   mockExecutor,
			}

			err := adapter.Login(context.TODO())

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
