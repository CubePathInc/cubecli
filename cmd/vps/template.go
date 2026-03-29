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
			resp, err := client.Get("/vps/templates")
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			var result struct {
				OperatingSystems []struct {
					TemplateName string `json:"template_name"`
					OSName       string `json:"os_name"`
					Version      string `json:"version"`
				} `json:"operating_systems"`
				Applications []struct {
					AppName         string `json:"app_name"`
					Version         string `json:"version"`
					RecommendedPlan string `json:"recommended_plan"`
				} `json:"applications"`
			}
			if err := json.Unmarshal(resp, &result); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}

			t := output.NewTable("OS Templates", []string{"Template Name", "OS", "Version"})
			for _, tmpl := range result.OperatingSystems {
				t.AddRow(tmpl.TemplateName, tmpl.OSName, tmpl.Version)
			}
			t.Render()

			if len(result.Applications) > 0 {
				a := output.NewTable("Application Templates", []string{"App", "Version", "Recommended Plan"})
				for _, app := range result.Applications {
					a.AddRow(app.AppName, app.Version, app.RecommendedPlan)
				}
				a.Render()
			}
			return nil
		},
	}

	vpsTemplateCmd.AddCommand(vpsTemplateListCmd)
	parent.AddCommand(vpsTemplateCmd)
}
