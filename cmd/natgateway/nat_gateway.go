package natgateway

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/CubePathInc/cubecli/internal/cmdutil"
	"github.com/CubePathInc/cubecli/internal/output"
	"github.com/spf13/cobra"
)

// floatingIP mirrors an entry of a NAT Gateway's floating IP list.
type floatingIP struct {
	Address string `json:"address"`
	Type    string `json:"type"`
}

// formatFloatingIPs joins floating IPs (IPv4 first) for display.
func formatFloatingIPs(ips []floatingIP) string {
	var v4, v6 []string
	for _, ip := range ips {
		if ip.Address == "" {
			continue
		}
		if ip.Type == "IPv6" {
			v6 = append(v6, ip.Address)
		} else {
			v4 = append(v4, ip.Address)
		}
	}
	return strings.Join(append(v4, v6...), ", ")
}

func NewCmd() *cobra.Command {
	ngCmd := &cobra.Command{
		Use:     "nat-gateway",
		Aliases: []string{"ng", "nat"},
		Short:   "Manage NAT gateways",
	}

	ngCmd.AddCommand(
		plansCmd(),
		listCmd(),
		showCmd(),
		createCmd(),
		updateCmd(),
		deleteCmd(),
		resizeCmd(),
		moveCmd(),
		protectionCmd(),
	)

	return ngCmd
}

// --- plans ---

func plansCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "plans",
		Short: "List available NAT gateway plans",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			s := output.NewSpinner("Fetching NAT gateway plans...")
			s.Start()
			resp, err := client.Get("/nat-gateway/plans")
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
					Name        string  `json:"name"`
					Description string  `json:"description"`
					PriceHour   float64 `json:"price_per_hour"`
					Bandwidth   int     `json:"bandwidth_mbps"`
					ConnPerSec  int     `json:"connections_per_second"`
				} `json:"plans"`
			}
			if err := json.Unmarshal(resp, &locations); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}

			t := output.NewTable("NAT Gateway Plans", []string{"Location", "Plan", "Price/Hour", "Bandwidth (Mbps)", "Conn/sec"})
			for _, loc := range locations {
				for _, p := range loc.Plans {
					t.AddRow(
						loc.LocationName,
						p.Name,
						fmt.Sprintf("$%.4f", p.PriceHour),
						strconv.Itoa(p.Bandwidth),
						strconv.Itoa(p.ConnPerSec),
					)
				}
			}
			t.Render()
			return nil
		},
	}
}

// --- list ---

func listCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List NAT gateways",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			filterLocation, _ := cmd.Flags().GetString("location")

			s := output.NewSpinner("Fetching NAT gateways...")
			s.Start()
			resp, err := client.Get("/nat-gateway/")
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			var ngs []struct {
				UUID         string       `json:"uuid"`
				Name         string       `json:"name"`
				Status       string       `json:"status"`
				PlanName     string       `json:"plan_name"`
				LocationName string       `json:"location_name"`
				NetworkName  string       `json:"network_name"`
				FloatingIPs  []floatingIP `json:"floating_ips"`
				Protected    bool         `json:"protected"`
			}
			if err := json.Unmarshal(resp, &ngs); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}

			t := output.NewTable("NAT Gateways", []string{"UUID", "Name", "Status", "Plan", "Location", "Network", "IPs", "Protected"})
			for _, ng := range ngs {
				if filterLocation != "" && !strings.EqualFold(ng.LocationName, filterLocation) {
					continue
				}
				protectedStr := "no"
				if ng.Protected {
					protectedStr = "yes"
				}
				t.AddRow(
					ng.UUID,
					ng.Name,
					output.FormatStatus(ng.Status),
					ng.PlanName,
					ng.LocationName,
					ng.NetworkName,
					formatFloatingIPs(ng.FloatingIPs),
					protectedStr,
				)
			}
			t.Render()
			return nil
		},
	}
	cmd.Flags().StringP("location", "l", "", "Filter by location")
	return cmd
}

