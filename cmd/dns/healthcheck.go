package dns

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/CubePathInc/cubecli/internal/cmdutil"
	"github.com/CubePathInc/cubecli/internal/output"
	"github.com/spf13/cobra"
)

type healthCheck struct {
	UUID               string `json:"uuid"`
	RecordUUID         string `json:"record_uuid"`
	Name               string `json:"name"`
	CheckType          string `json:"check_type"`
	Target             string `json:"target"`
	Port               *int   `json:"port"`
	Path               string `json:"path"`
	ExpectedStatus     *int   `json:"expected_status"`
	IntervalSecs       int    `json:"interval_secs"`
	TimeoutSecs        int    `json:"timeout_secs"`
	HealthyThreshold   int    `json:"healthy_threshold"`
	UnhealthyThreshold int    `json:"unhealthy_threshold"`
	Enabled            bool   `json:"enabled"`
	LastStatus         string `json:"last_status"`
	LastCheckAt        string `json:"last_check_at"`
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

// addHealthCheckCmd wires `dns health-check`: probes that pull an A/AAAA
// record's value from DNS answers while its target is down.
func addHealthCheckCmd(parent *cobra.Command) {
	hcCmd := &cobra.Command{
		Use:   "health-check",
		Short: "Manage DNS failover health checks (Pro and Business zones)",
		Long: `A health check probes the value of an A or AAAA record (or another target)
from every location; while it fails, the record is left out of DNS answers.
Enabled health checks are billed.`,
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
			t := output.NewTable("Health Checks", []string{"Record", "Name", "Probe", "Every", "Enabled", "Status", "Last Check"})
			for _, h := range checks {
				t.AddRow(h.RecordUUID, h.Name, h.probe(), fmt.Sprintf("%ds", h.IntervalSecs), cmdutil.YesNo(h.Enabled), output.FormatStatus(h.LastStatus), h.LastCheckAt)
			}
			t.Render()
			return nil
		},
	}

	showCmd := &cobra.Command{
		Use:   "show <zone_uuid> <record_uuid>",
		Short: "Show the health check of a record",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			s := output.NewSpinner("Fetching health check...")
			s.Start()
			resp, err := client.Get(fmt.Sprintf("/dns/zones/%s/records/%s/health-check", args[0], args[1]))
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
		Example: `  cubecli dns health-check set <zone_uuid> <record_uuid> --name web --type https --path /health
  cubecli dns health-check set <zone_uuid> <record_uuid> --name db --type tcp --port 5432 --interval 30`,
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
			resp, err := client.Put(fmt.Sprintf("/dns/zones/%s/records/%s/health-check", args[0], args[1]), body)
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
				return client.Delete(fmt.Sprintf("/dns/zones/%s/records/%s/health-check", args[0], args[1]))
			})
		},
	}
	deleteCmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")

	hcCmd.AddCommand(listCmd, showCmd, setCmd, deleteCmd)
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
	t.AddRow("Status", output.FormatStatus(h.LastStatus))
	if h.LastCheckAt != "" {
		t.AddRow("Last Check", h.LastCheckAt)
	}
	t.Render()
	return nil
}
