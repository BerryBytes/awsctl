package rds

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/BerryBytes/awsctl/models"
	"github.com/BerryBytes/awsctl/utils/common"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/smithy-go"
)

type RDSClientInterface = RDSAPI

type AwsRDSAdapter struct {
	Client   RDSAPI
	Cfg      aws.Config
	Executor common.CommandExecutor
}

func NewRDSClient(cfg aws.Config, cmdExecutor common.CommandExecutor) *AwsRDSAdapter {
	return &AwsRDSAdapter{
		Client:   rds.NewFromConfig(cfg),
		Cfg:      cfg,
		Executor: cmdExecutor,
	}
}

func (c *AwsRDSAdapter) DescribeDBInstances(ctx context.Context, input *rds.DescribeDBInstancesInput, opts ...func(*rds.Options)) (*rds.DescribeDBInstancesOutput, error) {
	return c.Client.DescribeDBInstances(ctx, input, opts...)
}

func (c *AwsRDSAdapter) DescribeDBClusters(ctx context.Context, input *rds.DescribeDBClustersInput, opts ...func(*rds.Options)) (*rds.DescribeDBClustersOutput, error) {
	return c.Client.DescribeDBClusters(ctx, input, opts...)
}

func (c *AwsRDSAdapter) ListRDSResources(ctx context.Context) ([]models.RDSInstance, error) {
	var resources []models.RDSInstance

	clusters, err := c.DescribeDBClusters(ctx, &rds.DescribeDBClustersInput{})
	if err == nil {
		for _, cluster := range clusters.DBClusters {
			if cluster.Endpoint != nil {
				resources = append(resources, models.RDSInstance{
					DBInstanceIdentifier: aws.ToString(cluster.DBClusterIdentifier),
					Engine:               aws.ToString(cluster.Engine),
					Endpoint:             aws.ToString(cluster.Endpoint),
				})
			}
		}
	}

	instances, err := c.DescribeDBInstances(ctx, &rds.DescribeDBInstancesInput{})
	if err == nil {
		for _, instance := range instances.DBInstances {
			if strings.Contains(strings.ToLower(aws.ToString(instance.Engine)), "aurora") &&
				instance.DBClusterIdentifier != nil {
				continue
			}

			if instance.Endpoint != nil && instance.Endpoint.Address != nil {
				resources = append(resources, models.RDSInstance{
					DBInstanceIdentifier: aws.ToString(instance.DBInstanceIdentifier),
					Engine:               aws.ToString(instance.Engine),
					Endpoint:             aws.ToString(instance.Endpoint.Address),
				})
			}
		}
	}

	sort.Slice(resources, func(i, j int) bool {
		return resources[i].DBInstanceIdentifier < resources[j].DBInstanceIdentifier
	})

	return resources, nil
}

func (c *AwsRDSAdapter) GetConnectionEndpoint(ctx context.Context, identifier string) (string, error) {
	cluster, err := c.DescribeDBClusters(ctx, &rds.DescribeDBClustersInput{
		DBClusterIdentifier: aws.String(identifier),
	})
	if err == nil && len(cluster.DBClusters) > 0 {
		cl := cluster.DBClusters[0]
		port := cl.Port
		if port == nil {
			port = aws.Int32(3306)
		}
		return fmt.Sprintf("%s:%d", aws.ToString(cl.Endpoint), *port), nil
	}

	instance, err := c.DescribeDBInstances(ctx, &rds.DescribeDBInstancesInput{
		DBInstanceIdentifier: aws.String(identifier),
	})
	if err != nil {
		return "", c.HandleAWSError(err, "getting RDS endpoint")
	}

	if len(instance.DBInstances) == 0 {
		return "", fmt.Errorf("no RDS resource found with identifier: %s", identifier)
	}

	inst := instance.DBInstances[0]
	port := inst.Endpoint.Port
	if port == nil {
		port = aws.Int32(GetDefaultPort(aws.ToString(inst.Engine)))
	}

	return fmt.Sprintf("%s:%d", aws.ToString(inst.Endpoint.Address), *port), nil
}

func (c *AwsRDSAdapter) GenerateAuthToken(endpoint, dbUser, region string) (string, error) {
	parts := strings.Split(endpoint, ":")
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid RDS endpoint format: %s", endpoint)
	}
	remoteHost := parts[0]
	remotePortStr := parts[1]

	remotePort, err := strconv.Atoi(remotePortStr)
	if err != nil {
		return "", fmt.Errorf("invalid port in RDS endpoint: %w", err)
	}

	portStr := strconv.Itoa(remotePort)

	output, err := c.Executor.RunCommand("aws", "rds", "generate-db-auth-token",
		"--hostname", remoteHost,
		"--port", portStr,
		"--region", region,
		"--username", dbUser,
	)
	if err != nil {
		return "", fmt.Errorf("failed to generate auth token: %w", err)
	}

	return string(output), nil
}

func GetDefaultPort(engine string) int32 {
	switch {
	case strings.Contains(engine, "postgres"):
		return 5432
	case strings.Contains(engine, "mysql"):
		return 3306
	case strings.Contains(engine, "mariadb"):
		return 3306
	case strings.Contains(engine, "sqlserver"):
		return 1433
	case strings.Contains(engine, "oracle"):
		return 1521
	default:
		return 3306
	}
}

func (c *AwsRDSAdapter) HandleAWSError(err error, operation string) error {
	var apiErr *smithy.GenericAPIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "RequestExpired":
			return fmt.Errorf("AWS request expired during %s: %w", operation, err)
		case "AuthFailure", "UnauthorizedOperation":
			return fmt.Errorf("AWS authentication failed during %s: %w", operation, err)
		case "OptInRequired":
			return fmt.Errorf("AWS region is not enabled during %s: %w", operation, err)
		case "DBInstanceNotFound", "DBClusterNotFoundFault":
			return fmt.Errorf("RDS resource not found during %s: %w", operation, err)
		}
	}
	return fmt.Errorf("failed during %s: %w", operation, err)
}
