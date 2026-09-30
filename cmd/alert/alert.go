package alert

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

func NewCmd() *cobra.Command {
	alertCmd := &cobra.Command{
		Use:     "alert",
		Aliases: []string{"alerts", "cloud-alert", "trigger"},
		Short:   "Manage cloud alerts and their notification channels",
		Long: `Cloud alerts watch a metric of a VPS, a baremetal server or an availability
group and notify a channel (Slack, Discord or email) when it stays past a
threshold. VPS and availability groups support cpu, ram, disk, network_in and
network_out; baremetal supports network_in and network_out.

Create a channel first with 'alert notificator create', then pass its id to
'alert create --notify'.`,
	}

	alertCmd.AddCommand(
		listCmd(),
		showCmd(),
		createCmd(),
		updateCmd(),
		deleteCmd(),
		historyCmd(),
		notificatorCmd(),
	)
	return alertCmd
}

// operators accepts the API names and the usual symbols.
var operators = map[string]string{
	"gt": "gt", ">": "gt",
	"lt": "lt", "<": "lt",
	"gte": "gte", ">=": "gte",
	"lte": "lte", "<=": "lte",
	"eq": "eq", "=": "eq", "==": "eq",
}

var operatorSymbols = map[string]string{"gt": ">", "lt": "<", "gte": ">=", "lte": "<=", "eq": "="}

func normalizeOperator(op string) (string, error) {
	if v, ok := operators[strings.ToLower(strings.TrimSpace(op))]; ok {
		return v, nil
	}
	return "", fmt.Errorf("invalid operator %q: use gt, lt, gte, lte or eq", op)
}

func normalizeTargetType(t string) string {
	return strings.ReplaceAll(strings.ToLower(strings.TrimSpace(t)), "-", "_")
}

func notifyActions(ids []string) []map[string]interface{} {
	actions := make([]map[string]interface{}, 0, len(ids))
	for i, id := range ids {
		actions = append(actions, map[string]interface{}{
			"action_type":    "notify",
			"notificator_id": id,
			"order":          i,
		})
	}
	return actions
}

// alertOut is the shape of GET /triggers/{id}.
type alertOut struct {
	ID              string  `json:"id"`
	ProjectID       int     `json:"project_id"`
	Name            string  `json:"name"`
	Description     string  `json:"description"`
	TargetType      string  `json:"target_type"`
	TargetID        string  `json:"target_id"`
	MetricType      string  `json:"metric_type"`
	Operator        string  `json:"operator"`
	Threshold       float64 `json:"threshold"`
	DurationSeconds int     `json:"duration_seconds"`
	CooldownSeconds int     `json:"cooldown_seconds"`
	Status          string  `json:"status"`
	ActionsCount    int     `json:"actions_count"`
	LastTriggeredAt string  `json:"last_triggered_at"`
	LastResolvedAt  string  `json:"last_resolved_at"`
	CreatedAt       string  `json:"created_at"`
	Actions         []struct {
		ID            string `json:"id"`
		ActionType    string `json:"action_type"`
		NotificatorID string `json:"notificator_id"`
		Enabled       bool   `json:"enabled"`
	} `json:"actions"`
}

func (a alertOut) condition() string {
	sym := operatorSymbols[a.Operator]
	if sym == "" {
		sym = a.Operator
	}
	return fmt.Sprintf("%s %s %g", a.MetricType, sym, a.Threshold)
}

// --- list ---

func listCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List cloud alerts",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			q := url.Values{}
			if p, _ := cmd.Flags().GetInt("project"); p > 0 {
				q.Set("project_id", strconv.Itoa(p))
			}
			if st, _ := cmd.Flags().GetString("status"); st != "" {
				q.Set("status", st)
			}
			path := "/triggers/"
			if len(q) > 0 {
				path += "?" + q.Encode()
			}

			s := output.NewSpinner("Fetching alerts...")
			s.Start()
			resp, err := client.Get(path)
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			var alerts []alertOut
			if err := json.Unmarshal(resp, &alerts); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}
			t := output.NewTable("Cloud Alerts", []string{"ID", "Name", "Target", "Condition", "Status", "Actions", "Project"})
			for _, a := range alerts {
				t.AddRow(a.ID, a.Name, a.TargetType+" "+a.TargetID, a.condition(), output.FormatStatus(a.Status), strconv.Itoa(a.ActionsCount), strconv.Itoa(a.ProjectID))
			}
			t.Render()
			return nil
		},
	}
	cmd.Flags().IntP("project", "p", 0, "Filter by project ID")
	cmd.Flags().String("status", "", "Filter by status: enabled, disabled, triggered, resolved")
	return cmd
}

