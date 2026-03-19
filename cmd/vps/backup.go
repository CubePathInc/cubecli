package vps

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/CubePathInc/cubecli/internal/cmdutil"
	"github.com/CubePathInc/cubecli/internal/output"
	"github.com/spf13/cobra"
)

func addBackupCmd(parent *cobra.Command) {
	backupCmd := &cobra.Command{
		Use:   "backup",
		Short: "Manage VPS backups",
	}

	backupListCmd := &cobra.Command{
		Use:   "list <vps_id>",
		Short: "List backups for a VPS",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			vpsID, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid vps_id: %s", args[0])
			}

			s := output.NewSpinner("Fetching backups...")
			s.Start()
			resp, err := client.Get(fmt.Sprintf("/vps/%d/backups", vpsID))
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			var result struct {
				Backups []struct {
					ID       int    `json:"id"`
					Type     string `json:"backup_type"`
					Status   string `json:"status"`
					Progress int     `json:"progress"`
					Size     float64 `json:"size_gb"`
					Notes    string `json:"notes"`
					Created  string `json:"created_at"`
				} `json:"backups"`
			}
			if err := json.Unmarshal(resp, &result); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}

			t := output.NewTable("Backups", []string{"ID", "Type", "Status", "Progress", "Size", "Notes", "Created"})
			for _, b := range result.Backups {
				t.AddRow(
					strconv.Itoa(b.ID),
					b.Type,
					output.FormatStatus(b.Status),
					fmt.Sprintf("%d%%", b.Progress),
					fmt.Sprintf("%.2f GB", b.Size),
					b.Notes,
					b.Created,
				)
			}
			t.Render()
			return nil
		},
	}

	backupCreateCmd := &cobra.Command{
		Use:   "create <vps_id>",
		Short: "Create a backup for a VPS",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			vpsID, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid vps_id: %s", args[0])
			}

			body := map[string]interface{}{}
			notes, _ := cmd.Flags().GetString("notes")
			if notes != "" {
				body["notes"] = notes
			}

			s := output.NewSpinner("Creating backup...")
			s.Start()
			resp, err := client.Post(fmt.Sprintf("/vps/%d/backups", vpsID), body)
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			output.PrintSuccess("Backup created successfully")
			return nil
		},
	}

	backupRestoreCmd := &cobra.Command{
		Use:   "restore <vps_id> <backup_id>",
		Short: "Restore a VPS from a backup",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			vpsID, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid vps_id: %s", args[0])
			}

			backupID, err := strconv.Atoi(args[1])
			if err != nil {
				return fmt.Errorf("invalid backup_id: %s", args[1])
			}

			if !cmdutil.CheckForce(cmd, "Are you sure you want to restore this backup? This will overwrite the current VPS state.") {
				output.PrintWarning("Aborted")
				return nil
			}

			body := map[string]interface{}{
				"confirm": true,
			}

			s := output.NewSpinner("Restoring backup...")
			s.Start()
			resp, err := client.Post(fmt.Sprintf("/vps/%d/backups/%d/restore", vpsID, backupID), body)
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			output.PrintSuccess("Backup restored successfully")
			return nil
		},
	}

	backupDeleteCmd := &cobra.Command{
		Use:   "delete <vps_id> <backup_id>",
		Short: "Delete a VPS backup",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			vpsID, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid vps_id: %s", args[0])
			}

			backupID, err := strconv.Atoi(args[1])
			if err != nil {
				return fmt.Errorf("invalid backup_id: %s", args[1])
			}

			if !cmdutil.CheckForce(cmd, "Are you sure you want to delete this backup?") {
				output.PrintWarning("Aborted")
				return nil
			}

			s := output.NewSpinner("Deleting backup...")
			s.Start()
			resp, err := client.Delete(fmt.Sprintf("/vps/%d/backups/%d", vpsID, backupID))
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			output.PrintSuccess("Backup deleted successfully")
			return nil
		},
	}

	backupSettingsCmd := &cobra.Command{
		Use:   "settings <vps_id>",
		Short: "Show backup settings for a VPS",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			vpsID, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid vps_id: %s", args[0])
			}

			s := output.NewSpinner("Fetching backup settings...")
			s.Start()
			resp, err := client.Get(fmt.Sprintf("/vps/%d/backup/settings", vpsID))
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			var settings struct {
				Enabled       bool `json:"enabled"`
				ScheduleHour  int  `json:"schedule_hour"`
				RetentionDays int  `json:"retention_days"`
				MaxBackups    int  `json:"max_backups"`
			}
			if err := json.Unmarshal(resp, &settings); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}

			enabledStr := "disabled"
			if settings.Enabled {
				enabledStr = "enabled"
			}

			t := output.NewTable("Backup Settings", []string{"Field", "Value"})
			t.AddRow("Enabled", output.FormatStatus(enabledStr))
			t.AddRow("Schedule Hour", strconv.Itoa(settings.ScheduleHour))
			t.AddRow("Retention Days", strconv.Itoa(settings.RetentionDays))
			t.AddRow("Max Backups", strconv.Itoa(settings.MaxBackups))
			t.Render()
			return nil
		},
	}

	backupConfigureCmd := &cobra.Command{
		Use:   "configure <vps_id>",
		Short: "Configure backup settings for a VPS",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			vpsID, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid vps_id: %s", args[0])
			}

			enable, _ := cmd.Flags().GetBool("enable")
			disable, _ := cmd.Flags().GetBool("disable")

			if !enable && !disable {
				return fmt.Errorf("one of --enable or --disable is required")
			}
			if enable && disable {
				return fmt.Errorf("--enable and --disable are mutually exclusive")
			}

			hour, _ := cmd.Flags().GetInt("hour")
			retention, _ := cmd.Flags().GetInt("retention")
			max, _ := cmd.Flags().GetInt("max")

			body := map[string]interface{}{
				"enabled":        enable,
				"schedule_hour":  hour,
				"retention_days": retention,
				"max_backups":    max,
			}

			s := output.NewSpinner("Updating backup settings...")
			s.Start()
			resp, err := client.Put(fmt.Sprintf("/vps/%d/backup/settings", vpsID), body)
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			output.PrintSuccess("Backup settings updated successfully")
			return nil
		},
	}

	backupCreateCmd.Flags().StringP("notes", "n", "", "Notes for the backup")

	backupRestoreCmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")
	backupDeleteCmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")

	backupConfigureCmd.Flags().Bool("enable", false, "Enable automatic backups")
	backupConfigureCmd.Flags().Bool("disable", false, "Disable automatic backups")
	backupConfigureCmd.Flags().Int("hour", 3, "Schedule hour for backups (0-23)")
	backupConfigureCmd.Flags().Int("retention", 7, "Retention period in days")
	backupConfigureCmd.Flags().Int("max", 3, "Maximum number of backups to keep")

	backupCmd.AddCommand(backupListCmd, backupCreateCmd, backupRestoreCmd, backupDeleteCmd, backupSettingsCmd, backupConfigureCmd)
	parent.AddCommand(backupCmd)
}
