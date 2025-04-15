package connection

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/BerryBytes/awsctl/models"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/smithy-go"
)

const (
	InstanceStateName = "instance-state-name"
	RunningState      = "running"
	TagRole           = "tag:Role"
	BastionWildcard   = "*bastion*"
	TagName           = "Name"
)

const (
	ErrRequestExpired       = "AWS request expired (likely due to clock skew or expired credentials). Current system time: %s. Please verify your system clock or refresh AWS credentials: %w"
	ErrAuthFailure          = "AWS authentication failed. Please verify your credentials and IAM permissions: %w"
	ErrRegionNotEnabled     = "AWS region is not enabled. Please opt-in for this region in your AWS account: %w"
	ErrMaxAttemptsExceeded  = "AWS request failed after multiple retries. This could be due to network issues, credential problems, or AWS service disruption: %w"
	ErrOperationFailed      = "AWS operation failed: %w"
	ErrListBastionInstances = "failed to list bastion instances: %w"
)

const (
	CodeRequestExpired = "RequestExpired"
	CodeAuthFailure    = "AuthFailure"
	CodeUnauthorized   = "UnauthorizedOperation"
	CodeOptInRequired  = "OptInRequired"
)

type realEC2Client struct {
	client EC2DescribeInstancesAPI
}

func NewEC2Client(client EC2DescribeInstancesAPI) EC2ClientInterface {
	return &realEC2Client{client: client}
}

func (r *realEC2Client) DescribeInstances(ctx context.Context, input *ec2.DescribeInstancesInput, opts ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
	return r.client.DescribeInstances(ctx, input, opts...)
}

func (c *realEC2Client) ListBastionInstances(ctx context.Context) ([]models.EC2Instance, error) {
	input := &ec2.DescribeInstancesInput{
		Filters: []types.Filter{
			{Name: aws.String(InstanceStateName), Values: []string{RunningState}},
		},
	}

	result, err := c.client.DescribeInstances(ctx, input)
	if err != nil {
		return nil, handleAWSError(err)
	}

	instances := filterBastionInstance(result.Reservations)

	return instances, nil
}

func handleAWSError(err error) error {
	var apiErr *smithy.GenericAPIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case CodeRequestExpired:
			return fmt.Errorf(ErrRequestExpired, time.Now().Format(time.RFC3339), err)
		case CodeAuthFailure, CodeUnauthorized:
			return fmt.Errorf(ErrAuthFailure, err)
		case CodeOptInRequired:
			return fmt.Errorf(ErrRegionNotEnabled, err)
		}
	}

	var opErr *smithy.OperationError
	if errors.As(err, &opErr) {
		if strings.Contains(err.Error(), "exceeded maximum number of attempts") {
			return fmt.Errorf(ErrMaxAttemptsExceeded, err)
		}
		return fmt.Errorf(ErrOperationFailed, err)
	}

	return fmt.Errorf(ErrListBastionInstances, err)
}

func filterBastionInstance(reservations []types.Reservation) []models.EC2Instance {
	var instances []models.EC2Instance

	for _, reservation := range reservations {
		for _, instance := range reservation.Instances {
			if instance.InstanceId == nil {
				continue
			}

			inst := models.EC2Instance{
				InstanceID:       aws.ToString(instance.InstanceId),
				PublicIPAddress:  aws.ToString(instance.PublicIpAddress),
				PrivateIPAddress: aws.ToString(instance.PrivateIpAddress),
				State:            string(instance.State.Name),
				InstanceType:     string(instance.InstanceType),
				Tags:             make(map[string]string),
			}

			if instance.Placement != nil {
				inst.AZ = aws.ToString(instance.Placement.AvailabilityZone)
			}

			isBastion := false
			for _, tag := range instance.Tags {
				if tag.Key == nil || tag.Value == nil {
					continue
				}
				key := aws.ToString(tag.Key)
				value := aws.ToString(tag.Value)
				inst.Tags[key] = value

				if key == TagName {
					inst.Name = value
				}

				if strings.Contains(strings.ToLower(value), "bastion") {
					isBastion = true
				}
			}

			if isBastion {
				instances = append(instances, inst)
			}
		}
	}

	sort.Slice(instances, func(i, j int) bool {
		if instances[i].Name == instances[j].Name {
			return instances[i].InstanceID < instances[j].InstanceID
		}
		return instances[i].Name < instances[j].Name
	})

	return instances

}

func NewEC2ClientWithRegion(region string) (EC2ClientInterface, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}
	ec2Client := ec2.NewFromConfig(cfg)
	return &realEC2Client{client: ec2Client}, nil
}
