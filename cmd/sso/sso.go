package sso

import (
	"github.com/BerryBytes/awsctl/internal/sso"
	generalUtils "github.com/BerryBytes/awsctl/utils/general"

	"github.com/spf13/cobra"
)

type SSODependencies struct {
	Client         sso.SSOClient
	GeneralManager generalUtils.GeneralUtilsInterface
}

func NewSSOCommands(deps SSODependencies) *cobra.Command {
	ssoCmd := &cobra.Command{
		Use:   "sso",
		Short: "Manage AWS SSO configurations",
		Long:  "A set of commands to manage and configure AWS SSO profiles.",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			if err := deps.GeneralManager.CheckAWSCLI(); err != nil {
				cmd.Println("Please install AWS CLI first: https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html")
				return err
			}
			return nil
		},
	}

	ssoCmd.AddCommand(InitCmd(deps.Client))
	ssoCmd.AddCommand(SetupCmd(deps.Client))

	return ssoCmd
}
