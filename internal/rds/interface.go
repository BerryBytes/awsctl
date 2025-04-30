package rds

import (
	"context"

	"github.com/BerryBytes/awsctl/internal/sso"
	"github.com/BerryBytes/awsctl/models"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/rds"
)

type RDSServiceInterface interface {
	Run() error
}

type RDSAPI interface {
	DescribeDBInstances(ctx context.Context, input *rds.DescribeDBInstancesInput, opts ...func(*rds.Options)) (*rds.DescribeDBInstancesOutput, error)
	DescribeDBClusters(ctx context.Context, input *rds.DescribeDBClustersInput, opts ...func(*rds.Options)) (*rds.DescribeDBClustersOutput, error)
}

type RDSAdapterInterface interface {
	DescribeDBInstances(ctx context.Context, input *rds.DescribeDBInstancesInput, opts ...func(*rds.Options)) (*rds.DescribeDBInstancesOutput, error)
	DescribeDBClusters(ctx context.Context, input *rds.DescribeDBClustersInput, opts ...func(*rds.Options)) (*rds.DescribeDBClustersOutput, error)
	ListRDSResources(ctx context.Context) ([]models.RDSInstance, error)
	GetConnectionEndpoint(ctx context.Context, identifier string) (string, error)
	GenerateAuthToken(endpoint, dbUser, region string) (string, error)
}

type RDSPromptInterface interface {
	PromptForRDSInstance(instances []models.RDSInstance) (string, error)
	PromptForProfile() (string, error)
	PromptForManualEndpoint() (endpoint, dbUser, region string, err error)
	SelectRDSAction() (RDSAction, error)
	PromptForDBUser() (string, error)
	GetAWSConfig() (profile, region string, err error)
}

type ConfigLoader interface {
	LoadDefaultConfig(ctx context.Context, opts ...func(*config.LoadOptions) error) (aws.Config, error)
}

type RDSClientFactory interface {
	NewRDSClient(cfg aws.Config, executor sso.CommandExecutor) RDSAdapterInterface
}
