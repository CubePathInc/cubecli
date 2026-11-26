package cdn

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/CubePathInc/cubecli/internal/cmdutil"
	"github.com/CubePathInc/cubecli/internal/output"
	"github.com/spf13/cobra"
)

const (
	underAttackMinDuration = 5 * time.Minute
	underAttackMaxDuration = 24 * time.Hour
)

type underAttackState struct {
	Active      bool     `json:"active"`
	Until       *string  `json:"until"`
	Source      *string  `json:"source"`
	Level       *int     `json:"level"`
	ExemptPaths []string `json:"exempt_paths"`
}

func printUnderAttack(resp json.RawMessage) error {
	var s underAttackState
	if err := json.Unmarshal(resp, &s); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}
	if !s.Active {
		fmt.Println("Under attack mode: off")
		return nil
	}
	str := func(p *string) string {
		if p == nil || *p == "" {
			return "-"
		}
		return *p
	}
	level := "-"
	if s.Level != nil {
		level = fmt.Sprint(*s.Level)
	}
	exempt := "none (whole zone is challenged)"
	if len(s.ExemptPaths) > 0 {
		exempt = strings.Join(s.ExemptPaths, ", ")
	}
	fmt.Printf("Under attack mode: ON\n  Until:   %s\n  Source:  %s\n  Level:   %s\n  Exempt:  %s\n", str(s.Until), str(s.Source), level, exempt)
	return nil
}

// addUnderAttackCmd wires `cdn under-attack on|off|status <zone_uuid>`.
func addUnderAttackCmd(parent *cobra.Command) {
	uaCmd := &cobra.Command{
		Use:   "under-attack",
		Short: "Turn the under attack mode (JS challenge for visitors) of a zone on or off",
		Long: `The under attack mode makes the edge send visitors a JavaScript challenge until it
expires or you turn it off. Browsers solve it once; clients that cannot run
JavaScript (APIs, webhooks, hotlinks, bots) receive an HTML 503 page unless
their path is exempt. Non-GET requests and websockets are rate limited instead
of challenged. A visitor whose IPv4 changes has to solve it again.`,
	}

	onCmd := &cobra.Command{
		Use:   "on <zone_uuid>",
		Short: "Turn the under attack mode on for a duration",
		Long: `Turn the under attack mode on. The mode turns itself off when --for expires.

Choose the paths that must not be challenged with --exempt (repeat the flag).
Without --exempt the whole zone would be challenged and every API client or
webhook sender would get a 503, so the command refuses unless you pass --whole-zone.`,
		Example: `  cubecli cdn under-attack on <zone_uuid> --for 30m --exempt '/api/*' --exempt '/webhook/*'
  cubecli cdn under-attack on <zone_uuid> --for 2h --level 3 --whole-zone`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			d, _ := cmd.Flags().GetDuration("for")
			if d < underAttackMinDuration || d > underAttackMaxDuration {
				return fmt.Errorf("--for must be between 5m and 24h (for example 30m or 2h)")
			}
			if d%time.Minute != 0 {
				return fmt.Errorf("--for must be a whole number of minutes")
			}
			level, _ := cmd.Flags().GetInt("level")
			if level != 2 && level != 3 {
				return fmt.Errorf("--level must be 2 or 3")
			}
			exempt, _ := cmd.Flags().GetStringArray("exempt")
			whole, _ := cmd.Flags().GetBool("whole-zone")
			clean := []string{}
			for _, p := range exempt {
				p = strings.TrimSpace(p)
				if p == "" {
					continue
				}
				if !strings.HasPrefix(p, "/") {
					return fmt.Errorf("--exempt %q must start with /", p)
				}
				clean = append(clean, p)
			}
			if len(clean) == 0 && !whole {
				return fmt.Errorf("refusing to challenge the whole zone: APIs, webhooks, hotlinks and bots that cannot run JavaScript would get an HTML 503.\nPass --exempt <path> (repeatable, e.g. --exempt '/api/*' --exempt '/webhook/*') or confirm with --whole-zone")
			}
			body := map[string]interface{}{
				"duration_minutes":   int(d / time.Minute),
				"level":              level,
				"exempt_paths":       clean,
				"confirm_whole_zone": whole && len(clean) == 0,
			}
			s := output.NewSpinner("Turning under attack mode on...")
			s.Start()
			resp, err := client.Post(fmt.Sprintf("/cdn/zones/%s/under-attack", args[0]), body)
			s.Stop()
			if err != nil {
				return err
			}
			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}
			return printUnderAttack(resp)
		},
	}
	onCmd.Flags().Duration("for", 0, "How long the mode stays on, 5m to 24h (required, for example 30m)")
	onCmd.Flags().Int("level", 2, "Challenge level: 2 or 3")
	onCmd.Flags().StringArray("exempt", nil, "Path that is not challenged, for example /api/* (repeatable)")
	onCmd.Flags().Bool("whole-zone", false, "Confirm that the whole zone is challenged, with no exempt paths")
	_ = onCmd.MarkFlagRequired("for")

	offCmd := &cobra.Command{
		Use:     "off <zone_uuid>",
		Short:   "Turn the under attack mode off",
		Example: `  cubecli cdn under-attack off <zone_uuid>`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			s := output.NewSpinner("Turning under attack mode off...")
			s.Start()
			resp, err := client.Delete(fmt.Sprintf("/cdn/zones/%s/under-attack", args[0]))
			s.Stop()
			if err != nil {
				return err
			}
			if cmdutil.IsJSON(cmd) {
				if len(resp) == 0 {
					return output.PrintJSON(map[string]interface{}{"active": false})
				}
				return output.PrintJSON(json.RawMessage(resp))
			}
			fmt.Println("Under attack mode: off")
			return nil
		},
	}

	statusCmd := &cobra.Command{
		Use:     "status <zone_uuid>",
		Short:   "Show whether the under attack mode is on",
		Example: `  cubecli cdn under-attack status <zone_uuid> --json`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			s := output.NewSpinner("Fetching under attack status...")
			s.Start()
			resp, err := client.Get(fmt.Sprintf("/cdn/zones/%s/under-attack", args[0]))
			s.Stop()
			if err != nil {
				return err
			}
			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}
			return printUnderAttack(resp)
		},
	}

	uaCmd.AddCommand(onCmd, offCmd, statusCmd)
	parent.AddCommand(uaCmd)
}
