package sso

import (
	"awsctl/internal/sso"
	promptutils "awsctl/utils/prompt"
	"errors"
	"fmt"

	"github.com/spf13/cobra"
)

func SetupCmd(ssoClient sso.SSOClient) *cobra.Command {
	setupCmd := &cobra.Command{
		Use:   "setup",
		Short: "Setup AWS SSO configuration",
		Long:  `Configure or modify AWS SSO settings.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("Setting up AWS SSO...")

			if err := ssoClient.SetupSSO(); err != nil {
				if errors.Is(err, promptutils.ErrInterrupted) {
					return nil
				}
				return fmt.Errorf("error setting up AWS SSO: %v", err)
			}
			return nil

		},
	}
	return setupCmd
}
