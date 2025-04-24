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

	keyPath := ""
	useInstanceConnect := strings.HasPrefix(host, "i-") && p.awsConfigured
	method := MethodSSH
	instanceID := ""
	resolvedHost := host

	if useInstanceConnect {
		instanceDetails, err := p.getInstanceDetails(ctx, host)
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
		} else {
			method = MethodSSM
			useInstanceConnect = false
		}
	}

	if !useInstanceConnect && method == MethodSSH {
		keyPath, err = p.prompter.PromptForSSHKeyPath("~/.ssh/id_ed25519")
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
		if err := common.ValidateSSHKey(p.fs, keyPath); err != nil {
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
		publicKey, tempKeyPath, err := p.generateTempSSHKey()
		if err != nil {
			return nil, fmt.Errorf("failed to generate temporary SSH key: %w", err)
		}

		az, err := p.getInstanceAvailabilityZone(ctx, host)
		if err != nil {
			return nil, fmt.Errorf("failed to get availability zone for instance %s: %w", host, err)
		}

		input := &ec2instanceconnect.SendSSHPublicKeyInput{
			InstanceId:       &host,
			InstanceOSUser:   &user,
			SSHPublicKey:     &publicKey,
			AvailabilityZone: &az,
		}

		resp, err := p.instanceConn.SendSSHPublicKey(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("failed to send SSH public key to instance %s for user %s: %w", host, user, err)
		}
		fmt.Printf("SendSSHPublicKey success: %v\n", resp.Success)

		details.KeyPath = tempKeyPath
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

func (p *ConnectionProvider) generateTempSSHKey() (publicKey, tempKeyPath string, err error) {
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
		tempFile.Close()
		os.Remove(tempKeyPath)
		return "", "", fmt.Errorf("failed to write private key: %w", err)
	}
	tempFile.Close()

	if err := os.Chmod(tempKeyPath, 0600); err != nil {
		os.Remove(tempKeyPath)
		return "", "", fmt.Errorf("failed to set key permissions: %w", err)
	}

	sshPublicKey, err := ssh.NewPublicKey(&privateKey.PublicKey)
	if err != nil {
		os.Remove(tempKeyPath)
		return "", "", fmt.Errorf("failed to generate public key: %w", err)
	}
	publicKeyBytes := ssh.MarshalAuthorizedKey(sshPublicKey)
	publicKey = string(publicKeyBytes)

	return publicKey, tempKeyPath, nil
}

func (p *ConnectionProvider) getInstanceAvailabilityZone(ctx context.Context, instanceID string) (string, error) {
	if p.ec2Client == nil {
		return "", fmt.Errorf("EC2 client not initialized")
	}

	input := &ec2.DescribeInstancesInput{
		InstanceIds: []string{instanceID},
	}

	result, err := p.ec2Client.DescribeInstances(ctx, input)
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

func (p *ConnectionProvider) getInstanceDetails(ctx context.Context, instanceID string) (*types.Instance, error) {
	if p.ec2Client == nil {
		return nil, fmt.Errorf("EC2 client not initialized")
	}

	input := &ec2.DescribeInstancesInput{
		InstanceIds: []string{instanceID},
	}

	result, err := p.ec2Client.DescribeInstances(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to describe instance %s: %w", instanceID, err)
	}

	if len(result.Reservations) == 0 || len(result.Reservations[0].Instances) == 0 {
		return nil, fmt.Errorf("instance %s not found", instanceID)
	}

	return &result.Reservations[0].Instances[0], nil
}
