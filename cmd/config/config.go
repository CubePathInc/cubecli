package config

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/CubePathInc/cubecli/internal/api"
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
		Short: "Set up CubeCLI configuration (adds or replaces the 'default' profile)",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Print("Enter your CubePath API token: ")
			scanner := bufio.NewScanner(os.Stdin)
			if !scanner.Scan() {
				return fmt.Errorf("failed to read input")
			}
			token := strings.TrimSpace(scanner.Text())
			if token == "" {
				return fmt.Errorf("API token cannot be empty")
			}

			profile := &internalConfig.Profile{APIToken: token}

			s := output.NewSpinner("Validating API token...")
			s.Start()
			client := api.NewClient(internalConfig.APIURL(profile), token)
			_, err := client.Get("/sshkey/user/sshkeys")
			s.Stop()
			if err != nil {
				return err
			}

			cfg := internalConfig.LoadOrEmpty()
			if cfg.Profiles == nil {
				cfg.Profiles = map[string]*internalConfig.Profile{}
			}
			if existing, ok := cfg.Profiles[internalConfig.DefaultProfileName]; ok {
				profile.APIURL = existing.APIURL
			}
			cfg.Profiles[internalConfig.DefaultProfileName] = profile
			if cfg.CurrentProfile == "" {
				cfg.CurrentProfile = internalConfig.DefaultProfileName
			}
			if err := internalConfig.Save(cfg); err != nil {
				return fmt.Errorf("failed to save config: %w", err)
			}

			output.PrintSuccess(fmt.Sprintf("Configuration saved to %s", internalConfig.Path()))
			output.PrintInfo(fmt.Sprintf("Active profile: %s", cfg.CurrentProfile))
			return nil
		},
	}

	configShowCmd := &cobra.Command{
		Use:   "show",
		Short: "Show current configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := internalConfig.LoadOrEmpty()
			explicit, _ := cmd.Flags().GetString("profile")
			profile, name, err := cfg.ActiveProfile(explicit)
			if err != nil {
				output.PrintError("No configuration found. Run 'cubecli config setup' or 'cubecli profile add <name>'.")
				return nil
			}

			masked := maskToken(profile.APIToken)

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(map[string]string{
					"active_profile": name,
					"api_token":      masked,
					"api_url":        internalConfig.APIURL(profile),
					"config_path":    internalConfig.Path(),
				})
			}

			t := output.NewTable("Configuration", []string{"Setting", "Value"})
			t.AddRow("Active Profile", name)
			t.AddRow("API Token", masked)
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
	if len(token) <= 12 {
		return "****"
	}
	return token[:8] + "..." + token[len(token)-4:]
}
