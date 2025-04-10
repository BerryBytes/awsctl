package sso

import (
	"github.com/BerryBytes/awsctl/internal/sso"
	generalUtils "github.com/BerryBytes/awsctl/utils/general"

	"github.com/spf13/cobra"
)

func NewSSOCommands(ssoClient sso.SSOClient, generalManager generalUtils.GeneralUtilsInterface) *cobra.Command {
	ssoCmd := &cobra.Command{
		Use:   "sso",
		Short: "Manage AWS SSO configurations",
		Long:  "A set of commands to manage and configure AWS SSO profiles.",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			if err := generalManager.CheckAWSCLI(); err != nil {
				cmd.Println("Please install AWS CLI first: https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html")
				return err
			}
			return nil
		},
	}

	ssoCmd.AddCommand(InitCmd(ssoClient))
	ssoCmd.AddCommand(SetupCmd(ssoClient))

	return ssoCmd
}
