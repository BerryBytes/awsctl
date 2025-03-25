package main

import (
	"awsctl/cmd/root"
	"awsctl/internal/sso"
	"fmt"
	"os"
)

func main() {
	awsClient := sso.DefaultAWSClient()
	ssoClient, err := sso.NewSSOClient(awsClient)
	if err != nil {
		fmt.Printf("Error initializing SSO client: %v\n", err)
		os.Exit(1)
	}

	rootCmd := root.NewRootCmd(ssoClient)
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
