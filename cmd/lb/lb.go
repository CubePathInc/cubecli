package lb

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/CubePathInc/cubecli/internal/cmdutil"
	"github.com/CubePathInc/cubecli/internal/output"
	"github.com/spf13/cobra"
)

func NewCmd() *cobra.Command {
	lbCmd := &cobra.Command{
		Use:   "lb",
		Short: "Manage load balancers",
	}

	lbListCmd := &cobra.Command{
		Use:   "list",
		Short: "List load balancers",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			s := output.NewSpinner("Fetching load balancers...")
			s.Start()
			resp, err := client.Get("/loadbalancer/")
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			var lbs []struct {
				UUID      string             `json:"uuid"`
				Name      string             `json:"name"`
				Status    string             `json:"status"`
				Plan      string             `json:"plan_name"`
				IP        string             `json:"floating_ip_address"`
				Listeners []json.RawMessage   `json:"listeners"`
				Location  string             `json:"location_name"`
			}
			if err := json.Unmarshal(resp, &lbs); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}

			t := output.NewTable("Load Balancers", []string{"UUID", "Name", "Status", "Plan", "IP", "Listeners", "Location"})
			for _, lb := range lbs {
				t.AddRow(
					lb.UUID,
					lb.Name,
					output.FormatStatus(lb.Status),
					lb.Plan,
					lb.IP,
					strconv.Itoa(len(lb.Listeners)),
					lb.Location,
				)
			}
			t.Render()
			return nil
		},
	}

	lbShowCmd := &cobra.Command{
		Use:   "show <lb_uuid>",
		Short: "Show load balancer details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			lbUUID := args[0]

			s := output.NewSpinner("Fetching load balancer...")
			s.Start()
			resp, err := client.Get("/loadbalancer/")
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				// Filter to just the matching LB for JSON output
				var lbs []json.RawMessage
				if err := json.Unmarshal(resp, &lbs); err != nil {
					return fmt.Errorf("failed to parse response: %w", err)
				}
				for _, raw := range lbs {
					var item struct {
						UUID string `json:"uuid"`
					}
					if err := json.Unmarshal(raw, &item); err != nil {
						continue
					}
					if item.UUID == lbUUID {
						return output.PrintJSON(raw)
					}
				}
				return fmt.Errorf("load balancer %s not found", lbUUID)
			}

			var lbs []struct {
				UUID     string `json:"uuid"`
				Name     string `json:"name"`
				Status   string `json:"status"`
				Plan     string `json:"plan_name"`
				IP       string `json:"floating_ip_address"`
				Location string `json:"location_name"`
				Label    string `json:"label"`
				Listeners []struct {
					UUID           string `json:"uuid"`
					Name           string `json:"name"`
					Protocol       string `json:"protocol"`
					SourcePort     int    `json:"source_port"`
					TargetPort     int    `json:"target_port"`
					Algorithm      string `json:"algorithm"`
					Enabled        bool   `json:"enabled"`
					StickySessions bool   `json:"sticky_sessions"`
					Targets        []struct {
						UUID       string `json:"uuid"`
						TargetType string `json:"target_type"`
						TargetUUID string `json:"target_uuid"`
						Port       int    `json:"port"`
						Weight     int    `json:"weight"`
						Status     string `json:"status"`
						Enabled    bool   `json:"enabled"`
					} `json:"targets"`
				} `json:"listeners"`
			}
			if err := json.Unmarshal(resp, &lbs); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}

			var found bool
			for _, lb := range lbs {
				if lb.UUID != lbUUID {
					continue
				}
				found = true

				// Info table
				info := output.NewTable("Load Balancer Info", []string{"Field", "Value"})
				info.AddRow("UUID", lb.UUID)
				info.AddRow("Name", lb.Name)
				info.AddRow("Status", output.FormatStatus(lb.Status))
				info.AddRow("Plan", lb.Plan)
				info.AddRow("IP", lb.IP)
				info.AddRow("Location", lb.Location)
				if lb.Label != "" {
					info.AddRow("Label", lb.Label)
				}
				info.Render()

				// Listeners table
				lt := output.NewTable("Listeners", []string{"UUID", "Name", "Protocol", "Port", "Target Port", "Algorithm", "Sticky", "Enabled"})
				for _, l := range lb.Listeners {
					stickyStr := "no"
					if l.StickySessions {
						stickyStr = "yes"
					}
					enabledStr := "no"
					if l.Enabled {
						enabledStr = "yes"
					}
					lt.AddRow(
						l.UUID,
						l.Name,
						l.Protocol,
						strconv.Itoa(l.SourcePort),
						strconv.Itoa(l.TargetPort),
						l.Algorithm,
						stickyStr,
						enabledStr,
					)
				}
				lt.Render()

				// Targets per listener
				for _, l := range lb.Listeners {
					tt := output.NewTable(fmt.Sprintf("Targets for %s", l.Name), []string{"UUID", "Type", "Target UUID", "Port", "Weight", "Status", "Enabled"})
					for _, tgt := range l.Targets {
						enabledStr := "no"
						if tgt.Enabled {
							enabledStr = "yes"
						}
						tt.AddRow(
							tgt.UUID,
							tgt.TargetType,
							tgt.TargetUUID,
							strconv.Itoa(tgt.Port),
							strconv.Itoa(tgt.Weight),
							output.FormatStatus(tgt.Status),
							enabledStr,
						)
					}
					tt.Render()
				}
				break
			}

			if !found {
				return fmt.Errorf("load balancer %s not found", lbUUID)
			}
			return nil
		},
	}

	lbCreateCmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new load balancer",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			name, _ := cmd.Flags().GetString("name")
			plan, _ := cmd.Flags().GetString("plan")
			location, _ := cmd.Flags().GetString("location")
			projectID, _ := cmd.Flags().GetInt("project")
			label, _ := cmd.Flags().GetString("label")

			body := map[string]interface{}{
				"name":          name,
				"plan_name":     plan,
				"location_name": location,
			}
			if projectID > 0 {
				body["project_id"] = projectID
			}
			if label != "" {
				body["label"] = label
			}

			s := output.NewSpinner("Creating load balancer...")
			s.Start()
			resp, err := client.Post("/loadbalancer/", body)
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			output.PrintSuccess("Load balancer created successfully")
			return nil
		},
	}

	lbUpdateCmd := &cobra.Command{
		Use:   "update <lb_uuid>",
		Short: "Update a load balancer",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			lbUUID := args[0]

			body := map[string]string{}
			if cmd.Flags().Changed("name") {
				name, _ := cmd.Flags().GetString("name")
				body["name"] = name
			}
			if cmd.Flags().Changed("label") {
				label, _ := cmd.Flags().GetString("label")
				body["label"] = label
			}

			if len(body) == 0 {
				return fmt.Errorf("at least one of --name or --label must be specified")
			}

			s := output.NewSpinner("Updating load balancer...")
			s.Start()
			resp, err := client.Patch(fmt.Sprintf("/loadbalancer/%s", lbUUID), body)
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			output.PrintSuccess("Load balancer updated successfully")
			return nil
		},
	}

	lbDeleteCmd := &cobra.Command{
		Use:   "delete <lb_uuid>",
		Short: "Delete a load balancer",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			lbUUID := args[0]

			if !cmdutil.CheckForce(cmd, "Are you sure you want to delete this load balancer?") {
				output.PrintWarning("Aborted")
				return nil
			}

			s := output.NewSpinner("Deleting load balancer...")
			s.Start()
			resp, err := client.Delete(fmt.Sprintf("/loadbalancer/%s", lbUUID))
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			output.PrintSuccess("Load balancer deleted successfully")
			return nil
		},
	}

	lbResizeCmd := &cobra.Command{
		Use:   "resize <lb_uuid>",
		Short: "Resize a load balancer",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			lbUUID := args[0]

			plan, _ := cmd.Flags().GetString("plan")

			body := map[string]string{
				"plan_name": plan,
			}

			s := output.NewSpinner("Resizing load balancer...")
			s.Start()
			resp, err := client.Post(fmt.Sprintf("/loadbalancer/%s/resize", lbUUID), body)
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			output.PrintSuccess("Load balancer resized successfully")
			return nil
		},
	}

	// Flags for create
	lbCreateCmd.Flags().StringP("name", "n", "", "Name of the load balancer")
	lbCreateCmd.Flags().StringP("plan", "p", "", "Plan name")
	lbCreateCmd.Flags().StringP("location", "l", "", "Location name")
	lbCreateCmd.Flags().Int("project", 0, "Project ID")
	lbCreateCmd.Flags().String("label", "", "Optional label")
	lbCreateCmd.MarkFlagRequired("name")
	lbCreateCmd.MarkFlagRequired("plan")
	lbCreateCmd.MarkFlagRequired("location")

	// Flags for update
	lbUpdateCmd.Flags().StringP("name", "n", "", "New name for the load balancer")
	lbUpdateCmd.Flags().String("label", "", "New label for the load balancer")

	// Flags for delete
	lbDeleteCmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")

	// Flags for resize
	lbResizeCmd.Flags().StringP("plan", "p", "", "New plan name")
	lbResizeCmd.MarkFlagRequired("plan")

	lbCmd.AddCommand(lbListCmd, lbShowCmd, lbCreateCmd, lbUpdateCmd, lbDeleteCmd, lbResizeCmd)

	addListenerCmd(lbCmd)
	addTargetCmd(lbCmd)
	addHealthCheckCmd(lbCmd)
	addPlanCmd(lbCmd)

	return lbCmd
}
