package floatingip

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/CubePathInc/cubecli/internal/cmdutil"
	"github.com/CubePathInc/cubecli/internal/output"
	"github.com/spf13/cobra"
)

func NewCmd() *cobra.Command {
	floatingIPCmd := &cobra.Command{
		Use:   "floating-ip",
		Short: "Manage floating IPs",
	}

	floatingIPListCmd := &cobra.Command{
		Use:   "list",
		Short: "List floating IPs",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			filterLocation, _ := cmd.Flags().GetString("location")
			verbose, _ := cmd.Flags().GetBool("verbose")

			s := output.NewSpinner("Fetching floating IPs...")
			s.Start()
			resp, err := client.Get("/floating_ips/organization")
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			type apiIP struct {
				Address        string  `json:"address"`
				Type           string  `json:"type"`
				Status         string  `json:"status"`
				IsPrimary      bool    `json:"is_primary"`
				VPSName        *string `json:"vps_name"`
				BaremetalName  *string `json:"baremetal_name"`
				LocationName   string  `json:"location_name"`
				ProtectionType string  `json:"protection_type"`
			}

			var result struct {
				SingleIPs []apiIP `json:"single_ips"`
				Subnets   []struct {
					Prefix         int    `json:"prefix"`
					ProtectionType string `json:"protection_type"`
					IPAddresses    []apiIP `json:"ip_addresses"`
				} `json:"subnets"`
			}
			if err := json.Unmarshal(resp, &result); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}

			type ipEntry struct {
				IPAddress      string
				Role           string
				Status         string
				AssignedTo     string
				Location       string
				DDoSProtection string
			}

			toEntry := func(ip apiIP) ipEntry {
				role := "secondary"
				if ip.IsPrimary {
					role = "primary"
				}
				assignedTo := "unassigned"
				if ip.VPSName != nil && *ip.VPSName != "" {
					assignedTo = *ip.VPSName
				} else if ip.BaremetalName != nil && *ip.BaremetalName != "" {
					assignedTo = *ip.BaremetalName
				}
				return ipEntry{
					IPAddress:      ip.Address,
					Role:           role,
					Status:         ip.Status,
					AssignedTo:     assignedTo,
					Location:       ip.LocationName,
					DDoSProtection: ip.ProtectionType,
				}
			}

			var allIPs []ipEntry
			for _, ip := range result.SingleIPs {
				allIPs = append(allIPs, toEntry(ip))
			}
			for _, subnet := range result.Subnets {
				for _, ip := range subnet.IPAddresses {
					allIPs = append(allIPs, toEntry(ip))
				}
			}

			// Apply filters
			var filtered []ipEntry
			for _, ip := range allIPs {
				if filterLocation != "" && !strings.EqualFold(ip.Location, filterLocation) {
					continue
				}
				filtered = append(filtered, ip)
			}

			if verbose {
				t := output.NewTable("Floating IPs", []string{"IP Address", "Role", "Status", "Assigned To", "Location", "DDoS Protection"})
				for _, ip := range filtered {
					t.AddRow(ip.IPAddress, ip.Role, output.FormatStatus(ip.Status), ip.AssignedTo, ip.Location, ip.DDoSProtection)
				}
				t.Render()
			} else {
				t := output.NewTable("Floating IPs", []string{"IP Address", "Role", "Status", "Assigned To", "Location"})
				for _, ip := range filtered {
					t.AddRow(ip.IPAddress, ip.Role, output.FormatStatus(ip.Status), ip.AssignedTo, ip.Location)
				}
				t.Render()
			}
			return nil
		},
	}

	floatingIPAcquireCmd := &cobra.Command{
		Use:   "acquire",
		Short: "Acquire a new floating IP",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			ipType, _ := cmd.Flags().GetString("type")
			location, _ := cmd.Flags().GetString("location")

			s := output.NewSpinner("Acquiring floating IP...")
			s.Start()
			resp, err := client.Post(fmt.Sprintf("/floating_ips/acquire?ip_type=%s&location_name=%s", ipType, location), nil)
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			output.PrintSuccess("Floating IP acquired successfully")
			return nil
		},
	}

	floatingIPReleaseCmd := &cobra.Command{
		Use:   "release <address>",
		Short: "Release a floating IP",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			address := args[0]

			if !cmdutil.CheckForce(cmd, fmt.Sprintf("Are you sure you want to release floating IP %s?", address)) {
				output.PrintWarning("Aborted")
				return nil
			}

			s := output.NewSpinner("Releasing floating IP...")
			s.Start()
			resp, err := client.Post(fmt.Sprintf("/floating_ips/release/%s", address), nil)
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			output.PrintSuccess("Floating IP released successfully")
			return nil
		},
	}

	floatingIPAssignCmd := &cobra.Command{
		Use:   "assign <address>",
		Short: "Assign a floating IP to a server",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			address := args[0]

			vpsID, _ := cmd.Flags().GetInt("vps")
			baremetalID, _ := cmd.Flags().GetInt("baremetal")

			if vpsID == 0 && baremetalID == 0 {
				return fmt.Errorf("exactly one of --vps or --baremetal is required")
			}
			if vpsID != 0 && baremetalID != 0 {
				return fmt.Errorf("exactly one of --vps or --baremetal is required, not both")
			}

			var endpoint string
			if vpsID != 0 {
				endpoint = fmt.Sprintf("/floating_ips/assign/vps/%d?address=%s", vpsID, address)
			} else {
				endpoint = fmt.Sprintf("/floating_ips/assign/baremetal/%d?address=%s", baremetalID, address)
			}

			s := output.NewSpinner("Assigning floating IP...")
			s.Start()
			resp, err := client.Post(endpoint, nil)
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			output.PrintSuccess("Floating IP assigned successfully")
			return nil
		},
	}

	floatingIPUnassignCmd := &cobra.Command{
		Use:   "unassign <address>",
		Short: "Unassign a floating IP from a server",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			address := args[0]

			if !cmdutil.CheckForce(cmd, fmt.Sprintf("Are you sure you want to unassign floating IP %s?", address)) {
				output.PrintWarning("Aborted")
				return nil
			}

			s := output.NewSpinner("Unassigning floating IP...")
			s.Start()
			resp, err := client.Post(fmt.Sprintf("/floating_ips/unassign/%s", address), nil)
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			output.PrintSuccess("Floating IP unassigned successfully")
			return nil
		},
	}

	floatingIPReverseDNSCmd := &cobra.Command{
		Use:   "reverse-dns <ip>",
		Short: "Configure reverse DNS for a floating IP",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			ip := args[0]

			hostname, _ := cmd.Flags().GetString("hostname")

			s := output.NewSpinner("Configuring reverse DNS...")
			s.Start()
			resp, err := client.Post(fmt.Sprintf("/floating_ips/reverse_dns/configure?ip=%s&reverse_dns=%s", ip, hostname), nil)
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			output.PrintSuccess("Reverse DNS configured successfully")
			return nil
		},
	}

	floatingIPListCmd.Flags().StringP("location", "l", "", "Filter by location")

	floatingIPAcquireCmd.Flags().StringP("type", "t", "IPv4", "IP type (IPv4 or IPv6)")
	floatingIPAcquireCmd.Flags().StringP("location", "l", "", "Location name")
	floatingIPAcquireCmd.MarkFlagRequired("location")

	floatingIPReleaseCmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")

	floatingIPAssignCmd.Flags().Int("vps", 0, "VPS ID to assign to")
	floatingIPAssignCmd.Flags().Int("baremetal", 0, "Baremetal ID to assign to")

	floatingIPUnassignCmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")

	floatingIPReverseDNSCmd.Flags().StringP("hostname", "r", "", "Reverse DNS hostname (empty string to delete)")
	floatingIPReverseDNSCmd.MarkFlagRequired("hostname")

	floatingIPCmd.AddCommand(floatingIPListCmd, floatingIPAcquireCmd, floatingIPReleaseCmd, floatingIPAssignCmd, floatingIPUnassignCmd, floatingIPReverseDNSCmd)
	return floatingIPCmd
}
