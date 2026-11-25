package cdn

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/CubePathInc/cubecli/internal/cmdutil"
	"github.com/CubePathInc/cubecli/internal/output"
	"github.com/spf13/cobra"
)

var wafCategoryPattern = regexp.MustCompile(`^[a-z0-9_-]{1,32}$`)

// addManagedWafFlags adds the managed WAF (detection) flags to `cdn zone update`.
func addManagedWafFlags(cmd *cobra.Command) {
	cmd.Flags().String("waf-mode", "", "Managed WAF detection mode: off or log (log only, nothing is blocked)")
	cmd.Flags().Int("waf-level", 0, "Managed WAF sensitivity level: 1 or 2")
	cmd.Flags().StringSlice("waf-exclude-rule-ids", nil, "Rule ids the managed WAF ignores (replaces the list, up to 256; pass an empty value to clear)")
	cmd.Flags().StringSlice("waf-exclude-categories", nil, "Categories the managed WAF ignores (replaces the list, up to 64; pass an empty value to clear)")
}

// managedWafBody builds the managed_waf object of a zone update from the --waf-* flags, or nil when none is set.
func managedWafBody(cmd *cobra.Command) (map[string]interface{}, error) {
	waf := map[string]interface{}{}
	if cmd.Flags().Changed("waf-mode") {
		mode, _ := cmd.Flags().GetString("waf-mode")
		if mode != "off" && mode != "log" {
			return nil, fmt.Errorf("--waf-mode must be off or log")
		}
		waf["mode"] = mode
	}
	if cmd.Flags().Changed("waf-level") {
		level, _ := cmd.Flags().GetInt("waf-level")
		if level != 1 && level != 2 {
			return nil, fmt.Errorf("--waf-level must be 1 or 2")
		}
		waf["level"] = level
	}
	if cmd.Flags().Changed("waf-exclude-rule-ids") {
		// A string slice, not an int slice: an empty value (--waf-exclude-rule-ids "") clears the list, and an int
		// slice would fail to parse it.
		raw, _ := cmd.Flags().GetStringSlice("waf-exclude-rule-ids")
		ids := []int{}
		for _, item := range raw {
			item = strings.TrimSpace(item)
			if item == "" {
				continue
			}
			id, err := strconv.Atoi(item)
			if err != nil {
				return nil, fmt.Errorf("--waf-exclude-rule-ids: %q is not a number", item)
			}
			ids = append(ids, id)
		}
		seen := map[int]bool{}
		for _, id := range ids {
			if id < 1 || int64(id) > 4294967295 {
				return nil, fmt.Errorf("--waf-exclude-rule-ids: %d is not between 1 and 4294967295", id)
			}
			if seen[id] {
				return nil, fmt.Errorf("--waf-exclude-rule-ids: %d is repeated", id)
			}
			seen[id] = true
		}
		if len(ids) > 256 {
			return nil, fmt.Errorf("--waf-exclude-rule-ids: at most 256 ids")
		}
		waf["exclude_rule_ids"] = ids
	}
	if cmd.Flags().Changed("waf-exclude-categories") {
		cats, _ := cmd.Flags().GetStringSlice("waf-exclude-categories")
		clean := []string{}
		seen := map[string]bool{}
		for _, c := range cats {
			if c == "" {
				continue
			}
			if !wafCategoryPattern.MatchString(c) {
				return nil, fmt.Errorf("--waf-exclude-categories: %q must match [a-z0-9_-], 1 to 32 characters", c)
			}
			if seen[c] {
				return nil, fmt.Errorf("--waf-exclude-categories: %q is repeated", c)
			}
			seen[c] = true
			clean = append(clean, c)
		}
		if len(clean) > 64 {
			return nil, fmt.Errorf("--waf-exclude-categories: at most 64 categories")
		}
		waf["exclude_categories"] = clean
	}
	if len(waf) == 0 {
		return nil, nil
	}
	return waf, nil
}

