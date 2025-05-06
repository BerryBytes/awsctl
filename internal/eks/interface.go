package eks

import (
	"context"

	"github.com/BerryBytes/awsctl/models"
	"github.com/BerryBytes/awsctl/utils/common"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/eks"
)

type EKSServiceInterface interface {
	Run() error
}

type EKSAPI interface {
	ListClusters(ctx context.Context, input *eks.ListClustersInput, opts ...func(*eks.Options)) (*eks.ListClustersOutput, error)
	DescribeCluster(ctx context.Context, input *eks.DescribeClusterInput, opts ...func(*eks.Options)) (*eks.DescribeClusterOutput, error)
}

type EKSAdapterInterface interface {
	ListClusters(ctx context.Context, input *eks.ListClustersInput, opts ...func(*eks.Options)) (*eks.ListClustersOutput, error)
	DescribeCluster(ctx context.Context, input *eks.DescribeClusterInput, opts ...func(*eks.Options)) (*eks.DescribeClusterOutput, error)
	ListEKSClusters(ctx context.Context) ([]models.EKSCluster, error)
	GetClusterDetails(ctx context.Context, clusterName string) (*models.EKSCluster, error)
	UpdateKubeconfig(cluster *models.EKSCluster, profile string) error
}

type EKSPromptInterface interface {
	PromptForEKSCluster(clusters []models.EKSCluster) (string, error)
	PromptForProfile() (string, error)
	PromptForManualCluster() (clusterName, endpoint, caData, region string, err error)
	SelectEKSAction() (EKSAction, error)
	GetAWSConfig() (profile, region string, err error)
}

type ConfigLoader interface {
	LoadDefaultConfig(ctx context.Context, opts ...func(*config.LoadOptions) error) (aws.Config, error)
}

type EKSClientFactory interface {
	NewEKSClient(cfg aws.Config, fs common.FileSystemInterface) EKSAdapterInterface
}
