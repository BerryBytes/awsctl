package rds

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	connection "github.com/BerryBytes/awsctl/internal/common"
	"github.com/BerryBytes/awsctl/internal/sso"
	"github.com/BerryBytes/awsctl/utils/common"
	promptUtils "github.com/BerryBytes/awsctl/utils/prompt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/spf13/afero"
)

type RDSService struct {
	RPrompter           RDSPromptInterface
	CPrompter           connection.ConnectionPrompter
	GPrompter           promptUtils.Prompter
	RDSClient           RDSAdapterInterface
	ConnServices        connection.ServicesInterface
	ConnProvider        *connection.ConnectionProvider
	SSHExecutor         common.SSHExecutorInterface
	Fs                  common.FileSystemInterface
	socksPort           int
	OsDetector          common.OSDetector
	ConfigLoader        ConfigLoader
	RDSClientFactory    RDSClientFactory
	TerminateSOCKSProxy func(port int, protocol string) error
}

type RealConfigLoader struct{}

func (r *RealConfigLoader) LoadDefaultConfig(ctx context.Context, opts ...func(*config.LoadOptions) error) (aws.Config, error) {
	return config.LoadDefaultConfig(ctx, opts...)
}

type RealRDSClientFactory struct{}

func (r *RealRDSClientFactory) NewRDSClient(cfg aws.Config, executor sso.CommandExecutor) RDSAdapterInterface {
	return NewRDSClient(cfg, executor)
}

