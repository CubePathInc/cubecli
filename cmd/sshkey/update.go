package sshkey

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/CubePathInc/cubecli/internal/cmdutil"
	"github.com/spf13/cobra"
)

func updateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update <key_id>",
		Short: "Rename an SSH key",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			keyID, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid key_id: %s", args[0])
			}
			name, _ := cmd.Flags().GetString("name")
			client := cmdutil.GetClient(cmd)
			return cmdutil.RunDetail(cmd, "Updating SSH key...", "SSH key updated successfully", func() (json.RawMessage, error) {
				return client.Put(fmt.Sprintf("/sshkey/%d", keyID), map[string]string{"name": name})
			})
		},
	}
	cmd.Flags().StringP("name", "n", "", "New name")
	_ = cmd.MarkFlagRequired("name")
	return cmd
}
