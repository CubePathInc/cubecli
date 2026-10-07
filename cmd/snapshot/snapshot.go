// Package snapshot implements `cubecli snapshot`: VPS snapshots, permanent
// copies of a VPS disk taken from the server itself or converted from a
// completed backup.
package snapshot

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

// Statuses the API accepts as a list filter (deleted snapshots are never listed).
var listStatuses = []string{"pending", "converting", "available", "failed", "deleting"}

// Location is a snapshot location as the API returns it.
type Location struct {
	Name        string  `json:"name"`
	Description *string `json:"description"`
}

// Snapshot mirrors SnapshotOut of GET /snapshots and GET /snapshots/{uuid}.
type Snapshot struct {
	UUID          string   `json:"uuid"`
	Name          string   `json:"name"`
	Description   *string  `json:"description"`
	Status        string   `json:"status"`
	OSType        *string  `json:"os_type"`
	DiskGB        *int     `json:"disk_gb"`
	EstimatedGB   int      `json:"estimated_gb"`
	BillableGB    *int     `json:"billable_gb"`
	Location      Location `json:"location"`
	StoreLocation Location `json:"store_location"`
	ProjectID     *int     `json:"project_id"`
	SourceVPS     struct {
		ID      *int    `json:"id"`
		Name    *string `json:"name"`
		Deleted bool    `json:"deleted"`
	} `json:"source_vps"`
	Template struct {
		TemplateName    string `json:"template_name"`
		Name            string `json:"name"`
		OperatingSystem string `json:"operating_system"`
	} `json:"template"`
	PriceGBMonth    float64          `json:"price_gb_month"`
	MonthlyCost     float64          `json:"monthly_cost"`
	HourlyCost      float64          `json:"hourly_cost"`
	DeployingCount  int              `json:"deploying_count"`
	CreatedAt       string           `json:"created_at"`
	AvailableAt     *string          `json:"available_at"`
	DeployEstimates []DeployEstimate `json:"deploy_estimates"`
}

// DeployEstimate is the expected deploy time of a snapshot in one location.
type DeployEstimate struct {
	LocationName string `json:"location_name"`
	Remote       bool   `json:"remote"`
	Minutes      int    `json:"minutes"`
}

// ListResponse is the body of GET /snapshots.
type ListResponse struct {
	Snapshots []Snapshot `json:"snapshots"`
	Total     int        `json:"total"`
}

// Quota is the body of GET /snapshots/quota.
type Quota struct {
	Count        int     `json:"count"`
	CountMax     int     `json:"count_max"`
	GB           int     `json:"gb"`
	GBMax        int     `json:"gb_max"`
	PriceGBMonth float64 `json:"price_gb_month"`
}

func parseList(resp []byte) (ListResponse, error) {
	var r ListResponse
	if err := json.Unmarshal(resp, &r); err != nil {
		return r, fmt.Errorf("failed to parse response: %w", err)
	}
	return r, nil
}

func parseSnapshot(resp []byte) (Snapshot, error) {
	var s Snapshot
	if err := json.Unmarshal(resp, &s); err != nil {
		return s, fmt.Errorf("failed to parse response: %w", err)
	}
	return s, nil
}

func parseQuota(resp []byte) (Quota, error) {
	var q Quota
	if err := json.Unmarshal(resp, &q); err != nil {
		return q, fmt.Errorf("failed to parse response: %w", err)
	}
	return q, nil
}

// sizeGB is the disk size used for billing: disk_gb once known, else the estimate.
func (s Snapshot) sizeGB() string {
	if s.DiskGB != nil {
		return fmt.Sprintf("%d GB", *s.DiskGB)
	}
	return fmt.Sprintf("~%d GB", s.EstimatedGB)
}

func (s Snapshot) sourceVPS() string {
	name := "-"
	if s.SourceVPS.Name != nil && *s.SourceVPS.Name != "" {
		name = *s.SourceVPS.Name
	}
	if s.SourceVPS.ID != nil {
		name = fmt.Sprintf("%s (%d)", name, *s.SourceVPS.ID)
	}
	if s.SourceVPS.Deleted {
		name += " (deleted)"
	}
	return name
}

func (s Snapshot) osName() string {
	os := s.Template.Name
	if os == "" {
		os = s.Template.TemplateName
	}
	if s.OSType != nil && *s.OSType != "" {
		if os == "" {
			return *s.OSType
		}
		return fmt.Sprintf("%s (%s)", os, *s.OSType)
	}
	if os == "" {
		return "-"
	}
	return os
}

