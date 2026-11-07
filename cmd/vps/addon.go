package vps

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/CubePathInc/cubecli/internal/api"
	"github.com/CubePathInc/cubecli/internal/cmdutil"
	"github.com/CubePathInc/cubecli/internal/output"
	"github.com/spf13/cobra"
)

func addAddonCmds(parent *cobra.Command) {
	addonCmd := &cobra.Command{
		Use:   "addon",
		Short: "Manage the billing add-ons of a VPS",
	}

	removeCmd := &cobra.Command{
		Use:   "remove <vps_id> <slug|uuid>",
		Short: "Remove a billing add-on from a VPS",
		Long: `Remove a billing add-on from a VPS, for example the 10 Gbps port (vps-port-10g).

Billing for it stops at once and what was already charged is not refunded. The
port goes back to 1 Gbps shortly, with no restart and no downtime. To add the port
again, use the Addons tab of the server in the panel.`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			vpsID, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid VPS ID: %s", args[0])
			}

			s := output.NewSpinner("Fetching the add-ons of the VPS...")
			s.Start()
			addon, err := findVPSAddon(client, vpsID, args[1])
			s.Stop()
			if err != nil {
				return err
			}

			msg := fmt.Sprintf("Remove %s from VPS %d? Billing for it stops at once and nothing is refunded.", addon.Slug, vpsID)
			if !cmdutil.CheckForce(cmd, msg) {
				output.PrintInfo("Operation cancelled")
				return nil
			}

			s = output.NewSpinner("Removing the add-on...")
			s.Start()
			resp, err := client.Delete(fmt.Sprintf("/vps/%d/addons/%s", vpsID, addon.UUID))
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}
			var result struct {
				Detail string `json:"detail"`
			}
			if err := json.Unmarshal(resp, &result); err != nil {
				return err
			}
			output.PrintSuccess(result.Detail)
			return nil
		},
	}
	removeCmd.Flags().BoolP("force", "f", false, "Skip confirmation")

	addonCmd.AddCommand(removeCmd)
	parent.AddCommand(addonCmd)
}

type vpsAddon struct {
	UUID   string `json:"uuid"`
	Slug   string `json:"slug"`
	Status string `json:"status"`
}

// findVPSAddon returns the live contract of the VPS with this slug or uuid. The
// list of servers carries the contracts with the vps:read scope, which any token
// that may remove an add-on has; /service-addons also asks for a billing permission.
func findVPSAddon(client *api.Client, vpsID int, ref string) (*vpsAddon, error) {
	resp, err := client.Get("/vps/")
	if err != nil {
		return nil, err
	}
	var servers []struct {
		ID     int        `json:"id"`
		Addons []vpsAddon `json:"addons"`
	}
	if err := json.Unmarshal(resp, &servers); err != nil {
		return nil, err
	}
	for _, server := range servers {
		if server.ID != vpsID {
			continue
		}
		for i, addon := range server.Addons {
			if (addon.Slug == ref || addon.UUID == ref) && addon.Status != "cancelled" && addon.Status != "completed" {
				return &server.Addons[i], nil
			}
		}
		return nil, fmt.Errorf("VPS %d has no add-on %s", vpsID, ref)
	}
	return nil, fmt.Errorf("VPS with ID %d not found", vpsID)
}
