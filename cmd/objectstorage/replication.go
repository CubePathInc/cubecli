package objectstorage

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"

	"github.com/CubePathInc/cubecli/internal/api"
	"github.com/CubePathInc/cubecli/internal/cmdutil"
	"github.com/CubePathInc/cubecli/internal/output"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// replSecretEnv is read for the secret access key of an external destination
// when --secret-key-stdin is not given. The secret is never taken from argv.
const replSecretEnv = "CUBEPATH_REPL_SECRET"

// replicationNotDR is printed with every replication to a CubePath bucket.
const replicationNotDR = "Both buckets are stored in the same CubePath location: this is not a disaster recovery copy. Use an external destination for an off site copy."

type replicationTag struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type replication struct {
	UUID        string  `json:"uuid"`
	Status      string  `json:"status"`
	PauseReason *string `json:"pause_reason"`
	Direction   string  `json:"direction"`
	Source      struct {
		BucketUUID       *string `json:"bucket_uuid"`
		BucketName       *string `json:"bucket_name"`
		ProjectID        *int    `json:"project_id"`
		OrganizationName *string `json:"organization_name"`
		SameOrganization bool    `json:"same_organization"`
	} `json:"source"`
	Destination struct {
		Type             string  `json:"type"`
		BucketUUID       *string `json:"bucket_uuid"`
		BucketName       *string `json:"bucket_name"`
		ProjectID        *int    `json:"project_id"`
		OrganizationName *string `json:"organization_name"`
		SameOrganization bool    `json:"same_organization"`
		Provider         *string `json:"provider"`
		Endpoint         *string `json:"endpoint"`
		Region           *string `json:"region"`
		Bucket           *string `json:"bucket"`
		PathStyle        string  `json:"path_style"`
		AccessKeyID      *string `json:"access_key_id"`
	} `json:"destination"`
	Rules struct {
		Enabled                 bool             `json:"enabled"`
		Prefix                  *string          `json:"prefix"`
		Tags                    []replicationTag `json:"tags"`
		DeleteMarkerReplication bool             `json:"delete_marker_replication"`
		DeleteReplication       bool             `json:"delete_replication"`
		ExistingObjects         bool             `json:"existing_objects"`
	} `json:"rules"`
	Health          string  `json:"health"`
	HealthReason    *string `json:"health_reason"`
	HealthCheckedAt *string `json:"health_checked_at"`
	Backfill        struct {
		Status        string  `json:"status"`
		StartedAt     *string `json:"started_at"`
		FinishedAt    *string `json:"finished_at"`
		Objects       int64   `json:"objects"`
		Bytes         int64   `json:"bytes"`
		FailedObjects int64   `json:"failed_objects"`
	} `json:"backfill"`
	ErrorMessage *string `json:"error_message"`
	CreatedAt    *string `json:"created_at"`
	ActiveAt     *string `json:"active_at"`
	Metrics      *struct {
		ReplicatedBytes24h   *int64  `json:"replicated_bytes_24h"`
		ReplicatedObjects24h *int64  `json:"replicated_objects_24h"`
		FailedObjects1h      *int64  `json:"failed_objects_1h"`
		QueuedObjects        *int64  `json:"queued_objects"`
		QueuedBytes          *int64  `json:"queued_bytes"`
		LastSampleAt         *string `json:"last_sample_at"`
		EgressBytesMonth     *int64  `json:"egress_bytes_month"`
	} `json:"metrics"`
}

func str(p *string) string {
	if p == nil || *p == "" {
		return "-"
	}
	return *p
}

func (r replication) sourceLabel() string {
	name := str(r.Source.BucketName)
	if !r.Source.SameOrganization && r.Source.OrganizationName != nil {
		name += " (" + *r.Source.OrganizationName + ")"
	}
	return name
}

func (r replication) destinationLabel() string {
	d := r.Destination
	if d.Type == "external" {
		return str(d.Bucket) + " on " + str(d.Endpoint)
	}
	name := str(d.BucketName)
	if !d.SameOrganization && d.OrganizationName != nil {
		name += " (" + *d.OrganizationName + ")"
	}
	return name
}

