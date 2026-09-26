package skills

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/CubePathInc/cubecli/internal/cmdutil"
	"github.com/CubePathInc/cubecli/internal/output"
	"github.com/CubePathInc/cubecli/internal/skills"
	"github.com/spf13/cobra"
)

func NewCmd() *cobra.Command {
	skillsCmd := &cobra.Command{
		Use:   "skills",
		Short: "Install the CubePath skills for AI coding agents",
		Long: `Install the CubePath agent skills (github.com/CubePathInc/skills), which
teach AI coding agents to manage CubePath with cubecli.

Targets:
  claude   ~/.claude/skills   Claude Code
  agents   ~/.agents/skills   Codex, Gemini CLI, Cursor

Without --agent, the targets of the agents found on this machine are used.
With --project, skills go into ./.claude/skills or ./.agents/skills of the
current directory instead, to commit them with a repository.

In Claude Code you can also install them as a plugin:
  /plugin marketplace add CubePathInc/skills`,
	}
	skillsCmd.AddCommand(newInstallCmd(), newUpdateCmd(), newListCmd(), newUninstallCmd())
	return skillsCmd
}

func addTargetFlags(cmd *cobra.Command) {
	cmd.Flags().StringSlice("agent", nil, "Target to use: claude, agents or all (repeatable; default: detected agents)")
	cmd.Flags().Bool("project", false, "Use the current directory instead of your home directory")
}

// resolveTargets turns --agent into targets. explicit reports whether the
// user chose them, as opposed to detection.
func resolveTargets(cmd *cobra.Command) (targets []skills.Target, explicit bool, err error) {
	ids, _ := cmd.Flags().GetStringSlice("agent")
	if len(ids) == 0 {
		return skills.DefaultTargets(), false, nil
	}
	seen := map[string]bool{}
	for _, id := range ids {
		id = strings.ToLower(strings.TrimSpace(id))
		if id == "all" {
			return skills.Targets, true, nil
		}
		t, ok := skills.TargetByID(id)
		if !ok {
			return nil, true, fmt.Errorf("unknown agent %q: use claude, agents or all", id)
		}
		if !seen[id] {
			seen[id] = true
			targets = append(targets, t)
		}
	}
	return targets, true, nil
}

func newInstallCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Download the latest skills and install them",
		Example: `  cubecli skills install
  cubecli skills install --agent claude
  cubecli skills install --agent agents --project`,
		RunE: func(cmd *cobra.Command, args []string) error {
			targets, explicit, err := resolveTargets(cmd)
			if err != nil {
				return err
			}
			if len(targets) == 0 {
				return fmt.Errorf("no supported AI agent found on this machine; pass --agent claude, agents or all")
			}
			project, _ := cmd.Flags().GetBool("project")
			tag, _ := cmd.Flags().GetString("version")
			force, _ := cmd.Flags().GetBool("force")
			if !explicit && skills.ClaudePluginInstalled() && !cmdutil.IsJSON(cmd) {
				output.PrintInfo("The CubePath plugin is installed in Claude Code, so ~/.claude/skills is left alone.")
			}
			return Run(cmd, targets, project, tag, force)
		},
	}
	addTargetFlags(cmd)
	cmd.Flags().String("version", "", "Install this release (e.g. v0.1.0) instead of the latest")
	cmd.Flags().BoolP("force", "f", false, "Overwrite skills with the same name that cubecli did not install, or that were modified")
	return cmd
}

func newUpdateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update the installed skills to the latest release",
		RunE: func(cmd *cobra.Command, args []string) error {
			project, _ := cmd.Flags().GetBool("project")
			force, _ := cmd.Flags().GetBool("force")
			var targets []skills.Target
			for _, t := range skills.Targets {
				dir, err := t.Dir(project)
				if err != nil {
					return err
				}
				if installed, _ := skills.List(dir); len(installed) > 0 {
					targets = append(targets, t)
				}
			}
			if len(targets) == 0 {
				output.PrintInfo("No CubePath skills installed. Run 'cubecli skills install'.")
				return nil
			}
			return Run(cmd, targets, project, "", force)
		},
	}
	cmd.Flags().Bool("project", false, "Update the skills of the current directory instead of your home directory")
	cmd.Flags().BoolP("force", "f", false, "Overwrite skills that were modified")
	return cmd
}

