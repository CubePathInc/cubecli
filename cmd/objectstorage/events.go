package objectstorage

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/CubePathInc/cubecli/internal/api"
	"github.com/CubePathInc/cubecli/internal/cmdutil"
	"github.com/CubePathInc/cubecli/internal/output"
	"github.com/spf13/cobra"
)

const destinationsPath = "/object-storage/event-destinations"

// eventTypes maps the short names accepted by --events to the API event types.
var eventTypes = map[string]string{
	"created": "object.created",
	"removed": "object.removed",
	"tagging": "object.tagging",
}

// eventDestination is the Destination object of the API. The webhook URL is only ever masked.
type eventDestination struct {
	UUID        string  `json:"uuid"`
	Name        string  `json:"name"`
	Type        string  `json:"type"`
	URLMasked   *string `json:"url_masked"`
	Notificator *struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Type string `json:"type"`
	} `json:"notificator"`
	PayloadFormat           string  `json:"payload_format"`
	Status                  string  `json:"status"`
	DisabledReason          *string `json:"disabled_reason"`
	PreviousSecretExpiresAt *string `json:"previous_secret_expires_at"`
	LastSuccessAt           *string `json:"last_success_at"`
	LastFailureAt           *string `json:"last_failure_at"`
	LastError               *string `json:"last_error"`
	RulesCount              int     `json:"rules_count"`
	CreatedAt               *string `json:"created_at"`
}

// target is the masked URL of a webhook or the channel of a notificator destination.
func (d eventDestination) target() string {
	if d.Notificator != nil {
		return fmt.Sprintf("%s (%s)", d.Notificator.Name, d.Notificator.Type)
	}
	return strOr(d.URLMasked, "-")
}

type eventRule struct {
	UUID        string `json:"uuid"`
	Name        string `json:"name"`
	BucketUUID  string `json:"bucket_uuid"`
	Destination *struct {
		UUID string `json:"uuid"`
		Name string `json:"name"`
		Type string `json:"type"`
	} `json:"destination"`
	Events       []string `json:"events"`
	Prefix       string   `json:"prefix"`
	Suffix       string   `json:"suffix"`
	Enabled      bool     `json:"enabled"`
	Status       string   `json:"status"`
	ErrorMessage *string  `json:"error_message"`
}

func strOr(s *string, def string) string {
	if s == nil || *s == "" {
		return def
	}
	return *s
}

// parseEvents turns --events created,removed,tagging (or the full object.* names) into API types.
func parseEvents(values []string) ([]string, error) {
	seen := map[string]bool{}
	var out []string
	for _, v := range values {
		v = strings.ToLower(strings.TrimSpace(v))
		if v == "" {
			continue
		}
		t, ok := eventTypes[strings.TrimPrefix(v, "object.")]
		if !ok {
			return nil, fmt.Errorf("unknown event %q: use created, removed or tagging", v)
		}
		if !seen[t] {
			seen[t] = true
			out = append(out, t)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("--events needs at least one of created, removed, tagging")
	}
	return out, nil
}

func eventsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "events",
		Aliases: []string{"event", "notifications"},
		Short:   "Send bucket events to a signed webhook or a Slack/Discord channel",
		Long: `Event notifications send what happens in a bucket (objects created, removed or
tagged) to a destination:

  webhook      an https URL; every delivery is signed with the destination's secret
  notificator  a Slack or Discord channel of Cloud Alerts ('cubecli alert notificator')

Create a destination once, then add rules to buckets that pick the events, an
optional key prefix and suffix, and the destination.

Webhook deliveries carry CubePath-Timestamp and CubePath-Signature headers. The
signature is v1=<hex HMAC-SHA256 of timestamp + "." + raw body> with the signing
secret; for 24 hours after a rotation it is "v1=<new>, v1=<previous>". Reject deliveries
whose timestamp is more than 5 minutes old.`,
	}
	cmd.AddCommand(destinationCmd(), ruleCmd())
	return cmd
}

// --- destinations ---

func destinationCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "destination",
		Aliases: []string{"destinations", "dest"},
		Short:   "Manage event destinations (webhooks and channels)",
	}
	cmd.AddCommand(destListCmd(), destGetCmd(), destCreateCmd(), destUpdateCmd(), destDeleteCmd(),
		destRotateCmd(), destTestCmd(), destDeliveriesCmd())
	return cmd
}

