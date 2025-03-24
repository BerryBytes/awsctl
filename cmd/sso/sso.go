package sso

import (
	"awsctl/internal/sso"
	generalutils "awsctl/utils/general"
	"fmt"

	"github.com/spf13/cobra"
)

func NewSSOCommands(ssoClient sso.SSOClient) *cobra.Command {
	ssoCmd := &cobra.Command{
		Use:   "sso",
		Short: "Manage AWS SSO configurations",
		Long:  `A set of commands to manage and configure AWS SSO profiles.`,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			generalManager := generalutils.NewGeneralUtilsManager()
			if err := generalManager.CheckAWSCLI(); err != nil {
				fmt.Println("Error:", err)
				fmt.Println("Please install AWS CLI first. Visit https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html")
				return err
			}
			fmt.Println("AWS CLI is installed and available in PATH.")
			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {

			if err := cmd.Help(); err != nil {
				fmt.Println("Error displaying help:", err)
			}
		},
	}

	// Add subcommands
	ssoCmd.AddCommand(InitCmd(ssoClient))
	ssoCmd.AddCommand(SetupCmd(ssoClient))

	return ssoCmd
}
