package cdn

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/CubePathInc/cubecli/internal/cmdutil"
	"github.com/CubePathInc/cubecli/internal/output"
	"github.com/spf13/cobra"
)

func addPlanCmd(parent *cobra.Command) {
	planCmd := &cobra.Command{
		Use:   "plan",
		Short: "Manage CDN plans",
	}

	planListCmd := &cobra.Command{
		Use:   "list",
		Short: "List available CDN plans",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			s := output.NewSpinner("Fetching CDN plans...")
			s.Start()
			resp, err := client.Get("/cdn/plans")
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			var plans []struct {
				Name        string  `json:"name"`
				BasePriceHr float64 `json:"base_price_hr"`
				MaxZones    int     `json:"max_zones"`
				MaxOrigins  int     `json:"max_origins"`
				CustomSSL   bool    `json:"custom_ssl"`
			}
			if err := json.Unmarshal(resp, &plans); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}

			t := output.NewTable("CDN Plans", []string{"Name", "Base Price/hr", "Max Zones", "Max Origins", "Custom SSL"})
			for _, p := range plans {
				t.AddRow(
					p.Name,
					fmt.Sprintf("$%.4f", p.BasePriceHr),
					strconv.Itoa(p.MaxZones),
					strconv.Itoa(p.MaxOrigins),
					strconv.FormatBool(p.CustomSSL),
				)
			}
			t.Render()
			return nil
		},
	}

	planCmd.AddCommand(planListCmd)
	parent.AddCommand(planCmd)
}
