package vps

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/CubePathInc/cubecli/internal/cmdutil"
	"github.com/CubePathInc/cubecli/internal/output"
	"github.com/spf13/cobra"
)

func NewCmd() *cobra.Command {
	vpsCmd := &cobra.Command{
		Use:   "vps",
		Short: "Manage VPS instances",
	}

	vpsCreateCmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new VPS instance",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			name, _ := cmd.Flags().GetString("name")
			plan, _ := cmd.Flags().GetString("plan")
			template, _ := cmd.Flags().GetString("template")
			projectID, _ := cmd.Flags().GetInt("project")
			location, _ := cmd.Flags().GetString("location")
			sshKeys, _ := cmd.Flags().GetStringSlice("ssh")
			networkID, _ := cmd.Flags().GetInt("network")
			label, _ := cmd.Flags().GetString("label")
			password, _ := cmd.Flags().GetString("password")
			ipv4, _ := cmd.Flags().GetBool("ipv4")
			firewalls, _ := cmd.Flags().GetIntSlice("firewall")
			backups, _ := cmd.Flags().GetBool("backups")
			cloudinit, _ := cmd.Flags().GetString("cloudinit")

			if len(sshKeys) == 0 && password == "" {
				return fmt.Errorf("either --ssh or --password must be provided")
			}

			// If cloudinit looks like a file path, try reading it
			if cloudinit != "" && !strings.HasPrefix(cloudinit, "#") {
				data, err := os.ReadFile(cloudinit)
				if err == nil {
					cloudinit = string(data)
				}
			}

			body := map[string]interface{}{
				"name":           name,
				"plan_name":      plan,
				"template_name":  template,
				"location_name":  location,
				"ipv4":           ipv4,
				"enable_backups": backups,
			}
			if label != "" {
				body["label"] = label
			}
			if len(sshKeys) > 0 {
				body["ssh_key_names"] = sshKeys
			}
			if networkID != 0 {
				body["network_id"] = networkID
			}
			if password != "" {
				body["password"] = password
			}
			if len(firewalls) > 0 {
				body["firewall_group_ids"] = firewalls
			}
			if cloudinit != "" {
				body["custom_cloudinit"] = cloudinit
			}

			s := output.NewSpinner("Creating VPS...")
			s.Start()
			resp, err := client.Post(fmt.Sprintf("/vps/create/%d", projectID), body)
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			var result map[string]interface{}
			if err := json.Unmarshal(resp, &result); err != nil {
				return err
			}
			if detail, ok := result["detail"].(string); ok {
				output.PrintSuccess(detail)
			} else {
				output.PrintSuccess("VPS created successfully")
			}
			return nil
		},
	}

	vpsListCmd := &cobra.Command{
		Use:   "list",
		Short: "List VPS instances",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			projectFilter, _ := cmd.Flags().GetInt("project")
			locationFilter, _ := cmd.Flags().GetString("location")

			s := output.NewSpinner("Fetching VPS instances...")
			s.Start()
			resp, err := client.Get("/projects/")
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			var projects []struct {
				Project struct {
					ID   int    `json:"id"`
					Name string `json:"name"`
				} `json:"project"`
				VPS []struct {
					ID          int    `json:"id"`
					Name        string `json:"name"`
					Status      string `json:"status"`
					FloatingIPs struct {
						List []struct {
							Address string `json:"address"`
						} `json:"list"`
					} `json:"floating_ips"`
					Plan struct {
						Name string `json:"plan_name"`
					} `json:"plan"`
					Template struct {
						Name   string `json:"template_name"`
						OSName string `json:"os_name"`
					} `json:"template"`
					Location struct {
						Name string `json:"location_name"`
					} `json:"location"`
				} `json:"vps"`
			}
			if err := json.Unmarshal(resp, &projects); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}

			t := output.NewTable("VPS Instances", []string{"ID", "Name", "Project", "Status", "IP", "Plan", "OS", "Location"})
			for _, p := range projects {
				if projectFilter != 0 && p.Project.ID != projectFilter {
					continue
				}
				for _, v := range p.VPS {
					if locationFilter != "" && v.Location.Name != locationFilter {
						continue
					}
					ip := ""
					if len(v.FloatingIPs.List) > 0 {
						ip = v.FloatingIPs.List[0].Address
					}
					t.AddRow(
						strconv.Itoa(v.ID),
						v.Name,
						p.Project.Name,
						output.FormatStatus(v.Status),
						ip,
						v.Plan.Name,
						v.Template.OSName,
						v.Location.Name,
					)
				}
			}
			t.Render()
			return nil
		},
	}

	vpsShowCmd := &cobra.Command{
		Use:   "show <vps_id>",
		Short: "Show VPS details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			vpsID, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid VPS ID: %s", args[0])
			}

			s := output.NewSpinner("Fetching VPS details...")
			s.Start()
			resp, err := client.Get("/projects/")
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			var projects []struct {
				Project struct {
					ID   int    `json:"id"`
					Name string `json:"name"`
				} `json:"project"`
				VPS []struct {
					ID       int    `json:"id"`
					Name     string `json:"name"`
					Status   string `json:"status"`
					Username string `json:"username"`
					Label    string `json:"label"`
					SSHKeys  []struct {
						Name string `json:"name"`
					} `json:"ssh_keys"`
					FloatingIPs struct {
						List []struct {
							Address string `json:"address"`
						} `json:"list"`
					} `json:"floating_ips"`
					Plan struct {
						Name      string `json:"plan_name"`
						VCPUs     int    `json:"cpu"`
						RAM       int    `json:"ram"`
						Storage   int    `json:"storage"`
						Bandwidth int    `json:"bandwidth"`
					} `json:"plan"`
					Template struct {
						Name   string `json:"template_name"`
						OSName string `json:"os_name"`
					} `json:"template"`
					Location struct {
						Name string `json:"location_name"`
					} `json:"location"`
					IPv4    string `json:"ipv4"`
					IPv6    string `json:"ipv6"`
					Network struct {
						Name string `json:"name"`
					} `json:"network"`
				} `json:"vps"`
			}
			if err := json.Unmarshal(resp, &projects); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}

			for _, p := range projects {
				for _, v := range p.VPS {
					if v.ID != vpsID {
						continue
					}

					sshKeyNames := make([]string, len(v.SSHKeys))
					for i, k := range v.SSHKeys {
						sshKeyNames[i] = k.Name
					}

					floatingIPs := make([]string, len(v.FloatingIPs.List))
					for i, f := range v.FloatingIPs.List {
						floatingIPs[i] = f.Address
					}

					t1 := output.NewTable("VPS Details", []string{"Field", "Value"})
					t1.AddRow("Name", v.Name)
					t1.AddRow("ID", strconv.Itoa(v.ID))
					t1.AddRow("Status", output.FormatStatus(v.Status))
					t1.AddRow("Project", p.Project.Name)
					t1.AddRow("Location", v.Location.Name)
					t1.AddRow("OS", v.Template.OSName)
					t1.AddRow("Username", v.Username)
					t1.AddRow("SSH Keys", strings.Join(sshKeyNames, ", "))
					t1.AddRow("Label", v.Label)
					t1.Render()

					t2 := output.NewTable("Resources & Network", []string{"Field", "Value"})
					t2.AddRow("Plan", v.Plan.Name)
					t2.AddRow("vCPUs", strconv.Itoa(v.Plan.VCPUs))
					t2.AddRow("RAM", fmt.Sprintf("%d MB", v.Plan.RAM))
					t2.AddRow("Storage", fmt.Sprintf("%d GB", v.Plan.Storage))
					t2.AddRow("Bandwidth", fmt.Sprintf("%d GB", v.Plan.Bandwidth))
					t2.AddRow("IPv4", v.IPv4)
					t2.AddRow("IPv6", v.IPv6)
					t2.AddRow("Floating IPs", strings.Join(floatingIPs, ", "))
					t2.AddRow("Network", v.Network.Name)
					t2.Render()

					return nil
				}
			}

			return fmt.Errorf("VPS with ID %d not found", vpsID)
		},
	}

	vpsDestroyCmd := &cobra.Command{
		Use:   "destroy <vps_id>",
		Short: "Destroy a VPS instance",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			vpsID, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid VPS ID: %s", args[0])
			}

			releaseIPs, _ := cmd.Flags().GetBool("release-ips")
			keepIPs, _ := cmd.Flags().GetBool("keep-ips")

			if keepIPs {
				releaseIPs = false
			}

			if !cmdutil.CheckForce(cmd, fmt.Sprintf("Are you sure you want to destroy VPS %d?", vpsID)) {
				output.PrintInfo("Operation cancelled")
				return nil
			}

			body := map[string]interface{}{
				"release_ips": releaseIPs,
			}

			s := output.NewSpinner("Destroying VPS...")
			s.Start()
			resp, err := client.Post(fmt.Sprintf("/vps/destroy/%d", vpsID), body)
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			var result map[string]interface{}
			if err := json.Unmarshal(resp, &result); err != nil {
				return err
			}
			if detail, ok := result["detail"].(string); ok {
				output.PrintSuccess(detail)
			} else {
				output.PrintSuccess("VPS destroyed successfully")
			}
			return nil
		},
	}

	vpsUpdateCmd := &cobra.Command{
		Use:   "update <vps_id>",
		Short: "Update a VPS instance",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			vpsID, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid VPS ID: %s", args[0])
			}

			label, _ := cmd.Flags().GetString("label")
			name, _ := cmd.Flags().GetString("name")

			if label == "" && name == "" {
				return fmt.Errorf("at least one of --label or --name must be provided")
			}

			body := map[string]interface{}{}
			if label != "" {
				body["label"] = label
			}
			if name != "" {
				body["name"] = name
			}

			s := output.NewSpinner("Updating VPS...")
			s.Start()
			resp, err := client.Patch(fmt.Sprintf("/vps/update/%d", vpsID), body)
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			var result map[string]interface{}
			if err := json.Unmarshal(resp, &result); err != nil {
				return err
			}
			if detail, ok := result["detail"].(string); ok {
				output.PrintSuccess(detail)
			} else {
				output.PrintSuccess("VPS updated successfully")
			}
			return nil
		},
	}

	vpsResizeCmd := &cobra.Command{
		Use:   "resize <vps_id>",
		Short: "Resize a VPS instance",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			vpsID, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid VPS ID: %s", args[0])
			}

			plan, _ := cmd.Flags().GetString("plan")

			if !cmdutil.CheckForce(cmd, fmt.Sprintf("Are you sure you want to resize VPS %d to plan %s?", vpsID, plan)) {
				output.PrintInfo("Operation cancelled")
				return nil
			}

			s := output.NewSpinner("Resizing VPS...")
			s.Start()
			resp, err := client.Post(fmt.Sprintf("/vps/resize/vps_id/%d/resize_plan/%s", vpsID, plan), nil)
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			var result map[string]interface{}
			if err := json.Unmarshal(resp, &result); err != nil {
				return err
			}
			if detail, ok := result["detail"].(string); ok {
				output.PrintSuccess(detail)
			} else {
				output.PrintSuccess("VPS resized successfully")
			}
			return nil
		},
	}

	vpsChangePasswordCmd := &cobra.Command{
		Use:   "change-password <vps_id>",
		Short: "Change VPS root password",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			vpsID, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid VPS ID: %s", args[0])
			}

			password, _ := cmd.Flags().GetString("new-password")

			body := map[string]interface{}{
				"password": password,
			}

			s := output.NewSpinner("Changing password...")
			s.Start()
			resp, err := client.Post(fmt.Sprintf("/vps/%d/change-password", vpsID), body)
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			var result map[string]interface{}
			if err := json.Unmarshal(resp, &result); err != nil {
				return err
			}
			if detail, ok := result["detail"].(string); ok {
				output.PrintSuccess(detail)
			} else {
				output.PrintSuccess("Password changed successfully")
			}
			return nil
		},
	}

	vpsReinstallCmd := &cobra.Command{
		Use:   "reinstall <vps_id>",
		Short: "Reinstall a VPS with a new template",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			vpsID, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid VPS ID: %s", args[0])
			}

			template, _ := cmd.Flags().GetString("template")

			if !cmdutil.CheckForce(cmd, fmt.Sprintf("Are you sure you want to reinstall VPS %d with template %s? This will destroy all data.", vpsID, template)) {
				output.PrintInfo("Operation cancelled")
				return nil
			}

			body := map[string]interface{}{
				"template_name": template,
			}

			s := output.NewSpinner("Reinstalling VPS...")
			s.Start()
			resp, err := client.Post(fmt.Sprintf("/vps/reinstall/%d", vpsID), body)
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			var result map[string]interface{}
			if err := json.Unmarshal(resp, &result); err != nil {
				return err
			}
			if detail, ok := result["detail"].(string); ok {
				output.PrintSuccess(detail)
			} else {
				output.PrintSuccess("VPS reinstalled successfully")
			}
			return nil
		},
	}

	// vps create flags
	vpsCreateCmd.Flags().StringP("name", "n", "", "VPS name")
	vpsCreateCmd.Flags().StringP("plan", "p", "", "Plan name")
	vpsCreateCmd.Flags().StringP("template", "t", "", "Template name")
	vpsCreateCmd.Flags().Int("project", 0, "Project ID")
	vpsCreateCmd.Flags().StringP("location", "l", "", "Location name")
	vpsCreateCmd.Flags().StringSliceP("ssh", "s", nil, "SSH key names (repeatable)")
	vpsCreateCmd.Flags().Int("network", 0, "Network ID")
	vpsCreateCmd.Flags().String("label", "", "VPS label")
	vpsCreateCmd.Flags().String("password", "", "Root password")
	vpsCreateCmd.Flags().Bool("ipv4", true, "Enable IPv4")
	vpsCreateCmd.Flags().Bool("no-ipv4", false, "Disable IPv4")
	vpsCreateCmd.Flags().IntSlice("firewall", nil, "Firewall group IDs (repeatable)")
	vpsCreateCmd.Flags().Bool("backups", false, "Enable backups")
	vpsCreateCmd.Flags().Bool("no-backups", false, "Disable backups")
	vpsCreateCmd.Flags().StringP("cloudinit", "c", "", "Cloud-init configuration or file path")
	_ = vpsCreateCmd.MarkFlagRequired("name")
	_ = vpsCreateCmd.MarkFlagRequired("plan")
	_ = vpsCreateCmd.MarkFlagRequired("template")
	_ = vpsCreateCmd.MarkFlagRequired("project")
	_ = vpsCreateCmd.MarkFlagRequired("location")

	// vps list flags
	vpsListCmd.Flags().IntP("project", "p", 0, "Filter by project ID")
	vpsListCmd.Flags().StringP("location", "l", "", "Filter by location")

	// vps destroy flags
	vpsDestroyCmd.Flags().Bool("release-ips", false, "Release floating IPs")
	vpsDestroyCmd.Flags().Bool("keep-ips", false, "Keep floating IPs")
	vpsDestroyCmd.Flags().BoolP("force", "f", false, "Skip confirmation")

	// vps update flags
	vpsUpdateCmd.Flags().String("label", "", "New label")
	vpsUpdateCmd.Flags().StringP("name", "n", "", "New name")

	// vps resize flags
	vpsResizeCmd.Flags().StringP("plan", "p", "", "New plan name")
	vpsResizeCmd.Flags().BoolP("force", "f", false, "Skip confirmation")
	_ = vpsResizeCmd.MarkFlagRequired("plan")

	// vps change-password flags
	vpsChangePasswordCmd.Flags().StringP("new-password", "p", "", "New root password")
	_ = vpsChangePasswordCmd.MarkFlagRequired("new-password")

	// vps reinstall flags
	vpsReinstallCmd.Flags().StringP("template", "t", "", "Template name")
	vpsReinstallCmd.Flags().BoolP("force", "f", false, "Skip confirmation")
	_ = vpsReinstallCmd.MarkFlagRequired("template")

	// Add subcommands to vps
	vpsCmd.AddCommand(
		vpsCreateCmd,
		vpsListCmd,
		vpsShowCmd,
		vpsDestroyCmd,
		vpsUpdateCmd,
		vpsResizeCmd,
		vpsChangePasswordCmd,
		vpsReinstallCmd,
	)

	// Add subcommands from other files
	addPowerCmd(vpsCmd)
	addPlanCmd(vpsCmd)
	addTemplateCmd(vpsCmd)
	addBackupCmd(vpsCmd)
	addISOCmd(vpsCmd)

	return vpsCmd
}
