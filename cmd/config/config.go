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
		Short: "Set up CubeCLI configuration",
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

			// Validate token
			s := output.NewSpinner("Validating API token...")
			s.Start()
			client := api.NewClient(internalConfig.APIURL(), token)
			_, err := client.Get("/sshkey/user/sshkeys")
			s.Stop()
			if err != nil {
				return err
			}

			cfg := &internalConfig.Config{APIToken: token}
			if err := internalConfig.Save(cfg); err != nil {
				return fmt.Errorf("failed to save config: %w", err)
			}

			output.PrintSuccess(fmt.Sprintf("Configuration saved to %s", internalConfig.Path()))
			return nil
		},
	}

	configShowCmd := &cobra.Command{
		Use:   "show",
		Short: "Show current configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := internalConfig.Load()
			if err != nil {
				output.PrintError("No configuration found. Run 'cubecli config setup' to configure.")
				return nil
			}

			token := cfg.APIToken
			masked := maskToken(token)

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(map[string]string{
					"api_token":   masked,
					"api_url":     internalConfig.APIURL(),
					"config_path": internalConfig.Path(),
				})
			}

			t := output.NewTable("Configuration", []string{"Setting", "Value"})
			t.AddRow("API Token", masked)
			t.AddRow("API URL", internalConfig.APIURL())
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
