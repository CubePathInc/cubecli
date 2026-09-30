package network

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/CubePathInc/cubecli/internal/cmdutil"
	"github.com/CubePathInc/cubecli/internal/output"
	"github.com/spf13/cobra"
)

// addBGPPeerCmd wires `network bgp-peer`: dynamic routes learned over eBGP
// from a customer router inside the network. The network side uses ASN 64512.
func addBGPPeerCmd(parent *cobra.Command) {
	bgpCmd := &cobra.Command{
		Use:   "bgp-peer",
		Short: "Manage BGP sessions of a private network (dynamic routes)",
		Long: `Manage eBGP sessions between the private network's gateway (ASN 64512) and
a router of yours inside the network: a VPS, a baremetal server or an IP.
Prefixes the router announces become routes of the network. Max 8 per network.`,
	}

	parseNetworkID := func(arg string) (int, error) {
		id, err := strconv.Atoi(arg)
		if err != nil {
			return 0, fmt.Errorf("invalid network_id: %s", arg)
		}
		return id, nil
	}

	listCmd := &cobra.Command{
		Use:   "list <network_id>",
		Short: "List BGP peers and their session state",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			networkID, err := parseNetworkID(args[0])
			if err != nil {
				return err
			}

			s := output.NewSpinner("Fetching BGP peers...")
			s.Start()
			resp, err := client.Get(fmt.Sprintf("/networks/%d/bgp-peers", networkID))
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			var peers []struct {
				ID               string   `json:"id"`
				PeerType         string   `json:"peer_type"`
				PeerTarget       string   `json:"peer_target"`
				RemoteASN        int64    `json:"remote_asn"`
				MaxPrefix        int      `json:"max_prefix"`
				Description      string   `json:"description"`
				Enabled          bool     `json:"enabled"`
				ResolvedPeerIP   string   `json:"resolved_peer_ip"`
				LastState        string   `json:"last_state"`
				ReceivedPrefixes []string `json:"received_prefixes"`
			}
			if err := json.Unmarshal(resp, &peers); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}

			t := output.NewTable("BGP Peers", []string{"ID", "Peer", "Peer IP", "Remote ASN", "Max Prefix", "Enabled", "State", "Prefixes", "Description"})
			for _, p := range peers {
				state := p.LastState
				if state == "" {
					state = "-"
				}
				t.AddRow(
					p.ID,
					p.PeerType+" "+p.PeerTarget,
					p.ResolvedPeerIP,
					strconv.FormatInt(p.RemoteASN, 10),
					strconv.Itoa(p.MaxPrefix),
					cmdutil.YesNo(p.Enabled),
					state,
					strings.Join(p.ReceivedPrefixes, ", "),
					p.Description,
				)
			}
			t.Render()
			return nil
		},
	}

	createCmd := &cobra.Command{
		Use:   "create <network_id>",
		Short: "Create a BGP peer",
		Example: `  cubecli network bgp-peer create 42 --type vps --target 123 --remote-asn 65010
  cubecli network bgp-peer create 42 --type ip --target 10.0.0.5 --remote-asn 65010 --max-prefix 50`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			networkID, err := parseNetworkID(args[0])
			if err != nil {
				return err
			}

			peerType, _ := cmd.Flags().GetString("type")
			target, _ := cmd.Flags().GetString("target")
			asn, _ := cmd.Flags().GetInt64("remote-asn")
			body := map[string]interface{}{
				"peer_type":   peerType,
				"peer_target": target,
				"remote_asn":  asn,
			}
			if cmd.Flags().Changed("max-prefix") {
				v, _ := cmd.Flags().GetInt("max-prefix")
				body["max_prefix"] = v
			}
			if d, _ := cmd.Flags().GetString("description"); d != "" {
				body["description"] = d
			}

			s := output.NewSpinner("Creating BGP peer...")
			s.Start()
			resp, err := client.Post(fmt.Sprintf("/networks/%d/bgp-peers", networkID), body)
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			var r struct {
				PeerID string `json:"peer_id"`
			}
			if json.Unmarshal(resp, &r) == nil && r.PeerID != "" {
				output.PrintSuccess(fmt.Sprintf("BGP peer created: %s", r.PeerID))
			} else {
				output.PrintSuccess("BGP peer created")
			}
			return nil
		},
	}
	createCmd.Flags().String("type", "", "Peer type: vps, baremetal or ip")
	createCmd.Flags().String("target", "", "VPS ID, baremetal ID or IP inside the network")
	createCmd.Flags().Int64("remote-asn", 0, "ASN of your router (not 64512)")
	createCmd.Flags().Int("max-prefix", 100, "Maximum prefixes accepted (1-1000)")
	createCmd.Flags().String("description", "", "Description")
	_ = createCmd.MarkFlagRequired("type")
	_ = createCmd.MarkFlagRequired("target")
	_ = createCmd.MarkFlagRequired("remote-asn")

	updateCmd := &cobra.Command{
		Use:   "update <network_id> <peer_id>",
		Short: "Update a BGP peer (type, target and ASN cannot change)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			networkID, err := parseNetworkID(args[0])
			if err != nil {
				return err
			}

			body := map[string]interface{}{}
			if cmd.Flags().Changed("max-prefix") {
				v, _ := cmd.Flags().GetInt("max-prefix")
				body["max_prefix"] = v
			}
			if cmd.Flags().Changed("description") {
				v, _ := cmd.Flags().GetString("description")
				body["description"] = v
			}
			if cmd.Flags().Changed("enabled") {
				v, _ := cmd.Flags().GetBool("enabled")
				body["enabled"] = v
			}
			if len(body) == 0 {
				return fmt.Errorf("at least one of --max-prefix, --description or --enabled must be specified")
			}

			return cmdutil.RunDetail(cmd, "Updating BGP peer...", "BGP peer updated", func() (json.RawMessage, error) {
				return client.Patch(fmt.Sprintf("/networks/%d/bgp-peers/%s", networkID, args[1]), body)
			})
		},
	}
	updateCmd.Flags().Int("max-prefix", 0, "Maximum prefixes accepted (1-1000)")
	updateCmd.Flags().String("description", "", "Description")
	updateCmd.Flags().Bool("enabled", true, "Enable or disable the session (--enabled=false)")

	deleteCmd := &cobra.Command{
		Use:   "delete <network_id> <peer_id>",
		Short: "Delete a BGP peer",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			networkID, err := parseNetworkID(args[0])
			if err != nil {
				return err
			}
			if !cmdutil.CheckForce(cmd, "Delete this BGP peer? Its routes are withdrawn.") {
				output.PrintWarning("Aborted")
				return nil
			}
			return cmdutil.RunDetail(cmd, "Deleting BGP peer...", "BGP peer deleted", func() (json.RawMessage, error) {
				return client.Delete(fmt.Sprintf("/networks/%d/bgp-peers/%s", networkID, args[1]))
			})
		},
	}
	deleteCmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")

	bgpCmd.AddCommand(listCmd, createCmd, updateCmd, deleteCmd)
	parent.AddCommand(bgpCmd)
}
