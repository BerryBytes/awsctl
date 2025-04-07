package bastion

import (
	"context"
	"errors"
	"testing"

	mock_awsctl "github.com/BerryBytes/awsctl/tests/mock"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/smithy-go"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func TestListBastionInstances(t *testing.T) {
	tests := []struct {
		name        string
		instances   []types.Instance
		wantCount   int
		wantIDs     []string
		description string
	}{
		{
			name: "Finds bastion in Role tag",
			instances: []types.Instance{
				{
					InstanceId: aws.String("i-123"),
					State:      &types.InstanceState{Name: types.InstanceStateNameRunning},
					Tags: []types.Tag{
						{Key: aws.String("Role"), Value: aws.String("bastion")},
					},
				},
			},
			wantCount:   1,
			wantIDs:     []string{"i-123"},
			description: "Should find instance with 'bastion' in Role tag",
		},
		{
			name: "Finds case-insensitive bastion",
			instances: []types.Instance{
				{
					InstanceId: aws.String("i-456"),
					State:      &types.InstanceState{Name: types.InstanceStateNameRunning},
					Tags: []types.Tag{
						{Key: aws.String("Service"), Value: aws.String("BASTION")},
					},
				},
			},
			wantCount:   1,
			wantIDs:     []string{"i-456"},
			description: "Should find instance with 'BASTION' in any tag (case-insensitive)",
		},
		{
			name: "Ignores non-bastion instances",
			instances: []types.Instance{
				{
					InstanceId: aws.String("i-789"),
					State:      &types.InstanceState{Name: types.InstanceStateNameRunning},
					Tags: []types.Tag{
						{Key: aws.String("Role"), Value: aws.String("web")},
					},
				},
			},
			wantCount:   0,
			wantIDs:     []string{},
			description: "Should exclude instances without 'bastion' in any tag",
		},
		{
			name: "Multiple bastions with sorting",
			instances: []types.Instance{
				{
					InstanceId: aws.String("i-bbb"),
					State:      &types.InstanceState{Name: types.InstanceStateNameRunning},
					Tags: []types.Tag{
						{Key: aws.String("Name"), Value: aws.String("z-bastion")},
						{Key: aws.String("Role"), Value: aws.String("bastion")},
					},
				},
				{
					InstanceId: aws.String("i-aaa"),
					State:      &types.InstanceState{Name: types.InstanceStateNameRunning},
					Tags: []types.Tag{
						{Key: aws.String("Name"), Value: aws.String("a-bastion")},
						{Key: aws.String("Service"), Value: aws.String("bastion")},
					},
				},
			},
			wantCount:   2,
			wantIDs:     []string{"i-aaa", "i-bbb"},
			description: "Should return multiple bastions sorted by Name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockAPI := mock_awsctl.NewMockEC2DescribeInstancesAPI(ctrl)
			client := &realEC2Client{client: mockAPI}

			mockAPI.EXPECT().DescribeInstances(gomock.Any(), &ec2.DescribeInstancesInput{
				Filters: []types.Filter{
					{Name: aws.String(InstanceStateName), Values: []string{RunningState}},
				},
			}).Return(&ec2.DescribeInstancesOutput{
				Reservations: []types.Reservation{
					{Instances: tt.instances},
				},
			}, nil)

			got, err := client.ListBastionInstances(context.Background())
			assert.NoError(t, err)
			assert.Equal(t, tt.wantCount, len(got), tt.description)

			for i, id := range tt.wantIDs {
				assert.Equal(t, id, got[i].InstanceID)
			}
		})
	}
}

func TestListBastionInstances_AWSFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAPI := mock_awsctl.NewMockEC2DescribeInstancesAPI(ctrl)
	client := &realEC2Client{client: mockAPI}

	awsError := &smithy.GenericAPIError{
		Code:    "AuthFailure",
		Message: "AWS authentication failed",
	}

	mockAPI.EXPECT().DescribeInstances(gomock.Any(), gomock.Any()).Return(
		&ec2.DescribeInstancesOutput{},
		awsError,
	)

	instances, err := client.ListBastionInstances(context.Background())

	assert.Error(t, err)
	assert.Nil(t, instances)
	assert.Contains(t, err.Error(), "AWS authentication failed. Please verify your credentials and IAM permissions")
}

