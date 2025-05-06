package eks_test

import (
	"context"
	"errors"
	"os"
	"testing"

	internalEKS "github.com/BerryBytes/awsctl/internal/eks"
	"github.com/BerryBytes/awsctl/models"
	mock_awsctl "github.com/BerryBytes/awsctl/tests/mock"
	mock_eks "github.com/BerryBytes/awsctl/tests/mock/eks"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	"github.com/aws/smithy-go"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

func TestListEKSClusters_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEKS := mock_eks.NewMockEKSAPI(ctrl)
	mockFS := mock_awsctl.NewMockFileSystemInterface(ctrl)
	adapter := &internalEKS.AwsEKSAdapter{
		Client:     mockEKS,
		Cfg:        aws.Config{Region: "us-west-2"},
		FileSystem: mockFS,
	}

	mockEKS.EXPECT().ListClusters(gomock.Any(), gomock.Any()).Return(&eks.ListClustersOutput{
		Clusters: []string{"test-cluster"},
	}, nil)

	mockEKS.EXPECT().DescribeCluster(gomock.Any(), gomock.Any()).Return(&eks.DescribeClusterOutput{
		Cluster: &ekstypes.Cluster{
			Endpoint: aws.String("https://example.com"),
			CertificateAuthority: &ekstypes.Certificate{
				Data: aws.String("cert-data"),
			},
		},
	}, nil)

	clusters, err := adapter.ListEKSClusters(context.TODO())
	assert.NoError(t, err)
	assert.Len(t, clusters, 1)
	assert.Equal(t, "test-cluster", clusters[0].ClusterName)
}

func TestGetClusterDetails_InvalidCluster(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEKS := mock_eks.NewMockEKSAPI(ctrl)
	adapter := &internalEKS.AwsEKSAdapter{Client: mockEKS}

	mockEKS.EXPECT().DescribeCluster(gomock.Any(), gomock.Any()).Return(&eks.DescribeClusterOutput{}, nil)

	_, err := adapter.GetClusterDetails(context.TODO(), "bad-cluster")
	assert.Error(t, err)
}

func TestUpdateKubeconfig_NewFile(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockFS := mock_awsctl.NewMockFileSystemInterface(ctrl)
	adapter := &internalEKS.AwsEKSAdapter{
		FileSystem: mockFS,
	}

	cluster := &models.EKSCluster{
		ClusterName:              "my-cluster",
		Endpoint:                 "https://api",
		Region:                   "us-west-2",
		CertificateAuthorityData: "cert-data",
	}

	mockFS.EXPECT().UserHomeDir().Return("/home/test", nil)
	mockFS.EXPECT().Stat("/home/test/.kube/config").Return(nil, errors.New("not found"))
	mockFS.EXPECT().MkdirAll("/home/test/.kube", gomock.Any()).Return(nil)
	mockFS.EXPECT().WriteFile("/home/test/.kube/config", gomock.Any(), gomock.Any()).Return(nil)

	err := adapter.UpdateKubeconfig(cluster, "test-profile")
	assert.NoError(t, err)
}

func TestHandleAWSError_ResourceNotFound(t *testing.T) {
	adapter := &internalEKS.AwsEKSAdapter{}

	err := adapter.HandleAWSError(&smithy.GenericAPIError{
		Code:    "ResourceNotFoundException",
		Message: "mock error",
	}, "operation")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "EKS resource not found")
}

func TestListEKSClusters_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEKS := mock_eks.NewMockEKSAPI(ctrl)
	adapter := &internalEKS.AwsEKSAdapter{Client: mockEKS}

	mockEKS.EXPECT().ListClusters(gomock.Any(), gomock.Any()).Return(nil, errors.New("API error"))

	_, err := adapter.ListEKSClusters(context.TODO())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "listing EKS clusters")
}

