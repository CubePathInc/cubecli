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

func NewCmd() *cobra.Command {
	networkCmd := &cobra.Command{
		Use:   "network",
		Short: "Manage networks",
	}

	networkCreateCmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new network",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			name, _ := cmd.Flags().GetString("name")
			location, _ := cmd.Flags().GetString("location")
			cidr, _ := cmd.Flags().GetString("cidr")
			projectID, _ := cmd.Flags().GetInt("project")
			label, _ := cmd.Flags().GetString("label")

			parts := strings.SplitN(cidr, "/", 2)
			if len(parts) != 2 {
				return fmt.Errorf("invalid CIDR format: %s (expected e.g. 10.0.0.0/24)", cidr)
			}
			ipRange := parts[0]
			prefix, err := strconv.Atoi(parts[1])
			if err != nil {
				return fmt.Errorf("invalid CIDR prefix: %s", parts[1])
			}

			body := map[string]interface{}{
				"name":          name,
				"location_name": location,
				"ip_range":      ipRange,
				"prefix":        prefix,
				"project_id":    projectID,
			}
			if label != "" {
				body["label"] = label
			}

			s := output.NewSpinner("Creating network...")
			s.Start()
			resp, err := client.Post("/networks/create_network", body)
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			output.PrintSuccess("Network created successfully")
			return nil
		},
	}

	networkListCmd := &cobra.Command{
		Use:   "list",
		Short: "List networks",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			filterProject, _ := cmd.Flags().GetInt("project")
			filterLocation, _ := cmd.Flags().GetString("location")

			s := output.NewSpinner("Fetching networks...")
			s.Start()
			resp, err := client.Get("/projects/")
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			var projects []struct {
				Project struct {
					ID   int    `json:"id"`
					Name string `json:"name"`
				} `json:"project"`
				Networks []struct {
					ID       int               `json:"id"`
					Name     string            `json:"name"`
					IPRange  string            `json:"ip_range"`
					Location string            `json:"location_name"`
					VPS      []json.RawMessage `json:"vps"`
				} `json:"networks"`
			}
			if err := json.Unmarshal(resp, &projects); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}

			t := output.NewTable("Networks", []string{"ID", "Name", "Project", "IP Range", "Location", "VPS Count"})
			for _, p := range projects {
				if filterProject > 0 && p.Project.ID != filterProject {
					continue
				}
				for _, n := range p.Networks {
					if filterLocation != "" && !strings.EqualFold(n.Location, filterLocation) {
						continue
					}
					t.AddRow(
						strconv.Itoa(n.ID),
						n.Name,
						p.Project.Name,
						n.IPRange,
						n.Location,
						strconv.Itoa(len(n.VPS)),
					)
				}
			}
			t.Render()
			return nil
		},
	}

	networkUpdateCmd := &cobra.Command{
		Use:   "update <network_id>",
		Short: "Update a network",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			networkID, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid network_id: %s", args[0])
			}

			name, _ := cmd.Flags().GetString("name")
			label, _ := cmd.Flags().GetString("label")

			if !cmd.Flags().Changed("name") && !cmd.Flags().Changed("label") {
				return fmt.Errorf("at least one of --name or --label must be specified")
			}

			body := map[string]string{
				"name":  name,
				"label": label,
			}

			s := output.NewSpinner("Updating network...")
			s.Start()
			resp, err := client.Put(fmt.Sprintf("/networks/%d", networkID), body)
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			output.PrintSuccess("Network updated successfully")
			return nil
		},
	}

	networkDeleteCmd := &cobra.Command{
		Use:   "delete <network_id>",
		Short: "Delete a network",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			networkID, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid network_id: %s", args[0])
			}

			if !cmdutil.CheckForce(cmd, "Are you sure you want to delete this network?") {
				output.PrintWarning("Aborted")
				return nil
			}

			s := output.NewSpinner("Deleting network...")
			s.Start()
			resp, err := client.Delete(fmt.Sprintf("/networks/%d", networkID))
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			output.PrintSuccess("Network deleted successfully")
			return nil
		},
	}

	networkCreateCmd.Flags().StringP("name", "n", "", "Name of the network")
	networkCreateCmd.Flags().StringP("location", "l", "", "Location for the network")
	networkCreateCmd.Flags().StringP("cidr", "c", "", "CIDR notation (e.g. 10.0.0.0/24)")
	networkCreateCmd.Flags().IntP("project", "p", 0, "Project ID")
	networkCreateCmd.Flags().String("label", "", "Optional label")
	networkCreateCmd.MarkFlagRequired("name")
	networkCreateCmd.MarkFlagRequired("location")
	networkCreateCmd.MarkFlagRequired("cidr")
	networkCreateCmd.MarkFlagRequired("project")

	networkListCmd.Flags().IntP("project", "p", 0, "Filter by project ID")
	networkListCmd.Flags().StringP("location", "l", "", "Filter by location")

	networkUpdateCmd.Flags().StringP("name", "n", "", "New name for the network")
	networkUpdateCmd.Flags().String("label", "", "New label for the network")

	networkDeleteCmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")

	networkCmd.AddCommand(networkCreateCmd, networkListCmd, networkUpdateCmd, networkDeleteCmd)
	return networkCmd
}
