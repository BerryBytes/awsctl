package bastion

import (
	"fmt"

	"github.com/spf13/cobra"
)

func NewBastionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bastion",
		Short: "Interactive bastion host connection manager",
		Long: `Interactive menu for managing bastion host connections.
Choose between SSH access, SOCKS proxy, or port forwarding.`,
		SilenceUsage: true,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Bastion command ran successfully.")
		},
	}

	return cmd
}
