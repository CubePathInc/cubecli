package vps

import (
	"encoding/json"
	"fmt"

	"github.com/CubePathInc/cubecli/internal/cmdutil"
	"github.com/CubePathInc/cubecli/internal/output"
	"github.com/spf13/cobra"
)

func addTemplateCmd(parent *cobra.Command) {
	vpsTemplateCmd := &cobra.Command{
		Use:   "template",
		Short: "Manage VPS templates",
	}

	vpsTemplateListCmd := &cobra.Command{
		Use:   "list",
		Short: "List available VPS templates",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			s := output.NewSpinner("Fetching templates...")
			s.Start()
			resp, err := client.Get("/pricing")
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			var pricing struct {
				VPS struct {
					Templates []struct {
						Name    string `json:"template_name"`
						OS      string `json:"os_name"`
						Version string `json:"version"`
					} `json:"templates"`
				} `json:"vps"`
			}
			if err := json.Unmarshal(resp, &pricing); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}

			t := output.NewTable("VPS Templates", []string{"Template Name", "OS", "Version"})
			for _, tmpl := range pricing.VPS.Templates {
				t.AddRow(tmpl.Name, tmpl.OS, tmpl.Version)
			}
			t.Render()
			return nil
		},
	}

	vpsTemplateCmd.AddCommand(vpsTemplateListCmd)
	parent.AddCommand(vpsTemplateCmd)
}
