package dns

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/CubePathInc/cubecli/internal/cmdutil"
	"github.com/CubePathInc/cubecli/internal/output"
	"github.com/spf13/cobra"
)

func NewCmd() *cobra.Command {
	dnsCmd := &cobra.Command{
		Use:   "dns",
		Short: "Manage DNS zones and records",
	}

	zoneCmd := &cobra.Command{
		Use:   "zone",
		Short: "Manage DNS zones",
	}

	zoneListCmd := &cobra.Command{
		Use:   "list",
		Short: "List DNS zones",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			path := "/dns/zones"
			projectID, _ := cmd.Flags().GetInt("project")
			if projectID > 0 {
				path = fmt.Sprintf("%s?project_id=%d", path, projectID)
			}

			s := output.NewSpinner("Fetching DNS zones...")
			s.Start()
			resp, err := client.Get(path)
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			var zones []struct {
				UUID        string   `json:"uuid"`
				Domain      string   `json:"domain"`
				Status      string   `json:"status"`
				Records     int      `json:"records"`
				Nameservers []string `json:"nameservers"`
			}
			if err := json.Unmarshal(resp, &zones); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}

			t := output.NewTable("DNS Zones", []string{"UUID", "Domain", "Status", "Records", "Nameservers"})
			for _, z := range zones {
				t.AddRow(
					z.UUID,
					z.Domain,
					output.FormatStatus(z.Status),
					strconv.Itoa(z.Records),
					strings.Join(z.Nameservers, ", "),
				)
			}
			t.Render()
			return nil
		},
	}

	zoneShowCmd := &cobra.Command{
		Use:   "show <zone_uuid>",
		Short: "Show DNS zone details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			s := output.NewSpinner("Fetching zone details...")
			s.Start()
			resp, err := client.Get(fmt.Sprintf("/dns/zones/%s", args[0]))
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			var zone struct {
				UUID        string   `json:"uuid"`
				Domain      string   `json:"domain"`
				Status      string   `json:"status"`
				Records     int      `json:"records"`
				Nameservers []string `json:"nameservers"`
			}
			if err := json.Unmarshal(resp, &zone); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}

			t := output.NewTable("Zone Details", []string{"Field", "Value"})
			t.AddRow("UUID", zone.UUID)
			t.AddRow("Domain", zone.Domain)
			t.AddRow("Status", output.FormatStatus(zone.Status))
			t.AddRow("Records", strconv.Itoa(zone.Records))
			t.AddRow("Nameservers", strings.Join(zone.Nameservers, ", "))
			t.Render()
			return nil
		},
	}

	zoneCreateCmd := &cobra.Command{
		Use:   "create <domain>",
		Short: "Create a new DNS zone",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			projectID, _ := cmd.Flags().GetInt("project")

			body := map[string]interface{}{
				"domain":     args[0],
				"project_id": projectID,
			}

			s := output.NewSpinner("Creating DNS zone...")
			s.Start()
			resp, err := client.Post("/dns/zones", body)
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			output.PrintSuccess("DNS zone created successfully")
			return nil
		},
	}

	zoneDeleteCmd := &cobra.Command{
		Use:   "delete <zone_uuid>",
		Short: "Delete a DNS zone",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			if !cmdutil.CheckForce(cmd, "Are you sure you want to delete this DNS zone?") {
				output.PrintWarning("Aborted")
				return nil
			}

			s := output.NewSpinner("Deleting DNS zone...")
			s.Start()
			resp, err := client.Delete(fmt.Sprintf("/dns/zones/%s", args[0]))
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			output.PrintSuccess("DNS zone deleted successfully")
			return nil
		},
	}

	zoneVerifyCmd := &cobra.Command{
		Use:   "verify <zone_uuid>",
		Short: "Verify a DNS zone",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			s := output.NewSpinner("Verifying DNS zone...")
			s.Start()
			resp, err := client.Post(fmt.Sprintf("/dns/zones/%s/verify", args[0]), nil)
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			var result struct {
				Verified    bool   `json:"verified"`
				Message     string `json:"message"`
				NextCheckAt string `json:"next_check_at"`
			}
			if err := json.Unmarshal(resp, &result); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}

			t := output.NewTable("Zone Verification", []string{"Field", "Value"})
			t.AddRow("Verified", strconv.FormatBool(result.Verified))
			t.AddRow("Message", result.Message)
			if result.NextCheckAt != "" {
				t.AddRow("Next Check At", result.NextCheckAt)
			}
			t.Render()

			if result.Verified {
				output.PrintSuccess("Zone verified successfully")
			} else {
				output.PrintWarning("Zone not yet verified")
			}
			return nil
		},
	}

	zoneScanCmd := &cobra.Command{
		Use:   "scan <zone_uuid>",
		Short: "Scan a DNS zone for records",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			autoImport, _ := cmd.Flags().GetBool("import")
			preview, _ := cmd.Flags().GetBool("preview")

			path := fmt.Sprintf("/dns/zones/%s/scan?auto_import=%t", args[0], autoImport)
			if preview {
				path = fmt.Sprintf("/dns/zones/%s/scan?auto_import=false", args[0])
			}

			s := output.NewSpinner("Scanning DNS zone...")
			s.Start()
			resp, err := client.Post(path, nil)
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			var result struct {
				Imported int `json:"imported"`
				Skipped  int `json:"skipped"`
				Errors   []struct {
					Message string `json:"message"`
				} `json:"errors"`
				Records []struct {
					Name    string `json:"name"`
					Type    string `json:"type"`
					Content string `json:"content"`
					TTL     int    `json:"ttl"`
				} `json:"records"`
			}
			if err := json.Unmarshal(resp, &result); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}

			t := output.NewTable("Scan Results", []string{"Field", "Value"})
			t.AddRow("Imported", strconv.Itoa(result.Imported))
			t.AddRow("Skipped", strconv.Itoa(result.Skipped))
			t.AddRow("Errors", strconv.Itoa(len(result.Errors)))
			t.Render()

			if len(result.Records) > 0 {
				rt := output.NewTable("Discovered Records", []string{"Name", "Type", "Content", "TTL"})
				for _, r := range result.Records {
					rt.AddRow(r.Name, r.Type, r.Content, strconv.Itoa(r.TTL))
				}
				rt.Render()
			}

			return nil
		},
	}

	// Zone flags
	zoneListCmd.Flags().IntP("project", "p", 0, "Filter by project ID")

	zoneCreateCmd.Flags().IntP("project", "p", 0, "Project ID")
	zoneCreateCmd.MarkFlagRequired("project")

	zoneDeleteCmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")

	zoneScanCmd.Flags().Bool("import", true, "Auto-import discovered records")
	zoneScanCmd.Flags().Bool("preview", false, "Preview records without importing")

	zoneCmd.AddCommand(zoneListCmd, zoneShowCmd, zoneCreateCmd, zoneDeleteCmd, zoneVerifyCmd, zoneScanCmd)
	dnsCmd.AddCommand(zoneCmd)

	addRecordCmd(dnsCmd)
	addSOACmd(dnsCmd)

	return dnsCmd
}
