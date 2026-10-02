package objectstorage

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/CubePathInc/cubecli/internal/api"
	"github.com/CubePathInc/cubecli/internal/cmdutil"
	"github.com/CubePathInc/cubecli/internal/output"
	"github.com/spf13/cobra"
)

// tierSummary is the "tier" object embedded in buckets, keys and usage.
type tierSummary struct {
	UUID  string `json:"uuid"`
	Slug  string `json:"slug"`
	Name  string `json:"name"`
	Media string `json:"media"`
}

// tierAliases are short names accepted by cubecli only; the API takes slugs and uuids.
var tierAliases = map[string]string{
	"ia": "infrequent_access",
}

var uuidRe = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// normalizeTier expands cubecli-only aliases ("ia") to the tier slug.
func normalizeTier(tier string) string {
	t := strings.TrimSpace(tier)
	if slug, ok := tierAliases[strings.ToLower(t)]; ok {
		return slug
	}
	return t
}

func isUUID(s string) bool {
	return uuidRe.MatchString(s)
}

func NewCmd() *cobra.Command {
	osCmd := &cobra.Command{
		Use:     "objectstorage",
		Aliases: []string{"s3", "object-storage"},
		Short:   "Manage Object Storage buckets and access keys (S3 compatible)",
		Long: `Manage CubePath Object Storage: S3-compatible buckets and the access keys
that S3 clients (AWS CLI, rclone, boto3, SDKs) use to reach them.

Tiers accept a slug, a uuid or the short alias "ia" (infrequent_access).
Buckets accept their uuid or their name.`,
	}

	osCmd.AddCommand(
		tiersCmd(),
		bucketCmd(),
		keyCmd(),
		usageCmd(),
	)

	return osCmd
}

// listQuery builds the ?project_id=&tier=&tag= query shared by the list endpoints.
func listQuery(cmd *cobra.Command) string {
	q := url.Values{}
	if projectID, _ := cmd.Flags().GetInt("project"); projectID > 0 {
		q.Set("project_id", strconv.Itoa(projectID))
	}
	if tier, _ := cmd.Flags().GetString("tier"); tier != "" {
		q.Set("tier", normalizeTier(tier))
	}
	addTagFilter(cmd, q)
	if len(q) == 0 {
		return ""
	}
	return "?" + q.Encode()
}

// --- tiers ---

func tiersCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "tiers",
		Short: "List storage tiers with endpoint, prices and free tier",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			s := output.NewSpinner("Fetching storage tiers...")
			s.Start()
			resp, err := client.Get("/object-storage/tiers")
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			var tiers []struct {
				Slug         string `json:"slug"`
				Name         string `json:"name"`
				Media        string `json:"media"`
				LocationName string `json:"location_name"`
				Region       string `json:"region"`
				Endpoint     string `json:"endpoint"`
				Prices       struct {
					StorageGBMonth float64 `json:"storage_gb_month"`
					EgressGB       float64 `json:"egress_gb"`
					ClassAPer1k    float64 `json:"class_a_per_1k"`
					ClassBPer1k    float64 `json:"class_b_per_1k"`
				} `json:"prices"`
				FreeTier struct {
					StorageGBMonth float64 `json:"storage_gb_month"`
					EgressGB       float64 `json:"egress_gb"`
					Requests       int64   `json:"requests"`
				} `json:"free_tier"`
				AcceptingNew bool `json:"accepting_new"`
			}
			if err := json.Unmarshal(resp, &tiers); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}

			t := output.NewTable("Storage Tiers", []string{"Slug", "Name", "Media", "Location", "Region", "Endpoint", "Storage/GiB-mo", "Egress/GiB", "Class A/1k", "Class B/1k", "Free tier", "New buckets"})
			for _, tier := range tiers {
				t.AddRow(
					tier.Slug,
					tier.Name,
					tier.Media,
					tier.LocationName,
					tier.Region,
					tier.Endpoint,
					formatPrice(tier.Prices.StorageGBMonth),
					formatPrice(tier.Prices.EgressGB),
					formatPrice(tier.Prices.ClassAPer1k),
					formatPrice(tier.Prices.ClassBPer1k),
					fmt.Sprintf("%g GiB, %g GiB egress, %s req", tier.FreeTier.StorageGBMonth, tier.FreeTier.EgressGB, formatCount(tier.FreeTier.Requests)),
					yesNo(tier.AcceptingNew),
				)
			}
			t.Render()
			return nil
		},
	}
}