// Run downloads a release and installs it into targets. Shared with `login`.
func Run(cmd *cobra.Command, targets []skills.Target, project bool, tag string, force bool) error {
	ctx, cancel := context.WithTimeout(cmd.Context(), 2*time.Minute)
	defer cancel()

	s := output.NewSpinner("Downloading CubePath skills...")
	if !cmdutil.IsJSON(cmd) {
		s.Start()
	}
	bundle, err := skills.Download(ctx, tag)
	s.Stop()
	if err != nil {
		return err
	}
	defer bundle.Cleanup()

	type result struct {
		Agent   string           `json:"agent"`
		Dir     string           `json:"dir"`
		Results []skills.Outcome `json:"skills"`
	}
	var results []result
	for _, t := range targets {
		dir, err := t.Dir(project)
		if err != nil {
			return err
		}
		outcomes, err := skills.Install(bundle, dir, force)
		if err != nil {
			return fmt.Errorf("installing into %s: %w", dir, err)
		}
		results = append(results, result{Agent: t.ID, Dir: dir, Results: outcomes})
	}

	if cmdutil.IsJSON(cmd) {
		return output.PrintJSON(map[string]interface{}{"version": bundle.Manifest.Version, "targets": results})
	}
	for i, r := range results {
		changed, skipped := 0, 0
		for _, o := range r.Results {
			if o.Action == "skipped" {
				skipped++
				output.PrintWarning(fmt.Sprintf("%s: %s skipped, %s", r.Dir, o.Name, o.Reason))
			} else {
				changed++
			}
		}
		if changed > 0 {
			output.PrintSuccess(fmt.Sprintf("%d CubePath skills v%s in %s (%s)", changed, bundle.Manifest.Version, r.Dir, targets[i].Name))
		}
	}
	output.PrintInfo("Restart your agent (or start a new session) to load them.")
	return nil
}

func newListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "Show the installed skills and whether an update is available",
		RunE: func(cmd *cobra.Command, args []string) error {
			project, _ := cmd.Flags().GetBool("project")

			ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
			latest, _ := skills.LatestVersion(ctx) // best effort, offline is fine
			cancel()

			type row struct {
				Agent    string `json:"agent"`
				Dir      string `json:"dir"`
				Skill    string `json:"skill"`
				Version  string `json:"version"`
				Modified bool   `json:"modified"`
			}
			var rows []row
			for _, t := range skills.Targets {
				dir, err := t.Dir(project)
				if err != nil {
					return err
				}
				installed, err := skills.List(dir)
				if err != nil {
					return err
				}
				for _, s := range installed {
					rows = append(rows, row{t.ID, dir, s.Name, s.Version, s.Modified})
				}
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(map[string]interface{}{"latest": latest, "installed": rows})
			}
			if len(rows) == 0 {
				output.PrintInfo("No CubePath skills installed. Run 'cubecli skills install'.")
				if skills.ClaudePluginInstalled() {
					output.PrintInfo("They are installed as a Claude Code plugin (managed with /plugin).")
				}
				return nil
			}
			t := output.NewTable("CubePath skills", []string{"Agent", "Skill", "Version", "Status", "Directory"})
			outdated := false
			for _, r := range rows {
				status := "ok"
				if latest != "" && "v"+strings.TrimPrefix(r.Version, "v") != latest {
					status = "update available"
					outdated = true
				}
				if r.Modified {
					status = "modified"
				}
				t.AddRow(r.Agent, r.Skill, r.Version, status, r.Dir)
			}
			t.Render()
			if outdated {
				output.PrintInfo(fmt.Sprintf("Run 'cubecli skills update' to get %s.", latest))
			}
			return nil
		},
	}
	cmd.Flags().Bool("project", false, "List the skills of the current directory instead of your home directory")
	return cmd
}

func newUninstallCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Remove the skills installed by cubecli",
		RunE: func(cmd *cobra.Command, args []string) error {
			ids, _ := cmd.Flags().GetStringSlice("agent")
			targets := skills.Targets
			if len(ids) > 0 {
				var err error
				if targets, _, err = resolveTargets(cmd); err != nil {
					return err
				}
			}
			project, _ := cmd.Flags().GetBool("project")
			force, _ := cmd.Flags().GetBool("force")

			var all []skills.Outcome
			for _, t := range targets {
				dir, err := t.Dir(project)
				if err != nil {
					return err
				}
				outcomes, err := skills.Uninstall(dir, force)
				if err != nil {
					return err
				}
				if !cmdutil.IsJSON(cmd) {
					removed := 0
					for _, o := range outcomes {
						if o.Action == "removed" {
							removed++
						} else {
							output.PrintWarning(fmt.Sprintf("%s: %s kept, %s", dir, o.Name, o.Reason))
						}
					}
					if removed > 0 {
						output.PrintSuccess(fmt.Sprintf("Removed %d CubePath skills from %s", removed, dir))
					}
				}
				all = append(all, outcomes...)
			}
			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(map[string]interface{}{"skills": all})
			}
			if len(all) == 0 {
				output.PrintInfo("No CubePath skills installed by cubecli.")
			}
			return nil
		},
	}
	cmd.Flags().StringSlice("agent", nil, "Target: claude, agents or all (default: all)")
	cmd.Flags().Bool("project", false, "Use the current directory instead of your home directory")
	cmd.Flags().BoolP("force", "f", false, "Also remove skills that were modified")
	return cmd
}
