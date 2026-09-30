package manageddatabase

import (
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/CubePathInc/cubecli/internal/cmdutil"
	"github.com/CubePathInc/cubecli/internal/output"
	"github.com/spf13/cobra"
)

func NewCmd() *cobra.Command {
	mdbCmd := &cobra.Command{
		Use:     "managed-database",
		Aliases: []string{"mdb", "dbaas"},
		Short:   "Manage managed databases (MySQL, PostgreSQL, Valkey)",
		Long: `Manage CubePath Managed Databases: highly available MySQL, PostgreSQL and
Valkey clusters. Each replica runs on its own node; plans are sized and priced
per node.

Provisioning and every change (scale, configuration, credential rotation) run
in the background: check progress with 'managed-database show'.`,
	}

	mdbCmd.AddCommand(
		plansCmd(),
		listCmd(),
		showCmd(),
		createCmd(),
		updateCmd(),
		deleteCmd(),
		cmdutil.ProtectionCmd("md_uuid", "Managed database", cmdutil.StringPath("/managed-databases/%s/protection")),
		scaleCmd(),
		credentialsCmd(),
		rotateCredentialsCmd(),
		configCmd(),
		metricsCmd(),
		databaseCmd(),
		userCmd(),
	)

	return mdbCmd
}

// --- plans ---

func plansCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plans",
		Short: "List managed database plans (prices are per node per hour)",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			path := "/managed-database-plans/"
			if engine, _ := cmd.Flags().GetString("engine"); engine != "" {
				path += "?engine=" + url.QueryEscape(engine)
			}

			s := output.NewSpinner("Fetching managed database plans...")
			s.Start()
			resp, err := client.Get(path)
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			var locations []struct {
				LocationName string `json:"location_name"`
				Plans        []struct {
					UUID         string  `json:"uuid"`
					Name         string  `json:"name"`
					Engine       string  `json:"engine"`
					CPU          int     `json:"cpu"`
					MemoryGB     float64 `json:"memory_gb"`
					StorageGB    int     `json:"storage_gb"`
					MaxReplicas  int     `json:"max_replicas"`
					PricePerHour float64 `json:"price_per_hour"`
				} `json:"plans"`
			}
			if err := json.Unmarshal(resp, &locations); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}

			t := output.NewTable("Managed Database Plans", []string{"Location", "Plan", "UUID", "Engine", "vCPU", "RAM (GB)", "Storage (GB)", "Max Replicas", "Price/Node/Hour"})
			for _, loc := range locations {
				for _, p := range loc.Plans {
					t.AddRow(
						loc.LocationName,
						p.Name,
						p.UUID,
						p.Engine,
						strconv.Itoa(p.CPU),
						fmt.Sprintf("%g", p.MemoryGB),
						strconv.Itoa(p.StorageGB),
						strconv.Itoa(p.MaxReplicas),
						fmt.Sprintf("$%.5f", p.PricePerHour),
					)
				}
			}
			t.Render()
			return nil
		},
	}
	cmd.Flags().StringP("engine", "e", "", "Filter by engine (mysql, postgresql, valkey)")
	return cmd
}

// summary is one entry of GET /managed-databases/.
type summary struct {
	UUID         string `json:"uuid"`
	ProjectID    int    `json:"project_id"`
	Name         string `json:"name"`
	Label        string `json:"label"`
	Engine       string `json:"engine"`
	Version      string `json:"version"`
	Status       string `json:"status"`
	EndpointHost string `json:"endpoint_host"`
	EndpointPort *int   `json:"endpoint_port"`
	Replicas     int    `json:"replicas"`
	Protected    bool   `json:"protected"`
}

func (s summary) endpoint() string {
	if s.EndpointHost == "" || s.EndpointPort == nil {
		return "-"
	}
	return fmt.Sprintf("%s:%d", s.EndpointHost, *s.EndpointPort)
}

// --- list ---

func listCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List managed databases",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			projectID, _ := cmd.Flags().GetInt("project")

			s := output.NewSpinner("Fetching managed databases...")
			s.Start()
			resp, err := client.Get("/managed-databases/")
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			var mdbs []summary
			if err := json.Unmarshal(resp, &mdbs); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}

			t := output.NewTable("Managed Databases", []string{"UUID", "Name", "Engine", "Status", "Replicas", "Endpoint", "Project", "Protected"})
			for _, m := range mdbs {
				if projectID > 0 && m.ProjectID != projectID {
					continue
				}
				t.AddRow(
					m.UUID,
					m.Name,
					m.Engine+" "+m.Version,
					output.FormatStatus(m.Status),
					strconv.Itoa(m.Replicas),
					m.endpoint(),
					strconv.Itoa(m.ProjectID),
					cmdutil.YesNo(m.Protected),
				)
			}
			t.Render()
			return nil
		},
	}
	cmd.Flags().IntP("project", "p", 0, "Filter by project ID")
	return cmd
}

// --- show ---

func showCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <md_uuid>",
		Short: "Show managed database details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			s := output.NewSpinner("Fetching managed database...")
			s.Start()
			resp, err := client.Get("/managed-databases/" + args[0])
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			var md struct {
				summary
				Topology string `json:"topology"`
				Plan     struct {
					Name         string  `json:"name"`
					CPU          int     `json:"cpu"`
					MemoryGB     float64 `json:"memory_gb"`
					StorageGB    int     `json:"storage_gb"`
					PricePerHour float64 `json:"price_per_hour"`
				} `json:"plan"`
				Location struct {
					LocationName string `json:"location_name"`
				} `json:"location"`
				BackupEnabled       bool   `json:"backup_enabled"`
				BackupScheduleCron  string `json:"backup_schedule_cron"`
				BackupRetentionDays int    `json:"backup_retention_days"`
				BillingType         string `json:"billing_type"`
				UpdatedAt           string `json:"updated_at"`
			}
			if err := json.Unmarshal(resp, &md); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}

			info := output.NewTable("Managed Database Info", []string{"Field", "Value"})
			info.AddRow("UUID", md.UUID)
			info.AddRow("Name", md.Name)
			if md.Label != "" {
				info.AddRow("Label", md.Label)
			}
			info.AddRow("Engine", md.Engine+" "+md.Version)
			info.AddRow("Topology", md.Topology)
			info.AddRow("Status", output.FormatStatus(md.Status))
			info.AddRow("Replicas", strconv.Itoa(md.Replicas))
			info.AddRow("Plan", fmt.Sprintf("%s (%d vCPU, %g GB RAM, %d GB storage per node)", md.Plan.Name, md.Plan.CPU, md.Plan.MemoryGB, md.Plan.StorageGB))
			info.AddRow("Price/Hour", fmt.Sprintf("$%.5f per node, $%.5f total", md.Plan.PricePerHour, md.Plan.PricePerHour*float64(md.Replicas)))
			info.AddRow("Location", md.Location.LocationName)
			info.AddRow("Endpoint", md.endpoint())
			info.AddRow("Project", strconv.Itoa(md.ProjectID))
			info.AddRow("Billing", md.BillingType)
			backup := "disabled"
			if md.BackupEnabled {
				backup = fmt.Sprintf("%s, kept %d days", md.BackupScheduleCron, md.BackupRetentionDays)
			}
			info.AddRow("Backups", backup)
			info.AddRow("Protected", cmdutil.YesNo(md.Protected))
			info.AddRow("Updated", md.UpdatedAt)
			info.Render()
			return nil
		},
	}
}

// --- create ---

func createCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a managed database",
		Long: `Create a managed database. The plan decides the location and the size of
each node; see 'managed-database plans'. Provisioning takes several minutes.`,
		Example: `  cubecli mdb create --name app-db --engine mysql --version 8.0.39 --plan <plan_uuid> --project 12
  cubecli mdb create --name cache --engine valkey --version 7.2.11 --plan <plan_uuid> --project 12 --replicas 2`,
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			name, _ := cmd.Flags().GetString("name")
			engine, _ := cmd.Flags().GetString("engine")
			version, _ := cmd.Flags().GetString("version")
			plan, _ := cmd.Flags().GetString("plan")
			projectID, _ := cmd.Flags().GetInt("project")

			body := map[string]interface{}{
				"project_id": projectID,
				"name":       name,
				"engine":     engine,
				"version":    version,
				"plan_uuid":  plan,
			}
			if cmd.Flags().Changed("replicas") {
				replicas, _ := cmd.Flags().GetInt("replicas")
				body["replicas"] = replicas
			}
			if topology, _ := cmd.Flags().GetString("topology"); topology != "" {
				body["topology"] = topology
			}
			if cron, _ := cmd.Flags().GetString("backup-schedule"); cron != "" {
				backup := map[string]interface{}{"schedule_cron": cron}
				if cmd.Flags().Changed("backup-retention") {
					days, _ := cmd.Flags().GetInt("backup-retention")
					backup["retention_days"] = days
				}
				body["backup"] = backup
			}

			s := output.NewSpinner("Creating managed database...")
			s.Start()
			resp, err := client.Post("/managed-databases/", body)
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			var result struct {
				UUID   string `json:"uuid"`
				Status string `json:"status"`
			}
			if err := json.Unmarshal(resp, &result); err == nil && result.UUID != "" {
				output.PrintSuccess(fmt.Sprintf("Managed database %s created (%s)", result.UUID, result.Status))
			} else {
				output.PrintSuccess("Managed database creation initiated")
			}
			return nil
		},
	}
	cmd.Flags().StringP("name", "n", "", "Name (lowercase letters, digits and hyphens)")
	cmd.Flags().StringP("engine", "e", "", "Engine: mysql, postgresql or valkey")
	cmd.Flags().String("version", "", "Engine version (e.g. 8.0.39, 17.5.0, 7.2.11)")
	cmd.Flags().String("plan", "", "Plan UUID (see 'managed-database plans')")
	cmd.Flags().IntP("project", "p", 0, "Project ID")
	cmd.Flags().Int("replicas", 3, "Number of replicas (one node each)")
	cmd.Flags().String("topology", "", "Topology (defaults per engine)")
	cmd.Flags().String("backup-schedule", "", "Backup schedule as a 5-field cron expression")
	cmd.Flags().Int("backup-retention", 7, "Backup retention in days (with --backup-schedule)")
	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("engine")
	_ = cmd.MarkFlagRequired("version")
	_ = cmd.MarkFlagRequired("plan")
	_ = cmd.MarkFlagRequired("project")
	return cmd
}

// --- update ---

func updateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update <md_uuid>",
		Short: "Update a managed database's name, label or backup policy",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			body := map[string]interface{}{}
			if cmd.Flags().Changed("name") {
				v, _ := cmd.Flags().GetString("name")
				body["name"] = v
			}
			if cmd.Flags().Changed("label") {
				v, _ := cmd.Flags().GetString("label")
				body["label"] = v
			}
			backup := map[string]interface{}{}
			if cmd.Flags().Changed("backup-enabled") {
				v, _ := cmd.Flags().GetBool("backup-enabled")
				backup["enabled"] = v
			}
			if cmd.Flags().Changed("backup-schedule") {
				v, _ := cmd.Flags().GetString("backup-schedule")
				backup["schedule_cron"] = v
			}
			if cmd.Flags().Changed("backup-retention") {
				v, _ := cmd.Flags().GetInt("backup-retention")
				backup["retention_days"] = v
			}
			if len(backup) > 0 {
				body["backup"] = backup
			}
			if len(body) == 0 {
				return fmt.Errorf("at least one of --name, --label, --backup-enabled, --backup-schedule or --backup-retention must be specified")
			}

			s := output.NewSpinner("Updating managed database...")
			s.Start()
			resp, err := client.Patch("/managed-databases/"+args[0], body)
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}
			output.PrintSuccess("Managed database updated successfully")
			return nil
		},
	}
	cmd.Flags().StringP("name", "n", "", "New name")
	cmd.Flags().String("label", "", "New label")
	cmd.Flags().Bool("backup-enabled", false, "Enable or disable backups (--backup-enabled=false to disable)")
	cmd.Flags().String("backup-schedule", "", "Backup schedule as a 5-field cron expression")
	cmd.Flags().Int("backup-retention", 0, "Backup retention in days")
	return cmd
}

// --- delete ---

func deleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <md_uuid>",
		Short: "Delete a managed database and all its data",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmdutil.CheckForce(cmd, fmt.Sprintf("Delete managed database %s? All its data will be lost.", args[0])) {
				output.PrintWarning("Aborted")
				return nil
			}

			client := cmdutil.GetClient(cmd)
			s := output.NewSpinner("Deleting managed database...")
			s.Start()
			resp, err := client.Delete("/managed-databases/" + args[0])
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}
			output.PrintSuccess("Managed database deletion initiated")
			return nil
		},
	}
	cmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")
	return cmd
}

// --- scale ---

func scaleCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "scale <md_uuid>",
		Short: "Change the number of replicas or move to another plan",
		Args:  cobra.ExactArgs(1),
		Example: `  cubecli mdb scale <md_uuid> --replicas 5
  cubecli mdb scale <md_uuid> --plan <plan_uuid>`,
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			body := map[string]interface{}{}
			if cmd.Flags().Changed("replicas") {
				v, _ := cmd.Flags().GetInt("replicas")
				body["replicas"] = v
			}
			if plan, _ := cmd.Flags().GetString("plan"); plan != "" {
				body["plan_uuid"] = plan
			}
			if len(body) != 1 {
				return fmt.Errorf("exactly one of --replicas or --plan is required")
			}

			s := output.NewSpinner("Scaling managed database...")
			s.Start()
			resp, err := client.Post(fmt.Sprintf("/managed-databases/%s/scale", args[0]), body)
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}
			output.PrintSuccess("Managed database scaling initiated")
			return nil
		},
	}
	cmd.Flags().Int("replicas", 0, "New number of replicas")
	cmd.Flags().String("plan", "", "UUID of the new plan (same engine and location)")
	return cmd
}

// --- credentials ---

func credentialsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "credentials <md_uuid>",
		Short: "Show the admin connection credentials",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			s := output.NewSpinner("Fetching credentials...")
			s.Start()
			resp, err := client.Get(fmt.Sprintf("/managed-databases/%s/credentials", args[0]))
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			var c struct {
				Host     string `json:"host"`
				Port     int    `json:"port"`
				Username string `json:"username"`
				Password string `json:"password"`
				URI      string `json:"uri"`
			}
			if err := json.Unmarshal(resp, &c); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}

			t := output.NewTable("Connection Credentials", []string{"Field", "Value"})
			t.AddRow("Host", c.Host)
			t.AddRow("Port", strconv.Itoa(c.Port))
			t.AddRow("Username", c.Username)
			t.AddRow("Password", c.Password)
			t.AddRow("URI", c.URI)
			t.Render()
			return nil
		},
	}
}

func rotateCredentialsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rotate-credentials <md_uuid>",
		Short: "Rotate the admin password",
		Long: `Rotate the admin password. Applications using the old password stop working
once the rotation completes; read the new one with 'managed-database credentials'.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmdutil.CheckForce(cmd, "Rotate the admin password? Applications using it must be updated.") {
				output.PrintWarning("Aborted")
				return nil
			}

			client := cmdutil.GetClient(cmd)
			s := output.NewSpinner("Rotating credentials...")
			s.Start()
			resp, err := client.Post(fmt.Sprintf("/managed-databases/%s/credentials/rotate", args[0]), nil)
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}
			output.PrintSuccess("Credentials rotation initiated")
			return nil
		},
	}
	cmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")
	return cmd
}

// --- config ---

func configCmd() *cobra.Command {
	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Show or change engine configuration parameters",
	}

	showCmd := &cobra.Command{
		Use:   "show <md_uuid>",
		Short: "List the tunable parameters with their bounds",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			s := output.NewSpinner("Fetching configuration...")
			s.Start()
			resp, err := client.Get(fmt.Sprintf("/managed-databases/%s/config", args[0]))
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			var cfg struct {
				Engine string `json:"engine"`
				Params map[string]struct {
					Type            string        `json:"type"`
					Value           interface{}   `json:"value"`
					RequiresRestart bool          `json:"requires_restart"`
					Min             interface{}   `json:"min"`
					Max             interface{}   `json:"max"`
					Enum            []interface{} `json:"enum"`
					Description     string        `json:"description"`
				} `json:"params"`
				Note string `json:"note"`
			}
			if err := json.Unmarshal(resp, &cfg); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}

			names := make([]string, 0, len(cfg.Params))
			for n := range cfg.Params {
				names = append(names, n)
			}
			sort.Strings(names)

			t := output.NewTable(fmt.Sprintf("Configuration (%s)", cfg.Engine), []string{"Parameter", "Type", "Value", "Allowed", "Restart", "Description"})
			for _, n := range names {
				p := cfg.Params[n]
				allowed := ""
				switch {
				case len(p.Enum) > 0:
					vals := make([]string, len(p.Enum))
					for i, v := range p.Enum {
						vals[i] = fmt.Sprint(v)
					}
					allowed = strings.Join(vals, ", ")
				case p.Min != nil || p.Max != nil:
					allowed = fmt.Sprintf("%v - %v", valueOrDash(p.Min), valueOrDash(p.Max))
				}
				t.AddRow(n, p.Type, fmt.Sprint(p.Value), allowed, cmdutil.YesNo(p.RequiresRestart), p.Description)
			}
			t.Render()
			if cfg.Note != "" {
				output.PrintInfo(cfg.Note)
			}
			return nil
		},
	}

	setCmd := &cobra.Command{
		Use:   "set <md_uuid> <name=value>...",
		Short: "Change one or more parameters",
		Long: `Change one or more configuration parameters. Numbers are sent as numbers and