// --- usage ---

func usageCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "usage",
		Short: "Show the month's Object Storage usage and cost per tier and bucket",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			q := url.Values{}
			if period, _ := cmd.Flags().GetString("period"); period != "" {
				q.Set("period", period)
			}
			if projectID, _ := cmd.Flags().GetInt("project"); projectID > 0 {
				q.Set("project_id", strconv.Itoa(projectID))
			}
			if tier, _ := cmd.Flags().GetString("tier"); tier != "" {
				q.Set("tier", normalizeTier(tier))
			}
			addTagFilter(cmd, q)
			path := "/object-storage/usage"
			if len(q) > 0 {
				path += "?" + q.Encode()
			}

			s := output.NewSpinner("Fetching Object Storage usage...")
			s.Start()
			resp, err := client.Get(path)
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			type quantities struct {
				StorageGiBMonth   *float64 `json:"storage_gib_month"`
				EgressBytes       *int64   `json:"egress_bytes"`
				CDNBytes          *int64   `json:"cdn_bytes"`
				ClassARequests    *int64   `json:"class_a_requests"`
				ClassBRequests    *int64   `json:"class_b_requests"`
				ClassBCDNRequests *int64   `json:"class_b_cdn_requests"`
			}
			type freeItem struct {
				Included float64  `json:"included"`
				Used     *float64 `json:"used"`
			}
			var usage struct {
				Period           string  `json:"period"`
				MetricsAvailable bool    `json:"metrics_available"`
				TotalCost        float64 `json:"total_cost"`
				ProjectedCost    float64 `json:"projected_cost"`
				Tiers            []struct {
					quantities
					Tier          tierSummary `json:"tier"`
					Cost          float64     `json:"cost"`
					ProjectedCost float64     `json:"projected_cost"`
					FreeTier      struct {
						StorageGBMonth freeItem `json:"storage_gb_month"`
						EgressGB       freeItem `json:"egress_gb"`
						Requests       freeItem `json:"requests"`
					} `json:"free_tier"`
				} `json:"tiers"`
				Buckets []struct {
					quantities
					Name   string            `json:"name"`
					Status string            `json:"status"`
					Cost   float64           `json:"cost"`
					Tags   map[string]string `json:"tags"`
				} `json:"buckets"`
			}
			if err := json.Unmarshal(resp, &usage); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}

			if !usage.MetricsAvailable {
				output.PrintWarning("Usage metrics are temporarily unavailable; costs are still shown.")
			}

			free := func(f freeItem) string {
				if f.Used == nil {
					return fmt.Sprintf("- / %g", f.Included)
				}
				return fmt.Sprintf("%g / %g", *f.Used, f.Included)
			}

			tt := output.NewTable(fmt.Sprintf("Object Storage usage %s", usage.Period), []string{"Tier", "Storage (GiB-mo)", "Egress", "CDN", "Class A", "Class B", "Cost", "Projected", "Free storage", "Free egress", "Free requests"})
			for _, tier := range usage.Tiers {
				tt.AddRow(
					tier.Tier.Name,
					formatGiBMonth(tier.StorageGiBMonth),
					formatBytesPtr(tier.EgressBytes),
					formatBytesPtr(tier.CDNBytes),
					formatCountPtr(tier.ClassARequests),
					formatCountPtr(tier.ClassBRequests),
					formatUSD(tier.Cost),
					formatUSD(tier.ProjectedCost),
					free(tier.FreeTier.StorageGBMonth),
					free(tier.FreeTier.EgressGB),
					free(tier.FreeTier.Requests),
				)
			}
			tt.Render()

			bt := output.NewTable("Buckets", []string{"Bucket", "Status", "Storage (GiB-mo)", "Egress", "CDN", "Class A", "Class B", "Cost", "Tags"})
			for _, b := range usage.Buckets {
				bt.AddRow(
					b.Name,
					output.FormatStatus(b.Status),
					formatGiBMonth(b.StorageGiBMonth),
					formatBytesPtr(b.EgressBytes),
					formatBytesPtr(b.CDNBytes),
					formatCountPtr(b.ClassARequests),
					formatCountPtr(b.ClassBRequests),
					formatUSD(b.Cost),
					formatTags(b.Tags, maxTagsColumn),
				)
			}
			bt.Render()

			fmt.Printf("\nTotal: %s (projected %s)\n", formatUSD(usage.TotalCost), formatUSD(usage.ProjectedCost))
			return nil
		},
	}
	cmd.Flags().String("period", "", "Month as YYYY-MM (default: current month, up to 12 months back)")
	cmd.Flags().IntP("project", "p", 0, "Only buckets of this project ID")
	cmd.Flags().String("tier", "", "Only this tier (slug, uuid or ia)")
	addTagFilterFlag(cmd)
	return cmd
}

