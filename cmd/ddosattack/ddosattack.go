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
				AttackID         int    `json:"attack_id"`
				IPAddress        string `json:"ip_address"`
				StartTime        string `json:"start_time"`
				Duration         int    `json:"duration"`
				PacketsSecondPeak int   `json:"packets_second_peak"`
				BytesSecondPeak  int    `json:"bytes_second_peak"`
				Status           string `json:"status"`
				Description      string `json:"description"`
			}
			if err := json.Unmarshal(resp, &attacks); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}

			t := output.NewTable("DDoS Attacks", []string{"Attack ID", "IP Address", "Start Time", "Duration (s)", "Peak PPS", "Peak Bps", "Status", "Description"})
			for _, a := range attacks {
				t.AddRow(
					strconv.Itoa(a.AttackID),
					a.IPAddress,
					a.StartTime,
					strconv.Itoa(a.Duration),
					strconv.Itoa(a.PacketsSecondPeak),
					strconv.Itoa(a.BytesSecondPeak),
					output.FormatStatus(a.Status),
					a.Description,
				)
			}
			t.Render()
			return nil
		},
	}

	ddosAttackCmd.AddCommand(ddosAttackListCmd)
	return ddosAttackCmd
}
