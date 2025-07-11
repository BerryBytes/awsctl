package sso

import (
	"errors"
	"fmt"
	"strings"

	"github.com/BerryBytes/awsctl/internal/sso"
	generalutils "github.com/BerryBytes/awsctl/utils/general"
	promptUtils "github.com/BerryBytes/awsctl/utils/prompt"

	"github.com/spf13/cobra"
)

func SetupCmd(ssoClient sso.SSOClient) *cobra.Command {
	var startURL string
	var region string
	var name string

	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Setup AWS SSO configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			if startURL != "" && !strings.HasPrefix(startURL, "https://") {
				return fmt.Errorf("invalid start URL: must begin with https://")
			}

			if region != "" {
				if !generalutils.IsRegionValid(region) {
					return fmt.Errorf("invalid AWS region: %s", region)
				}
			}

			if name != "" && !generalutils.IsValidSessionName(name) {
				return fmt.Errorf("invalid session name: must only contain letters, numbers, dashes, or underscores, and cannot start or end with a dash/underscore")
			}

			opts := sso.SSOFlagOptions{
				StartURL: startURL,
				Region:   region,
				Name:     name,
			}

			err := ssoClient.SetupSSO(opts)
			if err != nil {
				if errors.Is(err, promptUtils.ErrInterrupted) {
					return nil
				}
				return fmt.Errorf("SSO initialization failed: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "SSO session name")
	cmd.Flags().StringVar(&startURL, "start-url", "", "AWS SSO Start URL")
	cmd.Flags().StringVar(&region, "region", "", "AWS SSO Region")

	return cmd
}
