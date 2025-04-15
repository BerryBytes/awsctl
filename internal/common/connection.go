package connection

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/BerryBytes/awsctl/utils/common"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2instanceconnect"
)

const (
	MethodSSH string = "SSH"
	// MethodSSM string = "AWS Systems Manager (SSM)" // will add this option in future
)

type ConnectionDetails struct {
	Host               string
	User               string
	KeyPath            string
	InstanceID         string
	UseInstanceConnect bool
	Method             string
}

type ConnectionProvider struct {
	prompter      ConnectionPrompter
	fs            common.FileSystemInterface
	awsConfig     *aws.Config
	ec2Client     EC2ClientInterface
	instanceConn  EC2InstanceConnectInterface
	homeDir       func() (string, error)
	awsConfigured bool
}

func NewConnectionProvider(
	prompter ConnectionPrompter,
	fs common.FileSystemInterface,
	awsConfig *aws.Config,
	ec2Client EC2ClientInterface,
	ssmClient SSMClientInterface,
	instanceConn EC2InstanceConnectInterface,
) *ConnectionProvider {
	homeDir := os.UserHomeDir
	provider := &ConnectionProvider{
		prompter:      prompter,
		fs:            fs,
		awsConfig:     awsConfig,
		homeDir:       homeDir,
		awsConfigured: false,
	}

	if awsConfig != nil && awsConfig.Region != "" {
		if _, err := awsConfig.Credentials.Retrieve(context.Background()); err == nil {
			provider.ec2Client = ec2Client
			provider.instanceConn = instanceConn
			provider.awsConfigured = true

		}
	}

	return provider
}

func (p *ConnectionProvider) GetConnectionDetails(ctx context.Context) (*ConnectionDetails, error) {
	method, err := p.prompter.ChooseConnectionMethod()
	if err != nil {
		return nil, fmt.Errorf("failed to select connection method: %w", err)
	}

	switch method {
	case MethodSSH:
		return p.getSSHDetails(ctx)
	default:
		return nil, fmt.Errorf("unsupported connection method: %s", method)
	}
}

func (p *ConnectionProvider) getSSHDetails(ctx context.Context) (*ConnectionDetails, error) {
	host, err := p.getBastionHost(ctx)
	if err != nil {
		return nil, err
	}

	user, err := p.prompter.PromptForSSHUser("ec2-user")
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	keyPath, err := p.prompter.PromptForSSHKeyPath("~/.ssh/id_ed25519")
	if err != nil {
		return nil, fmt.Errorf("failed to get key path: %w", err)
	}

	if strings.HasPrefix(keyPath, "~/") {
		homeDir, err := p.homeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		keyPath = filepath.Join(homeDir, keyPath[2:])
	}

	details := &ConnectionDetails{
		Host:    host,
		User:    user,
		KeyPath: keyPath,
		Method:  MethodSSH,
	}

	if strings.HasPrefix(host, "i-") && p.awsConfig != nil {
		pubKeyPath := keyPath + ".pub"
		if _, err := p.fs.Stat(pubKeyPath); err == nil {
			if err := p.sendSSHPublicKey(ctx, host, user, pubKeyPath); err != nil {
				log.Printf("Warning: EC2 Instance Connect failed (%v), falling back to public IP", err)
				if ip, err := p.getInstancePublicIP(ctx, host); err == nil && ip != "" {
					details.Host = ip
				} else {
					return nil, fmt.Errorf("failed to get public IP for fallback: %w", err)
				}
			} else {
				details.UseInstanceConnect = true
			}
		}
	}

	if err := common.ValidateSSHKey(p.fs, keyPath); err != nil {
		return nil, fmt.Errorf("invalid SSH key: %w", err)
	}

	return details, nil
}

func (p *ConnectionProvider) getBastionHost(ctx context.Context) (string, error) {
	if !p.IsAWSConfigured() {
		fmt.Println("AWS configuration not found...")
		return p.prompter.PromptForBastionHost()
	}

	confirm, err := p.prompter.PromptForConfirmation("Look for bastion hosts in AWS?")
	if err != nil || !confirm {
		return p.prompter.PromptForBastionHost()
	}

	instances, err := p.ec2Client.ListBastionInstances(ctx)

	if err != nil {
		fmt.Printf("AWS lookup failed: %v\n", err)
		fmt.Println("Please enter bastion host details below:")
		return p.prompter.PromptForBastionHost()
	}

	if len(instances) == 0 {
		fmt.Println("No bastion hosts found in AWS")
		fmt.Println("Please enter bastion host details below:")
		return p.prompter.PromptForBastionHost()
	}

	return p.prompter.PromptForBastionInstance(instances)
}

func (p *ConnectionProvider) sendSSHPublicKey(ctx context.Context, instanceID, user, publicKeyPath string) error {
	keyContent, err := p.fs.ReadFile(publicKeyPath)
	if err != nil {
		return fmt.Errorf("failed to read public key: %w", err)
	}

	describeOutput, err := p.ec2Client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
		InstanceIds: []string{instanceID},
	})
	if err != nil {
		return fmt.Errorf("failed to describe instance: %w", err)
	}

	if len(describeOutput.Reservations) == 0 || len(describeOutput.Reservations[0].Instances) == 0 {
		return fmt.Errorf("no instance found with ID %s", instanceID)
	}

	instance := describeOutput.Reservations[0].Instances[0]
	if instance.Placement == nil || instance.Placement.AvailabilityZone == nil {
		return fmt.Errorf("instance %s has no availability zone", instanceID)
	}

	_, err = p.instanceConn.SendSSHPublicKey(ctx, &ec2instanceconnect.SendSSHPublicKeyInput{
		InstanceId:       &instanceID,
		InstanceOSUser:   &user,
		SSHPublicKey:     aws.String(string(keyContent)),
		AvailabilityZone: instance.Placement.AvailabilityZone,
	})
	return err
}

func (p *ConnectionProvider) getInstancePublicIP(ctx context.Context, instanceID string) (string, error) {
	describeOutput, err := p.ec2Client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
		InstanceIds: []string{instanceID},
	})
	if err != nil {
		return "", fmt.Errorf("failed to describe instance: %w", err)
	}

	if len(describeOutput.Reservations) == 0 || len(describeOutput.Reservations[0].Instances) == 0 {
		return "", fmt.Errorf("no instance found with ID %s", instanceID)
	}

	instance := describeOutput.Reservations[0].Instances[0]
	if instance.PublicIpAddress == nil {
		return "", fmt.Errorf("instance %s has no public IP address", instanceID)
	}

	return *instance.PublicIpAddress, nil
}

func (p *ConnectionProvider) IsAWSConfigured() bool {
	if p.awsConfig == nil {
		return false
	}

	if p.awsConfig.Region == "" {
		return false
	}

	if _, err := p.awsConfig.Credentials.Retrieve(context.TODO()); err != nil {
		return false
	}

	return true
}
