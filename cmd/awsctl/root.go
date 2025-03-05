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
	// Define the Run function to show help when no subcommand is provided
	Run: func(cmd *cobra.Command, args []string) {
		// Print a message before showing the help
		fmt.Println("No subcommand provided. Showing help...")
		cmd.Help() // Display the help message for the root command
	},
}

// Execute runs the CLI application
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