func (r replication) statusLabel() string {
	s := output.FormatStatus(r.Status)
	if r.PauseReason != nil && *r.PauseReason != "" {
		s += " (" + *r.PauseReason + ")"
	}
	return s
}

func (r replication) healthLabel() string {
	if r.HealthReason != nil && *r.HealthReason != "" {
		return r.Health + ": " + *r.HealthReason
	}
	return r.Health
}

// rulesLabel summarizes the filter and options: "prefix img/, delete markers".
func (r replication) rulesLabel() string {
	var parts []string
	if r.Rules.Prefix != nil && *r.Rules.Prefix != "" {
		parts = append(parts, "prefix "+*r.Rules.Prefix)
	}
	if len(r.Rules.Tags) > 0 {
		tags := make([]string, len(r.Rules.Tags))
		for i, t := range r.Rules.Tags {
			tags[i] = t.Key + "=" + t.Value
		}
		parts = append(parts, "tags "+strings.Join(tags, ","))
	}
	if len(parts) == 0 {
		parts = append(parts, "every object")
	}
	if r.Rules.DeleteMarkerReplication {
		parts = append(parts, "delete markers")
	}
	if r.Rules.DeleteReplication {
		parts = append(parts, "version deletes")
	}
	if !r.Rules.ExistingObjects {
		parts = append(parts, "new objects only")
	}
	return strings.Join(parts, ", ")
}

func replicationCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "replication",
		Aliases: []string{"replications", "repl"},
		Short:   "Replicate a bucket to another CubePath bucket or to an external S3 bucket",
		Long: `Replicate the objects of a bucket, asynchronously, to one destination: another
CubePath bucket of the same tier (of your organization, or of another one that
gives you a grant token) or a bucket of an external S3 compatible provider
over HTTPS (port 443).

Versioning must be enabled on the source and on a CubePath destination.
Buckets with Object Lock cannot be sources. A CubePath destination is stored in
the same location as the source: it is not a disaster recovery copy. Data sent to
an external destination is billed as egress of the source bucket.

Replications take their uuid; get, update, delete and resync also take the name
of the source bucket.`,
	}
	cmd.AddCommand(
		replicationListCmd(),
		replicationGetCmd(),
		replicationCreateCmd(),
		replicationUpdateCmd(),
		replicationDeleteCmd(),
		replicationResyncCmd(),
		replicationRevokeCmd(),
		replicationGrantCmd(),
	)
	return cmd
}

// resolveReplication returns the uuid of a replication given its uuid or the name
// (or uuid) of its source bucket. A uuid that is the source bucket of an outgoing
// replication maps to that replication; any other uuid is taken as is.
func resolveReplication(client *api.Client, ref string) (string, error) {
	ref = strings.TrimSpace(ref)
	resp, err := client.Get("/object-storage/replications?direction=outgoing")
	if err != nil {
		return "", err
	}
	var list []replication
	if err := json.Unmarshal(resp, &list); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}
	for _, r := range list {
		if r.UUID == ref {
			return r.UUID, nil
		}
	}
	for _, r := range list {
		if (r.Source.BucketName != nil && *r.Source.BucketName == ref) ||
			(r.Source.BucketUUID != nil && *r.Source.BucketUUID == ref) {
			return r.UUID, nil
		}
	}
	if isUUID(ref) {
		return ref, nil
	}
	return "", fmt.Errorf("no replication found for %q", ref)
}

// --- list ---

func replicationListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List outgoing and incoming replications",
		Example: `  cubecli s3 replication list
  cubecli s3 replication list --direction incoming --bucket photos-backup`,
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			direction, _ := cmd.Flags().GetString("direction")
			switch direction {
			case "", "all", "outgoing", "incoming":
			default:
				return fmt.Errorf("--direction must be outgoing, incoming or all")
			}
			bucket, _ := cmd.Flags().GetString("bucket")

			s := output.NewSpinner("Fetching replications...")
			s.Start()
			q := url.Values{}
			if direction != "" {
				q.Set("direction", direction)
			}
			var err error
			if bucket != "" {
				var uuid string
				if uuid, err = resolveBucket(client, bucket); err == nil {
					q.Set("bucket_uuid", uuid)
				}
			}
			var resp json.RawMessage
			if err == nil {
				path := "/object-storage/replications"
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
				return output.PrintJSON(json.RawMessage(resp))
			}

			var list []replication
			if err := json.Unmarshal(resp, &list); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}

			t := output.NewTable("Replications", []string{"UUID", "Direction", "Source", "Destination", "Type", "Status", "Health", "Initial copy", "Rules"})
			for _, r := range list {
				t.AddRow(
					r.UUID,
					r.Direction,
					r.sourceLabel(),
					r.destinationLabel(),
					r.Destination.Type,
					r.statusLabel(),
					r.healthLabel(),
					r.Backfill.Status,
					r.rulesLabel(),
				)
			}
			t.Render()
			return nil
		},
	}
	cmd.Flags().String("direction", "", "outgoing, incoming or all (default all)")
	cmd.Flags().String("bucket", "", "Only replications from (outgoing) or into (incoming) this bucket, name or uuid")
	return cmd
}

// --- get ---

func replicationGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "get <replication>",
		Aliases: []string{"show"},
		Short:   "Show a replication with its health, initial copy and metrics",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			s := output.NewSpinner("Fetching replication...")
			s.Start()
			uuid, err := resolveReplication(client, args[0])
			var resp json.RawMessage
			if err == nil {
				resp, err = client.Get("/object-storage/replications/" + uuid)
			}
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			var r replication
			if err := json.Unmarshal(resp, &r); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}

			info := output.NewTable("Replication", []string{"Field", "Value"})
			info.AddRow("UUID", r.UUID)
			info.AddRow("Status", r.statusLabel())
			if r.ErrorMessage != nil && *r.ErrorMessage != "" {
				info.AddRow("Last error", *r.ErrorMessage)
			}
			info.AddRow("Source", r.sourceLabel())
			info.AddRow("Destination", r.destinationLabel())
			info.AddRow("Destination type", r.Destination.Type)
			if r.Destination.Type == "external" {
				info.AddRow("Provider", str(r.Destination.Provider))
				info.AddRow("Region", str(r.Destination.Region))
				info.AddRow("Path style", r.Destination.PathStyle)
				info.AddRow("Access key ID", str(r.Destination.AccessKeyID))
			}
			info.AddRow("Enabled", yesNo(r.Rules.Enabled))
			info.AddRow("Rules", r.rulesLabel())
			info.AddRow("Health", r.healthLabel())
			info.AddRow("Health checked at", str(r.HealthCheckedAt))
			info.AddRow("Initial copy", r.Backfill.Status)
			info.AddRow("Copied", fmt.Sprintf("%s objects, %s", formatCount(r.Backfill.Objects), formatBytes(r.Backfill.Bytes)))
			if r.Backfill.FailedObjects > 0 {
				info.AddRow("Failed objects", formatCount(r.Backfill.FailedObjects))
			}
			info.AddRow("Created", str(r.CreatedAt))
			info.AddRow("Active since", str(r.ActiveAt))
			info.Render()

			if m := r.Metrics; m != nil {
				mt := output.NewTable("Metrics", []string{"Replicated (24h)", "Objects (24h)", "Failed (1h)", "Queued", "Queued size", "Egress this month", "Sampled at"})
				mt.AddRow(
					formatBytesPtr(m.ReplicatedBytes24h),
					formatCountPtr(m.ReplicatedObjects24h),
					formatCountPtr(m.FailedObjects1h),
					formatCountPtr(m.QueuedObjects),
					formatBytesPtr(m.QueuedBytes),
					formatBytesPtr(m.EgressBytesMonth),
					str(m.LastSampleAt),
				)
				mt.Render()
			} else {
				output.PrintWarning("Replication metrics are not available yet.")
			}
			if r.Destination.Type == "cubepath" {
				output.PrintInfo(replicationNotDR)
			}
			return nil
		},
	}
}

// --- rules ---