func TestListBastionInstances_RequestExpired(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAPI := mock_awsctl.NewMockEC2DescribeInstancesAPI(ctrl)
	client := &realEC2Client{client: mockAPI}

	awsError := &smithy.GenericAPIError{
		Code:    "RequestExpired",
		Message: "AWS request expired",
	}

	mockAPI.EXPECT().DescribeInstances(gomock.Any(), gomock.Any()).Return(nil, awsError)

	instances, err := client.ListBastionInstances(context.Background())

	assert.Error(t, err)
	assert.Nil(t, instances)
	assert.Contains(t, err.Error(), "AWS request expired (likely due to clock skew or expired credentials)")
	assert.Contains(t, err.Error(), "Please verify your system clock or refresh AWS credentials")
}

func TestListBastionInstances_AuthFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAPI := mock_awsctl.NewMockEC2DescribeInstancesAPI(ctrl)
	client := &realEC2Client{client: mockAPI}

	awsError := &smithy.GenericAPIError{
		Code:    "UnauthorizedOperation",
		Message: "Unauthorized operation",
	}

	mockAPI.EXPECT().DescribeInstances(gomock.Any(), gomock.Any()).Return(nil, awsError)

	instances, err := client.ListBastionInstances(context.Background())

	assert.Error(t, err)
	assert.Nil(t, instances)
	assert.Contains(t, err.Error(), "AWS authentication failed. Please verify your credentials and IAM permissions")
}

func TestListBastionInstances_OptInRequired(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAPI := mock_awsctl.NewMockEC2DescribeInstancesAPI(ctrl)
	client := &realEC2Client{client: mockAPI}

	awsError := &smithy.GenericAPIError{
		Code:    "OptInRequired",
		Message: "AWS region is not enabled",
	}

	mockAPI.EXPECT().DescribeInstances(gomock.Any(), gomock.Any()).Return(nil, awsError)

	instances, err := client.ListBastionInstances(context.Background())

	assert.Error(t, err)
	assert.Nil(t, instances)
	assert.Contains(t, err.Error(), "AWS region is not enabled. Please opt-in for this region in your AWS account")
}

func TestListBastionInstances_MaxRetryExceeded(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAPI := mock_awsctl.NewMockEC2DescribeInstancesAPI(ctrl)
	client := &realEC2Client{client: mockAPI}

	opErr := &smithy.OperationError{
		ServiceID:     "EC2",
		OperationName: "DescribeInstances",
		Err:           errors.New("exceeded maximum number of attempts"),
	}

	mockAPI.EXPECT().DescribeInstances(gomock.Any(), gomock.Any()).Return(nil, opErr)

	instances, err := client.ListBastionInstances(context.Background())

	assert.Error(t, err)
	assert.Nil(t, instances)
	assert.Contains(t, err.Error(), "AWS request failed after multiple retries")
	assert.Contains(t, err.Error(), "This could be due to network issues, credential problems, or AWS service disruption")
}

func TestListBastionInstances_GenericAWSFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAPI := mock_awsctl.NewMockEC2DescribeInstancesAPI(ctrl)
	client := &realEC2Client{client: mockAPI}

	opErr := &smithy.OperationError{
		ServiceID:     "EC2",
		OperationName: "DescribeInstances",
		Err:           errors.New("unexpected error"),
	}

	mockAPI.EXPECT().DescribeInstances(gomock.Any(), gomock.Any()).Return(nil, opErr)

	instances, err := client.ListBastionInstances(context.Background())

	assert.Error(t, err)
	assert.Nil(t, instances)
	assert.Contains(t, err.Error(), "AWS operation failed")
}

