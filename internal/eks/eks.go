package eks

import (
	"context"
	"fmt"
	"strings"
	"time"

	connection "github.com/BerryBytes/awsctl/internal/common"
	"github.com/BerryBytes/awsctl/internal/sso"
	"github.com/BerryBytes/awsctl/models"
	"github.com/BerryBytes/awsctl/utils/common"
	promptUtils "github.com/BerryBytes/awsctl/utils/prompt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
)

type EKSService struct {
	EPrompter        EKSPromptInterface
	CPrompter        connection.ConnectionPrompter
	EKSClient        EKSAdapterInterface
	ConnServices     connection.ServicesInterface
	ConnProvider     *connection.ConnectionProvider
	ConfigLoader     ConfigLoader
	EKSClientFactory EKSClientFactory
	FileSystem       common.FileSystemInterface
}
type RealConfigLoader struct{}

func (r *RealConfigLoader) LoadDefaultConfig(ctx context.Context, opts ...func(*config.LoadOptions) error) (aws.Config, error) {
	cfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return aws.Config{}, err
	}
	return aws.Config{
		Region:      cfg.Region,
		Credentials: cfg.Credentials,
	}, nil
}

type RealEKSClientFactory struct{}

func (r *RealEKSClientFactory) NewEKSClient(cfg aws.Config, fs common.FileSystemInterface) EKSAdapterInterface {
	return NewEKSClient(cfg, fs)
}

func NewEKSService(
	connServices connection.ServicesInterface,
	opts ...func(*EKSService),
) *EKSService {
	prompter := promptUtils.NewPrompt()
	configClient := &sso.RealSSOClient{Executor: &common.RealCommandExecutor{}}

	service := &EKSService{
		EPrompter:        NewEPrompter(prompter, configClient),
		ConnServices:     connServices,
		ConfigLoader:     &RealConfigLoader{},
		EKSClientFactory: &RealEKSClientFactory{},
		FileSystem:       &common.RealFileSystem{},
	}

	for _, opt := range opts {
		opt(service)
	}

	return service
}

func (s *EKSService) Run() error {
	return s.HandleKubeconfigUpdate()
}

func (s *EKSService) HandleKubeconfigUpdate() error {
	cluster, profile, err := s.GetEKSClusterDetails()
	if err != nil {
		return err
	}

	if err := s.EKSClient.UpdateKubeconfig(cluster, profile); err != nil {
		return fmt.Errorf("failed to update kubeconfig: %w", err)
	}

	fmt.Printf("\nKubeconfig updated successfully for cluster: %s\n", cluster.ClusterName)
	fmt.Printf("Cluster Endpoint: %s\n", cluster.Endpoint)
	fmt.Printf("Region: %s\n", cluster.Region)
	fmt.Printf("You can now use `kubectl` with context: %s\n", cluster.ClusterName)

	return nil
}

func (s *EKSService) GetEKSClusterDetails() (*models.EKSCluster, string, error) {
	if !s.ConnServices.IsAWSConfigured() {
		fmt.Println("AWS configuration not found - falling back to manual input")
		return s.HandleManualCluster()
	}

	defaultRegion := ""
	if s.ConnProvider != nil {
		var err error
		defaultRegion, err = s.ConnProvider.GetDefaultRegion()
		if err != nil {
			fmt.Printf("Failed to load default region: %v\n", err)
			defaultRegion = ""
		}
	}

	region, err := s.CPrompter.PromptForRegion(defaultRegion)
	if err != nil {
		fmt.Printf("Failed to get region: %v\n", err)
		fmt.Println("Proceeding with manual input")
		return s.HandleManualCluster()
	}

	profile, err := s.EPrompter.PromptForProfile()
	if err != nil {
		fmt.Printf("Failed to get AWS profile: %v\n", err)
		fmt.Println("Proceeding with manual input")
		return s.HandleManualCluster()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Load aws.Config with region and profile
	awsCfg, err := s.ConfigLoader.LoadDefaultConfig(ctx,
		config.WithRegion(region),
		config.WithSharedConfigProfile(profile),
	)
	if err != nil {
		fmt.Printf("AWS config failed: %v\n", err)
		if strings.Contains(err.Error(), "SSO") || strings.Contains(err.Error(), "token") {
			fmt.Println("Please run 'awsctl sso init' to authenticate")
		}
		return s.HandleManualCluster()
	}

	// Check credentials
	_, err = awsCfg.Credentials.Retrieve(ctx)
	if err != nil {
		fmt.Printf("Failed to retrieve credentials for profile %s: %v\n", profile, err)
		if strings.Contains(err.Error(), "SSO") || strings.Contains(err.Error(), "token") {
			fmt.Println("Please run 'awsctl sso init' to authenticate")
		}
		return s.HandleManualCluster()
	}

	s.EKSClient = s.EKSClientFactory.NewEKSClient(awsCfg, s.FileSystem)

	clusters, err := s.EKSClient.ListEKSClusters(context.TODO())
	if err != nil || len(clusters) == 0 {
		fmt.Println("No EKS clusters found")
		return s.HandleManualCluster()
	}

	selected, err := s.EPrompter.PromptForEKSCluster(clusters)
	if err != nil {
		return nil, "", err
	}

	for _, cluster := range clusters {
		if cluster.ClusterName == selected {
			return &cluster, profile, nil
		}
	}

	return nil, "", fmt.Errorf("selected cluster not found")
}

func (s *EKSService) HandleManualCluster() (*models.EKSCluster, string, error) {
	fmt.Println("Please enter EKS cluster details manually")
	clusterName, endpoint, caData, region, err := s.EPrompter.PromptForManualCluster()
	if err != nil {
		return nil, "", err
	}

	cluster := &models.EKSCluster{
		ClusterName:              clusterName,
		Endpoint:                 endpoint,
		Region:                   region,
		CertificateAuthorityData: caData,
	}

	fmt.Printf("Cluster Name: %s\nEndpoint: %s\nRegion: %s\n", clusterName, endpoint, region)
	return cluster, "", nil
}

func (s *EKSService) IsAWSConfigured() bool {
	if s.ConnServices == nil {
		return false
	}
	return s.ConnServices.IsAWSConfigured()
}
