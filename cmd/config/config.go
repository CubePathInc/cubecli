package config

import (
	authcmd "github.com/CubePathInc/cubecli/cmd/auth"
	"github.com/CubePathInc/cubecli/internal/cmdutil"
	internalConfig "github.com/CubePathInc/cubecli/internal/config"
	"github.com/CubePathInc/cubecli/internal/output"
	"github.com/spf13/cobra"
)

func NewCmd() *cobra.Command {
	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Configure CubeCLI",
	}

	configSetupCmd := &cobra.Command{
		Use:   "setup",
		Short: "Log in the 'default' profile (same as 'cubecli login default')",
		RunE: func(cmd *cobra.Command, args []string) error {
			useToken, _ := cmd.Flags().GetBool("token")
			noBrowser, _ := cmd.Flags().GetBool("no-browser")
			name := internalConfig.DefaultProfileName
			if useToken || !cmdutil.StdinIsTerminal() {
				return authcmd.TokenLogin(cmd, name, "", false)
			}
			return authcmd.BrowserLogin(cmd, name, "", noBrowser, false)
		},
	}
	configSetupCmd.Flags().Bool("token", false, "Store an API token instead of logging in with the browser")
	configSetupCmd.Flags().Bool("no-browser", false, "Print the login URL instead of opening a browser")

	configShowCmd := &cobra.Command{
		Use:   "show",
		Short: "Show current configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := internalConfig.LoadOrEmpty()
			explicit, _ := cmd.Flags().GetString("profile")
			profile, name, err := cfg.ActiveProfile(explicit)
			if err != nil {
				output.PrintError("No configuration found. Run 'cubecli login'.")
				return nil
			}

			credential := maskToken(profile.APIToken)
			if profile.OAuth != nil {
				credential = "browser login"
				if profile.OAuth.Email != "" {
					credential += " (" + profile.OAuth.Email + ")"
				}
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(map[string]string{
					"active_profile": name,
					"auth":           profile.AuthMethod(),
					"api_token":      maskToken(profile.APIToken),
					"api_url":        internalConfig.APIURL(profile),
					"config_path":    internalConfig.Path(),
				})
			}

			t := output.NewTable("Configuration", []string{"Setting", "Value"})
			t.AddRow("Active Profile", name)
			t.AddRow("Credentials", credential)
			t.AddRow("API URL", internalConfig.APIURL(profile))
			t.AddRow("Config Path", internalConfig.Path())
			t.Render()
			return nil
		},
	}

	configCmd.AddCommand(configSetupCmd, configShowCmd)
	return configCmd
}

func maskToken(token string) string {
	if token == "" {
		return ""
	}
	if len(token) <= 12 {
		return "****"
	}
	return token[:8] + "..." + token[len(token)-4:]
}
