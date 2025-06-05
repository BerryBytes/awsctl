package eks

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/BerryBytes/awsctl/models"
	mock_awsctl "github.com/BerryBytes/awsctl/tests/mock"
	mock_sso "github.com/BerryBytes/awsctl/tests/mock/sso"
	promptUtils "github.com/BerryBytes/awsctl/utils/prompt"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func TestNewEPrompter(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPrompt := mock_awsctl.NewMockPrompter(ctrl)
	mockConfigClient := mock_sso.NewMockSSOClient(ctrl)

	prompter := NewEPrompter(mockPrompt, mockConfigClient)

	assert.NotNil(t, prompter)
	assert.Equal(t, mockPrompt, prompter.Prompt)
	assert.Equal(t, mockConfigClient, prompter.AWSConfigClient)
}

func TestPromptForRegion_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPrompt := mock_awsctl.NewMockPrompter(ctrl)
	mockPrompt.EXPECT().
		PromptForInputWithValidation("Enter AWS region:", "", gomock.Any()).
		Return("us-west-2", nil)

	prompter := &EPrompter{Prompt: mockPrompt}

	region, err := prompter.PromptForRegion()

	assert.NoError(t, err)
	assert.Equal(t, "us-west-2", region)
}

func TestPromptForRegion_Empty(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPrompt := mock_awsctl.NewMockPrompter(ctrl)
	mockPrompt.EXPECT().
		PromptForInputWithValidation("Enter AWS region:", "", gomock.Any()).
		Return("", fmt.Errorf("invalid AWS region format or unrecognized region: "))

	prompter := &EPrompter{Prompt: mockPrompt}

	_, err := prompter.PromptForRegion()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid AWS region format")
}

func TestPromptForRegion_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPrompt := mock_awsctl.NewMockPrompter(ctrl)
	mockPrompt.EXPECT().
		PromptForInputWithValidation("Enter AWS region:", "", gomock.Any()).
		Return("", errors.New("input error"))

	prompter := &EPrompter{Prompt: mockPrompt}

	_, err := prompter.PromptForRegion()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "input error")
}

func TestPromptForRegion_Interrupted(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPrompt := mock_awsctl.NewMockPrompter(ctrl)
	mockPrompt.EXPECT().
		PromptForInputWithValidation("Enter AWS region:", "", gomock.Any()).
		Return("", promptUtils.ErrInterrupted)

	prompter := &EPrompter{Prompt: mockPrompt}

	_, err := prompter.PromptForRegion()

	assert.ErrorIs(t, err, promptUtils.ErrInterrupted)
}

func TestPromptForEKSCluster_NoClusters(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPrompt := mock_awsctl.NewMockPrompter(ctrl)

	gomock.InOrder(

		mockPrompt.EXPECT().
			PromptForInputWithValidation("Enter EKS cluster name:", "", gomock.Any()).
			Return("test-cluster", nil),

		mockPrompt.EXPECT().
			PromptForInputWithValidation("Enter EKS cluster endpoint (e.g., https://<endpoint>):", "", gomock.Any()).
			Return("https://ABCD123.gr7.us-west-2.eks.amazonaws.com", nil),

		mockPrompt.EXPECT().
			PromptForInputWithValidation("Enter Certificate Authority data (base64):", "", gomock.Any()).
			Return(base64.StdEncoding.EncodeToString([]byte("-----BEGIN CERTIFICATE-----test-data")), nil),
		mockPrompt.EXPECT().
			PromptForInputWithValidation("Enter AWS region:", "", gomock.Any()).
			Return("us-west-2", nil),
	)

	prompter := &EPrompter{Prompt: mockPrompt}

	clusterName, err := prompter.PromptForEKSCluster([]models.EKSCluster{})

	assert.NoError(t, err)
	assert.Equal(t, "test-cluster", clusterName)
}

