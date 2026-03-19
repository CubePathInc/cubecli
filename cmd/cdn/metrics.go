package cdn

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/CubePathInc/cubecli/internal/cmdutil"
	"github.com/CubePathInc/cubecli/internal/output"
	"github.com/spf13/cobra"
)

// printMetricsData handles generic metrics responses.
// If the response is a JSON object, it displays key-value pairs.
// If the response is a JSON array of objects, it displays as a table.
func printMetricsData(title string, data json.RawMessage) error {
	// Try as array of objects first
	var arr []map[string]interface{}
	if err := json.Unmarshal(data, &arr); err == nil && len(arr) > 0 {
		// Collect headers from the first element
		var headers []string
		for k := range arr[0] {
			headers = append(headers, k)
		}

		t := output.NewTable(title, headers)
		for _, item := range arr {
			var row []string
			for _, h := range headers {
				row = append(row, fmt.Sprintf("%v", item[h]))
			}
			t.AddRow(row...)
		}
		t.Render()
		return nil
	}

	// Try as object (key-value pairs)
	var obj map[string]interface{}
	if err := json.Unmarshal(data, &obj); err == nil {
		t := output.NewTable(title, []string{"Field", "Value"})
		for k, v := range obj {
			t.AddRow(k, fmt.Sprintf("%v", v))
		}
		t.Render()
		return nil
	}

	// Fallback: print raw JSON
	return output.PrintJSON(json.RawMessage(data))
}

func addMinutesFlag(cmd *cobra.Command) {
	cmd.Flags().IntP("minutes", "m", 60, "Time range in minutes")
}

func addLimitFlag(cmd *cobra.Command) {
	cmd.Flags().IntP("limit", "l", 20, "Maximum number of results")
}