type securityEvent struct {
	Timestamp string  `json:"timestamp"`
	Action    string  `json:"action"`
	Reason    string  `json:"reason"`
	RuleName  *string `json:"rule_name"`
	Status    int     `json:"status"`
	ClientIP  *string `json:"client_ip"`
	Country   string  `json:"country"`
	ASN       int     `json:"asn"`
	Host      string  `json:"host"`
	Method    string  `json:"method"`
	Path      string  `json:"path"`
	RequestID string  `json:"request_id"`
}

func orDash(s *string) string {
	if s == nil || *s == "" {
		return "-"
	}
	return *s
}

// addEventsCmd wires `cdn events <zone_uuid>`.
func addEventsCmd(parent *cobra.Command) {
	cmd := &cobra.Command{
		Use:   "events <zone_uuid>",
		Short: "List the blocked, rate limited and challenged requests of a zone",
		Long: `List the security events of a zone: requests that were blocked, rate limited or
(with --action challenged) sent to a challenge. Events are kept for 30 days and
start the day the feature was enabled. Only the path of the request is stored,
never the query string.

Visitor IPs are shown only to members with the permission to see them; for the
others the IP column is "-".`,
		Example: `  cubecli cdn events <zone_uuid> --minutes 1440 --action blocked
  cubecli cdn events <zone_uuid> --country ES --path-prefix /admin --all`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			all, _ := cmd.Flags().GetBool("all")
			maxPages := 1
			if all {
				maxPages = 20
			}
			cursor, _ := cmd.Flags().GetString("cursor")

			var events []securityEvent
			var raw []json.RawMessage
			var next string
			for page := 0; page < maxPages; page++ {
				q, err := eventsQuery(cmd, cursor)
				if err != nil {
					return err
				}
				s := output.NewSpinner("Fetching security events...")
				s.Start()
				resp, err := client.Get(fmt.Sprintf("/cdn/zones/%s/security-events?%s", args[0], q))
				s.Stop()
				if err != nil {
					return err
				}
				var p struct {
					Events     []securityEvent `json:"events"`
					NextCursor *string         `json:"next_cursor"`
				}
				if err := json.Unmarshal(resp, &p); err != nil {
					return fmt.Errorf("failed to parse response: %w", err)
				}
				var rawPage struct {
					Events []json.RawMessage `json:"events"`
				}
				_ = json.Unmarshal(resp, &rawPage)
				events = append(events, p.Events...)
				raw = append(raw, rawPage.Events...)
				next = ""
				if p.NextCursor != nil {
					next = *p.NextCursor
				}
				if next == "" {
					break
				}
				cursor = next
			}

			if cmdutil.IsJSON(cmd) {
				out := map[string]interface{}{"events": raw, "next_cursor": nil}
				if raw == nil {
					out["events"] = []json.RawMessage{}
				}
				if next != "" {
					out["next_cursor"] = next
				}
				return output.PrintJSON(out)
			}

			t := output.NewTable("Security Events", []string{"Time", "Action", "Reason", "Rule", "Status", "IP", "Country", "ASN", "Request"})
			for _, e := range events {
				t.AddRow(e.Timestamp, e.Action, e.Reason, orDash(e.RuleName), strconv.Itoa(e.Status), orDash(e.ClientIP), e.Country, strconv.Itoa(e.ASN), e.Method+" "+e.Host+e.Path)
			}
			t.Render()
			if next != "" {
				fmt.Printf("More events available: --cursor %s\n", next)
			}
			return nil
		},
	}
	cmd.Flags().IntP("minutes", "m", 60, "Time range in minutes (1 to 43200)")
	cmd.Flags().IntP("limit", "l", 50, "Events per page (1 to 200)")
	cmd.Flags().String("cursor", "", "Continue from the cursor of a previous page")
	cmd.Flags().Bool("all", false, "Follow the cursor and print up to 20 pages")
	cmd.Flags().String("action", "", "blocked, rate_limited or challenged (challenged is left out by default)")
	cmd.Flags().String("reason", "", "Block reason, for example blocked_ip")
	cmd.Flags().String("rule", "", "UUID of the rule that acted")
	cmd.Flags().String("country", "", "Two letter country code")
	cmd.Flags().Int("asn", 0, "Autonomous system number")
	cmd.Flags().String("client-ip", "", "Visitor IP (needs the permission to see visitor IPs)")
	cmd.Flags().String("path-prefix", "", "Path starts with")
	cmd.Flags().Int("status", 0, "Response status code")
	cmd.Flags().String("request-id", "", "Request id")
	parent.AddCommand(cmd)
}

