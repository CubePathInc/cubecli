package baremetal

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/CubePathInc/cubecli/internal/cmdutil"
	"github.com/CubePathInc/cubecli/internal/output"
	"github.com/spf13/cobra"
)

func addReinstallCmd(parent *cobra.Command) {
	reinstallCmd := &cobra.Command{
		Use:   "reinstall",
		Short: "Manage baremetal server reinstallation",
	}

	reinstallStartCmd := &cobra.Command{
		Use:   "start <id>",
		Short: "Start a baremetal server reinstallation",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			bmID, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid baremetal id: %s", args[0])
			}

			if !cmdutil.CheckForce(cmd, "Are you sure you want to reinstall this server? All data will be lost.") {
				output.PrintWarning("Aborted")
				return nil
			}

			osName, _ := cmd.Flags().GetString("os")
			hostname, _ := cmd.Flags().GetString("hostname")
			user, _ := cmd.Flags().GetString("user")
			password, _ := cmd.Flags().GetString("password")
			diskLayout, _ := cmd.Flags().GetString("disk-layout")

			body := map[string]interface{}{
				"os_name":  osName,
				"hostname": hostname,
				"user":     user,
				"password": password,
			}
			if diskLayout != "" {
				body["disk_layout_name"] = diskLayout
			}

			s := output.NewSpinner("Starting reinstallation...")
			s.Start()
			resp, err := client.Post(fmt.Sprintf("/baremetal/%d/reinstall", bmID), body)
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			output.PrintSuccess("Reinstallation started")
			return nil
		},
	}

	reinstallStatusCmd := &cobra.Command{
		Use:   "status <id>",
		Short: "Check reinstallation status",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			bmID, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid baremetal id: %s", args[0])
			}

			s := output.NewSpinner("Fetching reinstallation status...")
			s.Start()
			resp, err := client.Get(fmt.Sprintf("/baremetal/%d/reinstall/status", bmID))
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			var result struct {
				IsReinstalling bool   `json:"is_reinstalling"`
				Status         string `json:"status"`
				OSName         string `json:"os_name"`
			}
			if err := json.Unmarshal(resp, &result); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}

			reinstalling := "no"
			if result.IsReinstalling {
				reinstalling = "yes"
			}

			t := output.NewTable("Reinstallation Status", []string{"Field", "Value"})
			t.AddRow("Reinstalling", reinstalling)
			t.AddRow("Status", output.FormatStatus(result.Status))
			t.AddRow("OS", result.OSName)
			t.Render()

			return nil
		},
	}

	reinstallStartCmd.Flags().String("os", "", "OS name to install")
	reinstallStartCmd.Flags().String("hostname", "", "Hostname for the server")
	reinstallStartCmd.Flags().StringP("user", "u", "root", "Username")
	reinstallStartCmd.Flags().String("password", "", "Password for the server")
	reinstallStartCmd.Flags().String("disk-layout", "", "Disk layout name")
	reinstallStartCmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")
	reinstallStartCmd.MarkFlagRequired("os")
	reinstallStartCmd.MarkFlagRequired("hostname")
	reinstallStartCmd.MarkFlagRequired("password")

	reinstallCmd.AddCommand(reinstallStartCmd, reinstallStatusCmd)
	parent.AddCommand(reinstallCmd)
}
