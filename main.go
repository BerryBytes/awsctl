package main

import (
	"context"
	"fmt"
	"os"

	"github.com/BerryBytes/awsctl/cmd/root"
	"github.com/BerryBytes/awsctl/internal/bastion"
	"github.com/BerryBytes/awsctl/internal/sso"
	generalutils "github.com/BerryBytes/awsctl/utils/general"
	"github.com/aws/aws-sdk-go-v2/config"
)

func main() {
	awsClient := sso.DefaultAWSClient()

	bastionOpts := []func(*bastion.BastionService){}
	if _, err := config.LoadDefaultConfig(context.TODO()); err == nil {
		bastionOpts = append(bastionOpts, bastion.WithAWSConfig(context.TODO()))
	}

	bastionSvc := bastion.NewBastionService(bastionOpts...)

	ssoClient, err := sso.NewSSOClient(awsClient)
	generalManager := generalutils.NewGeneralUtilsManager()
	if err != nil {
		fmt.Printf("Error initializing SSO client: %v\n", err)
		os.Exit(1)
	}

	rootCmd := root.NewRootCmd(ssoClient, bastionSvc, generalManager)
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
