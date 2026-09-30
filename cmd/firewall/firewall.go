package firewall

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/CubePathInc/cubecli/internal/cmdutil"
	"github.com/CubePathInc/cubecli/internal/output"
	"github.com/spf13/cobra"
)

const ruleHelp = `Rules are allow rules; everything else inbound is dropped. Give each one with
--rule "<in|out> <tcp|udp|icmp|gre> [ports|-] [source|-] [comment...]":
ports is "443", "8000-8100" or "80,443" (tcp/udp only; "-" = all), source is
an IP or CIDR ("-" = any). --rules-file takes a JSON array of
{"direction","protocol","port","source","comment"} objects instead.`

func NewCmd() *cobra.Command {
	fwCmd := &cobra.Command{
		Use:   "firewall",
		Short: "Manage VPS firewall groups",
		Long: `Firewall groups are reusable sets of allow rules in a project. Assign up to 10
groups to a VPS with 'firewall assign'; a VPS with groups drops all other
inbound traffic.`,
	}

	groupCmd := &cobra.Command{
		Use:   "group",
		Short: "Manage firewall groups",
	}
	groupCmd.AddCommand(listCmd(), showCmd(), createCmd(), updateCmd(), deleteCmd())

	fwCmd.AddCommand(groupCmd, assignCmd())
	return fwCmd
}

type rule struct {
	Direction string  `json:"direction"`
	Protocol  string  `json:"protocol"`
	Port      *string `json:"port"`
	Source    *string `json:"source"`
	Comment   *string `json:"comment"`
}

type group struct {
	ID        int    `json:"id"`
	ProjectID int    `json:"project_id"`
	Name      string `json:"name"`
	Rules     []rule `json:"rules"`
	Enabled   bool   `json:"enabled"`
	VPSCount  int    `json:"vps_count"`
}

func deref(s *string, empty string) string {
	if s == nil || *s == "" {
		return empty
	}
	return *s
}

// parseRule reads "<direction> <protocol> [ports|-] [source|-] [comment...]".
func parseRule(spec string) (rule, error) {
	f := strings.Fields(spec)
	if len(f) < 2 {
		return rule{}, fmt.Errorf("invalid rule %q: expected \"<in|out> <protocol> [ports] [source] [comment]\"", spec)
	}
	r := rule{Direction: strings.ToLower(f[0]), Protocol: strings.ToLower(f[1])}
	if r.Direction != "in" && r.Direction != "out" {
		return rule{}, fmt.Errorf("invalid rule %q: direction must be in or out", spec)
	}
	opt := func(i int) *string {
		if len(f) <= i || f[i] == "-" {
			return nil
		}
		v := f[i]
		return &v
	}
	r.Port = opt(2)
	r.Source = opt(3)
	if len(f) > 4 {
		c := strings.Join(f[4:], " ")
		r.Comment = &c
	}
	return r, nil
}

// rulesFromFlags returns the rules given with --rule or --rules-file, and
// whether any were given.
func rulesFromFlags(cmd *cobra.Command) ([]rule, bool, error) {
	specs, _ := cmd.Flags().GetStringArray("rule")
	file, _ := cmd.Flags().GetString("rules-file")
	if len(specs) > 0 && file != "" {
		return nil, false, fmt.Errorf("use either --rule or --rules-file, not both")
	}
	if file != "" {
		var rules []rule
		if err := cmdutil.ReadJSONFile(file, &rules); err != nil {
			return nil, false, err
		}
		return rules, true, nil
	}
	rules := []rule{}
	for _, s := range specs {
		r, err := parseRule(s)
		if err != nil {
			return nil, false, err
		}
		rules = append(rules, r)
	}
	return rules, len(specs) > 0, nil
}

func addRuleFlags(cmd *cobra.Command) {
	cmd.Flags().StringArray("rule", nil, `Allow rule "<in|out> <protocol> [ports|-] [source|-] [comment]" (repeatable)`)
	cmd.Flags().String("rules-file", "", "JSON array of rules (\"-\" for stdin)")
}

func renderGroup(title string, g group) {
	t := output.NewTable(title, []string{"Field", "Value"})
	t.AddRow("ID", strconv.Itoa(g.ID))
	t.AddRow("Name", g.Name)
	t.AddRow("Project", strconv.Itoa(g.ProjectID))
	t.AddRow("Enabled", cmdutil.YesNo(g.Enabled))
	t.AddRow("VPS", strconv.Itoa(g.VPSCount))
	t.Render()

	rt := output.NewTable("Rules", []string{"Direction", "Protocol", "Ports", "Source", "Comment"})
	for _, r := range g.Rules {
		rt.AddRow(r.Direction, r.Protocol, deref(r.Port, "all"), deref(r.Source, "any"), deref(r.Comment, ""))
	}
	rt.Render()
}

func fetchGroups(cmd *cobra.Command) (json.RawMessage, []group, error) {
	client := cmdutil.GetClient(cmd)
	s := output.NewSpinner("Fetching firewall groups...")
	s.Start()
	resp, err := client.Get("/firewall/groups")
	s.Stop()
	if err != nil {
		return nil, nil, err
	}
	var groups []group
	if err := json.Unmarshal(resp, &groups); err != nil {
		return nil, nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return resp, groups, nil
}

// --- list ---

func listCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List firewall groups",
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, _ := cmd.Flags().GetInt("project")
			resp, groups, err := fetchGroups(cmd)
			if err != nil {
				return err
			}
			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			t := output.NewTable("Firewall Groups", []string{"ID", "Name", "Project", "Rules", "Enabled", "VPS"})
			for _, g := range groups {
				if projectID > 0 && g.ProjectID != projectID {
					continue
				}
				t.AddRow(strconv.Itoa(g.ID), g.Name, strconv.Itoa(g.ProjectID), strconv.Itoa(len(g.Rules)), cmdutil.YesNo(g.Enabled), strconv.Itoa(g.VPSCount))
			}
			t.Render()
			return nil
		},
	}
	cmd.Flags().IntP("project", "p", 0, "Filter by project ID")
	return cmd
}

