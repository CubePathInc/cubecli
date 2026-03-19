package baremetal

import (
	"encoding/json"
	"fmt"

	"github.com/CubePathInc/cubecli/internal/cmdutil"
	"github.com/CubePathInc/cubecli/internal/output"
	"github.com/spf13/cobra"
)

func addModelCmd(parent *cobra.Command) {
	modelCmd := &cobra.Command{
		Use:   "model",
		Short: "Manage baremetal models",
	}

	modelListCmd := &cobra.Command{
		Use:   "list",
		Short: "List available baremetal models",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			inStock, _ := cmd.Flags().GetBool("in-stock")
			outOfStock, _ := cmd.Flags().GetBool("out-of-stock")

			s := output.NewSpinner("Fetching baremetal models...")
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
				Baremetal struct {
					Locations []struct {
						LocationName string `json:"location_name"`
						Description  string `json:"description"`
						BaremetalModels []struct {
							ModelName      string  `json:"model_name"`
							CPU            string  `json:"cpu"`
							RAMSize        int     `json:"ram_size"`
							RAMType        string  `json:"ram_type"`
							DiskSize       string  `json:"disk_size"`
							DiskType       string  `json:"disk_type"`
							Port           int     `json:"port"`
							Price          float64 `json:"price"`
							Setup          float64 `json:"setup"`
							StockAvailable int     `json:"stock_available"`
						} `json:"baremetal_models"`
					} `json:"locations"`
				} `json:"baremetal"`
			}
			if err := json.Unmarshal(resp, &pricing); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}

			t := output.NewTable("Baremetal Models", []string{"Model", "CPU", "RAM", "Disk", "Port", "Price/Month", "Setup", "Location", "Available"})
			for _, loc := range pricing.Baremetal.Locations {
				for _, m := range loc.BaremetalModels {
					if inStock && m.StockAvailable == 0 {
						continue
					}
					if outOfStock && m.StockAvailable > 0 {
						continue
					}

					availability := "No"
					if m.StockAvailable > 0 {
						availability = fmt.Sprintf("Yes (%d)", m.StockAvailable)
					}

					t.AddRow(
						m.ModelName,
						m.CPU,
						fmt.Sprintf("%d GB %s", m.RAMSize, m.RAMType),
						fmt.Sprintf("%s %s", m.DiskSize, m.DiskType),
						fmt.Sprintf("%d Mbps", m.Port),
						fmt.Sprintf("$%.2f", m.Price),
						fmt.Sprintf("$%.2f", m.Setup),
						loc.LocationName,
						output.FormatStatus(availability),
					)
				}
			}
			t.Render()
			return nil
		},
	}

	modelListCmd.Flags().Bool("in-stock", false, "Show only in-stock models")
	modelListCmd.Flags().Bool("out-of-stock", false, "Show only out-of-stock models")

	modelCmd.AddCommand(modelListCmd)
	parent.AddCommand(modelCmd)
}
