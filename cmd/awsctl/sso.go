package awsctl

import (
	"fmt"

	"github.com/spf13/cobra"
)

var refresh bool

var ssoCmd = &cobra.Command{
	Use:   "sso",
	Short: "AWS SSO Initialization and setup",
	Long:  `This command manages AWS SSO initialization and setup.`,
}

var ssoInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Authenticate with AWS SSO",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Init command run successfully!!!")
		return nil
	},
}

var ssoSetupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Setup AWS SSO in ~/.aws/config",
	Long:  "This command configures AWS SSO authentication by adding profiles to ~/.aws/config.",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Setup command run successfully!!!")
		return nil
	},
}

func init() {
	// Add the subcommands to the sso command
	ssoCmd.AddCommand(ssoInitCmd)
	ssoCmd.AddCommand(ssoSetupCmd)
	ssoCmd.Flags().BoolVarP(&refresh, "refresh", "r", false, "Force SSO re-login")

	// Add the ssoCmd to the root command
	rootCmd.AddCommand(ssoCmd)
}
