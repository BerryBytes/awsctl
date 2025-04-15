package root

import (
	"github.com/BerryBytes/awsctl/internal/sso"
	"github.com/BerryBytes/awsctl/utils/common"
	generalUtils "github.com/BerryBytes/awsctl/utils/general"

	bastionCmd "github.com/BerryBytes/awsctl/cmd/bastion"
	cmdSSO "github.com/BerryBytes/awsctl/cmd/sso"
	"github.com/BerryBytes/awsctl/internal/bastion"

	"github.com/spf13/cobra"
)

type RootDependencies struct {
	SSOClient      sso.SSOClient
	BastionService bastion.BastionServiceInterface
	GeneralManager generalUtils.GeneralUtilsInterface
	FileSystem     common.FileSystemInterface
}

func NewRootCmd(deps RootDependencies) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "awsctl",
		Short: "AWS CLI Tool",
		Long:  `A CLI tool for managing AWS services and configurations.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	rootCmd.AddCommand(cmdSSO.NewSSOCommands(cmdSSO.SSODependencies{
		Client:         deps.SSOClient,
		GeneralManager: deps.GeneralManager,
	}))

	rootCmd.AddCommand(bastionCmd.NewBastionCmd(bastionCmd.BastionDependencies{
		Service: deps.BastionService,
	}))

	return rootCmd
}
