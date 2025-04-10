package bastion

import (
	"context"

	"github.com/BerryBytes/awsctl/models"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2instanceconnect"
)

type EC2ClientInterface interface {
	DescribeInstances(ctx context.Context, input *ec2.DescribeInstancesInput, opts ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error)
	ListBastionInstances(ctx context.Context) ([]models.EC2Instance, error)
}

type EC2DescribeInstancesAPI interface {
	DescribeInstances(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error)
}
type BastionPrompterInterface interface {
	SelectAction() (string, error)
	PromptForSOCKSProxyPort(defaultPort int) (int, error)
	PromptForBastionHost() (string, error)
	PromptForSSHUser(defaultUser string) (string, error)
	PromptForSSHKeyPath(defaultPath string) (string, error)
	PromptForConfirmation(prompt string) (bool, error)
	PromptForLocalPort(name string, defaultPort int) (int, error)
	PromptForRemoteHost() (string, error)
	PromptForRemotePort(name string) (int, error)
	PromptForBastionInstance(instances []models.EC2Instance) (string, error)
}

type BastionServiceInterface interface {
	Run() error
}
type EC2InstanceConnectClient interface {
	SendSSHPublicKey(ctx context.Context, params *ec2instanceconnect.SendSSHPublicKeyInput, optFns ...func(*ec2instanceconnect.Options)) (*ec2instanceconnect.SendSSHPublicKeyOutput, error)
}

var _ BastionPrompterInterface = (*BastionPrompter)(nil)
