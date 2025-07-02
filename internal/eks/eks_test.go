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

func TestEKSService_Run_Success(t *testing.T) {
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
		PromptForProfile().
		Return(testProfile, nil)
	mockEPrompter.EXPECT().
		PromptForEKSCluster([]models.EKSCluster{cluster}).
		Return(cluster.ClusterName, nil)

	mockEKSClient := mock_eks.NewMockEKSAdapterInterface(ctrl)
	mockEKSClient.EXPECT().
		ListEKSClusters(gomock.Any()).
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
		PromptForRegion("").
		Return(testRegion, nil)

	mockConfigLoader := mock_eks.NewMockConfigLoader(ctrl)
	mockConfigLoader.EXPECT().
		LoadDefaultConfig(gomock.Any(), gomock.Any()).
		Return(aws.Config{
			Region: testRegion,
			Credentials: aws.CredentialsProviderFunc(func(ctx context.Context) (aws.Credentials, error) {
				return aws.Credentials{AccessKeyID: "test"}, nil
			}),
		}, nil)

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
		EKSClient:        nil,
		FileSystem:       &common.RealFileSystem{},
	}

	err := service.Run()
	assert.NoError(t, err)
}

func TestEKSService_Run_AWSNotConfigured(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	old := os.Stdout
	defer func() { os.Stdout = old }()
	os.Stdout = os.NewFile(0, os.DevNull)

	clusterName := "test-cluster"
	endpoint := "https://test.endpoint"
	region := "us-west-2"
	caData := "ca-data"

	expectedCluster := &models.EKSCluster{
		ClusterName:              clusterName,
		Endpoint:                 endpoint,
		Region:                   region,
		CertificateAuthorityData: caData,
	}

	mockEPrompter := mock_eks.NewMockEKSPromptInterface(ctrl)
	mockEPrompter.EXPECT().
		PromptForManualCluster().
		Return(clusterName, endpoint, caData, region, nil)

	mockEKSClient := mock_eks.NewMockEKSAdapterInterface(ctrl)
	mockEKSClient.EXPECT().
		UpdateKubeconfig(expectedCluster, "").
		Return(nil)

	mockConnServices := mock_awsctl.NewMockServicesInterface(ctrl)
	mockConnServices.EXPECT().
		IsAWSConfigured().
		Return(false)

	service := &eks.EKSService{
		EPrompter:    mockEPrompter,
		ConnServices: mockConnServices,
		EKSClient:    mockEKSClient,
		FileSystem:   &common.RealFileSystem{},
	}

	err := service.Run()
	assert.NoError(t, err)
}

func TestEKSService_Run_ManualClusterError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEPrompter := mock_eks.NewMockEKSPromptInterface(ctrl)
	mockEPrompter.EXPECT().
		PromptForManualCluster().
		Return("", "", "", "", errors.New("kubeconfig update failed"))

	mockConnServices := mock_awsctl.NewMockServicesInterface(ctrl)
	mockConnServices.EXPECT().
		IsAWSConfigured().
		Return(false)

	service := &eks.EKSService{
		EPrompter:    mockEPrompter,
		ConnServices: mockConnServices,
		FileSystem:   &common.RealFileSystem{},
	}

	err := service.Run()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "kubeconfig update failed")
}

