package ddosmitigation

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/CubePathInc/cubecli/internal/cmdutil"
	"github.com/CubePathInc/cubecli/internal/output"
	"github.com/spf13/cobra"
)

const timeLayout = "2006-01-02T15:04:05Z"

// timeWindow resolves --start/--end (RFC 3339) or --since (a duration back from now).
func timeWindow(cmd *cobra.Command, now time.Time) (string, string, error) {
	startStr, _ := cmd.Flags().GetString("start")
	endStr, _ := cmd.Flags().GetString("end")
	since, _ := cmd.Flags().GetDuration("since")

	end := now.UTC()
	if endStr != "" {
		t, err := time.Parse(time.RFC3339, endStr)
		if err != nil {
			return "", "", fmt.Errorf("invalid --end %q: use RFC 3339, e.g. 2026-09-30T10:00:00Z", endStr)
		}
		end = t.UTC()
	}
	start := end.Add(-since)
	if startStr != "" {
		t, err := time.Parse(time.RFC3339, startStr)
		if err != nil {
			return "", "", fmt.Errorf("invalid --start %q: use RFC 3339, e.g. 2026-09-30T09:00:00Z", startStr)
		}
		start = t.UTC()
	}
	if !start.Before(end) {
		return "", "", fmt.Errorf("the start of the window must be before its end")
	}
	return start.Format(timeLayout), end.Format(timeLayout), nil
}

func addWindowFlags(cmd *cobra.Command) {
	cmd.Flags().Duration("since", time.Hour, "Window length back from --end (e.g. 15m, 6h)")
	cmd.Flags().String("start", "", "Window start (RFC 3339); overrides --since")
	cmd.Flags().String("end", "", "Window end (RFC 3339, default now)")
}