// parseReplicationTags turns --tag key=value flags into the API's [{key, value}] list.
func parseReplicationTags(flags []string) ([]replicationTag, error) {
	tags := make([]replicationTag, 0, len(flags))
	seen := map[string]bool{}
	for _, f := range flags {
		key, value, _ := strings.Cut(f, "=")
		if key == "" {
			return nil, fmt.Errorf("invalid --tag %q: expected key=value", f)
		}
		if seen[key] {
			return nil, fmt.Errorf("tag %q given more than once", key)
		}
		seen[key] = true
		tags = append(tags, replicationTag{Key: key, Value: value})
	}
	if len(tags) > 10 {
		return nil, fmt.Errorf("a replication filter takes at most 10 tags, got %d", len(tags))
	}
	return tags, nil
}

func addReplicationRuleFlags(cmd *cobra.Command) {
	cmd.Flags().String("prefix", "", "Only replicate objects under this prefix")
	cmd.Flags().StringArray("tag", nil, "Only replicate objects carrying this tag, key=value (repeatable, all must match, up to 10; not with --prefix)")
	cmd.Flags().Bool("delete-markers", false, "Replicate delete markers (not with --tag)")
	cmd.Flags().Bool("deletes", false, "Replicate deletes of a specific version")
}

// readReplicationSecret returns the secret access key of an external destination:
// from stdin with --secret-key-stdin, else from CUBEPATH_REPL_SECRET, else asked
// without echo on a terminal, else read from piped stdin.
func readReplicationSecret(cmd *cobra.Command) (string, error) {
	fromStdin, _ := cmd.Flags().GetBool("secret-key-stdin")
	if !fromStdin {
		if v := os.Getenv(replSecretEnv); v != "" {
			return v, nil
		}
		if cmdutil.StdinIsTerminal() {
			fmt.Fprint(os.Stderr, "Secret access key: ")
			raw, err := term.ReadPassword(int(os.Stdin.Fd()))
			fmt.Fprintln(os.Stderr)
			if err != nil {
				return "", fmt.Errorf("failed to read the secret: %w", err)
			}
			return secretOrError(string(raw))
		}
	}
	line, err := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("failed to read the secret from stdin: %w", err)
	}
	return secretOrError(line)
}

func secretOrError(s string) (string, error) {
	s = strings.TrimRight(s, "\r\n")
	if s == "" {
		return "", fmt.Errorf("the secret access key is empty: pipe it on stdin or set %s", replSecretEnv)
	}
	return s, nil
}

// --- create ---

func replicationCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create <source-bucket>",
		Short: "Replicate a bucket to a CubePath bucket or to an external S3 bucket",
		Long: `Replicate a bucket (name or uuid) to one destination.

CubePath destination: --dest-bucket takes a bucket of your organization (name or
uuid) or the uuid of a bucket of another organization together with the
--grant-token its owner created with "cubecli s3 replication grant create".
Both buckets must be in the same tier and have versioning enabled. They are
stored in the same location: this is not a disaster recovery copy.

External destination: --external with --endpoint (public HTTPS host of the
provider, port 443 only), --region, --bucket and --access-key. The secret access
key is never taken from the command line: pipe it with --secret-key-stdin, set
CUBEPATH_REPL_SECRET, or type it when asked. Data sent to an external
destination is billed as egress of the source bucket. Versioning must be enabled
on the external bucket too.

By default the objects already in the bucket are copied as well; use
--no-existing-objects to replicate only new writes. Buckets with Object Lock
cannot be replication sources.`,
		Example: `  cubecli s3 replication create photos --dest-bucket photos-copy
  cubecli s3 replication create photos --dest-bucket 2b6c0e0a-... --grant-token cprg_...
  printf '%s' "$AWS_SECRET" | cubecli s3 replication create photos --external --provider aws \
      --endpoint s3.eu-west-1.amazonaws.com --region eu-west-1 --bucket acme-photos-backup \
      --access-key AKIA... --secret-key-stdin --prefix img/`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			f := cmd.Flags()

			external, _ := f.GetBool("external")
			destBucket, _ := f.GetString("dest-bucket")
			grant, _ := f.GetString("grant-token")
			cubepathOnly := []string{"dest-bucket", "grant-token"}
			externalOnly := []string{"provider", "endpoint", "region", "bucket", "path-style", "access-key", "secret-key-stdin"}

			dest := map[string]interface{}{}
			if external {
				for _, name := range cubepathOnly {
					if f.Changed(name) {
						return fmt.Errorf("--%s does not apply to an external destination", name)
					}
				}
				for _, name := range []string{"endpoint", "region", "bucket", "access-key"} {
					if v, _ := f.GetString(name); strings.TrimSpace(v) == "" {
						return fmt.Errorf("--%s is required with --external", name)
					}
				}
				provider, _ := f.GetString("provider")
				pathStyle, _ := f.GetString("path-style")
				if provider != "aws" && provider != "wasabi" && provider != "other" {
					return fmt.Errorf("--provider must be aws, wasabi or other")
				}
				if pathStyle != "auto" && pathStyle != "on" && pathStyle != "off" {
					return fmt.Errorf("--path-style must be auto, on or off")
				}
				endpoint, _ := f.GetString("endpoint")
				if strings.Contains(endpoint, "://") {
					return fmt.Errorf("--endpoint takes a host name without scheme, such as s3.eu-west-1.amazonaws.com")
				}
				region, _ := f.GetString("region")
				bucket, _ := f.GetString("bucket")
				accessKey, _ := f.GetString("access-key")
				dest["type"] = "external"
				dest["provider"] = provider
				dest["endpoint"] = strings.TrimSpace(endpoint)
				dest["region"] = strings.TrimSpace(region)
				dest["bucket"] = strings.TrimSpace(bucket)
				dest["path_style"] = pathStyle
				dest["access_key_id"] = strings.TrimSpace(accessKey)
			} else {
				for _, name := range externalOnly {
					if f.Changed(name) {
						return fmt.Errorf("--%s needs --external", name)
					}
				}
				if strings.TrimSpace(destBucket) == "" {
					return fmt.Errorf("give --dest-bucket for a CubePath destination, or --external")
				}
				dest["type"] = "cubepath"
				if grant != "" {
					dest["grant_token"] = strings.TrimSpace(grant)
				}
			}

			body := map[string]interface{}{"destination": dest}
			if f.Changed("prefix") {
				p, _ := f.GetString("prefix")
				body["prefix"] = p
			}
			if f.Changed("tag") {
				if f.Changed("prefix") {
					return fmt.Errorf("filter by --prefix or by --tag, not both")
				}
				tagFlags, _ := f.GetStringArray("tag")
				tags, err := parseReplicationTags(tagFlags)
				if err != nil {
					return err
				}
				body["tags"] = tags
			}
			markers, _ := f.GetBool("delete-markers")
			if markers && f.Changed("tag") {
				return fmt.Errorf("delete markers cannot be replicated with a tag filter")
			}
			deletes, _ := f.GetBool("deletes")
			noExisting, _ := f.GetBool("no-existing-objects")
			body["delete_marker_replication"] = markers
			body["delete_replication"] = deletes
			body["existing_objects"] = !noExisting

			if external {
				secret, err := readReplicationSecret(cmd)
				if err != nil {
					return err
				}
				dest["secret_access_key"] = secret
			}

			s := output.NewSpinner("Creating replication...")
			s.Start()
			var resp json.RawMessage
			source, err := resolveBucket(client, args[0])
			if err == nil {
				body["source_bucket_uuid"] = source
				if !external {
					var d string
					if d, err = resolveBucket(client, destBucket); err == nil {
						dest["bucket_uuid"] = d
					}
				}
			}
			if err == nil {
				resp, err = client.Post("/object-storage/replications", body)
			}
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			var created struct {
				UUID   string `json:"uuid"`
				Status string `json:"status"`
			}
			if err := json.Unmarshal(resp, &created); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}
			output.PrintSuccess(fmt.Sprintf("Replication %s is being configured (%s)", created.UUID, created.Status))
			if external {
				output.PrintInfo("Data sent to the external destination is billed as egress of the source bucket.")
			} else {
				output.PrintInfo(replicationNotDR)
			}
			return nil
		},
	}
	f := cmd.Flags()
	f.String("dest-bucket", "", "CubePath destination bucket: name or uuid (uuid for a bucket of another organization)")
	f.String("grant-token", "", "Grant token from the owner of a destination bucket of another organization")
	f.Bool("external", false, "Replicate to an external S3 compatible bucket")
	f.String("provider", "other", "External provider: aws, wasabi or other (informative)")
	f.String("endpoint", "", "External endpoint: public HTTPS host name, optionally with :443")
	f.String("region", "", "External bucket region")
	f.String("bucket", "", "External bucket name")
	f.String("path-style", "auto", "External addressing: auto, on or off")
	f.String("access-key", "", "External access key ID")
	f.Bool("secret-key-stdin", false, "Read the external secret access key from stdin (else "+replSecretEnv+" or a prompt)")
	addReplicationRuleFlags(cmd)
	f.Bool("no-existing-objects", false, "Do not copy the objects already in the bucket, only new writes")
	return cmd
}