func TestEKSService_GetEKSClusterDetails_AWSConfigured(t *testing.T) {
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
		ListEKSClusters(gomock.Any()).
		Return([]models.EKSCluster{cluster}, nil)

	mockConnServices := mock_awsctl.NewMockServicesInterface(ctrl)
	mockConnServices.EXPECT().
		IsAWSConfigured().
		Return(true)

	mockCPrompter := mock_awsctl.NewMockConnectionPrompter(ctrl)
	mockCPrompter.EXPECT().
		PromptForRegion("").
		Return(testRegion, nil)

	mockConfigLoader := mock_eks.NewMockConfigLoader(ctrl)
	mockConfigLoader.EXPECT().
		LoadDefaultConfig(gomock.Any(), gomock.Any()).
		Return(aws.Config{
			Region: testRegion,
			Credentials: aws.CredentialsProviderFunc(func(ctx context.Context) (aws.Credentials, error) {
				return aws.Credentials{AccessKeyID: "test"}, nil
			}),
		}, nil)

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

func TestEKSService_GetEKSClusterDetails_ManualFallback(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cluster := models.EKSCluster{ClusterName: "test-cluster"}
	testRegion := "us-west-2"

	mockEPrompter := mock_eks.NewMockEKSPromptInterface(ctrl)
	mockEPrompter.EXPECT().
		PromptForManualCluster().
		Return(cluster.ClusterName, "https://test.endpoint", "ca-data", testRegion, nil)

	mockConnServices := mock_awsctl.NewMockServicesInterface(ctrl)
	mockConnServices.EXPECT().
		IsAWSConfigured().
		Return(false)

	service := &eks.EKSService{
		EPrompter:    mockEPrompter,
		ConnServices: mockConnServices,
		FileSystem:   &common.RealFileSystem{},
	}

	resultCluster, profile, err := service.GetEKSClusterDetails()
	assert.NoError(t, err)
	assert.Equal(t, cluster.ClusterName, resultCluster.ClusterName)
	assert.Equal(t, "", profile)
}

func TestEKSService_GetEKSClusterDetails_RegionPromptError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cluster := models.EKSCluster{ClusterName: "test-cluster"}
	testRegion := "us-west-2"

	mockEPrompter := mock_eks.NewMockEKSPromptInterface(ctrl)
	mockEPrompter.EXPECT().
		PromptForManualCluster().
		Return(cluster.ClusterName, "https://test.endpoint", "ca-data", testRegion, nil)

	mockConnServices := mock_awsctl.NewMockServicesInterface(ctrl)
	mockConnServices.EXPECT().
		IsAWSConfigured().
		Return(true)

	mockCPrompter := mock_awsctl.NewMockConnectionPrompter(ctrl)
	mockCPrompter.EXPECT().
		PromptForRegion("").
		Return("", errors.New("region error"))

	service := &eks.EKSService{
		EPrompter:    mockEPrompter,
		CPrompter:    mockCPrompter,
		ConnServices: mockConnServices,
		FileSystem:   &common.RealFileSystem{},
	}

	resultCluster, profile, err := service.GetEKSClusterDetails()
	assert.NoError(t, err)
	assert.Equal(t, cluster.ClusterName, resultCluster.ClusterName)
	assert.Equal(t, "", profile)
}

func TestEKSService_GetEKSClusterDetails_ConfigLoadError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

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
		PromptForRegion("").
		Return(testRegion, nil)

	mockConfigLoader := mock_eks.NewMockConfigLoader(ctrl)
	mockConfigLoader.EXPECT().
		LoadDefaultConfig(gomock.Any(), gomock.Any()).
		Return(aws.Config{}, errors.New("config error"))

	service := &eks.EKSService{
		EPrompter:    mockEPrompter,
		CPrompter:    mockCPrompter,
		ConnServices: mockConnServices,
		ConfigLoader: mockConfigLoader,
		FileSystem:   &common.RealFileSystem{},
	}

	resultCluster, profile, err := service.GetEKSClusterDetails()
	assert.NoError(t, err)
	assert.Equal(t, cluster.ClusterName, resultCluster.ClusterName)
	assert.Equal(t, "", profile)
}

func TestEKSService_HandleManualCluster_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cluster := models.EKSCluster{
		ClusterName: "test-cluster",
		Endpoint:    "https://test.endpoint",
		Region:      "us-west-2",
	}

	mockEPrompter := mock_eks.NewMockEKSPromptInterface(ctrl)
	mockEPrompter.EXPECT().
		PromptForManualCluster().
		Return(cluster.ClusterName, cluster.Endpoint, "ca-data", cluster.Region, nil)

	service := &eks.EKSService{
		EPrompter: mockEPrompter,
	}

	resultCluster, profile, err := service.HandleManualCluster()
	assert.NoError(t, err)
	assert.Equal(t, cluster.ClusterName, resultCluster.ClusterName)
	assert.Equal(t, cluster.Endpoint, resultCluster.Endpoint)
	assert.Equal(t, cluster.Region, resultCluster.Region)
	assert.Equal(t, "", profile)
}

