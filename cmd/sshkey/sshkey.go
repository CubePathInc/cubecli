package sshkey

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
	sshKeyCmd := &cobra.Command{
		Use:   "ssh-key",
		Short: "Manage SSH keys",
	}

	sshKeyCreateCmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new SSH key",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			name, _ := cmd.Flags().GetString("name")
			keyFile, _ := cmd.Flags().GetString("public-key-from-file")
			keyStr, _ := cmd.Flags().GetString("public-key")

			if keyFile == "" && keyStr == "" {
				return fmt.Errorf("one of --public-key-from-file (-f) or --public-key (-k) is required")
			}

			var sshKey string
			if keyFile != "" {
				data, err := os.ReadFile(keyFile)
				if err != nil {
					return fmt.Errorf("failed to read public key file: %w", err)
				}
				sshKey = strings.TrimSpace(string(data))
			} else {
				sshKey = strings.TrimSpace(keyStr)
			}

			body := map[string]string{
				"name":    name,
				"ssh_key": sshKey,
			}

			s := output.NewSpinner("Creating SSH key...")
			s.Start()
			resp, err := client.Post("/sshkey/create", body)
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			var result struct {
				ID          int    `json:"id"`
				Name        string `json:"name"`
				KeyType     string `json:"key_type"`
				Fingerprint string `json:"fingerprint"`
			}
			if err := json.Unmarshal(resp, &result); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}

			t := output.NewTable("SSH Key Created", []string{"Field", "Value"})
			t.AddRow("ID", strconv.Itoa(result.ID))
			t.AddRow("Name", result.Name)
			t.AddRow("Type", result.KeyType)
			t.AddRow("Fingerprint", result.Fingerprint)
			t.Render()

			output.PrintSuccess("SSH key created successfully")
			return nil
		},
	}

	sshKeyListCmd := &cobra.Command{
		Use:   "list",
		Short: "List SSH keys",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			s := output.NewSpinner("Fetching SSH keys...")
			s.Start()
			resp, err := client.Get("/sshkey/user/sshkeys")
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			var result struct {
				SSHKeys []struct {
					ID          int    `json:"id"`
					Name        string `json:"name"`
					KeyType     string `json:"key_type"`
					Fingerprint string `json:"fingerprint"`
				} `json:"sshkeys"`
			}
			if err := json.Unmarshal(resp, &result); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}

			t := output.NewTable("SSH Keys", []string{"ID", "Name", "Type", "Fingerprint"})
			for _, k := range result.SSHKeys {
				t.AddRow(strconv.Itoa(k.ID), k.Name, k.KeyType, k.Fingerprint)
			}
			t.Render()
			return nil
		},
	}

	sshKeyDeleteCmd := &cobra.Command{
		Use:   "delete <key_id>",
		Short: "Delete an SSH key",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			keyID, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid key_id: %s", args[0])
			}

			if !cmdutil.CheckForce(cmd, "Are you sure you want to delete this SSH key?") {
				output.PrintWarning("Aborted")
				return nil
			}

			s := output.NewSpinner("Deleting SSH key...")
			s.Start()
			resp, err := client.Delete(fmt.Sprintf("/sshkey/%d", keyID))
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			output.PrintSuccess("SSH key deleted successfully")
			return nil
		},
	}

	sshKeyCreateCmd.Flags().StringP("name", "n", "", "Name of the SSH key")
	sshKeyCreateCmd.Flags().StringP("public-key-from-file", "f", "", "Path to public key file")
	sshKeyCreateCmd.Flags().StringP("public-key", "k", "", "Public key string")
	sshKeyCreateCmd.MarkFlagRequired("name")

	sshKeyDeleteCmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")

	sshKeyCmd.AddCommand(sshKeyCreateCmd, sshKeyListCmd, sshKeyDeleteCmd)
	return sshKeyCmd
}
