package connection

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/BerryBytes/awsctl/models"
	mock_awsctl "github.com/BerryBytes/awsctl/tests/mock"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/smithy-go"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockEC2DescribeInstancesAPI struct {
	mock.Mock
}

func (m *MockEC2DescribeInstancesAPI) DescribeInstances(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
	args := m.Called(ctx, params, optFns)
	return args.Get(0).(*ec2.DescribeInstancesOutput), args.Error(1)
}

func TestNewEC2Client(t *testing.T) {
	mockAPI := &MockEC2DescribeInstancesAPI{}
	client := NewEC2Client(mockAPI)

	assert.NotNil(t, client)
	assert.IsType(t, &realEC2Client{}, client)
}

func TestListBastionInstances_Success(t *testing.T) {
	mockAPI := &MockEC2DescribeInstancesAPI{}
	client := NewEC2Client(mockAPI)

	output := &ec2.DescribeInstancesOutput{
		Reservations: []types.Reservation{
			{
				Instances: []types.Instance{
					{
						InstanceId:       aws.String("i-1234567890"),
						PublicIpAddress:  aws.String("1.2.3.4"),
						PrivateIpAddress: aws.String("10.0.0.1"),
						State: &types.InstanceState{
							Name: types.InstanceStateNameRunning,
						},
						InstanceType: types.InstanceTypeT2Micro,
						Placement: &types.Placement{
							AvailabilityZone: aws.String("us-east-1a"),
						},
						Tags: []types.Tag{
							{
								Key:   aws.String("Name"),
								Value: aws.String("bastion-host"),
							},
							{
								Key:   aws.String("Role"),
								Value: aws.String("bastion"),
							},
						},
					},
					{
						InstanceId:       aws.String("i-0987654321"),
						PublicIpAddress:  aws.String("5.6.7.8"),
						PrivateIpAddress: aws.String("10.0.0.2"),
						State: &types.InstanceState{
							Name: types.InstanceStateNameRunning,
						},
						InstanceType: types.InstanceTypeT2Small,
						Placement: &types.Placement{
							AvailabilityZone: aws.String("us-east-1b"),
						},
						Tags: []types.Tag{
							{
								Key:   aws.String("Name"),
								Value: aws.String("other-host"),
							},
							{
								Key:   aws.String("Role"),
								Value: aws.String("web-server"),
							},
						},
					},
				},
			},
		},
	}

	mockAPI.On("DescribeInstances", mock.Anything, mock.AnythingOfType("*ec2.DescribeInstancesInput"), mock.Anything).Return(output, nil)

	instances, err := client.ListBastionInstances(context.Background())

	assert.NoError(t, err)
	assert.Len(t, instances, 1)
	assert.Equal(t, "i-1234567890", instances[0].InstanceID)
	assert.Equal(t, "bastion-host", instances[0].Name)
	assert.Equal(t, "1.2.3.4", instances[0].PublicIPAddress)
	assert.Equal(t, "us-east-1a", instances[0].AZ)

	mockAPI.AssertExpectations(t)
}

func TestListBastionInstances_NoBastion(t *testing.T) {
	mockAPI := &MockEC2DescribeInstancesAPI{}
	client := NewEC2Client(mockAPI)

	output := &ec2.DescribeInstancesOutput{
		Reservations: []types.Reservation{
			{
				Instances: []types.Instance{
					{
						InstanceId: aws.String("i-0987654321"),
						State: &types.InstanceState{
							Name: types.InstanceStateNameRunning,
						},
						Tags: []types.Tag{
							{
								Key:   aws.String("Name"),
								Value: aws.String("web-server"),
							},
						},
					},
				},
			},
		},
	}

	mockAPI.On("DescribeInstances", mock.Anything, mock.AnythingOfType("*ec2.DescribeInstancesInput"), mock.Anything).Return(output, nil)

	instances, err := client.ListBastionInstances(context.Background())

	assert.NoError(t, err)
	assert.Empty(t, instances)
}

func TestListBastionInstances_RequestExpiredError(t *testing.T) {
	mockAPI := &MockEC2DescribeInstancesAPI{}
	client := NewEC2Client(mockAPI)

	apiErr := &smithy.GenericAPIError{
		Code:    CodeRequestExpired,
		Message: "Request has expired",
	}

	mockAPI.On("DescribeInstances", mock.Anything, mock.AnythingOfType("*ec2.DescribeInstancesInput"), mock.Anything).Return(
		&ec2.DescribeInstancesOutput{},
		apiErr,
	)

	_, err := client.ListBastionInstances(context.Background())

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "AWS request expired")
	assert.Contains(t, err.Error(), time.Now().Format(time.RFC3339))
}