// resolveDestination returns the uuid of a destination given its uuid or its name.
func resolveDestination(client *api.Client, ref string) (string, error) {
	if isUUID(ref) {
		return ref, nil
	}
	resp, err := client.Get(destinationsPath)
	if err != nil {
		return "", err
	}
	var dests []eventDestination
	if err := json.Unmarshal(resp, &dests); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}
	var matches []string
	for _, d := range dests {
		if d.Name == ref {
			matches = append(matches, d.UUID)
		}
	}
	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		return "", fmt.Errorf("event destination %q not found", ref)
	default:
		return "", fmt.Errorf("several event destinations are named %q; use the uuid", ref)
	}
}

func renderDestination(d eventDestination) {
	t := output.NewTable("Event Destination", []string{"Field", "Value"})
	t.AddRow("UUID", d.UUID)
	t.AddRow("Name", d.Name)
	t.AddRow("Type", d.Type)
	t.AddRow("Target", d.target())
	t.AddRow("Payload format", d.PayloadFormat)
	status := output.FormatStatus(d.Status)
	if d.DisabledReason != nil && *d.DisabledReason != "" {
		status += " (" + *d.DisabledReason + ")"
	}
	t.AddRow("Status", status)
	t.AddRow("Rules", strconv.Itoa(d.RulesCount))
	t.AddRow("Last success", strOr(d.LastSuccessAt, "-"))
	t.AddRow("Last failure", strOr(d.LastFailureAt, "-"))
	if d.LastError != nil && *d.LastError != "" {
		t.AddRow("Last error", *d.LastError)
	}
	if d.PreviousSecretExpiresAt != nil && *d.PreviousSecretExpiresAt != "" {
		t.AddRow("Previous secret signs until", *d.PreviousSecretExpiresAt)
	}
	t.AddRow("Created", strOr(d.CreatedAt, "-"))
	t.Render()
}

// printSecretResponse renders a create or rotate-secret answer, the only ones with the secret.
func printSecretResponse(cmd *cobra.Command, resp json.RawMessage) error {
	if cmdutil.IsJSON(cmd) {
		return output.PrintJSON(resp)
	}
	var out struct {
		Destination             eventDestination `json:"destination"`
		SigningSecret           *string          `json:"signing_secret"`
		PreviousSecretExpiresAt *string          `json:"previous_secret_expires_at"`
	}
	if err := json.Unmarshal(resp, &out); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}
	renderDestination(out.Destination)
	if out.SigningSecret != nil && *out.SigningSecret != "" {
		fmt.Printf("\nSigning secret: %s\n", *out.SigningSecret)
		output.PrintWarning("Copy the signing secret now: it will not be shown again.")
	}
	if out.PreviousSecretExpiresAt != nil && *out.PreviousSecretExpiresAt != "" {
		output.PrintInfo("The previous secret keeps signing until " + *out.PreviousSecretExpiresAt + " (UTC).")
	}
	return nil
}

func destListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List event destinations",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			s := output.NewSpinner("Fetching event destinations...")
			s.Start()
			resp, err := client.Get(destinationsPath)
			s.Stop()
			if err != nil {
				return err
			}
			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}
			var dests []eventDestination
			if err := json.Unmarshal(resp, &dests); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}
			t := output.NewTable("Event Destinations", []string{"UUID", "Name", "Type", "Target", "Format", "Status", "Rules", "Last success", "Last failure"})
			for _, d := range dests {
				t.AddRow(d.UUID, d.Name, d.Type, d.target(), d.PayloadFormat, output.FormatStatus(d.Status),
					strconv.Itoa(d.RulesCount), strOr(d.LastSuccessAt, "-"), strOr(d.LastFailureAt, "-"))
			}
			t.Render()
			return nil
		},
	}
}

func destGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <destination>",
		Short: "Show an event destination (uuid or name)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			s := output.NewSpinner("Fetching event destination...")
			s.Start()
			uuid, err := resolveDestination(client, args[0])
			var resp json.RawMessage
			if err == nil {
				resp, err = client.Get(destinationsPath + "/" + url.PathEscape(uuid))
			}
			s.Stop()
			if err != nil {
				return err
			}
			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(resp)
			}
			var d eventDestination
			if err := json.Unmarshal(resp, &d); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}
			renderDestination(d)
			return nil
		},
	}
}

