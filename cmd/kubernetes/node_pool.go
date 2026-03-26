package kubernetes

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/CubePathInc/cubecli/internal/cmdutil"
	"github.com/CubePathInc/cubecli/internal/output"
	"github.com/spf13/cobra"
)

func newNodePoolCmd() *cobra.Command {
	npCmd := &cobra.Command{
		Use:     "node-pool",
		Aliases: []string{"np"},
		Short:   "Manage node pools",
	}

	npCmd.AddCommand(
		npListCmd(),
		npCreateCmd(),
		npUpdateCmd(),
		npDeleteCmd(),
		npAddNodesCmd(),
		npRemoveNodeCmd(),
	)

	return npCmd
}

// --- list ---

func npListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list <cluster_uuid>",
		Short: "List node pools in a cluster",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			s := output.NewSpinner("Fetching node pools...")
			s.Start()
			resp, err := client.Get("/kubernetes/" + args[0] + "/node-pools/")
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			var pools []struct {
				UUID         string `json:"uuid"`
				Name         string `json:"name"`
				DesiredNodes int    `json:"desired_nodes"`
				MinNodes     int    `json:"min_nodes"`
				MaxNodes     int    `json:"max_nodes"`
				AutoScale    bool   `json:"auto_scale"`
				Plan         struct {
					Name string `json:"name"`
				} `json:"plan"`
				Nodes []json.RawMessage `json:"nodes"`
			}
			if err := json.Unmarshal(resp, &pools); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}

			t := output.NewTable("Node Pools", []string{"UUID", "Name", "Plan", "Nodes", "Min", "Max", "AutoScale"})
			for _, p := range pools {
				autoScale := "Off"
				if p.AutoScale {
					autoScale = "On"
				}
				t.AddRow(
					p.UUID,
					p.Name,
					p.Plan.Name,
					strconv.Itoa(len(p.Nodes)),
					strconv.Itoa(p.MinNodes),
					strconv.Itoa(p.MaxNodes),
					autoScale,
				)
			}
			t.Render()
			return nil
		},
	}
}

// --- create ---

func npCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create <cluster_uuid>",
		Short: "Create a new node pool",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			name, _ := cmd.Flags().GetString("name")
			plan, _ := cmd.Flags().GetString("plan")
			count, _ := cmd.Flags().GetInt("count")
			autoScale, _ := cmd.Flags().GetBool("auto-scale")
			labelArgs, _ := cmd.Flags().GetStringSlice("label")
			taintArgs, _ := cmd.Flags().GetStringSlice("taint")

			body := map[string]interface{}{
				"name":       name,
				"plan":       plan,
				"count":      count,
				"auto_scale": autoScale,
			}

			if len(labelArgs) > 0 {
				body["labels"] = parseLabels(labelArgs)
			}
			if len(taintArgs) > 0 {
				body["taints"] = parseTaints(taintArgs)
			}

			s := output.NewSpinner("Creating node pool...")
			s.Start()
			resp, err := client.Post("/kubernetes/"+args[0]+"/node-pools/", body)
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			var result struct {
				Detail string `json:"detail"`
				UUID   string `json:"uuid"`
			}
			if err := json.Unmarshal(resp, &result); err != nil {
				return err
			}
			if result.UUID != "" {
				output.PrintSuccess(fmt.Sprintf("Node pool created: %s", result.UUID))
			} else if result.Detail != "" {
				output.PrintSuccess(result.Detail)
			} else {
				output.PrintSuccess("Node pool creation initiated")
			}
			return nil
		},
	}
	cmd.Flags().StringP("name", "n", "default", "Node pool name")
	cmd.Flags().String("plan", "", "Server plan")
	cmd.Flags().Int("count", 1, "Number of worker nodes")
	cmd.Flags().Bool("auto-scale", true, "Enable auto scaling")
	cmd.Flags().StringSlice("label", nil, "Node labels (key=value, can specify multiple)")
	cmd.Flags().StringSlice("taint", nil, "Node taints (key=value:Effect, can specify multiple)")
	_ = cmd.MarkFlagRequired("plan")
	return cmd
}

// --- update ---

func npUpdateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update <cluster_uuid> <pool_uuid>",
		Short: "Update a node pool",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			body := map[string]interface{}{}
			if cmd.Flags().Changed("name") {
				v, _ := cmd.Flags().GetString("name")
				body["name"] = v
			}
			if cmd.Flags().Changed("desired-nodes") {
				v, _ := cmd.Flags().GetInt("desired-nodes")
				body["desired_nodes"] = v
			}
			if cmd.Flags().Changed("min-nodes") {
				v, _ := cmd.Flags().GetInt("min-nodes")
				body["min_nodes"] = v
			}
			if cmd.Flags().Changed("max-nodes") {
				v, _ := cmd.Flags().GetInt("max-nodes")
				body["max_nodes"] = v
			}
			if cmd.Flags().Changed("auto-scale") {
				v, _ := cmd.Flags().GetBool("auto-scale")
				body["auto_scale"] = v
			}
			if cmd.Flags().Changed("label") {
				labelArgs, _ := cmd.Flags().GetStringSlice("label")
				body["labels"] = parseLabels(labelArgs)
			}
			if cmd.Flags().Changed("taint") {
				taintArgs, _ := cmd.Flags().GetStringSlice("taint")
				body["taints"] = parseTaints(taintArgs)
			}

			if len(body) == 0 {
				return fmt.Errorf("at least one flag must be provided")
			}

			s := output.NewSpinner("Updating node pool...")
			s.Start()
			resp, err := client.Patch("/kubernetes/"+args[0]+"/node-pools/"+args[1], body)
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			output.PrintSuccess("Node pool updated successfully")
			return nil
		},
	}
	cmd.Flags().StringP("name", "n", "", "New pool name")
	cmd.Flags().Int("desired-nodes", 0, "Desired number of nodes")
	cmd.Flags().Int("min-nodes", 0, "Minimum number of nodes")
	cmd.Flags().Int("max-nodes", 0, "Maximum number of nodes")
	cmd.Flags().Bool("auto-scale", false, "Enable/disable auto scaling")
	cmd.Flags().StringSlice("label", nil, "Node labels (key=value)")
	cmd.Flags().StringSlice("taint", nil, "Node taints (key=value:Effect)")
	return cmd
}

// --- delete ---

func npDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <cluster_uuid> <pool_uuid>",
		Short: "Delete a node pool",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmdutil.CheckForce(cmd, fmt.Sprintf("Are you sure you want to delete node pool %s?", args[1])) {
				return nil
			}

			client := cmdutil.GetClient(cmd)

			s := output.NewSpinner("Deleting node pool...")
			s.Start()
			resp, err := client.Delete("/kubernetes/" + args[0] + "/node-pools/" + args[1])
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			output.PrintSuccess("Node pool deletion initiated")
			return nil
		},
	}
	cmd.Flags().BoolP("force", "f", false, "Skip confirmation")
	return cmd
}

// --- add-nodes ---

func npAddNodesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add-nodes <cluster_uuid> <pool_uuid>",
		Short: "Add worker nodes to a pool",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			count, _ := cmd.Flags().GetInt("count")

			body := map[string]interface{}{
				"count": count,
			}

			s := output.NewSpinner("Adding nodes...")
			s.Start()
			resp, err := client.Post("/kubernetes/"+args[0]+"/node-pools/"+args[1]+"/nodes", body)
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			output.PrintSuccess(fmt.Sprintf("Adding %d node(s) to pool", count))
			return nil
		},
	}
	cmd.Flags().Int("count", 1, "Number of nodes to add")
	return cmd
}

// --- remove-node ---

func npRemoveNodeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove-node <cluster_uuid> <pool_uuid> <vps_id>",
		Short: "Remove a specific worker node from a pool",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmdutil.CheckForce(cmd, fmt.Sprintf("Are you sure you want to remove node %s?", args[2])) {
				return nil
			}

			client := cmdutil.GetClient(cmd)

			s := output.NewSpinner("Removing node...")
			s.Start()
			resp, err := client.Delete("/kubernetes/" + args[0] + "/node-pools/" + args[1] + "/nodes/" + args[2])
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			output.PrintSuccess("Node removal initiated")
			return nil
		},
	}
	cmd.Flags().BoolP("force", "f", false, "Skip confirmation")
	return cmd
}
