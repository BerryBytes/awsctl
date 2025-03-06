package awsctl

import (
	"awsctl/internal/aws"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "awsctl",
	Short: "A CLI tool for AWS operations",
	Long:  `A Golang-based CLI for AWS tasks such as EKS, RDS, bastian and SSO authentication.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		fmt.Println("Checking if AWS CLI is installed...")
		aws.InitAWSSetup()
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("No subcommand provided. Showing help...")
		return cmd.Help() // Return the error instead of ignoring it
	},
}

// Execute runs the CLI application
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
