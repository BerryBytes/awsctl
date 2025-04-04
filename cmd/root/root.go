package root

import (
	"github.com/BerryBytes/awsctl/internal/sso"

	bastionCmd "github.com/BerryBytes/awsctl/cmd/bastion"
	cmdSSO "github.com/BerryBytes/awsctl/cmd/sso"
	"github.com/BerryBytes/awsctl/internal/bastion"

	"github.com/spf13/cobra"
)

func NewRootCmd(ssoClient sso.SSOClient, bastionSvc bastion.BastionServiceInterface) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "awsctl",
		Short: "AWS CLI Tool",
		Long:  `A CLI tool for managing AWS services and configurations.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	rootCmd.AddCommand(cmdSSO.NewSSOCommands(ssoClient))
	rootCmd.AddCommand(bastionCmd.NewBastionCmd(bastionSvc))

	return rootCmd
}
