package root

import (
	cmdSSO "awsctl/cmd/sso"
	"awsctl/internal/sso"

	"github.com/spf13/cobra"
)

func NewRootCmd(ssoClient sso.SSOClient) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "awsctl",
		Short: "AWS CLI Tool",
		Long:  `A CLI tool for managing AWS services and configurations.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	rootCmd.AddCommand(cmdSSO.NewSSOCommands(ssoClient))
	return rootCmd
}
