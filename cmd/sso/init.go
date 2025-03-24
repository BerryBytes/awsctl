package sso

import (
	"awsctl/internal/sso"
	promptutils "awsctl/utils/prompt"
	"errors"
	"fmt"

	"github.com/spf13/cobra"
)

var refresh bool
var noBrowser bool

func InitCmd(ssoClient sso.SSOClient) *cobra.Command {
	initCmd := &cobra.Command{
		Use:   "init",
		Short: "Authenticate with AWS SSO",
		RunE: func(cmd *cobra.Command, args []string) error {
			refreshFlag, err := cmd.Flags().GetBool("refresh")
			if err != nil {
				return fmt.Errorf("could not get refresh flag: %w", err)
			}

			noBrowserFlag, err := cmd.Flags().GetBool("no-browser")
			if err != nil {
				return fmt.Errorf("could not get no-browser flag: %w", err)
			}

			err = ssoClient.InitSSO(refreshFlag, noBrowserFlag)
			if errors.Is(err, promptutils.ErrInterrupted) {
				return nil
			} else if err != nil {
				return fmt.Errorf("failed to set up AWS SSO: %w", err)
			}

			// fmt.Println("AWS SSO initialized successfully.")
			return nil
		},
	}

	initCmd.Flags().BoolVarP(&refresh, "refresh", "r", false, "Force SSO re-login")
	initCmd.Flags().BoolVar(&noBrowser, "no-browser", false, "Disable the browser-based login flow")

	return initCmd
}