func checkFormat(format string) error {
	if format != "" && format != "cubepath" && format != "s3" {
		return fmt.Errorf("--format must be cubepath or s3")
	}
	return nil
}

func destCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a webhook or channel destination; the signing secret is shown only once",
		Long: `Create an event destination.

--webhook takes an https URL on port 443 or 8443 that resolves to a public
address. The answer carries the signing secret (whsec_...): it is shown only
now, store it where your receiver can read it.

--channel takes the id of a Slack or Discord channel of Cloud Alerts
('cubecli alert notificator list'). Channel deliveries are not signed.

--format picks the payload: cubepath (default) or s3 ({"Records":[...]} in the
AWS shape, to reuse existing handlers).`,
		Example: `  cubecli s3 events destination create --name uploads-hook --webhook https://example.com/hooks/storage
  cubecli s3 events destination create --name ops --channel 7c1e0d2a --format cubepath`,
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			webhook, _ := cmd.Flags().GetString("webhook")
			channel, _ := cmd.Flags().GetString("channel")
			format, _ := cmd.Flags().GetString("format")
			if (webhook == "") == (channel == "") {
				return fmt.Errorf("give either --webhook <url> or --channel <notificator id>")
			}
			if err := checkFormat(format); err != nil {
				return err
			}
			body := map[string]interface{}{"name": name}
			if webhook != "" {
				body["type"] = "webhook"
				body["url"] = webhook
			} else {
				body["type"] = "notificator"
				body["notificator_id"] = channel
			}
			if format != "" {
				body["payload_format"] = format
			}

			client := cmdutil.GetClient(cmd)
			s := output.NewSpinner("Creating event destination...")
			s.Start()
			resp, err := client.Post(destinationsPath, body)
			s.Stop()
			if err != nil {
				return err
			}
			return printSecretResponse(cmd, resp)
		},
	}
	cmd.Flags().StringP("name", "n", "", "Destination name")
	cmd.Flags().String("webhook", "", "Webhook URL (https)")
	cmd.Flags().String("channel", "", "Cloud Alerts notificator id (Slack or Discord)")
	cmd.Flags().String("format", "", "Payload format: cubepath (default) or s3")
	_ = cmd.MarkFlagRequired("name")
	cmd.MarkFlagsMutuallyExclusive("webhook", "channel")
	return cmd
}

func destUpdateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update <destination>",
		Short: "Rename, change the URL or format, enable or disable a destination",
		Args:  cobra.ExactArgs(1),
		Example: `  cubecli s3 events destination update uploads-hook --webhook https://example.com/v2/hooks
  cubecli s3 events destination update uploads-hook --enable`,
		RunE: func(cmd *cobra.Command, args []string) error {
			body := map[string]interface{}{}
			if cmd.Flags().Changed("name") {
				v, _ := cmd.Flags().GetString("name")
				body["name"] = v
			}
			if cmd.Flags().Changed("webhook") {
				v, _ := cmd.Flags().GetString("webhook")
				body["url"] = v
			}
			if cmd.Flags().Changed("format") {
				v, _ := cmd.Flags().GetString("format")
				if err := checkFormat(v); err != nil {
					return err
				}
				body["payload_format"] = v
			}
			if on, _ := cmd.Flags().GetBool("enable"); on {
				body["enabled"] = true
			}
			if off, _ := cmd.Flags().GetBool("disable"); off {
				body["enabled"] = false
			}
			if len(body) == 0 {
				return fmt.Errorf("nothing to update: give --name, --webhook, --format, --enable or --disable")
			}

			client := cmdutil.GetClient(cmd)
			s := output.NewSpinner("Updating event destination...")
			s.Start()
			uuid, err := resolveDestination(client, args[0])
			var resp json.RawMessage
			if err == nil {
				resp, err = client.Patch(destinationsPath+"/"+url.PathEscape(uuid), body)
			}
			s.Stop()
			if err != nil {
				return err
			}
			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(resp)
			}
			var d eventDestination
			if err := json.Unmarshal(resp, &d); err == nil && d.UUID != "" {
				renderDestination(d)
				return nil
			}
			output.PrintSuccess("Event destination updated")
			return nil
		},
	}
	cmd.Flags().StringP("name", "n", "", "New name")
	cmd.Flags().String("webhook", "", "New webhook URL (webhook destinations only)")
	cmd.Flags().String("format", "", "Payload format: cubepath or s3")
	cmd.Flags().Bool("enable", false, "Enable the destination (also after it was disabled for failing)")
	cmd.Flags().Bool("disable", false, "Disable the destination: events for it are dropped")
	cmd.MarkFlagsMutuallyExclusive("enable", "disable")
	return cmd
}

func destDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <destination>",
		Short: "Delete an event destination (it must have no rules)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmdutil.CheckForce(cmd, fmt.Sprintf("Delete event destination %s?", args[0])) {
				output.PrintWarning("Aborted")
				return nil
			}
			client := cmdutil.GetClient(cmd)
			s := output.NewSpinner("Deleting event destination...")
			s.Start()
			uuid, err := resolveDestination(client, args[0])
			var resp json.RawMessage
			if err == nil {
				resp, err = client.Delete(destinationsPath + "/" + url.PathEscape(uuid))
			}
			s.Stop()
			if err != nil {
				return err
			}
			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(resp)
			}
			output.PrintSuccess("Event destination deleted")
			return nil
		},
	}
	cmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")
	return cmd
}

func destRotateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rotate-secret <destination>",
		Short: "Issue a new signing secret; the previous one keeps signing for 24 hours",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmdutil.CheckForce(cmd, fmt.Sprintf("Rotate the signing secret of %s? The current secret stops signing in 24 hours.", args[0])) {
				output.PrintWarning("Aborted")
				return nil
			}
			client := cmdutil.GetClient(cmd)
			s := output.NewSpinner("Rotating signing secret...")
			s.Start()
			uuid, err := resolveDestination(client, args[0])
			var resp json.RawMessage
			if err == nil {
				resp, err = client.Post(destinationsPath+"/"+url.PathEscape(uuid)+"/rotate-secret", nil)
			}
			s.Stop()
			if err != nil {
				return err
			}
			return printSecretResponse(cmd, resp)
		},
	}
	cmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")
	return cmd
}

func destTestCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "test <destination>",
		Short: "Send a cubepath.ping test event to a destination",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			s := output.NewSpinner("Sending test event...")
			s.Start()
			uuid, err := resolveDestination(client, args[0])
			var resp json.RawMessage
			if err == nil {
				resp, err = client.Post(destinationsPath+"/"+url.PathEscape(uuid)+"/test", nil)
			}
			s.Stop()
			if err != nil {
				return err
			}
			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(resp)
			}
			output.PrintSuccess("Test event sent; check the result with 'cubecli s3 events destination deliveries " + args[0] + "'")
			return nil
		},
	}
}

// delivery is one row of the delivery history.
type delivery struct {
	TS         string  `json:"ts"`
	TSMs       int64   `json:"ts_ms"`
	EventID    string  `json:"event_id"`
	DeliveryID string  `json:"delivery_id"`
	EventType  string  `json:"event_type"`
	BucketUUID string  `json:"bucket_uuid"`
	BucketName *string `json:"bucket_name"`
	RuleUUID   string  `json:"rule_uuid"`
	ObjectKey  string  `json:"object_key"`
	Attempt    int     `json:"attempt"`
	Status     string  `json:"status"`
	HTTPStatus int     `json:"http_status"`
	LatencyMS  int64   `json:"latency_ms"`
	Error      string  `json:"error"`
}

// deliveriesPage is the GET .../deliveries answer: newest first, next_before pages back.
type deliveriesPage struct {
	Deliveries []delivery `json:"deliveries"`
	NextBefore *int64     `json:"next_before"`
}

// parseBefore takes unix milliseconds (what next_before gives) or a UTC time and returns
// unix milliseconds.
func parseBefore(v string) (string, error) {
	if _, err := strconv.ParseInt(v, 10, 64); err == nil {
		return v, nil
	}
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02T15:04:05", "2006-01-02"} {
		if t, err := time.Parse(layout, v); err == nil {
			return strconv.FormatInt(t.UnixMilli(), 10), nil
		}
	}
	return "", fmt.Errorf("invalid --before %q: use unix milliseconds (next_before) or a UTC time such as 2026-10-01T00:00:00", v)
}

func destDeliveriesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deliveries <destination>",
		Short: "Show the latest deliveries of a destination, newest first",
		Long: `Show the delivery history of a destination (90 days), newest first.

failed is an attempt that is retried later; dead is an event given up after
the last retry. When the page is full the next (older) page is printed as a
--before value.`,
		Args: cobra.ExactArgs(1),
		Example: `  cubecli s3 events destination deliveries uploads-hook --status failed
  cubecli s3 events destination deliveries uploads-hook --limit 200 --before 1790964001250`,
		RunE: func(cmd *cobra.Command, args []string) error {
			status, _ := cmd.Flags().GetString("status")
			limit, _ := cmd.Flags().GetInt("limit")
			before, _ := cmd.Flags().GetString("before")
			if status != "" && status != "success" && status != "failed" && status != "dead" {
				return fmt.Errorf("--status must be success, failed or dead")
			}
			if limit < 0 || limit > 200 {
				return fmt.Errorf("--limit must be between 1 and 200")
			}
			q := url.Values{}
			if status != "" {
				q.Set("status", status)
			}
			if limit > 0 {
				q.Set("limit", strconv.Itoa(limit))
			}
			if before != "" {
				ms, err := parseBefore(before)
				if err != nil {
					return err
				}
				q.Set("before", ms)
			}

			client := cmdutil.GetClient(cmd)
			s := output.NewSpinner("Fetching deliveries...")
			s.Start()
			uuid, err := resolveDestination(client, args[0])
			var resp json.RawMessage
			if err == nil {
				path := destinationsPath + "/" + url.PathEscape(uuid) + "/deliveries"
				if len(q) > 0 {
					path += "?" + q.Encode()
				}
				resp, err = client.Get(path)
			}
			s.Stop()
			if err != nil {
				return err
			}
			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(resp)
			}
			var page deliveriesPage
			if err := json.Unmarshal(resp, &page); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}
			t := output.NewTable("Deliveries", []string{"Time", "Event", "Bucket", "Object", "Attempt", "Status", "HTTP", "Latency", "Error"})
			for _, d := range page.Deliveries {
				httpStatus, errMsg := "-", "-"
				if d.HTTPStatus > 0 {
					httpStatus = strconv.Itoa(d.HTTPStatus)
				}
				if d.Error != "" {
					errMsg = d.Error
				}
				t.AddRow(d.TS, d.EventType, strOr(d.BucketName, "-"), d.ObjectKey, strconv.Itoa(d.Attempt),
					output.FormatStatus(d.Status), httpStatus, strconv.FormatInt(d.LatencyMS, 10)+" ms", errMsg)
			}
			t.Render()
			if page.NextBefore != nil {
				fmt.Printf("\nOlder deliveries: --before %d\n", *page.NextBefore)
			}
			return nil
		},
	}
	cmd.Flags().String("status", "", "Only success, failed or dead deliveries")
	cmd.Flags().Int("limit", 0, "Number of deliveries, 1 to 200 (default 50)")
	cmd.Flags().String("before", "", "Only deliveries before this point: unix milliseconds (as printed) or a UTC time")
	return cmd
}

// --- rules ---

func ruleCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "rule",
		Aliases: []string{"rules"},
		Short:   "Manage the event rules of a bucket",
	}
	cmd.AddCommand(ruleListCmd(), ruleCreateCmd(), ruleUpdateCmd(), ruleDeleteCmd())
	return cmd
}

func rulesPath(bucketUUID string) string {
	return "/object-storage/buckets/" + url.PathEscape(bucketUUID) + "/event-rules"
}

// resolveRule returns the bucket uuid and the rule uuid given the bucket (name or uuid) and the
// rule (uuid or name).
func resolveRule(client *api.Client, bucket, ref string) (string, string, error) {
	bucketUUID, err := resolveBucket(client, bucket)
	if err != nil {
		return "", "", err
	}
	if isUUID(ref) {
		return bucketUUID, ref, nil
	}
	resp, err := client.Get(rulesPath(bucketUUID))
	if err != nil {
		return "", "", err
	}
	var rules []eventRule
	if err := json.Unmarshal(resp, &rules); err != nil {
		return "", "", fmt.Errorf("failed to parse response: %w", err)
	}
	var matches []string
	for _, r := range rules {
		if r.Name == ref {
			matches = append(matches, r.UUID)
		}
	}
	switch len(matches) {
	case 1:
		return bucketUUID, matches[0], nil
	case 0:
		return "", "", fmt.Errorf("event rule %q not found in bucket %s", ref, bucket)
	default:
		return "", "", fmt.Errorf("several event rules are named %q; use the uuid", ref)
	}
}