// --- show ---

func showCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <group_id>",
		Short: "Show a firewall group and its rules",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid group_id: %s", args[0])
			}
			_, groups, err := fetchGroups(cmd)
			if err != nil {
				return err
			}
			for _, g := range groups {
				if g.ID == id {
					if cmdutil.IsJSON(cmd) {
						return output.PrintJSON(g)
					}
					renderGroup("Firewall Group", g)
					return nil
				}
			}
			return fmt.Errorf("firewall group %d not found", id)
		},
	}
}

// --- create ---

func createCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a firewall group",
		Long:  "Create a firewall group in a project.\n\n" + ruleHelp,
		Example: `  cubecli firewall group create --project 12 --name web \
    --rule "in tcp 80,443 - web traffic" --rule "in tcp 22 203.0.113.0/24 office ssh" --rule "in icmp"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			projectID, _ := cmd.Flags().GetInt("project")
			name, _ := cmd.Flags().GetString("name")
			disabled, _ := cmd.Flags().GetBool("disabled")
			rules, _, err := rulesFromFlags(cmd)
			if err != nil {
				return err
			}

			body := map[string]interface{}{
				"name":    name,
				"rules":   rules,
				"enabled": !disabled,
			}

			s := output.NewSpinner("Creating firewall group...")
			s.Start()
			resp, err := client.Post(fmt.Sprintf("/firewall/groups?project_id=%d", projectID), body)
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}
			var g group
			if err := json.Unmarshal(resp, &g); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}
			renderGroup("Firewall Group Created", g)
			output.PrintSuccess("Firewall group created successfully")
			return nil
		},
	}
	cmd.Flags().IntP("project", "p", 0, "Project ID")
	cmd.Flags().StringP("name", "n", "", "Group name")
	cmd.Flags().Bool("disabled", false, "Create the group disabled")
	addRuleFlags(cmd)
	_ = cmd.MarkFlagRequired("project")
	_ = cmd.MarkFlagRequired("name")
	return cmd
}

// --- update ---

func updateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update <group_id>",
		Short: "Rename, enable/disable or replace the rules of a firewall group",
		Long:  "Update a firewall group. --rule or --rules-file replace all its rules; the\nchange is applied to every VPS using the group.\n\n" + ruleHelp,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			id, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid group_id: %s", args[0])
			}

			body := map[string]interface{}{}
			if cmd.Flags().Changed("name") {
				v, _ := cmd.Flags().GetString("name")
				body["name"] = v
			}
			if cmd.Flags().Changed("enabled") {
				v, _ := cmd.Flags().GetBool("enabled")
				body["enabled"] = v
			}
			rules, given, err := rulesFromFlags(cmd)
			if err != nil {
				return err
			}
			if given {
				body["rules"] = rules
			}
			if len(body) == 0 {
				return fmt.Errorf("at least one of --name, --enabled, --rule or --rules-file must be specified")
			}

			s := output.NewSpinner("Updating firewall group...")
			s.Start()
			resp, err := client.Put(fmt.Sprintf("/firewall/groups/%d", id), body)
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}
			output.PrintSuccess("Firewall group updated successfully")
			return nil
		},
	}
	cmd.Flags().StringP("name", "n", "", "New name")
	cmd.Flags().Bool("enabled", true, "Enable or disable the group (--enabled=false)")
	addRuleFlags(cmd)
	return cmd
}

// --- delete ---

func deleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <group_id>",
		Short: "Delete a firewall group (unassign it from every VPS first)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid group_id: %s", args[0])
			}
			if !cmdutil.CheckForce(cmd, fmt.Sprintf("Delete firewall group %d?", id)) {
				output.PrintWarning("Aborted")
				return nil
			}

			client := cmdutil.GetClient(cmd)
			s := output.NewSpinner("Deleting firewall group...")
			s.Start()
			resp, err := client.Delete(fmt.Sprintf("/firewall/groups/%d", id))
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}
			output.PrintSuccess("Firewall group deleted successfully")
			return nil
		},
	}
	cmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")
	return cmd
}

// --- assign ---

func assignCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "assign <vps_id>",
		Short: "Set the firewall groups of a VPS (replaces the current ones)",
		Long: `Set the firewall groups of a VPS, in priority order. The groups must be in
the VPS's project. --none removes every group and turns the firewall off.`,
		Example: `  cubecli firewall assign 345 --group 3 --group 7
  cubecli firewall assign 345 --none`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			vpsID, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid vps_id: %s", args[0])
			}
			ids, _ := cmd.Flags().GetIntSlice("group")
			none, _ := cmd.Flags().GetBool("none")
			if none == (len(ids) > 0) {
				return fmt.Errorf("exactly one of --group or --none is required")
			}
			if ids == nil {
				ids = []int{}
			}

			s := output.NewSpinner("Updating firewall groups...")
			s.Start()
			resp, err := client.Put(fmt.Sprintf("/firewall/vps/%d/groups", vpsID), map[string]interface{}{"firewall_group_ids": ids})
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}
			output.PrintSuccess("Firewall groups updated successfully")
			return nil
		},
	}
	cmd.Flags().IntSlice("group", nil, "Firewall group ID, in priority order (repeatable, max 10)")
	cmd.Flags().Bool("none", false, "Remove every firewall group from the VPS")
	return cmd
}