// --- tags ---

// maxTagsColumn is the width of the TAGS column in tables; longer lists are cut with "...".
const maxTagsColumn = 40

// addTagFilterFlag adds the repeatable --tag filter of the list and usage commands.
func addTagFilterFlag(cmd *cobra.Command) {
	cmd.Flags().StringArray("tag", nil, "Only buckets with this tag: key (any value) or key=value (repeatable, all must match, up to 10)")
}

// addTagFilter copies every --tag filter into q as tag=..., which the API splits on the first "=".
func addTagFilter(cmd *cobra.Command, q url.Values) {
	if cmd.Flags().Lookup("tag") == nil {
		return
	}
	tags, _ := cmd.Flags().GetStringArray("tag")
	for _, t := range tags {
		if t = strings.TrimSpace(t); t != "" {
			q.Add("tag", t)
		}
	}
}

// parseTags turns --tag key=value flags into a map. A flag without "=" sets an empty value.
// The API validates the full rules (charset, lengths, reserved prefixes); this only catches
// what would be ambiguous on the command line.
func parseTags(flags []string) (map[string]string, error) {
	tags := make(map[string]string, len(flags))
	for _, f := range flags {
		key, value, _ := strings.Cut(f, "=")
		if key == "" {
			return nil, fmt.Errorf("invalid --tag %q: expected key=value", f)
		}
		if key != strings.TrimSpace(key) {
			return nil, fmt.Errorf("invalid --tag %q: the key cannot start or end with spaces", f)
		}
		if _, dup := tags[key]; dup {
			return nil, fmt.Errorf("tag %q given more than once", key)
		}
		tags[key] = value
	}
	if len(tags) > 50 {
		return nil, fmt.Errorf("a bucket can have at most 50 tags, got %d", len(tags))
	}
	return tags, nil
}

// formatTags renders tags as "k=v,k2=v2" sorted by key, cut to max characters ("-" when none).
func formatTags(tags map[string]string, max int) string {
	if len(tags) == 0 {
		return "-"
	}
	keys := make([]string, 0, len(tags))
	for k := range tags {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = k + "=" + tags[k]
	}
	s := strings.Join(parts, ",")
	if max > 3 && utf8.RuneCountInString(s) > max {
		return string([]rune(s)[:max-3]) + "..."
	}
	return s
}

// --- resolution helpers ---

// ResolveBucket returns the uuid of a bucket given its uuid or its name. The CDN
// origin commands use it to add a bucket as an origin.
func ResolveBucket(client *api.Client, ref string) (string, error) {
	return resolveBucket(client, ref)
}

// resolveBucket returns the uuid of a bucket given its uuid or its name.
func resolveBucket(client *api.Client, ref string) (string, error) {
	uuids, err := resolveBuckets(client, []string{ref})
	if err != nil {
		return "", err
	}
	return uuids[0], nil
}