func TestListEKSClusters_MultiplePages(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEKS := mock_eks.NewMockEKSAPI(ctrl)
	mockFS := mock_awsctl.NewMockFileSystemInterface(ctrl)
	adapter := &internalEKS.AwsEKSAdapter{
		Client:     mockEKS,
		FileSystem: mockFS,
	}

	mockEKS.EXPECT().ListClusters(gomock.Any(), gomock.Any()).Return(&eks.ListClustersOutput{
		Clusters:  []string{"b-cluster"},
		NextToken: aws.String("token"),
	}, nil)

	mockEKS.EXPECT().ListClusters(gomock.Any(), gomock.Any()).Return(&eks.ListClustersOutput{
		Clusters: []string{"a-cluster"},
	}, nil)

	mockEKS.EXPECT().DescribeCluster(gomock.Any(), gomock.Any()).Times(2).Return(&eks.DescribeClusterOutput{
		Cluster: &ekstypes.Cluster{
			Endpoint: aws.String("https://example.com"),
			CertificateAuthority: &ekstypes.Certificate{
				Data: aws.String("cert"),
			},
		},
	}, nil)

	clusters, err := adapter.ListEKSClusters(context.TODO())
	assert.NoError(t, err)
	assert.Len(t, clusters, 2)
	assert.Equal(t, "a-cluster", clusters[0].ClusterName)
}

func TestGetClusterDetails_DescribeError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEKS := mock_eks.NewMockEKSAPI(ctrl)
	adapter := &internalEKS.AwsEKSAdapter{Client: mockEKS}

	mockEKS.EXPECT().DescribeCluster(gomock.Any(), gomock.Any()).
		Return(nil, &smithy.GenericAPIError{
			Code:    "UnauthorizedOperation",
			Message: "not authorized",
		})

	_, err := adapter.GetClusterDetails(context.TODO(), "test")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "authentication failed")
}

func TestUpdateKubeconfig_HomeDirError(t *testing.T) {
	mockFS := mock_awsctl.NewMockFileSystemInterface(gomock.NewController(t))
	adapter := &internalEKS.AwsEKSAdapter{FileSystem: mockFS}

	mockFS.EXPECT().UserHomeDir().Return("", errors.New("home dir fail"))

	err := adapter.UpdateKubeconfig(&models.EKSCluster{}, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "home directory")
}

func TestUpdateKubeconfig_FileExists_UnmarshalFail(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockFS := mock_awsctl.NewMockFileSystemInterface(ctrl)
	adapter := &internalEKS.AwsEKSAdapter{FileSystem: mockFS}

	mockFS.EXPECT().UserHomeDir().Return("/home/test", nil)
	mockFS.EXPECT().Stat("/home/test/.kube/config").Return(nil, nil)
	mockFS.EXPECT().ReadFile("/home/test/.kube/config").Return([]byte("bad yaml"), nil)

	err := adapter.UpdateKubeconfig(&models.EKSCluster{
		ClusterName:              "my-cluster",
		Endpoint:                 "https://api",
		Region:                   "us-west-2",
		CertificateAuthorityData: "cert-data",
	}, "profile")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse kubeconfig")
}

func TestHandleAWSError_RequestExpired(t *testing.T) {
	adapter := &internalEKS.AwsEKSAdapter{}
	err := adapter.HandleAWSError(&smithy.GenericAPIError{
		Code:    "RequestExpired",
		Message: "expired",
	}, "operation")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "request expired")
}

func TestHandleAWSError_AuthFailure(t *testing.T) {
	adapter := &internalEKS.AwsEKSAdapter{}
	err := adapter.HandleAWSError(&smithy.GenericAPIError{
		Code:    "AuthFailure",
		Message: "denied",
	}, "operation")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "authentication failed")
}

func TestHandleAWSError_Fallback(t *testing.T) {
	adapter := &internalEKS.AwsEKSAdapter{}
	err := adapter.HandleAWSError(errors.New("unknown error"), "some-op")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed during some-op")
}

