package lb

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/CubePathInc/cubecli/internal/cmdutil"
	"github.com/CubePathInc/cubecli/internal/output"
	"github.com/spf13/cobra"
)

func addPlanCmd(parent *cobra.Command) {
	lbPlanCmd := &cobra.Command{
		Use:   "plan",
		Short: "Manage load balancer plans",
	}

	lbPlanListCmd := &cobra.Command{
		Use:   "list",
		Short: "List available load balancer plans",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			s := output.NewSpinner("Fetching load balancer plans...")
			s.Start()
			resp, err := client.Get("/loadbalancer/plans")
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			var locations []struct {
				LocationName string `json:"location_name"`
				Plans        []struct {
					Name         string  `json:"name"`
					PriceHour    float64 `json:"price_per_hour"`
					PriceMonth   float64 `json:"price_per_month"`
					MaxListeners int     `json:"max_listeners"`
					MaxTargets   int     `json:"max_targets"`
					ConnPerSec   int     `json:"connections_per_second"`
				} `json:"plans"`
			}
			if err := json.Unmarshal(resp, &locations); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}

			t := output.NewTable("Load Balancer Plans", []string{"Plan", "Price/Hour", "Price/Month", "Max Listeners", "Max Targets", "Conn/sec"})
			for _, loc := range locations {
				for _, p := range loc.Plans {
					t.AddRow(
						p.Name,
						fmt.Sprintf("$%.4f", p.PriceHour),
						fmt.Sprintf("$%.2f", p.PriceMonth),
						strconv.Itoa(p.MaxListeners),
						strconv.Itoa(p.MaxTargets),
						strconv.Itoa(p.ConnPerSec),
					)
				}
			}
			t.Render()
			return nil
		},
	}

	lbPlanCmd.AddCommand(lbPlanListCmd)
	parent.AddCommand(lbPlanCmd)
}
