package kubernetes

import (
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/CubePathInc/cubecli/internal/cmdutil"
	"github.com/CubePathInc/cubecli/internal/output"
	"github.com/spf13/cobra"
)

func protectionCmd() *cobra.Command {
	return cmdutil.ProtectionCmd("cluster_uuid", "Kubernetes cluster", cmdutil.StringPath("/kubernetes/%s/protection"))
}

// metricsCmd shows cluster health series, or a node's with --node.
func metricsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "metrics <cluster_uuid>",
		Short: "Show cluster health metrics, or a node's with --node",
		Long: `Show the cluster's health series (nodes ready, pending and failed pods, API
latency) or, with --node, a node's readiness and usage plus its server's.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			timeRange, _ := cmd.Flags().GetString("range")
			node, _ := cmd.Flags().GetString("node")

			path := "/kubernetes/" + args[0] + "/metrics"
			title := "Cluster Metrics"
			if node != "" {
				path = "/kubernetes/" + args[0] + "/nodes/" + url.PathEscape(node) + "/metrics"
				title = fmt.Sprintf("Node %s Metrics", node)
			}
			path += "?time_range=" + url.QueryEscape(timeRange)

			s := output.NewSpinner("Fetching metrics...")
			s.Start()
			resp, err := client.Get(path)
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}
			return output.RenderSeries(title, resp)
		},
	}
	cmd.Flags().String("range", "1h", "Time range: 1h, 3h, 6h, 12h, 24h, 3d, 7d, 30d")
	cmd.Flags().String("node", "", "Node name (see 'kubernetes show')")
	return cmd
}
