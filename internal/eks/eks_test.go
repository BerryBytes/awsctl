package eks_test

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/BerryBytes/awsctl/internal/eks"
	"github.com/BerryBytes/awsctl/models"
	mock_awsctl "github.com/BerryBytes/awsctl/tests/mock"
	mock_eks "github.com/BerryBytes/awsctl/tests/mock/eks"
	"github.com/BerryBytes/awsctl/utils/common"
	promptUtils "github.com/BerryBytes/awsctl/utils/prompt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func TestNewEKSService(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConnServices := mock_awsctl.NewMockServicesInterface(ctrl)
	service := eks.NewEKSService(mockConnServices)

	assert.NotNil(t, service)
	assert.NotNil(t, service.EPrompter)
	assert.NotNil(t, service.ConnServices)
	assert.NotNil(t, service.ConfigLoader)
	assert.NotNil(t, service.EKSClientFactory)
	assert.NotNil(t, service.FileSystem)
}

func TestEKSService_Run_ExitAction(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEPrompter := mock_eks.NewMockEKSPromptInterface(ctrl)
	mockEPrompter.EXPECT().
		SelectEKSAction().
		Return(eks.ExitEKS, nil)

	service := &eks.EKSService{
		EPrompter: mockEPrompter,
	}

	err := service.Run()
	assert.NoError(t, err)
}

func TestEKSService_Run_UpdateKubeConfig(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	old := os.Stdout
	defer func() { os.Stdout = old }()
	os.Stdout = os.NewFile(0, os.DevNull)

	cluster := models.EKSCluster{ClusterName: "test-cluster"}
	testProfile := "default"
	testRegion := "us-west-2"

	mockEPrompter := mock_eks.NewMockEKSPromptInterface(ctrl)
	mockEPrompter.EXPECT().
		SelectEKSAction().
		Return(eks.UpdateKubeConfig, nil)
	mockEPrompter.EXPECT().
		PromptForProfile().
		Return(testProfile, nil)
	mockEPrompter.EXPECT().
		PromptForEKSCluster([]models.EKSCluster{cluster}).
		Return(cluster.ClusterName, nil)

	mockEKSClient := mock_eks.NewMockEKSAdapterInterface(ctrl)
	mockEKSClient.EXPECT().
		ListEKSClusters(context.TODO()).
		Return([]models.EKSCluster{cluster}, nil)
	mockEKSClient.EXPECT().
		UpdateKubeconfig(&cluster, testProfile).
		Return(nil)

	mockConnServices := mock_awsctl.NewMockServicesInterface(ctrl)
	mockConnServices.EXPECT().
		IsAWSConfigured().
		Return(true)

	mockCPrompter := mock_awsctl.NewMockConnectionPrompter(ctrl)
	mockCPrompter.EXPECT().
		PromptForConfirmation("Look for EKS clusters in AWS?").
		Return(true, nil)
	mockCPrompter.EXPECT().
		PromptForRegion("").
		Return(testRegion, nil)

	mockConfigLoader := mock_eks.NewMockConfigLoader(ctrl)

	mockFactory := mock_eks.NewMockEKSClientFactory(ctrl)

	service := &eks.EKSService{
		EPrompter:        mockEPrompter,
		CPrompter:        mockCPrompter,
		EKSClient:        mockEKSClient,
		ConnServices:     mockConnServices,
		ConfigLoader:     mockConfigLoader,
		EKSClientFactory: mockFactory,
		FileSystem:       &common.RealFileSystem{},
	}

	err := service.Run()
	assert.NoError(t, err)
}

func TestEKSService_Run_Interrupted(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEPrompter := mock_eks.NewMockEKSPromptInterface(ctrl)
	mockEPrompter.EXPECT().
		SelectEKSAction().
		Return(eks.ExitEKS, promptUtils.ErrInterrupted)

	service := &eks.EKSService{
		EPrompter: mockEPrompter,
	}

	err := service.Run()
	assert.Equal(t, promptUtils.ErrInterrupted, err)
}

