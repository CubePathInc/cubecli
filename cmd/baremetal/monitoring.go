package baremetal

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/CubePathInc/cubecli/internal/cmdutil"
	"github.com/CubePathInc/cubecli/internal/output"
	"github.com/spf13/cobra"
)

func addMonitoringCmd(parent *cobra.Command) {
	monitoringCmd := &cobra.Command{
		Use:   "monitoring",
		Short: "Manage baremetal server monitoring",
	}

	monitoringEnableCmd := &cobra.Command{
		Use:   "enable <id>",
		Short: "Enable monitoring for a baremetal server",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			bmID, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid baremetal id: %s", args[0])
			}

			s := output.NewSpinner("Enabling monitoring...")
			s.Start()
			resp, err := client.Put(fmt.Sprintf("/baremetal/%d/monitoring?enable=true", bmID), nil)
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			output.PrintSuccess("Monitoring enabled")
			return nil
		},
	}

	monitoringDisableCmd := &cobra.Command{
		Use:   "disable <id>",
		Short: "Disable monitoring for a baremetal server",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			bmID, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid baremetal id: %s", args[0])
			}

			s := output.NewSpinner("Disabling monitoring...")
			s.Start()
			resp, err := client.Put(fmt.Sprintf("/baremetal/%d/monitoring?enable=false", bmID), nil)
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			output.PrintSuccess("Monitoring disabled")
			return nil
		},
	}

	monitoringStatusCmd := &cobra.Command{
		Use:   "status <id>",
		Short: "Show monitoring status for a baremetal server",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			bmID, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid baremetal id: %s", args[0])
			}

			s := output.NewSpinner("Fetching monitoring status...")
			s.Start()
			resp, err := client.Get("/projects/")
			s.Stop()
			if err != nil {
				return err
			}

			var projects []struct {
				Project struct {
					ID   int    `json:"id"`
					Name string `json:"name"`
				} `json:"project"`
				Baremetals []struct {
					ID               int    `json:"id"`
					Hostname         string `json:"hostname"`
					MonitoringEnable bool   `json:"monitoring_enable"`
					FloatingIPs      []struct {
						Type           string `json:"type"`
						Address        string `json:"address"`
						ProtectionType string `json:"protection_type"`
					} `json:"floating_ips"`
				} `json:"baremetals"`
			}
			if err := json.Unmarshal(resp, &projects); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}

			for _, p := range projects {
				for _, bm := range p.Baremetals {
					if bm.ID == bmID {
						if cmdutil.IsJSON(cmd) {
							return output.PrintJSON(map[string]interface{}{
								"id":                bm.ID,
								"hostname":          bm.Hostname,
								"monitoring_enable": bm.MonitoringEnable,
							})
						}

						status := "disabled"
						if bm.MonitoringEnable {
							status = "enabled"
						}

						var ips []string
						for _, fip := range bm.FloatingIPs {
							ips = append(ips, fip.Address)
						}

						t := output.NewTable("Monitoring Status", []string{"Field", "Value"})
						t.AddRow("ID", strconv.Itoa(bm.ID))
						t.AddRow("Hostname", bm.Hostname)
						t.AddRow("IP Addresses", strings.Join(ips, ", "))
						t.AddRow("Monitoring", output.FormatStatus(status))
						t.Render()

						return nil
					}
				}
			}

			return fmt.Errorf("baremetal server with ID %d not found", bmID)
		},
	}

	monitoringCmd.AddCommand(monitoringEnableCmd, monitoringDisableCmd, monitoringStatusCmd)
	parent.AddCommand(monitoringCmd)
}
