package ddosmitigation

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/CubePathInc/cubecli/internal/cmdutil"
	"github.com/CubePathInc/cubecli/internal/output"
	"github.com/spf13/cobra"
)

// profileField is one integer field of a protection profile.
type profileField struct {
	Name  string // JSON field
	Usage string
	// Threshold fields must be at least 1 on writes (0 used to mean "no limit").
	Threshold bool
}

var profileFields = []profileField{
	{Name: "tcp_validation_level", Usage: "TCP validation level (0-10)"},
	{Name: "tcp_validation_sym_level", Usage: "Symmetric TCP validation level (0-10, needs symmetric routing)"},
	{Name: "udp_validation_level", Usage: "UDP validation level (0-10)"},
	{Name: "invalid_filter_level", Usage: "Invalid packet filter level (0-10)"},
	{Name: "fragmented_filter_level", Usage: "Fragmented packet filter level (0-10)"},
	{Name: "amplification_udp_level", Usage: "UDP amplification filter level (0-10)"},
	{Name: "amplification_tcp_level", Usage: "TCP amplification filter level (0-10)"},
	{Name: "icmp_rate_limit_level", Usage: "ICMP rate limit level (0-10)"},
	{Name: "same_packet_size_level", Usage: "Same packet size filter level (0-10)"},
	{Name: "stateful_firewall_level", Usage: "Stateful firewall level (0-10, needs symmetric routing)"},
	{Name: "default_action", Usage: "Default action: 0 filter, 1 accept, 2 drop"},
	{Name: "country_mode", Usage: "Country filter: 0 off, 1 blacklist, 2 whitelist"},
	{Name: "asn_mode", Usage: "ASN filter: 0 off, 1 blacklist, 2 whitelist"},
	{Name: "prefix_list_mode", Usage: "Prefix list filter: 0 off, 1 blacklist, 2 whitelist"},
	{Name: "udp_threshold_pps", Usage: "UDP threshold (packets/s)", Threshold: true},
	{Name: "tcp_threshold_pps", Usage: "TCP threshold (packets/s)", Threshold: true},
	{Name: "tcp_syn_threshold_pps", Usage: "TCP SYN threshold (packets/s)", Threshold: true},
	{Name: "tcp_ack_threshold_pps", Usage: "TCP ACK threshold (packets/s)", Threshold: true},
	{Name: "icmp_threshold_pps", Usage: "ICMP threshold (packets/s)", Threshold: true},
	{Name: "udp_threshold_mbps", Usage: "UDP threshold (Mbps)", Threshold: true},
	{Name: "tcp_threshold_mbps", Usage: "TCP threshold (Mbps)", Threshold: true},
	{Name: "tcp_syn_threshold_mbps", Usage: "TCP SYN threshold (Mbps)", Threshold: true},
	{Name: "tcp_ack_threshold_mbps", Usage: "TCP ACK threshold (Mbps)", Threshold: true},
	{Name: "icmp_threshold_mbps", Usage: "ICMP threshold (Mbps)", Threshold: true},
	{Name: "syn_flood_threshold", Usage: "SYN flood threshold (0-10000)"},
	{Name: "syn_flood_block_secs", Usage: "SYN flood block time in seconds (0-86400)"},
}

func flagName(field string) string {
	return strings.ReplaceAll(field, "_", "-")
}

