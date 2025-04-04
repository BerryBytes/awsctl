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
)

type BastionService struct {
	BPrompter     BastionPrompterInterface
	EC2Client     EC2ClientInterface
	Fs            common.FileSystemInterface
	SSHExecutor   common.SSHExecutorInterface
	AwsConfigured bool
	configLoader  AWSConfigLoader
	homeDir       func() (string, error)
}

type AWSConfigLoader func(ctx context.Context, optFns ...func(*config.LoadOptions) error) (aws.Config, error)

func NewBastionService(opts ...func(*BastionService)) *BastionService {
	service := &BastionService{
		BPrompter:   NewBastionPrompter(),
		Fs:          &common.RealFileSystem{},
		SSHExecutor: &common.RealSSHExecutor{},
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
			log.Printf("Failed to retrieve AWS credentials: %v", credErr)
			return
		}

		ec2Client := ec2.NewFromConfig(awsCfg)
		s.EC2Client = NewEC2Client(ec2Client)
		s.AwsConfigured = true
	}
}

func (s *BastionService) Run() error {
	for {
		action, err := s.BPrompter.SelectAction()
		if err != nil {
			if errors.Is(err, promptUtils.ErrInterrupted) {
				return promptUtils.ErrInterrupted
			}
			return fmt.Errorf("profile selection aborted: %v", err)
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
			return nil
		}
	}
}

func (s *BastionService) HandleSSH() error {
	host, user, keyPath, err := s.getConnectionDetails()
	if err != nil {
		return fmt.Errorf("failed to get connection details: %w", err)
	}

	builder := common.NewSSHCommandBuilder(host, user, keyPath)
	return common.ExecuteSSHCommand(s.SSHExecutor, builder.Build())
}

func (s *BastionService) HandleSOCKS() error {
	localPort, err := s.BPrompter.PromptForSOCKSProxyPort(9999)
	if err != nil {
		return err
	}

	host, user, keyPath, err := s.getConnectionDetails()
	if err != nil {
		return err
	}

	builder := common.NewSSHCommandBuilder(host, user, keyPath).
		WithSOCKS(localPort).
		WithBackground()

	return common.ExecuteSSHCommand(s.SSHExecutor, builder.Build())
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

	host, user, keyPath, err := s.getConnectionDetails()
	if err != nil {
		return err
	}

	builder := common.NewSSHCommandBuilder(host, user, keyPath).
		WithForwarding(localPort, remoteHost, remotePort).
		WithBackground()

	return common.ExecuteSSHCommand(s.SSHExecutor, builder.Build())
}

func (s *BastionService) getConnectionDetails() (host, user, keyPath string, err error) {
	host, err = s.GetBastionHost()
	if err != nil {
		return "", "", "", fmt.Errorf("failed to get host: %w", err)
	}

	user, err = s.BPrompter.PromptForSSHUser("ubuntu")
	if err != nil {
		return "", "", "", fmt.Errorf("failed to get user: %w", err)
	}

	keyPath, err = s.BPrompter.PromptForSSHKeyPath("~/.ssh/id_ed25519")
	if err != nil {
		return "", "", "", fmt.Errorf("failed to get key path: %w", err)
	}

	if strings.HasPrefix(keyPath, "~/") {
		var homeDir string
		if s.homeDir != nil {
			homeDir, err = s.homeDir()
		} else {
			homeDir, err = os.UserHomeDir()
		}
		if err != nil {
			return "", "", "", fmt.Errorf("failed to get home directory: %w", err)
		}
		keyPath = filepath.Join(homeDir, keyPath[2:])
	}
	if err := common.ValidateSSHKey(s.Fs, keyPath); err != nil {
		return "", "", "", fmt.Errorf("invalid SSH key: %w", err)
	}

	return host, user, keyPath, nil

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
