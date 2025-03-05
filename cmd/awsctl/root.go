package awsctl

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "your-app-name",
	Short: "A CLI tool for AWS operations",
	Long:  `A Golang-based CLI for AWS tasks such as EKS, RDS, and SSO authentication.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("No subcommand provided. Showing help...")
		cmd.Help()
	},
}

// Execute runs the CLI application
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
