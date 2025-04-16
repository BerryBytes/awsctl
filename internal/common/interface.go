package connection

import (
	"context"

	"github.com/BerryBytes/awsctl/models"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2instanceconnect"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

type ConnectionPrompter interface {
	ChooseConnectionMethod() (string, error)
	SelectAction() (string, error)
	PromptForSOCKSProxyPort(defaultPort int) (int, error)
	PromptForBastionHost() (string, error)
	PromptForSSHUser(defaultUser string) (string, error)
	PromptForSSHKeyPath(defaultPath string) (string, error)
	PromptForConfirmation(prompt string) (bool, error)
	PromptForLocalPort(usage string, defaultPort int) (int, error)
	PromptForRemoteHost() (string, error)
	PromptForRemotePort(usage string) (int, error)
	PromptForBastionInstance(instances []models.EC2Instance) (string, error)
	PromptForInstanceID() (string, error)
	PromptForRegion(defaultRegion string) (string, error)
}

type EC2ClientInterface interface {
	DescribeInstances(ctx context.Context, input *ec2.DescribeInstancesInput, opts ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error)
	ListBastionInstances(ctx context.Context) ([]models.EC2Instance, error)
}

type EC2DescribeInstancesAPI interface {
	DescribeInstances(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error)
}

type EC2InstanceConnectInterface interface {
	SendSSHPublicKey(ctx context.Context, input *ec2instanceconnect.SendSSHPublicKeyInput) (*ec2instanceconnect.SendSSHPublicKeyOutput, error)
}

type SSMClientInterface interface {
	StartSession(ctx context.Context, input *ssm.StartSessionInput) (*ssm.StartSessionOutput, error)
}

type SSMExecutorInterface interface {
	StartSession(instanceID string) error
	StartPortForwardingSession(instanceID string, localPort int, remoteHost string, remotePort int) error
}

type ec2InstanceConnectWrapper struct {
	client *ec2instanceconnect.Client
}

func (w *ec2InstanceConnectWrapper) SendSSHPublicKey(ctx context.Context, input *ec2instanceconnect.SendSSHPublicKeyInput) (*ec2instanceconnect.SendSSHPublicKeyOutput, error) {
	return w.client.SendSSHPublicKey(ctx, input)
}

func NewEC2InstanceConnectAdapter(client *ec2instanceconnect.Client) EC2InstanceConnectInterface {
	return &ec2InstanceConnectWrapper{client: client}
}

type ServicesInterface interface {
	SSHIntoBastion(ctx context.Context) error
	StartSOCKSProxy(ctx context.Context, port int) error
	StartPortForwarding(ctx context.Context, localPort int, remoteHost string, remotePort int) error
}

var _ ServicesInterface = (*Services)(nil)

type AWSConfigLoader interface {
	LoadDefaultConfig(ctx context.Context) (aws.Config, error)
}

type DefaultAWSConfigLoader struct{}

func (d *DefaultAWSConfigLoader) LoadDefaultConfig(ctx context.Context) (aws.Config, error) {
	return config.LoadDefaultConfig(ctx)
}