func TestDescribeInstances(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAPI := mock_awsctl.NewMockEC2DescribeInstancesAPI(ctrl)
	client := &realEC2Client{client: mockAPI}

	input := &ec2.DescribeInstancesInput{
		InstanceIds: []string{"i-1234567890"},
	}

	expectedOutput := &ec2.DescribeInstancesOutput{
		Reservations: []types.Reservation{
			{
				Instances: []types.Instance{
					{
						InstanceId: aws.String("i-1234567890"),
					},
				},
			},
		},
	}

	mockAPI.EXPECT().DescribeInstances(gomock.Any(), input).Return(expectedOutput, nil)

	output, err := client.DescribeInstances(context.Background(), input)

	assert.NoError(t, err)
	assert.Equal(t, expectedOutput, output)
}

func TestHandleAWSError_GenericError(t *testing.T) {
	genericErr := errors.New("some generic error")
	wrappedErr := handleAWSError(genericErr)

	assert.Error(t, wrappedErr)
	assert.Contains(t, wrappedErr.Error(), "failed to list bastion instances")
	assert.Contains(t, wrappedErr.Error(), "some generic error")
}

func TestFilterBastionInstance_NilInstanceID(t *testing.T) {
	reservations := []types.Reservation{
		{
			Instances: []types.Instance{
				{
					InstanceId: nil,
					State:      &types.InstanceState{Name: types.InstanceStateNameRunning},
					Tags: []types.Tag{
						{Key: aws.String("Role"), Value: aws.String("bastion")},
					},
				},
				{
					InstanceId: aws.String("i-1234567890"),
					State:      &types.InstanceState{Name: types.InstanceStateNameRunning},
					Tags: []types.Tag{
						{Key: aws.String("Role"), Value: aws.String("bastion")},
					},
				},
			},
		},
	}

	instances := filterBastionInstance(reservations)

	assert.Len(t, instances, 1)
	assert.Equal(t, "i-1234567890", instances[0].InstanceID)
}

func TestFilterBastionInstance_NilTagKey(t *testing.T) {
	reservations := []types.Reservation{
		{
			Instances: []types.Instance{
				{
					InstanceId: aws.String("i-1234567890"),
					State:      &types.InstanceState{Name: types.InstanceStateNameRunning},
					Tags: []types.Tag{
						{Key: nil, Value: aws.String("should be skipped")},
						{Key: aws.String("Role"), Value: aws.String("bastion")},
						{Key: aws.String("Name"), Value: nil},
					},
				},
			},
		},
	}

	instances := filterBastionInstance(reservations)

	assert.Len(t, instances, 1)
	assert.Equal(t, "i-1234567890", instances[0].InstanceID)
	assert.Equal(t, "bastion", instances[0].Tags["Role"])
	assert.Empty(t, instances[0].Name)
	assert.NotContains(t, instances[0].Tags, "")
}

func TestListBastionInstances_Sorting(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAPI := mock_awsctl.NewMockEC2DescribeInstancesAPI(ctrl)
	client := &realEC2Client{client: mockAPI}

	testInstances := []types.Instance{
		{
			InstanceId: aws.String("i-2"),
			State:      &types.InstanceState{Name: types.InstanceStateNameRunning},
			Tags: []types.Tag{
				{Key: aws.String("Name"), Value: aws.String("beta-bastion")},
				{Key: aws.String("Role"), Value: aws.String("bastion")},
			},
		},
		{
			InstanceId: aws.String("i-1"),
			State:      &types.InstanceState{Name: types.InstanceStateNameRunning},
			Tags: []types.Tag{
				{Key: aws.String("Name"), Value: aws.String("alpha-bastion")},
				{Key: aws.String("Role"), Value: aws.String("bastion")},
			},
		},
	}

	mockAPI.EXPECT().DescribeInstances(gomock.Any(), gomock.Any()).Return(&ec2.DescribeInstancesOutput{
		Reservations: []types.Reservation{
			{Instances: testInstances},
		},
	}, nil)

	instances, err := client.ListBastionInstances(context.Background())

	assert.NoError(t, err)
	assert.Len(t, instances, 2)
	assert.Equal(t, "alpha-bastion", instances[0].Name)
	assert.Equal(t, "beta-bastion", instances[1].Name)
}
