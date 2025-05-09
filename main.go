package main

import (
	"context"
	"fmt"
	"os"

	"github.com/BerryBytes/awsctl/cmd/root"
	"github.com/BerryBytes/awsctl/internal/bastion"
	connection "github.com/BerryBytes/awsctl/internal/common"
	"github.com/BerryBytes/awsctl/internal/eks"
	"github.com/BerryBytes/awsctl/internal/rds"
	"github.com/BerryBytes/awsctl/internal/sso"
	"github.com/BerryBytes/awsctl/utils/common"
	generalutils "github.com/BerryBytes/awsctl/utils/general"
	promptUtils "github.com/BerryBytes/awsctl/utils/prompt"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2instanceconnect"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

func main() {
	awsClient := sso.DefaultAWSClient()
	ssoClient, err := sso.NewSSOClient(awsClient)
	if err != nil {
		fmt.Printf("Error initializing SSO client: %v\n", err)
		os.Exit(1)
	}

	generalManager := generalutils.NewGeneralUtilsManager()
	fileSystem := &common.RealFileSystem{}
	gPrompter := promptUtils.NewPrompt()

	ctx := context.TODO()
	awsConfig, _ := config.LoadDefaultConfig(ctx)

	ec2Client := connection.NewEC2Client(ec2.NewFromConfig(awsConfig))
	ssmClient := ssm.NewFromConfig(awsConfig)
	configLoader := &connection.DefaultAWSConfigLoader{}
	instanceConn := connection.NewEC2InstanceConnectAdapter(ec2instanceconnect.NewFromConfig(awsConfig))

	prompter := connection.NewConnectionPrompter()
	provider := connection.NewConnectionProvider(
		prompter,
		fileSystem,
		awsConfig,
		ec2Client,
		ssmClient,
		instanceConn,
		configLoader,
	)

	services := connection.NewServices(provider)
	bastionSvc := bastion.NewBastionService(services, prompter)
	sshExecutor := &common.RealSSHExecutor{}
	rdsSvc := rds.NewRDSService(
		services,
		func(s *rds.RDSService) {
			s.ConnProvider = provider
			s.SSHExecutor = sshExecutor
			s.Fs = fileSystem
			s.CPrompter = prompter
			s.GPrompter = gPrompter
		},
	)

	eksSvc := eks.NewEKSService(
		services,
		func(s *eks.EKSService) {
			s.ConnProvider = provider
			s.CPrompter = prompter
		},
	)

	rootCmd := root.NewRootCmd(root.RootDependencies{
		SSOClient:      ssoClient,
		BastionService: bastionSvc,
		GeneralManager: generalManager,
		FileSystem:     fileSystem,
		RDSService:     rdsSvc,
		EKSService:     eksSvc,
	})
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
