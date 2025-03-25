package sso

import (
	"fmt"

	"github.com/BerryBytes/awsctl/internal/sso"
	generalutils "github.com/BerryBytes/awsctl/utils/general"

	"github.com/spf13/cobra"
)

func NewSSOCommands(ssoClient sso.SSOClient) *cobra.Command {
	ssoCmd := &cobra.Command{
		Use:   "sso",
		Short: "Manage AWS SSO configurations",
		Long:  "A set of commands to manage and configure AWS SSO profiles.",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			generalManager := generalutils.NewGeneralUtilsManager()
			if err := generalManager.CheckAWSCLI(); err != nil {
				fmt.Println("Error:", err)
				fmt.Println("Please install AWS CLI first: https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html")
				return err
			}
			fmt.Println("AWS CLI is installed and available in PATH.")
			return nil
		},
	}

	ssoCmd.AddCommand(InitCmd(ssoClient))
	ssoCmd.AddCommand(SetupCmd(ssoClient))

	return ssoCmd
}
