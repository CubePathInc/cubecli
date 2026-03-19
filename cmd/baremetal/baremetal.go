package baremetal

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/CubePathInc/cubecli/internal/cmdutil"
	"github.com/CubePathInc/cubecli/internal/output"
	"github.com/spf13/cobra"
)

func NewCmd() *cobra.Command {
	baremetalCmd := &cobra.Command{
		Use:   "baremetal",
		Short: "Manage baremetal servers",
	}

	baremetalDeployCmd := &cobra.Command{
		Use:   "deploy",
		Short: "Deploy a new baremetal server",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			projectID, _ := cmd.Flags().GetInt("project")
			location, _ := cmd.Flags().GetString("location")
			model, _ := cmd.Flags().GetString("model")
			hostname, _ := cmd.Flags().GetString("hostname")
			password, _ := cmd.Flags().GetString("password")
			user, _ := cmd.Flags().GetString("user")
			label, _ := cmd.Flags().GetString("label")
			osName, _ := cmd.Flags().GetString("os")
			diskLayout, _ := cmd.Flags().GetString("disk-layout")
			sshKeys, _ := cmd.Flags().GetStringSlice("ssh")

			body := map[string]interface{}{
				"location_name": location,
				"model_name":    model,
				"hostname":      hostname,
				"user":          user,
				"password":      password,
			}
			if label != "" {
				body["label"] = label
			}
			if osName != "" {
				body["os_name"] = osName
			}
			if diskLayout != "" {
				body["disk_layout_name"] = diskLayout
			}
			if len(sshKeys) > 0 {
				body["ssh_key_names"] = sshKeys
			}

			s := output.NewSpinner("Deploying baremetal server...")
			s.Start()
			resp, err := client.Post(fmt.Sprintf("/baremetal/deploy/%d", projectID), body)
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			output.PrintSuccess("Baremetal server deployment initiated")
			return nil
		},
	}

	baremetalListCmd := &cobra.Command{
		Use:   "list",
		Short: "List baremetal servers",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			filterProject, _ := cmd.Flags().GetInt("project")
			filterLocation, _ := cmd.Flags().GetString("location")

			s := output.NewSpinner("Fetching baremetal servers...")
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
				Baremetals []struct {
					ID         int    `json:"id"`
					Hostname   string `json:"hostname"`
					Status     string `json:"status"`
					FloatingIPs []struct {
						Type           string `json:"type"`
						Address        string `json:"address"`
						ProtectionType string `json:"protection_type"`
					} `json:"floating_ips"`
					BaremetalModel struct {
						ModelName string `json:"model_name"`
					} `json:"baremetal_model"`
					OS struct {
						Name string `json:"name"`
					} `json:"os"`
					MonitoringEnable bool `json:"monitoring_enable"`
					Location         struct {
						LocationName string `json:"location_name"`
					} `json:"location"`
				} `json:"baremetals"`
			}
			if err := json.Unmarshal(resp, &projects); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}

			t := output.NewTable("Baremetal Servers", []string{"ID", "Hostname", "Project", "Status", "IP", "Model", "OS", "Monitoring", "Location"})
			for _, p := range projects {
				if filterProject > 0 && p.Project.ID != filterProject {
					continue
				}
				for _, bm := range p.Baremetals {
					if filterLocation != "" && !strings.EqualFold(bm.Location.LocationName, filterLocation) {
						continue
					}
					monitoring := "disabled"
					if bm.MonitoringEnable {
						monitoring = "enabled"
					}
					var ips []string
					for _, fip := range bm.FloatingIPs {
						ips = append(ips, fip.Address)
					}
					t.AddRow(
						strconv.Itoa(bm.ID),
						bm.Hostname,
						p.Project.Name,
						output.FormatStatus(bm.Status),
						strings.Join(ips, ", "),
						bm.BaremetalModel.ModelName,
						bm.OS.Name,
						output.FormatStatus(monitoring),
						bm.Location.LocationName,
					)
				}
			}
			t.Render()
			return nil
		},
	}

	baremetalShowCmd := &cobra.Command{
		Use:   "show <id>",
		Short: "Show baremetal server details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			bmID, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid baremetal id: %s", args[0])
			}

			s := output.NewSpinner("Fetching baremetal details...")
			s.Start()
			resp, err := client.Get("/projects/")
			s.Stop()
			if err != nil {
				return err
			}

			var projects []struct {
				Project struct {
					ID   int    `json:"id"`
					Name string `json:"name"`
				} `json:"project"`
				Baremetals []struct {
					ID       int    `json:"id"`
					Hostname string `json:"hostname"`
					Status   string `json:"status"`
					Label    string `json:"label"`
					FloatingIPs []struct {
						Type           string `json:"type"`
						Address        string `json:"address"`
						ProtectionType string `json:"protection_type"`
					} `json:"floating_ips"`
					BaremetalModel struct {
						ModelName string  `json:"model_name"`
						CPU       string  `json:"cpu"`
						CPUSpecs  string  `json:"cpu_specs"`
						CPUBench  float64 `json:"cpu_bench"`
						RAMSize   int     `json:"ram_size"`
						RAMType   string  `json:"ram_type"`
						DiskSize  string  `json:"disk_size"`
						DiskType  string  `json:"disk_type"`
						Port      int     `json:"port"`
						KVM       string  `json:"kvm"`
						Price     float64 `json:"price"`
					} `json:"baremetal_model"`
					OS struct {
						Name string `json:"name"`
					} `json:"os"`
					MonitoringEnable bool   `json:"monitoring_enable"`
					SSHUsername      string `json:"ssh_username"`
					SSHKey           struct {
						Name string `json:"name"`
					} `json:"ssh_key"`
					Location struct {
						LocationName string `json:"location_name"`
					} `json:"location"`
				} `json:"baremetals"`
			}
			if err := json.Unmarshal(resp, &projects); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}

			for _, p := range projects {
				for _, bm := range p.Baremetals {
					if bm.ID == bmID {
						if cmdutil.IsJSON(cmd) {
							return output.PrintJSON(bm)
						}

						monitoring := "disabled"
						if bm.MonitoringEnable {
							monitoring = "enabled"
						}

						// Basic Info
						basic := output.NewTable("Basic Info", []string{"Field", "Value"})
						basic.AddRow("ID", strconv.Itoa(bm.ID))
						basic.AddRow("Hostname", bm.Hostname)
						basic.AddRow("Project", p.Project.Name)
						basic.AddRow("Status", output.FormatStatus(bm.Status))
						basic.AddRow("Label", bm.Label)
						basic.AddRow("OS", bm.OS.Name)
						basic.AddRow("Monitoring", output.FormatStatus(monitoring))
						basic.AddRow("SSH Username", bm.SSHUsername)
						basic.AddRow("SSH Key", bm.SSHKey.Name)
						basic.AddRow("Location", bm.Location.LocationName)
						basic.Render()

						// Hardware Specs
						hw := output.NewTable("Hardware Specs", []string{"Field", "Value"})
						hw.AddRow("Model", bm.BaremetalModel.ModelName)
						hw.AddRow("CPU", bm.BaremetalModel.CPU)
						hw.AddRow("CPU Specs", bm.BaremetalModel.CPUSpecs)
						hw.AddRow("CPU Bench", fmt.Sprintf("%.0f", bm.BaremetalModel.CPUBench))
						hw.AddRow("RAM", fmt.Sprintf("%d GB %s", bm.BaremetalModel.RAMSize, bm.BaremetalModel.RAMType))
						hw.AddRow("Disk", fmt.Sprintf("%s %s", bm.BaremetalModel.DiskSize, bm.BaremetalModel.DiskType))
						hw.AddRow("Port", fmt.Sprintf("%d Mbps", bm.BaremetalModel.Port))
						hw.AddRow("KVM", bm.BaremetalModel.KVM)
						hw.AddRow("Price", fmt.Sprintf("$%.2f/mo", bm.BaremetalModel.Price))
						hw.Render()

						// Network Info
						net := output.NewTable("Network Info", []string{"Type", "Address", "Protection"})
						for _, fip := range bm.FloatingIPs {
							net.AddRow(fip.Type, fip.Address, fip.ProtectionType)
						}
						net.Render()

						return nil
					}
				}
			}

			return fmt.Errorf("baremetal server with ID %d not found", bmID)
		},
	}

	baremetalSensorsCmd := &cobra.Command{
		Use:   "sensors <id>",
		Short: "Show BMC sensor data for a baremetal server",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			bmID, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid baremetal id: %s", args[0])
			}

			s := output.NewSpinner("Fetching sensor data...")
			s.Start()
			resp, err := client.Get(fmt.Sprintf("/baremetal/%d/bmc-sensors", bmID))
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			var sensors struct {
				Node          string `json:"node"`
				IPMIAvailable bool   `json:"ipmi_available"`
				PowerOn       bool   `json:"power_on"`
				Sensors       struct {
					Temperatures []struct {
						Name  string  `json:"name"`
						Value float64 `json:"value"`
					} `json:"temperatures"`
					Fans []struct {
						Name  string  `json:"name"`
						Value float64 `json:"value"`
					} `json:"fans"`
				} `json:"sensors"`
			}
			if err := json.Unmarshal(resp, &sensors); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}

			ipmiStatus := "unavailable"
			if sensors.IPMIAvailable {
				ipmiStatus = "available"
			}
			powerStatus := "off"
			if sensors.PowerOn {
				powerStatus = "on"
			}

			info := output.NewTable("BMC Status", []string{"Field", "Value"})
			info.AddRow("Node", sensors.Node)
			info.AddRow("IPMI", output.FormatStatus(ipmiStatus))
			info.AddRow("Power", output.FormatStatus(powerStatus))
			info.Render()

			if len(sensors.Sensors.Temperatures) > 0 {
				tt := output.NewTable("Temperatures", []string{"Sensor", "Value"})
				for _, temp := range sensors.Sensors.Temperatures {
					tt.AddRow(temp.Name, fmt.Sprintf("%.1f", temp.Value))
				}
				tt.Render()
			}

			if len(sensors.Sensors.Fans) > 0 {
				ft := output.NewTable("Fans", []string{"Sensor", "Value"})
				for _, fan := range sensors.Sensors.Fans {
					ft.AddRow(fan.Name, fmt.Sprintf("%.0f RPM", fan.Value))
				}
				ft.Render()
			}

			return nil
		},
	}

	baremetalRescueCmd := &cobra.Command{
		Use:   "rescue <id>",
		Short: "Boot baremetal server into rescue mode",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			bmID, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid baremetal id: %s", args[0])
			}

			s := output.NewSpinner("Activating rescue mode...")
			s.Start()
			resp, err := client.Post(fmt.Sprintf("/baremetal/%d/rescue", bmID), nil)
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			var result struct {
				Detail   string `json:"detail"`
				Username string `json:"username"`
				Password string `json:"password"`
			}
			if err := json.Unmarshal(resp, &result); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}

			output.PrintSuccess("Rescue mode activated")
			t := output.NewTable("Rescue Credentials", []string{"Field", "Value"})
			t.AddRow("Detail", result.Detail)
			t.AddRow("Username", result.Username)
			t.AddRow("Password", result.Password)
			t.Render()

			return nil
		},
	}

	baremetalResetBMCCmd := &cobra.Command{
		Use:   "reset-bmc <id>",
		Short: "Reset the BMC of a baremetal server",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			bmID, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid baremetal id: %s", args[0])
			}

			if !cmdutil.CheckForce(cmd, "Are you sure you want to reset the BMC?") {
				output.PrintWarning("Aborted")
				return nil
			}

			s := output.NewSpinner("Resetting BMC...")
			s.Start()
			resp, err := client.Post(fmt.Sprintf("/baremetal/%d/reset-bmc", bmID), nil)
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			output.PrintSuccess("BMC reset successfully")
			return nil
		},
	}

	baremetalUpdateCmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update a baremetal server",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			bmID, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid baremetal id: %s", args[0])
			}

			body := map[string]interface{}{}
			if cmd.Flags().Changed("hostname") {
				hostname, _ := cmd.Flags().GetString("hostname")
				body["hostname"] = hostname
			}
			if cmd.Flags().Changed("tags") {
				tags, _ := cmd.Flags().GetString("tags")
				body["tags"] = tags
			}

			if len(body) == 0 {
				return fmt.Errorf("at least one of --hostname or --tags must be specified")
			}

			s := output.NewSpinner("Updating baremetal server...")
			s.Start()
			resp, err := client.Patch(fmt.Sprintf("/baremetal/update/%d", bmID), body)
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			output.PrintSuccess("Baremetal server updated successfully")
			return nil
		},
	}

	baremetalIPMICmd := &cobra.Command{
		Use:   "ipmi <id>",
		Short: "Create an IPMI proxy session",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			bmID, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid baremetal id: %s", args[0])
			}

			s := output.NewSpinner("Creating IPMI session...")
			s.Start()
			resp, err := client.Post(fmt.Sprintf("/ipmi-proxy/create-session/%d", bmID), nil)
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			var result struct {
				ProxyURL    string `json:"proxy_url"`
				Credentials struct {
					Username string `json:"username"`
					Password string `json:"password"`
				} `json:"credentials"`
			}
			if err := json.Unmarshal(resp, &result); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}

			output.PrintSuccess("IPMI session created")
			t := output.NewTable("IPMI Session", []string{"Field", "Value"})
			t.AddRow("Proxy URL", result.ProxyURL)
			t.AddRow("Username", result.Credentials.Username)
			t.AddRow("Password", result.Credentials.Password)
			t.Render()

			return nil
		},
	}

	// deploy flags
	baremetalDeployCmd.Flags().IntP("project", "p", 0, "Project ID")
	baremetalDeployCmd.Flags().StringP("location", "l", "", "Location name")
	baremetalDeployCmd.Flags().StringP("model", "m", "", "Model name")
	baremetalDeployCmd.Flags().String("hostname", "", "Hostname for the server")
	baremetalDeployCmd.Flags().String("password", "", "Password for the server")
	baremetalDeployCmd.Flags().StringP("user", "u", "root", "Username")
	baremetalDeployCmd.Flags().String("label", "", "Optional label")
	baremetalDeployCmd.Flags().String("os", "", "OS name")
	baremetalDeployCmd.Flags().String("disk-layout", "", "Disk layout name")
	baremetalDeployCmd.Flags().StringSliceP("ssh", "s", nil, "SSH key names")
	baremetalDeployCmd.MarkFlagRequired("project")
	baremetalDeployCmd.MarkFlagRequired("location")
	baremetalDeployCmd.MarkFlagRequired("model")
	baremetalDeployCmd.MarkFlagRequired("hostname")
	baremetalDeployCmd.MarkFlagRequired("password")

	// list flags
	baremetalListCmd.Flags().IntP("project", "p", 0, "Filter by project ID")
	baremetalListCmd.Flags().StringP("location", "l", "", "Filter by location")

	// update flags
	baremetalUpdateCmd.Flags().String("hostname", "", "New hostname")
	baremetalUpdateCmd.Flags().String("tags", "", "Tags for the server")

	// reset-bmc flags
	baremetalResetBMCCmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")

	baremetalCmd.AddCommand(
		baremetalDeployCmd,
		baremetalListCmd,
		baremetalShowCmd,
		baremetalSensorsCmd,
		baremetalRescueCmd,
		baremetalResetBMCCmd,
		baremetalUpdateCmd,
		baremetalIPMICmd,
	)

	addPowerCmd(baremetalCmd)
	addReinstallCmd(baremetalCmd)
	addMonitoringCmd(baremetalCmd)
	addModelCmd(baremetalCmd)

	return baremetalCmd
}
