package ddosattack

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/CubePathInc/cubecli/internal/cmdutil"
	"github.com/CubePathInc/cubecli/internal/output"
	"github.com/spf13/cobra"
)

func NewCmd() *cobra.Command {
	ddosAttackCmd := &cobra.Command{
		Use:   "ddos-attack",
		Short: "Manage DDoS attack information",
	}

	ddosAttackListCmd := &cobra.Command{
		Use:   "list",
		Short: "List DDoS attacks",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			s := output.NewSpinner("Fetching DDoS attacks...")
			s.Start()
			resp, err := client.Get("/ddos-attacks/attacks")
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			// Check if response is a message (e.g. no attacks found)
			var msgResp struct {
				Detail string `json:"detail"`
			}
			if err := json.Unmarshal(resp, &msgResp); err == nil && msgResp.Detail != "" {
				output.PrintInfo(msgResp.Detail)
				return nil
			}

			var attacks []struct {
				AttackID          int     `json:"attack_id"`
				IPAddress         string  `json:"ip_address"`
				StartTime         string  `json:"start_time"`
				Duration          int     `json:"duration"`
				PacketsSecondPeak int     `json:"packets_second_peak"`
				GbpsPeak          float64 `json:"gbps_peak"`
				Status            string  `json:"status"`
				Description       string  `json:"description"`
			}
			if err := json.Unmarshal(resp, &attacks); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}

			t := output.NewTable("DDoS Attacks", []string{"Attack ID", "IP Address", "Start Time", "Duration (s)", "Peak PPS", "Peak Gbps", "Status", "Description"})
			for _, a := range attacks {
				t.AddRow(
					strconv.Itoa(a.AttackID),
					a.IPAddress,
					a.StartTime,
					strconv.Itoa(a.Duration),
					strconv.Itoa(a.PacketsSecondPeak),
					fmt.Sprintf("%g", a.GbpsPeak),
					output.FormatStatus(a.Status),
					a.Description,
				)
			}
			t.Render()
			return nil
		},
	}

	ddosAttackCmd.AddCommand(ddosAttackListCmd, attackDataCmd("details <attack_id>", "details", "Show the details of an attack"), attackDataCmd("traffic <attack_id>", "traffic-graph", "Show the traffic time series of an attack"))
	return ddosAttackCmd
}

// attackDataCmd prints an attack's details or traffic series as JSON: their
// shape comes from the mitigation platform and varies per attack type.
func attackDataCmd(use, endpoint, short string) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			attackID, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid attack_id: %s", args[0])
			}

			s := output.NewSpinner("Fetching attack data...")
			s.Start()
			resp, err := client.Get(fmt.Sprintf("/ddos-attacks/attacks/%d/%s", attackID, endpoint))
			s.Stop()
			if err != nil {
				return err
			}
			return output.PrintJSON(json.RawMessage(resp))
		},
	}
}
