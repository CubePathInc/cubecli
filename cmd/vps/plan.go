package vps

import (
	"encoding/json"
	"fmt"
	"strconv"

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
			resp, err := client.Get("/vps/plans")
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			var pricing struct {
				Locations []struct {
					LocationName string `json:"location_name"`
					Clusters     []struct {
						Plans []vpsPlanRow `json:"plans"`
					} `json:"clusters"`
				} `json:"locations"`
			}
			if err := json.Unmarshal(resp, &pricing); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}

			t := output.NewTable("VPS Plans", []string{"Location", "Plan", "vCPUs", "RAM", "Storage", "Bandwidth", "List/Hour", "Price/Hour", "Discount", "Price/Month"})

			// One row per plan and location: the same plan name can have a
			// different price (or discount) in each location.
			for _, loc := range pricing.Locations {
				seen := make(map[string]bool)
				for _, cluster := range loc.Clusters {
					for _, plan := range cluster.Plans {
						if seen[plan.Name] {
							continue
						}
						seen[plan.Name] = true
						effText, effHourly, discount := plan.prices()
						t.AddRow(
							loc.LocationName,
							plan.Name,
							fmt.Sprintf("%d", plan.CPU),
							fmt.Sprintf("%d MB", plan.RAM),
							fmt.Sprintf("%d GB", plan.Storage),
							fmt.Sprintf("%d GB", plan.Bandwidth),
							fmt.Sprintf("$%s", plan.PricePerHour.String()),
							fmt.Sprintf("$%s", effText),
							discount,
							fmt.Sprintf("$%.2f", effHourly*24*30),
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

// vpsPlanRow is a plan as returned by /vps/plans. PricePerHour is the list
// price; the discount fields are null when no discount is active.
type vpsPlanRow struct {
	Name                  string       `json:"plan_name"`
	CPU                   int          `json:"cpu"`
	RAM                   int          `json:"ram"`
	Storage               int          `json:"storage"`
	Bandwidth             int          `json:"bandwidth"`
	PricePerHour          json.Number  `json:"price_per_hour"`
	EffectivePricePerHour *json.Number `json:"effective_price_per_hour"`
	DiscountPercent       *json.Number `json:"discount_percent"`
}

// prices returns the price charged per hour (as text and number) and the
// discount column ("-50%", or empty without a discount).
func (p vpsPlanRow) prices() (string, float64, string) {
	price, discount := p.PricePerHour, ""
	if p.EffectivePricePerHour != nil && p.DiscountPercent != nil {
		pct := p.DiscountPercent.String()
		if f, err := p.DiscountPercent.Float64(); err == nil {
			pct = strconv.FormatFloat(f, 'f', -1, 64)
		}
		price, discount = *p.EffectivePricePerHour, "-"+pct+"%"
	}
	hourly, _ := price.Float64()
	return price.String(), hourly, discount
}