true/false as booleans; anything else is sent as text. Parameters marked
'Restart' in 'config show' trigger a brief rolling restart.`,
		Example: `  cubecli mdb config set <md_uuid> max_connections=500 slow_query_log=ON`,
		Args:    cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			params, err := parseParams(args[1:])
			if err != nil {
				return err
			}

			client := cmdutil.GetClient(cmd)
			s := output.NewSpinner("Updating configuration...")
			s.Start()
			resp, err := client.Patch(fmt.Sprintf("/managed-databases/%s/config", args[0]), map[string]interface{}{"params": params})
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			var r struct {
				Detail string `json:"detail"`
			}
			if json.Unmarshal(resp, &r) == nil && r.Detail != "" {
				output.PrintSuccess(r.Detail)
			} else {
				output.PrintSuccess("Configuration update initiated")
			}
			return nil
		},
	}

	configCmd.AddCommand(showCmd, setCmd)
	return configCmd
}

func valueOrDash(v interface{}) string {
	if v == nil {
		return "-"
	}
	return fmt.Sprint(v)
}

// parseParams turns name=value pairs into typed JSON values.
func parseParams(pairs []string) (map[string]interface{}, error) {
	params := map[string]interface{}{}
	for _, pair := range pairs {
		name, value, ok := strings.Cut(pair, "=")
		name = strings.TrimSpace(name)
		if !ok || name == "" {
			return nil, fmt.Errorf("invalid parameter %q: expected name=value", pair)
		}
		switch {
		case value == "true" || value == "false":
			params[name] = value == "true"
		default:
			if i, err := strconv.ParseInt(value, 10, 64); err == nil {
				params[name] = i
			} else if f, err := strconv.ParseFloat(value, 64); err == nil {
				params[name] = f
			} else {
				params[name] = value
			}
		}
	}
	return params, nil
}

// --- metrics ---

func metricsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "metrics <md_uuid>",
		Short: "Show time-series metrics (connections, cpu, memory, replication lag)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			q := url.Values{}
			if r, _ := cmd.Flags().GetString("range"); r != "" {
				q.Set("time_range", r)
			}
			if m, _ := cmd.Flags().GetStringSlice("metric"); len(m) > 0 {
				q.Set("metrics", strings.Join(m, ","))
			}
			path := fmt.Sprintf("/managed-databases/%s/metrics", args[0])
			if len(q) > 0 {
				path += "?" + q.Encode()
			}

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
			return output.RenderSeries("Managed Database Metrics", resp)
		},
	}
	cmd.Flags().String("range", "1h", "Time range: 1h, 24h, 7d, 30d...")
	cmd.Flags().StringSlice("metric", nil, "Metrics to fetch (connections, cpu, memory, replication_lag); default all")
	return cmd
}
