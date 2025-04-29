package rds

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	connection "github.com/BerryBytes/awsctl/internal/common"
	"github.com/BerryBytes/awsctl/internal/sso"
	"github.com/BerryBytes/awsctl/utils/common"
	promptUtils "github.com/BerryBytes/awsctl/utils/prompt"
	"github.com/aws/aws-sdk-go-v2/config"
)

type RDSService struct {
	RPrompter    RDSPromptInterface
	CPrompter    connection.ConnectionPrompter
	GPrompter    promptUtils.Prompter
	RDSClient    RDSAdapterInterface
	ConnServices connection.ServicesInterface
	ConnProvider *connection.ConnectionProvider
	SSHExecutor  common.SSHExecutorInterface
	Fs           common.FileSystemInterface
	socksPort    int
	OsDetector   common.OSDetector
}

func NewRDSService(
	connServices connection.ServicesInterface,
	opts ...func(*RDSService),
) *RDSService {
	prompter := promptUtils.NewPrompt()
	configClient := &sso.RealAWSConfigClient{Executor: &sso.RealCommandExecutor{}}

	service := &RDSService{
		RPrompter:    NewRPrompter(prompter, configClient),
		ConnServices: connServices,
		socksPort:    0,
	}

	for _, opt := range opts {
		opt(service)
	}

	return service
}

func (s *RDSService) Run() error {
	for {
		action, err := s.RPrompter.SelectRDSAction()
		if err != nil {
			if errors.Is(err, promptUtils.ErrInterrupted) {
				if s.socksPort != 0 {
					_ = s.CleanupSOCKS()

				}
				return promptUtils.ErrInterrupted
			}
			return fmt.Errorf("action selection aborted: %v", err)
		}

		switch action {
		case ConnectDirect:
			if err := s.HandleDirectConnection(); err != nil {
				return fmt.Errorf("direct connection failed: %w", err)
			}
			return nil
		case ConnectViaTunnel:
			if err := s.HandleTunnelConnection(); err != nil {
				return fmt.Errorf("tunnel connection failed: %w", err)
			}
			return nil
		case ConnectViaSOCKS:
			if err := s.HandleSOCKSConnection(); err != nil {
				return fmt.Errorf("SOCKS connection failed: %w", err)
			}
			fmt.Println("SOCKS proxy is running. Select 'Exit' to terminate it.")
		case ExitRDS:
			return nil
		}
	}
}

func (s *RDSService) HandleDirectConnection() error {
	endpoint, dbUser, region, err := s.getRDSConnectionDetails()
	if err != nil {
		return err
	}

	authToken, err := s.RDSClient.GenerateAuthToken(endpoint, dbUser, region)
	if err != nil {
		return fmt.Errorf("failed to generate RDS auth token: %w", err)
	}

	fmt.Printf("\nRDS Connection Details:\n")
	fmt.Printf("Endpoint: %s\n", endpoint)
	fmt.Printf("User: %s\n", dbUser)
	fmt.Printf("Auth Token: %s\n", authToken)
	fmt.Printf("You can now connect using your database client\n")

	return nil
}

func (s *RDSService) HandleTunnelConnection() error {
	rdsEndpoint, dbUser, region, err := s.getRDSConnectionDetails()
	if err != nil {
		return fmt.Errorf("failed to get RDS connection details: %w", err)
	}

	parts := strings.Split(rdsEndpoint, ":")
	if len(parts) != 2 {
		return fmt.Errorf("invalid RDS endpoint format: %s", rdsEndpoint)
	}
	remoteHost := parts[0]
	remotePort, err := strconv.Atoi(parts[1])
	if err != nil {
		return fmt.Errorf("invalid port in RDS endpoint: %w", err)
	}

	localPort, err := s.CPrompter.PromptForLocalPort("RDS", 3306)
	if err != nil {
		return fmt.Errorf("failed to get local port: %w", err)
	}

	authToken, err := s.RDSClient.GenerateAuthToken(rdsEndpoint, dbUser, region)
	if err != nil {
		return fmt.Errorf("failed to generate RDS auth token: %w", err)
	}

	fmt.Printf("\nSSH Tunnel Configuration:\n")
	fmt.Printf("Local port: %d\n", localPort)
	fmt.Printf("Forwarding to: %s:%d\n", remoteHost, remotePort)
	fmt.Printf("Use these credentials to connect via localhost:\n")
	fmt.Printf(" - Host: 127.0.0.1\n")
	fmt.Printf(" - Port: %d\n", localPort)
	fmt.Printf(" - User: %s\n", dbUser)
	fmt.Printf(" - Token: %s\n\n", authToken)
	fmt.Println("Starting port forwarding... Press Ctrl+C to stop.")

	err = s.ConnServices.StartPortForwarding(
		context.Background(),
		localPort,
		remoteHost,
		remotePort,
	)

	if err != nil {
		return fmt.Errorf("port forwarding failed: %w", err)
	}

	fmt.Println("Port forwarding terminated.")
	return nil
}

