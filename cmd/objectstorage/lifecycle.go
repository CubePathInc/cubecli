package objectstorage

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/CubePathInc/cubecli/internal/api"
	"github.com/CubePathInc/cubecli/internal/cmdutil"
	"github.com/CubePathInc/cubecli/internal/output"
	"github.com/spf13/cobra"
)

// lifecyclePollInterval is how often --wait reads the bucket's lifecycle again.
var lifecyclePollInterval = 5 * time.Second

type lifecycleTag struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type lifecycleRule struct {
	ID      string `json:"id"`
	Enabled bool   `json:"enabled"`
	Filter  *struct {
		Prefix                *string        `json:"prefix"`
		Tags                  []lifecycleTag `json:"tags"`
		ObjectSizeGreaterThan *int64         `json:"object_size_greater_than"`
		ObjectSizeLessThan    *int64         `json:"object_size_less_than"`
	} `json:"filter"`
	Expiration *struct {
		Days                      *int    `json:"days"`
		Date                      *string `json:"date"`
		ExpiredObjectDeleteMarker *bool   `json:"expired_object_delete_marker"`
	} `json:"expiration"`
	NoncurrentVersionExpiration *struct {
		NoncurrentDays          int  `json:"noncurrent_days"`
		NewerNoncurrentVersions *int `json:"newer_noncurrent_versions"`
	} `json:"noncurrent_version_expiration"`
	AbortIncompleteMultipartUpload *struct {
		DaysAfterInitiation int `json:"days_after_initiation"`
	} `json:"abort_incomplete_multipart_upload"`
}

type lifecycleState struct {
	BucketUUID        string          `json:"bucket_uuid"`
	Status            string          `json:"status"`
	Rules             []lifecycleRule `json:"rules"`
	Generation        int             `json:"generation"`
	AppliedGeneration int             `json:"applied_generation"`
	Error             *string         `json:"error"`
	UpdatedAt         *string         `json:"updated_at"`
	Notes             []string        `json:"notes"`
}

func lifecyclePath(uuid string) string {
	return "/object-storage/buckets/" + uuid + "/lifecycle"
}

func bucketLifecycleCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "lifecycle",
		Short: "Manage a bucket's lifecycle rules",
		Long: `Lifecycle rules delete objects in the background: current objects after a number of
days or on a date, noncurrent versions of a versioned bucket, orphan delete markers and
incomplete multipart uploads.

Rules are applied asynchronously (usually in seconds, up to 10 minutes after a previous
change of the same bucket) and objects are removed within 48 hours of their due date.
Deletions are permanent. In a versioned bucket an expiration only adds a delete marker:
add a noncurrent version rule to free the space.`,
	}
	cmd.AddCommand(lifecycleGetCmd(), lifecycleSetCmd(), lifecycleDeleteCmd())
	return cmd
}

// --- get ---

func lifecycleGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <bucket>",
		Short: "Show a bucket's lifecycle rules and whether they are applied",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			s := output.NewSpinner("Fetching lifecycle rules...")
			s.Start()
			uuid, err := resolveBucket(client, args[0])
			var resp json.RawMessage
			if err == nil {
				resp, err = client.Get(lifecyclePath(uuid))
			}
			s.Stop()
			if err != nil {
				return err
			}
			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(resp)
			}
			var state lifecycleState
			if err := json.Unmarshal(resp, &state); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}
			printLifecycle(args[0], state)
			return nil
		},
	}
}

func printLifecycle(bucket string, state lifecycleState) {
	if len(state.Rules) == 0 {
		fmt.Printf("Bucket %s has no lifecycle rules (incomplete multipart uploads are aborted after 7 days).\n", bucket)
	} else {
		t := output.NewTable(fmt.Sprintf("Lifecycle rules of %s", bucket), []string{"ID", "Enabled", "Scope", "Actions"})
		for _, r := range state.Rules {
			enabled := "yes"
			if !r.Enabled {
				enabled = "no"
			}
			t.AddRow(r.ID, enabled, lifecycleScope(r), lifecycleActions(r))
		}
		t.Render()
	}
	applied := fmt.Sprintf("generation %d, applied %d", state.Generation, state.AppliedGeneration)
	fmt.Printf("\nStatus: %s (%s)\n", state.Status, applied)
	if state.Error != nil && *state.Error != "" {
		output.PrintWarning(*state.Error)
	}
	for _, note := range state.Notes {
		output.PrintInfo(note)
	}
}

