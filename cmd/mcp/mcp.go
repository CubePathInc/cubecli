package mcp

import (
	"fmt"
	"strings"

	"github.com/CubePathInc/cubecli/internal/cmdutil"
	internalConfig "github.com/CubePathInc/cubecli/internal/config"
	"github.com/CubePathInc/cubecli/internal/mcp"
	"github.com/CubePathInc/cubecli/internal/output"
	"github.com/spf13/cobra"
)

func NewCmd() *cobra.Command {
	mcpCmd := &cobra.Command{
		Use:   "mcp",
		Short: "Connect the CubePath MCP server to your AI agents",
		Long: `Add the CubePath MCP server to the AI agents on this machine, so they can
manage your infrastructure through its tools.

Supported agents: claude (Claude Code), codex, gemini (Gemini CLI), cursor,
vscode. Without --agent, every agent found on this machine is used.

cubecli only registers the server URL. Each agent then asks you to approve the
access in the browser, where you choose the organization and permissions.
Disconnect it any time at https://my.cubepath.com/account/connections.`,
	}
	mcpCmd.AddCommand(newInstallCmd(), newStatusCmd(), newUninstallCmd())
	return mcpCmd
}

// serverURL is the MCP endpoint for the active profile's API.
func serverURL(cmd *cobra.Command) string {
	explicit, _ := cmd.Flags().GetString("profile")
	p, _, err := internalConfig.LoadOrEmpty().ActiveProfile(explicit)
	if err != nil {
		p = nil
	}
	return mcp.ResolveURL(cmd.Context(), internalConfig.APIURL(p))
}

func resolveClients(cmd *cobra.Command, fallback []mcp.Client) ([]mcp.Client, error) {
	ids, _ := cmd.Flags().GetStringSlice("agent")
	if len(ids) == 0 {
		return fallback, nil
	}
	var out []mcp.Client
	seen := map[string]bool{}
	for _, id := range ids {
		id = strings.ToLower(strings.TrimSpace(id))
		if id == "all" {
			return mcp.Clients, nil
		}
		c, ok := mcp.ClientByID(id)
		if !ok {
			return nil, fmt.Errorf("unknown agent %q: use claude, codex, gemini, cursor, vscode or all", id)
		}
		if !seen[id] {
			seen[id] = true
			out = append(out, c)
		}
	}
	return out, nil
}

func newInstallCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Add the CubePath MCP server to your AI agents",
		Example: `  cubecli mcp install
  cubecli mcp install --agent claude --agent cursor`,
		RunE: func(cmd *cobra.Command, args []string) error {
			clients, err := resolveClients(cmd, mcp.Detected())
			if err != nil {
				return err
			}
			if len(clients) == 0 {
				return fmt.Errorf("no supported AI agent found on this machine; pass --agent claude, codex, gemini, cursor or vscode")
			}
			return Install(cmd, clients, serverURL(cmd))
		},
	}
	cmd.Flags().StringSlice("agent", nil, "Agent to configure: claude, codex, gemini, cursor, vscode or all (repeatable; default: detected agents)")
	return cmd
}

// Result of configuring one agent.
type Result struct {
	Agent    string `json:"agent"`
	Status   string `json:"status"` // added, already configured, failed
	Error    string `json:"error,omitempty"`
	NextStep string `json:"next_step,omitempty"`
}

// Install adds the server to each client. Shared with `login`.
func Install(cmd *cobra.Command, clients []mcp.Client, url string) error {
	var results []Result
	failed := 0
	for _, c := range clients {
		r := Result{Agent: c.ID}
		if ok, _ := c.Configured(url); ok {
			r.Status = "already configured"
		} else if err := c.Install(cmd.Context(), url); err != nil {
			r.Status, r.Error = "failed", err.Error()
			failed++
		} else {
			r.Status, r.NextStep = "added", c.NextStep
		}
		results = append(results, r)
	}

	if cmdutil.IsJSON(cmd) {
		if err := output.PrintJSON(map[string]interface{}{"url": url, "agents": results}); err != nil {
			return err
		}
	} else {
		for i, r := range results {
			name := clients[i].Name
			switch r.Status {
			case "added":
				output.PrintSuccess(fmt.Sprintf("CubePath MCP server added to %s. %s", name, r.NextStep))
			case "already configured":
				output.PrintInfo(fmt.Sprintf("%s already has the CubePath MCP server.", name))
			default:
				output.PrintWarning(fmt.Sprintf("Could not configure %s: %s", name, r.Error))
			}
		}
	}
	if failed == len(results) && failed > 0 {
		return fmt.Errorf("the CubePath MCP server could not be added to any agent")
	}
	return nil
}

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show which AI agents have the CubePath MCP server",
		RunE: func(cmd *cobra.Command, args []string) error {
			url := serverURL(cmd)
			type row struct {
				Agent      string `json:"agent"`
				Installed  bool   `json:"installed"`
				Configured bool   `json:"configured"`
			}
			var rows []row
			for _, c := range mcp.Clients {
				r := row{Agent: c.ID, Installed: c.Detected()}
				if r.Installed {
					r.Configured, _ = c.Configured(url)
				}
				rows = append(rows, r)
			}
			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(map[string]interface{}{"url": url, "agents": rows})
			}
			t := output.NewTable("CubePath MCP server", []string{"Agent", "Status"})
			for i, r := range rows {
				status := "not installed"
				switch {
				case r.Configured:
					status = "configured"
				case r.Installed:
					status = "not configured"
				}
				t.AddRow(mcp.Clients[i].Name, status)
			}
			t.Render()
			output.PrintInfo(fmt.Sprintf("Server URL: %s", url))
			return nil
		},
	}
}

func newUninstallCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Remove the CubePath MCP server from your AI agents",
		Long: `Remove the CubePath MCP server from your AI agents' configuration.

This does not revoke the access they were granted. Disconnect them at
https://my.cubepath.com/account/connections.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			url := serverURL(cmd)
			clients, err := resolveClients(cmd, nil)
			if err != nil {
				return err
			}
			if clients == nil {
				for _, c := range mcp.Detected() {
					if ok, _ := c.Configured(url); ok {
						clients = append(clients, c)
					}
				}
			}
			var results []Result
			for _, c := range clients {
				r := Result{Agent: c.ID, Status: "removed"}
				if err := c.Uninstall(cmd.Context(), url); err != nil {
					r.Status, r.Error = "failed", err.Error()
				}
				results = append(results, r)
				if !cmdutil.IsJSON(cmd) {
					if r.Error != "" {
						output.PrintWarning(fmt.Sprintf("Could not update %s: %s", c.Name, r.Error))
					} else {
						output.PrintSuccess(fmt.Sprintf("Removed the CubePath MCP server from %s", c.Name))
					}
				}
			}
			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(map[string]interface{}{"agents": results})
			}
			if len(clients) == 0 {
				output.PrintInfo("No agent has the CubePath MCP server configured.")
			} else {
				output.PrintInfo("To revoke the access itself, disconnect the apps at https://my.cubepath.com/account/connections.")
			}
			return nil
		},
	}
	cmd.Flags().StringSlice("agent", nil, "Agent: claude, codex, gemini, cursor, vscode or all (default: every agent that has it)")
	return cmd
}
