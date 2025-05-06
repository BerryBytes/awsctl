package rds_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/BerryBytes/awsctl/internal/rds"
	mock_awsctl "github.com/BerryBytes/awsctl/tests/mock"
	mock_rds "github.com/BerryBytes/awsctl/tests/mock/rds"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsrds "github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/aws/smithy-go"
	"github.com/golang/mock/gomock"
)

func TestGenerateAuthToken(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	adapter := &rds.AwsRDSAdapter{
		Client:   nil,
		Executor: nil,
	}

	t.Run("Successful auth token generation", func(t *testing.T) {
		mockExecutor := mock_awsctl.NewMockCommandExecutor(ctrl)
		mockExecutor.EXPECT().RunCommand(
			"aws", "rds", "generate-db-auth-token",
			"--hostname", "rds.endpoint",
			"--port", "3306",
			"--region", "us-west-2",
			"--username", "dbuser",
		).Return([]byte("auth-token"), nil)

		adapter.Executor = mockExecutor
		token, err := adapter.GenerateAuthToken("rds.endpoint:3306", "dbuser", "us-west-2")
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if token != "auth-token" {
			t.Errorf("Expected token auth-token, got %s", token)
		}
	})

	t.Run("Invalid endpoint format", func(t *testing.T) {
		adapter.Executor = nil
		_, err := adapter.GenerateAuthToken("invalid-endpoint", "dbuser", "us-west-2")
		if err == nil || !strings.Contains(err.Error(), "invalid RDS endpoint format") {
			t.Errorf("Expected invalid endpoint format error, got %v", err)
		}
	})

	t.Run("Invalid port", func(t *testing.T) {
		adapter.Executor = nil
		_, err := adapter.GenerateAuthToken("rds.endpoint:invalid", "dbuser", "us-west-2")
		if err == nil || !strings.Contains(err.Error(), "invalid port") {
			t.Errorf("Expected invalid port error, got %v", err)
		}
	})
}

func TestHandleAWSError(t *testing.T) {
	adapter := &rds.AwsRDSAdapter{}

	tests := []struct {
		name        string
		err         error
		operation   string
		expectedErr string
	}{
		{
			name:        "RequestExpired",
			err:         &smithy.GenericAPIError{Code: "RequestExpired", Message: "expired"},
			operation:   "test-op",
			expectedErr: "AWS request expired during test-op",
		},
		{
			name:        "AuthFailure",
			err:         &smithy.GenericAPIError{Code: "AuthFailure", Message: "unauthorized"},
			operation:   "test-op",
			expectedErr: "AWS authentication failed during test-op",
		},
		{
			name:        "DBInstanceNotFound",
			err:         &smithy.GenericAPIError{Code: "DBInstanceNotFound", Message: "not found"},
			operation:   "test-op",
			expectedErr: "RDS resource not found during test-op",
		},
		{
			name:        "Generic error",
			err:         errors.New("generic error"),
			operation:   "test-op",
			expectedErr: "failed during test-op: generic error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := adapter.HandleAWSError(tt.err, tt.operation)
			if !strings.Contains(err.Error(), tt.expectedErr) {
				t.Errorf("Expected error containing %q, got %v", tt.expectedErr, err)
			}
		})
	}
}

func TestGetDefaultPort(t *testing.T) {
	tests := []struct {
		engine   string
		expected int32
	}{
		{"postgres", 5432},
		{"mysql", 3306},
		{"mariadb", 3306},
		{"sqlserver", 1433},
		{"oracle", 1521},
		{"unknown", 3306},
	}

	for _, tt := range tests {
		t.Run(tt.engine, func(t *testing.T) {
			port := rds.GetDefaultPort(tt.engine)
			if port != tt.expected {
				t.Errorf("Expected port %d for engine %s, got %d", tt.expected, tt.engine, port)
			}
		})
	}
}

func TestNewRDSClient(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := aws.Config{}
	mockExecutor := mock_awsctl.NewMockCommandExecutor(ctrl)

	adapter := rds.NewRDSClient(cfg, mockExecutor)

	if adapter.Client == nil {
		t.Error("Expected non-nil RDS client")
	}

	if adapter.Executor != mockExecutor {
		t.Error("Expected executor to match input executor")
	}
}

func TestDescribeDBInstances(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mock_rds.NewMockRDSAPI(ctrl)
	adapter := &rds.AwsRDSAdapter{Client: mockClient}

	ctx := context.Background()
	input := &awsrds.DescribeDBInstancesInput{}

	t.Run("Successful describe instances", func(t *testing.T) {
		expectedOutput := &awsrds.DescribeDBInstancesOutput{
			DBInstances: []types.DBInstance{
				{DBInstanceIdentifier: aws.String("test-instance")},
			},
		}
		mockClient.EXPECT().DescribeDBInstances(ctx, input, gomock.Any()).Return(expectedOutput, nil)

		output, err := adapter.DescribeDBInstances(ctx, input)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if len(output.DBInstances) != 1 || aws.ToString(output.DBInstances[0].DBInstanceIdentifier) != "test-instance" {
			t.Errorf("Unexpected output: %+v", output)
		}
	})
}