func locationName(l Location) string {
	if l.Name == "" {
		return "-"
	}
	return l.Name
}

func strOr(p *string, def string) string {
	if p == nil || *p == "" {
		return def
	}
	return *p
}

func money(v float64) string {
	return "$" + strconv.FormatFloat(v, 'f', 2, 64)
}

func NewCmd() *cobra.Command {
	snapshotCmd := &cobra.Command{
		Use:     "snapshot",
		Aliases: []string{"snapshots"},
		Short:   "Manage VPS snapshots",
		Long: `Manage VPS snapshots.

A snapshot is a permanent copy of a VPS disk, taken directly from the server
(backups do not need to be enabled) or converted from a completed backup.
It belongs to the organization: it does not expire with the backup retention and
survives the deletion of the source VPS. It is billed per GB of disk per month
until deleted (see "cubecli snapshot quota" for the price and your limits).

Deploy a new VPS from a snapshot in any location with:
  cubecli vps create --snapshot <uuid> ...`,
	}

	snapshotCmd.AddCommand(
		listCmd(),
		getCmd(),
		quotaCmd(),
		createCmd(),
		updateCmd(),
		renameCmd(),
		moveProjectCmd(),
		deleteCmd(),
	)
	return snapshotCmd
}

// --- list ---

func listCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List the snapshots of the organization",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			q := url.Values{}
			if p, _ := cmd.Flags().GetInt("project"); p != 0 {
				q.Set("project_id", strconv.Itoa(p))
			}
			if st, _ := cmd.Flags().GetString("status"); st != "" {
				if !contains(listStatuses, st) {
					return fmt.Errorf("invalid --status %q: use one of %s", st, strings.Join(listStatuses, ", "))
				}
				q.Set("status", st)
			}
			if v, _ := cmd.Flags().GetInt("vps"); v != 0 {
				q.Set("source_vps_id", strconv.Itoa(v))
			}
			if l, _ := cmd.Flags().GetString("location"); l != "" {
				q.Set("location_name", l)
			}
			limit, _ := cmd.Flags().GetInt("limit")
			if limit < 1 || limit > 100 {
				return fmt.Errorf("--limit must be between 1 and 100")
			}
			q.Set("limit", strconv.Itoa(limit))
			offset, _ := cmd.Flags().GetInt("offset")
			if offset < 0 {
				return fmt.Errorf("--offset can not be negative")
			}
			if offset > 0 {
				q.Set("offset", strconv.Itoa(offset))
			}

			client := cmdutil.GetClient(cmd)
			s := output.NewSpinner("Fetching snapshots...")
			s.Start()
			resp, err := client.Get("/snapshots?" + q.Encode())
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			result, err := parseList(resp)
			if err != nil {
				return err
			}
			if len(result.Snapshots) == 0 {
				fmt.Println("No snapshots found.")
				return nil
			}

			t := output.NewTable("Snapshots", []string{"UUID", "Name", "Source VPS", "OS", "Location", "Size", "Created", "Status", "Monthly"})
			for _, sn := range result.Snapshots {
				t.AddRow(
					sn.UUID,
					sn.Name,
					sn.sourceVPS(),
					sn.osName(),
					locationName(sn.Location),
					sn.sizeGB(),
					sn.CreatedAt,
					output.FormatStatus(sn.Status),
					money(sn.MonthlyCost),
				)
			}
			t.Render()
			if result.Total > offset+len(result.Snapshots) {
				output.PrintInfo(fmt.Sprintf("Showing %d of %d snapshots; use --offset %d for more", len(result.Snapshots), result.Total, offset+len(result.Snapshots)))
			}
			return nil
		},
	}
	cmd.Flags().IntP("project", "p", 0, "Filter by project ID")
	cmd.Flags().String("status", "", "Filter by status ("+strings.Join(listStatuses, ", ")+")")
	cmd.Flags().Int("vps", 0, "Filter by source VPS ID")
	cmd.Flags().StringP("location", "l", "", "Filter by location name")
	cmd.Flags().Int("limit", 50, "Maximum number of snapshots (1-100)")
	cmd.Flags().Int("offset", 0, "Number of snapshots to skip")
	return cmd
}

// --- get ---

func getCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "get <snapshot_uuid>",
		Aliases: []string{"show"},
		Short:   "Show snapshot details and deploy time estimates",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			s := output.NewSpinner("Fetching snapshot...")
			s.Start()
			resp, err := client.Get("/snapshots/" + url.PathEscape(args[0]))
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			sn, err := parseSnapshot(resp)
			if err != nil {
				return err
			}

			info := output.NewTable("Snapshot", []string{"Field", "Value"})
			info.AddRow("UUID", sn.UUID)
			info.AddRow("Name", sn.Name)
			if sn.Description != nil && *sn.Description != "" {
				info.AddRow("Description", *sn.Description)
			}
			info.AddRow("Status", output.FormatStatus(sn.Status))
			info.AddRow("OS", sn.osName())
			info.AddRow("Size", sn.sizeGB())
			info.AddRow("Location", locationName(sn.Location))
			info.AddRow("Stored in", locationName(sn.StoreLocation))
			if sn.ProjectID != nil {
				info.AddRow("Project", strconv.Itoa(*sn.ProjectID))
			}
			info.AddRow("Source VPS", sn.sourceVPS())
			info.AddRow("Price", fmt.Sprintf("$%s per GB per month", strconv.FormatFloat(sn.PriceGBMonth, 'f', -1, 64)))
			info.AddRow("Cost", fmt.Sprintf("%s/month (%s/hour)", money(sn.MonthlyCost), strconv.FormatFloat(sn.HourlyCost, 'f', 6, 64)))
			info.AddRow("Deploying now", strconv.Itoa(sn.DeployingCount))
			info.AddRow("Created", sn.CreatedAt)
			info.AddRow("Available since", strOr(sn.AvailableAt, "-"))
			info.Render()

			if len(sn.DeployEstimates) > 0 {
				et := output.NewTable("Deploy Estimates", []string{"Location", "Remote", "Minutes"})
				for _, e := range sn.DeployEstimates {
					et.AddRow(e.LocationName, cmdutil.YesNo(e.Remote), strconv.Itoa(e.Minutes))
				}
				et.Render()
			}
			return nil
		},
	}
}

// --- quota ---

func quotaCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "quota",
		Short: "Show the snapshot limits, usage and price of the organization",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			s := output.NewSpinner("Fetching snapshot quota...")
			s.Start()
			resp, err := client.Get("/snapshots/quota")
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			q, err := parseQuota(resp)
			if err != nil {
				return err
			}
			t := output.NewTable("Snapshot Quota", []string{"Field", "Value"})
			t.AddRow("Count", fmt.Sprintf("%d / %d", q.Count, q.CountMax))
			t.AddRow("Storage", fmt.Sprintf("%d / %d GB", q.GB, q.GBMax))
			t.AddRow("Price", fmt.Sprintf("$%s per GB per month", strconv.FormatFloat(q.PriceGBMonth, 'f', -1, 64)))
			t.Render()
			return nil
		},
	}
}

// --- create ---

func createCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Take a snapshot of a VPS now, or convert one of its backups",
		Long: `Take a snapshot of a VPS now, or convert one of its completed backups.

Without --backup the snapshot copies the current disk of the VPS: backups do
not need to be enabled. With --backup it converts that completed backup
instead (find the ID with "cubecli vps backup list <vps_id>").

The snapshot is billed per GB of the VPS disk per month until you delete it.
It is queued and takes a few minutes: follow it with
"cubecli snapshot get <uuid>" until its status is available.`,
		Example: `  cubecli snapshot create --vps 20467 --name web-01-golden
  cubecli snapshot create --vps 20467 --backup 991 --name web-01-before-upgrade`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			vpsID, _ := cmd.Flags().GetInt("vps")
			backupID, _ := cmd.Flags().GetInt("backup")
			name, _ := cmd.Flags().GetString("name")
			if strings.TrimSpace(name) == "" {
				return fmt.Errorf("--name can not be empty")
			}
			body := map[string]interface{}{
				"vps_id": vpsID,
				"name":   name,
			}
			if cmd.Flags().Changed("backup") {
				if backupID <= 0 {
					return fmt.Errorf("--backup must be a backup ID (omit it to snapshot the VPS now)")
				}
				body["backup_id"] = backupID
			}
			if cmd.Flags().Changed("description") {
				d, _ := cmd.Flags().GetString("description")
				body["description"] = d
			}

			client := cmdutil.GetClient(cmd)
			s := output.NewSpinner("Requesting snapshot...")
			s.Start()
			resp, err := client.Post("/snapshots", body)
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			var r struct {
				Detail   string   `json:"detail"`
				Snapshot Snapshot `json:"snapshot"`
			}
			if err := json.Unmarshal(resp, &r); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}
			msg := r.Detail
			if msg == "" {
				msg = "Snapshot requested"
			}
			output.PrintSuccess(msg)
			if r.Snapshot.UUID != "" {
				output.PrintInfo(fmt.Sprintf("Snapshot %s, about %s/month. Follow it with: cubecli snapshot get %s", r.Snapshot.UUID, money(r.Snapshot.MonthlyCost), r.Snapshot.UUID))
			}
			return nil
		},
	}
	cmd.Flags().Int("vps", 0, "Source VPS ID")
	cmd.Flags().Int("backup", 0, "Convert this completed backup of the VPS instead of taking the snapshot now")
	cmd.Flags().StringP("name", "n", "", "Snapshot name (up to 100 characters)")
	cmd.Flags().StringP("description", "d", "", "Snapshot description (up to 500 characters)")
	_ = cmd.MarkFlagRequired("vps")
	_ = cmd.MarkFlagRequired("name")
	return cmd
}

