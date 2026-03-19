package vps

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/CubePathInc/cubecli/internal/cmdutil"
	"github.com/CubePathInc/cubecli/internal/output"
	"github.com/spf13/cobra"
)

func newPowerCommand(action, endpoint, spinnerMsg, successMsg string) *cobra.Command {
	return &cobra.Command{
		Use:   fmt.Sprintf("%s <vps_id>", action),
		Short: fmt.Sprintf("%s a VPS instance", capitalize(action)),
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			vpsID, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid VPS ID: %s", args[0])
			}

			s := output.NewSpinner(spinnerMsg)
			s.Start()
			resp, err := client.Post(fmt.Sprintf("/vps/%d/power/%s", vpsID, endpoint), nil)
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			var result map[string]interface{}
			if err := json.Unmarshal(resp, &result); err != nil {
				return err
			}
			if detail, ok := result["detail"].(string); ok {
				output.PrintSuccess(detail)
			} else {
				output.PrintSuccess(successMsg)
			}
			return nil
		},
	}
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return string(s[0]-32) + s[1:]
}

func addPowerCmd(parent *cobra.Command) {
	vpsPowerCmd := &cobra.Command{
		Use:   "power",
		Short: "VPS power management commands",
	}

	vpsPowerCmd.AddCommand(
		newPowerCommand("start", "start_vps", "Starting VPS...", "VPS started successfully"),
		newPowerCommand("stop", "stop_vps", "Stopping VPS...", "VPS stopped successfully"),
		newPowerCommand("restart", "restart_vps", "Restarting VPS...", "VPS restarted successfully"),
		newPowerCommand("reset", "reset_vps", "Resetting VPS...", "VPS reset successfully"),
	)

	parent.AddCommand(vpsPowerCmd)
}
