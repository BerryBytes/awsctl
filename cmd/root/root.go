package root

import (
	cmdSSO "awsctl/cmd/sso"
	clientSSO "awsctl/internal/sso"
	"fmt"

	"github.com/spf13/cobra"
)

var RootCmd = &cobra.Command{
	Use:   "awsctl",
	Short: "AWS CLI Tool",
	Long:  `A CLI tool for managing AWS services and configurations.`,
	RunE: func(cmd *cobra.Command, args []string) error {

		fmt.Println("No subcommand provided. Showing help...")
		return cmd.Help()
	},
}

func init() {
	awsClient := clientSSO.DefaultAWSClient()

	// Initialize the SSO client
	ssoClient, err := clientSSO.NewSSOClient(awsClient)
	if err != nil {
		fmt.Printf("Error initializing SSO client: %v\n", err)
		return
	}

	RootCmd.AddCommand(cmdSSO.NewSSOCommands(ssoClient))
}