func TestEKSService_getEKSClusterDetails_ManualFallback(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEPrompter := mock_eks.NewMockEKSPromptInterface(ctrl)
	mockEPrompter.EXPECT().
		PromptForManualCluster().
		Return("test-cluster", "https://test.endpoint", "ca-data", "us-west-2", nil)

	mockConnServices := mock_awsctl.NewMockServicesInterface(ctrl)
	mockConnServices.EXPECT().
		IsAWSConfigured().
		Return(false)

	service := &eks.EKSService{
		EPrompter:    mockEPrompter,
		ConnServices: mockConnServices,
	}

	cluster, profile, err := service.GetEKSClusterDetails()
	assert.NoError(t, err)
	assert.Equal(t, "test-cluster", cluster.ClusterName)
	assert.Equal(t, "", profile)
}

func TestEKSService_handleManualCluster_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEPrompter := mock_eks.NewMockEKSPromptInterface(ctrl)
	mockEPrompter.EXPECT().
		PromptForManualCluster().
		Return("test-cluster", "https://test.endpoint", "ca-data", "us-west-2", nil)

	service := &eks.EKSService{
		EPrompter: mockEPrompter,
	}

	cluster, profile, err := service.HandleManualCluster()
	assert.NoError(t, err)
	assert.Equal(t, "test-cluster", cluster.ClusterName)
	assert.Equal(t, "https://test.endpoint", cluster.Endpoint)
	assert.Equal(t, "us-west-2", cluster.Region)
	assert.Equal(t, "", profile)
}

func TestEKSService_handleManualCluster_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEPrompter := mock_eks.NewMockEKSPromptInterface(ctrl)
	mockEPrompter.EXPECT().
		PromptForManualCluster().
		Return("", "", "", "", errors.New("input error"))

	service := &eks.EKSService{
		EPrompter: mockEPrompter,
	}

	_, _, err := service.HandleManualCluster()
	assert.Error(t, err)
}

func TestEKSService_HandleKubeconfigUpdate_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	testCluster := models.EKSCluster{
		ClusterName: "test-cluster",
		Endpoint:    "https://test.endpoint",
		Region:      "us-west-2",
	}
	testProfile := "default"
	testRegion := "us-west-2"

	mockEKSClient := mock_eks.NewMockEKSAdapterInterface(ctrl)
	mockEKSClient.EXPECT().
		ListEKSClusters(gomock.Any()).
		Return([]models.EKSCluster{testCluster}, nil)
	mockEKSClient.EXPECT().
		UpdateKubeconfig(&testCluster, testProfile).
		Return(nil)

	mockFactory := mock_eks.NewMockEKSClientFactory(ctrl)
	mockFactory.EXPECT().
		NewEKSClient(gomock.Any(), gomock.Any()).
		Return(mockEKSClient)

	mockConfigLoader := mock_eks.NewMockConfigLoader(ctrl)
	mockConfigLoader.EXPECT().
		LoadDefaultConfig(
			gomock.Any(),
			gomock.Any(),
			gomock.Any(),
		).
		Return(aws.Config{}, nil)

	mockEPrompter := mock_eks.NewMockEKSPromptInterface(ctrl)
	mockEPrompter.EXPECT().
		PromptForProfile().
		Return(testProfile, nil)
	mockEPrompter.EXPECT().
		PromptForEKSCluster([]models.EKSCluster{testCluster}).
		Return(testCluster.ClusterName, nil)

	mockCPrompter := mock_awsctl.NewMockConnectionPrompter(ctrl)
	mockCPrompter.EXPECT().
		PromptForConfirmation("Look for EKS clusters in AWS?").
		Return(true, nil)
	mockCPrompter.EXPECT().
		PromptForRegion("").
		Return(testRegion, nil)

	mockConnServices := mock_awsctl.NewMockServicesInterface(ctrl)
	mockConnServices.EXPECT().
		IsAWSConfigured().
		Return(true)

	service := &eks.EKSService{
		EPrompter:        mockEPrompter,
		CPrompter:        mockCPrompter,
		ConnServices:     mockConnServices,
		ConfigLoader:     mockConfigLoader,
		EKSClientFactory: mockFactory,
		FileSystem:       &common.RealFileSystem{},
	}

	err := service.HandleKubeconfigUpdate()
	assert.NoError(t, err)
}

