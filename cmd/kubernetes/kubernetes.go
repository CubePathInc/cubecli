package kubernetes

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/CubePathInc/cubecli/internal/cmdutil"
	"github.com/CubePathInc/cubecli/internal/output"
	"github.com/spf13/cobra"
)

func NewCmd() *cobra.Command {
	k8sCmd := &cobra.Command{
		Use:     "kubernetes",
		Aliases: []string{"k8s"},
		Short:   "Manage Kubernetes clusters",
	}

	k8sCmd.AddCommand(
		versionsCmd(),
		plansCmd(),
		listCmd(),
		showCmd(),
		createCmd(),
		updateCmd(),
		deleteCmd(),
		kubeconfigCmd(),
		moveCmd(),
		loadbalancersCmd(),
		newNodePoolCmd(),
		newAddonCmd(),
	)

	return k8sCmd
}

// --- versions ---

func versionsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "versions",
		Short: "List available Kubernetes versions",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			s := output.NewSpinner("Fetching Kubernetes versions...")
			s.Start()
			resp, err := client.Get("/kubernetes/versions")
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			var versions []struct {
				Version   string `json:"version"`
				IsDefault bool   `json:"is_default"`
				MinCPU    int    `json:"min_cpu"`
				MinRAMMB  int    `json:"min_ram_mb"`
			}
			if err := json.Unmarshal(resp, &versions); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}

			t := output.NewTable("Kubernetes Versions", []string{"Version", "Default", "Min CPU", "Min RAM (MB)"})
			for _, v := range versions {
				def := ""
				if v.IsDefault {
					def = "✓"
				}
				t.AddRow(v.Version, def, strconv.Itoa(v.MinCPU), strconv.Itoa(v.MinRAMMB))
			}
			t.Render()
			return nil
		},
	}
}

// --- plans ---

func plansCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plans",
		Short: "List server plans compatible with Kubernetes",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			path := "/kubernetes/plans"
			version, _ := cmd.Flags().GetString("version")
			if version != "" {
				path += "?version=" + version
			}

			s := output.NewSpinner("Fetching plans...")
			s.Start()
			resp, err := client.Get(path)
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			var plans []struct {
				ID           int     `json:"id"`
				Name         string  `json:"name"`
				CPU          int     `json:"cpu"`
				RAM          int     `json:"ram"`
				Storage      int     `json:"storage"`
				PricePerHour float64 `json:"price_per_hour"`
			}
			if err := json.Unmarshal(resp, &plans); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}

			t := output.NewTable("Kubernetes Plans", []string{"ID", "Name", "CPU", "RAM (MB)", "Storage (GB)", "Price/h"})
			for _, p := range plans {
				t.AddRow(
					strconv.Itoa(p.ID),
					p.Name,
					strconv.Itoa(p.CPU),
					strconv.Itoa(p.RAM),
					strconv.Itoa(p.Storage),
					fmt.Sprintf("$%.4f", p.PricePerHour),
				)
			}
			t.Render()
			return nil
		},
	}
	cmd.Flags().String("version", "", "Filter plans by Kubernetes version")
	return cmd
}

// --- list ---

func listCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List Kubernetes clusters",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			s := output.NewSpinner("Fetching clusters...")
			s.Start()
			resp, err := client.Get("/kubernetes/")
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			var clusters []struct {
				UUID           string `json:"uuid"`
				Name           string `json:"name"`
				Status         string `json:"status"`
				Version        string `json:"version"`
				HAControlPlane bool   `json:"ha_control_plane"`
				Location       struct {
					LocationName string `json:"location_name"`
				} `json:"location"`
				WorkerCount   int `json:"worker_count"`
				NodePoolCount int `json:"node_pool_count"`
			}
			if err := json.Unmarshal(resp, &clusters); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}

			t := output.NewTable("Kubernetes Clusters", []string{"UUID", "Name", "Status", "Version", "HA", "Location", "Workers", "Pools"})
			for _, c := range clusters {
				ha := ""
				if c.HAControlPlane {
					ha = "✓"
				}
				t.AddRow(
					c.UUID,
					c.Name,
					output.FormatStatus(c.Status),
					c.Version,
					ha,
					c.Location.LocationName,
					strconv.Itoa(c.WorkerCount),
					strconv.Itoa(c.NodePoolCount),
				)
			}
			t.Render()
			return nil
		},
	}
}

