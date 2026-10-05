package dns

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/CubePathInc/cubecli/internal/cmdutil"
	"github.com/CubePathInc/cubecli/internal/output"
	"github.com/spf13/cobra"
)

type healthCheckPoPStatus struct {
	PoP    string `json:"pop"`
	Status string `json:"status"`
}

// healthCheckSummary is the aggregated state across every probing location
// (status_summary); older API versions do not send it.
type healthCheckSummary struct {
	Status         string                 `json:"status"`
	NodesReporting int                    `json:"nodes_reporting"`
	NodesUnhealthy int                    `json:"nodes_unhealthy"`
	PoPs           []healthCheckPoPStatus `json:"pops"`
	LastChangeAt   *string                `json:"last_change_at"`
}

type healthCheck struct {
	UUID               string              `json:"uuid"`
	RecordUUID         string              `json:"record_uuid"`
	Name               string              `json:"name"`
	CheckType          string              `json:"check_type"`
	Target             string              `json:"target"`
	Port               *int                `json:"port"`
	Path               string              `json:"path"`
	ExpectedStatus     *int                `json:"expected_status"`
	IntervalSecs       int                 `json:"interval_secs"`
	TimeoutSecs        int                 `json:"timeout_secs"`
	HealthyThreshold   int                 `json:"healthy_threshold"`
	UnhealthyThreshold int                 `json:"unhealthy_threshold"`
	Enabled            bool                `json:"enabled"`
	LastStatus         string              `json:"last_status"`
	LastCheckAt        string              `json:"last_check_at"`
	StatusSummary      *healthCheckSummary `json:"status_summary"`
	Uptime24h          *float64            `json:"uptime_24h"`
}

func (h healthCheck) probe() string {
	target := h.Target
	if target == "" {
		target = "(record value)"
	}
	if h.Port != nil {
		target = fmt.Sprintf("%s:%d", target, *h.Port)
	}
	return fmt.Sprintf("%s %s%s", h.CheckType, target, h.Path)
}

// status prefers the aggregated status (healthy, degraded, unhealthy, unknown,
// paused) and falls back to last_status.
func (h healthCheck) status() string {
	s := h.StatusSummary
	if s == nil || s.Status == "" {
		return output.FormatStatus(h.LastStatus)
	}
	out := output.FormatStatus(s.Status)
	if s.NodesReporting > 0 && s.NodesUnhealthy > 0 {
		out += fmt.Sprintf(" (%d/%d down)", s.NodesUnhealthy, s.NodesReporting)
	}
	return out
}

func formatPct(v *float64) string {
	if v == nil {
		return "-"
	}
	return strconv.FormatFloat(*v, 'f', 2, 64) + "%"
}

func formatSecs(secs int64) string {
	if secs <= 0 {
		return "0s"
	}
	d, h, m, s := secs/86400, secs%86400/3600, secs%3600/60, secs%60
	switch {
	case d > 0:
		return fmt.Sprintf("%dd %dh", d, h)
	case h > 0:
		return fmt.Sprintf("%dh %dm", h, m)
	case m > 0:
		return fmt.Sprintf("%dm %ds", m, s)
	default:
		return fmt.Sprintf("%ds", s)
	}
}

func strOr(s *string, def string) string {
	if s == nil || *s == "" {
		return def
	}
	return *s
}

// healthHistoryRanges are the windows the history endpoint accepts.
var healthHistoryRanges = []string{"24h", "7d", "30d", "90d"}

type healthHistoryPoP struct {
	PoP            string   `json:"pop"`
	Region         string   `json:"region"`
	Status         string   `json:"status"`
	NodesTotal     int      `json:"nodes_total"`
	NodesUnhealthy int      `json:"nodes_unhealthy"`
	UptimePct      *float64 `json:"uptime_pct"`
	CoveragePct    *float64 `json:"coverage_pct"`
	LastErrorKind  *string  `json:"last_error_kind"`
	LastHTTPStatus *int     `json:"last_http_status"`
	LastLatencyMs  *int     `json:"last_latency_ms"`
	LastChangeAt   *string  `json:"last_change_at"`
}

type healthHistoryIncident struct {
	PoP          string  `json:"pop"`
	NodeIndex    int     `json:"node_index"`
	StartedAt    string  `json:"started_at"`
	ResolvedAt   *string `json:"resolved_at"`
	DurationSecs *int64  `json:"duration_secs"`
	ErrorKind    *string `json:"error_kind"`
	HTTPStatus   *int    `json:"http_status"`
}

