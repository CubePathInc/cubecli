package baremetal

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/CubePathInc/cubecli/internal/cmdutil"
	"github.com/CubePathInc/cubecli/internal/output"
	"github.com/spf13/cobra"
)

func addPowerCmd(parent *cobra.Command) {
	powerCmd := &cobra.Command{
		Use:   "power",
		Short: "Manage baremetal server power state",
	}

	powerStartCmd := &cobra.Command{
		Use:   "start <id>",
		Short: "Power on a baremetal server",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			bmID, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid baremetal id: %s", args[0])
			}

			s := output.NewSpinner("Starting baremetal server...")
			s.Start()
			resp, err := client.Post(fmt.Sprintf("/baremetal/%d/power/start_metal", bmID), nil)
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			output.PrintSuccess("Baremetal server powered on")
			return nil
		},
	}

	powerStopCmd := &cobra.Command{
		Use:   "stop <id>",
		Short: "Power off a baremetal server",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			bmID, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid baremetal id: %s", args[0])
			}

			s := output.NewSpinner("Stopping baremetal server...")
			s.Start()
			resp, err := client.Post(fmt.Sprintf("/baremetal/%d/power/stop_metal", bmID), nil)
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			output.PrintSuccess("Baremetal server powered off")
			return nil
		},
	}

	powerRestartCmd := &cobra.Command{
		Use:   "restart <id>",
		Short: "Restart a baremetal server",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			bmID, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid baremetal id: %s", args[0])
			}

			s := output.NewSpinner("Restarting baremetal server...")
			s.Start()
			resp, err := client.Post(fmt.Sprintf("/baremetal/%d/power/restart_metal", bmID), nil)
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			output.PrintSuccess("Baremetal server restarted")
			return nil
		},
	}

	powerCmd.AddCommand(powerStartCmd, powerStopCmd, powerRestartCmd)
	parent.AddCommand(powerCmd)
}
