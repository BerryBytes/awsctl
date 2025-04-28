package connection

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"

	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/BerryBytes/awsctl/utils/common"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2instanceconnect"
	"golang.org/x/crypto/ssh"
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
	Prompter      ConnectionPrompter
	Fs            common.FileSystemInterface
	AwsConfig     aws.Config
	Ec2Client     EC2ClientInterface
	InstanceConn  EC2InstanceConnectInterface
	SsmClient     SSMClientInterface
	HomeDir       func() (string, error)
	AwsConfigured bool
	ConfigLoader  AWSConfigLoader
	NewEC2Client  func(region string, loader AWSConfigLoader) (EC2ClientInterface, error)
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
		Prompter:      prompter,
		Fs:            fs,
		AwsConfig:     awsConfig,
		HomeDir:       homeDir,
		Ec2Client:     ec2Client,
		InstanceConn:  instanceConn,
		SsmClient:     ssmClient,
		AwsConfigured: isAWSConfigured(awsConfig),
		ConfigLoader:  configLoader,
		NewEC2Client:  NewEC2ClientWithRegion,
	}
	if provider.AwsConfigured {
		provider.Ec2Client = ec2Client
		provider.InstanceConn = instanceConn
	}

	return provider
}

func (p *ConnectionProvider) GetConnectionDetails(ctx context.Context) (*ConnectionDetails, error) {
	method, err := p.Prompter.ChooseConnectionMethod()
	if err != nil {
		return nil, fmt.Errorf("failed to select connection method: %w", err)
	}

	switch method {
	case MethodSSH:
		return p.getSSHDetails(ctx)
	case MethodSSM:
		return p.GetSSMDetails(ctx)
	default:
		return nil, fmt.Errorf("unsupported connection method: %s", method)
	}
}

func (p *ConnectionProvider) getSSHDetails(ctx context.Context) (*ConnectionDetails, error) {
	host, err := p.GetBastionHost(ctx)
	if err != nil {
		return nil, err
	}

	user, err := p.Prompter.PromptForSSHUser("ec2-user")
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	keyPath := ""
	useInstanceConnect := strings.HasPrefix(host, "i-") && p.AwsConfigured
	method := MethodSSH
	instanceID := ""
	resolvedHost := host

	if useInstanceConnect {
		instanceDetails, err := p.GetInstanceDetails(ctx, host)
		if err != nil {
			return nil, fmt.Errorf("failed to get instance details for %s: %w", host, err)
		}
		instanceID = host
		hasPublicIP := instanceDetails.PublicIpAddress != nil && *instanceDetails.PublicIpAddress != ""

		if hasPublicIP {
			if instanceDetails.PublicIpAddress != nil && *instanceDetails.PublicIpAddress != "" {
				resolvedHost = *instanceDetails.PublicIpAddress
			} else if instanceDetails.PublicDnsName != nil && *instanceDetails.PublicDnsName != "" {
				resolvedHost = *instanceDetails.PublicDnsName
			} else {
				return nil, fmt.Errorf("instance %s has no valid public IP or DNS name", host)
			}
		}
	}

	if !useInstanceConnect && method == MethodSSH {
		keyPath, err = p.Prompter.PromptForSSHKeyPath("~/.ssh/id_ed25519")
		if err != nil {
			return nil, fmt.Errorf("failed to get key path: %w", err)
		}
		if strings.HasPrefix(keyPath, "~/") {
			homeDir, err := p.HomeDir()
			if err != nil {
				return nil, fmt.Errorf("failed to get home directory: %w", err)
			}
			keyPath = filepath.Join(homeDir, keyPath[2:])
		}
		if err := common.ValidateSSHKey(p.Fs, keyPath); err != nil {
			return nil, fmt.Errorf("invalid SSH key: %w", err)
		}
	}

	details := &ConnectionDetails{
		Host:               resolvedHost,
		User:               user,
		KeyPath:            keyPath,
		Method:             method,
		UseInstanceConnect: useInstanceConnect,
		InstanceID:         instanceID,
	}

	if useInstanceConnect {
		publicKey, tempKeyPath, err := p.GenerateTempSSHKey()
		if err != nil {
			return nil, fmt.Errorf("failed to generate temporary SSH key: %w", err)
		}

		az, err := p.GetInstanceAvailabilityZone(ctx, host)
		if err != nil {
			return nil, fmt.Errorf("failed to get availability zone for instance %s: %w", host, err)
		}

		input := &ec2instanceconnect.SendSSHPublicKeyInput{
			InstanceId:       &host,
			InstanceOSUser:   &user,
			SSHPublicKey:     &publicKey,
			AvailabilityZone: &az,
		}

		_, err = p.InstanceConn.SendSSHPublicKey(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("failed to send SSH public key to instance %s for user %s: %w", host, user, err)
		}

		details.KeyPath = tempKeyPath
	}

	return details, nil
}