// --- update ---

func replicationUpdateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update <replication>",
		Short: "Change the rules, pause or resume, or rotate external credentials",
		Long: `Change a replication (uuid or source bucket name).

--enabled=false pauses it and --enabled resumes it. --prefix or --tag replace
the filter (--clear-filter removes it). --rotate-credentials sets a new access
key of an external destination with --access-key; the secret is read with
--secret-key-stdin, from CUBEPATH_REPL_SECRET or from a prompt, never from the
command line.`,
		Example: `  cubecli s3 replication update photos --enabled=false
  cubecli s3 replication update photos --prefix img/ --delete-markers
  printf '%s' "$NEW_SECRET" | cubecli s3 replication update photos --rotate-credentials --access-key AKIA... --secret-key-stdin`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			f := cmd.Flags()

			body := map[string]interface{}{}
			if f.Changed("enabled") {
				v, _ := f.GetBool("enabled")
				body["enabled"] = v
			}
			clearFilter, _ := f.GetBool("clear-filter")
			if clearFilter && (f.Changed("prefix") || f.Changed("tag")) {
				return fmt.Errorf("--clear-filter cannot be used with --prefix or --tag")
			}
			if f.Changed("prefix") && f.Changed("tag") {
				return fmt.Errorf("filter by --prefix or by --tag, not both")
			}
			if clearFilter {
				body["prefix"] = nil
				body["tags"] = nil
			}
			if f.Changed("prefix") {
				p, _ := f.GetString("prefix")
				body["prefix"] = p
				body["tags"] = nil
			}
			if f.Changed("tag") {
				tagFlags, _ := f.GetStringArray("tag")
				tags, err := parseReplicationTags(tagFlags)
				if err != nil {
					return err
				}
				body["tags"] = tags
				body["prefix"] = nil
			}
			for flag, field := range map[string]string{
				"delete-markers":   "delete_marker_replication",
				"deletes":          "delete_replication",
				"existing-objects": "existing_objects",
			} {
				if f.Changed(flag) {
					v, _ := f.GetBool(flag)
					body[field] = v
				}
			}
			rotate, _ := f.GetBool("rotate-credentials")
			if !rotate && (f.Changed("access-key") || f.Changed("secret-key-stdin")) {
				return fmt.Errorf("--access-key and --secret-key-stdin need --rotate-credentials")
			}
			if len(body) == 0 && !rotate {
				return fmt.Errorf("nothing to change: give --enabled, a rule flag or --rotate-credentials")
			}
			if rotate {
				accessKey, _ := f.GetString("access-key")
				if strings.TrimSpace(accessKey) == "" {
					return fmt.Errorf("--rotate-credentials needs --access-key")
				}
				secret, err := readReplicationSecret(cmd)
				if err != nil {
					return err
				}
				body["destination"] = map[string]string{
					"access_key_id":     strings.TrimSpace(accessKey),
					"secret_access_key": secret,
				}
			}

			s := output.NewSpinner("Updating replication...")
			s.Start()
			uuid, err := resolveReplication(client, args[0])
			var resp json.RawMessage
			if err == nil {
				resp, err = client.Patch("/object-storage/replications/"+uuid, body)
			}
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}
			output.PrintSuccess("Replication updated")
			return nil
		},
	}
	f := cmd.Flags()
	f.Bool("enabled", true, "Resume (--enabled) or pause (--enabled=false) the replication")
	addReplicationRuleFlags(cmd)
	f.Bool("clear-filter", false, "Remove the prefix or tag filter: replicate every object")
	f.Bool("existing-objects", true, "Copy existing objects on resync (--existing-objects=false turns it off)")
	f.Bool("rotate-credentials", false, "Replace the access key of an external destination")
	f.String("access-key", "", "New external access key ID (with --rotate-credentials)")
	f.Bool("secret-key-stdin", false, "Read the new secret access key from stdin (else "+replSecretEnv+" or a prompt)")
	return cmd
}

