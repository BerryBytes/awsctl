package main

import (
	"context"
	"fmt"
	"os"

	"github.com/BerryBytes/awsctl/cmd/root"
	"github.com/BerryBytes/awsctl/internal/bastion"
	connection "github.com/BerryBytes/awsctl/internal/common"
	"github.com/BerryBytes/awsctl/internal/sso"
	"github.com/BerryBytes/awsctl/utils/common"
	generalutils "github.com/BerryBytes/awsctl/utils/general"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2instanceconnect"
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

	ctx := context.TODO()
	awsConfig, _ := config.LoadDefaultConfig(ctx)

	var ec2Client connection.EC2ClientInterface
	var instanceConn connection.EC2InstanceConnectInterface

	if awsConfig.Region != "" {
		ec2Client = connection.NewEC2Client(ec2.NewFromConfig(awsConfig))
		instanceConn = connection.NewEC2InstanceConnectAdapter(ec2instanceconnect.NewFromConfig(awsConfig))
	}

	prompter := connection.NewConnectionPrompter()
	provider := connection.NewConnectionProvider(
		prompter,
		fileSystem,
		&awsConfig,
		ec2Client,
		nil,
		instanceConn,
	)

	services := connection.NewServices(provider)
	bastionSvc := bastion.NewBastionService(services, prompter)

	rootCmd := root.NewRootCmd(root.RootDependencies{
		SSOClient:      ssoClient,
		BastionService: bastionSvc,
		GeneralManager: generalManager,
		FileSystem:     fileSystem,
	})

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