func TestEKSService_HandleKubeconfigUpdate_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	old := os.Stdout
	defer func() { os.Stdout = old }()
	os.Stdout = os.NewFile(0, os.DevNull)

	mockConnServices := mock_awsctl.NewMockServicesInterface(ctrl)
	mockConnServices.EXPECT().
		IsAWSConfigured().
		Return(true)

	mockCPrompter := mock_awsctl.NewMockConnectionPrompter(ctrl)
	mockCPrompter.EXPECT().
		PromptForConfirmation("Look for EKS clusters in AWS?").
		Return(true, nil)
	mockCPrompter.EXPECT().
		PromptForRegion("").
		Return("us-west-2", nil)

	mockEPrompter := mock_eks.NewMockEKSPromptInterface(ctrl)
	mockEPrompter.EXPECT().
		PromptForProfile().
		Return("", errors.New("profile error"))
	mockEPrompter.EXPECT().
		PromptForManualCluster().
		Return("", "", "", "", errors.New("manual input error"))

	mockConfigLoader := mock_eks.NewMockConfigLoader(ctrl)

	mockFactory := mock_eks.NewMockEKSClientFactory(ctrl)

	service := &eks.EKSService{
		EPrompter:        mockEPrompter,
		CPrompter:        mockCPrompter,
		ConnServices:     mockConnServices,
		ConfigLoader:     mockConfigLoader,
		EKSClientFactory: mockFactory,
		FileSystem:       &common.RealFileSystem{},
	}

	err := service.HandleKubeconfigUpdate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "manual input error")
}

func TestEKSService_getEKSClusterDetails_AWSConfigured(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	old := os.Stdout
	defer func() { os.Stdout = old }()
	os.Stdout = os.NewFile(0, os.DevNull)

	cluster := models.EKSCluster{ClusterName: "test-cluster"}
	testRegion := "us-west-2"
	testProfile := "default"

	mockEPrompter := mock_eks.NewMockEKSPromptInterface(ctrl)
	mockEPrompter.EXPECT().
		PromptForProfile().
		Return(testProfile, nil)
	mockEPrompter.EXPECT().
		PromptForEKSCluster([]models.EKSCluster{cluster}).
		Return(cluster.ClusterName, nil)

	mockEKSClient := mock_eks.NewMockEKSAdapterInterface(ctrl)
	mockEKSClient.EXPECT().
		ListEKSClusters(context.TODO()).
		Return([]models.EKSCluster{cluster}, nil)

	mockConnServices := mock_awsctl.NewMockServicesInterface(ctrl)
	mockConnServices.EXPECT().
		IsAWSConfigured().
		Return(true)

	mockCPrompter := mock_awsctl.NewMockConnectionPrompter(ctrl)
	mockCPrompter.EXPECT().
		PromptForConfirmation("Look for EKS clusters in AWS?").
		Return(true, nil)
	mockCPrompter.EXPECT().
		PromptForRegion("").
		Return(testRegion, nil)

	mockConfigLoader := mock_eks.NewMockConfigLoader(ctrl)
	mockConfigLoader.EXPECT().
		LoadDefaultConfig(
			context.TODO(),
			gomock.Any(),
			gomock.Any(),
		).
		Return(aws.Config{}, nil)

	mockFactory := mock_eks.NewMockEKSClientFactory(ctrl)
	mockFactory.EXPECT().
		NewEKSClient(gomock.Any(), gomock.Any()).
		Return(mockEKSClient)

	service := &eks.EKSService{
		EPrompter:        mockEPrompter,
		CPrompter:        mockCPrompter,
		ConnServices:     mockConnServices,
		ConfigLoader:     mockConfigLoader,
		EKSClientFactory: mockFactory,
		FileSystem:       &common.RealFileSystem{},
	}

	resultCluster, profile, err := service.GetEKSClusterDetails()
	assert.NoError(t, err)
	assert.Equal(t, cluster.ClusterName, resultCluster.ClusterName)
	assert.Equal(t, testProfile, profile)
}

func TestRealConfigLoader(t *testing.T) {
	loader := &eks.RealConfigLoader{}
	cfg, err := loader.LoadDefaultConfig(context.TODO())
	assert.NoError(t, err)
	assert.NotNil(t, cfg)
}

