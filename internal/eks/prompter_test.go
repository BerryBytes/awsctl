package eks

import (
	"errors"
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
		PromptForInput("Enter AWS region (e.g., us-east-1):", "").
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
		PromptForInput("Enter AWS region (e.g., us-east-1):", "").
		Return("", nil)

	prompter := &EPrompter{Prompt: mockPrompt}

	_, err := prompter.PromptForRegion()

	assert.Error(t, err)
	assert.Equal(t, "AWS region cannot be empty", err.Error())
}

func TestPromptForRegion_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPrompt := mock_awsctl.NewMockPrompter(ctrl)
	mockPrompt.EXPECT().
		PromptForInput("Enter AWS region (e.g., us-east-1):", "").
		Return("", errors.New("input error"))

	prompter := &EPrompter{Prompt: mockPrompt}

	_, err := prompter.PromptForRegion()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get AWS region")
}

func TestPromptForRegion_Interrupted(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPrompt := mock_awsctl.NewMockPrompter(ctrl)
	mockPrompt.EXPECT().
		PromptForInput("Enter AWS region (e.g., us-east-1):", "").
		Return("", promptUtils.ErrInterrupted)

	prompter := &EPrompter{Prompt: mockPrompt}

	_, err := prompter.PromptForRegion()

	assert.ErrorIs(t, err, promptUtils.ErrInterrupted)
}

func TestPromptForEKSCluster_NoClusters(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPrompt := mock_awsctl.NewMockPrompter(ctrl)
	mockPrompt.EXPECT().
		PromptForInput("Enter EKS cluster name:", "").
		Return("test-cluster", nil)
	mockPrompt.EXPECT().
		PromptForInput("Enter EKS cluster endpoint (e.g., https://<endpoint>):", "").
		Return("https://test.endpoint", nil)
	mockPrompt.EXPECT().
		PromptForInput("Enter Certificate Authority data (base64):", "").
		Return("test-ca-data", nil)
	mockPrompt.EXPECT().
		PromptForInput("Enter AWS region (e.g., us-east-1):", "").
		Return("us-west-2", nil)

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
			PromptForInput("Enter EKS cluster name:", "").
			Return("test-cluster", nil),
		mockPrompt.EXPECT().
			PromptForInput("Enter EKS cluster endpoint (e.g., https://<endpoint>):", "").
			Return("https://test.endpoint", nil),
		mockPrompt.EXPECT().
			PromptForInput("Enter Certificate Authority data (base64):", "").
			Return("test-ca-data", nil),
		mockPrompt.EXPECT().
			PromptForInput("Enter AWS region (e.g., us-east-1):", "").
			Return("us-west-2", nil),
	)

	prompter := &EPrompter{Prompt: mockPrompt}

	clusterName, endpoint, caData, region, err := prompter.PromptForManualCluster()

	assert.NoError(t, err)
	assert.Equal(t, "test-cluster", clusterName)
	assert.Equal(t, "https://test.endpoint", endpoint)
	assert.Equal(t, "test-ca-data", caData)
	assert.Equal(t, "us-west-2", region)
}

func TestPromptForManualCluster_InvalidEndpoint(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPrompt := mock_awsctl.NewMockPrompter(ctrl)
	gomock.InOrder(
		mockPrompt.EXPECT().
			PromptForInput("Enter EKS cluster name:", "").
			Return("test-cluster", nil),
		mockPrompt.EXPECT().
			PromptForInput("Enter EKS cluster endpoint (e.g., https://<endpoint>):", "").
			Return("http://test.endpoint", nil),
	)

	prompter := &EPrompter{Prompt: mockPrompt}

	_, _, _, _, err := prompter.PromptForManualCluster()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid endpoint format")
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
		PromptForInput("Enter AWS region (e.g., us-east-1):", "").
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
			PromptForInput("Enter AWS region (e.g., us-east-1):", "").
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

func TestPromptForManualCluster_ErrorCases(t *testing.T) {
	tests := []struct {
		name          string
		mockResponses []interface{}
		expectedError string
	}{
		{
			name: "Cluster name error",
			mockResponses: []interface{}{
				[]interface{}{"", errors.New("name error")},
			},
			expectedError: "failed to input cluster name",
		},
		{
			name: "Endpoint error",
			mockResponses: []interface{}{
				[]interface{}{"test-cluster", nil},
				[]interface{}{"", errors.New("endpoint error")},
			},
			expectedError: "failed to input endpoint",
		},
		{
			name: "CA data error",
			mockResponses: []interface{}{
				[]interface{}{"test-cluster", nil},
				[]interface{}{"https://test.endpoint", nil},
				[]interface{}{"", errors.New("ca data error")},
			},
			expectedError: "failed to input CA data",
		},
		{
			name: "Region error",
			mockResponses: []interface{}{
				[]interface{}{"test-cluster", nil},
				[]interface{}{"https://test.endpoint", nil},
				[]interface{}{"test-ca-data", nil},
				[]interface{}{"", errors.New("region error")},
			},
			expectedError: "failed to input region",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockPrompt := mock_awsctl.NewMockPrompter(ctrl)
			call := mockPrompt.EXPECT().
				PromptForInput("Enter EKS cluster name:", "").
				Return(tt.mockResponses[0].([]interface{})...)

			for i := 1; i < len(tt.mockResponses); i++ {
				switch i {
				case 1:
					call = mockPrompt.EXPECT().
						PromptForInput("Enter EKS cluster endpoint (e.g., https://<endpoint>):", "").
						After(call).
						Return(tt.mockResponses[i].([]interface{})...)
				case 2:
					call = mockPrompt.EXPECT().
						PromptForInput("Enter Certificate Authority data (base64):", "").
						After(call).
						Return(tt.mockResponses[i].([]interface{})...)
				case 3:
					mockPrompt.EXPECT().
						PromptForInput("Enter AWS region (e.g., us-east-1):", "").
						After(call).
						Return(tt.mockResponses[i].([]interface{})...)
				}
			}

			prompter := &EPrompter{Prompt: mockPrompt}

			_, _, _, _, err := prompter.PromptForManualCluster()

			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedError)
		})
	}
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
