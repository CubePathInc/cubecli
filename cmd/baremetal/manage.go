package baremetal

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/CubePathInc/cubecli/internal/cmdutil"
	"github.com/CubePathInc/cubecli/internal/output"
	"github.com/spf13/cobra"
)

// addManageCmds adds protection, project moves, SSH keys, the private
// network, the KVM console and the OS catalog of a server.
func addManageCmds(parent *cobra.Command) {
	parent.AddCommand(
		cmdutil.ProtectionCmd("id", "Baremetal", cmdutil.IntPath("/baremetal/%d/protection", "id")),
		cmdutil.MoveProjectCmd("id", "baremetal server", cmdutil.IntPath("/baremetal/%d/move-project", "id")),
		sshKeyCmd(),
		networkCmd(),
		kvmCmd(),
		osCmd(),
	)
}

func parseID(arg string) (int, error) {
	id, err := strconv.Atoi(arg)
	if err != nil {
		return 0, fmt.Errorf("invalid baremetal ID: %s", arg)
	}
	return id, nil
}

func sshKeyCmd() *cobra.Command {
	keyCmd := &cobra.Command{
		Use:   "ssh-key",
		Short: "Add or remove SSH keys of a baremetal server",
	}

	addCmd := &cobra.Command{
		Use:   "add <id> <ssh_key_id>...",
		Short: "Add SSH keys to a server (used by the next install)",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			bmID, err := parseID(args[0])
			if err != nil {
				return err
			}
			ids := make([]int, 0, len(args)-1)
			for _, a := range args[1:] {
				id, err := strconv.Atoi(a)
				if err != nil {
					return fmt.Errorf("invalid ssh_key_id: %s", a)
				}
				ids = append(ids, id)
			}
			client := cmdutil.GetClient(cmd)
			return cmdutil.RunDetail(cmd, "Adding SSH keys...", "SSH keys added", func() (json.RawMessage, error) {
				return client.Post(fmt.Sprintf("/baremetal/%d/ssh-keys", bmID), ids)
			})
		},
	}

	removeCmd := &cobra.Command{
		Use:   "remove <id> <ssh_key_id>",
		Short: "Remove an SSH key from a server",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			bmID, err := parseID(args[0])
			if err != nil {
				return err
			}
			keyID, err := strconv.Atoi(args[1])
			if err != nil {
				return fmt.Errorf("invalid ssh_key_id: %s", args[1])
			}
			client := cmdutil.GetClient(cmd)
			return cmdutil.RunDetail(cmd, "Removing SSH key...", "SSH key removed", func() (json.RawMessage, error) {
				return client.Delete(fmt.Sprintf("/baremetal/%d/ssh-keys/%d", bmID, keyID))
			})
		},
	}

	keyCmd.AddCommand(addCmd, removeCmd)
	return keyCmd
}

func networkCmd() *cobra.Command {
	netCmd := &cobra.Command{
		Use:   "network",
		Short: "Attach or detach the private network of a baremetal server",
	}

	attachCmd := &cobra.Command{
		Use:   "attach <id>",
		Short: "Attach a server to a private network in its location",
		Long: `Attach a baremetal server to a private network. An address is picked
automatically; restart the server to apply the change.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			bmID, err := parseID(args[0])
			if err != nil {
				return err
			}
			networkID, _ := cmd.Flags().GetInt("network")
			client := cmdutil.GetClient(cmd)
			return cmdutil.RunDetail(cmd, "Attaching private network...", "Private network attached", func() (json.RawMessage, error) {
				return client.Post(fmt.Sprintf("/baremetal/%d/network", bmID), map[string]int{"network_id": networkID})
			})
		},
	}
	attachCmd.Flags().Int("network", 0, "Private network ID")
	_ = attachCmd.MarkFlagRequired("network")

	detachCmd := &cobra.Command{
		Use:   "detach <id>",
		Short: "Detach a server from its private network",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			bmID, err := parseID(args[0])
			if err != nil {
				return err
			}
			if !cmdutil.CheckForce(cmd, fmt.Sprintf("Detach baremetal %d from its private network?", bmID)) {
				output.PrintWarning("Aborted")
				return nil
			}
			client := cmdutil.GetClient(cmd)
			return cmdutil.RunDetail(cmd, "Detaching private network...", "Private network detached", func() (json.RawMessage, error) {
				return client.Delete(fmt.Sprintf("/baremetal/%d/network", bmID))
			})
		},
	}
	detachCmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")

	netCmd.AddCommand(attachCmd, detachCmd)
	return netCmd
}

func kvmCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "kvm <id>",
		Short: "Show the KVM console URL and credentials of a server",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			bmID, err := parseID(args[0])
			if err != nil {
				return err
			}
			client := cmdutil.GetClient(cmd)

			s := output.NewSpinner("Fetching KVM access...")
			s.Start()
			resp, err := client.Get(fmt.Sprintf("/baremetal/%d/kvm", bmID))
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			var k struct {
				URL      string  `json:"url"`
				Username string  `json:"username"`
				Password *string `json:"password"`
			}
			if err := json.Unmarshal(resp, &k); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}
			password := "not set yet"
			if k.Password != nil {
				password = *k.Password
			}
			t := output.NewTable("KVM Console", []string{"Field", "Value"})
			t.AddRow("URL", k.URL)
			t.AddRow("Username", k.Username)
			t.AddRow("Password", password)
			t.Render()
			return nil
		},
	}
}

func osCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "os <id>",
		Short: "List the operating systems and disk layouts a server can be installed with",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			bmID, err := parseID(args[0])
			if err != nil {
				return err
			}
			client := cmdutil.GetClient(cmd)

			s := output.NewSpinner("Fetching operating systems...")
			s.Start()
			resp, err := client.Get(fmt.Sprintf("/baremetal/os/%d", bmID))
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			var list []struct {
				OSName          string `json:"os_name"`
				OperatingSystem string `json:"operating_system"`
				DiskLayouts     []struct {
					DiskLayoutName string `json:"disk_layout_name"`
					Name           string `json:"name"`
					RaidType       string `json:"raid_type"`
					DiskType       string `json:"disk_type"`
					DiskCount      int    `json:"disk_count"`
				} `json:"disk_layouts"`
			}
			if err := json.Unmarshal(resp, &list); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}
			t := output.NewTable("Operating Systems", []string{"OS", "Name", "Disk Layout", "RAID", "Disks"})
			for _, o := range list {
				if len(o.DiskLayouts) == 0 {
					t.AddRow(o.OSName, o.OperatingSystem, "-", "-", "-")
				}
				for _, d := range o.DiskLayouts {
					t.AddRow(o.OSName, o.OperatingSystem, d.DiskLayoutName, d.RaidType, fmt.Sprintf("%d x %s", d.DiskCount, d.DiskType))
				}
			}
			t.Render()
			return nil
		},
	}
}