func TestEKSService_HandleManualCluster_Error(t *testing.T) {
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

func TestEKSService_IsAWSConfigured(t *testing.T) {
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

func TestEKSService_GetEKSClusterDetails_ProfileError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	old := os.Stdout
	defer func() { os.Stdout = old }()
	os.Stdout = os.NewFile(0, os.DevNull)

	testRegion := "us-west-2"
	cluster := models.EKSCluster{ClusterName: "test-cluster"}

	mockEPrompter := mock_eks.NewMockEKSPromptInterface(ctrl)
	mockEPrompter.EXPECT().
		PromptForProfile().
		Return("", errors.New("profile error"))
	mockEPrompter.EXPECT().
		PromptForManualCluster().
		Return(cluster.ClusterName, "https://test.endpoint", "ca-data", testRegion, nil)

	mockConnServices := mock_awsctl.NewMockServicesInterface(ctrl)
	mockConnServices.EXPECT().
		IsAWSConfigured().
		Return(true)

	mockCPrompter := mock_awsctl.NewMockConnectionPrompter(ctrl)
	mockCPrompter.EXPECT().
		PromptForRegion("").
		Return(testRegion, nil)

	service := &eks.EKSService{
		EPrompter:    mockEPrompter,
		CPrompter:    mockCPrompter,
		ConnServices: mockConnServices,
		FileSystem:   &common.RealFileSystem{},
	}

	_, _, err := service.GetEKSClusterDetails()
	assert.NoError(t, err)
}

func TestEKSService_GetEKSClusterDetails_CredentialsError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	old := os.Stdout
	defer func() { os.Stdout = old }()
	os.Stdout = os.NewFile(0, os.DevNull)

	testProfile := "default"
	testRegion := "us-west-2"
	cluster := models.EKSCluster{ClusterName: "test-cluster"}

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
		PromptForRegion("").
		Return(testRegion, nil)

	mockConfigLoader := mock_eks.NewMockConfigLoader(ctrl)
	mockConfigLoader.EXPECT().
		LoadDefaultConfig(gomock.Any(), gomock.Any()).
		Return(aws.Config{
			Region: testRegion,
			Credentials: aws.CredentialsProviderFunc(func(ctx context.Context) (aws.Credentials, error) {
				return aws.Credentials{}, errors.New("SSO token expired")
			}),
		}, nil)

	service := &eks.EKSService{
		EPrompter:    mockEPrompter,
		CPrompter:    mockCPrompter,
		ConnServices: mockConnServices,
		ConfigLoader: mockConfigLoader,
		FileSystem:   &common.RealFileSystem{},
	}

	_, _, err := service.GetEKSClusterDetails()
	assert.NoError(t, err)
}

func TestEKSService_GetEKSClusterDetails_NoClustersFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	old := os.Stdout
	defer func() { os.Stdout = old }()
	os.Stdout = os.NewFile(0, os.DevNull)

	testProfile := "default"
	testRegion := "us-west-2"
	cluster := models.EKSCluster{ClusterName: "test-cluster"}

	mockEPrompter := mock_eks.NewMockEKSPromptInterface(ctrl)
	mockEPrompter.EXPECT().
		PromptForProfile().
		Return(testProfile, nil)
	mockEPrompter.EXPECT().
		PromptForManualCluster().
		Return(cluster.ClusterName, "https://test.endpoint", "ca-data", testRegion, nil)

	mockEKSClient := mock_eks.NewMockEKSAdapterInterface(ctrl)
	mockEKSClient.EXPECT().
		ListEKSClusters(gomock.Any()).
		Return([]models.EKSCluster{}, nil)

	mockConnServices := mock_awsctl.NewMockServicesInterface(ctrl)
	mockConnServices.EXPECT().
		IsAWSConfigured().
		Return(true)

	mockCPrompter := mock_awsctl.NewMockConnectionPrompter(ctrl)
	mockCPrompter.EXPECT().
		PromptForRegion("").
		Return(testRegion, nil)

	mockConfigLoader := mock_eks.NewMockConfigLoader(ctrl)
	mockConfigLoader.EXPECT().
		LoadDefaultConfig(gomock.Any(), gomock.Any()).
		Return(aws.Config{
			Region: testRegion,
			Credentials: aws.CredentialsProviderFunc(func(ctx context.Context) (aws.Credentials, error) {
				return aws.Credentials{AccessKeyID: "test"}, nil
			}),
		}, nil)

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

	_, _, err := service.GetEKSClusterDetails()
	assert.NoError(t, err)
}