func profileCmd() *cobra.Command {
	profileCmd := &cobra.Command{
		Use:   "profile",
		Short: "Show and tune the protection profile of an IP or subnet",
	}

	showCmd := &cobra.Command{
		Use:   "show <network>",
		Short: "Show the protection profile (platform defaults if none is set)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			s := output.NewSpinner("Fetching profile...")
			s.Start()
			resp, err := client.Get("/ddos-mitigation/profiles/" + args[0])
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			var p map[string]interface{}
			if err := json.Unmarshal(resp, &p); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}
			t := output.NewTable("Protection Profile "+fmt.Sprint(p["network"]), []string{"Setting", "Value"})
			for _, f := range profileFields {
				t.AddRow(f.Name, fmt.Sprint(p[f.Name]))
			}
			t.AddRow("always_on_mitigation", fmt.Sprint(p["always_on_mitigation"]))
			t.AddRow("symmetric_routing", fmt.Sprint(p["symmetric_routing"]))
			t.Render()
			return nil
		},
	}

	updateCmd := &cobra.Command{
		Use:   "update <network>",
		Short: "Change settings of the protection profile",
		Long: `Change settings of the protection profile. Settings you do not pass keep
their current value. Thresholds stored as 0 (no limit, no longer accepted by
the API) are reset to the platform default when the profile is saved.

--always-on and --symmetric-routing (0 or 1) control traffic redirection to the
scrubbing edge; symmetric routing requires always-on.`,
		Example: `  cubecli ddos profile update 194.26.100.205 --udp-threshold-pps 5000 --country-mode 1
  cubecli ddos profile update 194.26.100.205 --always-on 1`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			network := args[0]

			changed := false
			for _, f := range profileFields {
				changed = changed || cmd.Flags().Changed(flagName(f.Name))
			}
			changed = changed || cmd.Flags().Changed("always-on") || cmd.Flags().Changed("symmetric-routing")
			if !changed {
				return fmt.Errorf("no setting given; see --help for the available flags")
			}

			s := output.NewSpinner("Updating profile...")
			s.Start()
			current, err := client.Get("/ddos-mitigation/profiles/" + network)
			if err != nil {
				s.Stop()
				return err
			}
			body, err := mergeProfile(cmd, current)
			if err != nil {
				s.Stop()
				return err
			}
			resp, err := client.Put("/ddos-mitigation/profiles/"+network, body)
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}
			output.PrintSuccess("Profile updated successfully")
			return nil
		},
	}
	for _, f := range profileFields {
		updateCmd.Flags().Int(flagName(f.Name), 0, f.Usage)
	}
	updateCmd.Flags().Int("always-on", 0, "Always-on mitigation: 1 on, 0 off")
	updateCmd.Flags().Int("symmetric-routing", 0, "Symmetric routing: 1 on, 0 off (needs always-on)")

	deleteCmd := &cobra.Command{
		Use:   "delete <network>",
		Short: "Delete the profile and go back to the platform defaults",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmdutil.CheckForce(cmd, fmt.Sprintf("Reset the protection profile of %s to the defaults?", args[0])) {
				output.PrintWarning("Aborted")
				return nil
			}
			client := cmdutil.GetClient(cmd)
			return cmdutil.RunDetail(cmd, "Deleting profile...", "Profile deleted", func() (json.RawMessage, error) {
				return client.Delete("/ddos-mitigation/profiles/" + args[0])
			})
		},
	}
	deleteCmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")

	profileCmd.AddCommand(showCmd, updateCmd, deleteCmd)
	profileCmd.AddCommand(assignmentCmds()...)
	return profileCmd
}

// mergeProfile builds the full PUT body from the current profile plus the
// flags that were set. The PUT replaces the whole profile.
func mergeProfile(cmd *cobra.Command, current json.RawMessage) (map[string]interface{}, error) {
	var cur map[string]interface{}
	if err := json.Unmarshal(current, &cur); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	body := map[string]interface{}{}
	for _, f := range profileFields {
		if cmd.Flags().Changed(flagName(f.Name)) {
			v, _ := cmd.Flags().GetInt(flagName(f.Name))
			body[f.Name] = v
			continue
		}
		n, ok := cur[f.Name].(float64)
		if !ok || (f.Threshold && n < 1) {
			continue // server default
		}
		body[f.Name] = int(n)
	}
	if cmd.Flags().Changed("always-on") {
		v, _ := cmd.Flags().GetInt("always-on")
		body["always_on_mitigation"] = v
	}
	if cmd.Flags().Changed("symmetric-routing") {
		v, _ := cmd.Flags().GetInt("symmetric-routing")
		body["symmetric_routing"] = v
	}
	return body, nil
}

