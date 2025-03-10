package awsctl

import (
	"awsctl/internal/aws"
	"fmt"

	"github.com/spf13/cobra"
)

var refresh bool
var noBrowser bool

var ssoCmd = &cobra.Command{
	Use:   "sso",
	Short: "AWS SSO Initialization and setup",
	Long:  `This command manages AWS SSO initialization and setup.`,
}

var ssoInitCmd = &cobra.Command{
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
		return aws.SsoRun(refreshFlag, noBrowserFlag)
	},
}

var ssoSetupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Setup AWS SSO in ~/.aws/config",
	Long:  "This command configures AWS SSO authentication by adding profiles to ~/.aws/config.",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("🔧 Setting up AWS SSO...")

		if err := aws.SetupSSO(); err != nil {
			return fmt.Errorf("❌ Error setting up AWS SSO: %w", err)
		}

		fmt.Println("✅ AWS SSO setup completed successfully!")
		return nil
	},
}

func init() {
	// Add the subcommands to the sso command
	ssoCmd.AddCommand(ssoInitCmd)
	ssoCmd.AddCommand(ssoSetupCmd)
	ssoCmd.PersistentFlags().BoolVarP(&refresh, "refresh", "r", false, "Force SSO re-login")
	ssoCmd.PersistentFlags().BoolVar(&noBrowser, "no-browser", false, "Disable the browser-based login flow")

	// Add the ssoCmd to the root command
	rootCmd.AddCommand(ssoCmd)
}
