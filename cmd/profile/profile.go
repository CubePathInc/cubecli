package profile

import (
	"fmt"
	"os"
	"strings"

	authcmd "github.com/CubePathInc/cubecli/cmd/auth"
	"github.com/CubePathInc/cubecli/internal/cmdutil"
	internalConfig "github.com/CubePathInc/cubecli/internal/config"
	"github.com/CubePathInc/cubecli/internal/output"
	"github.com/spf13/cobra"
)

func NewCmd() *cobra.Command {
	profileCmd := &cobra.Command{
		Use:     "profile",
		Short:   "Manage authentication profiles",
		Aliases: []string{"profiles"},
	}

	profileCmd.AddCommand(
		newAddCmd(),
		newListCmd(),
		newUseCmd(),
		newCurrentCmd(),
		newDeleteCmd(),
		newRenameCmd(),
	)
	return profileCmd
}

func newAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Add a new profile and log it in (browser login, or --token)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := strings.TrimSpace(args[0])
			if name == "" {
				return fmt.Errorf("profile name cannot be empty")
			}

			cfg := internalConfig.LoadOrEmpty()
			if _, exists := cfg.Profiles[name]; exists {
				replace, _ := cmd.Flags().GetBool("force")
				if !replace && !cmdutil.ConfirmAction(fmt.Sprintf("Profile %q already exists. Replace?", name)) {
					output.PrintWarning("Aborted")
					return nil
				}
			}

			apiURL, _ := cmd.Flags().GetString("api-url")
			useToken, _ := cmd.Flags().GetBool("token")
			noBrowser, _ := cmd.Flags().GetBool("no-browser")
			if useToken || !cmdutil.StdinIsTerminal() {
				return authcmd.TokenLogin(cmd, name, apiURL, false)
			}
			return authcmd.BrowserLogin(cmd, name, apiURL, noBrowser, false)
		},
	}
	cmd.Flags().String("api-url", "", "Override API URL for this profile")
	cmd.Flags().Bool("token", false, "Store an API token instead of logging in with the browser")
	cmd.Flags().Bool("no-browser", false, "Print the login URL instead of opening a browser")
	cmd.Flags().BoolP("force", "f", false, "Replace profile without confirmation if it already exists")
	return cmd
}

func newListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List configured profiles",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := internalConfig.LoadOrEmpty()
			names := cfg.ProfileNames()

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(map[string]interface{}{
					"current":  cfg.CurrentProfile,
					"profiles": names,
				})
			}

			if len(names) == 0 {
				output.PrintWarning("No profiles configured. Run 'cubecli login'.")
				return nil
			}

			anyCustomURL := false
			for _, n := range names {
				if cfg.Profiles[n].APIURL != "" && cfg.Profiles[n].APIURL != internalConfig.DefaultAPIURL {
					anyCustomURL = true
					break
				}
			}

			headers := []string{"Active", "Name"}
			if anyCustomURL {
				headers = []string{"Active", "Name", "API URL"}
			}
			t := output.NewTable("Profiles", headers)
			for _, n := range names {
				p := cfg.Profiles[n]
				active := ""
				if n == cfg.CurrentProfile {
					active = "*"
				}
				if anyCustomURL {
					apiURL := p.APIURL
					if apiURL == "" {
						apiURL = internalConfig.DefaultAPIURL
					}
					t.AddRow(active, n, apiURL)
				} else {
					t.AddRow(active, n)
				}
			}
			t.Render()
			return nil
		},
	}
	return cmd
}

func newUseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "use <name>",
		Short: "Switch the active profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			cfg := internalConfig.LoadOrEmpty()
			if _, ok := cfg.Profiles[name]; !ok {
				return fmt.Errorf("profile %q not found", name)
			}
			cfg.CurrentProfile = name
			if err := internalConfig.Save(cfg); err != nil {
				return err
			}
			output.PrintSuccess(fmt.Sprintf("Active profile: %s", name))
			return nil
		},
	}
}

func newCurrentCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "current",
		Short: "Show the active profile",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := internalConfig.LoadOrEmpty()
			name := cfg.ActiveProfileName("")
			if _, ok := cfg.Profiles[name]; !ok && os.Getenv("CUBE_API_TOKEN") == "" {
				return fmt.Errorf("no active profile configured")
			}
			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(map[string]string{"profile": name})
			}
			fmt.Println(name)
			return nil
		},
	}
}

func newDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "delete <name>",
		Aliases: []string{"rm"},
		Short:   "Delete a profile",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			cfg := internalConfig.LoadOrEmpty()
			p, ok := cfg.Profiles[name]
			if !ok {
				return fmt.Errorf("profile %q not found", name)
			}
			if !cmdutil.CheckForce(cmd, fmt.Sprintf("Delete profile %q?", name)) {
				output.PrintWarning("Aborted")
				return nil
			}
			if p.OAuth != nil {
				authcmd.RevokeSession(p.OAuth)
			}
			delete(cfg.Profiles, name)
			if cfg.CurrentProfile == name {
				cfg.CurrentProfile = ""
				for other := range cfg.Profiles {
					cfg.CurrentProfile = other
					break
				}
			}
			if err := internalConfig.Save(cfg); err != nil {
				return err
			}
			output.PrintSuccess(fmt.Sprintf("Profile %q deleted", name))
			if cfg.CurrentProfile != "" {
				output.PrintInfo(fmt.Sprintf("Active profile: %s", cfg.CurrentProfile))
			}
			return nil
		},
	}
	cmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")
	return cmd
}

func newRenameCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rename <old> <new>",
		Short: "Rename a profile",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			old, newName := args[0], args[1]
			cfg := internalConfig.LoadOrEmpty()
			p, ok := cfg.Profiles[old]
			if !ok {
				return fmt.Errorf("profile %q not found", old)
			}
			if _, exists := cfg.Profiles[newName]; exists {
				return fmt.Errorf("profile %q already exists", newName)
			}
			delete(cfg.Profiles, old)
			cfg.Profiles[newName] = p
			if cfg.CurrentProfile == old {
				cfg.CurrentProfile = newName
			}
			if err := internalConfig.Save(cfg); err != nil {
				return err
			}
			output.PrintSuccess(fmt.Sprintf("Profile renamed: %s → %s", old, newName))
			return nil
		},
	}
}