func TestPromptForEKSCluster_WithClusters(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	clusters := []models.EKSCluster{
		{ClusterName: "cluster1", Region: "us-west-1"},
		{ClusterName: "cluster2", Region: "us-east-1"},
	}

	mockPrompt := mock_awsctl.NewMockPrompter(ctrl)
	mockPrompt.EXPECT().
		PromptForSelection("Select an EKS cluster:", []string{
			"cluster1 (us-west-1)",
			"cluster2 (us-east-1)",
		}).
		Return("cluster1 (us-west-1)", nil)

	prompter := &EPrompter{Prompt: mockPrompt}

	clusterName, err := prompter.PromptForEKSCluster(clusters)

	assert.NoError(t, err)
	assert.Equal(t, "cluster1", clusterName)
}

func TestPromptForEKSCluster_SelectionError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	clusters := []models.EKSCluster{
		{ClusterName: "cluster1", Region: "us-west-1"},
	}

	mockPrompt := mock_awsctl.NewMockPrompter(ctrl)
	mockPrompt.EXPECT().
		PromptForSelection("Select an EKS cluster:", []string{"cluster1 (us-west-1)"}).
		Return("", errors.New("selection error"))

	prompter := &EPrompter{Prompt: mockPrompt}

	_, err := prompter.PromptForEKSCluster(clusters)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to select EKS cluster")
}

func TestPromptForProfile_EnvVarSet(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	err := os.Setenv("AWS_PROFILE", "test-profile")
	assert.NoError(t, err)
	defer func() {
		err := os.Unsetenv("AWS_PROFILE")
		assert.NoError(t, err)
	}()

	mockConfigClient := mock_sso.NewMockSSOClient(ctrl)
	prompter := &EPrompter{AWSConfigClient: mockConfigClient}

	profile, err := prompter.PromptForProfile()

	assert.NoError(t, err)
	assert.Equal(t, "test-profile", profile)
}

func TestPromptForProfile_SingleProfile(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConfigClient := mock_sso.NewMockSSOClient(ctrl)
	mockConfigClient.EXPECT().
		ValidProfiles().
		Return([]string{"default"}, nil)

	prompter := &EPrompter{AWSConfigClient: mockConfigClient}

	profile, err := prompter.PromptForProfile()

	assert.NoError(t, err)
	assert.Equal(t, "default", profile)
}

func TestPromptForProfile_MultipleProfiles(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPrompt := mock_awsctl.NewMockPrompter(ctrl)
	mockPrompt.EXPECT().
		PromptForSelection("Select an AWS profile:", []string{"profile1", "profile2"}).
		Return("profile2", nil)

	mockConfigClient := mock_sso.NewMockSSOClient(ctrl)
	mockConfigClient.EXPECT().
		ValidProfiles().
		Return([]string{"profile1", "profile2"}, nil)

	prompter := &EPrompter{
		Prompt:          mockPrompt,
		AWSConfigClient: mockConfigClient,
	}

	profile, err := prompter.PromptForProfile()

	assert.NoError(t, err)
	assert.Equal(t, "profile2", profile)
}

func TestPromptForProfile_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConfigClient := mock_sso.NewMockSSOClient(ctrl)
	mockConfigClient.EXPECT().
		ValidProfiles().
		Return(nil, errors.New("config error"))

	prompter := &EPrompter{AWSConfigClient: mockConfigClient}

	_, err := prompter.PromptForProfile()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to list valid profiles")
}

func TestSelectEKSAction(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tests := []struct {
		name          string
		mockReturn    string
		expected      EKSAction
		expectedError bool
	}{
		{
			name:          "Update kubeconfig",
			mockReturn:    "Update kubeconfig",
			expected:      UpdateKubeConfig,
			expectedError: false,
		},
		{
			name:          "Exit",
			mockReturn:    "Exit",
			expected:      ExitEKS,
			expectedError: false,
		},
		{
			name:          "Error",
			mockReturn:    "",
			expected:      ExitEKS,
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockPrompt := mock_awsctl.NewMockPrompter(ctrl)
			if tt.expectedError {
				mockPrompt.EXPECT().
					PromptForSelection("Select an EKS action:", []string{"Update kubeconfig", "Exit"}).
					Return("", errors.New("selection error"))
			} else {
				mockPrompt.EXPECT().
					PromptForSelection("Select an EKS action:", []string{"Update kubeconfig", "Exit"}).
					Return(tt.mockReturn, nil)
			}

			prompter := &EPrompter{Prompt: mockPrompt}

			action, err := prompter.SelectEKSAction()

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, action)
			}
		})
	}
}

