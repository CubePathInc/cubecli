package availabilitygroup

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/CubePathInc/cubecli/internal/cmdutil"
	"github.com/CubePathInc/cubecli/internal/output"
	"github.com/spf13/cobra"
)

func NewCmd() *cobra.Command {
	agCmd := &cobra.Command{
		Use:     "availability-group",
		Aliases: []string{"ag"},
		Short:   "Manage VPS availability groups",
	}

	agCmd.AddCommand(
		listCmd(),
		showCmd(),
		createCmd(),
		deleteCmd(),
		addVPSCmd(),
		removeVPSCmd(),
	)

	return agCmd
}

// --- list ---

func listCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list <project_id>",
		Short: "List availability groups for a project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			projectID, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid project_id: %s", args[0])
			}

			path := fmt.Sprintf("/vps/availability-groups/project/%d", projectID)
			if loc, _ := cmd.Flags().GetString("location"); loc != "" {
				path += "?location_name=" + loc
			}

			s := output.NewSpinner("Fetching availability groups...")
			s.Start()
			resp, err := client.Get(path)
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			var result struct {
				Groups []struct {
					UUID         string `json:"uuid"`
					Name         string `json:"name"`
					Strategy     string `json:"strategy"`
					LocationName string `json:"location_name"`
					MaxServers   int    `json:"max_servers"`
					VPSCount     int    `json:"vps_count"`
				} `json:"groups"`
			}
			if err := json.Unmarshal(resp, &result); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}

			if len(result.Groups) == 0 {
				fmt.Println("No availability groups found for this project.")
				return nil
			}

			t := output.NewTable("Availability Groups", []string{"UUID", "Name", "Strategy", "Location", "VPS", "Max"})
			for _, g := range result.Groups {
				t.AddRow(
					g.UUID,
					g.Name,
					g.Strategy,
					g.LocationName,
					strconv.Itoa(g.VPSCount),
					strconv.Itoa(g.MaxServers),
				)
			}
			t.Render()
			return nil
		},
	}
	cmd.Flags().StringP("location", "l", "", "Filter by location")
	return cmd
}

// --- show ---

func showCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <group_uuid>",
		Short: "Show availability group details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			s := output.NewSpinner("Fetching availability group...")
			s.Start()
			resp, err := client.Get("/vps/availability-groups/" + args[0])
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			var g struct {
				UUID         string `json:"uuid"`
				Name         string `json:"name"`
				Description  string `json:"description"`
				Strategy     string `json:"strategy"`
				LocationName string `json:"location_name"`
				MaxServers   int    `json:"max_servers"`
				VPSList      []struct {
					ID     int    `json:"id"`
					Name   string `json:"name"`
					Status string `json:"status"`
				} `json:"vps_list"`
			}
			if err := json.Unmarshal(resp, &g); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}

			info := output.NewTable("Availability Group", []string{"Field", "Value"})
			info.AddRow("UUID", g.UUID)
			info.AddRow("Name", g.Name)
			if g.Description != "" {
				info.AddRow("Description", g.Description)
			}
			info.AddRow("Strategy", g.Strategy)
			info.AddRow("Location", g.LocationName)
			info.AddRow("Max Servers", strconv.Itoa(g.MaxServers))
			info.Render()

			if len(g.VPSList) > 0 {
				vt := output.NewTable("VPS in Group", []string{"ID", "Name", "Status"})
				for _, v := range g.VPSList {
					vt.AddRow(strconv.Itoa(v.ID), v.Name, output.FormatStatus(v.Status))
				}
				vt.Render()
			}
			return nil
		},
	}
}

// --- create ---

func createCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new availability group",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			projectID, _ := cmd.Flags().GetInt("project")
			name, _ := cmd.Flags().GetString("name")
			location, _ := cmd.Flags().GetString("location")
			description, _ := cmd.Flags().GetString("description")
			strategy, _ := cmd.Flags().GetString("strategy")

			body := map[string]interface{}{
				"project_id":    projectID,
				"name":          name,
				"location_name": location,
			}
			if description != "" {
				body["description"] = description
			}
			if strategy != "" {
				body["strategy"] = strategy
			}

			s := output.NewSpinner("Creating availability group...")
			s.Start()
			resp, err := client.Post("/vps/availability-groups/", body)
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			output.PrintSuccess("Availability group created successfully")
			return nil
		},
	}
	cmd.Flags().IntP("project", "p", 0, "Project ID")
	cmd.Flags().StringP("name", "n", "", "Group name")
	cmd.Flags().StringP("location", "l", "", "Location name")
	cmd.Flags().String("description", "", "Optional description")
	cmd.Flags().String("strategy", "", "Placement strategy (spread or cluster)")
	_ = cmd.MarkFlagRequired("project")
	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("location")
	return cmd
}

// --- delete ---

func deleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <group_uuid>",
		Short: "Delete an availability group",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmdutil.CheckForce(cmd, fmt.Sprintf("Are you sure you want to delete availability group %s?", args[0])) {
				output.PrintWarning("Aborted")
				return nil
			}

			client := cmdutil.GetClient(cmd)

			s := output.NewSpinner("Deleting availability group...")
			s.Start()
			resp, err := client.Delete("/vps/availability-groups/" + args[0])
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			output.PrintSuccess("Availability group deleted successfully")
			return nil
		},
	}
	cmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")
	return cmd
}

// --- add-vps ---

func addVPSCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add-vps <group_uuid> <vps_id>",
		Short: "Add a VPS to an availability group",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			if _, err := strconv.Atoi(args[1]); err != nil {
				return fmt.Errorf("invalid vps_id: %s", args[1])
			}

			s := output.NewSpinner("Adding VPS to group...")
			s.Start()
			resp, err := client.Post(fmt.Sprintf("/vps/availability-groups/%s/vps/%s", args[0], args[1]), nil)
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			output.PrintSuccess("VPS added to availability group")
			return nil
		},
	}
}

// --- remove-vps ---

func removeVPSCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove-vps <group_uuid> <vps_id>",
		Short: "Remove a VPS from an availability group",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmdutil.CheckForce(cmd, fmt.Sprintf("Remove VPS %s from group %s?", args[1], args[0])) {
				output.PrintWarning("Aborted")
				return nil
			}

			client := cmdutil.GetClient(cmd)
			if _, err := strconv.Atoi(args[1]); err != nil {
				return fmt.Errorf("invalid vps_id: %s", args[1])
			}

			s := output.NewSpinner("Removing VPS from group...")
			s.Start()
			resp, err := client.Delete(fmt.Sprintf("/vps/availability-groups/%s/vps/%s", args[0], args[1]))
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			output.PrintSuccess("VPS removed from availability group")
			return nil
		},
	}
	cmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")
	return cmd
}