func TestRealEKSClientFactory(t *testing.T) {
	factory := &eks.RealEKSClientFactory{}
	cfg := aws.Config{Region: "us-west-2"}
	client := factory.NewEKSClient(cfg, &common.RealFileSystem{})
	assert.NotNil(t, client)
}

func TestEKSService_Run_ActionSelectionError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEPrompter := mock_eks.NewMockEKSPromptInterface(ctrl)
	mockEPrompter.EXPECT().
		SelectEKSAction().
		Return(eks.EKSAction(99), errors.New("selection error"))

	service := &eks.EKSService{
		EPrompter: mockEPrompter,
	}

	err := service.Run()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "action selection aborted")
}

func TestEKSService_Run_UpdateKubeConfigError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEPrompter := mock_eks.NewMockEKSPromptInterface(ctrl)
	mockEPrompter.EXPECT().
		SelectEKSAction().
		Return(eks.UpdateKubeConfig, nil)

	mockConnServices := mock_awsctl.NewMockServicesInterface(ctrl)
	mockConnServices.EXPECT().
		IsAWSConfigured().
		Return(false)

	mockEPrompter.EXPECT().
		PromptForManualCluster().
		Return("", "", "", "", errors.New("manual input error"))

	service := &eks.EKSService{
		EPrompter:    mockEPrompter,
		ConnServices: mockConnServices,
	}

	err := service.Run()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "kubeconfig update failed")
}

func TestEKSService_isAWSConfigured(t *testing.T) {
	t.Run("nil ConnServices", func(t *testing.T) {
		service := &eks.EKSService{}
		assert.False(t, service.IsAWSConfigured())
	})

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	t.Run("configured", func(t *testing.T) {
		mockConnServices := mock_awsctl.NewMockServicesInterface(ctrl)
		mockConnServices.EXPECT().
			IsAWSConfigured().
			Return(true)

		service := &eks.EKSService{
			ConnServices: mockConnServices,
		}
		assert.True(t, service.IsAWSConfigured())
	})

	t.Run("not configured", func(t *testing.T) {
		mockConnServices := mock_awsctl.NewMockServicesInterface(ctrl)
		mockConnServices.EXPECT().
			IsAWSConfigured().
			Return(false)

		service := &eks.EKSService{
			ConnServices: mockConnServices,
		}
		assert.False(t, service.IsAWSConfigured())
	})
}

func TestEKSService_getEKSClusterDetails_PromptForConfirmationFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	old := os.Stdout
	defer func() { os.Stdout = old }()
	os.Stdout = os.NewFile(0, os.DevNull)

	cluster := models.EKSCluster{ClusterName: "test-cluster"}

	mockEPrompter := mock_eks.NewMockEKSPromptInterface(ctrl)
	mockEPrompter.EXPECT().
		PromptForManualCluster().
		Return(cluster.ClusterName, "https://test.endpoint", "ca-data", "us-west-2", nil)

	mockConnServices := mock_awsctl.NewMockServicesInterface(ctrl)
	mockConnServices.EXPECT().
		IsAWSConfigured().
		Return(true)

	mockCPrompter := mock_awsctl.NewMockConnectionPrompter(ctrl)
	mockCPrompter.EXPECT().
		PromptForConfirmation("Look for EKS clusters in AWS?").
		AnyTimes().
		DoAndReturn(func(_ string) (bool, error) {
			return false, errors.New("confirmation error")
		})

	service := &eks.EKSService{
		EPrompter:        mockEPrompter,
		CPrompter:        mockCPrompter,
		ConnServices:     mockConnServices,
		ConfigLoader:     mock_eks.NewMockConfigLoader(ctrl),
		EKSClientFactory: mock_eks.NewMockEKSClientFactory(ctrl),
		FileSystem:       &common.RealFileSystem{},
	}

	clusterResult, profile, err := service.GetEKSClusterDetails()
	assert.NoError(t, err)
	assert.Equal(t, cluster.ClusterName, clusterResult.ClusterName)
	assert.Equal(t, "", profile)
}