func TestPromptForManualCluster(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPrompt := mock_awsctl.NewMockPrompter(ctrl)

	gomock.InOrder(
		mockPrompt.EXPECT().
			PromptForInputWithValidation(
				"Enter EKS cluster name:",
				"",
				gomock.Any(),
			).Return("test-cluster", nil),

		mockPrompt.EXPECT().
			PromptForInputWithValidation(
				"Enter EKS cluster endpoint (e.g., https://<endpoint>):",
				"",
				gomock.Any(),
			).Return("https://ABCD123.gr7.us-west-2.eks.amazonaws.com", nil),

		mockPrompt.EXPECT().
			PromptForInputWithValidation(
				"Enter Certificate Authority data (base64):",
				"",
				gomock.Any(),
			).Return(base64.StdEncoding.EncodeToString([]byte("-----BEGIN CERTIFICATE-----...")), nil),

		mockPrompt.EXPECT().
			PromptForInputWithValidation(
				"Enter AWS region:",
				"",
				gomock.Any(),
			).Return("us-west-2", nil),
	)

	prompter := &EPrompter{Prompt: mockPrompt}

	clusterName, endpoint, caData, region, err := prompter.PromptForManualCluster()

	assert.NoError(t, err)
	assert.Equal(t, "test-cluster", clusterName)
	assert.Equal(t, "https://ABCD123.gr7.us-west-2.eks.amazonaws.com", endpoint)
	assert.NotEmpty(t, caData)
	assert.Equal(t, "us-west-2", region)
}

func TestGetAWSConfig_FromEnv(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	err := os.Setenv("AWS_PROFILE", "test-profile")
	assert.NoError(t, err)
	err = os.Setenv("AWS_REGION", "us-west-2")
	assert.NoError(t, err)
	defer func() {
		err := os.Unsetenv("AWS_PROFILE")
		assert.NoError(t, err)
		err = os.Unsetenv("AWS_REGION")
		assert.NoError(t, err)
	}()

	mockConfigClient := mock_sso.NewMockSSOClient(ctrl)
	prompter := &EPrompter{AWSConfigClient: mockConfigClient}

	profile, region, err := prompter.GetAWSConfig()

	assert.NoError(t, err)
	assert.Equal(t, "test-profile", profile)
	assert.Equal(t, "us-west-2", region)
}

func TestGetAWSConfig_PromptForRegion(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	err := os.Setenv("AWS_PROFILE", "test-profile")
	assert.NoError(t, err)
	defer func() {
		err := os.Unsetenv("AWS_PROFILE")
		assert.NoError(t, err)
	}()

	mockPrompt := mock_awsctl.NewMockPrompter(ctrl)
	mockPrompt.EXPECT().
		PromptForInputWithValidation("Enter AWS region:", "", gomock.Any()).
		Return("us-east-1", nil)

	mockConfigClient := mock_sso.NewMockSSOClient(ctrl)
	prompter := &EPrompter{
		Prompt:          mockPrompt,
		AWSConfigClient: mockConfigClient,
	}

	profile, region, err := prompter.GetAWSConfig()

	assert.NoError(t, err)
	assert.Equal(t, "test-profile", profile)
	assert.Equal(t, "us-east-1", region)
}

func TestGetAWSConfig_PromptForBoth(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPrompt := mock_awsctl.NewMockPrompter(ctrl)
	gomock.InOrder(
		mockPrompt.EXPECT().
			PromptForSelection("Select AWS profile:", []string{"profile1", "profile2"}).
			Return("profile2", nil),
		mockPrompt.EXPECT().
			PromptForInputWithValidation("Enter AWS region:", "", gomock.Any()).
			Return("eu-west-1", nil),
	)

	mockConfigClient := mock_sso.NewMockSSOClient(ctrl)
	mockConfigClient.EXPECT().
		ValidProfiles().
		Return([]string{"profile1", "profile2"}, nil)

	prompter := &EPrompter{
		Prompt:          mockPrompt,
		AWSConfigClient: mockConfigClient,
	}

	profile, region, err := prompter.GetAWSConfig()

	assert.NoError(t, err)
	assert.Equal(t, "profile2", profile)
	assert.Equal(t, "eu-west-1", region)
}

