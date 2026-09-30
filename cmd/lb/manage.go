package lb

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/CubePathInc/cubecli/internal/cmdutil"
	"github.com/CubePathInc/cubecli/internal/output"
	"github.com/spf13/cobra"
)

// addManageCmds adds protection and project moves.
func addManageCmds(parent *cobra.Command) {
	parent.AddCommand(
		cmdutil.ProtectionCmd("lb_uuid", "Load balancer", cmdutil.StringPath("/loadbalancer/%s/protection")),
		cmdutil.MoveProjectCmd("lb_uuid", "load balancer", cmdutil.StringPath("/loadbalancer/%s/move-project")),
	)
}

// parseBatchTarget reads "<type>:<id>[:port[:weight]]".
func parseBatchTarget(spec string) (map[string]interface{}, error) {
	parts := strings.Split(spec, ":")
	if len(parts) < 2 || len(parts) > 4 || parts[0] == "" || parts[1] == "" {
		return nil, fmt.Errorf("invalid target %q: expected <type>:<id>[:port[:weight]]", spec)
	}
	t := map[string]interface{}{
		"target_type": strings.ReplaceAll(strings.ToLower(parts[0]), "-", "_"),
		"target_uuid": parts[1],
	}
	if len(parts) > 2 && parts[2] != "" {
		port, err := strconv.Atoi(parts[2])
		if err != nil {
			return nil, fmt.Errorf("invalid port in target %q", spec)
		}
		t["port"] = port
	}
	if len(parts) > 3 && parts[3] != "" {
		weight, err := strconv.Atoi(parts[3])
		if err != nil {
			return nil, fmt.Errorf("invalid weight in target %q", spec)
		}
		t["weight"] = weight
	}
	return t, nil
}

func targetBatchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add-batch <lb_uuid> <listener_uuid>",
		Short: "Add up to 50 targets to a listener at once",
		Long: `Add several targets in one operation; nothing is added if any of them is
invalid. Give each with --target <type>:<id>[:port[:weight]], where type is vps,
baremetal or availability_group, or pass a JSON array with --file.`,
		Example: `  cubecli lb target add-batch <lb_uuid> <listener_uuid> --target vps:101 --target vps:102:8080:50`,
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			specs, _ := cmd.Flags().GetStringArray("target")
			file, _ := cmd.Flags().GetString("file")
			if (len(specs) > 0) == (file != "") {
				return fmt.Errorf("exactly one of --target or --file is required")
			}
			var targets []map[string]interface{}
			if file != "" {
				if err := cmdutil.ReadJSONFile(file, &targets); err != nil {
					return err
				}
			}
			for _, spec := range specs {
				t, err := parseBatchTarget(spec)
				if err != nil {
					return err
				}
				targets = append(targets, t)
			}

			s := output.NewSpinner("Adding targets...")
			s.Start()
			resp, err := client.Post(fmt.Sprintf("/loadbalancer/%s/listeners/%s/targets/batch", args[0], args[1]), map[string]interface{}{"targets": targets})
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			var r struct {
				Detail  string `json:"detail"`
				Targets []struct {
					UUID       string `json:"uuid"`
					TargetType string `json:"target_type"`
					TargetUUID string `json:"target_uuid"`
					TargetName string `json:"target_name"`
				} `json:"targets"`
			}
			if err := json.Unmarshal(resp, &r); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}
			t := output.NewTable("Targets Added", []string{"UUID", "Type", "Target", "Name"})
			for _, tg := range r.Targets {
				t.AddRow(tg.UUID, tg.TargetType, tg.TargetUUID, tg.TargetName)
			}
			t.Render()
			output.PrintSuccess(r.Detail)
			return nil
		},
	}
	cmd.Flags().StringArray("target", nil, "Target as <type>:<id>[:port[:weight]] (repeatable)")
	cmd.Flags().String("file", "", "JSON array of {target_type, target_uuid, port, weight, enabled} (\"-\" for stdin)")
	return cmd
}
