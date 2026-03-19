package vps

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/CubePathInc/cubecli/internal/cmdutil"
	"github.com/CubePathInc/cubecli/internal/output"
	"github.com/spf13/cobra"
)

func addISOCmd(parent *cobra.Command) {
	isoCmd := &cobra.Command{
		Use:   "iso",
		Short: "Manage VPS ISO images",
	}

	isoListCmd := &cobra.Command{
		Use:   "list <vps_id>",
		Short: "List available ISOs for a VPS",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			vpsID, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid vps_id: %s", args[0])
			}

			s := output.NewSpinner("Fetching ISOs...")
			s.Start()
			resp, err := client.Get(fmt.Sprintf("/vps/%d/isos", vpsID))
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			var result struct {
				Items []struct {
					ID   string `json:"id"`
					Name string `json:"name"`
					FileSize  int    `json:"file_size"`
				IsMounted bool   `json:"is_mounted"`
				} `json:"items"`
				MountedISOID string `json:"mounted_iso_id"`
			}
			if err := json.Unmarshal(resp, &result); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}

			t := output.NewTable("ISOs", []string{"ID", "Name", "Size", "Mounted"})
			for _, iso := range result.Items {
				mounted := ""
				if iso.IsMounted {
					mounted = output.FormatStatus("active")
				}
				t.AddRow(iso.ID, iso.Name, fmt.Sprintf("%d", iso.FileSize), mounted)
			}
			t.Render()
			return nil
		},
	}

	isoMountCmd := &cobra.Command{
		Use:   "mount <vps_id> <iso_id>",
		Short: "Mount an ISO to a VPS",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			vpsID, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid vps_id: %s", args[0])
			}

			isoID := args[1]

			body := map[string]interface{}{
				"iso_id": isoID,
			}

			s := output.NewSpinner("Mounting ISO...")
			s.Start()
			resp, err := client.Post(fmt.Sprintf("/vps/%d/iso", vpsID), body)
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			output.PrintSuccess("ISO mounted successfully")
			return nil
		},
	}

	isoUnmountCmd := &cobra.Command{
		Use:   "unmount <vps_id>",
		Short: "Unmount the ISO from a VPS",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			vpsID, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid vps_id: %s", args[0])
			}

			s := output.NewSpinner("Unmounting ISO...")
			s.Start()
			resp, err := client.Delete(fmt.Sprintf("/vps/%d/iso", vpsID))
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			output.PrintSuccess("ISO unmounted successfully")
			return nil
		},
	}

	isoCmd.AddCommand(isoListCmd, isoMountCmd, isoUnmountCmd)
	parent.AddCommand(isoCmd)
}