// --- update / rename / move-project ---

func patch(cmd *cobra.Command, uuid string, body map[string]interface{}, spin string) error {
	client := cmdutil.GetClient(cmd)
	return cmdutil.RunDetail(cmd, spin, "Snapshot updated", func() (json.RawMessage, error) {
		return client.Patch("/snapshots/"+url.PathEscape(uuid), body)
	})
}

func updateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update <snapshot_uuid>",
		Short: "Rename a snapshot, change its description or move it to another project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			body := map[string]interface{}{}
			if cmd.Flags().Changed("name") {
				name, _ := cmd.Flags().GetString("name")
				if strings.TrimSpace(name) == "" {
					return fmt.Errorf("--name can not be empty")
				}
				body["name"] = name
			}
			if cmd.Flags().Changed("description") {
				d, _ := cmd.Flags().GetString("description")
				if d == "" {
					body["description"] = nil
				} else {
					body["description"] = d
				}
			}
			if cmd.Flags().Changed("project") {
				p, _ := cmd.Flags().GetInt("project")
				body["project_id"] = p
			}
			if len(body) == 0 {
				return fmt.Errorf("nothing to update: use --name, --description or --project")
			}
			return patch(cmd, args[0], body, "Updating snapshot...")
		},
	}
	cmd.Flags().StringP("name", "n", "", "New name")
	cmd.Flags().StringP("description", "d", "", `New description ("" clears it)`)
	cmd.Flags().IntP("project", "p", 0, "Move to this project ID")
	return cmd
}

func renameCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rename <snapshot_uuid> <new_name>",
		Short: "Rename a snapshot",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(args[1]) == "" {
				return fmt.Errorf("the new name can not be empty")
			}
			return patch(cmd, args[0], map[string]interface{}{"name": args[1]}, "Renaming snapshot...")
		},
	}
}

func moveProjectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "move-project <snapshot_uuid>",
		Aliases: []string{"move"},
		Short:   "Move a snapshot to another project in the same organization",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p, _ := cmd.Flags().GetInt("project")
			return patch(cmd, args[0], map[string]interface{}{"project_id": p}, "Moving snapshot...")
		},
	}
	cmd.Flags().IntP("project", "p", 0, "Target project ID")
	_ = cmd.MarkFlagRequired("project")
	return cmd
}

// --- delete ---

func deleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <snapshot_uuid>",
		Short: "Delete a snapshot permanently and stop its billing",
		Long: `Delete a snapshot permanently and stop its billing.

Servers already deployed from the snapshot are not affected. A snapshot can not
be deleted while it is being converted or while a deployment is using it.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmdutil.CheckForce(cmd, fmt.Sprintf("Are you sure you want to delete snapshot %s? This can not be undone.", args[0])) {
				output.PrintWarning("Aborted")
				return nil
			}
			client := cmdutil.GetClient(cmd)
			return cmdutil.RunDetail(cmd, "Deleting snapshot...", "Snapshot deletion started", func() (json.RawMessage, error) {
				return client.Delete("/snapshots/" + url.PathEscape(args[0]))
			})
		},
	}
	cmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")
	return cmd
}

func contains(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}
