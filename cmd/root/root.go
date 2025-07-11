package root

import (
	"github.com/BerryBytes/awsctl/internal/ecr"
	"github.com/BerryBytes/awsctl/internal/eks"
	"github.com/BerryBytes/awsctl/internal/rds"
	"github.com/BerryBytes/awsctl/internal/sso"
	"github.com/BerryBytes/awsctl/utils/common"
	generalUtils "github.com/BerryBytes/awsctl/utils/general"

	bastionCmd "github.com/BerryBytes/awsctl/cmd/bastion"
	ecrCmd "github.com/BerryBytes/awsctl/cmd/ecr"
	eksCmd "github.com/BerryBytes/awsctl/cmd/eks"
	rdsCmd "github.com/BerryBytes/awsctl/cmd/rds"

	cmdSSO "github.com/BerryBytes/awsctl/cmd/sso"
	"github.com/BerryBytes/awsctl/internal/bastion"

	"github.com/spf13/cobra"
)

type RootDependencies struct {
	SSOSetupClient sso.SSOClient
	BastionService bastion.BastionServiceInterface
	GeneralManager generalUtils.GeneralUtilsInterface
	FileSystem     common.FileSystemInterface
	RDSService     rds.RDSServiceInterface
	EKSService     eks.EKSServiceInterface
	ECRService     ecr.ECRServiceInterface
	Version        string
}

func NewRootCmd(deps RootDependencies) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "awsctl",
		Short: "AWS CLI Tool",
		Long:  `A CLI tool for managing AWS services and configurations.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
		Version: deps.Version,
	}
	rootCmd.SetVersionTemplate(`{{printf "%s version %s\n" .Name .Version}}`)

	rootCmd.AddCommand(cmdSSO.NewSSOCommands(cmdSSO.SSODependencies{
		SetupClient:    deps.SSOSetupClient,
		GeneralManager: deps.GeneralManager,
	}))

	rootCmd.AddCommand(bastionCmd.NewBastionCmd(bastionCmd.BastionDependencies{
		Service: deps.BastionService,
	}))

	rootCmd.AddCommand(rdsCmd.NewRDSCmd(rdsCmd.RDSDependencies{
		Service: deps.RDSService,
	}))

	rootCmd.AddCommand(eksCmd.NewEKSCmd(eksCmd.EKSDependencies{
		Service: deps.EKSService,
	}))

	rootCmd.AddCommand(ecrCmd.NewECRCmd(ecrCmd.ECRDependencies{
		Service: deps.ECRService,
	}))

	return rootCmd
}
