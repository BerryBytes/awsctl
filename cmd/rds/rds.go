package rds

import (
	"errors"

	"github.com/BerryBytes/awsctl/internal/rds"
	promptutils "github.com/BerryBytes/awsctl/utils/prompt"
	"github.com/spf13/cobra"
)

type RDSDependencies struct {
	Service rds.RDSServiceInterface
}

func NewRDSCmd(deps RDSDependencies) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rds",
		Short: "Interactive RDS connection manager",
		Long: `Interactive menu for managing RDS database connections.
Choose between direct connection, SSH tunnel, or SOCKS proxy.`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			err := deps.Service.Run()
			if errors.Is(err, promptutils.ErrInterrupted) {
				return nil
			}
			return err
		},
	}

	return cmd
}
