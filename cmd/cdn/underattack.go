package cdn

import (
	"encoding/json"
	"fmt"
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
	Active bool    `json:"active"`
	Until  *string `json:"until"`
	Source *string `json:"source"`
	Level  *int    `json:"level"`
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
	until := "no end (until it is turned off)"
	if s.Until != nil && *s.Until != "" {
		until = *s.Until
	}
	fmt.Printf("Under attack mode: ON\n  Until:   %s\n  Source:  %s\n  Level:   %s\n", until, str(s.Source), level)
	return nil
}

// addUnderAttackCmd wires `cdn under-attack on|off|status <zone_uuid>`.
func addUnderAttackCmd(parent *cobra.Command) {
	uaCmd := &cobra.Command{
		Use:   "under-attack",
		Short: "Turn the under attack mode (JS challenge for visitors) of a zone on or off",
		Long: `The under attack mode makes the edge send visitors a JavaScript challenge until you
turn it off, or until the duration you set expires. Browsers solve it once;
clients that cannot run JavaScript (APIs, webhooks, hotlinks, bots) receive an
HTML 503 page unless a bypass WAF rule lets them through. Non-GET requests and
websockets are rate limited instead of challenged. A visitor whose IPv4 changes
has to solve it again.`,
	}

	onCmd := &cobra.Command{
		Use:   "on <zone_uuid>",
		Short: "Turn the under attack mode on, with an optional end",
		Long: `Turn the under attack mode on. Without --duration it stays on until you run
"under-attack off". With --duration (5m to 24h) it turns itself off when it expires.

Requests that must not be challenged (APIs, webhooks, health checks) are
excepted with a WAF rule of type bypass, not with a flag here:
  cubecli cdn waf create <zone_uuid> --type bypass ...`,
		Example: `  cubecli cdn under-attack on <zone_uuid>
  cubecli cdn under-attack on <zone_uuid> --duration 2h --level 3`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			level, _ := cmd.Flags().GetInt("level")
			if level != 2 && level != 3 {
				return fmt.Errorf("--level must be 2 or 3")
			}
			body := map[string]interface{}{"level": level}
			if cmd.Flags().Changed("duration") {
				d, _ := cmd.Flags().GetDuration("duration")
				if d < underAttackMinDuration || d > underAttackMaxDuration {
					return fmt.Errorf("--duration must be between 5m and 24h (for example 30m or 2h)")
				}
				if d%time.Minute != 0 {
					return fmt.Errorf("--duration must be a whole number of minutes")
				}
				body["duration_minutes"] = int(d / time.Minute)
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
	onCmd.Flags().Duration("duration", 0, "How long the mode stays on, 5m to 24h (for example 30m). Default: until turned off")
	onCmd.Flags().Int("level", 2, "Challenge level: 2 or 3")

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