func TestEKSService_getEKSClusterDetails_PromptForRegionError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	old := os.Stdout
	defer func() { os.Stdout = old }()
	os.Stdout = os.NewFile(0, os.DevNull)

	cluster := models.EKSCluster{ClusterName: "test-cluster"}

	mockEPrompter := mock_eks.NewMockEKSPromptInterface(ctrl)
	mockEPrompter.EXPECT().
		PromptForManualCluster().
		Return(cluster.ClusterName, "https://test.endpoint", "ca-data", "us-west-2", nil)

	mockConnServices := mock_awsctl.NewMockServicesInterface(ctrl)
	mockConnServices.EXPECT().
		IsAWSConfigured().
		Return(true)

	mockCPrompter := mock_awsctl.NewMockConnectionPrompter(ctrl)
	mockCPrompter.EXPECT().
		PromptForConfirmation("Look for EKS clusters in AWS?").
		Return(true, nil)
	mockCPrompter.EXPECT().
		PromptForRegion("").
		Return("", errors.New("region error"))

	service := &eks.EKSService{
		EPrompter:        mockEPrompter,
		CPrompter:        mockCPrompter,
		ConnServices:     mockConnServices,
		ConfigLoader:     mock_eks.NewMockConfigLoader(ctrl),
		EKSClientFactory: mock_eks.NewMockEKSClientFactory(ctrl),
		FileSystem:       &common.RealFileSystem{},
	}

	clusterResult, profile, err := service.GetEKSClusterDetails()
	assert.NoError(t, err)
	assert.Equal(t, cluster.ClusterName, clusterResult.ClusterName)
	assert.Equal(t, "", profile)
}

func TestEKSService_getEKSClusterDetails_LoadDefaultConfigError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	old := os.Stdout
	defer func() { os.Stdout = old }()
	os.Stdout = os.NewFile(0, os.DevNull)

	cluster := models.EKSCluster{ClusterName: "test-cluster"}
	testRegion := "us-west-2"
	testProfile := "default"

	mockEPrompter := mock_eks.NewMockEKSPromptInterface(ctrl)
	mockEPrompter.EXPECT().
		PromptForProfile().
		Return(testProfile, nil)
	mockEPrompter.EXPECT().
		PromptForManualCluster().
		Return(cluster.ClusterName, "https://test.endpoint", "ca-data", testRegion, nil)

	mockConnServices := mock_awsctl.NewMockServicesInterface(ctrl)
	mockConnServices.EXPECT().
		IsAWSConfigured().
		Return(true)

	mockCPrompter := mock_awsctl.NewMockConnectionPrompter(ctrl)
	mockCPrompter.EXPECT().
		PromptForConfirmation("Look for EKS clusters in AWS?").
		Return(true, nil)
	mockCPrompter.EXPECT().
		PromptForRegion("").
		Return(testRegion, nil)

	mockConfigLoader := mock_eks.NewMockConfigLoader(ctrl)
	mockConfigLoader.EXPECT().
		LoadDefaultConfig(
			context.TODO(),
			gomock.Any(),
			gomock.Any(),
		).
		Return(aws.Config{}, errors.New("config error"))

	service := &eks.EKSService{
		EPrompter:        mockEPrompter,
		CPrompter:        mockCPrompter,
		ConnServices:     mockConnServices,
		ConfigLoader:     mockConfigLoader,
		EKSClientFactory: mock_eks.NewMockEKSClientFactory(ctrl),
		FileSystem:       &common.RealFileSystem{},
	}

	clusterResult, profile, err := service.GetEKSClusterDetails()
	assert.NoError(t, err)
	assert.Equal(t, cluster.ClusterName, clusterResult.ClusterName)
	assert.Equal(t, "", profile)
}