func TestListBastionInstances_AuthFailureError(t *testing.T) {
	mockAPI := &MockEC2DescribeInstancesAPI{}
	client := NewEC2Client(mockAPI)

	apiErr := &smithy.GenericAPIError{
		Code:    CodeAuthFailure,
		Message: "Auth failure",
	}

	mockAPI.On("DescribeInstances", mock.Anything, mock.AnythingOfType("*ec2.DescribeInstancesInput"), mock.Anything).Return(
		&ec2.DescribeInstancesOutput{},
		apiErr,
	)

	_, err := client.ListBastionInstances(context.Background())

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "AWS authentication failed")
}

func TestListBastionInstances_RegionNotEnabledError(t *testing.T) {
	mockAPI := &MockEC2DescribeInstancesAPI{}
	client := NewEC2Client(mockAPI)

	apiErr := &smithy.GenericAPIError{
		Code:    CodeOptInRequired,
		Message: "Region not enabled",
	}

	mockAPI.On("DescribeInstances", mock.Anything, mock.AnythingOfType("*ec2.DescribeInstancesInput"), mock.Anything).Return(
		&ec2.DescribeInstancesOutput{},
		apiErr,
	)

	_, err := client.ListBastionInstances(context.Background())

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "AWS region is not enabled")
}

func TestListBastionInstances_MaxAttemptsExceededError(t *testing.T) {
	mockAPI := &MockEC2DescribeInstancesAPI{}
	client := NewEC2Client(mockAPI)

	opErr := &smithy.OperationError{
		Err: errors.New("exceeded maximum number of attempts"),
	}

	mockAPI.On("DescribeInstances", mock.Anything, mock.AnythingOfType("*ec2.DescribeInstancesInput"), mock.Anything).Return(
		&ec2.DescribeInstancesOutput{},
		opErr,
	)

	_, err := client.ListBastionInstances(context.Background())

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "AWS request failed after multiple retries")
}

func TestListBastionInstances_GenericOperationError(t *testing.T) {
	mockAPI := &MockEC2DescribeInstancesAPI{}
	client := NewEC2Client(mockAPI)

	opErr := &smithy.OperationError{
		Err: errors.New("some operation error"),
	}

	mockAPI.On("DescribeInstances", mock.Anything, mock.AnythingOfType("*ec2.DescribeInstancesInput"), mock.Anything).Return(
		&ec2.DescribeInstancesOutput{},
		opErr,
	)

	_, err := client.ListBastionInstances(context.Background())

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "AWS operation failed")
}

func TestListBastionInstances_UnknownError(t *testing.T) {
	mockAPI := &MockEC2DescribeInstancesAPI{}
	client := NewEC2Client(mockAPI)

	errUnknown := errors.New("some unknown error")

	mockAPI.On("DescribeInstances", mock.Anything, mock.AnythingOfType("*ec2.DescribeInstancesInput"), mock.Anything).Return(
		&ec2.DescribeInstancesOutput{},
		errUnknown,
	)

	_, err := client.ListBastionInstances(context.Background())

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to list bastion instances")
}

