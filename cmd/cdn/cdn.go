package cdn

import (
	"encoding/json"
	"fmt"

	"github.com/CubePathInc/cubecli/internal/cmdutil"
	"github.com/CubePathInc/cubecli/internal/output"
	"github.com/spf13/cobra"
)

func NewCmd() *cobra.Command {
	cdnCmd := &cobra.Command{
		Use:   "cdn",
		Short: "Manage CDN zones and distribution",
	}

	zoneCmd := &cobra.Command{
		Use:   "zone",
		Short: "Manage CDN zones",
	}

	zoneListCmd := &cobra.Command{
		Use:   "list",
		Short: "List CDN zones",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			s := output.NewSpinner("Fetching CDN zones...")
			s.Start()
			resp, err := client.Get("/cdn/zones")
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			var zones []struct {
				UUID         string          `json:"uuid"`
				Name         string          `json:"name"`
				Domain       string          `json:"domain"`
				CustomDomain string          `json:"custom_domain"`
				Status       string          `json:"status"`
				Plan         json.RawMessage `json:"plan"`
			}
			if err := json.Unmarshal(resp, &zones); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}

			t := output.NewTable("CDN Zones", []string{"UUID", "Name", "Domain", "Custom Domain", "Status", "Plan"})
			for _, z := range zones {
				t.AddRow(
					z.UUID,
					z.Name,
					z.Domain,
					z.CustomDomain,
					output.FormatStatus(z.Status),
					extractPlanName(z.Plan),
				)
			}
			t.Render()
			return nil
		},
	}

	zoneShowCmd := &cobra.Command{
		Use:   "show <zone_uuid>",
		Short: "Show CDN zone details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			zoneUUID := args[0]

			s := output.NewSpinner("Fetching CDN zone details...")
			s.Start()
			resp, err := client.Get(fmt.Sprintf("/cdn/zones/%s", zoneUUID))
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			var zone struct {
				UUID         string          `json:"uuid"`
				Name         string          `json:"name"`
				Domain       string          `json:"domain"`
				CustomDomain string          `json:"custom_domain"`
				Status       string          `json:"status"`
				Plan         json.RawMessage `json:"plan"`
				SSLType      string          `json:"ssl_type"`
				Certificate  string          `json:"certificate"`
				CreatedAt    string          `json:"created_at"`
				UpdatedAt    string          `json:"updated_at"`
			}
			if err := json.Unmarshal(resp, &zone); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}

			planName := extractPlanName(zone.Plan)

			t := output.NewTable("CDN Zone Details", []string{"Field", "Value"})
			t.AddRow("UUID", zone.UUID)
			t.AddRow("Name", zone.Name)
			t.AddRow("Domain", zone.Domain)
			t.AddRow("Custom Domain", zone.CustomDomain)
			t.AddRow("Status", output.FormatStatus(zone.Status))
			t.AddRow("Plan", planName)
			t.AddRow("SSL Type", zone.SSLType)
			t.AddRow("Certificate", zone.Certificate)
			t.AddRow("Created At", zone.CreatedAt)
			t.AddRow("Updated At", zone.UpdatedAt)
			t.Render()
			return nil
		},
	}

	zoneCreateCmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new CDN zone",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			name, _ := cmd.Flags().GetString("name")
			plan, _ := cmd.Flags().GetString("plan")
			domain, _ := cmd.Flags().GetString("domain")
			projectID, _ := cmd.Flags().GetInt("project")

			if len(name) < 3 || len(name) > 32 {
				return fmt.Errorf("name must be between 3 and 32 characters")
			}

			body := map[string]interface{}{
				"name":      name,
				"plan_name": plan,
			}
			if domain != "" {
				body["custom_domain"] = domain
			}
			if projectID > 0 {
				body["project_id"] = projectID
			}

			s := output.NewSpinner("Creating CDN zone...")
			s.Start()
			resp, err := client.Post("/cdn/zones", body)
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			var zone struct {
				UUID   string `json:"uuid"`
				Name   string `json:"name"`
				Domain string `json:"domain"`
				Status string `json:"status"`
			}
			if err := json.Unmarshal(resp, &zone); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}

			t := output.NewTable("CDN Zone Created", []string{"Field", "Value"})
			t.AddRow("UUID", zone.UUID)
			t.AddRow("Name", zone.Name)
			t.AddRow("Domain", zone.Domain)
			t.AddRow("Status", output.FormatStatus(zone.Status))
			t.Render()

			output.PrintSuccess("CDN zone created successfully")
			return nil
		},
	}

	zoneUpdateCmd := &cobra.Command{
		Use:   "update <zone_uuid>",
		Short: "Update a CDN zone",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			zoneUUID := args[0]

			body := map[string]interface{}{}
			if cmd.Flags().Changed("name") {
				name, _ := cmd.Flags().GetString("name")
				body["name"] = name
			}
			if cmd.Flags().Changed("domain") {
				domain, _ := cmd.Flags().GetString("domain")
				body["custom_domain"] = domain
			}
			if cmd.Flags().Changed("ssl-type") {
				sslType, _ := cmd.Flags().GetString("ssl-type")
				body["ssl_type"] = sslType
			}
			if cmd.Flags().Changed("certificate") {
				cert, _ := cmd.Flags().GetString("certificate")
				body["certificate"] = cert
			}

			if len(body) == 0 {
				return fmt.Errorf("at least one of --name, --domain, --ssl-type, or --certificate must be specified")
			}

			s := output.NewSpinner("Updating CDN zone...")
			s.Start()
			resp, err := client.Patch(fmt.Sprintf("/cdn/zones/%s", zoneUUID), body)
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			output.PrintSuccess("CDN zone updated successfully")
			return nil
		},
	}

	zoneDeleteCmd := &cobra.Command{
		Use:   "delete <zone_uuid>",
		Short: "Delete a CDN zone",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			zoneUUID := args[0]

			if !cmdutil.CheckForce(cmd, fmt.Sprintf("Are you sure you want to delete CDN zone %s?", zoneUUID)) {
				output.PrintWarning("Aborted")
				return nil
			}

			s := output.NewSpinner("Deleting CDN zone...")
			s.Start()
			resp, err := client.Delete(fmt.Sprintf("/cdn/zones/%s", zoneUUID))
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			output.PrintSuccess("CDN zone deleted successfully")
			return nil
		},
	}

	zonePricingCmd := &cobra.Command{
		Use:   "pricing <zone_uuid>",
		Short: "Show CDN zone pricing",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			zoneUUID := args[0]

			s := output.NewSpinner("Fetching CDN zone pricing...")
			s.Start()
			resp, err := client.Get(fmt.Sprintf("/cdn/zones/%s/pricing", zoneUUID))
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			var pricing map[string]interface{}
			if err := json.Unmarshal(resp, &pricing); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}

			t := output.NewTable("CDN Zone Pricing", []string{"Field", "Value"})
			for k, v := range pricing {
				t.AddRow(k, fmt.Sprintf("%v", v))
			}
			t.Render()
			return nil
		},
	}

	// Zone create flags
	zoneCreateCmd.Flags().StringP("name", "n", "", "Name of the CDN zone (3-32 characters)")
	zoneCreateCmd.Flags().StringP("plan", "p", "", "Plan name")
	zoneCreateCmd.Flags().StringP("domain", "d", "", "Custom domain")
	zoneCreateCmd.Flags().Int("project", 0, "Project ID")
	zoneCreateCmd.MarkFlagRequired("name")
	zoneCreateCmd.MarkFlagRequired("plan")

	// Zone update flags
	zoneUpdateCmd.Flags().StringP("name", "n", "", "New name for the CDN zone")
	zoneUpdateCmd.Flags().StringP("domain", "d", "", "Custom domain")
	zoneUpdateCmd.Flags().String("ssl-type", "", "SSL type")
	zoneUpdateCmd.Flags().String("certificate", "", "SSL certificate")

	// Zone delete flags
	zoneDeleteCmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")

	// Build zone subcommand tree
	zoneCmd.AddCommand(zoneListCmd, zoneShowCmd, zoneCreateCmd, zoneUpdateCmd, zoneDeleteCmd, zonePricingCmd)
	cdnCmd.AddCommand(zoneCmd)

	// Add sub-command groups
	addOriginCmd(cdnCmd)
	addRuleCmd(cdnCmd)
	addWAFCmd(cdnCmd)
	addMetricsCmd(cdnCmd)
	addPlanCmd(cdnCmd)

	return cdnCmd
}

func extractPlanName(raw json.RawMessage) string {
	if raw == nil || string(raw) == "null" {
		return ""
	}
	// Try as string first
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	// Try as object with name/plan_name
	var obj struct {
		Name     string `json:"name"`
		PlanName string `json:"plan_name"`
	}
	if err := json.Unmarshal(raw, &obj); err == nil {
		if obj.PlanName != "" {
			return obj.PlanName
		}
		return obj.Name
	}
	return ""
}