func (s *RDSService) HandleSOCKSConnection() error {
	// Get all connection details first
	rdsEndpoint, dbUser, _, err := s.getRDSConnectionDetails()
	if err != nil {
		return err
	}

	// Parse endpoint information
	parts := strings.Split(rdsEndpoint, ":")
	if len(parts) != 2 {
		return fmt.Errorf("invalid RDS endpoint format: %s", rdsEndpoint)
	}
	remoteHost := parts[0]
	remotePort, err := strconv.Atoi(parts[1])
	if err != nil {
		return fmt.Errorf("invalid port in RDS endpoint: %w", err)
	}

	// Get SOCKS port
	socksPort, err := s.CPrompter.PromptForSOCKSProxyPort(1080)
	if err != nil {
		return err
	}

	// authToken, err := s.RDSClient.GenerateAuthToken(rdsEndpoint, dbUser, region)
	// if err != nil {
	// 	return fmt.Errorf("failed to generate RDS auth token: %w", err)
	// }

	fmt.Printf("\nSOCKS Proxy Configuration:\n")
	fmt.Printf("SOCKS Proxy: 127.0.0.1:%d\n", socksPort)
	fmt.Printf("RDS Endpoint: %s:%d\n", remoteHost, remotePort)
	fmt.Printf("User: %s\n", dbUser)
	// fmt.Printf("Auth Token: %s\n\n", authToken)
	fmt.Println("Starting SOCKS proxy... Press Ctrl+C to stop.")

	s.socksPort = socksPort

	if err := s.ConnServices.StartSOCKSProxy(context.Background(), socksPort); err != nil {
		return fmt.Errorf("SOCKS proxy failed: %w", err)
	}

	return nil
}

func (s *RDSService) getRDSConnectionDetails() (endpoint, dbUser, region string, err error) {
	if !s.isAWSConfigured() {
		fmt.Println("AWS configuration not found - falling back to manual connection")
		return s.handleManualConnection()
	}
	confirm, err := s.CPrompter.PromptForConfirmation("Look for RDS instances in AWS?")
	if err != nil || !confirm {
		fmt.Println("Proceeding with manual connection")
		return s.handleManualConnection()
	}

	defaultRegion := ""
	if s.ConnProvider != nil {
		defaultRegion, err = s.ConnProvider.GetDefaultRegion()
		if err != nil {
			fmt.Printf("Failed to load default region: %v\n", err)
			defaultRegion = ""
		}
	}

	region, err = s.CPrompter.PromptForRegion(defaultRegion)
	if err != nil {
		fmt.Printf("Failed to get region: %v\n", err)
		fmt.Println("Proceeding with manual connection")
		return s.handleManualConnection()
	}

	profile, err := s.RPrompter.PromptForProfile()
	if err != nil {
		fmt.Printf("Failed to get AWS profile: %v\n", err)
		fmt.Println("Proceeding with manual connection")
		return s.handleManualConnection()
	}

	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(region),
		config.WithSharedConfigProfile(profile),
	)

	if err != nil {
		fmt.Printf("AWS config failed: %v\n", err)
		return s.handleManualConnection()
	}
	if s.RDSClient == nil {
		s.RDSClient = NewRDSClient(cfg, &sso.RealCommandExecutor{})
	}

	resources, err := s.RDSClient.ListRDSResources(context.TODO())
	if err != nil || len(resources) == 0 {
		fmt.Println("No RDS resources found")
		return s.handleManualConnection()
	}

	selected, err := s.RPrompter.PromptForRDSInstance(resources)
	if err != nil {
		return "", "", "", err
	}
	fmt.Printf("selected rds: %s", selected)
	// here endpoint will contain port as well
	endpoint, err = s.RDSClient.GetConnectionEndpoint(context.TODO(), selected)
	if err != nil {
		return "", "", "", err
	}

	dbUser, err = s.GPrompter.PromptForInput("Enter database username:", "")
	return endpoint, dbUser, region, err
}

func (s *RDSService) handleManualConnection() (string, string, string, error) {
	fmt.Println("Please enter connection details manually")
	// here endpoint will also have port(host:port)
	endpoint, dbUser, region, err := s.RPrompter.PromptForManualEndpoint()
	if err == nil {
		fmt.Printf("Endpoint: %s\nUser: %s\n", endpoint, dbUser)
	}
	return endpoint, dbUser, region, err
}

func (s *RDSService) CleanupSOCKS() error {
	if s.socksPort == 0 {
		return nil
	}
	err := common.TerminateSOCKSProxy(s.SSHExecutor, s.socksPort, s.OsDetector)
	if err == nil {
		fmt.Printf("SOCKS proxy on port %d terminated.\n", s.socksPort)
		s.socksPort = 0
	}
	return err
}

func (s *RDSService) isAWSConfigured() bool {
	if s.ConnServices == nil {
		return false
	}
	return s.ConnServices.IsAWSConfigured()
}

func (s *RDSService) SOCKSPort() int {
	return s.socksPort
}

func (s *RDSService) SetSOCKSPort(port int) {
	s.socksPort = port
}

func (s *RDSService) GetConnectionDetails() (string, string, string, error) {
	return s.getRDSConnectionDetails()
}
