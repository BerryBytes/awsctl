package sso

import (
	"errors"
	"fmt"

	"github.com/BerryBytes/awsctl/internal/sso"
	promptUtils "github.com/BerryBytes/awsctl/utils/prompt"

	"github.com/spf13/cobra"
)

func SetupCmd(ssoClient sso.SSOClient) *cobra.Command {
	return &cobra.Command{
		Use:   "setup",
		Short: "Setup AWS SSO configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			err := ssoClient.SetupSSO()
			if err != nil {
				if errors.Is(err, promptUtils.ErrInterrupted) {
					return nil
				}
				return fmt.Errorf("SSO initialization failed: %w", err)
			}
			cmd.Println("AWS SSO setup completed successfully.")
			return nil
		},
	}
}