// --- show ---

func showCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <alert_id>",
		Short: "Show a cloud alert and its actions",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			s := output.NewSpinner("Fetching alert...")
			s.Start()
			resp, err := client.Get("/triggers/" + args[0])
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}
			return renderAlert("Cloud Alert", resp)
		},
	}
}

func renderAlert(title string, resp json.RawMessage) error {
	var a alertOut
	if err := json.Unmarshal(resp, &a); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}
	t := output.NewTable(title, []string{"Field", "Value"})
	t.AddRow("ID", a.ID)
	t.AddRow("Name", a.Name)
	if a.Description != "" {
		t.AddRow("Description", a.Description)
	}
	t.AddRow("Project", strconv.Itoa(a.ProjectID))
	t.AddRow("Target", a.TargetType+" "+a.TargetID)
	t.AddRow("Condition", a.condition())
	t.AddRow("For", fmt.Sprintf("%ds", a.DurationSeconds))
	t.AddRow("Cooldown", fmt.Sprintf("%ds", a.CooldownSeconds))
	t.AddRow("Status", output.FormatStatus(a.Status))
	if a.LastTriggeredAt != "" {
		t.AddRow("Last triggered", a.LastTriggeredAt)
	}
	if a.LastResolvedAt != "" {
		t.AddRow("Last resolved", a.LastResolvedAt)
	}
	for _, act := range a.Actions {
		state := ""
		if !act.Enabled {
			state = " (disabled)"
		}
		t.AddRow("Action", fmt.Sprintf("%s %s%s", act.ActionType, act.NotificatorID, state))
	}
	t.Render()
	return nil
}

// --- create ---

func createCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a cloud alert",
		Example: `  cubecli alert create --project 12 --name high-cpu --target-type vps --target 345 \
    --metric cpu --operator gt --threshold 90 --duration 300 --notify <notificator_id>`,
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			projectID, _ := cmd.Flags().GetInt("project")
			name, _ := cmd.Flags().GetString("name")
			targetType, _ := cmd.Flags().GetString("target-type")
			target, _ := cmd.Flags().GetString("target")
			metric, _ := cmd.Flags().GetString("metric")
			opStr, _ := cmd.Flags().GetString("operator")
			threshold, _ := cmd.Flags().GetFloat64("threshold")
			notify, _ := cmd.Flags().GetStringSlice("notify")

			op, err := normalizeOperator(opStr)
			if err != nil {
				return err
			}

			body := map[string]interface{}{
				"project_id":  projectID,
				"name":        name,
				"target_type": normalizeTargetType(targetType),
				"target_id":   target,
				"metric_type": strings.ToLower(metric),
				"operator":    op,
				"threshold":   threshold,
				"actions":     notifyActions(notify),
			}
			if d, _ := cmd.Flags().GetString("description"); d != "" {
				body["description"] = d
			}
			if cmd.Flags().Changed("duration") {
				v, _ := cmd.Flags().GetInt("duration")
				body["duration_seconds"] = v
			}
			if cmd.Flags().Changed("cooldown") {
				v, _ := cmd.Flags().GetInt("cooldown")
				body["cooldown_seconds"] = v
			}

			s := output.NewSpinner("Creating alert...")
			s.Start()
			resp, err := client.Post("/triggers/", body)
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}
			if err := renderAlert("Cloud Alert Created", resp); err != nil {
				return err
			}
			output.PrintSuccess("Alert created successfully")
			return nil
		},
	}
	cmd.Flags().IntP("project", "p", 0, "Project ID")
	cmd.Flags().StringP("name", "n", "", "Alert name")
	cmd.Flags().String("description", "", "Description")
	cmd.Flags().String("target-type", "vps", "Target type: vps, baremetal or availability-group")
	cmd.Flags().String("target", "", "Target: VPS or baremetal id, or availability group UUID")
	cmd.Flags().String("metric", "", "Metric: cpu, ram, disk, network_in, network_out")
	cmd.Flags().String("operator", "gt", "Operator: gt, lt, gte, lte, eq")
	cmd.Flags().Float64("threshold", 0, "Threshold value")
	cmd.Flags().Int("duration", 300, "Seconds the condition must hold before firing (60-3600)")
	cmd.Flags().Int("cooldown", 600, "Seconds between repeated notifications (60-86400)")
	cmd.Flags().StringSlice("notify", nil, "Notificator id to notify (repeatable, 1-10)")
	_ = cmd.MarkFlagRequired("project")
	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("target")
	_ = cmd.MarkFlagRequired("metric")
	_ = cmd.MarkFlagRequired("threshold")
	_ = cmd.MarkFlagRequired("notify")
	return cmd
}