func lifecycleScope(r lifecycleRule) string {
	if r.Filter == nil {
		return "whole bucket"
	}
	parts := []string{}
	if r.Filter.Prefix != nil && *r.Filter.Prefix != "" {
		parts = append(parts, "prefix "+*r.Filter.Prefix)
	}
	for _, tag := range r.Filter.Tags {
		parts = append(parts, "tag "+tag.Key+"="+tag.Value)
	}
	if r.Filter.ObjectSizeGreaterThan != nil {
		parts = append(parts, "size > "+formatBytes(*r.Filter.ObjectSizeGreaterThan))
	}
	if r.Filter.ObjectSizeLessThan != nil {
		parts = append(parts, "size < "+formatBytes(*r.Filter.ObjectSizeLessThan))
	}
	if len(parts) == 0 {
		return "whole bucket"
	}
	return strings.Join(parts, ", ")
}

func lifecycleActions(r lifecycleRule) string {
	parts := []string{}
	if e := r.Expiration; e != nil {
		switch {
		case e.Days != nil:
			parts = append(parts, fmt.Sprintf("delete after %d days", *e.Days))
		case e.Date != nil:
			parts = append(parts, "delete on "+*e.Date)
		case e.ExpiredObjectDeleteMarker != nil && *e.ExpiredObjectDeleteMarker:
			parts = append(parts, "remove orphan delete markers")
		}
	}
	if n := r.NoncurrentVersionExpiration; n != nil {
		text := fmt.Sprintf("delete noncurrent versions after %d days", n.NoncurrentDays)
		if n.NewerNoncurrentVersions != nil {
			text += fmt.Sprintf(" (keep %d)", *n.NewerNoncurrentVersions)
		}
		parts = append(parts, text)
	}
	if a := r.AbortIncompleteMultipartUpload; a != nil {
		parts = append(parts, fmt.Sprintf("abort uploads after %d days", a.DaysAfterInitiation))
	}
	return strings.Join(parts, "; ")
}

// --- set ---

// readLifecycleRules reads a rules document: {"rules": [...]} or a bare [...] array.
func readLifecycleRules(path string, stdin io.Reader) ([]interface{}, error) {
	var raw []byte
	var err error
	if path == "-" {
		raw, err = io.ReadAll(stdin)
	} else {
		raw, err = os.ReadFile(path)
	}
	if err != nil {
		return nil, fmt.Errorf("cannot read %s: %w", path, err)
	}
	var doc struct {
		Rules []interface{} `json:"rules"`
	}
	if err := json.Unmarshal(raw, &doc); err == nil && doc.Rules != nil {
		return doc.Rules, nil
	}
	var rules []interface{}
	if err := json.Unmarshal(raw, &rules); err != nil {
		return nil, fmt.Errorf(`%s is not a rules document: use {"rules": [...]} or a JSON array of rules`, path)
	}
	return rules, nil
}

func lifecycleSetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set <bucket>",
		Short: "Replace every lifecycle rule of a bucket",
		Long: `Replace every lifecycle rule of a bucket, from a JSON file (--file rules.json, or
--file - for stdin) or, for the common case, one expiration rule built from flags:

  cubecli s3 bucket lifecycle set logs --expire-days 30 --prefix logs/

Rule format: {"id": "logs-30d", "enabled": true, "filter": {"prefix": "logs/"},
"expiration": {"days": 30}}. Also "expiration": {"date": "2027-01-01"} or
{"expired_object_delete_marker": true}, "noncurrent_version_expiration":
{"noncurrent_days": 30, "newer_noncurrent_versions": 3} and
"abort_incomplete_multipart_upload": {"days_after_initiation": 2}. Up to 100 rules.

Expiration rules delete objects permanently. --wait returns once the rules are applied.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			file, _ := cmd.Flags().GetString("file")
			days, _ := cmd.Flags().GetInt("expire-days")
			prefix, _ := cmd.Flags().GetString("prefix")
			ruleID, _ := cmd.Flags().GetString("id")

			var rules []interface{}
			switch {
			case file != "" && days != 0:
				return errors.New("use either --file or --expire-days, not both")
			case file != "":
				var err error
				if rules, err = readLifecycleRules(file, cmd.InOrStdin()); err != nil {
					return err
				}
			case days != 0:
				if days < 1 || days > 36500 {
					return errors.New("--expire-days must be between 1 and 36500")
				}
				if ruleID == "" {
					ruleID = fmt.Sprintf("expire-%dd", days)
				}
				rule := map[string]interface{}{"id": ruleID, "enabled": true, "expiration": map[string]interface{}{"days": days}}
				if prefix != "" {
					rule["filter"] = map[string]interface{}{"prefix": prefix}
				}
				rules = []interface{}{rule}
			default:
				return errors.New("give the rules with --file (or --file - for stdin) or --expire-days")
			}
			if len(rules) == 0 {
				return errors.New("no rules given: to remove every rule use `cubecli s3 bucket lifecycle delete`")
			}

			msg := fmt.Sprintf("Replace every lifecycle rule of bucket %s with %d rule(s)? Expired objects are deleted permanently.", args[0], len(rules))
			if !cmdutil.CheckForce(cmd, msg) {
				output.PrintWarning("Aborted")
				return nil
			}

			client := cmdutil.GetClient(cmd)
			s := output.NewSpinner("Setting lifecycle rules...")
			s.Start()
			uuid, err := resolveBucket(client, args[0])
			var resp json.RawMessage
			if err == nil {
				resp, err = client.Put(lifecyclePath(uuid), map[string]interface{}{"rules": rules})
			}
			s.Stop()
			if err != nil {
				return err
			}
			return finishLifecycleChange(cmd, client, args[0], uuid, resp)
		},
	}
	cmd.Flags().String("file", "", "JSON file with the rules ({\"rules\": [...]} or [...]); - reads stdin")
	cmd.Flags().Int("expire-days", 0, "Build one rule that deletes objects this many days after they are written")
	cmd.Flags().String("prefix", "", "With --expire-days: only objects whose key starts with this prefix")
	cmd.Flags().String("id", "", "With --expire-days: the rule ID (default expire-<days>d)")
	addLifecycleWaitFlags(cmd)
	cmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")
	return cmd
}

// --- delete ---

func lifecycleDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <bucket>",
		Short: "Remove every lifecycle rule of a bucket",
		Long: `Remove every lifecycle rule of a bucket: nothing is deleted by rules from then on.
