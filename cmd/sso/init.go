package sso

import (
	"errors"
	"fmt"

	"github.com/BerryBytes/awsctl/internal/sso"
	promptutils "github.com/BerryBytes/awsctl/utils/prompt"

	"github.com/spf13/cobra"
)

var refresh bool
var noBrowser bool

func InitCmd(ssoClient sso.SSOClient) *cobra.Command {
	initCmd := &cobra.Command{
		Use:   "init",
		Short: "Authenticate with AWS SSO",
		RunE: func(cmd *cobra.Command, args []string) error {
			refresh, _ := cmd.Flags().GetBool("refresh")
			noBrowser, _ := cmd.Flags().GetBool("no-browser")

			err := ssoClient.InitSSO(refresh, noBrowser)
			if err != nil {
				if errors.Is(err, promptutils.ErrInterrupted) {
					return nil
				}
				return fmt.Errorf("SSO initialization failed: %w", err)
			}
			cmd.Println("AWS SSO init completed successfully.")

			return nil
		},
	}

	initCmd.Flags().BoolVarP(&refresh, "refresh", "r", false, "Force SSO re-login")
	initCmd.Flags().BoolVar(&noBrowser, "no-browser", false, "Disable the browser-based login flow")
	return initCmd
}
