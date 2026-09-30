package cmdutil

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/CubePathInc/cubecli/internal/output"
	"github.com/spf13/cobra"
)

// PathFunc turns the command's positional argument into an API path.
type PathFunc func(arg string) (string, error)

// IntPath returns a PathFunc for routes keyed by a numeric id, e.g. "/vps/%d/protection".
func IntPath(format, argName string) PathFunc {
	return func(arg string) (string, error) {
		id, err := strconv.Atoi(arg)
		if err != nil {
			return "", fmt.Errorf("invalid %s: %s", argName, arg)
		}
		return fmt.Sprintf(format, id), nil
	}
}

// StringPath returns a PathFunc for routes keyed by a uuid or name, e.g. "/loadbalancer/%s/protection".
func StringPath(format string) PathFunc {
	return func(arg string) (string, error) {
		return fmt.Sprintf(format, arg), nil
	}
}

// ProtectionCmd builds `protection <arg> --enable|--disable`, which posts
// {"enabled": bool} to path(arg). Protection blocks deletion of the resource.
func ProtectionCmd(argName, noun string, path PathFunc) *cobra.Command {
	cmd := &cobra.Command{
		Use:   fmt.Sprintf("protection <%s>", argName),
		Short: "Enable or disable destruction protection",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			enable, _ := cmd.Flags().GetBool("enable")
			disable, _ := cmd.Flags().GetBool("disable")
			if enable == disable {
				return fmt.Errorf("exactly one of --enable or --disable is required")
			}
			p, err := path(args[0])
			if err != nil {
				return err
			}

			client := GetClient(cmd)
			s := output.NewSpinner("Updating protection...")
			s.Start()
			resp, err := client.Post(p, map[string]interface{}{"enabled": enable})
			s.Stop()
			if err != nil {
				return err
			}

			if IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}
			if enable {
				output.PrintSuccess(noun + " protection enabled")
			} else {
				output.PrintSuccess(noun + " protection disabled")
			}
			return nil
		},
	}
	cmd.Flags().Bool("enable", false, "Enable destruction protection")
	cmd.Flags().Bool("disable", false, "Disable destruction protection")
	return cmd
}

// MoveProjectCmd builds `move-project <arg> --project N`, which posts
// {"project_id": N} to path(arg).
func MoveProjectCmd(argName, noun string, path PathFunc) *cobra.Command {
	cmd := &cobra.Command{
		Use:     fmt.Sprintf("move-project <%s>", argName),
		Aliases: []string{"move"},
		Short:   fmt.Sprintf("Move a %s to another project in the same organization", noun),
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, _ := cmd.Flags().GetInt("project")
			p, err := path(args[0])
			if err != nil {
				return err
			}

			client := GetClient(cmd)
			s := output.NewSpinner(fmt.Sprintf("Moving %s...", noun))
			s.Start()
			resp, err := client.Post(p, map[string]interface{}{"project_id": projectID})
			s.Stop()
			if err != nil {
				return err
			}

			if IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}
			output.PrintSuccess(fmt.Sprintf("Moved %s to project %d", noun, projectID))
			return nil
		},
	}
	cmd.Flags().IntP("project", "p", 0, "Target project ID")
	_ = cmd.MarkFlagRequired("project")
	return cmd
}

// ReadJSONFile decodes a JSON document from path, or from stdin when path is "-".
func ReadJSONFile(path string, v interface{}) error {
	var data []byte
	var err error
	if path == "-" {
		data, err = io.ReadAll(os.Stdin)
	} else {
		data, err = os.ReadFile(path)
	}
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", path, err)
	}
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("invalid JSON in %s: %w", path, err)
	}
	return nil
}

// YesNo renders a bool for tables.
func YesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

// RunDetail runs one request behind a spinner and prints the API's "detail"
// message (fallback when there is none), or the raw response with --json.
func RunDetail(cmd *cobra.Command, spin, fallback string, call func() (json.RawMessage, error)) error {
	s := output.NewSpinner(spin)
	s.Start()
	resp, err := call()
	s.Stop()
	if err != nil {
		return err
	}
	if IsJSON(cmd) {
		return output.PrintJSON(json.RawMessage(resp))
	}
	var r struct {
		Detail string `json:"detail"`
	}
	if json.Unmarshal(resp, &r) == nil && r.Detail != "" {
		output.PrintSuccess(r.Detail)
	} else {
		output.PrintSuccess(fallback)
	}
	return nil
}
