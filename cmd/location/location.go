package location

import (
	"encoding/json"
	"fmt"

	"github.com/CubePathInc/cubecli/internal/cmdutil"
	"github.com/CubePathInc/cubecli/internal/output"
	"github.com/spf13/cobra"
)

func NewCmd() *cobra.Command {
	locationCmd := &cobra.Command{
		Use:   "location",
		Short: "Manage locations",
	}

	locationListCmd := &cobra.Command{
		Use:   "list",
		Short: "List available locations",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			s := output.NewSpinner("Fetching locations...")
			s.Start()
			resp, err := client.Get("/pricing")
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			var result struct {
				VPS struct {
					Locations []struct {
						LocationName string `json:"location_name"`
						Description  string `json:"description"`
					} `json:"locations"`
				} `json:"vps"`
			}
			if err := json.Unmarshal(resp, &result); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}

			t := output.NewTable("Locations", []string{"Code", "Name"})
			for _, loc := range result.VPS.Locations {
				t.AddRow(loc.LocationName, loc.Description)
			}
			t.Render()
			return nil
		},
	}

	locationCmd.AddCommand(locationListCmd)
	return locationCmd
}