type healthHistory struct {
	CheckUUID       string `json:"check_uuid"`
	TimeRange       string `json:"time_range"`
	RetentionDays   int    `json:"retention_days"`
	Clamped         bool   `json:"clamped"`
	HistoryStartsAt string `json:"history_starts_at"`
	Start           string `json:"start"`
	End             string `json:"end"`
	Overall         struct {
		Status         string   `json:"status"`
		NodesReporting int      `json:"nodes_reporting"`
		NodesUnhealthy int      `json:"nodes_unhealthy"`
		UptimePct      *float64 `json:"uptime_pct"`
		CoveragePct    *float64 `json:"coverage_pct"`
		OutageSecs     int64    `json:"outage_secs"`
		LastChangeAt   *string  `json:"last_change_at"`
	} `json:"overall"`
	PoPs      []healthHistoryPoP      `json:"pops"`
	Incidents []healthHistoryIncident `json:"incidents"`
}

func lastError(kind *string, httpStatus *int) string {
	k := strOr(kind, "")
	if httpStatus != nil {
		if k == "" {
			return fmt.Sprintf("HTTP %d", *httpStatus)
		}
		return fmt.Sprintf("%s (HTTP %d)", k, *httpStatus)
	}
	if k == "" {
		return "-"
	}
	return k
}

func healthCheckPath(zone, record string) string {
	return fmt.Sprintf("/dns/zones/%s/records/%s/health-check", zone, record)
}

