package vps

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/CubePathInc/cubecli/internal/cmdutil"
	"github.com/CubePathInc/cubecli/internal/output"
	"github.com/spf13/cobra"
)

// addManageCmds adds protection, project moves, SSH keys, the private network
// and the console.
func addManageCmds(parent *cobra.Command) {
	parent.AddCommand(
		cmdutil.ProtectionCmd("vps_id", "VPS", cmdutil.IntPath("/vps/%d/protection", "vps_id")),
		cmdutil.MoveProjectCmd("vps_id", "VPS", cmdutil.IntPath("/vps/%d/move-project", "vps_id")),
		sshKeyCmd(),
		networkCmd(),
		consoleCmd(),
	)
}

func parseVPSID(arg string) (int, error) {
	id, err := strconv.Atoi(arg)
	if err != nil {
		return 0, fmt.Errorf("invalid vps_id: %s", arg)
	}
	return id, nil
}

func sshKeyCmd() *cobra.Command {
	keyCmd := &cobra.Command{
		Use:   "ssh-key",
		Short: "Add or remove SSH keys of a VPS",
	}

	addCmd := &cobra.Command{
		Use:   "add <vps_id> <ssh_key_id>...",
		Short: "Add SSH keys to a VPS",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			vpsID, err := parseVPSID(args[0])
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
				return client.Post(fmt.Sprintf("/vps/%d/ssh-keys", vpsID), ids)
			})
		},
	}

	removeCmd := &cobra.Command{
		Use:   "remove <vps_id> <ssh_key_id>",
		Short: "Remove an SSH key from a VPS",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			vpsID, err := parseVPSID(args[0])
			if err != nil {
				return err
			}
			keyID, err := strconv.Atoi(args[1])
			if err != nil {
				return fmt.Errorf("invalid ssh_key_id: %s", args[1])
			}
			client := cmdutil.GetClient(cmd)
			return cmdutil.RunDetail(cmd, "Removing SSH key...", "SSH key removed", func() (json.RawMessage, error) {
				return client.Delete(fmt.Sprintf("/vps/%d/ssh-keys/%d", vpsID, keyID))
			})
		},
	}

	keyCmd.AddCommand(addCmd, removeCmd)
	return keyCmd
}

func networkCmd() *cobra.Command {
	netCmd := &cobra.Command{
		Use:   "network",
		Short: "Attach or detach the private network of a VPS",
	}

	attachCmd := &cobra.Command{
		Use:   "attach <vps_id>",
		Short: "Attach a VPS to a private network in its location",
		Long: `Attach a VPS to a private network. An address is picked automatically
(see 'vps show'); restart the VPS to apply the change.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			vpsID, err := parseVPSID(args[0])
			if err != nil {
				return err
			}
			networkID, _ := cmd.Flags().GetInt("network")
			client := cmdutil.GetClient(cmd)
			return cmdutil.RunDetail(cmd, "Attaching private network...", "Private network attached", func() (json.RawMessage, error) {
				return client.Post(fmt.Sprintf("/vps/%d/network", vpsID), map[string]int{"network_id": networkID})
			})
		},
	}
	attachCmd.Flags().Int("network", 0, "Private network ID")
	_ = attachCmd.MarkFlagRequired("network")

	detachCmd := &cobra.Command{
		Use:   "detach <vps_id>",
		Short: "Detach a VPS from its private network",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			vpsID, err := parseVPSID(args[0])
			if err != nil {
				return err
			}
			if !cmdutil.CheckForce(cmd, fmt.Sprintf("Detach VPS %d from its private network?", vpsID)) {
				output.PrintWarning("Aborted")
				return nil
			}
			client := cmdutil.GetClient(cmd)
			return cmdutil.RunDetail(cmd, "Detaching private network...", "Private network detached", func() (json.RawMessage, error) {
				return client.Delete(fmt.Sprintf("/vps/%d/network", vpsID))
			})
		},
	}
	detachCmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")

	netCmd.AddCommand(attachCmd, detachCmd)
	return netCmd
}

func consoleCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "console <vps_id>",
		Short: "Open a VNC console session (valid for 5 minutes)",
		Long: `Open a VNC console session on a running VPS. Connect a noVNC client to the
WebSocket URL and use the ticket as the VNC password within 5 minutes.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			vpsID, err := parseVPSID(args[0])
			if err != nil {
				return err
			}
			client := cmdutil.GetClient(cmd)

			s := output.NewSpinner("Opening console...")
			s.Start()
			resp, err := client.Post(fmt.Sprintf("/vps/%d/vnc-url", vpsID), nil)
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			var r struct {
				WebsocketURL string `json:"websocket_url"`
				SessionID    string `json:"session_id"`
				VNCInfo      struct {
					Ticket string `json:"ticket"`
				} `json:"vnc_info"`
			}
			if err := json.Unmarshal(resp, &r); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}
			t := output.NewTable("VNC Console", []string{"Field", "Value"})
			t.AddRow("WebSocket URL", r.WebsocketURL)
			t.AddRow("Session", r.SessionID)
			t.AddRow("Password", r.VNCInfo.Ticket)
			t.Render()
			return nil
		},
	}
}
