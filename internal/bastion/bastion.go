package bastion

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/BerryBytes/awsctl/utils/common"
	promptUtils "github.com/BerryBytes/awsctl/utils/prompt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2instanceconnect"
)

type BastionService struct {
	BPrompter             BastionPrompterInterface
	EC2Client             EC2ClientInterface
	Fs                    common.FileSystemInterface
	SSHExecutor           common.SSHExecutorInterface
	AwsConfigured         bool
	configLoader          AWSConfigLoader
	homeDir               func() (string, error)
	socksPort             int
	InstanceConnectClient EC2InstanceConnectClient
	awsConfig             aws.Config
	osDetector            common.RuntimeOSDetector
}

type AWSConfigLoader func(ctx context.Context, optFns ...func(*config.LoadOptions) error) (aws.Config, error)

func NewBastionService(opts ...func(*BastionService)) *BastionService {
	service := &BastionService{
		BPrompter:   NewBastionPrompter(),
		Fs:          &common.RealFileSystem{},
		SSHExecutor: &common.RealSSHExecutor{},
		socksPort:   0,
	}

	for _, opt := range opts {
		opt(service)
	}

	return service
}

func WithAWSConfig(ctx context.Context) func(*BastionService) {
	return func(s *BastionService) {
		if s.configLoader == nil {
			s.configLoader = config.LoadDefaultConfig
		}

		awsCfg, err := s.configLoader(ctx)
		if err != nil {
			return
		}

		if awsCfg.Region == "" {
			return
		}

		if _, credErr := awsCfg.Credentials.Retrieve(ctx); credErr != nil {
			return
		}

		ec2Client := ec2.NewFromConfig(awsCfg)
		s.awsConfig = awsCfg
		s.EC2Client = NewEC2Client(ec2Client)
		s.InstanceConnectClient = ec2instanceconnect.NewFromConfig(awsCfg)
		s.AwsConfigured = true
	}
}

func (s *BastionService) Run() error {
	for {
		action, err := s.BPrompter.SelectAction()
		if err != nil {
			if errors.Is(err, promptUtils.ErrInterrupted) {
				if s.socksPort != 0 {
					if err := s.CleanupSOCKS(); err != nil {
						fmt.Printf("Failed to cleanup SOCKS proxy: %v\n", err)
					}
				}
				return promptUtils.ErrInterrupted
			}
			return fmt.Errorf("action selection aborted: %v", err)
		}

		switch action {
		case SSHIntoBastion:
			if err := s.HandleSSH(); err != nil {
				return fmt.Errorf("SSH into bastion failed: %w", err)
			}
			return nil
		case StartSOCKSProxy:
			if err := s.HandleSOCKS(); err != nil {
				return fmt.Errorf("SOCKS proxy setup failed: %w", err)
			}
		case PortForwarding:
			if err := s.HandlePortForward(); err != nil {
				return fmt.Errorf("port forwarding setup failed: %w", err)
			}
		case ExitBastion:
			if s.socksPort != 0 {
				if err := s.CleanupSOCKS(); err != nil {
					fmt.Printf("Failed to cleanup SOCKS proxy: %v\n", err)
				}
			}
			return nil
		}
	}
}

func (s *BastionService) HandleSSH() error {
	host, user, keyPath, useInstanceConnect, err := s.getConnectionDetails()
	if err != nil {
		return fmt.Errorf("failed to get connection details: %w", err)
	}

	builder := common.NewSSHCommandBuilder(host, user, keyPath, useInstanceConnect)
	return common.ExecuteSSHCommand(s.SSHExecutor, builder.Build())
}

func (s *BastionService) HandleSOCKS() error {
	localPort, err := s.BPrompter.PromptForSOCKSProxyPort(9999)
	if err != nil {
		return err
	}

	host, user, keyPath, useInstanceConnect, err := s.getConnectionDetails()
	if err != nil {
		return err
	}

	builder := common.NewSSHCommandBuilder(host, user, keyPath, useInstanceConnect).
		WithSOCKS(localPort)

	fmt.Printf("Establishing SOCKS proxy connection on port %d...\n", localPort)

	err = common.ExecuteSSHCommand(s.SSHExecutor, builder.Build())
	if err != nil {
		return fmt.Errorf("failed to start SOCKS proxy: %w", err)
	}

	fmt.Printf("\nSOCKS Proxy established on port %d\n", localPort)
	fmt.Printf("You can now configure your apps to use: socks5://127.0.0.1:%d\n", localPort)
	if useInstanceConnect {
		fmt.Println("Using EC2 Instance Connect — session valid ~60 seconds unless kept alive")
	}
	fmt.Println("Press Ctrl+C to terminate the session.")
	return nil
}

func (s *BastionService) HandlePortForward() error {
	localPort, err := s.BPrompter.PromptForLocalPort("port forwarding", 3500)
	if err != nil {
		return err
	}

	remoteHost, err := s.BPrompter.PromptForRemoteHost()
	if err != nil {
		return err
	}

	remotePort, err := s.BPrompter.PromptForRemotePort("destination")
	if err != nil {
		return err
	}

	host, user, keyPath, useInstanceConnect, err := s.getConnectionDetails()
	if err != nil {
		return err
	}

	builder := common.NewSSHCommandBuilder(host, user, keyPath, useInstanceConnect).
		WithForwarding(localPort, remoteHost, remotePort)

	fmt.Printf("Establishing port forwarding: localhost:%d → %s:%d...\n", localPort, remoteHost, remotePort)
	if useInstanceConnect {
		fmt.Println("Using EC2 Instance Connect - session expires in ~60 seconds")
	}

	err = common.ExecuteSSHCommand(s.SSHExecutor, builder.Build())
	if err != nil {
		return fmt.Errorf("failed to start port forwarding: %w", err)
	}

	fmt.Printf("\nPort forwarding established\n")
	fmt.Printf("Listening on localhost:%d → forwarding to %s:%d\n", localPort, remoteHost, remotePort)
	fmt.Println("Press Ctrl+C to stop")
	return nil
}