func TestEKSService_getEKSClusterDetails_ListEKSClustersErrorOrEmpty(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	old := os.Stdout
	defer func() { os.Stdout = old }()
	os.Stdout = os.NewFile(0, os.DevNull)

	cluster := models.EKSCluster{ClusterName: "test-cluster"}
	testRegion := "us-west-2"
	testProfile := "default"

	mockEPrompter := mock_eks.NewMockEKSPromptInterface(ctrl)
	mockEPrompter.EXPECT().
		PromptForProfile().
		Return(testProfile, nil)
	mockEPrompter.EXPECT().
		PromptForManualCluster().
		Return(cluster.ClusterName, "https://test.endpoint", "ca-data", testRegion, nil)

	mockEKSClient := mock_eks.NewMockEKSAdapterInterface(ctrl)
	mockEKSClient.EXPECT().
		ListEKSClusters(context.TODO()).
		Return(nil, errors.New("list error"))

	mockConnServices := mock_awsctl.NewMockServicesInterface(ctrl)
	mockConnServices.EXPECT().
		IsAWSConfigured().
		Return(true)

	mockCPrompter := mock_awsctl.NewMockConnectionPrompter(ctrl)
	mockCPrompter.EXPECT().
		PromptForConfirmation("Look for EKS clusters in AWS?").
		Return(true, nil)
	mockCPrompter.EXPECT().
		PromptForRegion("").
		Return(testRegion, nil)

	mockConfigLoader := mock_eks.NewMockConfigLoader(ctrl)
	mockConfigLoader.EXPECT().
		LoadDefaultConfig(
			context.TODO(),
			gomock.Any(),
			gomock.Any(),
		).
		Return(aws.Config{}, nil)

	mockFactory := mock_eks.NewMockEKSClientFactory(ctrl)
	mockFactory.EXPECT().
		NewEKSClient(gomock.Any(), gomock.Any()).
		Return(mockEKSClient)

	service := &eks.EKSService{
		EPrompter:        mockEPrompter,
		CPrompter:        mockCPrompter,
		ConnServices:     mockConnServices,
		ConfigLoader:     mockConfigLoader,
		EKSClientFactory: mockFactory,
		FileSystem:       &common.RealFileSystem{},
	}

	clusterResult, profile, err := service.GetEKSClusterDetails()
	assert.NoError(t, err)
	assert.Equal(t, cluster.ClusterName, clusterResult.ClusterName)
	assert.Equal(t, "", profile)
}

func TestEKSService_getEKSClusterDetails_ClusterNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	old := os.Stdout
	defer func() { os.Stdout = old }()
	os.Stdout = os.NewFile(0, os.DevNull)

	cluster := models.EKSCluster{ClusterName: "test-cluster"}
	testRegion := "us-west-2"
	testProfile := "default"

	mockEPrompter := mock_eks.NewMockEKSPromptInterface(ctrl)
	mockEPrompter.EXPECT().
		PromptForProfile().
		Return(testProfile, nil)
	mockEPrompter.EXPECT().
		PromptForEKSCluster([]models.EKSCluster{cluster}).
		Return("non-existent-cluster", nil)

	mockEKSClient := mock_eks.NewMockEKSAdapterInterface(ctrl)
	mockEKSClient.EXPECT().
		ListEKSClusters(context.TODO()).
		Return([]models.EKSCluster{cluster}, nil)

	mockConnServices := mock_awsctl.NewMockServicesInterface(ctrl)
	mockConnServices.EXPECT().
		IsAWSConfigured().
		Return(true)

	mockCPrompter := mock_awsctl.NewMockConnectionPrompter(ctrl)
	mockCPrompter.EXPECT().
		PromptForConfirmation("Look for EKS clusters in AWS?").
		Return(true, nil)
	mockCPrompter.EXPECT().
		PromptForRegion("").
		Return(testRegion, nil)

	mockConfigLoader := mock_eks.NewMockConfigLoader(ctrl)
	mockConfigLoader.EXPECT().
		LoadDefaultConfig(
			context.TODO(),
			gomock.Any(),
			gomock.Any(),
		).
		Return(aws.Config{}, nil)

	mockFactory := mock_eks.NewMockEKSClientFactory(ctrl)
	mockFactory.EXPECT().
		NewEKSClient(gomock.Any(), gomock.Any()).
		Return(mockEKSClient)

	service := &eks.EKSService{
		EPrompter:        mockEPrompter,
		CPrompter:        mockCPrompter,
		ConnServices:     mockConnServices,
		ConfigLoader:     mockConfigLoader,
		EKSClientFactory: mockFactory,
		FileSystem:       &common.RealFileSystem{},
	}

	clusterResult, profile, err := service.GetEKSClusterDetails()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "selected cluster not found")
	assert.Nil(t, clusterResult)
	assert.Equal(t, "", profile)
}