func eventsQuery(cmd *cobra.Command, cursor string) (string, error) {
	q := url.Values{}
	minutes, _ := cmd.Flags().GetInt("minutes")
	limit, _ := cmd.Flags().GetInt("limit")
	if minutes < 1 || minutes > 43200 {
		return "", fmt.Errorf("--minutes must be 1 to 43200")
	}
	if limit < 1 || limit > 200 {
		return "", fmt.Errorf("--limit must be 1 to 200")
	}
	q.Set("minutes", strconv.Itoa(minutes))
	q.Set("limit", strconv.Itoa(limit))
	if cursor != "" {
		q.Set("cursor", cursor)
	}
	action, _ := cmd.Flags().GetString("action")
	switch action {
	case "", "blocked", "rate_limited", "challenged":
	default:
		return "", fmt.Errorf("--action must be blocked, rate_limited or challenged")
	}
	for flag, key := range map[string]string{"action": "action", "reason": "reason", "rule": "rule_uuid", "client-ip": "client_ip", "path-prefix": "path_prefix", "request-id": "request_id"} {
		if v, _ := cmd.Flags().GetString(flag); v != "" {
			q.Set(key, v)
		}
	}
	if v, _ := cmd.Flags().GetString("country"); v != "" {
		q.Set("country", strings.ToUpper(v))
	}
	for _, flag := range []string{"asn", "status"} {
		if v, _ := cmd.Flags().GetInt(flag); v != 0 {
			q.Set(flag, strconv.Itoa(v))
		}
	}
	return q.Encode(), nil
}

