package ecr

import (
	"context"
	"fmt"
	"os"

	connection "github.com/BerryBytes/awsctl/internal/common"
	"github.com/BerryBytes/awsctl/internal/sso"
	"github.com/BerryBytes/awsctl/utils/common"
	promptUtils "github.com/BerryBytes/awsctl/utils/prompt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
)

type ECRService struct {
	EPrompter        ECRPromptInterface
	CPrompter        connection.ConnectionPrompter
	AWSClient        ProfileProvider
	ECRClient        ECRAdapterInterface
	ConnServices     connection.ServicesInterface
	ConnProvider     *connection.ConnectionProvider
	Prompt           promptUtils.Prompter
	ConfigLoader     ConfigLoader
	ECRClientFactory ECRClientFactory
	FileSystem       common.FileSystemInterface
	Executor         common.CommandExecutor
}

func NewECRService(
	connServices connection.ServicesInterface,
	awsClient ProfileProvider,
	opts ...func(*ECRService),
) *ECRService {
	prompter := connection.NewConnectionPrompter()
	prompt := promptUtils.NewPrompt()
	configClient := &sso.RealSSOClient{Executor: &common.RealCommandExecutor{}}

	service := &ECRService{
		EPrompter:        NewEPrompter(prompt, configClient),
		CPrompter:        prompter,
		AWSClient:        awsClient,
		ConnServices:     connServices,
		Prompt:           prompt,
		ConfigLoader:     &RealConfigLoader{},
		ECRClientFactory: &RealECRClientFactory{},
		FileSystem:       &common.RealFileSystem{},
		Executor:         &common.RealCommandExecutor{},
	}

	for _, opt := range opts {
		opt(service)
	}

	return service
}

type RealConfigLoader struct{}

func (r *RealConfigLoader) LoadDefaultConfig(ctx context.Context, opts ...func(*config.LoadOptions) error) (aws.Config, error) {
	return config.LoadDefaultConfig(ctx, opts...)
}

type RealECRClientFactory struct{}

func (r *RealECRClientFactory) NewECRClient(cfg aws.Config, fs common.FileSystemInterface, executor common.CommandExecutor) ECRAdapterInterface {
	return NewECRClient(cfg, fs, executor)
}

func (s *ECRService) Run() error {
	for {
		action, err := s.EPrompter.SelectECRAction()
		if err != nil {
			return fmt.Errorf("action selection aborted: %v", err)
		}

		switch action {
		case LoginECR:
			if err := s.HandleECRLogin(); err != nil {
				return fmt.Errorf("ECR login failed: %w", err)
			}
			return nil
		case ExitECR:
			return nil
		}
	}
}

func (s *ECRService) HandleECRLogin() error {
	if !s.ConnServices.IsAWSConfigured() {
		return fmt.Errorf("AWS configuration not found")
	}

	confirm, err := s.CPrompter.PromptForConfirmation("Login to AWS ECR?")
	if err != nil {
		return fmt.Errorf("failed to confirm ECR login: %w", err)
	}
	if !confirm {
		fmt.Println("ECR login cancelled")
		return nil
	}

	defaultRegion := ""
	if s.ConnProvider != nil {
		defaultRegion, err = s.ConnProvider.GetDefaultRegion()
		if err != nil {
			fmt.Printf("Failed to load default region: %v\n", err)
			defaultRegion = ""
		}
	}

	region, err := s.CPrompter.PromptForRegion(defaultRegion)
	if err != nil {
		return fmt.Errorf("failed to get region: %w", err)
	}

	profile := os.Getenv("AWS_PROFILE")
	if profile == "" {
		profiles, err := s.AWSClient.ValidProfiles()
		if err != nil {
			return fmt.Errorf("failed to list valid profiles: %w", err)
		}
		if len(profiles) == 0 {
			return fmt.Errorf("no valid AWS profiles found")
		}
		if len(profiles) == 1 {
			profile = profiles[0]
		} else {
			profile, err = s.Prompt.PromptForSelection("Select AWS profile:", profiles)
			if err != nil {
				return fmt.Errorf("failed to select AWS profile: %w", err)
			}
		}
	}

	cfg, err := s.ConfigLoader.LoadDefaultConfig(context.TODO(),
		config.WithRegion(region),
		config.WithSharedConfigProfile(profile),
	)
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}

	if s.ECRClient == nil {
		s.ECRClient = s.ECRClientFactory.NewECRClient(cfg, s.FileSystem, s.Executor)
	}

	err = s.ECRClient.Login(context.TODO())
	if err != nil {
		return fmt.Errorf("ECR login failed: %w", err)
	}

	fmt.Println("Successfully logged in to AWS ECR")
	return nil
}
