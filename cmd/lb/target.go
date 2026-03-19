package lb

import (
	"encoding/json"
	"fmt"

	"github.com/CubePathInc/cubecli/internal/cmdutil"
	"github.com/CubePathInc/cubecli/internal/output"
	"github.com/spf13/cobra"
)

func addTargetCmd(parent *cobra.Command) {
	targetCmd := &cobra.Command{
		Use:   "target",
		Short: "Manage load balancer targets",
	}

	targetAddCmd := &cobra.Command{
		Use:   "add <lb_uuid> <listener_uuid>",
		Short: "Add a target to a listener",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			lbUUID := args[0]
			listenerUUID := args[1]

			targetType, _ := cmd.Flags().GetString("type")
			targetUUID, _ := cmd.Flags().GetString("target")
			port, _ := cmd.Flags().GetInt("port")
			weight, _ := cmd.Flags().GetInt("weight")

			body := map[string]interface{}{
				"target_type": targetType,
				"target_uuid": targetUUID,
				"weight":      weight,
			}
			if cmd.Flags().Changed("port") {
				body["port"] = port
			}

			s := output.NewSpinner("Adding target...")
			s.Start()
			resp, err := client.Post(fmt.Sprintf("/loadbalancer/%s/listeners/%s/targets", lbUUID, listenerUUID), body)
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			output.PrintSuccess("Target added successfully")
			return nil
		},
	}

	targetUpdateCmd := &cobra.Command{
		Use:   "update <lb_uuid> <listener_uuid> <target_uuid>",
		Short: "Update a target",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			lbUUID := args[0]
			listenerUUID := args[1]
			targetUUID := args[2]

			body := map[string]interface{}{}
			if cmd.Flags().Changed("port") {
				port, _ := cmd.Flags().GetInt("port")
				body["port"] = port
			}
			if cmd.Flags().Changed("weight") {
				weight, _ := cmd.Flags().GetInt("weight")
				body["weight"] = weight
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

			s := output.NewSpinner("Updating target...")
			s.Start()
			resp, err := client.Patch(fmt.Sprintf("/loadbalancer/%s/listeners/%s/targets/%s", lbUUID, listenerUUID, targetUUID), body)
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			output.PrintSuccess("Target updated successfully")
			return nil
		},
	}

	targetRemoveCmd := &cobra.Command{
		Use:   "remove <lb_uuid> <listener_uuid> <target_uuid>",
		Short: "Remove a target from a listener",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			lbUUID := args[0]
			listenerUUID := args[1]
			targetUUID := args[2]

			if !cmdutil.CheckForce(cmd, "Are you sure you want to remove this target?") {
				output.PrintWarning("Aborted")
				return nil
			}

			s := output.NewSpinner("Removing target...")
			s.Start()
			resp, err := client.Delete(fmt.Sprintf("/loadbalancer/%s/listeners/%s/targets/%s", lbUUID, listenerUUID, targetUUID))
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			output.PrintSuccess("Target removed successfully")
			return nil
		},
	}

	targetDrainCmd := &cobra.Command{
		Use:   "drain <lb_uuid> <listener_uuid> <target_uuid>",
		Short: "Drain a target",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			lbUUID := args[0]
			listenerUUID := args[1]
			targetUUID := args[2]

			s := output.NewSpinner("Draining target...")
			s.Start()
			resp, err := client.Post(fmt.Sprintf("/loadbalancer/%s/listeners/%s/targets/%s/drain", lbUUID, listenerUUID, targetUUID), nil)
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			output.PrintSuccess("Target drain initiated successfully")
			return nil
		},
	}

	// Flags for add
	targetAddCmd.Flags().StringP("type", "t", "", "Target type (vps, baremetal, availability_group)")
	targetAddCmd.Flags().String("target", "", "Target UUID")
	targetAddCmd.Flags().IntP("port", "p", 0, "Target port")
	targetAddCmd.Flags().IntP("weight", "w", 100, "Target weight (1-100)")
	targetAddCmd.MarkFlagRequired("type")
	targetAddCmd.MarkFlagRequired("target")

	// Flags for update
	targetUpdateCmd.Flags().IntP("port", "p", 0, "New target port")
	targetUpdateCmd.Flags().IntP("weight", "w", 0, "New target weight (1-100)")
	targetUpdateCmd.Flags().Bool("enable", false, "Enable the target")
	targetUpdateCmd.Flags().Bool("disable", false, "Disable the target")

	// Flags for remove
	targetRemoveCmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")

	targetCmd.AddCommand(targetAddCmd, targetUpdateCmd, targetRemoveCmd, targetDrainCmd)
	parent.AddCommand(targetCmd)
}