// addWafMetricsCmds adds `cdn metrics waf` and `cdn metrics waf-events`.
func addWafMetricsCmds(metricsCmd *cobra.Command) {
	wafCmd := &cobra.Command{
		Use:   "waf <zone_uuid>",
		Short: "Show what the managed WAF (detection) found",
		Long: `Show what the managed WAF found in a zone: requests evaluated, flagged, the ones
that would be blocked (score 5 or more, indicative) and the rules, paths and
countries seen most. The managed WAF only detects and logs: it does not block,
does not inspect the body and does not evaluate what is served from cache.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			minutes, _ := cmd.Flags().GetInt("minutes")
			s := output.NewSpinner("Fetching managed WAF metrics...")
			s.Start()
			resp, err := client.Get(fmt.Sprintf("/cdn/zones/%s/metrics/waf?minutes=%d", args[0], minutes))
			s.Stop()
			if err != nil {
				return err
			}
			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}
			var m struct {
				Ruleset    *string `json:"ruleset"`
				Evaluated  int     `json:"evaluated"`
				Flagged    int     `json:"flagged"`
				WouldBlock int     `json:"would_block"`
				Skipped    int     `json:"skipped"`
				TopRules   []struct {
					RuleID   string  `json:"rule_id"`
					Category *string `json:"category"`
					Hits     int     `json:"hits"`
				} `json:"top_rules"`
				TopPaths []struct {
					Path string `json:"path"`
					Hits int    `json:"hits"`
				} `json:"top_paths"`
				TopCountries []struct {
					Country string `json:"country"`
					Hits    int    `json:"hits"`
				} `json:"top_countries"`
			}
			if err := json.Unmarshal(resp, &m); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}
			t := output.NewTable("Managed WAF (detection, log only)", []string{"Field", "Value"})
			t.AddRow("Rule set", orDash(m.Ruleset))
			t.AddRow("Evaluated", strconv.Itoa(m.Evaluated))
			t.AddRow("Flagged", strconv.Itoa(m.Flagged))
			t.AddRow("Would block (indicative)", strconv.Itoa(m.WouldBlock))
			t.AddRow("Not evaluated", strconv.Itoa(m.Skipped))
			t.Render()
			if len(m.TopRules) > 0 {
				r := output.NewTable("Top rules", []string{"Rule", "Category", "Hits"})
				for _, x := range m.TopRules {
					r.AddRow(x.RuleID, orDash(x.Category), strconv.Itoa(x.Hits))
				}
				r.Render()
			}
			if len(m.TopPaths) > 0 {
				r := output.NewTable("Top paths", []string{"Path", "Hits"})
				for _, x := range m.TopPaths {
					r.AddRow(x.Path, strconv.Itoa(x.Hits))
				}
				r.Render()
			}
			if len(m.TopCountries) > 0 {
				r := output.NewTable("Top countries", []string{"Country", "Hits"})
				for _, x := range m.TopCountries {
					r.AddRow(x.Country, strconv.Itoa(x.Hits))
				}
				r.Render()
			}
			return nil
		},
	}
	eventsCmd := &cobra.Command{
		Use:   "waf-events <zone_uuid>",
		Short: "List the requests the managed WAF flagged",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			minutes, _ := cmd.Flags().GetInt("minutes")
			limit, _ := cmd.Flags().GetInt("limit")
			q := url.Values{"minutes": {strconv.Itoa(minutes)}, "limit": {strconv.Itoa(limit)}}
			if v, _ := cmd.Flags().GetInt("min-score"); v > 0 {
				q.Set("min_score", strconv.Itoa(v))
			}
			if v, _ := cmd.Flags().GetString("rule-id"); v != "" {
				q.Set("rule_id", v)
			}
			s := output.NewSpinner("Fetching managed WAF events...")
			s.Start()
			resp, err := client.Get(fmt.Sprintf("/cdn/zones/%s/metrics/waf/events?%s", args[0], q.Encode()))
			s.Stop()
			if err != nil {
				return err
			}
			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}
			var p struct {
				Events []struct {
					Timestamp  string   `json:"timestamp"`
					Score      int      `json:"score"`
					WouldBlock bool     `json:"would_block"`
					RuleIDs    []string `json:"rule_ids"`
					Method     string   `json:"method"`
					Host       string   `json:"host"`
					Path       string   `json:"path"`
					Status     int      `json:"status"`
					Country    string   `json:"country"`
					ClientIP   *string  `json:"client_ip"`
				} `json:"events"`
			}
			if err := json.Unmarshal(resp, &p); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}
			t := output.NewTable("Managed WAF events", []string{"Time", "Score", "Would block", "Rules", "Status", "IP", "Country", "Request"})
			for _, e := range p.Events {
				t.AddRow(e.Timestamp, strconv.Itoa(e.Score), cmdutil.YesNo(e.WouldBlock), strings.Join(e.RuleIDs, ","), strconv.Itoa(e.Status), orDash(e.ClientIP), e.Country, e.Method+" "+e.Host+e.Path)
			}
			t.Render()
			return nil
		},
	}
	wafCmd.Flags().IntP("minutes", "m", 60, "Time range in minutes (1 to 1440)")
	eventsCmd.Flags().IntP("minutes", "m", 60, "Time range in minutes (1 to 1440)")
	eventsCmd.Flags().IntP("limit", "l", 50, "Maximum number of events (1 to 200)")
	eventsCmd.Flags().Int("min-score", 0, "Only requests with at least this score")
	eventsCmd.Flags().String("rule-id", "", "Only requests that matched this rule id")
	metricsCmd.AddCommand(wafCmd, eventsCmd)
}