func TestListRDSResources(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mock_rds.NewMockRDSAPI(ctrl)
	adapter := &rds.AwsRDSAdapter{Client: mockClient}

	ctx := context.Background()

	t.Run("Successfully lists both clusters and instances", func(t *testing.T) {
		mockClusters := &awsrds.DescribeDBClustersOutput{
			DBClusters: []types.DBCluster{
				{
					DBClusterIdentifier: aws.String("cluster-1"),
					Engine:              aws.String("aurora-mysql"),
					Endpoint:            aws.String("cluster-1.endpoint"),
				},
			},
		}
		mockClient.EXPECT().DescribeDBClusters(ctx, gomock.Any(), gomock.Any()).Return(mockClusters, nil)

		mockInstances := &awsrds.DescribeDBInstancesOutput{
			DBInstances: []types.DBInstance{
				{
					DBInstanceIdentifier: aws.String("instance-1"),
					Engine:               aws.String("mysql"),
					Endpoint: &types.Endpoint{
						Address: aws.String("instance-1.endpoint"),
					},
				},
			},
		}
		mockClient.EXPECT().DescribeDBInstances(ctx, gomock.Any(), gomock.Any()).Return(mockInstances, nil)

		resources, err := adapter.ListRDSResources(ctx)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if len(resources) != 2 {
			t.Fatalf("Expected 2 resources, got %d", len(resources))
		}

		if resources[0].DBInstanceIdentifier != "cluster-1" {
			t.Error("Resources not sorted correctly")
		}
	})

	t.Run("Filters out Aurora instances with cluster identifier", func(t *testing.T) {
		mockInstances := &awsrds.DescribeDBInstancesOutput{
			DBInstances: []types.DBInstance{
				{
					DBInstanceIdentifier: aws.String("aurora-instance"),
					Engine:               aws.String("aurora-mysql"),
					DBClusterIdentifier:  aws.String("cluster-1"),
					Endpoint: &types.Endpoint{
						Address: aws.String("aurora-instance.endpoint"),
					},
				},
			},
		}
		mockClient.EXPECT().DescribeDBClusters(ctx, gomock.Any(), gomock.Any()).Return(&awsrds.DescribeDBClustersOutput{}, nil)
		mockClient.EXPECT().DescribeDBInstances(ctx, gomock.Any(), gomock.Any()).Return(mockInstances, nil)

		resources, err := adapter.ListRDSResources(ctx)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if len(resources) != 0 {
			t.Fatalf("Expected Aurora instance to be filtered out, but got %d resources", len(resources))
		}
	})

}

func TestGetConnectionEndpoint(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mock_rds.NewMockRDSAPI(ctrl)
	adapter := &rds.AwsRDSAdapter{Client: mockClient}

	ctx := context.Background()

	t.Run("Returns cluster endpoint when found", func(t *testing.T) {
		mockCluster := &awsrds.DescribeDBClustersOutput{
			DBClusters: []types.DBCluster{
				{
					DBClusterIdentifier: aws.String("cluster-1"),
					Endpoint:            aws.String("cluster-1.endpoint"),
					Port:                aws.Int32(3306),
				},
			},
		}
		mockClient.EXPECT().DescribeDBClusters(ctx, gomock.Any(), gomock.Any()).Return(mockCluster, nil)

		endpoint, err := adapter.GetConnectionEndpoint(ctx, "cluster-1")
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		expected := "cluster-1.endpoint:3306"
		if endpoint != expected {
			t.Fatalf("Expected endpoint %s, got %s", expected, endpoint)
		}
	})

	t.Run("Returns instance endpoint when cluster not found", func(t *testing.T) {
		mockClient.EXPECT().DescribeDBClusters(ctx, gomock.Any(), gomock.Any()).Return(&awsrds.DescribeDBClustersOutput{}, nil)

		mockInstance := &awsrds.DescribeDBInstancesOutput{
			DBInstances: []types.DBInstance{
				{
					DBInstanceIdentifier: aws.String("instance-1"),
					Engine:               aws.String("mysql"),
					Endpoint: &types.Endpoint{
						Address: aws.String("instance-1.endpoint"),
						Port:    aws.Int32(3306),
					},
				},
			},
		}
		mockClient.EXPECT().DescribeDBInstances(ctx, gomock.Any(), gomock.Any()).Return(mockInstance, nil)

		endpoint, err := adapter.GetConnectionEndpoint(ctx, "instance-1")
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		expected := "instance-1.endpoint:3306"
		if endpoint != expected {
			t.Fatalf("Expected endpoint %s, got %s", expected, endpoint)
		}
	})

	t.Run("Uses default port when not specified", func(t *testing.T) {
		mockCluster := &awsrds.DescribeDBClustersOutput{
			DBClusters: []types.DBCluster{
				{
					DBClusterIdentifier: aws.String("cluster-1"),
					Endpoint:            aws.String("cluster-1.endpoint"),
					Port:                nil,
				},
			},
		}
		mockClient.EXPECT().DescribeDBClusters(ctx, gomock.Any(), gomock.Any()).Return(mockCluster, nil)

		endpoint, err := adapter.GetConnectionEndpoint(ctx, "cluster-1")
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		expected := "cluster-1.endpoint:3306"
		if endpoint != expected {
			t.Fatalf("Expected endpoint %s, got %s", expected, endpoint)
		}
	})

	t.Run("Returns error when resource not found", func(t *testing.T) {
		mockClient.EXPECT().DescribeDBClusters(ctx, gomock.Any(), gomock.Any()).Return(&awsrds.DescribeDBClustersOutput{}, nil)
		mockClient.EXPECT().DescribeDBInstances(ctx, gomock.Any(), gomock.Any()).Return(&awsrds.DescribeDBInstancesOutput{}, nil)

		_, err := adapter.GetConnectionEndpoint(ctx, "nonexistent")
		if err == nil {
			t.Error("Expected error, got nil")
		}
	})

}