func TestPromptForEKSCluster_Interrupted(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	clusters := []models.EKSCluster{
		{ClusterName: "cluster1", Region: "us-west-1"},
	}

	mockPrompt := mock_awsctl.NewMockPrompter(ctrl)
	mockPrompt.EXPECT().
		PromptForSelection("Select an EKS cluster:", []string{"cluster1 (us-west-1)"}).
		Return("", promptUtils.ErrInterrupted)

	prompter := &EPrompter{Prompt: mockPrompt}

	_, err := prompter.PromptForEKSCluster(clusters)

	assert.ErrorIs(t, err, promptUtils.ErrInterrupted)
}

func TestPromptForEKSCluster_InvalidSelection(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	clusters := []models.EKSCluster{
		{ClusterName: "cluster1", Region: "us-west-1"},
	}

	mockPrompt := mock_awsctl.NewMockPrompter(ctrl)
	mockPrompt.EXPECT().
		PromptForSelection("Select an EKS cluster:", []string{"cluster1 (us-west-1)"}).
		Return("invalid-cluster", nil)

	prompter := &EPrompter{Prompt: mockPrompt}

	_, err := prompter.PromptForEKSCluster(clusters)

	assert.Error(t, err)
	assert.Equal(t, "invalid selection", err.Error())
}

func TestPromptForProfile_NoValidProfiles(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConfigClient := mock_sso.NewMockSSOClient(ctrl)
	mockConfigClient.EXPECT().
		ValidProfiles().
		Return([]string{}, nil)

	prompter := &EPrompter{AWSConfigClient: mockConfigClient}

	_, err := prompter.PromptForProfile()

	assert.Error(t, err)
	assert.Equal(t, "no valid AWS profiles found", err.Error())
}

func TestSelectEKSAction_InvalidAction(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPrompt := mock_awsctl.NewMockPrompter(ctrl)
	mockPrompt.EXPECT().
		PromptForSelection("Select an EKS action:", []string{"Update kubeconfig", "Exit"}).
		Return("Invalid Action", nil)

	prompter := &EPrompter{Prompt: mockPrompt}

	_, err := prompter.SelectEKSAction()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid action selected")
}

func TestGetAWSConfig_NoProfiles(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConfigClient := mock_sso.NewMockSSOClient(ctrl)
	mockConfigClient.EXPECT().
		ValidProfiles().
		Return(nil, errors.New("config error"))

	prompter := &EPrompter{AWSConfigClient: mockConfigClient}

	_, _, err := prompter.GetAWSConfig()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to retrieve AWS profiles")
}

func TestGetAWSConfig_EmptyProfiles(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConfigClient := mock_sso.NewMockSSOClient(ctrl)
	mockConfigClient.EXPECT().
		ValidProfiles().
		Return([]string{}, nil)

	prompter := &EPrompter{AWSConfigClient: mockConfigClient}

	_, _, err := prompter.GetAWSConfig()

	assert.Error(t, err)
	assert.Equal(t, "no AWS profiles found", err.Error())
}

