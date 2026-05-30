package network

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/CubePathInc/cubecli/internal/cmdutil"
	"github.com/CubePathInc/cubecli/internal/output"
	"github.com/spf13/cobra"
)

// addRouteCmd wires the `network route` subcommands (list/create/delete) for
// managing custom routes on a private network.
func addRouteCmd(parent *cobra.Command) {
	routeCmd := &cobra.Command{
		Use:   "route",
		Short: "Manage private network routes",
	}

	routeListCmd := &cobra.Command{
		Use:   "list <network_id>",
		Short: "List routes for a private network",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			networkID, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid network_id: %s", args[0])
			}

			s := output.NewSpinner("Fetching routes...")
			s.Start()
			resp, err := client.Get(fmt.Sprintf("/networks/%d/routes", networkID))
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			var routes []struct {
				ID              string `json:"id"`
				Destination     string `json:"destination"`
				NextHopType     string `json:"next_hop_type"`
				NextHopTarget   string `json:"next_hop_target"`
				ResolvedNextHop string `json:"resolved_next_hop_ip"`
				Description     string `json:"description"`
				NatGatewayName  string `json:"nat_gateway_name"`
			}
			if err := json.Unmarshal(resp, &routes); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}

			if len(routes) == 0 {
				fmt.Println("No routes found for this network.")
				return nil
			}

			t := output.NewTable("Network Routes", []string{"ID", "Destination", "Next Hop", "Target", "Resolved IP", "NAT Gateway", "Description"})
			for _, r := range routes {
				t.AddRow(
					r.ID,
					r.Destination,
					r.NextHopType,
					r.NextHopTarget,
					r.ResolvedNextHop,
					r.NatGatewayName,
					r.Description,
				)
			}
			t.Render()
			return nil
		},
	}

	routeCreateCmd := &cobra.Command{
		Use:   "create <network_id>",
		Short: "Create a route on a private network",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			networkID, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid network_id: %s", args[0])
			}

			destination, _ := cmd.Flags().GetString("destination")
			nextHopType, _ := cmd.Flags().GetString("next-hop-type")
			nextHopTarget, _ := cmd.Flags().GetString("next-hop-target")
			description, _ := cmd.Flags().GetString("description")

			body := map[string]interface{}{
				"destination":     destination,
				"next_hop_type":   nextHopType,
				"next_hop_target": nextHopTarget,
			}
			if description != "" {
				body["description"] = description
			}

			s := output.NewSpinner("Creating route...")
			s.Start()
			resp, err := client.Post(fmt.Sprintf("/networks/%d/routes", networkID), body)
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			output.PrintSuccess("Route created successfully")
			return nil
		},
	}

	routeDeleteCmd := &cobra.Command{
		Use:   "delete <network_id> <route_id>",
		Short: "Delete a route from a private network",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			networkID, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid network_id: %s", args[0])
			}
			routeID := args[1]

			if !cmdutil.CheckForce(cmd, "Are you sure you want to delete this route?") {
				output.PrintWarning("Aborted")
				return nil
			}

			s := output.NewSpinner("Deleting route...")
			s.Start()
			resp, err := client.Delete(fmt.Sprintf("/networks/%d/routes/%s", networkID, routeID))
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			output.PrintSuccess("Route deleted successfully")
			return nil
		},
	}

	routeCreateCmd.Flags().StringP("destination", "d", "", "Destination CIDR (e.g. 0.0.0.0/0)")
	routeCreateCmd.Flags().StringP("next-hop-type", "t", "", "Next hop type (ip, vps, baremetal)")
	routeCreateCmd.Flags().StringP("next-hop-target", "g", "", "Next hop target (IP address, or numeric VPS/baremetal ID)")
	routeCreateCmd.Flags().String("description", "", "Optional description")
	routeCreateCmd.MarkFlagRequired("destination")
	routeCreateCmd.MarkFlagRequired("next-hop-type")
	routeCreateCmd.MarkFlagRequired("next-hop-target")

	routeDeleteCmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")

	routeCmd.AddCommand(routeListCmd, routeCreateCmd, routeDeleteCmd)
	parent.AddCommand(routeCmd)
}
