package ecr_test

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/BerryBytes/awsctl/internal/ecr"
	mock_awsctl "github.com/BerryBytes/awsctl/tests/mock"
	mock_ecr "github.com/BerryBytes/awsctl/tests/mock/ecr"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func TestNewECRService(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConnServices := mock_awsctl.NewMockServicesInterface(ctrl)
	mockAWSClient := mock_ecr.NewMockProfileProvider(ctrl)

	service := ecr.NewECRService(mockConnServices, mockAWSClient)

	assert.NotNil(t, service)
	assert.NotNil(t, service.EPrompter)
	assert.NotNil(t, service.CPrompter)
	assert.Equal(t, mockAWSClient, service.AWSClient)
	assert.Equal(t, mockConnServices, service.ConnServices)
}

func TestECRService_Run(t *testing.T) {
	tests := []struct {
		name          string
		action        ecr.ECRAction
		promptError   error
		loginError    error
		expectedError string
	}{
		{
			name:   "Successful login",
			action: ecr.LoginECR,
		},
		{
			name:   "Exit action",
			action: ecr.ExitECR,
		},
		{
			name:          "Prompt error",
			promptError:   errors.New("prompt error"),
			expectedError: "action selection aborted: prompt error",
		},
		{
			name:          "Login error",
			action:        ecr.LoginECR,
			loginError:    errors.New("login error"),
			expectedError: "ECR login failed: login error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockEPrompter := mock_ecr.NewMockECRPromptInterface(ctrl)
			mockConnServices := mock_awsctl.NewMockServicesInterface(ctrl)
			mockAWSClient := mock_ecr.NewMockProfileProvider(ctrl)
			mockCPrompter := mock_awsctl.NewMockConnectionPrompter(ctrl)
			mockPrompt := mock_awsctl.NewMockPrompter(ctrl)
			mockConfigLoader := mock_ecr.NewMockConfigLoader(ctrl)
			mockECRClientFactory := mock_ecr.NewMockECRClientFactory(ctrl)
			mockECRClient := mock_ecr.NewMockECRAdapterInterface(ctrl)
			mockFileSystem := mock_awsctl.NewMockFileSystemInterface(ctrl)
			mockExecutor := mock_awsctl.NewMockCommandExecutor(ctrl)

			service := &ecr.ECRService{
				EPrompter:        mockEPrompter,
				CPrompter:        mockCPrompter,
				AWSClient:        mockAWSClient,
				ConnServices:     mockConnServices,
				Prompt:           mockPrompt,
				ConfigLoader:     mockConfigLoader,
				ECRClientFactory: mockECRClientFactory,
				ECRClient:        nil, // Set to nil to test NewECRClient
				FileSystem:       mockFileSystem,
				Executor:         mockExecutor,
			}

			mockConnServices.EXPECT().IsAWSConfigured().Return(true).AnyTimes()
			mockEPrompter.EXPECT().SelectECRAction().Return(tt.action, tt.promptError)

			if tt.promptError == nil && tt.action == ecr.LoginECR {
				mockCPrompter.EXPECT().PromptForConfirmation("Login to AWS ECR?").Return(true, nil)
				mockCPrompter.EXPECT().PromptForRegion("").Return("us-east-1", nil)
				mockAWSClient.EXPECT().ValidProfiles().Return([]string{"default"}, nil)
				mockConfigLoader.EXPECT().LoadDefaultConfig(gomock.Any(), gomock.Any()).Return(aws.Config{}, nil)
				mockECRClientFactory.EXPECT().NewECRClient(gomock.Any(), mockFileSystem, mockExecutor).Return(mockECRClient)
				mockECRClient.EXPECT().Login(gomock.Any()).Return(tt.loginError)
			}

			err := service.Run()

			if tt.expectedError != "" {
				assert.Error(t, err, "Error expected")
				assert.Contains(t, err.Error(), tt.expectedError, "Error message should match")
			} else {
				assert.NoError(t, err, "No error expected")
			}
		})
	}
}