// --- delete ---

func replicationDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <replication>",
		Short: "Stop and remove a replication (the data already copied stays in the destination)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmdutil.CheckForce(cmd, fmt.Sprintf("Remove replication %s? New objects stop being copied; the data already copied stays.", args[0])) {
				output.PrintWarning("Aborted")
				return nil
			}
			client := cmdutil.GetClient(cmd)

			s := output.NewSpinner("Removing replication...")
			s.Start()
			uuid, err := resolveReplication(client, args[0])
			var resp json.RawMessage
			if err == nil {
				resp, err = client.Delete("/object-storage/replications/" + uuid)
			}
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}
			output.PrintSuccess("Replication removal started")
			return nil
		},
	}
	cmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")
	return cmd
}

// --- resync ---

func replicationResyncCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "resync <replication>",
		Short: "Copy the existing objects again",
		Long: `Send the objects already in the source bucket to the destination again, for
example after the destination was unavailable. --older-than-days limits it to
objects older than that many days. Data sent to an external destination is
billed as egress again.`,
		Example: `  cubecli s3 replication resync photos
  cubecli s3 replication resync photos --older-than-days 3`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			body := map[string]interface{}{}
			if cmd.Flags().Changed("older-than-days") {
				days, _ := cmd.Flags().GetInt("older-than-days")
				if days < 1 || days > 36500 {
					return fmt.Errorf("--older-than-days must be between 1 and 36500")
				}
				body["older_than_days"] = days
			}

			s := output.NewSpinner("Queueing resync...")
			s.Start()
			uuid, err := resolveReplication(client, args[0])
			var resp json.RawMessage
			if err == nil {
				resp, err = client.Post("/object-storage/replications/"+uuid+"/resync", body)
			}
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}
			output.PrintSuccess("Resync queued")
			return nil
		},
	}
	cmd.Flags().Int("older-than-days", 0, "Only objects older than this many days (default: every object)")
	return cmd
}

// --- revoke ---

func replicationRevokeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "revoke <replication-uuid>",
		Short: "Stop a replication from another organization into one of your buckets",
		Long: `Stop an incoming replication from another organization (see
"cubecli s3 replication list --direction incoming"). The data already copied
stays in your bucket. The source owner cannot resume it: it needs a new grant.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !isUUID(args[0]) {
				return fmt.Errorf("revoke takes the replication uuid, see: cubecli s3 replication list --direction incoming")
			}
			if !cmdutil.CheckForce(cmd, fmt.Sprintf("Revoke incoming replication %s? The source owner cannot resume it.", args[0])) {
				output.PrintWarning("Aborted")
				return nil
			}
			return cmdutil.RunDetail(cmd, "Revoking replication...", "Incoming replication revoked", func() (json.RawMessage, error) {
				return cmdutil.GetClient(cmd).Post("/object-storage/replications/"+args[0]+"/revoke", nil)
			})
		},
	}
	cmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")
	return cmd
}

// --- grants ---

func replicationGrantCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "grant",
		Aliases: []string{"grants"},
		Short:   "Let another organization replicate into one of your buckets",
		Long: `A grant is a one use token that lets another organization replicate into one
