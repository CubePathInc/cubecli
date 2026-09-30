package ddosmitigation

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/CubePathInc/cubecli/internal/cmdutil"
	"github.com/CubePathInc/cubecli/internal/output"
	"github.com/spf13/cobra"
)

// protocols maps names to IP protocol numbers for --protocol.
var protocols = map[string]int{"any": 0, "icmp": 1, "tcp": 6, "udp": 17}

func parseProtocol(v string) (int, error) {
	if n, ok := protocols[strings.ToLower(v)]; ok {
		return n, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 || n > 255 {
		return 0, fmt.Errorf("invalid protocol %q: use any, tcp, udp, icmp or a number 0-255", v)
	}
	return n, nil
}

func protocolName(n int) string {
	for name, v := range protocols {
		if v == n {
			return strings.ToUpper(name)
		}
	}
	return strconv.Itoa(n)
}

// rateFields are the per-source rate limits used by actions 60 (pps) and 61 (Mbps).
var rateFields = []string{"tcp_syn", "tcp_ack", "tcp_synack", "tcp_rst", "tcp_fin", "tcp_all", "udp", "icmp"}

const actionsHelp = `Actions: 0 drop, 1 accept, 2 filter (continue),
10/11/12 FiveM TCP (monitor/auth/auth+rate limit), 15/16/17 FiveM UDP
(monitor/auth/self-auth), 20 RDP TCP, 21 RDP UDP, 30 DNS UDP, 31 DNS TCP,
40 Minecraft Java (TCP), 50 TLS validation (TCP), 60 source rate limit (pps),
61 source rate limit (Mbps). Protocol-specific actions need the matching
--protocol; 60 and 61 use the --tcp-syn ... --icmp limits.`

func ruleCmd() *cobra.Command {
	ruleCmd := &cobra.Command{
		Use:     "rule",
		Aliases: []string{"firewall-rule"},
		Short:   "Manage edge firewall rules of a protected IP (max 20 per IP)",
	}

	listCmd := &cobra.Command{
		Use:   "list <network>",
		Short: "List the firewall rules of an IP or subnet",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := fetch(cmd, "Fetching firewall rules...", "/ddos-mitigation/firewall-rules/"+args[0])
			if err != nil || resp == nil {
				return err
			}

			var r struct {
				Rules []struct {
					ID          int    `json:"id"`
					Network     string `json:"network"`
					Protocol    int    `json:"protocol"`
					DstPort     int    `json:"dst_port"`
					Action      int    `json:"action"`
					ActionLabel string `json:"action_label"`
				} `json:"rules"`
			}
			if err := json.Unmarshal(resp, &r); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}
			t := output.NewTable("Firewall Rules", []string{"ID", "Network", "Protocol", "Port", "Action"})
			for _, rule := range r.Rules {
				port := strconv.Itoa(rule.DstPort)
				if rule.DstPort == 0 {
					port = "any"
				}
				t.AddRow(strconv.Itoa(rule.ID), rule.Network, protocolName(rule.Protocol), port, fmt.Sprintf("%d %s", rule.Action, rule.ActionLabel))
			}
			t.Render()
			return nil
		},
	}

	createCmd := &cobra.Command{
		Use:   "create <network>",
		Short: "Create a firewall rule (a subnet gets one rule per IP)",
		Long:  "Create a firewall rule on an IP or an IPv4 subnet up to /24.\n\n" + actionsHelp,
		Example: `  cubecli ddos rule create 194.26.100.205 --protocol tcp --port 3389 --action 20
  cubecli ddos rule create 194.26.100.205 --protocol udp --port 0 --action 60 --udp 5000`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			protoStr, _ := cmd.Flags().GetString("protocol")
			proto, err := parseProtocol(protoStr)
			if err != nil {
				return err
			}
			port, _ := cmd.Flags().GetInt("port")
			action, _ := cmd.Flags().GetInt("action")

			body := map[string]interface{}{
				"network":  args[0],
				"protocol": proto,
				"dst_port": port,
				"action":   action,
			}
			for _, f := range rateFields {
				if cmd.Flags().Changed(flagName(f)) {
					v, _ := cmd.Flags().GetInt(flagName(f))
					body[f] = v
				}
			}

			return cmdutil.RunDetail(cmd, "Creating firewall rule...", "Rule created", func() (json.RawMessage, error) {
				return client.Post("/ddos-mitigation/firewall-rules", body)
			})
		},
	}
	createCmd.Flags().String("protocol", "any", "Protocol: any, tcp, udp, icmp or a number")
	createCmd.Flags().Int("port", 0, "Destination port (0 = any)")
	createCmd.Flags().Int("action", 0, "Action number (see above)")
	for _, f := range rateFields {
		createCmd.Flags().Int(flagName(f), 0, fmt.Sprintf("Rate limit for %s (actions 60/61)", strings.ToUpper(strings.ReplaceAll(f, "_", " "))))
	}
	_ = createCmd.MarkFlagRequired("action")

	deleteCmd := &cobra.Command{
		Use:   "delete <rule_id>",
		Short: "Delete a firewall rule",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid rule_id: %s", args[0])
			}
			if !cmdutil.CheckForce(cmd, fmt.Sprintf("Delete firewall rule %d?", id)) {
				output.PrintWarning("Aborted")
				return nil
			}
			client := cmdutil.GetClient(cmd)
			return cmdutil.RunDetail(cmd, "Deleting firewall rule...", "Rule deleted", func() (json.RawMessage, error) {
				return client.Delete(fmt.Sprintf("/ddos-mitigation/firewall-rules/%d", id))
			})
		},
	}
	deleteCmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")

	deleteBulkCmd := &cobra.Command{
		Use:   "delete-matching <network>",
		Short: "Delete the rules for a protocol and port on an IP or on every IP of a subnet",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			protoStr, _ := cmd.Flags().GetString("protocol")
			proto, err := parseProtocol(protoStr)
			if err != nil {
				return err
			}
			port, _ := cmd.Flags().GetInt("port")
			if !cmdutil.CheckForce(cmd, fmt.Sprintf("Delete the %s port %d rules of %s?", protocolName(proto), port, args[0])) {
				output.PrintWarning("Aborted")
				return nil
			}
			q := url.Values{}
			q.Set("network", args[0])
			q.Set("protocol", strconv.Itoa(proto))
			q.Set("dst_port", strconv.Itoa(port))
			client := cmdutil.GetClient(cmd)
			return cmdutil.RunDetail(cmd, "Deleting firewall rules...", "Rules deleted", func() (json.RawMessage, error) {
				return client.Delete("/ddos-mitigation/firewall-rules/bulk?" + q.Encode())
			})
		},
	}
	deleteBulkCmd.Flags().String("protocol", "any", "Protocol: any, tcp, udp, icmp or a number")
	deleteBulkCmd.Flags().Int("port", 0, "Destination port (0 = any)")
	deleteBulkCmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")

	ruleCmd.AddCommand(listCmd, createCmd, deleteCmd, deleteBulkCmd)
	return ruleCmd
}