func (s *BastionService) getConnectionDetails() (host, user, keyPath string, useInstanceConnect bool, err error) {
	host, err = s.GetBastionHost()
	if err != nil {
		return "", "", "", false, fmt.Errorf("failed to get host: %w", err)
	}

	user, err = s.BPrompter.PromptForSSHUser("ec2-user")
	if err != nil {
		return "", "", "", false, fmt.Errorf("failed to get user: %w", err)
	}

	keyPath, err = s.BPrompter.PromptForSSHKeyPath("~/.ssh/id_ed25519")
	if err != nil {
		return "", "", "", false, fmt.Errorf("failed to get key path: %w", err)
	}

	if strings.HasPrefix(keyPath, "~/") {
		var homeDir string
		if s.homeDir != nil {
			homeDir, err = s.homeDir()
		} else {
			homeDir, err = os.UserHomeDir()
		}
		if err != nil {
			return "", "", "", false, fmt.Errorf("failed to get home directory: %w", err)
		}
		keyPath = filepath.Join(homeDir, keyPath[2:])
	}

	useInstanceConnect = strings.HasPrefix(host, "i-") && s.AwsConfigured
	if useInstanceConnect {
		pubKeyPath := keyPath + ".pub"
		if _, err := s.Fs.Stat(pubKeyPath); err == nil {
			if err := s.SendSSHPublicKey(context.TODO(), host, user, pubKeyPath); err != nil {
				log.Printf("Warning: EC2 Instance Connect failed (%v), falling back to public IP", err)
				useInstanceConnect = false
				if ip, err := s.getInstancePublicIP(host); err == nil && ip != "" {
					host = ip
				} else {
					return "", "", "", false, fmt.Errorf("failed to get public IP for fallback: %w", err)
				}
			}
		} else {
			useInstanceConnect = false
		}
	}

	if err := common.ValidateSSHKey(s.Fs, keyPath); err != nil {
		return "", "", "", false, fmt.Errorf("invalid SSH key: %w", err)
	}

	return host, user, keyPath, useInstanceConnect, nil
}

func (s *BastionService) GetBastionHost() (string, error) {

	if !s.AwsConfigured {
		fmt.Println("Please enter bastion host details below:")
		return s.BPrompter.PromptForBastionHost()
	}

	confirm, err := s.BPrompter.PromptForConfirmation("Look for bastion hosts in AWS?")
	if err != nil {
		if errors.Is(err, promptUtils.ErrInterrupted) {
			return "", promptUtils.ErrInterrupted
		}
		return "", fmt.Errorf("bastion host selection aborted: %v", err)
	}
	if !confirm {
		fmt.Println("Please enter bastion host details below:")
		return s.BPrompter.PromptForBastionHost()
	}

	instances, err := s.EC2Client.ListBastionInstances(context.TODO())
	if err != nil {
		fmt.Printf("AWS lookup failed: %v\n", err)
		fmt.Println("Please enter bastion host details below:")
		return s.BPrompter.PromptForBastionHost()
	}

	if len(instances) == 0 {
		fmt.Println("No bastion hosts found in AWS")
		fmt.Println("Please enter bastion host details below:")
		return s.BPrompter.PromptForBastionHost()
	}

	return s.BPrompter.PromptForBastionInstance(instances)
}

func (s *BastionService) CleanupSOCKS() error {
	if s.socksPort == 0 {
		return nil
	}
	err := common.TerminateSOCKSProxy(s.SSHExecutor, s.socksPort, s.osDetector)
	if err == nil {
		fmt.Printf("SOCKS proxy on port %d terminated.\n", s.socksPort)
		s.socksPort = 0
	}
	return err
}

func (s *BastionService) SendSSHPublicKey(ctx context.Context, instanceID, user, publicKeyPath string) error {
	keyContent, err := s.Fs.ReadFile(publicKeyPath)
	if err != nil {
		return fmt.Errorf("failed to read public key: %w", err)
	}

	describeOutput, err := s.EC2Client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
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
	az := instance.Placement.AvailabilityZone

	_, err = s.InstanceConnectClient.SendSSHPublicKey(ctx, &ec2instanceconnect.SendSSHPublicKeyInput{
		InstanceId:       &instanceID,
		InstanceOSUser:   &user,
		SSHPublicKey:     aws.String(string(keyContent)),
		AvailabilityZone: az,
	})
	if err != nil {
		return fmt.Errorf("failed to send SSH public key: %w", err)
	}

	return nil
}

func (s *BastionService) getInstancePublicIP(instanceID string) (string, error) {
	if !s.AwsConfigured {
		return "", errors.New("AWS not configured")
	}

	ctx := context.TODO()
	describeOutput, err := s.EC2Client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
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