func addMetricsCmd(parent *cobra.Command) {
	metricsCmd := &cobra.Command{
		Use:   "metrics",
		Short: "CDN metrics and analytics",
	}

	metricsSummaryCmd := &cobra.Command{
		Use:   "summary <zone_uuid>",
		Short: "Show CDN metrics summary",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			zoneUUID := args[0]
			minutes, _ := cmd.Flags().GetInt("minutes")

			s := output.NewSpinner("Fetching metrics summary...")
			s.Start()
			resp, err := client.Get(fmt.Sprintf("/cdn/zones/%s/metrics/summary?minutes=%d", zoneUUID, minutes))
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			var summary struct {
				TotalRequests  int     `json:"total_requests"`
				TotalBandwidth float64 `json:"total_bandwidth"`
				CacheHitRate   float64 `json:"cache_hit_rate"`
				ErrorRate      float64 `json:"error_rate"`
			}
			if err := json.Unmarshal(resp, &summary); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}

			t := output.NewTable("CDN Metrics Summary", []string{"Field", "Value"})
			t.AddRow("Total Requests", strconv.Itoa(summary.TotalRequests))
			t.AddRow("Total Bandwidth", fmt.Sprintf("%.2f", summary.TotalBandwidth))
			t.AddRow("Cache Hit Rate", fmt.Sprintf("%.2f%%", summary.CacheHitRate))
			t.AddRow("Error Rate", fmt.Sprintf("%.2f%%", summary.ErrorRate))
			t.Render()
			return nil
		},
	}

	metricsRequestsCmd := &cobra.Command{
		Use:   "requests <zone_uuid>",
		Short: "Show CDN request metrics",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			zoneUUID := args[0]
			minutes, _ := cmd.Flags().GetInt("minutes")
			interval, _ := cmd.Flags().GetInt("interval")

			s := output.NewSpinner("Fetching request metrics...")
			s.Start()
			resp, err := client.Get(fmt.Sprintf("/cdn/zones/%s/metrics/requests?minutes=%d&interval_seconds=%d", zoneUUID, minutes, interval))
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			return printMetricsData("Request Metrics", resp)
		},
	}

	metricsBandwidthCmd := &cobra.Command{
		Use:   "bandwidth <zone_uuid>",
		Short: "Show CDN bandwidth metrics",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			zoneUUID := args[0]
			minutes, _ := cmd.Flags().GetInt("minutes")
			groupBy, _ := cmd.Flags().GetString("group-by")
			interval, _ := cmd.Flags().GetInt("interval")

			s := output.NewSpinner("Fetching bandwidth metrics...")
			s.Start()
			resp, err := client.Get(fmt.Sprintf("/cdn/zones/%s/metrics/bandwidth?minutes=%d&group_by=%s&interval_seconds=%d", zoneUUID, minutes, groupBy, interval))
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			return printMetricsData("Bandwidth Metrics", resp)
		},
	}

	metricsCacheCmd := &cobra.Command{
		Use:   "cache <zone_uuid>",
		Short: "Show CDN cache metrics",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			zoneUUID := args[0]
			minutes, _ := cmd.Flags().GetInt("minutes")

			s := output.NewSpinner("Fetching cache metrics...")
			s.Start()
			resp, err := client.Get(fmt.Sprintf("/cdn/zones/%s/metrics/cache?minutes=%d", zoneUUID, minutes))
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			return printMetricsData("Cache Metrics", resp)
		},
	}

	metricsStatusCodesCmd := &cobra.Command{
		Use:   "status-codes <zone_uuid>",
		Short: "Show CDN status code metrics",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			zoneUUID := args[0]
			minutes, _ := cmd.Flags().GetInt("minutes")

			s := output.NewSpinner("Fetching status code metrics...")
			s.Start()
			resp, err := client.Get(fmt.Sprintf("/cdn/zones/%s/metrics/status-codes?minutes=%d", zoneUUID, minutes))
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			return printMetricsData("Status Code Metrics", resp)
		},
	}

	metricsTopURLsCmd := &cobra.Command{
		Use:   "top-urls <zone_uuid>",
		Short: "Show top requested URLs",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			zoneUUID := args[0]
			minutes, _ := cmd.Flags().GetInt("minutes")
			limit, _ := cmd.Flags().GetInt("limit")

			s := output.NewSpinner("Fetching top URLs...")
			s.Start()
			resp, err := client.Get(fmt.Sprintf("/cdn/zones/%s/metrics/top-urls?minutes=%d&limit=%d", zoneUUID, minutes, limit))
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			return printMetricsData("Top URLs", resp)
		},
	}

	metricsTopCountriesCmd := &cobra.Command{
		Use:   "top-countries <zone_uuid>",
		Short: "Show top countries by requests",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			zoneUUID := args[0]
			minutes, _ := cmd.Flags().GetInt("minutes")
			limit, _ := cmd.Flags().GetInt("limit")

			s := output.NewSpinner("Fetching top countries...")
			s.Start()
			resp, err := client.Get(fmt.Sprintf("/cdn/zones/%s/metrics/top-countries?minutes=%d&limit=%d", zoneUUID, minutes, limit))
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			return printMetricsData("Top Countries", resp)
		},
	}

	metricsTopASNCmd := &cobra.Command{
		Use:   "top-asn <zone_uuid>",
		Short: "Show top ASNs by requests",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			zoneUUID := args[0]
			minutes, _ := cmd.Flags().GetInt("minutes")
			limit, _ := cmd.Flags().GetInt("limit")

			s := output.NewSpinner("Fetching top ASNs...")
			s.Start()
			resp, err := client.Get(fmt.Sprintf("/cdn/zones/%s/metrics/top-asn?minutes=%d&limit=%d", zoneUUID, minutes, limit))
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			return printMetricsData("Top ASNs", resp)
		},
	}

	metricsTopUserAgentsCmd := &cobra.Command{
		Use:   "top-user-agents <zone_uuid>",
		Short: "Show top user agents by requests",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			zoneUUID := args[0]
			minutes, _ := cmd.Flags().GetInt("minutes")
			limit, _ := cmd.Flags().GetInt("limit")

			s := output.NewSpinner("Fetching top user agents...")
			s.Start()
			resp, err := client.Get(fmt.Sprintf("/cdn/zones/%s/metrics/top-user-agents?minutes=%d&limit=%d", zoneUUID, minutes, limit))
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			return printMetricsData("Top User Agents", resp)
		},
	}

	metricsBlockedCmd := &cobra.Command{
		Use:   "blocked <zone_uuid>",
		Short: "Show blocked request metrics",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			zoneUUID := args[0]
			minutes, _ := cmd.Flags().GetInt("minutes")

			s := output.NewSpinner("Fetching blocked metrics...")
			s.Start()
			resp, err := client.Get(fmt.Sprintf("/cdn/zones/%s/metrics/blocked?minutes=%d", zoneUUID, minutes))
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			return printMetricsData("Blocked Metrics", resp)
		},
	}

	metricsPopsCmd := &cobra.Command{
		Use:   "pops <zone_uuid>",
		Short: "Show metrics by PoP location",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			zoneUUID := args[0]
			minutes, _ := cmd.Flags().GetInt("minutes")

			s := output.NewSpinner("Fetching PoP metrics...")
			s.Start()
			resp, err := client.Get(fmt.Sprintf("/cdn/zones/%s/metrics/pops?minutes=%d", zoneUUID, minutes))
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			return printMetricsData("PoP Metrics", resp)
		},
	}

	metricsFileExtensionsCmd := &cobra.Command{
		Use:   "file-extensions <zone_uuid>",
		Short: "Show metrics by file extension",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			zoneUUID := args[0]
			minutes, _ := cmd.Flags().GetInt("minutes")
			limit, _ := cmd.Flags().GetInt("limit")

			s := output.NewSpinner("Fetching file extension metrics...")
			s.Start()
			resp, err := client.Get(fmt.Sprintf("/cdn/zones/%s/metrics/file-extensions?minutes=%d&limit=%d", zoneUUID, minutes, limit))
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			return printMetricsData("File Extension Metrics", resp)
		},
	}

	// Summary
	addMinutesFlag(metricsSummaryCmd)

	// Requests
	addMinutesFlag(metricsRequestsCmd)
	metricsRequestsCmd.Flags().Int("interval", 60, "Interval in seconds")

	// Bandwidth
	addMinutesFlag(metricsBandwidthCmd)
	metricsBandwidthCmd.Flags().StringP("group-by", "g", "time", "Group by field")
	metricsBandwidthCmd.Flags().Int("interval", 60, "Interval in seconds")

	// Cache
	addMinutesFlag(metricsCacheCmd)

	// Status codes
	addMinutesFlag(metricsStatusCodesCmd)

	// Top URLs
	addMinutesFlag(metricsTopURLsCmd)
	addLimitFlag(metricsTopURLsCmd)

	// Top countries
	addMinutesFlag(metricsTopCountriesCmd)
	addLimitFlag(metricsTopCountriesCmd)

	// Top ASN
	addMinutesFlag(metricsTopASNCmd)
	addLimitFlag(metricsTopASNCmd)

	// Top user agents
	addMinutesFlag(metricsTopUserAgentsCmd)
	addLimitFlag(metricsTopUserAgentsCmd)

	// Blocked
	addMinutesFlag(metricsBlockedCmd)

	// PoPs
	addMinutesFlag(metricsPopsCmd)

	// File extensions
	addMinutesFlag(metricsFileExtensionsCmd)
	addLimitFlag(metricsFileExtensionsCmd)

	metricsCmd.AddCommand(
		metricsSummaryCmd,
		metricsRequestsCmd,
		metricsBandwidthCmd,
		metricsCacheCmd,
		metricsStatusCodesCmd,
		metricsTopURLsCmd,
		metricsTopCountriesCmd,
		metricsTopASNCmd,
		metricsTopUserAgentsCmd,
		metricsBlockedCmd,
		metricsPopsCmd,
		metricsFileExtensionsCmd,
	)
	parent.AddCommand(metricsCmd)
}