Incomplete multipart uploads are still aborted after 7 days.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmdutil.CheckForce(cmd, fmt.Sprintf("Remove every lifecycle rule of bucket %s?", args[0])) {
				output.PrintWarning("Aborted")
				return nil
			}
			client := cmdutil.GetClient(cmd)
			s := output.NewSpinner("Removing lifecycle rules...")
			s.Start()
			uuid, err := resolveBucket(client, args[0])
			var resp json.RawMessage
			if err == nil {
				resp, err = client.Delete(lifecyclePath(uuid))
			}
			s.Stop()
			if err != nil {
				return err
			}
			return finishLifecycleChange(cmd, client, args[0], uuid, resp)
		},
	}
	addLifecycleWaitFlags(cmd)
	cmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")
	return cmd
}

func addLifecycleWaitFlags(cmd *cobra.Command) {
	cmd.Flags().Bool("wait", false, "Wait until the rules are applied")
	cmd.Flags().Duration("wait-timeout", 15*time.Minute, "With --wait: give up after this long")
}

// finishLifecycleChange prints the answer of a PUT or DELETE and, with --wait, polls the
// lifecycle until the generation it returned is applied (or the change failed).
func finishLifecycleChange(cmd *cobra.Command, client *api.Client, bucket, uuid string, resp json.RawMessage) error {
	var answer struct {
		Detail     string   `json:"detail"`
		Generation *int     `json:"generation"`
		Notes      []string `json:"notes"`
	}
	_ = json.Unmarshal(resp, &answer)
	wait, _ := cmd.Flags().GetBool("wait")
	if !wait || answer.Generation == nil {
		if cmdutil.IsJSON(cmd) {
			return output.PrintJSON(resp)
		}
		output.PrintSuccess(answer.Detail)
		for _, note := range answer.Notes {
			output.PrintInfo(note)
		}
		return nil
	}

	timeout, _ := cmd.Flags().GetDuration("wait-timeout")
	deadline := time.Now().Add(timeout)
	s := output.NewSpinner("Waiting for the rules to be applied...")
	s.Start()
	var state lifecycleState
	for {
		raw, err := client.Get(lifecyclePath(uuid))
		if err != nil {
			s.Stop()
			return err
		}
		if err := json.Unmarshal(raw, &state); err != nil {
			s.Stop()
			return fmt.Errorf("failed to parse response: %w", err)
		}
		if state.AppliedGeneration >= *answer.Generation || state.Status == "error" || state.Generation > *answer.Generation {
			s.Stop()
			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(raw)
			}
			break
		}
		if time.Now().After(deadline) {
			s.Stop()
			return fmt.Errorf("the rules of bucket %s are not applied yet after %s (generation %d, applied %d)", bucket, timeout, state.Generation, state.AppliedGeneration)
		}
		time.Sleep(lifecyclePollInterval)
	}
	if state.Status == "error" {
		msg := "the storage service rejected the rules"
		if state.Error != nil && *state.Error != "" {
			msg = *state.Error
		}
		return fmt.Errorf("%s", msg)
	}
	if state.Generation > *answer.Generation {
		output.PrintWarning("Another change of the rules came after this one; showing the current state.")
	} else {
		output.PrintSuccess("Lifecycle rules applied")
	}
	printLifecycle(bucket, state)
	return nil
}