func (p *ConnectionProvider) GetSSMDetails(ctx context.Context) (*ConnectionDetails, error) {
	if !p.AwsConfigured {
		return nil, errors.New("AWS configuration required for SSM access")
	}

	instanceID, awsErr := p.GetBastionInstanceID(ctx, true)
	if awsErr == nil {
		return &ConnectionDetails{
			InstanceID: instanceID,
			Method:     MethodSSM,
			SSMClient:  p.SsmClient,
		}, nil
	}

	fmt.Printf("AWS lookup failed: %v\n", awsErr)
	fmt.Println("Please enter the instance ID manually (e.g., i-1234567890abcdef0)")

	host, err := p.Prompter.PromptForInstanceID()
	if err != nil {
		return nil, fmt.Errorf("failed to get instance ID: %w", err)
	}

	if !strings.HasPrefix(host, "i-") {
		return nil, errors.New("invalid instance ID format - should start with 'i-'")
	}

	return &ConnectionDetails{
		InstanceID: host,
		Method:     MethodSSM,
		SSMClient:  p.SsmClient,
	}, nil
}

func (p *ConnectionProvider) GetBastionInstanceID(ctx context.Context, isSSM bool) (string, error) {
	defaultRegion, err := p.GetDefaultRegion()
	if err != nil {
		fmt.Printf("Failed to load default region: %v\n", err)
		defaultRegion = ""
	}

	region, err := p.Prompter.PromptForRegion(defaultRegion)
	if err != nil {
		return "", fmt.Errorf("failed to get region: %w", err)
	}

	loader := &DefaultAWSConfigLoader{}
	ec2Client, err := p.NewEC2Client(region, loader)
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

	return p.Prompter.PromptForBastionInstance(instances, isSSM)
}

