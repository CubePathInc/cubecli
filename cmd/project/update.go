package project

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/CubePathInc/cubecli/internal/cmdutil"
	"github.com/spf13/cobra"
)

func updateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update <project_id>",
		Short: "Rename a project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid project_id: %s", args[0])
			}
			name, _ := cmd.Flags().GetString("name")
			client := cmdutil.GetClient(cmd)
			return cmdutil.RunDetail(cmd, "Updating project...", "Project updated successfully", func() (json.RawMessage, error) {
				return client.Put(fmt.Sprintf("/projects/%d", projectID), map[string]string{"name": name})
			})
		},
	}
	cmd.Flags().StringP("name", "n", "", "New name (2-50 characters)")
	_ = cmd.MarkFlagRequired("name")
	return cmd
}