of your buckets. Give the token and the bucket uuid to the other organization;
it uses them in "cubecli s3 replication create --dest-bucket <uuid> --grant-token
<token>". Grants expire (7 days by default, 30 at most) and can be revoked until
they are used. No grant is needed between buckets of your own organization.`,
	}
	cmd.AddCommand(grantCreateCmd(), grantListCmd(), grantDeleteCmd())
	return cmd
}

func grantCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "create <bucket>",
		Short:   "Create a grant for a bucket; the token is shown only once",
		Example: `  cubecli s3 replication grant create photos-backup --note "for Acme" --expires-in-days 3`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			days, _ := cmd.Flags().GetInt("expires-in-days")
			if days < 1 || days > 30 {
				return fmt.Errorf("--expires-in-days must be between 1 and 30")
			}
			body := map[string]interface{}{"expires_in_days": days}
			if cmd.Flags().Changed("note") {
				note, _ := cmd.Flags().GetString("note")
				body["note"] = note
			}

			s := output.NewSpinner("Creating replication grant...")
			s.Start()
			uuid, err := resolveBucket(client, args[0])
			var resp json.RawMessage
			if err == nil {
				resp, err = client.Post("/object-storage/buckets/"+uuid+"/replication-grants", body)
			}
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			var g struct {
				UUID       string  `json:"uuid"`
				Token      string  `json:"token"`
				BucketUUID string  `json:"bucket_uuid"`
				Note       *string `json:"note"`
				ExpiresAt  string  `json:"expires_at"`
			}
			if err := json.Unmarshal(resp, &g); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}
			info := output.NewTable("Replication Grant", []string{"Field", "Value"})
			info.AddRow("UUID", g.UUID)
			info.AddRow("Token", g.Token)
			info.AddRow("Bucket UUID", g.BucketUUID)
			info.AddRow("Note", str(g.Note))
			info.AddRow("Expires", g.ExpiresAt)
			info.Render()
			output.PrintWarning("Copy the token now: it will not be shown again. Share it with the bucket UUID.")
			return nil
		},
	}
	cmd.Flags().String("note", "", "Note to remember who the grant is for")
	cmd.Flags().Int("expires-in-days", 7, "Days until the grant expires (1 to 30)")
	return cmd
}

func grantListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list <bucket>",
		Short: "List the grants of a bucket (tokens are never shown again)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			s := output.NewSpinner("Fetching replication grants...")
			s.Start()
			uuid, err := resolveBucket(client, args[0])
			var resp json.RawMessage
			if err == nil {
				resp, err = client.Get("/object-storage/buckets/" + uuid + "/replication-grants")
			}
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			var grants []struct {
				UUID        string  `json:"uuid"`
				TokenPrefix string  `json:"token_prefix"`
				Note        *string `json:"note"`
				Status      string  `json:"status"`
				ExpiresAt   *string `json:"expires_at"`
				UsedAt      *string `json:"used_at"`
				RevokedAt   *string `json:"revoked_at"`
				CreatedAt   *string `json:"created_at"`
			}
			if err := json.Unmarshal(resp, &grants); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}
			t := output.NewTable("Replication Grants", []string{"UUID", "Token", "Note", "Status", "Expires", "Used", "Revoked", "Created"})
			for _, g := range grants {
				t.AddRow(g.UUID, g.TokenPrefix+"...", str(g.Note), output.FormatStatus(g.Status), str(g.ExpiresAt), str(g.UsedAt), str(g.RevokedAt), str(g.CreatedAt))
			}
			t.Render()
			return nil
		},
	}
}

func grantDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "delete <grant-uuid>",
		Aliases: []string{"revoke"},
		Short:   "Revoke a grant that was not used yet",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !isUUID(args[0]) {
				return fmt.Errorf("delete takes the grant uuid, see: cubecli s3 replication grant list <bucket>")
			}
			if !cmdutil.CheckForce(cmd, fmt.Sprintf("Revoke replication grant %s?", args[0])) {
				output.PrintWarning("Aborted")
				return nil
			}
			return cmdutil.RunDetail(cmd, "Revoking replication grant...", "Replication grant revoked", func() (json.RawMessage, error) {
				return cmdutil.GetClient(cmd).Delete("/object-storage/replication-grants/" + args[0])
			})
		},
	}
	cmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")
	return cmd
}