// resolveBuckets maps bucket names or uuids to uuids with a single list call.
// Names win over uuids: a bucket may be named like a uuid (lowercase hex and
// hyphens), so a uuid-shaped ref is only taken as a uuid when no bucket has that name.
func resolveBuckets(client *api.Client, refs []string) ([]string, error) {
	resp, err := client.Get("/object-storage/buckets")
	if err != nil {
		return nil, err
	}
	var buckets []struct {
		UUID string `json:"uuid"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(resp, &buckets); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	byName := make(map[string]string, len(buckets))
	for _, b := range buckets {
		byName[b.Name] = b.UUID
	}
	uuids := make([]string, 0, len(refs))
	for _, ref := range refs {
		ref = strings.TrimSpace(ref)
		if uuid, ok := byName[ref]; ok {
			uuids = append(uuids, uuid)
			continue
		}
		if isUUID(ref) {
			uuids = append(uuids, ref)
			continue
		}
		return nil, fmt.Errorf("bucket %q not found", ref)
	}
	return uuids, nil
}

// resolveKey returns the uuid of an access key given its uuid, access key ID or name.
func resolveKey(client *api.Client, ref string) (string, error) {
	if isUUID(ref) {
		return ref, nil
	}
	resp, err := client.Get("/object-storage/keys")
	if err != nil {
		return "", err
	}
	var keys []struct {
		UUID        string `json:"uuid"`
		Name        string `json:"name"`
		AccessKeyID string `json:"access_key_id"`
	}
	if err := json.Unmarshal(resp, &keys); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}
	var byName []string
	for _, k := range keys {
		if k.AccessKeyID == ref {
			return k.UUID, nil
		}
		if k.Name == ref {
			byName = append(byName, k.UUID)
		}
	}
	switch len(byName) {
	case 1:
		return byName[0], nil
	case 0:
		return "", fmt.Errorf("access key %q not found", ref)
	default:
		return "", fmt.Errorf("several access keys are named %q; use the uuid or the access key ID", ref)
	}
}

// --- formatting ---

const (
	kib = 1024
	mib = 1024 * kib
	gib = 1024 * mib
	tib = 1024 * gib
)

// formatBytes uses binary units, labelled as such: "1.25 GiB", "512.0 MiB", "0 B".
func formatBytes(b int64) string {
	f := float64(b)
	switch {
	case b >= tib:
		return fmt.Sprintf("%.2f TiB", f/tib)
	case b >= gib:
		return fmt.Sprintf("%.2f GiB", f/gib)
	case b >= mib:
		return fmt.Sprintf("%.1f MiB", f/mib)
	case b >= kib:
		return fmt.Sprintf("%.1f KiB", f/kib)
	default:
		return fmt.Sprintf("%d B", b)
	}
}

func formatBytesPtr(b *int64) string {
	if b == nil {
		return "-"
	}
	return formatBytes(*b)
}

// formatCount groups thousands: 1234567 -> "1,234,567".
func formatCount(n int64) string {
	s := strconv.FormatInt(n, 10)
	neg := strings.HasPrefix(s, "-")
	if neg {
		s = s[1:]
	}
	var b strings.Builder
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(c)
	}
	if neg {
		return "-" + b.String()
	}
	return b.String()
}

func formatCountPtr(n *int64) string {
	if n == nil {
		return "-"
	}
	return formatCount(*n)
}

func formatGiBMonth(v *float64) string {
	if v == nil {
		return "-"
	}
	return strconv.FormatFloat(*v, 'f', 4, 64)
}

// formatUSD keeps four decimals for amounts under a dollar, which are often
// fractions of a cent.
func formatUSD(v float64) string {
	if v != 0 && v < 1 && v > -1 {
		return fmt.Sprintf("$%.4f", v)
	}
	return fmt.Sprintf("$%.2f", v)
}

// formatPrice prints a unit price without trailing zeros: 0.004 -> "$0.004".
func formatPrice(v float64) string {
	return "$" + strconv.FormatFloat(v, 'f', -1, 64)
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}