func TestFilterBastionInstance(t *testing.T) {
	tests := []struct {
		name      string
		input     []types.Reservation
		expected  []models.EC2Instance
		expectLen int
	}{
		{
			name: "Bastion instance with Name tag",
			input: []types.Reservation{
				{
					Instances: []types.Instance{
						{
							InstanceId: aws.String("i-123"),
							State: &types.InstanceState{
								Name: types.InstanceStateNameRunning,
							},
							Tags: []types.Tag{
								{
									Key:   aws.String("Name"),
									Value: aws.String("prod-bastion"),
								},
							},
						},
					},
				},
			},
			expectLen: 1,
			expected: []models.EC2Instance{
				{
					InstanceID: "i-123",
					Name:       "prod-bastion",
					State:      "running",
					Tags: map[string]string{
						"Name": "prod-bastion",
					},
				},
			},
		},
		{
			name: "Bastion instance with Role tag",
			input: []types.Reservation{
				{
					Instances: []types.Instance{
						{
							InstanceId: aws.String("i-456"),
							State: &types.InstanceState{
								Name: types.InstanceStateNameRunning,
							},
							Tags: []types.Tag{
								{
									Key:   aws.String("Role"),
									Value: aws.String("bastion-server"),
								},
							},
						},
					},
				},
			},
			expectLen: 1,
			expected: []models.EC2Instance{
				{
					InstanceID: "i-456",
					State:      "running",
					Tags: map[string]string{
						"Role": "bastion-server",
					},
				},
			},
		},
		{
			name: "Non-bastion instance",
			input: []types.Reservation{
				{
					Instances: []types.Instance{
						{
							InstanceId: aws.String("i-789"),
							State: &types.InstanceState{
								Name: types.InstanceStateNameRunning,
							},
							Tags: []types.Tag{
								{
									Key:   aws.String("Name"),
									Value: aws.String("web-server"),
								},
							},
						},
					},
				},
			},
			expectLen: 0,
			expected:  []models.EC2Instance{},
		},
		{
			name: "Instance with nil tags",
			input: []types.Reservation{
				{
					Instances: []types.Instance{
						{
							InstanceId: aws.String("i-000"),
							State: &types.InstanceState{
								Name: types.InstanceStateNameRunning,
							},
							Tags: []types.Tag{
								{
									Key:   nil,
									Value: aws.String("some-value"),
								},
								{
									Key:   aws.String("Name"),
									Value: nil,
								},
							},
						},
					},
				},
			},
			expectLen: 0,
			expected:  []models.EC2Instance{},
		},
		{
			name: "Instance with all fields populated",
			input: []types.Reservation{
				{
					Instances: []types.Instance{
						{
							InstanceId:       aws.String("i-111"),
							PublicIpAddress:  aws.String("1.1.1.1"),
							PrivateIpAddress: aws.String("10.0.0.1"),
							State: &types.InstanceState{
								Name: types.InstanceStateNameRunning,
							},
							InstanceType: types.InstanceTypeT2Micro,
							Placement: &types.Placement{
								AvailabilityZone: aws.String("us-east-1a"),
							},
							Tags: []types.Tag{
								{
									Key:   aws.String("Name"),
									Value: aws.String("full-bastion"),
								},
								{
									Key:   aws.String("Role"),
									Value: aws.String("bastion"),
								},
							},
						},
					},
				},
			},
			expectLen: 1,
			expected: []models.EC2Instance{
				{
					InstanceID:       "i-111",
					Name:             "full-bastion",
					PublicIPAddress:  "1.1.1.1",
					PrivateIPAddress: "10.0.0.1",
					State:            "running",
					InstanceType:     "t2.micro",
					AZ:               "us-east-1a",
					Tags: map[string]string{
						"Name": "full-bastion",
						"Role": "bastion",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filterBastionInstance(tt.input)
			assert.Len(t, result, tt.expectLen)

			if tt.expectLen > 0 {
				assert.Equal(t, tt.expected[0].InstanceID, result[0].InstanceID)
				assert.Equal(t, tt.expected[0].Name, result[0].Name)
				assert.Equal(t, tt.expected[0].State, result[0].State)

				for k, v := range tt.expected[0].Tags {
					assert.Equal(t, v, result[0].Tags[k])
				}
			}
		})
	}
}

func TestFilterBastionInstance_Sorting(t *testing.T) {
	input := []types.Reservation{
		{
			Instances: []types.Instance{
				{
					InstanceId: aws.String("i-2"),
					State: &types.InstanceState{
						Name: types.InstanceStateNameRunning,
					},
					Tags: []types.Tag{
						{
							Key:   aws.String("Name"),
							Value: aws.String("beta-bastion"),
						},
						{
							Key:   aws.String("Role"),
							Value: aws.String("bastion"),
						},
					},
				},
				{
					InstanceId: aws.String("i-1"),
					State: &types.InstanceState{
						Name: types.InstanceStateNameRunning,
					},
					Tags: []types.Tag{
						{
							Key:   aws.String("Name"),
							Value: aws.String("alpha-bastion"),
						},
						{
							Key:   aws.String("Role"),
							Value: aws.String("bastion"),
						},
					},
				},
				{
					InstanceId: aws.String("i-3"),
					State: &types.InstanceState{
						Name: types.InstanceStateNameRunning,
					},
					Tags: []types.Tag{
						{
							Key:   aws.String("Name"),
							Value: aws.String("alpha-bastion"),
						},
						{
							Key:   aws.String("Role"),
							Value: aws.String("bastion"),
						},
					},
				},
			},
		},
	}

	result := filterBastionInstance(input)
	assert.Len(t, result, 3)
	assert.Equal(t, "i-1", result[0].InstanceID)
	assert.Equal(t, "i-3", result[1].InstanceID)
	assert.Equal(t, "i-2", result[2].InstanceID)
}

func TestNewEC2ClientWithRegion_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLoader := mock_awsctl.NewMockAWSConfigLoader(ctrl)

	testConfig := aws.Config{
		Region: "us-east-1",
	}

	mockLoader.EXPECT().
		LoadDefaultConfig(gomock.Any()).
		Return(testConfig, nil)

	client, err := NewEC2ClientWithRegion("us-west-2", mockLoader)

	assert.NoError(t, err)
	assert.NotNil(t, client)
	assert.IsType(t, &realEC2Client{}, client)

}

func TestNewEC2ClientWithRegion_ConfigLoadError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLoader := mock_awsctl.NewMockAWSConfigLoader(ctrl)

	expectedErr := errors.New("failed to load config")

	mockLoader.EXPECT().
		LoadDefaultConfig(gomock.Any()).
		Return(aws.Config{}, expectedErr)

	client, err := NewEC2ClientWithRegion("us-west-2", mockLoader)

	assert.Error(t, err)
	assert.Nil(t, client)
	assert.Contains(t, err.Error(), "failed to load AWS config")
	assert.ErrorIs(t, err, expectedErr)
}
