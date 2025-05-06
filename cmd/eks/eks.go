package eks

import (
	"errors"

	"github.com/BerryBytes/awsctl/internal/eks"
	promptutils "github.com/BerryBytes/awsctl/utils/prompt"
	"github.com/spf13/cobra"
)

type EKSDependencies struct {
	Service eks.EKSServiceInterface
}

func NewEKSCmd(deps EKSDependencies) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "eks",
		Short: "Interactive EKS cluster manager",
		Long: `Interactive menu for managing EKS cluster configurations.
Supports updating kubeconfig for EKS clusters.`,
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
