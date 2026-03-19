package vps

import (
	"encoding/json"
	"fmt"

	"github.com/CubePathInc/cubecli/internal/cmdutil"
	"github.com/CubePathInc/cubecli/internal/output"
	"github.com/spf13/cobra"
)

func addPlanCmd(parent *cobra.Command) {
	vpsPlanCmd := &cobra.Command{
		Use:   "plan",
		Short: "Manage VPS plans",
	}

	vpsPlanListCmd := &cobra.Command{
		Use:   "list",
		Short: "List available VPS plans",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			s := output.NewSpinner("Fetching plans...")
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
					Locations []struct {
						Clusters []struct {
							Plans []struct {
								Name          string  `json:"plan_name"`
								CPU           int     `json:"cpu"`
								RAM           int     `json:"ram"`
								Storage       int     `json:"storage"`
								Bandwidth     int     `json:"bandwidth"`
								PricePerHour  json.Number `json:"price_per_hour"`
							} `json:"plans"`
						} `json:"clusters"`
					} `json:"locations"`
				} `json:"vps"`
			}
			if err := json.Unmarshal(resp, &pricing); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}

			t := output.NewTable("VPS Plans", []string{"Plan", "vCPUs", "RAM", "Storage", "Bandwidth", "Price/Hour"})

			seen := make(map[string]bool)
			for _, loc := range pricing.VPS.Locations {
				for _, cluster := range loc.Clusters {
					for _, plan := range cluster.Plans {
						if seen[plan.Name] {
							continue
						}
						seen[plan.Name] = true
						t.AddRow(
							plan.Name,
							fmt.Sprintf("%d", plan.CPU),
							fmt.Sprintf("%d MB", plan.RAM),
							fmt.Sprintf("%d GB", plan.Storage),
							fmt.Sprintf("%d GB", plan.Bandwidth),
							fmt.Sprintf("$%s", plan.PricePerHour.String()),
						)
					}
				}
			}
			t.Render()
			return nil
		},
	}

	vpsPlanCmd.AddCommand(vpsPlanListCmd)
	parent.AddCommand(vpsPlanCmd)
}
