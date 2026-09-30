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

func NewCmd() *cobra.Command {
	ddosCmd := &cobra.Command{
		Use:     "ddos-mitigation",
		Aliases: []string{"ddos"},
		Short:   "Manage DDoS protection of Premium floating IPs",
		Long: `Tune the DDoS mitigation of floating IPs with Premium protection: per-IP
protection profiles (filters, thresholds, geo/ASN/prefix-list modes), firewall
rules applied at the scrubbing edge, prefix lists and captured traffic.

Networks are a single IP (194.26.100.205) or an IPv4 subnet up to /24
(194.26.100.0/24); subnet changes are applied to every IP in it.`,
	}

	ddosCmd.AddCommand(
		ipsCmd(),
		profileCmd(),
		ruleCmd(),
		prefixListCmd(),
		countriesCmd(),
		asnsCmd(),
		trafficCmd(),
	)
	return ddosCmd
}

// --- ips ---

func ipsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ips",
		Short: "List Premium-protected IPs and subnets with their profile status",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			q := url.Values{}
			if v, _ := cmd.Flags().GetString("ip-type"); v != "" {
				q.Set("ip_type", v)
			}
			if v, _ := cmd.Flags().GetString("location"); v != "" {
				q.Set("location", v)
			}
			if cmd.Flags().Changed("has-profile") {
				v, _ := cmd.Flags().GetBool("has-profile")
				q.Set("has_profile", strconv.FormatBool(v))
			}
			path := "/ddos-mitigation/ips"
			if len(q) > 0 {
				path += "?" + q.Encode()
			}

			s := output.NewSpinner("Fetching protected IPs...")
			s.Start()
			resp, err := client.Get(path)
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			type entry struct {
				Network            string `json:"network"`
				Prefix             int    `json:"prefix"`
				IPType             string `json:"ip_type"`
				ProtectionType     string `json:"protection_type"`
				LocationName       string `json:"location_name"`
				HasProfile         bool   `json:"has_profile"`
				FirewallRulesCount int    `json:"firewall_rules_count"`
			}
			var result struct {
				SingleIPs []entry `json:"single_ips"`
				Subnets   []entry `json:"subnets"`
			}
			if err := json.Unmarshal(resp, &result); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}

			t := output.NewTable("Protected IPs", []string{"Network", "Type", "Protection", "Location", "Profile", "Firewall Rules"})
			for _, e := range result.SingleIPs {
				t.AddRow(e.Network, e.IPType, e.ProtectionType, e.LocationName, cmdutil.YesNo(e.HasProfile), strconv.Itoa(e.FirewallRulesCount))
			}
			for _, e := range result.Subnets {
				t.AddRow(fmt.Sprintf("%s/%d", e.Network, e.Prefix), e.IPType, e.ProtectionType, e.LocationName, cmdutil.YesNo(e.HasProfile), strconv.Itoa(e.FirewallRulesCount))
			}
			t.Render()
			return nil
		},
	}
	cmd.Flags().String("ip-type", "", "Filter by IP type (IPv4 or IPv6)")
	cmd.Flags().StringP("location", "l", "", "Filter by location")
	cmd.Flags().Bool("has-profile", false, "Filter by profile presence (--has-profile=false for IPs without one)")
	return cmd
}

// --- catalogs ---

func countriesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "countries",
		Short: "List the countries available for geo filtering",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			s := output.NewSpinner("Fetching countries...")
			s.Start()
			resp, err := client.Get("/ddos-mitigation/countries")
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}
			return renderCountries("Countries", resp)
		},
	}
}

func asnsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "asns",
		Short: "Search the ASNs available for ASN filtering",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			path := "/ddos-mitigation/asns"
			if v, _ := cmd.Flags().GetString("search"); v != "" {
				path += "?search=" + url.QueryEscape(v)
			}

			s := output.NewSpinner("Fetching ASNs...")
			s.Start()
			resp, err := client.Get(path)
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}
			return renderASNs("ASNs", resp)
		},
	}
	cmd.Flags().StringP("search", "s", "", "ASN number (exact) or part of its name")
	return cmd
}

func renderCountries(title string, resp json.RawMessage) error {
	var r struct {
		Countries []struct {
			ISOCode string `json:"iso_code"`
			Name    string `json:"name"`
		} `json:"countries"`
	}
	if err := json.Unmarshal(resp, &r); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}
	t := output.NewTable(title, []string{"Code", "Name"})
	for _, c := range r.Countries {
		t.AddRow(c.ISOCode, c.Name)
	}
	t.Render()
	return nil
}

func renderASNs(title string, resp json.RawMessage) error {
	var r struct {
		ASNs []struct {
			ASN  int64  `json:"asn"`
			Name string `json:"name"`
		} `json:"asns"`
	}
	if err := json.Unmarshal(resp, &r); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}
	t := output.NewTable(title, []string{"ASN", "Name"})
	for _, a := range r.ASNs {
		t.AddRow(strconv.FormatInt(a.ASN, 10), a.Name)
	}
	t.Render()
	return nil
}

func renderPrefixLists(title string, resp json.RawMessage) error {
	var r struct {
		PrefixLists []struct {
			UUID         string `json:"uuid"`
			Name         string `json:"name"`
			Description  string `json:"description"`
			IsGlobal     bool   `json:"is_global"`
			EntriesCount int    `json:"entries_count"`
		} `json:"prefix_lists"`
	}
	if err := json.Unmarshal(resp, &r); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}
	t := output.NewTable(title, []string{"UUID", "Name", "Description", "Global", "Entries"})
	for _, p := range r.PrefixLists {
		t.AddRow(p.UUID, p.Name, p.Description, cmdutil.YesNo(p.IsGlobal), strconv.Itoa(p.EntriesCount))
	}
	t.Render()
	return nil
}

// splitList splits comma-separated flag values and drops blanks.
func splitList(values []string) []string {
	out := []string{}
	for _, v := range values {
		for _, p := range strings.Split(v, ",") {
			if p = strings.TrimSpace(p); p != "" {
				out = append(out, p)
			}
		}
	}
	return out
}