func TestPromptForManualCluster_ValidationErrors(t *testing.T) {
	tests := []struct {
		name          string
		mockResponses []interface{}
		expectedError string
	}{
		{
			name: "Empty cluster name",
			mockResponses: []interface{}{
				[]interface{}{"", fmt.Errorf("cluster name cannot be empty")},
			},
			expectedError: "cluster name cannot be empty",
		},
		{
			name: "Cluster name too long",
			mockResponses: []interface{}{
				[]interface{}{"this-cluster-name-is-way-too-long-to-be-accepted-by-the-validation", fmt.Errorf("cluster name must be 40 characters or less (recommended for usability)")},
			},
			expectedError: "cluster name must be 40 characters or less",
		},
		{
			name: "Invalid cluster name format",
			mockResponses: []interface{}{
				[]interface{}{"123-cluster", fmt.Errorf("invalid format. Must:\n- Start with a letter\n- Contain only [a-z, A-Z, 0-9, -, _]\n- Not end with '-' or '_'")},
			},
			expectedError: "invalid format",
		},
		{
			name: "Invalid endpoint - no https",
			mockResponses: []interface{}{
				[]interface{}{"valid-cluster", nil},
				[]interface{}{"http://invalid.endpoint", fmt.Errorf("endpoint must start with https://")},
			},
			expectedError: "endpoint must start with https://",
		},
		{
			name: "Invalid endpoint - invalid URL",
			mockResponses: []interface{}{
				[]interface{}{"valid-cluster", nil},
				[]interface{}{"https://invalid::url", fmt.Errorf("invalid URL format: parse \"https://invalid::url\": invalid port \"::url\" after host")},
			},
			expectedError: "invalid URL format",
		},
		{
			name: "Invalid endpoint - missing hostname",
			mockResponses: []interface{}{
				[]interface{}{"valid-cluster", nil},
				[]interface{}{"https://", fmt.Errorf("missing hostname in endpoint")},
			},
			expectedError: "missing hostname in endpoint",
		},
		{
			name: "Invalid endpoint - invalid format",
			mockResponses: []interface{}{
				[]interface{}{"valid-cluster", nil},
				[]interface{}{"https://invalid.example.com", fmt.Errorf("invalid EKS endpoint format")},
			},
			expectedError: "invalid EKS endpoint format",
		},
		{
			name: "Empty CA data",
			mockResponses: []interface{}{
				[]interface{}{"valid-cluster", nil},
				[]interface{}{"https://ABCD123.gr7.us-west-2.eks.amazonaws.com", nil},
				[]interface{}{"", fmt.Errorf("CA data cannot be empty")},
			},
			expectedError: "CA data cannot be empty",
		},
		{
			name: "Invalid CA data - not base64",
			mockResponses: []interface{}{
				[]interface{}{"valid-cluster", nil},
				[]interface{}{"https://ABCD123.gr7.us-west-2.eks.amazonaws.com", nil},
				[]interface{}{"not-base64-data", fmt.Errorf("invalid base64 data")},
			},
			expectedError: "invalid base64 data",
		},
		{
			name: "Invalid CA data - not PEM",
			mockResponses: []interface{}{
				[]interface{}{"valid-cluster", nil},
				[]interface{}{"https://ABCD123.gr7.us-west-2.eks.amazonaws.com", nil},
				[]interface{}{base64.StdEncoding.EncodeToString([]byte("not-a-certificate")), fmt.Errorf("CA data should be a PEM certificate in base64 format")},
			},
			expectedError: "CA data should be a PEM certificate in base64 format",
		},
		{
			name: "Empty region",
			mockResponses: []interface{}{
				[]interface{}{"valid-cluster", nil},
				[]interface{}{"https://ABCD123.gr7.us-west-2.eks.amazonaws.com", nil},
				[]interface{}{base64.StdEncoding.EncodeToString([]byte("-----BEGIN CERTIFICATE-----test-data")), nil},
				[]interface{}{"", fmt.Errorf("AWS region cannot be empty")},
			},
			expectedError: "AWS region cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockPrompt := mock_awsctl.NewMockPrompter(ctrl)
			var call *gomock.Call

			for i, response := range tt.mockResponses {
				resp := response.([]interface{})
				input, err := resp[0].(string), resp[1]
				switch i {
				case 0:
					call = mockPrompt.EXPECT().
						PromptForInputWithValidation("Enter EKS cluster name:", "", gomock.Any()).
						Return(input, err)
				case 1:
					call = mockPrompt.EXPECT().
						PromptForInputWithValidation("Enter EKS cluster endpoint (e.g., https://<endpoint>):", "", gomock.Any()).
						After(call).
						Return(input, err)
				case 2:
					call = mockPrompt.EXPECT().
						PromptForInputWithValidation("Enter Certificate Authority data (base64):", "", gomock.Any()).
						After(call).
						Return(input, err)
				case 3:
					call = mockPrompt.EXPECT().
						PromptForInputWithValidation("Enter AWS region:", "", gomock.Any()).
						After(call).
						Return(input, err)
				}
			}

			prompter := &EPrompter{Prompt: mockPrompt}

			_, _, _, _, err := prompter.PromptForManualCluster()

			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedError)
		})
	}
}
