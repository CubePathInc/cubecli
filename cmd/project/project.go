package project

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/CubePathInc/cubecli/internal/cmdutil"
	"github.com/CubePathInc/cubecli/internal/output"
	"github.com/spf13/cobra"
)

func NewCmd() *cobra.Command {
	projectCmd := &cobra.Command{
		Use:   "project",
		Short: "Manage projects",
	}

	projectCreateCmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new project",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			name, _ := cmd.Flags().GetString("name")
			description, _ := cmd.Flags().GetString("description")

			body := map[string]string{"name": name}
			if description != "" {
				body["description"] = description
			}

			s := output.NewSpinner("Creating project...")
			s.Start()
			resp, err := client.Post("/projects/", body)
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			output.PrintSuccess("Project created successfully")
			return nil
		},
	}

	projectListCmd := &cobra.Command{
		Use:   "list",
		Short: "List projects",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			s := output.NewSpinner("Fetching projects...")
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
					ID          int    `json:"id"`
					Name        string `json:"name"`
					Description string `json:"description"`
				} `json:"project"`
				VPS      []json.RawMessage `json:"vps"`
				Networks []json.RawMessage `json:"networks"`
			}
			if err := json.Unmarshal(resp, &projects); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}

			t := output.NewTable("Projects", []string{"ID", "Name", "Description", "VPS Count", "Networks"})
			for _, p := range projects {
				t.AddRow(
					strconv.Itoa(p.Project.ID),
					p.Project.Name,
					p.Project.Description,
					strconv.Itoa(len(p.VPS)),
					strconv.Itoa(len(p.Networks)),
				)
			}
			t.Render()
			return nil
		},
	}

	projectShowCmd := &cobra.Command{
		Use:   "show <project_id>",
		Short: "Show project details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			projectID, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid project_id: %s", args[0])
			}

			s := output.NewSpinner("Fetching project details...")
			s.Start()
			resp, err := client.Get("/projects/")
			s.Stop()
			if err != nil {
				return err
			}

			var projects []struct {
				Project struct {
					ID          int    `json:"id"`
					Name        string `json:"name"`
					Description string `json:"description"`
				} `json:"project"`
				VPS []struct {
					ID       int    `json:"id"`
					Hostname string `json:"hostname"`
					Status   string `json:"status"`
				} `json:"vps"`
				Networks []struct {
					ID       int    `json:"id"`
					Name     string `json:"name"`
					IPRange  string `json:"ip_range"`
					Location string `json:"location_name"`
				} `json:"networks"`
			}
			if err := json.Unmarshal(resp, &projects); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}

			var found bool
			for _, p := range projects {
				if p.Project.ID == projectID {
					found = true

					if cmdutil.IsJSON(cmd) {
						return output.PrintJSON(p)
					}

					t := output.NewTable("Project Details", []string{"Field", "Value"})
					t.AddRow("ID", strconv.Itoa(p.Project.ID))
					t.AddRow("Name", p.Project.Name)
					t.AddRow("Description", p.Project.Description)
					t.AddRow("VPS Count", strconv.Itoa(len(p.VPS)))
					t.AddRow("Networks", strconv.Itoa(len(p.Networks)))
					t.Render()

					if len(p.VPS) > 0 {
						vt := output.NewTable("VPS Instances", []string{"ID", "Hostname", "Status"})
						for _, v := range p.VPS {
							vt.AddRow(strconv.Itoa(v.ID), v.Hostname, output.FormatStatus(v.Status))
						}
						vt.Render()
					}

					if len(p.Networks) > 0 {
						nt := output.NewTable("Networks", []string{"ID", "Name", "IP Range", "Location"})
						for _, n := range p.Networks {
							nt.AddRow(strconv.Itoa(n.ID), n.Name, n.IPRange, n.Location)
						}
						nt.Render()
					}

					break
				}
			}

			if !found {
				return fmt.Errorf("project with ID %d not found", projectID)
			}

			return nil
		},
	}

	projectDeleteCmd := &cobra.Command{
		Use:   "delete <project_id>",
		Short: "Delete a project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			projectID, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid project_id: %s", args[0])
			}

			if !cmdutil.CheckForce(cmd, "Are you sure you want to delete this project?") {
				output.PrintWarning("Aborted")
				return nil
			}

			s := output.NewSpinner("Deleting project...")
			s.Start()
			resp, err := client.Delete(fmt.Sprintf("/projects/%d", projectID))
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			output.PrintSuccess("Project deleted successfully")
			return nil
		},
	}

	projectCreateCmd.Flags().StringP("name", "n", "", "Name of the project")
	projectCreateCmd.Flags().StringP("description", "d", "", "Description of the project")
	projectCreateCmd.MarkFlagRequired("name")

	projectDeleteCmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")

	projectCmd.AddCommand(projectCreateCmd, projectListCmd, projectShowCmd, projectDeleteCmd)
	return projectCmd
}