// addHealthCheckCmd wires `dns healthcheck`: probes that pull an A/AAAA
// record's value from DNS answers while its target is down.
func addHealthCheckCmd(parent *cobra.Command) {
	hcCmd := &cobra.Command{
		Use:     "healthcheck",
		Aliases: []string{"hc", "health-check"},
		Short:   "Manage DNS failover health checks (Pro and Business zones)",
		Long: `A health check probes the value of an A or AAAA record (or another target)
from every CDN location; while it fails, the record is left out of DNS answers.
Enabled health checks are billed.

Status is aggregated across locations: healthy, degraded (some locations
see it down), unhealthy (every location sees it down), unknown or paused.
Use "history" for uptime per location and past incidents.`,
	}

	listCmd := &cobra.Command{
		Use:   "list <zone_uuid>",
		Short: "List the health checks of a zone",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			s := output.NewSpinner("Fetching health checks...")
			s.Start()
			resp, err := client.Get(fmt.Sprintf("/dns/zones/%s/health-checks", args[0]))
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			var checks []healthCheck
			if err := json.Unmarshal(resp, &checks); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}
			t := output.NewTable("Health Checks", []string{"Record", "Name", "Probe", "Every", "Enabled", "Status", "Uptime 24h", "Last Check"})
			for _, h := range checks {
				t.AddRow(h.RecordUUID, h.Name, h.probe(), fmt.Sprintf("%ds", h.IntervalSecs), cmdutil.YesNo(h.Enabled), h.status(), formatPct(h.Uptime24h), h.LastCheckAt)
			}
			t.Render()
			return nil
		},
	}

	getCmd := &cobra.Command{
		Use:     "get <zone_uuid> <record_uuid>",
		Aliases: []string{"show"},
		Short:   "Show the health check of a record",
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			s := output.NewSpinner("Fetching health check...")
			s.Start()
			resp, err := client.Get(healthCheckPath(args[0], args[1]))
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}
			return renderHealthCheck("Health Check", resp)
		},
	}

	setCmd := &cobra.Command{
		Use:   "set <zone_uuid> <record_uuid>",
		Short: "Create or replace the health check of an A/AAAA record",
		Long: `Create or replace the health check of an A or AAAA record.
Every field is replaced: flags you leave out go back to their defaults.`,
		Example: `  cubecli dns healthcheck set <zone_uuid> <record_uuid> --name web --type https --path /health
  cubecli dns hc set <zone_uuid> <record_uuid> --name db --type tcp --port 5432 --interval 30
  cubecli dns hc set <zone_uuid> <record_uuid> --name web --type http --expected-status 204 --disabled`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			name, _ := cmd.Flags().GetString("name")
			checkType, _ := cmd.Flags().GetString("type")
			body := map[string]interface{}{
				"name":       name,
				"check_type": checkType,
			}
			if v, _ := cmd.Flags().GetString("target"); v != "" {
				body["target"] = v
			}
			if v, _ := cmd.Flags().GetString("path"); v != "" {
				body["path"] = v
			}
			ints := map[string]string{
				"port": "port", "expected-status": "expected_status", "interval": "interval_secs",
				"timeout": "timeout_secs", "healthy-threshold": "healthy_threshold", "unhealthy-threshold": "unhealthy_threshold",
			}
			for flag, field := range ints {
				if cmd.Flags().Changed(flag) {
					v, _ := cmd.Flags().GetInt(flag)
					body[field] = v
				}
			}
			disabled, _ := cmd.Flags().GetBool("disabled")
			body["enabled"] = !disabled

			s := output.NewSpinner("Saving health check...")
			s.Start()
			resp, err := client.Put(healthCheckPath(args[0], args[1]), body)
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}
			if err := renderHealthCheck("Health Check", resp); err != nil {
				return err
			}
			output.PrintSuccess("Health check saved")
			return nil
		},
	}
	setCmd.Flags().StringP("name", "n", "", "Name")
	setCmd.Flags().String("type", "", "Check type: http, https, tcp or ping")
	setCmd.Flags().String("target", "", "Hostname or IP to probe (default: the record's value)")
	setCmd.Flags().Int("port", 0, "Port (required for tcp)")
	setCmd.Flags().String("path", "", "HTTP path, e.g. /health (http/https)")
	setCmd.Flags().Int("expected-status", 200, "Expected HTTP status (http/https)")
	setCmd.Flags().Int("interval", 60, "Seconds between checks (10-3600)")
	setCmd.Flags().Int("timeout", 5, "Timeout in seconds (1-60, below the interval)")
	setCmd.Flags().Int("healthy-threshold", 2, "Successes before healthy (1-10)")
	setCmd.Flags().Int("unhealthy-threshold", 3, "Failures before unhealthy (1-10)")
	setCmd.Flags().Bool("disabled", false, "Save the check disabled (not billed, no failover)")
	_ = setCmd.MarkFlagRequired("name")
	_ = setCmd.MarkFlagRequired("type")

	deleteCmd := &cobra.Command{
		Use:   "delete <zone_uuid> <record_uuid>",
		Short: "Delete the health check of a record",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmdutil.CheckForce(cmd, "Delete this health check?") {
				output.PrintWarning("Aborted")
				return nil
			}
			client := cmdutil.GetClient(cmd)
			return cmdutil.RunDetail(cmd, "Deleting health check...", "Health check deleted", func() (json.RawMessage, error) {
				return client.Delete(healthCheckPath(args[0], args[1]))
			})
		},
	}
	deleteCmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")

	historyCmd := &cobra.Command{
		Use:   "history <zone_uuid> <record_uuid>",
		Short: "Show uptime per location and recent incidents of a health check",
		Long: `Show the uptime of a health check over a window: the overall status, one row
per CDN location (PoP) and the most recent incidents.

History is kept 30 days on Pro zones and 90 days on Business zones (at least
7 days while the zone has a health check); a longer --range is shortened to
the retention.`,
		Example: `  cubecli dns healthcheck history <zone_uuid> <record_uuid>
  cubecli dns hc history <zone_uuid> <record_uuid> --range 30d --incidents 50
  cubecli dns hc history <zone_uuid> <record_uuid> --range 7d --json`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			rng, _ := cmd.Flags().GetString("range")
			valid := false
			for _, r := range healthHistoryRanges {
				valid = valid || rng == r
			}
			if !valid {
				return fmt.Errorf("invalid --range %q: use one of %s", rng, strings.Join(healthHistoryRanges, ", "))
			}
			limit, _ := cmd.Flags().GetInt("incidents")
			client := cmdutil.GetClient(cmd)

			s := output.NewSpinner("Fetching health check history...")
			s.Start()
			resp, err := client.Get(healthCheckPath(args[0], args[1]) + "/history?time_range=" + url.QueryEscape(rng))
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}
			return renderHealthHistory(resp, limit)
		},
	}
	historyCmd.Flags().StringP("range", "r", "24h", "Window: 24h, 7d, 30d or 90d")
	historyCmd.Flags().Int("incidents", 10, "Maximum incidents to list (0 for all returned)")

	hcCmd.AddCommand(listCmd, getCmd, setCmd, deleteCmd, historyCmd)
	parent.AddCommand(hcCmd)
}