func (p *ConnectionProvider) GetBastionHost(ctx context.Context) (string, error) {
	if !p.AwsConfigured {
		fmt.Println("AWS configuration not found...")
		return p.Prompter.PromptForBastionHost()
	}

	confirm, err := p.Prompter.PromptForConfirmation("Look for bastion hosts in AWS?")
	if err != nil || !confirm {
		return p.Prompter.PromptForBastionHost()
	}

	defaultRegion, err := p.GetDefaultRegion()
	if err != nil {
		fmt.Printf("Failed to load default region: %v\n", err)
		defaultRegion = ""
	}

	region, err := p.Prompter.PromptForRegion(defaultRegion)
	if err != nil {
		fmt.Printf("Failed to get region: %v\n", err)
		return p.Prompter.PromptForBastionHost()
	}

	loader := &DefaultAWSConfigLoader{}
	ec2Client, err := p.NewEC2Client(region, loader)
	if err != nil {
		log.Printf("Failed to initialize EC2 client: %v", err)
		return p.Prompter.PromptForBastionHost()
	}

	instances, err := ec2Client.ListBastionInstances(ctx)
	if err != nil {
		log.Printf("AWS lookup failed: %v", err)
		log.Println("Please enter bastion host details below:")
		return p.Prompter.PromptForBastionHost()
	}

	if len(instances) == 0 {
		fmt.Println("No bastion hosts found in AWS")
		fmt.Println("Please enter bastion host details below:")
		return p.Prompter.PromptForBastionHost()
	}

	return p.Prompter.PromptForBastionInstance(instances, false)
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

func (p *ConnectionProvider) GetDefaultRegion() (string, error) {
	if p.AwsConfig.Region != "" {
		return p.AwsConfig.Region, nil
	}
	cfg, err := p.ConfigLoader.LoadDefaultConfig(context.TODO())
	if err != nil {
		return "", fmt.Errorf("failed to load AWS configuration: %w", err)
	}
	if cfg.Region == "" {
		return "", errors.New("no region configured in AWS config")
	}
	return cfg.Region, nil
}

func (p *ConnectionProvider) GenerateTempSSHKey() (publicKey, tempKeyPath string, err error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate private key: %w", err)
	}

	privateKeyPEM := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	}

	tempDir := os.TempDir()
	tempFile, err := os.CreateTemp(tempDir, "awsctl-ssh-key-*.pem")
	if err != nil {
		return "", "", fmt.Errorf("failed to create temporary file: %w", err)
	}
	tempKeyPath = tempFile.Name()

	if err := pem.Encode(tempFile, privateKeyPEM); err != nil {
		_ = tempFile.Close()
		_ = p.Fs.Remove(tempKeyPath)
		return "", "", fmt.Errorf("failed to write private key: %w", err)
	}
	_ = tempFile.Close()

	if err := os.Chmod(tempKeyPath, 0600); err != nil {
		_ = p.Fs.Remove(tempKeyPath)
		return "", "", fmt.Errorf("failed to set key permissions: %w", err)
	}

	sshPublicKey, err := ssh.NewPublicKey(&privateKey.PublicKey)
	if err != nil {
		_ = p.Fs.Remove(tempKeyPath)
		return "", "", fmt.Errorf("failed to generate public key: %w", err)
	}
	publicKeyBytes := ssh.MarshalAuthorizedKey(sshPublicKey)
	publicKey = string(publicKeyBytes)

	return publicKey, tempKeyPath, nil
}

func (p *ConnectionProvider) GetInstanceAvailabilityZone(ctx context.Context, instanceID string) (string, error) {
	if p.Ec2Client == nil {
		return "", fmt.Errorf("EC2 client not initialized")
	}

	input := &ec2.DescribeInstancesInput{
		InstanceIds: []string{instanceID},
	}

	result, err := p.Ec2Client.DescribeInstances(ctx, input)
	if err != nil {
		return "", fmt.Errorf("failed to describe instance %s: %w", instanceID, err)
	}

	if len(result.Reservations) == 0 || len(result.Reservations[0].Instances) == 0 {
		return "", fmt.Errorf("instance %s not found", instanceID)
	}

	instance := result.Reservations[0].Instances[0]
	if instance.Placement == nil || instance.Placement.AvailabilityZone == nil {
		return "", fmt.Errorf("availability zone not found for instance %s", instanceID)
	}

	return *instance.Placement.AvailabilityZone, nil
}

func (p *ConnectionProvider) GetInstanceDetails(ctx context.Context, instanceID string) (*types.Instance, error) {
	if p.Ec2Client == nil {
		return nil, fmt.Errorf("EC2 client not initialized")
	}

	input := &ec2.DescribeInstancesInput{
		InstanceIds: []string{instanceID},
	}

	result, err := p.Ec2Client.DescribeInstances(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to describe instance %s: %w", instanceID, err)
	}

	if len(result.Reservations) == 0 || len(result.Reservations[0].Instances) == 0 {
		return nil, fmt.Errorf("instance %s not found", instanceID)
	}

	return &result.Reservations[0].Instances[0], nil
}