func NewRDSService(
	connServices connection.ServicesInterface,
	opts ...func(*RDSService),
) *RDSService {
	prompter := promptUtils.NewPrompt()
	configClient := &sso.RealAWSConfigClient{Executor: &sso.RealCommandExecutor{}}

	service := &RDSService{
		RPrompter:           NewRPrompter(prompter, configClient),
		ConnServices:        connServices,
		socksPort:           0,
		ConfigLoader:        &RealConfigLoader{},
		RDSClientFactory:    &RealRDSClientFactory{},
		TerminateSOCKSProxy: common.TerminateSOCKSProxy,
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
	authMethod, err := s.RPrompter.PromptForAuthMethod("Select authentication method for RDS:", []string{"Token", "Native password"})
	if err != nil {
		return fmt.Errorf("failed to get authentication method: %w", err)
	}
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

	var tempFiles []common.TempFile
	var rdsCleanup func()
	var mysqlCommand string

	if authMethod == "Token" {
		authToken, err := s.RDSClient.GenerateAuthToken(rdsEndpoint, dbUser, region)
		if err != nil {
			return fmt.Errorf("failed to generate RDS auth token: %w", err)
		}

		certPath, err := s.HandleSSLCertificate(region)
		if err != nil {
			return fmt.Errorf("failed to handle SSL certificate: %w", err)
		}

		configContent := fmt.Sprintf(`[client]
host=127.0.0.1
port=%d
user=%s
password=%s
ssl-ca=%s
`, localPort, dbUser, authToken, certPath)

		tmpFile, err := os.CreateTemp("", "mysql-config-*.cnf")
		if err != nil {
			return fmt.Errorf("failed to create temp config: %w", err)
		}
		defer func() {
			if err := tmpFile.Close(); err != nil {
				fmt.Fprintf(os.Stderr, "warning: failed to close tmp file: %v\n", err)
			}
		}()

		if _, err := tmpFile.WriteString(configContent); err != nil {
			return fmt.Errorf("failed to write config: %w", err)
		}

		if err := os.Chmod(tmpFile.Name(), 0600); err != nil {
			return fmt.Errorf("failed to set config file permissions: %w", err)
		}

		tempFiles = []common.TempFile{
			{Path: tmpFile.Name(), Desc: "temporary MySQL config"},
		}

		rdsCleanup = common.SetupCleanup(afero.NewOsFs(), tempFiles)
		mysqlCommand = fmt.Sprintf("\nRun this command:\nmysql --defaults-file=%s\n", tmpFile.Name())
	} else {
		rdsCleanup = func() {}
	}

	fmt.Printf("\nSSH Tunnel Configuration:\n")
	fmt.Printf("Local port: %d\n", localPort)
	fmt.Printf("Forwarding to: %s:%d\n", remoteHost, remotePort)
	fmt.Printf("Use these credentials to connect via localhost:\n")
	fmt.Printf(" - Host: 127.0.0.1\n")
	fmt.Printf(" - Port: %d\n", localPort)
	fmt.Printf(" - User: %s\n", dbUser)

	if authMethod == "Token" {
		fmt.Print(mysqlCommand)
		fmt.Println("\nNote: This temporary configuration will be deleted when port forwarding ends.")
	} else {
		fmt.Println("\nNote: Use your database client (e.g., mysql -h 127.0.0.1 -P <port> -u <user> -p) to connect.")
	}

	portForwardCleanup, stopPortForwarding, err := s.ConnServices.StartPortForwarding(context.Background(), localPort, remoteHost, remotePort)
	if err != nil {
		rdsCleanup()
		return fmt.Errorf("tunnel connection failed: port forwarding failed: %w", err)
	}

	cleanup := func() {
		stopPortForwarding()
		rdsCleanup()
		portForwardCleanup()
	}
	defer cleanup()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-sigChan:
		fmt.Println("Port forwarding session closed.")
		return nil
	case <-context.Background().Done():
		fmt.Println("Port forwarding session closed due to context cancellation.")
		return context.Background().Err()
	}
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

	if s.RDSClient == nil {
		cfg, err := s.ConfigLoader.LoadDefaultConfig(context.TODO(),
			config.WithRegion(region),
			config.WithSharedConfigProfile(profile),
		)
		if err != nil {
			fmt.Printf("AWS config failed: %v\n", err)
			return s.handleManualConnection()
		}
		s.RDSClient = s.RDSClientFactory.NewRDSClient(cfg, &sso.RealCommandExecutor{})
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
	err := s.TerminateSOCKSProxy(s.socksPort, "ssh")
	if err == nil {
		fmt.Printf("SOCKS proxy on port %d terminated.\n", s.socksPort)
		s.socksPort = 0
	}
	return err
}

func (s *RDSService) HandleSSLCertificate(region string) (string, error) {
	downloadChoice, err := s.GPrompter.PromptForSelection(
		"To connect securely, an RDS SSL certificate is required:",
		[]string{"Download certificate automatically", "Provide custom certificate path"},
	)
	if err != nil {
		if errors.Is(err, promptUtils.ErrInterrupted) {
			return "", fmt.Errorf("operation cancelled by user")
		}
		return "", fmt.Errorf("certificate selection failed: %w", err)
	}

	if downloadChoice == "Download certificate automatically" {
		certPath, err := DownloadSSLCertificate(s, region)
		if err != nil {
			return "", fmt.Errorf("failed to download certificate: %w", err)
		}
		return certPath, nil
	}
	rdsCertificate := fmt.Sprintf(".rds-certs/%s-bundle.pem", region)

	homeDir, err := s.Fs.UserHomeDir()
	if err != nil {
		homeDir = os.Getenv("HOME")
	}
	defaultCertPath := filepath.Join(homeDir, rdsCertificate)

	certPath, err := s.GPrompter.PromptForInput(
		"Enter path to your RDS SSL CA certificate file",
		defaultCertPath,
	)
	if err != nil {
		if errors.Is(err, promptUtils.ErrInterrupted) {
			return "", fmt.Errorf("operation cancelled by user")
		}
		return "", fmt.Errorf("certificate path input failed: %w", err)
	}

	if _, err := s.Fs.Stat(certPath); err != nil {
		return "", fmt.Errorf("certificate not found at %s: %w", certPath, err)
	}

	return certPath, nil
}

func DownloadSSLCertificate(s *RDSService, region string) (string, error) {
	certURL := fmt.Sprintf("https://truststore.pki.rds.amazonaws.com/%s/%s-bundle.pem", region, region)
	fmt.Printf("Downloading RDS SSL certificate from %s...\n", certURL)

	resp, err := http.Get(certURL)
	if err != nil {
		return "", fmt.Errorf("failed to download from %s: %w", certURL, err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to close response body: %v\n", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to download certificate: HTTP %d", resp.StatusCode)
	}

	certData, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read certificate: %w", err)
	}

	homeDir, err := s.Fs.UserHomeDir()
	if err != nil {
		homeDir = os.Getenv("HOME")
	}
	certDir := filepath.Join(homeDir, ".rds-certs")
	if err := s.Fs.MkdirAll(certDir, 0700); err != nil {
		return "", fmt.Errorf("failed to create cert directory: %w", err)
	}

	certPath := filepath.Join(certDir, fmt.Sprintf("%s-bundle.pem", region))
	if err := s.Fs.WriteFile(certPath, certData, 0600); err != nil {
		return "", fmt.Errorf("failed to write certificate: %w", err)
	}

	fmt.Printf("Certificate saved to: %s\n", certPath)
	return certPath, nil
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
