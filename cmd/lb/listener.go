package lb

import (
	"encoding/json"
	"fmt"

	"github.com/CubePathInc/cubecli/internal/cmdutil"
	"github.com/CubePathInc/cubecli/internal/output"
	"github.com/spf13/cobra"
)

func addListenerCmd(parent *cobra.Command) {
	listenerCmd := &cobra.Command{
		Use:   "listener",
		Short: "Manage load balancer listeners",
	}

	listenerCreateCmd := &cobra.Command{
		Use:   "create <lb_uuid>",
		Short: "Create a listener on a load balancer",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			lbUUID := args[0]

			name, _ := cmd.Flags().GetString("name")
			protocol, _ := cmd.Flags().GetString("protocol")
			port, _ := cmd.Flags().GetInt("port")
			targetPort, _ := cmd.Flags().GetInt("target-port")
			algorithm, _ := cmd.Flags().GetString("algorithm")
			sticky, _ := cmd.Flags().GetBool("sticky")

			body := map[string]interface{}{
				"name":            name,
				"protocol":        protocol,
				"source_port":     port,
				"target_port":     targetPort,
				"algorithm":       algorithm,
				"sticky_sessions": sticky,
			}

			s := output.NewSpinner("Creating listener...")
			s.Start()
			resp, err := client.Post(fmt.Sprintf("/loadbalancer/%s/listeners", lbUUID), body)
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			output.PrintSuccess("Listener created successfully")
			return nil
		},
	}

	listenerUpdateCmd := &cobra.Command{
		Use:   "update <lb_uuid> <listener_uuid>",
		Short: "Update a listener on a load balancer",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			lbUUID := args[0]
			listenerUUID := args[1]

			body := map[string]interface{}{}
			if cmd.Flags().Changed("name") {
				name, _ := cmd.Flags().GetString("name")
				body["name"] = name
			}
			if cmd.Flags().Changed("target-port") {
				targetPort, _ := cmd.Flags().GetInt("target-port")
				body["target_port"] = targetPort
			}
			if cmd.Flags().Changed("algorithm") {
				algorithm, _ := cmd.Flags().GetString("algorithm")
				body["algorithm"] = algorithm
			}

			enable, _ := cmd.Flags().GetBool("enable")
			disable, _ := cmd.Flags().GetBool("disable")
			if enable && disable {
				return fmt.Errorf("--enable and --disable are mutually exclusive")
			}
			if enable {
				body["enabled"] = true
			}
			if disable {
				body["enabled"] = false
			}

			if len(body) == 0 {
				return fmt.Errorf("at least one flag must be specified")
			}

			s := output.NewSpinner("Updating listener...")
			s.Start()
			resp, err := client.Patch(fmt.Sprintf("/loadbalancer/%s/listeners/%s", lbUUID, listenerUUID), body)
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			output.PrintSuccess("Listener updated successfully")
			return nil
		},
	}

	listenerDeleteCmd := &cobra.Command{
		Use:   "delete <lb_uuid> <listener_uuid>",
		Short: "Delete a listener from a load balancer",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			lbUUID := args[0]
			listenerUUID := args[1]

			if !cmdutil.CheckForce(cmd, "Are you sure you want to delete this listener?") {
				output.PrintWarning("Aborted")
				return nil
			}

			s := output.NewSpinner("Deleting listener...")
			s.Start()
			resp, err := client.Delete(fmt.Sprintf("/loadbalancer/%s/listeners/%s", lbUUID, listenerUUID))
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			output.PrintSuccess("Listener deleted successfully")
			return nil
		},
	}

	// Flags for create
	listenerCreateCmd.Flags().StringP("name", "n", "", "Name of the listener")
	listenerCreateCmd.Flags().StringP("protocol", "p", "http", "Protocol (e.g. http, https, tcp)")
	listenerCreateCmd.Flags().Int("port", 0, "Source port")
	listenerCreateCmd.Flags().Int("target-port", 0, "Target port")
	listenerCreateCmd.Flags().StringP("algorithm", "a", "round_robin", "Load balancing algorithm")
	listenerCreateCmd.Flags().Bool("sticky", false, "Enable sticky sessions")
	listenerCreateCmd.Flags().Bool("no-sticky", false, "Disable sticky sessions")
	listenerCreateCmd.MarkFlagRequired("name")
	listenerCreateCmd.MarkFlagRequired("port")
	listenerCreateCmd.MarkFlagRequired("target-port")

	// Flags for update
	listenerUpdateCmd.Flags().StringP("name", "n", "", "New name for the listener")
	listenerUpdateCmd.Flags().Int("target-port", 0, "New target port")
	listenerUpdateCmd.Flags().StringP("algorithm", "a", "", "New load balancing algorithm")
	listenerUpdateCmd.Flags().Bool("enable", false, "Enable the listener")
	listenerUpdateCmd.Flags().Bool("disable", false, "Disable the listener")

	// Flags for delete
	listenerDeleteCmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")

	listenerCmd.AddCommand(listenerCreateCmd, listenerUpdateCmd, listenerDeleteCmd)
	parent.AddCommand(listenerCmd)
}
