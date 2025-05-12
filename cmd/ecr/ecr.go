package ecr

import (
	"errors"

	"github.com/BerryBytes/awsctl/internal/ecr"
	promptutils "github.com/BerryBytes/awsctl/utils/prompt"
	"github.com/spf13/cobra"
)

type ECRDependencies struct {
	Service ecr.ECRServiceInterface
}

func NewECRCmd(deps ECRDependencies) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ecr",
		Short: "Interactive AWS ECR login manager",
		Long: `Interactive menu for logging into AWS Elastic Container Registry (ECR).
Supports authentication to ECR repositories.`,
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
