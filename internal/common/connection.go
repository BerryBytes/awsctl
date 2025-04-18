package connection

import (
	"context"
	"errors"
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
	MethodSSM string = "AWS Systems Manager (SSM)"
)

type ConnectionDetails struct {
	Host               string
	User               string
	KeyPath            string
	InstanceID         string
	UseInstanceConnect bool
	Method             string
	SSMClient          SSMClientInterface
}

type ConnectionProvider struct {
	prompter      ConnectionPrompter
	fs            common.FileSystemInterface
	awsConfig     aws.Config
	ec2Client     EC2ClientInterface
	instanceConn  EC2InstanceConnectInterface
	ssmClient     SSMClientInterface
	homeDir       func() (string, error)
	awsConfigured bool
	configLoader  AWSConfigLoader
	newEC2Client  func(region string, loader AWSConfigLoader) (EC2ClientInterface, error)
}

func NewConnectionProvider(
	prompter ConnectionPrompter,
	fs common.FileSystemInterface,
	awsConfig aws.Config,
	ec2Client EC2ClientInterface,
	ssmClient SSMClientInterface,
	instanceConn EC2InstanceConnectInterface,
	configLoader AWSConfigLoader,
) *ConnectionProvider {
	homeDir := os.UserHomeDir
	if configLoader == nil {
		configLoader = &DefaultAWSConfigLoader{}
	}
	provider := &ConnectionProvider{
		prompter:      prompter,
		fs:            fs,
		awsConfig:     awsConfig,
		homeDir:       homeDir,
		ec2Client:     ec2Client,
		instanceConn:  instanceConn,
		ssmClient:     ssmClient,
		awsConfigured: isAWSConfigured(awsConfig),
		configLoader:  configLoader,
		newEC2Client:  NewEC2ClientWithRegion,
	}
	if provider.awsConfigured {
		provider.ec2Client = ec2Client
		provider.instanceConn = instanceConn
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
	case MethodSSM:
		return p.getSSMDetails(ctx)
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

	if strings.HasPrefix(host, "i-") && p.awsConfigured {
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

func (p *ConnectionProvider) getSSMDetails(ctx context.Context) (*ConnectionDetails, error) {
	if !p.awsConfigured {
		return nil, errors.New("AWS configuration required for SSM access")
	}

	instanceID, awsErr := p.getBastionInstanceID(ctx, true)
	if awsErr == nil {
		return &ConnectionDetails{
			InstanceID: instanceID,
			Method:     MethodSSM,
			SSMClient:  p.ssmClient,
		}, nil
	}

	fmt.Printf("AWS lookup failed: %v\n", awsErr)
	fmt.Println("Please enter the instance ID manually (e.g., i-1234567890abcdef0)")

	host, err := p.prompter.PromptForInstanceID()
	if err != nil {
		return nil, fmt.Errorf("failed to get instance ID: %w", err)
	}

	if !strings.HasPrefix(host, "i-") {
		return nil, errors.New("invalid instance ID format - should start with 'i-'")
	}

	return &ConnectionDetails{
		InstanceID: host,
		Method:     MethodSSM,
		SSMClient:  p.ssmClient,
	}, nil
}

func (p *ConnectionProvider) getBastionInstanceID(ctx context.Context, isSSM bool) (string, error) {
	defaultRegion, err := p.getDefaultRegion()
	if err != nil {
		fmt.Printf("Failed to load default region: %v\n", err)
		defaultRegion = ""
	}

	region, err := p.prompter.PromptForRegion(defaultRegion)
	if err != nil {
		return "", fmt.Errorf("failed to get region: %w", err)
	}

	loader := &DefaultAWSConfigLoader{}
	ec2Client, err := p.newEC2Client(region, loader)
	if err != nil {
		return "", fmt.Errorf("failed to initialize EC2 client: %w", err)
	}

	instances, err := ec2Client.ListBastionInstances(ctx)
	if err != nil {
		return "", fmt.Errorf("AWS lookup failed: %w", err)
	}

	if len(instances) == 0 {
		return "", errors.New("no bastion hosts found in AWS")
	}

	return p.prompter.PromptForBastionInstance(instances, isSSM)
}

func (p *ConnectionProvider) getBastionHost(ctx context.Context) (string, error) {
	if !p.awsConfigured {
		fmt.Println("AWS configuration not found...")
		return p.prompter.PromptForBastionHost()
	}

	confirm, err := p.prompter.PromptForConfirmation("Look for bastion hosts in AWS?")
	if err != nil || !confirm {
		return p.prompter.PromptForBastionHost()
	}

	defaultRegion, err := p.getDefaultRegion()
	if err != nil {
		fmt.Printf("Failed to load default region: %v\n", err)
		defaultRegion = ""
	}

	region, err := p.prompter.PromptForRegion(defaultRegion)
	if err != nil {
		fmt.Printf("Failed to get region: %v\n", err)
		return p.prompter.PromptForBastionHost()
	}

	loader := &DefaultAWSConfigLoader{}
	ec2Client, err := p.newEC2Client(region, loader)
	if err != nil {
		log.Printf("Failed to initialize EC2 client: %v", err)
		return p.prompter.PromptForBastionHost()
	}

	instances, err := ec2Client.ListBastionInstances(ctx)
	if err != nil {
		log.Printf("AWS lookup failed: %v", err)
		log.Println("Please enter bastion host details below:")
		return p.prompter.PromptForBastionHost()
	}

	if len(instances) == 0 {
		fmt.Println("No bastion hosts found in AWS")
		fmt.Println("Please enter bastion host details below:")
		return p.prompter.PromptForBastionHost()
	}

	return p.prompter.PromptForBastionInstance(instances, false)
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

func isAWSConfigured(cfg aws.Config) bool {
	if cfg.Region == "" {
		return false
	}
	if _, err := cfg.Credentials.Retrieve(context.TODO()); err != nil {
		return false
	}
	return true
}

func (p *ConnectionProvider) getDefaultRegion() (string, error) {
	if p.awsConfig.Region != "" {
		return p.awsConfig.Region, nil
	}
	cfg, err := p.configLoader.LoadDefaultConfig(context.TODO())
	if err != nil {
		return "", fmt.Errorf("failed to load AWS configuration: %w", err)
	}
	if cfg.Region == "" {
		return "", errors.New("no region configured in AWS config")
	}
	return cfg.Region, nil
}