func trafficCmd() *cobra.Command {
	trafficCmd := &cobra.Command{
		Use:   "traffic",
		Short: "Inspect traffic seen by the scrubbing edge for your Premium IPs",
	}

	protectedCmd := &cobra.Command{
		Use:   "protected-ips",
		Short: "List the IPs whose traffic can be captured",
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := fetch(cmd, "Fetching protected IPs...", "/ddos-mitigation/traffic-capture/protected-ips")
			if err != nil || resp == nil {
				return err
			}
			var r struct {
				IPs []struct {
					Network  string `json:"network"`
					Location string `json:"location"`
				} `json:"ips"`
			}
			if err := json.Unmarshal(resp, &r); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}
			t := output.NewTable("Protected IPs", []string{"Network", "Location"})
			for _, ip := range r.IPs {
				t.AddRow(ip.Network, ip.Location)
			}
			t.Render()
			return nil
		},
	}

	captureCmd := &cobra.Command{
		Use:   "capture <destination_ip_or_cidr>",
		Short: "Show sampled packets sent to an IP (newest first)",
		Example: `  cubecli ddos traffic capture 194.26.100.205 --since 15m --action DROP --limit 100
  cubecli ddos traffic capture 194.26.100.205 --protocol UDP --dst-port 30120`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			start, end, err := timeWindow(cmd, time.Now())
			if err != nil {
				return err
			}
			body := map[string]interface{}{
				"start_time":     start,
				"end_time":       end,
				"destination_ip": args[0],
			}
			lists := map[string]string{
				"source":         "include_src_ips",
				"exclude-source": "exclude_src_ips",
				"protocol":       "include_protocols",
				"action":         "include_actions",
				"tcp-flag":       "include_tcp_flags",
			}
			for flag, field := range lists {
				if v, _ := cmd.Flags().GetStringSlice(flag); len(v) > 0 {
					up := splitList(v)
					for i := range up {
						if field != "include_src_ips" && field != "exclude_src_ips" {
							up[i] = strings.ToUpper(up[i])
						}
					}
					body[field] = up
				}
			}
			if v, _ := cmd.Flags().GetIntSlice("dst-port"); len(v) > 0 {
				body["include_dst_ports"] = v
			}
			if v, _ := cmd.Flags().GetIntSlice("src-port"); len(v) > 0 {
				body["include_src_ports"] = v
			}
			if limit, _ := cmd.Flags().GetInt("limit"); limit > 0 {
				body["limit"] = limit
			}

			s := output.NewSpinner("Capturing traffic...")
			s.Start()
			resp, err := client.Post("/ddos-mitigation/traffic-capture", body)
			s.Stop()
			if err != nil {
				return err
			}
			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			var r struct {
				TotalLogs int `json:"total_logs"`
				Logs      []struct {
					Timestamp  string `json:"timestamp"`
					SrcIP      string `json:"src_ip"`
					SrcPort    int    `json:"src_port"`
					DstIP      string `json:"dst_ip"`
					DstPort    int    `json:"dst_port"`
					Protocol   string `json:"protocol"`
					Action     string `json:"action"`
					Mitigation string `json:"mitigation_name"`
					PacketLen  int    `json:"packet_len"`
					TCPFlags   string `json:"tcp_flags"`
					SrcCountry string `json:"src_country"`
				} `json:"logs"`
			}
			if err := json.Unmarshal(resp, &r); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}
			t := output.NewTable(fmt.Sprintf("Captured Packets (%d)", r.TotalLogs), []string{"Time", "Source", "Country", "Destination", "Protocol", "Flags", "Length", "Action", "Mitigation"})
			for _, l := range r.Logs {
				t.AddRow(l.Timestamp, fmt.Sprintf("%s:%d", l.SrcIP, l.SrcPort), l.SrcCountry, fmt.Sprintf("%s:%d", l.DstIP, l.DstPort), l.Protocol, l.TCPFlags, strconv.Itoa(l.PacketLen), l.Action, l.Mitigation)
			}
			t.Render()
			return nil
		},
	}
	addWindowFlags(captureCmd)
	captureCmd.Flags().StringSlice("source", nil, "Only these source IPs or CIDRs (max 10)")
	captureCmd.Flags().StringSlice("exclude-source", nil, "Exclude these source IPs or CIDRs (max 10)")
	captureCmd.Flags().StringSlice("protocol", nil, "Only these protocols: TCP, UDP, ICMP, OTHER")
	captureCmd.Flags().StringSlice("action", nil, "Only these actions: PASS, DROP")
	captureCmd.Flags().StringSlice("tcp-flag", nil, "Only packets with these TCP flags (SYN, ACK, FIN, RST...)")
	captureCmd.Flags().IntSlice("dst-port", nil, "Only these destination ports")
	captureCmd.Flags().IntSlice("src-port", nil, "Only these source ports")
	captureCmd.Flags().Int("limit", 1000, "Maximum packets to return (1-100000)")

	statsCmd := &cobra.Command{
		Use:   "stats [destination_ip_or_cidr...]",
		Short: "Show passed and dropped traffic over time (all Premium IPs by default)",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			start, end, err := timeWindow(cmd, time.Now())
			if err != nil {
				return err
			}
			interval, _ := cmd.Flags().GetString("interval")
			body := map[string]interface{}{
				"start_time":      start,
				"end_time":        end,
				"destination_ips": append([]string{}, args...),
				"interval":        interval,
			}

			s := output.NewSpinner("Fetching traffic statistics...")
			s.Start()
			resp, err := client.Post("/ddos-mitigation/traffic-capture/stats", body)
			s.Stop()
			if err != nil {
				return err
			}
			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			var r struct {
				TotalPass int64 `json:"total_pass"`
				TotalDrop int64 `json:"total_drop"`
				Buckets   []struct {
					Timestamp string  `json:"timestamp"`
					PassCount int64   `json:"pass_count"`
					DropCount int64   `json:"drop_count"`
					PassPPS   float64 `json:"pass_pps"`
					DropPPS   float64 `json:"drop_pps"`
					PassBytes int64   `json:"pass_bytes"`
					DropBytes int64   `json:"drop_bytes"`
				} `json:"buckets"`
			}
			if err := json.Unmarshal(resp, &r); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}
			t := output.NewTable(fmt.Sprintf("Traffic (%d passed, %d dropped samples)", r.TotalPass, r.TotalDrop), []string{"Time", "Pass PPS", "Drop PPS", "Pass Bytes", "Drop Bytes"})
			for _, b := range r.Buckets {
				t.AddRow(b.Timestamp, fmt.Sprintf("%.2f", b.PassPPS), fmt.Sprintf("%.2f", b.DropPPS), strconv.FormatInt(b.PassBytes, 10), strconv.FormatInt(b.DropBytes, 10))
			}
			t.Render()
			return nil
		},
	}
	addWindowFlags(statsCmd)
	statsCmd.Flags().String("interval", "1m", "Bucket size: 10s, 30s, 1m, 5m, 15m, 1h")

	trafficCmd.AddCommand(protectedCmd, captureCmd, statsCmd)
	return trafficCmd
}