// --- show ---

func showCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <ng_uuid>",
		Short: "Show NAT gateway details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			s := output.NewSpinner("Fetching NAT gateway...")
			s.Start()
			resp, err := client.Get("/nat-gateway/" + args[0])
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			var ng struct {
				UUID         string `json:"uuid"`
				Name         string `json:"name"`
				Label        string `json:"label"`
				Status       string `json:"status"`
				LocationName string `json:"location_name"`
				ProjectID    int    `json:"project_id"`
				ProjectName  string `json:"project_name"`
				NetworkID    int    `json:"network_id"`
				NetworkName  string `json:"network_name"`
				NetworkCIDR  string `json:"network_cidr"`
				PrivateIP    string `json:"private_ip"`
				Protected    bool   `json:"protected"`
				CreatedAt    string `json:"created_at"`
				Plan         struct {
					Name         string  `json:"name"`
					PricePerHour float64 `json:"price_per_hour"`
					Bandwidth    int     `json:"bandwidth_mbps"`
				} `json:"plan"`
				FloatingIP *floatingIP `json:"floating_ip"`
			}
			if err := json.Unmarshal(resp, &ng); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}

			info := output.NewTable("NAT Gateway Info", []string{"Field", "Value"})
			info.AddRow("UUID", ng.UUID)
			info.AddRow("Name", ng.Name)
			if ng.Label != "" {
				info.AddRow("Label", ng.Label)
			}
			info.AddRow("Status", output.FormatStatus(ng.Status))
			info.AddRow("Plan", ng.Plan.Name)
			info.AddRow("Bandwidth (Mbps)", strconv.Itoa(ng.Plan.Bandwidth))
			info.AddRow("Location", ng.LocationName)
			info.AddRow("Project", ng.ProjectName)
			info.AddRow("Network", fmt.Sprintf("%s (%s)", ng.NetworkName, ng.NetworkCIDR))
			if ng.FloatingIP != nil {
				info.AddRow("Public IP", ng.FloatingIP.Address)
			}
			if ng.PrivateIP != "" {
				info.AddRow("Private IP", ng.PrivateIP)
			}
			protectedStr := "no"
			if ng.Protected {
				protectedStr = "yes"
			}
			info.AddRow("Protected", protectedStr)
			info.AddRow("Created", ng.CreatedAt)
			info.Render()
			return nil
		},
	}
}

// --- create ---

func createCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new NAT gateway on a private network",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			name, _ := cmd.Flags().GetString("name")
			plan, _ := cmd.Flags().GetString("plan")
			networkID, _ := cmd.Flags().GetInt("network-id")
			projectID, _ := cmd.Flags().GetInt("project")
			label, _ := cmd.Flags().GetString("label")

			body := map[string]interface{}{
				"name":       name,
				"plan_name":  plan,
				"network_id": networkID,
			}
			if projectID > 0 {
				body["project_id"] = projectID
			}
			if label != "" {
				body["label"] = label
			}

			s := output.NewSpinner("Creating NAT gateway...")
			s.Start()
			resp, err := client.Post("/nat-gateway/", body)
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			var result struct {
				UUID string `json:"uuid"`
			}
			if err := json.Unmarshal(resp, &result); err == nil && result.UUID != "" {
				output.PrintSuccess(fmt.Sprintf("NAT gateway created: %s", result.UUID))
			} else {
				output.PrintSuccess("NAT gateway creation initiated")
			}
			return nil
		},
	}
	cmd.Flags().StringP("name", "n", "", "NAT gateway name")
	cmd.Flags().String("plan", "", "Plan name (see 'nat-gateway plans')")
	cmd.Flags().Int("network-id", 0, "ID of the private network to attach")
	cmd.Flags().IntP("project", "p", 0, "Project ID (inferred from the network if omitted)")
	cmd.Flags().String("label", "", "Optional label")
	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("plan")
	_ = cmd.MarkFlagRequired("network-id")
	return cmd
}