// --- show ---

func showCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <cluster_uuid>",
		Short: "Show Kubernetes cluster details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			s := output.NewSpinner("Fetching cluster...")
			s.Start()
			resp, err := client.Get("/kubernetes/" + args[0])
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			var cluster struct {
				UUID           string `json:"uuid"`
				Name           string `json:"name"`
				Label          string `json:"label"`
				Status         string `json:"status"`
				Version        string `json:"version"`
				HAControlPlane bool   `json:"ha_control_plane"`
				APIEndpoint    string `json:"api_endpoint"`
				PodCIDR        string `json:"pod_cidr"`
				ServiceCIDR    string `json:"service_cidr"`
				BillingType    string `json:"billing_type"`
				Location       struct {
					LocationName string `json:"location_name"`
					Description  string `json:"description"`
				} `json:"location"`
				Network *struct {
					Name    string `json:"name"`
					IPRange string `json:"ip_range"`
					Prefix  int    `json:"prefix"`
				} `json:"network"`
				NodePools []struct {
					UUID         string `json:"uuid"`
					Name         string `json:"name"`
					DesiredNodes int    `json:"desired_nodes"`
					MinNodes     int    `json:"min_nodes"`
					MaxNodes     int    `json:"max_nodes"`
					AutoScale    bool   `json:"auto_scale"`
					Plan         struct {
						Name string `json:"name"`
					} `json:"plan"`
					Nodes []struct {
						VPSName    string `json:"vps_name"`
						VPSStatus  string `json:"vps_status"`
						K8sStatus  string `json:"k8s_status"`
						FloatingIP string `json:"floating_ip"`
						PrivateIP  string `json:"private_ip"`
					} `json:"nodes"`
				} `json:"node_pools"`
				CreatedAt string `json:"created_at"`
			}
			if err := json.Unmarshal(resp, &cluster); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}

			ha := "No"
			if cluster.HAControlPlane {
				ha = "Yes"
			}

			fmt.Println()
			fmt.Printf("  Cluster:      %s (%s)\n", cluster.Name, cluster.UUID)
			if cluster.Label != "" {
				fmt.Printf("  Label:        %s\n", cluster.Label)
			}
			fmt.Printf("  Status:       %s\n", output.FormatStatus(cluster.Status))
			fmt.Printf("  Version:      %s\n", cluster.Version)
			fmt.Printf("  HA:           %s\n", ha)
			if cluster.APIEndpoint != "" {
				fmt.Printf("  API Endpoint: %s\n", cluster.APIEndpoint)
			}
			fmt.Printf("  Location:     %s\n", cluster.Location.LocationName)
			fmt.Printf("  Pod CIDR:     %s\n", cluster.PodCIDR)
			fmt.Printf("  Service CIDR: %s\n", cluster.ServiceCIDR)
			if cluster.Network != nil {
				fmt.Printf("  Network:      %s (%s/%d)\n", cluster.Network.Name, cluster.Network.IPRange, cluster.Network.Prefix)
			}
			fmt.Printf("  Billing:      %s\n", cluster.BillingType)
			fmt.Printf("  Created:      %s\n", cluster.CreatedAt)
			fmt.Println()

			for _, pool := range cluster.NodePools {
				autoScale := "Off"
				if pool.AutoScale {
					autoScale = "On"
				}
				fmt.Printf("  Node Pool: %s (%s) — Plan: %s, Nodes: %d (min: %d, max: %d), AutoScale: %s\n",
					pool.Name, pool.UUID, pool.Plan.Name, pool.DesiredNodes, pool.MinNodes, pool.MaxNodes, autoScale)

				if len(pool.Nodes) > 0 {
					nt := output.NewTable("", []string{"Name", "VPS Status", "K8s Status", "Floating IP", "Private IP"})
					for _, n := range pool.Nodes {
						nt.AddRow(
							n.VPSName,
							output.FormatStatus(n.VPSStatus),
							output.FormatStatus(n.K8sStatus),
							n.FloatingIP,
							n.PrivateIP,
						)
					}
					nt.Render()
				}
			}

			return nil
		},
	}
}

// --- create ---

func createCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new Kubernetes cluster",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			projectID, _ := cmd.Flags().GetInt("project")
			name, _ := cmd.Flags().GetString("name")
			location, _ := cmd.Flags().GetString("location")
			version, _ := cmd.Flags().GetString("version")
			ha, _ := cmd.Flags().GetBool("ha")
			plan, _ := cmd.Flags().GetString("plan")
			nodes, _ := cmd.Flags().GetInt("nodes")
			networkID, _ := cmd.Flags().GetInt("network-id")
			nodeCIDR, _ := cmd.Flags().GetString("node-cidr")
			podCIDR, _ := cmd.Flags().GetString("pod-cidr")
			serviceCIDR, _ := cmd.Flags().GetString("service-cidr")

			body := map[string]interface{}{
				"project_id":       projectID,
				"name":             name,
				"location_name":    location,
				"ha_control_plane": ha,
				"node_pools": []map[string]interface{}{
					{
						"name":  "default",
						"plan":  plan,
						"count": nodes,
					},
				},
			}

			if version != "" {
				body["version"] = version
			}

			network := map[string]interface{}{}
			if networkID != 0 {
				network["network_id"] = networkID
			}
			if nodeCIDR != "" {
				network["node_cidr"] = nodeCIDR
			}
			if podCIDR != "10.42.0.0/16" {
				network["pod_cidr"] = podCIDR
			}
			if serviceCIDR != "10.43.0.0/16" {
				network["service_cidr"] = serviceCIDR
			}
			if len(network) > 0 {
				body["network"] = network
			}

			s := output.NewSpinner("Creating Kubernetes cluster...")
			s.Start()
			resp, err := client.Post("/kubernetes/", body)
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
				output.PrintSuccess(fmt.Sprintf("Cluster created: %s", result.UUID))
			} else if result.Detail != "" {
				output.PrintSuccess(result.Detail)
			} else {
				output.PrintSuccess("Cluster creation initiated")
			}
			return nil
		},
	}
	cmd.Flags().IntP("project", "p", 0, "Project ID")
	cmd.Flags().StringP("name", "n", "", "Cluster name")
	cmd.Flags().StringP("location", "l", "", "Location name")
	cmd.Flags().String("version", "", "Kubernetes version (default: latest)")
	cmd.Flags().Bool("ha", false, "Enable HA control plane")
	cmd.Flags().String("plan", "", "Server plan for the default node pool")
	cmd.Flags().Int("nodes", 1, "Number of initial worker nodes")
	cmd.Flags().Int("network-id", 0, "Existing network ID")
	cmd.Flags().String("node-cidr", "", "Custom node CIDR")
	cmd.Flags().String("pod-cidr", "10.42.0.0/16", "Pod CIDR")
	cmd.Flags().String("service-cidr", "10.43.0.0/16", "Service CIDR")
	_ = cmd.MarkFlagRequired("project")
	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("location")
	_ = cmd.MarkFlagRequired("plan")
	return cmd
}

// --- update ---

func updateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update <cluster_uuid>",
		Short: "Update a Kubernetes cluster",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			body := map[string]interface{}{}
			if cmd.Flags().Changed("name") {
				name, _ := cmd.Flags().GetString("name")
				body["name"] = name
			}
			if cmd.Flags().Changed("label") {
				label, _ := cmd.Flags().GetString("label")
				body["label"] = label
			}

			if len(body) == 0 {
				return fmt.Errorf("at least one of --name or --label must be provided")
			}

			s := output.NewSpinner("Updating cluster...")
			s.Start()
			resp, err := client.Patch("/kubernetes/"+args[0], body)
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			output.PrintSuccess("Cluster updated successfully")
			return nil
		},
	}
	cmd.Flags().StringP("name", "n", "", "New cluster name")
	cmd.Flags().String("label", "", "New cluster label")
	return cmd
}

// --- delete ---

func deleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <cluster_uuid>",
		Short: "Delete a Kubernetes cluster",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmdutil.CheckForce(cmd, fmt.Sprintf("Are you sure you want to delete cluster %s?", args[0])) {
				return nil
			}

			client := cmdutil.GetClient(cmd)

			s := output.NewSpinner("Deleting cluster...")
			s.Start()
			resp, err := client.Delete("/kubernetes/" + args[0])
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			output.PrintSuccess("Cluster deletion initiated")
			return nil
		},
	}
	cmd.Flags().BoolP("force", "f", false, "Skip confirmation")
	return cmd
}

// --- kubeconfig ---

func kubeconfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "kubeconfig <cluster_uuid>",
		Short: "Download kubeconfig for a cluster",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			s := output.NewSpinner("Fetching kubeconfig...")
			s.Start()
			resp, err := client.Get("/kubernetes/" + args[0] + "/kubeconfig")
			s.Stop()
			if err != nil {
				return err
			}

			outputPath, _ := cmd.Flags().GetString("output")
			if outputPath != "" {
				// The response is a JSON with kubeconfig field or raw YAML
				var result struct {
					Kubeconfig string `json:"kubeconfig"`
				}
				if err := json.Unmarshal(resp, &result); err == nil && result.Kubeconfig != "" {
					if err := os.WriteFile(outputPath, []byte(result.Kubeconfig), 0600); err != nil {
						return fmt.Errorf("failed to write kubeconfig: %w", err)
					}
					output.PrintSuccess(fmt.Sprintf("Kubeconfig saved to %s", outputPath))
					return nil
				}
				// Fallback: write raw response
				if err := os.WriteFile(outputPath, resp, 0600); err != nil {
					return fmt.Errorf("failed to write kubeconfig: %w", err)
				}
				output.PrintSuccess(fmt.Sprintf("Kubeconfig saved to %s", outputPath))
				return nil
			}

			// Print to stdout
			var result struct {
				Kubeconfig string `json:"kubeconfig"`
			}
			if err := json.Unmarshal(resp, &result); err == nil && result.Kubeconfig != "" {
				fmt.Print(result.Kubeconfig)
			} else {
				fmt.Print(string(resp))
			}
			return nil
		},
	}
	cmd.Flags().StringP("output", "o", "", "Save kubeconfig to file")
	return cmd
}

// --- move ---

func moveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "move <cluster_uuid>",
		Short: "Move a cluster to another project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			projectID, _ := cmd.Flags().GetInt("project")

			body := map[string]interface{}{
				"project_id": projectID,
			}

			s := output.NewSpinner("Moving cluster...")
			s.Start()
			resp, err := client.Post("/kubernetes/"+args[0]+"/move", body)
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			output.PrintSuccess("Cluster moved successfully")
			return nil
		},
	}
	cmd.Flags().IntP("project", "p", 0, "Target project ID")
	_ = cmd.MarkFlagRequired("project")
	return cmd
}

// --- loadbalancers ---

func loadbalancersCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "loadbalancers <cluster_uuid>",
		Short: "List load balancers targeting a cluster",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			s := output.NewSpinner("Fetching load balancers...")
			s.Start()
			resp, err := client.Get("/kubernetes/" + args[0] + "/loadbalancers")
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			var lbs []struct {
				UUID   string `json:"uuid"`
				Name   string `json:"name"`
				Status string `json:"status"`
				IP     string `json:"floating_ip_address"`
			}
			if err := json.Unmarshal(resp, &lbs); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}

			if len(lbs) == 0 {
				fmt.Println("No load balancers found for this cluster.")
				return nil
			}

			t := output.NewTable("Cluster Load Balancers", []string{"UUID", "Name", "Status", "IP"})
			for _, lb := range lbs {
				t.AddRow(lb.UUID, lb.Name, output.FormatStatus(lb.Status), lb.IP)
			}
			t.Render()
			return nil
		},
	}
}

// parseLabels converts "key1=val1,key2=val2" to map[string]string
func parseLabels(raw []string) map[string]string {
	labels := make(map[string]string)
	for _, l := range raw {
		parts := strings.SplitN(l, "=", 2)
		if len(parts) == 2 {
			labels[parts[0]] = parts[1]
		}
	}
	return labels
}

// parseTaints converts "key=val:Effect" to []map[string]string
func parseTaints(raw []string) []map[string]string {
	var taints []map[string]string
	for _, t := range raw {
		// format: key=value:Effect
		colonIdx := strings.LastIndex(t, ":")
		if colonIdx < 0 {
			continue
		}
		kvPart := t[:colonIdx]
		effect := t[colonIdx+1:]
		eqIdx := strings.Index(kvPart, "=")
		if eqIdx < 0 {
			continue
		}
		taints = append(taints, map[string]string{
			"key":    kvPart[:eqIdx],
			"value":  kvPart[eqIdx+1:],
			"effect": effect,
		})
	}
	return taints
}