// assignmentCmds are the per-profile country, ASN and prefix-list lists.
func assignmentCmds() []*cobra.Command {
	countries := &cobra.Command{
		Use:   "countries <network>",
		Short: "List the countries assigned to the profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := fetch(cmd, "Fetching countries...", "/ddos-mitigation/profiles/"+args[0]+"/countries")
			if err != nil || resp == nil {
				return err
			}
			return renderCountries("Assigned Countries", resp)
		},
	}
	setCountries := &cobra.Command{
		Use:   "set-countries <network> [ISO codes...]",
		Short: "Replace the countries of the profile (no codes clears them)",
		Long: `Replace the countries used by the profile's country filter (see --country-mode
of 'profile update'). Codes not in 'ddos-mitigation countries' are ignored.`,
		Example: `  cubecli ddos profile set-countries 194.26.100.205 CN RU
  cubecli ddos profile set-countries 194.26.100.205`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			codes := splitList(args[1:])
			for i := range codes {
				codes[i] = strings.ToUpper(codes[i])
			}
			return cmdutil.RunDetail(cmd, "Updating countries...", "Countries updated", func() (json.RawMessage, error) {
				return client.Put("/ddos-mitigation/profiles/"+args[0]+"/countries", map[string]interface{}{"iso_codes": codes})
			})
		},
	}

	asns := &cobra.Command{
		Use:   "asns <network>",
		Short: "List the ASNs assigned to the profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := fetch(cmd, "Fetching ASNs...", "/ddos-mitigation/profiles/"+args[0]+"/asns")
			if err != nil || resp == nil {
				return err
			}
			return renderASNs("Assigned ASNs", resp)
		},
	}
	setASNs := &cobra.Command{
		Use:     "set-asns <network> [ASNs...]",
		Short:   "Replace the ASNs of the profile (no ASNs clears them)",
		Example: `  cubecli ddos profile set-asns 194.26.100.205 13335 15169`,
		Args:    cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			list := []int64{}
			for _, a := range splitList(args[1:]) {
				n, err := strconv.ParseInt(strings.TrimPrefix(strings.ToUpper(a), "AS"), 10, 64)
				if err != nil {
					return fmt.Errorf("invalid ASN: %s", a)
				}
				list = append(list, n)
			}
			return cmdutil.RunDetail(cmd, "Updating ASNs...", "ASNs updated", func() (json.RawMessage, error) {
				return client.Put("/ddos-mitigation/profiles/"+args[0]+"/asns", map[string]interface{}{"asns": list})
			})
		},
	}

	lists := &cobra.Command{
		Use:   "prefix-lists <network>",
		Short: "List the prefix lists assigned to the profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := fetch(cmd, "Fetching prefix lists...", "/ddos-mitigation/profiles/"+args[0]+"/prefix-lists")
			if err != nil || resp == nil {
				return err
			}
			return renderPrefixLists("Assigned Prefix Lists", resp)
		},
	}
	setLists := &cobra.Command{
		Use:   "set-prefix-lists <network> [prefix list UUIDs...]",
		Short: "Replace the prefix lists of the profile (no UUIDs clears them)",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			uuids := splitList(args[1:])
			return cmdutil.RunDetail(cmd, "Updating prefix lists...", "Prefix lists updated", func() (json.RawMessage, error) {
				return client.Put("/ddos-mitigation/profiles/"+args[0]+"/prefix-lists", map[string]interface{}{"uuids": uuids})
			})
		},
	}

	return []*cobra.Command{countries, setCountries, asns, setASNs, lists, setLists}
}

// fetch GETs path; with --json it prints the response and returns nil, nil.
func fetch(cmd *cobra.Command, spin, path string) (json.RawMessage, error) {
	client := cmdutil.GetClient(cmd)
	s := output.NewSpinner(spin)
	s.Start()
	resp, err := client.Get(path)
	s.Stop()
	if err != nil {
		return nil, err
	}
	if cmdutil.IsJSON(cmd) {
		return nil, output.PrintJSON(json.RawMessage(resp))
	}
	return resp, nil
}