func TestUpdateKubeconfig_UpdateExistingEntries(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockFS := mock_awsctl.NewMockFileSystemInterface(ctrl)
	adapter := &internalEKS.AwsEKSAdapter{
		FileSystem: mockFS,
	}

	cluster := &models.EKSCluster{
		ClusterName:              "existing-cluster",
		Endpoint:                 "https://new-endpoint",
		Region:                   "us-west-2",
		CertificateAuthorityData: "new-cert",
	}

	existingKubeconfig := internalEKS.Kubeconfig{
		Clusters: []internalEKS.ClusterEntry{
			{
				Name: "existing-cluster",
				Cluster: internalEKS.ClusterData{
					Server:                   "https://old-endpoint",
					CertificateAuthorityData: "old-cert",
				},
			},
		},
		Users: []internalEKS.UserEntry{
			{
				Name: "existing-cluster-user",
				User: internalEKS.UserData{
					Exec: internalEKS.ExecConfig{
						APIVersion: "client.authentication.k8s.io/v1beta1",
						Command:    "aws",
						Args:       []string{"--region", "us-west-2", "eks", "get-token", "--cluster-name", "existing-cluster"},
					},
				},
			},
		},
		Contexts: []internalEKS.ContextEntry{
			{
				Name: "existing-cluster",
				Context: internalEKS.ContextData{
					Cluster: "existing-cluster",
					User:    "existing-cluster-user",
				},
			},
		},
		CurrentContext: "existing-cluster",
	}
	yamlData, _ := yaml.Marshal(existingKubeconfig)

	var updatedConfig []byte
	mockFS.EXPECT().UserHomeDir().Return("/home/test", nil)
	mockFS.EXPECT().Stat("/home/test/.kube/config").Return(nil, nil)
	mockFS.EXPECT().ReadFile("/home/test/.kube/config").Return(yamlData, nil)
	mockFS.EXPECT().MkdirAll("/home/test/.kube", gomock.Any()).Return(nil)
	mockFS.EXPECT().WriteFile("/home/test/.kube/config", gomock.Any(), gomock.Any()).
		DoAndReturn(func(path string, data []byte, perm os.FileMode) error {
			updatedConfig = data
			return nil
		})

	err := adapter.UpdateKubeconfig(cluster, "aws")
	assert.NoError(t, err)

	var updatedKubeconfig internalEKS.Kubeconfig
	err = yaml.Unmarshal(updatedConfig, &updatedKubeconfig)
	assert.NoError(t, err)

	assert.Len(t, updatedKubeconfig.Clusters, 1)
	assert.Equal(t, "existing-cluster", updatedKubeconfig.Clusters[0].Name)
	assert.Equal(t, "https://new-endpoint", updatedKubeconfig.Clusters[0].Cluster.Server)
	assert.Equal(t, "new-cert", updatedKubeconfig.Clusters[0].Cluster.CertificateAuthorityData)

	assert.Len(t, updatedKubeconfig.Users, 1)
	assert.Equal(t, "existing-cluster-user", updatedKubeconfig.Users[0].Name)
	assert.Equal(t, "client.authentication.k8s.io/v1beta1", updatedKubeconfig.Users[0].User.Exec.APIVersion)

	assert.Len(t, updatedKubeconfig.Contexts, 1)
	assert.Equal(t, "existing-cluster", updatedKubeconfig.Contexts[0].Name)
	assert.Equal(t, "existing-cluster", updatedKubeconfig.Contexts[0].Context.Cluster)
	assert.Equal(t, "existing-cluster-user", updatedKubeconfig.Contexts[0].Context.User)
}

func TestUpdateKubeconfig_MkdirAllError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockFS := mock_awsctl.NewMockFileSystemInterface(ctrl)
	adapter := &internalEKS.AwsEKSAdapter{
		FileSystem: mockFS,
	}

	cluster := &models.EKSCluster{
		ClusterName:              "my-cluster",
		Endpoint:                 "https://api",
		Region:                   "us-west-2",
		CertificateAuthorityData: "cert-data",
	}

	mockFS.EXPECT().UserHomeDir().Return("/home/test", nil)
	mockFS.EXPECT().Stat("/home/test/.kube/config").Return(nil, errors.New("not found"))
	mockFS.EXPECT().MkdirAll("/home/test/.kube", gomock.Any()).Return(errors.New("mkdir error"))

	err := adapter.UpdateKubeconfig(cluster, "test-profile")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create kubeconfig directory")
}

func TestUpdateKubeconfig_WriteFileError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockFS := mock_awsctl.NewMockFileSystemInterface(ctrl)
	adapter := &internalEKS.AwsEKSAdapter{
		FileSystem: mockFS,
	}

	cluster := &models.EKSCluster{
		ClusterName:              "my-cluster",
		Endpoint:                 "https://api",
		Region:                   "us-west-2",
		CertificateAuthorityData: "cert-data",
	}

	mockFS.EXPECT().UserHomeDir().Return("/home/test", nil)
	mockFS.EXPECT().Stat("/home/test/.kube/config").Return(nil, errors.New("not found"))
	mockFS.EXPECT().MkdirAll("/home/test/.kube", gomock.Any()).Return(nil)
	mockFS.EXPECT().WriteFile("/home/test/.kube/config", gomock.Any(), gomock.Any()).Return(errors.New("write error"))

	err := adapter.UpdateKubeconfig(cluster, "test-profile")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to write kubeconfig")
}