// --- update ---

func updateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update <ng_uuid>",
		Short: "Update a NAT gateway",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			body := map[string]interface{}{}
			if cmd.Flags().Changed("name") {
				name, _ := cmd.Flags().GetString("name")
				body["name"] = name
			}
			if cmd.Flags().Changed("label") {
				label, _ := cmd.Flags().GetString("label")
				body["label"] = label
			}

			if len(body) == 0 {
				return fmt.Errorf("at least one of --name or --label must be specified")
			}

			s := output.NewSpinner("Updating NAT gateway...")
			s.Start()
			resp, err := client.Patch("/nat-gateway/"+args[0], body)
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			output.PrintSuccess("NAT gateway updated successfully")
			return nil
		},
	}
	cmd.Flags().StringP("name", "n", "", "New name for the NAT gateway")
	cmd.Flags().String("label", "", "New label for the NAT gateway")
	return cmd
}

// --- delete ---

func deleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <ng_uuid>",
		Short: "Delete a NAT gateway",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmdutil.CheckForce(cmd, fmt.Sprintf("Are you sure you want to delete NAT gateway %s?", args[0])) {
				output.PrintWarning("Aborted")
				return nil
			}

			client := cmdutil.GetClient(cmd)

			s := output.NewSpinner("Deleting NAT gateway...")
			s.Start()
			resp, err := client.Delete("/nat-gateway/" + args[0])
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			output.PrintSuccess("NAT gateway deletion initiated")
			return nil
		},
	}
	cmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")
	return cmd
}

// --- resize ---

func resizeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "resize <ng_uuid>",
		Short: "Resize a NAT gateway to a different plan",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			plan, _ := cmd.Flags().GetString("plan")
			body := map[string]string{"plan_name": plan}

			s := output.NewSpinner("Resizing NAT gateway...")
			s.Start()
			resp, err := client.Post(fmt.Sprintf("/nat-gateway/%s/resize", args[0]), body)
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			output.PrintSuccess("NAT gateway resize initiated")
			return nil
		},
	}
	cmd.Flags().StringP("plan", "p", "", "New plan name")
	_ = cmd.MarkFlagRequired("plan")
	return cmd
}

// --- move ---

func moveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "move <ng_uuid>",
		Short: "Move a NAT gateway to another project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			projectID, _ := cmd.Flags().GetInt("project")

			body := map[string]interface{}{"project_id": projectID}

			s := output.NewSpinner("Moving NAT gateway...")
			s.Start()
			resp, err := client.Post(fmt.Sprintf("/nat-gateway/%s/move-to-project", args[0]), body)
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			output.PrintSuccess("NAT gateway moved successfully")
			return nil
		},
	}
	cmd.Flags().IntP("project", "p", 0, "Target project ID")
	_ = cmd.MarkFlagRequired("project")
	return cmd
}

// --- protection ---

func protectionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "protection <ng_uuid>",
		Short: "Enable or disable destruction protection",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			enable, _ := cmd.Flags().GetBool("enable")
			disable, _ := cmd.Flags().GetBool("disable")
			if enable == disable {
				return fmt.Errorf("exactly one of --enable or --disable is required")
			}

			body := map[string]interface{}{"enabled": enable}

			s := output.NewSpinner("Updating protection...")
			s.Start()
			resp, err := client.Post(fmt.Sprintf("/nat-gateway/%s/protection", args[0]), body)
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			if enable {
				output.PrintSuccess("Protection enabled")
			} else {
				output.PrintSuccess("Protection disabled")
			}
			return nil
		},
	}
	cmd.Flags().Bool("enable", false, "Enable destruction protection")
	cmd.Flags().Bool("disable", false, "Disable destruction protection")
	return cmd
}