// --- update ---

func updateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update <alert_id>",
		Short: "Update a cloud alert (--notify replaces all its actions)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			body := map[string]interface{}{}
			str := map[string]string{"name": "name", "description": "description", "target": "target_id", "metric": "metric_type", "status": "status"}
			for flag, field := range str {
				if cmd.Flags().Changed(flag) {
					v, _ := cmd.Flags().GetString(flag)
					body[field] = v
				}
			}
			if cmd.Flags().Changed("target-type") {
				v, _ := cmd.Flags().GetString("target-type")
				body["target_type"] = normalizeTargetType(v)
			}
			if cmd.Flags().Changed("operator") {
				v, _ := cmd.Flags().GetString("operator")
				op, err := normalizeOperator(v)
				if err != nil {
					return err
				}
				body["operator"] = op
			}
			if cmd.Flags().Changed("threshold") {
				v, _ := cmd.Flags().GetFloat64("threshold")
				body["threshold"] = v
			}
			if cmd.Flags().Changed("duration") {
				v, _ := cmd.Flags().GetInt("duration")
				body["duration_seconds"] = v
			}
			if cmd.Flags().Changed("cooldown") {
				v, _ := cmd.Flags().GetInt("cooldown")
				body["cooldown_seconds"] = v
			}
			if cmd.Flags().Changed("notify") {
				v, _ := cmd.Flags().GetStringSlice("notify")
				body["actions"] = notifyActions(v)
			}
			if len(body) == 0 {
				return fmt.Errorf("nothing to update; see --help for the available flags")
			}

			s := output.NewSpinner("Updating alert...")
			s.Start()
			resp, err := client.Put("/triggers/"+args[0], body)
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}
			output.PrintSuccess("Alert updated successfully")
			return nil
		},
	}
	cmd.Flags().StringP("name", "n", "", "New name")
	cmd.Flags().String("description", "", "New description")
	cmd.Flags().String("target-type", "", "New target type: vps, baremetal or availability-group")
	cmd.Flags().String("target", "", "New target")
	cmd.Flags().String("metric", "", "New metric")
	cmd.Flags().String("operator", "", "New operator")
	cmd.Flags().Float64("threshold", 0, "New threshold")
	cmd.Flags().Int("duration", 0, "New duration in seconds")
	cmd.Flags().Int("cooldown", 0, "New cooldown in seconds")
	cmd.Flags().String("status", "", "enabled or disabled")
	cmd.Flags().StringSlice("notify", nil, "Notificator ids; replaces the current actions")
	return cmd
}

// --- delete ---

func deleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <alert_id>",
		Short: "Delete a cloud alert",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmdutil.CheckForce(cmd, "Delete this alert and its history?") {
				output.PrintWarning("Aborted")
				return nil
			}

			client := cmdutil.GetClient(cmd)
			s := output.NewSpinner("Deleting alert...")
			s.Start()
			resp, err := client.Delete("/triggers/" + args[0])
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}
			output.PrintSuccess("Alert deleted successfully")
			return nil
		},
	}
	cmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")
	return cmd
}

// --- history ---

func historyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "history <alert_id>",
		Short: "Show when an alert fired and recovered (newest first)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			limit, _ := cmd.Flags().GetInt("limit")

			s := output.NewSpinner("Fetching alert history...")
			s.Start()
			resp, err := client.Get(fmt.Sprintf("/triggers/%s/history?limit=%d", args[0], limit))
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			var events []struct {
				EventType   string   `json:"event_type"`
				MetricValue *float64 `json:"metric_value"`
				CreatedAt   string   `json:"created_at"`
			}
			if err := json.Unmarshal(resp, &events); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}
			t := output.NewTable("Alert History", []string{"Time", "Event", "Value"})
			for _, e := range events {
				v := "-"
				if e.MetricValue != nil {
					v = fmt.Sprintf("%g", *e.MetricValue)
				}
				t.AddRow(e.CreatedAt, e.EventType, v)
			}
			t.Render()
			return nil
		},
	}
	cmd.Flags().Int("limit", 50, "Maximum events (1-200)")
	return cmd
}