func TestECRService_HandleECRLogin(t *testing.T) {
	tests := []struct {
		name               string
		awsConfigured      bool
		confirm            bool
		confirmError       error
		region             string
		regionPromptError  error
		awsProfileEnv      string
		validProfiles      []string
		profilesError      error
		selectedProfile    string
		profileSelectError error
		configError        error
		loginError         error
		ecrClientPreSet    bool
		expectedError      string
		setupEnv           bool
	}{
		{
			name:               "Successful login with AWS_PROFILE",
			awsConfigured:      true,
			confirm:            true,
			confirmError:       nil,
			region:             "us-east-1",
			regionPromptError:  nil,
			awsProfileEnv:      "test-profile",
			validProfiles:      nil,
			profilesError:      nil,
			selectedProfile:    "",
			profileSelectError: nil,
			configError:        nil,
			loginError:         nil,
			ecrClientPreSet:    true,
			expectedError:      "",
			setupEnv:           true,
		},
		{
			name:               "Successful login with single profile",
			awsConfigured:      true,
			confirm:            true,
			confirmError:       nil,
			region:             "us-east-1",
			regionPromptError:  nil,
			awsProfileEnv:      "",
			validProfiles:      []string{"default"},
			profilesError:      nil,
			selectedProfile:    "",
			profileSelectError: nil,
			configError:        nil,
			loginError:         nil,
			ecrClientPreSet:    false,
			expectedError:      "",
			setupEnv:           false,
		},
		{
			name:               "Successful login with multiple profiles",
			awsConfigured:      true,
			confirm:            true,
			confirmError:       nil,
			region:             "us-east-1",
			regionPromptError:  nil,
			awsProfileEnv:      "",
			validProfiles:      []string{"p1", "p2"},
			profilesError:      nil,
			selectedProfile:    "p1",
			profileSelectError: nil,
			configError:        nil,
			loginError:         nil,
			ecrClientPreSet:    false,
			expectedError:      "",
			setupEnv:           false,
		},
		{
			name:               "AWS not configured",
			awsConfigured:      false,
			confirm:            true,
			confirmError:       nil,
			region:             "",
			regionPromptError:  nil,
			awsProfileEnv:      "",
			validProfiles:      nil,
			profilesError:      nil,
			selectedProfile:    "",
			profileSelectError: nil,
			configError:        nil,
			loginError:         nil,
			ecrClientPreSet:    false,
			expectedError:      "AWS configuration not found",
			setupEnv:           false,
		},
		{
			name:               "Confirmation denied",
			awsConfigured:      true,
			confirm:            false,
			confirmError:       nil,
			region:             "",
			regionPromptError:  nil,
			awsProfileEnv:      "",
			validProfiles:      nil,
			profilesError:      nil,
			selectedProfile:    "",
			profileSelectError: nil,
			configError:        nil,
			loginError:         nil,
			ecrClientPreSet:    false,
			expectedError:      "",
			setupEnv:           false,
		},
		{
			name:               "Confirmation error",
			awsConfigured:      true,
			confirm:            false,
			confirmError:       errors.New("confirm error"),
			region:             "",
			regionPromptError:  nil,
			awsProfileEnv:      "",
			validProfiles:      nil,
			profilesError:      nil,
			selectedProfile:    "",
			profileSelectError: nil,
			configError:        nil,
			loginError:         nil,
			ecrClientPreSet:    false,
			expectedError:      "failed to confirm ECR login: confirm error",
			setupEnv:           false,
		},
		{
			name:               "Region prompt error",
			awsConfigured:      true,
			confirm:            true,
			confirmError:       nil,
			region:             "",
			regionPromptError:  errors.New("region error"),
			awsProfileEnv:      "",
			validProfiles:      nil,
			profilesError:      nil,
			selectedProfile:    "",
			profileSelectError: nil,
			configError:        nil,
			loginError:         nil,
			ecrClientPreSet:    false,
			expectedError:      "failed to get region: region error",
			setupEnv:           false,
		},
		{
			name:               "Profiles error",
			awsConfigured:      true,
			confirm:            true,
			confirmError:       nil,
			region:             "us-east-1",
			regionPromptError:  nil,
			awsProfileEnv:      "",
			validProfiles:      nil,
			profilesError:      errors.New("profiles error"),
			selectedProfile:    "",
			profileSelectError: nil,
			configError:        nil,
			loginError:         nil,
			ecrClientPreSet:    false,
			expectedError:      "failed to list valid profiles: profiles error",
			setupEnv:           false,
		},
		{
			name:               "No profiles found",
			awsConfigured:      true,
			confirm:            true,
			confirmError:       nil,
			region:             "us-east-1",
			regionPromptError:  nil,
			awsProfileEnv:      "",
			validProfiles:      []string{},
			profilesError:      nil,
			selectedProfile:    "",
			profileSelectError: nil,
			configError:        nil,
			loginError:         nil,
			ecrClientPreSet:    false,
			expectedError:      "no valid AWS profiles found",
			setupEnv:           false,
		},
		{
			name:               "Profile selection error",
			awsConfigured:      true,
			confirm:            true,
			confirmError:       nil,
			region:             "us-east-1",
			regionPromptError:  nil,
			awsProfileEnv:      "",
			validProfiles:      []string{"p1", "p2"},
			profilesError:      nil,
			selectedProfile:    "",
			profileSelectError: errors.New("select error"),
			configError:        nil,
			loginError:         nil,
			ecrClientPreSet:    false,
			expectedError:      "failed to select AWS profile: select error",
			setupEnv:           false,
		},
		{
			name:               "Config load error",
			awsConfigured:      true,
			confirm:            true,
			confirmError:       nil,
			region:             "us-east-1",
			regionPromptError:  nil,
			awsProfileEnv:      "test-profile",
			validProfiles:      nil,
			profilesError:      nil,
			selectedProfile:    "",
			profileSelectError: nil,
			configError:        errors.New("config error"),
			loginError:         nil,
			ecrClientPreSet:    false,
			expectedError:      "failed to load AWS config: config error",
			setupEnv:           true,
		},
		{
			name:               "Login error",
			awsConfigured:      true,
			confirm:            true,
			confirmError:       nil,
			region:             "us-east-1",
			regionPromptError:  nil,
			awsProfileEnv:      "test-profile",
			validProfiles:      nil,
			profilesError:      nil,
			selectedProfile:    "",
			profileSelectError: nil,
			configError:        nil,
			loginError:         errors.New("login error"),
			ecrClientPreSet:    true,
			expectedError:      "ECR login failed: login error",
			setupEnv:           true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setupEnv {
				os.Setenv("AWS_PROFILE", tt.awsProfileEnv)
				defer os.Unsetenv("AWS_PROFILE")
			} else {
				os.Unsetenv("AWS_PROFILE")
			}

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockEPrompter := mock_ecr.NewMockECRPromptInterface(ctrl)
			mockAWSClient := mock_ecr.NewMockProfileProvider(ctrl)
			mockConnServices := mock_awsctl.NewMockServicesInterface(ctrl)
			mockCPrompter := mock_awsctl.NewMockConnectionPrompter(ctrl)
			mockPrompt := mock_awsctl.NewMockPrompter(ctrl)
			mockConfigLoader := mock_ecr.NewMockConfigLoader(ctrl)
			mockECRClientFactory := mock_ecr.NewMockECRClientFactory(ctrl)
			mockECRClient := mock_ecr.NewMockECRAdapterInterface(ctrl)
			mockFileSystem := mock_awsctl.NewMockFileSystemInterface(ctrl)
			mockExecutor := mock_awsctl.NewMockCommandExecutor(ctrl)

			service := &ecr.ECRService{
				EPrompter:        mockEPrompter,
				CPrompter:        mockCPrompter,
				AWSClient:        mockAWSClient,
				ConnServices:     mockConnServices,
				Prompt:           mockPrompt,
				ConfigLoader:     mockConfigLoader,
				ECRClientFactory: mockECRClientFactory,
				ECRClient:        nil,
				FileSystem:       mockFileSystem,
				Executor:         mockExecutor,
			}

			if tt.ecrClientPreSet {
				service.ECRClient = mockECRClient
			}

			mockConnServices.EXPECT().IsAWSConfigured().Return(tt.awsConfigured)

			if tt.awsConfigured {
				mockCPrompter.EXPECT().PromptForConfirmation("Login to AWS ECR?").Return(tt.confirm, tt.confirmError)

				if tt.confirm && tt.confirmError == nil {
					mockCPrompter.EXPECT().PromptForRegion("").Return(tt.region, tt.regionPromptError)

					if tt.regionPromptError == nil {
						if tt.awsProfileEnv == "" {
							mockAWSClient.EXPECT().ValidProfiles().Return(tt.validProfiles, tt.profilesError)
							if len(tt.validProfiles) > 1 {
								mockPrompt.EXPECT().PromptForSelection("Select AWS profile:", tt.validProfiles).
									Return(tt.selectedProfile, tt.profileSelectError)
							}
						}
						if tt.profilesError == nil && (len(tt.validProfiles) > 0 || tt.awsProfileEnv != "") && tt.profileSelectError == nil {
							mockConfigLoader.EXPECT().LoadDefaultConfig(gomock.Any(), gomock.Any()).Return(aws.Config{}, tt.configError)
							if tt.configError == nil {
								if !tt.ecrClientPreSet {
									mockECRClientFactory.EXPECT().NewECRClient(gomock.Any(), mockFileSystem, mockExecutor).Return(mockECRClient)
								}
								mockECRClient.EXPECT().Login(gomock.Any()).Return(tt.loginError)
							}
						}
					}
				}
			}

			err := service.HandleECRLogin()

			if tt.expectedError != "" {
				assert.Error(t, err, "Error expected")
				assert.Contains(t, err.Error(), tt.expectedError, "Error message should match")
			} else {
				assert.NoError(t, err, "No error expected")
			}
		})
	}
}

func TestRealConfigLoader_LoadDefaultConfig(t *testing.T) {

	loader := &ecr.RealConfigLoader{}
	cfg, err := loader.LoadDefaultConfig(context.TODO())
	if err == nil {
		assert.NotNil(t, cfg, "Config should be returned on success")
	}
}

func TestRealECRClientFactory_NewECRClient(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	factory := &ecr.RealECRClientFactory{}
	mockFileSystem := mock_awsctl.NewMockFileSystemInterface(ctrl)
	mockExecutor := mock_awsctl.NewMockCommandExecutor(ctrl)

	client := factory.NewECRClient(aws.Config{}, mockFileSystem, mockExecutor)

	assert.NotNil(t, client, "Client should be created")
	assert.Implements(t, (*ecr.ECRAdapterInterface)(nil), client, "Client should implement ECRAdapterInterface")
}
