package bastion

import (
	"context"
	"errors"

	"github.com/BerryBytes/awsctl/internal/bastion"
	promptutils "github.com/BerryBytes/awsctl/utils/prompt"
	"github.com/spf13/cobra"
)

type BastionDependencies struct {
	Service bastion.BastionServiceInterface
}

func NewBastionCmd(deps BastionDependencies) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bastion",
		Short: "Interactive bastion host connection manager",
		Long: `Interactive menu for managing bastion host connections.
Choose between SSH access, SOCKS proxy, or port forwarding.`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			go func() {
				<-cmd.Context().Done()
				cancel()
			}()

			err := deps.Service.Run(ctx)
			if errors.Is(err, promptutils.ErrInterrupted) {
				return nil
			}
			return err
		},
	}

	return cmd
}
