package lb

import (
	"encoding/json"
	"fmt"

	"github.com/CubePathInc/cubecli/internal/cmdutil"
	"github.com/CubePathInc/cubecli/internal/output"
	"github.com/spf13/cobra"
)

func addHealthCheckCmd(parent *cobra.Command) {
	healthCheckCmd := &cobra.Command{
		Use:   "health-check",
		Short: "Manage load balancer health checks",
	}

	healthCheckConfigureCmd := &cobra.Command{
		Use:   "configure <lb_uuid> <listener_uuid>",
		Short: "Configure a health check for a listener",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			lbUUID := args[0]
			listenerUUID := args[1]

			protocol, _ := cmd.Flags().GetString("protocol")
			path, _ := cmd.Flags().GetString("path")
			interval, _ := cmd.Flags().GetInt("interval")
			timeout, _ := cmd.Flags().GetInt("timeout")
			healthy, _ := cmd.Flags().GetInt("healthy")
			unhealthy, _ := cmd.Flags().GetInt("unhealthy")
			codes, _ := cmd.Flags().GetString("codes")

			body := map[string]interface{}{
				"protocol":            protocol,
				"path":                path,
				"interval_seconds":    interval,
				"timeout_seconds":     timeout,
				"healthy_threshold":   healthy,
				"unhealthy_threshold": unhealthy,
				"expected_codes":      codes,
			}

			s := output.NewSpinner("Configuring health check...")
			s.Start()
			resp, err := client.Put(fmt.Sprintf("/loadbalancer/%s/listeners/%s/health-check", lbUUID, listenerUUID), body)
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			output.PrintSuccess("Health check configured successfully")
			return nil
		},
	}

	healthCheckDeleteCmd := &cobra.Command{
		Use:   "delete <lb_uuid> <listener_uuid>",
		Short: "Delete a health check from a listener",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			lbUUID := args[0]
			listenerUUID := args[1]

			if !cmdutil.CheckForce(cmd, "Are you sure you want to delete this health check?") {
				output.PrintWarning("Aborted")
				return nil
			}

			s := output.NewSpinner("Deleting health check...")
			s.Start()
			resp, err := client.Delete(fmt.Sprintf("/loadbalancer/%s/listeners/%s/health-check", lbUUID, listenerUUID))
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			output.PrintSuccess("Health check deleted successfully")
			return nil
		},
	}

	// Flags for configure
	healthCheckConfigureCmd.Flags().StringP("protocol", "p", "http", "Health check protocol")
	healthCheckConfigureCmd.Flags().String("path", "/", "Health check path")
	healthCheckConfigureCmd.Flags().Int("interval", 30, "Interval in seconds (5-300)")
	healthCheckConfigureCmd.Flags().Int("timeout", 5, "Timeout in seconds (1-60)")
	healthCheckConfigureCmd.Flags().Int("healthy", 2, "Healthy threshold (1-10)")
	healthCheckConfigureCmd.Flags().Int("unhealthy", 3, "Unhealthy threshold (1-10)")
	healthCheckConfigureCmd.Flags().String("codes", "200-399", "Expected HTTP status codes")

	// Flags for delete
	healthCheckDeleteCmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")

	healthCheckCmd.AddCommand(healthCheckConfigureCmd, healthCheckDeleteCmd)
	parent.AddCommand(healthCheckCmd)
}
