package bastion

import (
	"errors"

	"github.com/BerryBytes/awsctl/internal/bastion"
	promptutils "github.com/BerryBytes/awsctl/utils/prompt"
	"github.com/spf13/cobra"
)

func NewBastionCmd(bastionSvc bastion.BastionServiceInterface) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bastion",
		Short: "Interactive bastion host connection manager",
		Long: `Interactive menu for managing bastion host connections.
Choose between SSH access, SOCKS proxy, or port forwarding.`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			err := bastionSvc.Run()
			if errors.Is(err, promptutils.ErrInterrupted) {
				return nil
			}
			return err
		},
	}

	return cmd
}