func renderHealthCheck(title string, resp json.RawMessage) error {
	var h healthCheck
	if err := json.Unmarshal(resp, &h); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}
	t := output.NewTable(title, []string{"Field", "Value"})
	t.AddRow("UUID", h.UUID)
	t.AddRow("Record", h.RecordUUID)
	t.AddRow("Name", h.Name)
	t.AddRow("Probe", h.probe())
	if h.ExpectedStatus != nil && (h.CheckType == "http" || h.CheckType == "https") {
		t.AddRow("Expected Status", strconv.Itoa(*h.ExpectedStatus))
	}
	t.AddRow("Interval", fmt.Sprintf("%ds (timeout %ds)", h.IntervalSecs, h.TimeoutSecs))
	t.AddRow("Thresholds", fmt.Sprintf("%d up, %d down", h.HealthyThreshold, h.UnhealthyThreshold))
	t.AddRow("Enabled", cmdutil.YesNo(h.Enabled))
	t.AddRow("Status", h.status())
	if s := h.StatusSummary; s != nil {
		if len(s.PoPs) > 0 {
			pops := make([]string, 0, len(s.PoPs))
			for _, p := range s.PoPs {
				pops = append(pops, p.PoP+" "+output.FormatStatus(p.Status))
			}
			t.AddRow("Locations", strings.Join(pops, ", "))
		}
		if s.LastChangeAt != nil {
			t.AddRow("Last Change", *s.LastChangeAt)
		}
	}
	if h.Uptime24h != nil {
		t.AddRow("Uptime 24h", formatPct(h.Uptime24h))
	}
	if h.LastCheckAt != "" {
		t.AddRow("Last Check", h.LastCheckAt)
	}
	t.Render()
	return nil
}

func renderHealthHistory(resp json.RawMessage, incidentLimit int) error {
	var h healthHistory
	if err := json.Unmarshal(resp, &h); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}
	o := h.Overall

	t := output.NewTable(fmt.Sprintf("Health Check History (%s)", h.TimeRange), []string{"Field", "Value"})
	status := output.FormatStatus(o.Status)
	if o.NodesReporting > 0 {
		status += fmt.Sprintf(" (%d/%d nodes down)", o.NodesUnhealthy, o.NodesReporting)
	}
	t.AddRow("Status", status)
	t.AddRow("Uptime", formatPct(o.UptimePct))
	t.AddRow("Coverage", formatPct(o.CoveragePct))
	t.AddRow("Full Outage", formatSecs(o.OutageSecs))
	t.AddRow("Last Change", strOr(o.LastChangeAt, "-"))
	t.AddRow("Window", fmt.Sprintf("%s to %s", h.Start, h.End))
	if h.HistoryStartsAt != "" && h.HistoryStartsAt != h.Start {
		t.AddRow("Data Since", h.HistoryStartsAt)
	}
	retention := fmt.Sprintf("%d days", h.RetentionDays)
	if h.Clamped {
		retention += " (range shortened to the retention)"
	}
	t.AddRow("Retention", retention)
	t.Render()

	if len(h.PoPs) > 0 {
		pt := output.NewTable("Locations", []string{"PoP", "Region", "Status", "Down", "Uptime", "Coverage", "Last Error", "Last Change"})
		for _, p := range h.PoPs {
			pt.AddRow(p.PoP, p.Region, output.FormatStatus(p.Status),
				fmt.Sprintf("%d/%d", p.NodesUnhealthy, p.NodesTotal),
				formatPct(p.UptimePct), formatPct(p.CoveragePct),
				lastError(p.LastErrorKind, p.LastHTTPStatus), strOr(p.LastChangeAt, "-"))
		}
		pt.Render()
	}

	if len(h.Incidents) == 0 {
		output.PrintInfo("No incidents in this window")
		return nil
	}
	incidents := h.Incidents
	title := "Recent Incidents"
	if incidentLimit > 0 && len(incidents) > incidentLimit {
		incidents = incidents[:incidentLimit]
		title = fmt.Sprintf("Recent Incidents (%d of %d)", incidentLimit, len(h.Incidents))
	}
	it := output.NewTable(title, []string{"Location", "Started", "Resolved", "Duration", "Error"})
	for _, i := range incidents {
		resolved, duration := "ongoing", "-"
		if i.ResolvedAt != nil {
			resolved = *i.ResolvedAt
		}
		if i.DurationSecs != nil {
			duration = formatSecs(*i.DurationSecs)
		}
		it.AddRow(fmt.Sprintf("%s #%d", i.PoP, i.NodeIndex), i.StartedAt, resolved, duration, lastError(i.ErrorKind, i.HTTPStatus))
	}
	it.Render()
	return nil
}