func prefixListCmd() *cobra.Command {
	plCmd := &cobra.Command{
		Use:   "prefix-list",
		Short: "Manage prefix lists used by protection profiles (max 3, 100 entries each)",
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List your prefix lists and the platform ones",
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := fetch(cmd, "Fetching prefix lists...", "/ddos-mitigation/prefix-lists")
			if err != nil || resp == nil {
				return err
			}
			return renderPrefixLists("Prefix Lists", resp)
		},
	}

	createCmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a prefix list",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			body := map[string]interface{}{"name": args[0]}
			if d, _ := cmd.Flags().GetString("description"); d != "" {
				body["description"] = d
			}
			if err := cmdutil.RunDetail(cmd, "Creating prefix list...", "Prefix list created", func() (json.RawMessage, error) {
				return client.Post("/ddos-mitigation/prefix-lists", body)
			}); err != nil || cmdutil.IsJSON(cmd) {
				return err
			}
			// The create response carries no uuid: look it up by name.
			if resp, err := client.Get("/ddos-mitigation/prefix-lists"); err == nil {
				var r struct {
					PrefixLists []struct {
						UUID     string `json:"uuid"`
						Name     string `json:"name"`
						IsGlobal bool   `json:"is_global"`
					} `json:"prefix_lists"`
				}
				if json.Unmarshal(resp, &r) == nil {
					for _, p := range r.PrefixLists {
						if p.Name == args[0] && !p.IsGlobal {
							output.PrintInfo("UUID: " + p.UUID)
						}
					}
				}
			}
			return nil
		},
	}
	createCmd.Flags().StringP("description", "d", "", "Description")

	deleteCmd := &cobra.Command{
		Use:   "delete <uuid>",
		Short: "Delete a prefix list and its entries",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmdutil.CheckForce(cmd, "Delete this prefix list? Profiles using it lose it.") {
				output.PrintWarning("Aborted")
				return nil
			}
			client := cmdutil.GetClient(cmd)
			return cmdutil.RunDetail(cmd, "Deleting prefix list...", "Prefix list deleted", func() (json.RawMessage, error) {
				return client.Delete("/ddos-mitigation/prefix-lists/" + args[0])
			})
		},
	}
	deleteCmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")

	entriesCmd := &cobra.Command{
		Use:   "entries <uuid>",
		Short: "List the entries of a prefix list",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := fetch(cmd, "Fetching entries...", "/ddos-mitigation/prefix-lists/"+args[0]+"/entries")
			if err != nil || resp == nil {
				return err
			}
			var entries []struct {
				Network string `json:"network"`
			}
			if err := json.Unmarshal(resp, &entries); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}
			t := output.NewTable("Prefix List Entries", []string{"Network"})
			for _, e := range entries {
				t.AddRow(e.Network)
			}
			t.Render()
			return nil
		},
	}

	addCmd := &cobra.Command{
		Use:   "add-entry <uuid> <ip_or_cidr>",
		Short: "Add an IP or CIDR to a prefix list",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			return cmdutil.RunDetail(cmd, "Adding entry...", "Entry added", func() (json.RawMessage, error) {
				return client.Post("/ddos-mitigation/prefix-lists/"+args[0]+"/entries", map[string]string{"network": args[1]})
			})
		},
	}

	removeCmd := &cobra.Command{
		Use:   "remove-entry <uuid> <ip_or_cidr>",
		Short: "Remove an IP or CIDR from a prefix list",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			return cmdutil.RunDetail(cmd, "Removing entry...", "Entry removed", func() (json.RawMessage, error) {
				return client.Delete("/ddos-mitigation/prefix-lists/" + args[0] + "/entries/" + args[1])
			})
		},
	}

	plCmd.AddCommand(listCmd, createCmd, deleteCmd, entriesCmd, addCmd, removeCmd)
	return plCmd
}
