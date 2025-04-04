package main

import (
	"context"
	"fmt"
	"os"

	"github.com/BerryBytes/awsctl/cmd/root"
	"github.com/BerryBytes/awsctl/internal/bastion"
	"github.com/BerryBytes/awsctl/internal/sso"
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
	if err != nil {
		fmt.Printf("Error initializing SSO client: %v\n", err)
		os.Exit(1)
	}

	rootCmd := root.NewRootCmd(ssoClient, bastionSvc)
	if err := rootCmd.Execute(); err != nil {
		// check: commented below code because the error was logged or printed twice
		// fmt.Println(err)
		os.Exit(1)
	}
}
