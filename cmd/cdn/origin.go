package cdn

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/CubePathInc/cubecli/internal/cmdutil"
	"github.com/CubePathInc/cubecli/internal/output"
	"github.com/spf13/cobra"
)

func addOriginCmd(parent *cobra.Command) {
	originCmd := &cobra.Command{
		Use:   "origin",
		Short: "Manage CDN origins",
	}

	originListCmd := &cobra.Command{
		Use:   "list <zone_uuid>",
		Short: "List CDN origins for a zone",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			zoneUUID := args[0]

			s := output.NewSpinner("Fetching CDN origins...")
			s.Start()
			resp, err := client.Get(fmt.Sprintf("/cdn/zones/%s/origins", zoneUUID))
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			var origins []struct {
				UUID     string `json:"uuid"`
				Name     string `json:"name"`
				Address  string `json:"address"`
				Port     int    `json:"port"`
				Protocol string `json:"protocol"`
				Weight   int    `json:"weight"`
				Priority int    `json:"priority"`
				Backup   bool   `json:"is_backup"`
				Health   string `json:"health"`
				Enabled  bool   `json:"enabled"`
			}
			if err := json.Unmarshal(resp, &origins); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}

			t := output.NewTable("CDN Origins", []string{"UUID", "Name", "Address", "Port", "Protocol", "Weight", "Priority", "Backup", "Health", "Enabled"})
			for _, o := range origins {
				t.AddRow(
					o.UUID,
					o.Name,
					o.Address,
					strconv.Itoa(o.Port),
					o.Protocol,
					strconv.Itoa(o.Weight),
					strconv.Itoa(o.Priority),
					strconv.FormatBool(o.Backup),
					o.Health,
					strconv.FormatBool(o.Enabled),
				)
			}
			t.Render()
			return nil
		},
	}

	originCreateCmd := &cobra.Command{
		Use:   "create <zone_uuid>",
		Short: "Create a new CDN origin",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			zoneUUID := args[0]

			name, _ := cmd.Flags().GetString("name")
			originURL, _ := cmd.Flags().GetString("url")
			address, _ := cmd.Flags().GetString("address")
			port, _ := cmd.Flags().GetInt("port")
			protocol, _ := cmd.Flags().GetString("protocol")
			weight, _ := cmd.Flags().GetInt("weight")
			priority, _ := cmd.Flags().GetInt("priority")
			backup, _ := cmd.Flags().GetBool("backup")
			healthPath, _ := cmd.Flags().GetString("health-path")
			noHealthCheck, _ := cmd.Flags().GetBool("no-health-check")
			noVerifySSL, _ := cmd.Flags().GetBool("no-verify-ssl")
			hostHeader, _ := cmd.Flags().GetString("host-header")
			basePath, _ := cmd.Flags().GetString("base-path")

			body := map[string]interface{}{
				"name":                 name,
				"weight":               weight,
				"priority":             priority,
				"is_backup":            backup,
				"health_check_enabled": !noHealthCheck,
				"health_check_path":    healthPath,
				"verify_ssl":           !noVerifySSL,
				"enabled":              true,
			}
			if originURL != "" {
				body["origin_url"] = originURL
			}
			if address != "" {
				body["address"] = address
			}
			if port > 0 {
				body["port"] = port
			}
			if protocol != "" {
				body["protocol"] = protocol
			}
			if hostHeader != "" {
				body["host_header"] = hostHeader
			}
			if basePath != "" {
				body["base_path"] = basePath
			}

			s := output.NewSpinner("Creating CDN origin...")
			s.Start()
			resp, err := client.Post(fmt.Sprintf("/cdn/zones/%s/origins", zoneUUID), body)
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			var origin struct {
				UUID    string `json:"uuid"`
				Name    string `json:"name"`
				Address string `json:"address"`
			}
			if err := json.Unmarshal(resp, &origin); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}

			t := output.NewTable("CDN Origin Created", []string{"Field", "Value"})
			t.AddRow("UUID", origin.UUID)
			t.AddRow("Name", origin.Name)
			t.AddRow("Address", origin.Address)
			t.Render()

			output.PrintSuccess("CDN origin created successfully")
			return nil
		},
	}

	originUpdateCmd := &cobra.Command{
		Use:   "update <zone_uuid> <origin_uuid>",
		Short: "Update a CDN origin",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			zoneUUID := args[0]
			originUUID := args[1]

			body := map[string]interface{}{}
			if cmd.Flags().Changed("name") {
				name, _ := cmd.Flags().GetString("name")
				body["name"] = name
			}
			if cmd.Flags().Changed("address") {
				address, _ := cmd.Flags().GetString("address")
				body["address"] = address
			}
			if cmd.Flags().Changed("port") {
				port, _ := cmd.Flags().GetInt("port")
				body["port"] = port
			}
			if cmd.Flags().Changed("protocol") {
				protocol, _ := cmd.Flags().GetString("protocol")
				body["protocol"] = protocol
			}
			if cmd.Flags().Changed("weight") {
				weight, _ := cmd.Flags().GetInt("weight")
				body["weight"] = weight
			}
			if cmd.Flags().Changed("priority") {
				priority, _ := cmd.Flags().GetInt("priority")
				body["priority"] = priority
			}
			if cmd.Flags().Changed("host-header") {
				hostHeader, _ := cmd.Flags().GetString("host-header")
				body["host_header"] = hostHeader
			}
			if cmd.Flags().Changed("base-path") {
				basePath, _ := cmd.Flags().GetString("base-path")
				body["base_path"] = basePath
			}

			if len(body) == 0 {
				return fmt.Errorf("at least one flag must be specified")
			}

			s := output.NewSpinner("Updating CDN origin...")
			s.Start()
			resp, err := client.Patch(fmt.Sprintf("/cdn/zones/%s/origins/%s", zoneUUID, originUUID), body)
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			output.PrintSuccess("CDN origin updated successfully")
			return nil
		},
	}

	originDeleteCmd := &cobra.Command{
		Use:   "delete <zone_uuid> <origin_uuid>",
		Short: "Delete a CDN origin",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			zoneUUID := args[0]
			originUUID := args[1]

			if !cmdutil.CheckForce(cmd, fmt.Sprintf("Are you sure you want to delete CDN origin %s?", originUUID)) {
				output.PrintWarning("Aborted")
				return nil
			}

			s := output.NewSpinner("Deleting CDN origin...")
			s.Start()
			resp, err := client.Delete(fmt.Sprintf("/cdn/zones/%s/origins/%s", zoneUUID, originUUID))
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			output.PrintSuccess("CDN origin deleted successfully")
			return nil
		},
	}

	// Origin create flags
	originCreateCmd.Flags().StringP("name", "n", "", "Name of the origin")
	originCreateCmd.Flags().StringP("url", "u", "", "Origin URL")
	originCreateCmd.Flags().StringP("address", "a", "", "Origin address")
	originCreateCmd.Flags().IntP("port", "p", 0, "Origin port")
	originCreateCmd.Flags().String("protocol", "", "Origin protocol")
	originCreateCmd.Flags().IntP("weight", "w", 100, "Origin weight")
	originCreateCmd.Flags().Int("priority", 1, "Origin priority")
	originCreateCmd.Flags().Bool("backup", false, "Mark origin as backup")
	originCreateCmd.Flags().String("health-path", "/health", "Health check path")
	originCreateCmd.Flags().Bool("no-health-check", false, "Disable health checks")
	originCreateCmd.Flags().Bool("no-verify-ssl", false, "Disable SSL verification")
	originCreateCmd.Flags().String("host-header", "", "Host header override")
	originCreateCmd.Flags().String("base-path", "", "Base path for the origin")
	originCreateCmd.MarkFlagRequired("name")

	// Origin update flags
	originUpdateCmd.Flags().StringP("name", "n", "", "New name for the origin")
	originUpdateCmd.Flags().StringP("address", "a", "", "New address")
	originUpdateCmd.Flags().IntP("port", "p", 0, "New port")
	originUpdateCmd.Flags().String("protocol", "", "New protocol")
	originUpdateCmd.Flags().IntP("weight", "w", 0, "New weight")
	originUpdateCmd.Flags().Int("priority", 0, "New priority")
	originUpdateCmd.Flags().String("host-header", "", "New host header override")
	originUpdateCmd.Flags().String("base-path", "", "New base path")

	// Origin delete flags
	originDeleteCmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")

	originCmd.AddCommand(originListCmd, originCreateCmd, originUpdateCmd, originDeleteCmd)
	parent.AddCommand(originCmd)
}