func shortEvents(events []string) string {
	out := make([]string, len(events))
	for i, e := range events {
		out[i] = strings.TrimPrefix(e, "object.")
	}
	return strings.Join(out, ",")
}

func renderRules(title string, rules []eventRule) {
	t := output.NewTable(title, []string{"UUID", "Name", "Destination", "Events", "Prefix", "Suffix", "Enabled", "Status"})
	for _, r := range rules {
		status := output.FormatStatus(r.Status)
		if r.ErrorMessage != nil && *r.ErrorMessage != "" {
			status += ": " + *r.ErrorMessage
		}
		prefix, suffix := r.Prefix, r.Suffix
		if prefix == "" {
			prefix = "-"
		}
		if suffix == "" {
			suffix = "-"
		}
		dest := "-"
		if r.Destination != nil {
			dest = r.Destination.Name
		}
		t.AddRow(r.UUID, r.Name, dest, shortEvents(r.Events), prefix, suffix, yesNo(r.Enabled), status)
	}
	t.Render()
}

func ruleListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List the event rules of a bucket",
		RunE: func(cmd *cobra.Command, args []string) error {
			bucket, _ := cmd.Flags().GetString("bucket")
			client := cmdutil.GetClient(cmd)
			s := output.NewSpinner("Fetching event rules...")
			s.Start()
			bucketUUID, err := resolveBucket(client, bucket)
			var resp json.RawMessage
			if err == nil {
				resp, err = client.Get(rulesPath(bucketUUID))
			}
			s.Stop()
			if err != nil {
				return err
			}
			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(resp)
			}
			var rules []eventRule
			if err := json.Unmarshal(resp, &rules); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}
			renderRules("Event Rules", rules)
			return nil
		},
	}
	cmd.Flags().StringP("bucket", "b", "", "Bucket name or uuid")
	_ = cmd.MarkFlagRequired("bucket")
	return cmd
}

func ruleCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Send a bucket's events to a destination",
		Long: `Create an event rule on a bucket. --events takes created, removed and/or
tagging. --prefix and --suffix limit the rule to matching object keys.

The rule is applied in the background: its status is pending for a few seconds,
then active ('cubecli s3 events rule list --bucket <bucket>').`,
		Example: `  cubecli s3 events rule create --bucket photos --destination uploads-hook --events created --prefix incoming/ --suffix .jpg
  cubecli s3 events rule create --bucket photos --destination ops --events created,removed,tagging --name everything`,
		RunE: func(cmd *cobra.Command, args []string) error {
			bucket, _ := cmd.Flags().GetString("bucket")
			destination, _ := cmd.Flags().GetString("destination")
			eventFlags, _ := cmd.Flags().GetStringSlice("events")
			prefix, _ := cmd.Flags().GetString("prefix")
			suffix, _ := cmd.Flags().GetString("suffix")
			name, _ := cmd.Flags().GetString("name")
			disabled, _ := cmd.Flags().GetBool("disabled")
			events, err := parseEvents(eventFlags)
			if err != nil {
				return err
			}
			if name == "" {
				name = strings.Join(append([]string{"on"}, strings.Split(shortEvents(events), ",")...), "-")
			}

			client := cmdutil.GetClient(cmd)
			s := output.NewSpinner("Creating event rule...")
			s.Start()
			bucketUUID, err := resolveBucket(client, bucket)
			var destUUID string
			if err == nil {
				destUUID, err = resolveDestination(client, destination)
			}
			var resp json.RawMessage
			if err == nil {
				body := map[string]interface{}{
					"name":             name,
					"destination_uuid": destUUID,
					"events":           events,
					"prefix":           prefix,
					"suffix":           suffix,
					"enabled":          !disabled,
				}
				resp, err = client.Post(rulesPath(bucketUUID), body)
			}
			s.Stop()
			if err != nil {
				return err
			}
			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(resp)
			}
			var r eventRule
			if err := json.Unmarshal(resp, &r); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}
			renderRules("Event Rule", []eventRule{r})
			return nil
		},
	}
	cmd.Flags().StringP("bucket", "b", "", "Bucket name or uuid")
	cmd.Flags().StringP("destination", "d", "", "Destination name or uuid")
	cmd.Flags().StringSlice("events", nil, "Events: created, removed, tagging (comma separated)")
	cmd.Flags().String("prefix", "", "Only keys starting with this prefix")
	cmd.Flags().String("suffix", "", "Only keys ending with this suffix")
	cmd.Flags().StringP("name", "n", "", "Rule name (default: on-<events>)")
	cmd.Flags().Bool("disabled", false, "Create the rule disabled")
	_ = cmd.MarkFlagRequired("bucket")
	_ = cmd.MarkFlagRequired("destination")
	_ = cmd.MarkFlagRequired("events")
	return cmd
}

func ruleUpdateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "update <rule>",
		Short:   "Change an event rule (uuid or name)",
		Args:    cobra.ExactArgs(1),
		Example: `  cubecli s3 events rule update on-created --bucket photos --events created,removed --suffix ""`,
		RunE: func(cmd *cobra.Command, args []string) error {
			bucket, _ := cmd.Flags().GetString("bucket")
			body := map[string]interface{}{}
			for _, f := range []string{"name", "prefix", "suffix"} {
				if cmd.Flags().Changed(f) {
					v, _ := cmd.Flags().GetString(f)
					body[f] = v
				}
			}
			if cmd.Flags().Changed("events") {
				v, _ := cmd.Flags().GetStringSlice("events")
				events, err := parseEvents(v)
				if err != nil {
					return err
				}
				body["events"] = events
			}
			if on, _ := cmd.Flags().GetBool("enable"); on {
				body["enabled"] = true
			}
			if off, _ := cmd.Flags().GetBool("disable"); off {
				body["enabled"] = false
			}
			destination, _ := cmd.Flags().GetString("destination")
			if len(body) == 0 && destination == "" {
				return fmt.Errorf("nothing to update: give --name, --destination, --events, --prefix, --suffix, --enable or --disable")
			}

			client := cmdutil.GetClient(cmd)
			s := output.NewSpinner("Updating event rule...")
			s.Start()
			bucketUUID, ruleUUID, err := resolveRule(client, bucket, args[0])
			if err == nil && destination != "" {
				var destUUID string
				destUUID, err = resolveDestination(client, destination)
				body["destination_uuid"] = destUUID
			}
			var resp json.RawMessage
			if err == nil {
				resp, err = client.Patch(rulesPath(bucketUUID)+"/"+url.PathEscape(ruleUUID), body)
			}
			s.Stop()
			if err != nil {
				return err
			}
			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(resp)
			}
			var r eventRule
			if err := json.Unmarshal(resp, &r); err == nil && r.UUID != "" {
				renderRules("Event Rule", []eventRule{r})
				return nil
			}
			output.PrintSuccess("Event rule updated")
			return nil
		},
	}
	cmd.Flags().StringP("bucket", "b", "", "Bucket name or uuid")
	cmd.Flags().StringP("name", "n", "", "New name")
	cmd.Flags().StringP("destination", "d", "", "New destination (name or uuid)")
	cmd.Flags().StringSlice("events", nil, "Events: created, removed, tagging (comma separated)")
	cmd.Flags().String("prefix", "", "Key prefix (\"\" for none)")
	cmd.Flags().String("suffix", "", "Key suffix (\"\" for none)")
	cmd.Flags().Bool("enable", false, "Enable the rule")
	cmd.Flags().Bool("disable", false, "Disable the rule")
	cmd.MarkFlagsMutuallyExclusive("enable", "disable")
	_ = cmd.MarkFlagRequired("bucket")
	return cmd
}

func ruleDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <rule>",
		Short: "Delete an event rule (uuid or name)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			bucket, _ := cmd.Flags().GetString("bucket")
			if !cmdutil.CheckForce(cmd, fmt.Sprintf("Delete event rule %s of bucket %s? Its events stop being sent.", args[0], bucket)) {
				output.PrintWarning("Aborted")
				return nil
			}
			client := cmdutil.GetClient(cmd)
			s := output.NewSpinner("Deleting event rule...")
			s.Start()
			bucketUUID, ruleUUID, err := resolveRule(client, bucket, args[0])
			var resp json.RawMessage
			if err == nil {
				resp, err = client.Delete(rulesPath(bucketUUID) + "/" + url.PathEscape(ruleUUID))
			}
			s.Stop()
			if err != nil {
				return err
			}
			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(resp)
			}
			output.PrintSuccess("Event rule deletion started")
			return nil
		},
	}
	cmd.Flags().StringP("bucket", "b", "", "Bucket name or uuid")
	cmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")
	_ = cmd.MarkFlagRequired("bucket")
	return cmd
}
