package objectstorage

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/CubePathInc/cubecli/internal/cmdutil"
	"github.com/CubePathInc/cubecli/internal/output"
	"github.com/spf13/cobra"
)

func keyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "key",
		Aliases: []string{"keys", "access-key"},
		Short:   "Manage access keys for S3 clients",
	}
	cmd.AddCommand(keyListCmd(), keyCreateCmd(), keyDeleteCmd())
	return cmd
}

// --- list ---

func keyListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List access keys (secrets are never shown again)",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			s := output.NewSpinner("Fetching access keys...")
			s.Start()
			resp, err := client.Get("/object-storage/keys" + listQuery(cmd))
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			var keys []struct {
				UUID        string `json:"uuid"`
				Name        string `json:"name"`
				AccessKeyID string `json:"access_key_id"`
				Permission  string `json:"permission"`
				BucketScope []struct {
					Name string `json:"name"`
				} `json:"bucket_scope"`
				ProjectID *int        `json:"project_id"`
				Tier      tierSummary `json:"tier"`
				Status    string      `json:"status"`
				ExpiresAt *string     `json:"expires_at"`
			}
			if err := json.Unmarshal(resp, &keys); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}

			t := output.NewTable("Access Keys", []string{"UUID", "Name", "Access Key ID", "Permission", "Buckets", "Tier", "Project", "Status", "Expires"})
			for _, k := range keys {
				scope := "all"
				if k.BucketScope != nil {
					names := make([]string, 0, len(k.BucketScope))
					for _, b := range k.BucketScope {
						names = append(names, b.Name)
					}
					scope = strings.Join(names, ", ")
					if len(names) == 0 {
						// Scoped to buckets that have all been deleted: the key reaches nothing.
						scope = "none (buckets deleted)"
					}
				}
				expires := "never"
				if k.ExpiresAt != nil && *k.ExpiresAt != "" {
					expires = *k.ExpiresAt
				}
				t.AddRow(
					k.UUID,
					k.Name,
					k.AccessKeyID,
					k.Permission,
					scope,
					k.Tier.Name,
					intPtr(k.ProjectID),
					output.FormatStatus(k.Status),
					expires,
				)
			}
			t.Render()
			return nil
		},
	}
	cmd.Flags().IntP("project", "p", 0, "Only keys of this project ID")
	cmd.Flags().String("tier", "", "Only keys of this tier (slug, uuid or ia)")
	return cmd
}

// --- create ---

func keyCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create an access key; the secret is shown only once",
		Long: `Create an access key for S3 clients. The secret access key is returned only
once: save it right away. The key works once its status is active (usually 10
to 20 seconds).

Without --bucket the key reaches every bucket of the project in that tier,
present and future. --output prints ready-to-use credentials:

  env     .env file with the standard AWS_* variables
  rclone  rclone.conf remote
  aws     ~/.aws/credentials profile (with region and endpoint_url)`,
		Example: `  cubecli s3 key create --name backups --tier ia
  cubecli s3 key create --name web --tier ia --bucket photos --permission read_only --output env > .env.cubepath-storage
  cubecli s3 key create --name nightly --tier ia --expires-in 720h --output rclone >> ~/.config/rclone/rclone.conf`,
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			name, _ := cmd.Flags().GetString("name")
			tier, _ := cmd.Flags().GetString("tier")
			projectID, _ := cmd.Flags().GetInt("project")
			permission, _ := cmd.Flags().GetString("permission")
			buckets, _ := cmd.Flags().GetStringSlice("bucket")
			format, _ := cmd.Flags().GetString("output")
			expiresAt, _ := cmd.Flags().GetString("expires-at")
			expiresIn, _ := cmd.Flags().GetString("expires-in")

			if format != "" && credentialWriters[format] == nil {
				return fmt.Errorf("unknown --output %q: use env, rclone or aws", format)
			}
			if permission != "read_write" && permission != "read_only" {
				return fmt.Errorf("--permission must be read_write or read_only")
			}
			expiry, err := parseExpiry(expiresAt, expiresIn, time.Now())
			if err != nil {
				return err
			}

			body := map[string]interface{}{
				"name":       name,
				"tier":       normalizeTier(tier),
				"permission": permission,
			}
			if projectID > 0 {
				body["project_id"] = projectID
			}
			if expiry != "" {
				body["expires_at"] = expiry
			}

			s := output.NewSpinner("Creating access key...")
			s.Start()
			if len(buckets) > 0 {
				var uuids []string
				uuids, err = resolveBuckets(client, buckets)
				body["bucket_uuids"] = uuids
			}
			var resp json.RawMessage
			if err == nil {
				resp, err = client.Post("/object-storage/keys", body)
			}
			s.Stop()
			if err != nil {
				return err
			}

			var key createdKey
			if err := json.Unmarshal(resp, &key); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}

			if format != "" {
				// Only the credentials go to stdout, so the output can be redirected to a file.
				fmt.Print(credentialWriters[format](key))
				fmt.Fprintln(os.Stderr, "Save these credentials now: the secret will not be shown again.")
				return nil
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			info := output.NewTable("Access Key", []string{"Field", "Value"})
			info.AddRow("UUID", key.UUID)
			info.AddRow("Name", key.Name)
			info.AddRow("Access key ID", key.AccessKeyID)
			info.AddRow("Secret access key", key.SecretAccessKey)
			info.AddRow("Permission", key.Permission)
			info.AddRow("Endpoint", key.Endpoint)
			info.AddRow("Region", key.Region)
			info.AddRow("Status", output.FormatStatus(key.Status))
			info.Render()
			output.PrintWarning("Copy the secret now: it will not be shown again.")
			return nil
		},
	}
	cmd.Flags().StringP("name", "n", "", "Key name")
	cmd.Flags().String("tier", "", "Storage tier: slug, uuid or ia")
	cmd.Flags().IntP("project", "p", 0, "Project ID (default: the organization's first project)")
	cmd.Flags().String("permission", "read_write", "read_write or read_only")
	cmd.Flags().StringSlice("bucket", nil, "Limit the key to these buckets (name or uuid, repeatable; default: every bucket)")
	cmd.Flags().String("expires-at", "", "Expiry time, UTC (YYYY-MM-DD or YYYY-MM-DDTHH:MM:SS, or RFC 3339)")
	cmd.Flags().String("expires-in", "", "Expiry from now: a duration such as 720h or 30d")
	cmd.Flags().StringP("output", "o", "", "Print the credentials as env, rclone or aws")
	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("tier")
	cmd.MarkFlagsMutuallyExclusive("expires-at", "expires-in")
	return cmd
}

// --- delete ---

func keyDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <key>",
		Short: "Revoke an access key (uuid, access key ID or name)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmdutil.CheckForce(cmd, fmt.Sprintf("Revoke access key %s? Clients using it stop working.", args[0])) {
				output.PrintWarning("Aborted")
				return nil
			}

			client := cmdutil.GetClient(cmd)

			s := output.NewSpinner("Deleting access key...")
			s.Start()
			uuid, err := resolveKey(client, args[0])
			var resp json.RawMessage
			if err == nil {
				resp, err = client.Delete("/object-storage/keys/" + uuid)
			}
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			output.PrintSuccess("Access key deletion started")
			return nil
		},
	}
	cmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")
	return cmd
}

var daysRe = regexp.MustCompile(`^(\d+)d$`)

// parseExpiry turns --expires-at / --expires-in into the API's UTC timestamp
// without offset ("2026-10-01T00:00:00"). Both empty means no expiry.
func parseExpiry(at, in string, now time.Time) (string, error) {
	const apiLayout = "2006-01-02T15:04:05"
	if in != "" {
		var d time.Duration
		if m := daysRe.FindStringSubmatch(in); m != nil {
			n, _ := strconv.Atoi(m[1])
			d = time.Duration(n) * 24 * time.Hour
		} else {
			var err error
			if d, err = time.ParseDuration(in); err != nil {
				return "", fmt.Errorf("invalid --expires-in %q: use a duration such as 720h or 30d", in)
			}
		}
		if d <= 0 {
			return "", fmt.Errorf("--expires-in must be positive")
		}
		return now.Add(d).UTC().Format(apiLayout), nil
	}
	if at == "" {
		return "", nil
	}
	if t, err := time.Parse(time.RFC3339, at); err == nil {
		return t.UTC().Format(apiLayout), nil
	}
	for _, layout := range []string{apiLayout, "2006-01-02T15:04", "2006-01-02"} {
		if t, err := time.Parse(layout, at); err == nil {
			return t.Format(apiLayout), nil
		}
	}
	return "", fmt.Errorf("invalid --expires-at %q: use YYYY-MM-DD, YYYY-MM-DDTHH:MM:SS (UTC) or RFC 3339", at)
}

// createdKey is the POST /object-storage/keys response, the only one that carries the secret.
type createdKey struct {
	UUID            string `json:"uuid"`
	Name            string `json:"name"`
	AccessKeyID     string `json:"access_key_id"`
	SecretAccessKey string `json:"secret_access_key"`
	Permission      string `json:"permission"`
	Region          string `json:"region"`
	Endpoint        string `json:"endpoint"`
	Status          string `json:"status"`
}

// The credential files match the dashboard's show-once dialog
// (cubepath-dashboard src/lib/objectStorage.ts).
var credentialWriters = map[string]func(createdKey) string{
	"env":    envFile,
	"rclone": rcloneConf,
	"aws":    awsCredentials,
}

var nonSlugRe = regexp.MustCompile(`[^a-z0-9]+`)

// profileName is the rclone remote / AWS profile name derived from the key name.
func profileName(keyName string) string {
	slug := strings.Trim(nonSlugRe.ReplaceAllString(strings.ToLower(keyName), "-"), "-")
	if slug == "" {
		slug = "storage"
	}
	return "cubepath-" + slug
}

func lines(l ...string) string {
	return strings.Join(l, "\n") + "\n"
}

func envFile(k createdKey) string {
	return lines(
		"# CubePath Object Storage: "+k.Name,
		"AWS_ACCESS_KEY_ID="+k.AccessKeyID,
		"AWS_SECRET_ACCESS_KEY="+k.SecretAccessKey,
		"AWS_REGION="+k.Region,
		"AWS_DEFAULT_REGION="+k.Region,
		"AWS_ENDPOINT_URL="+k.Endpoint,
		"AWS_ENDPOINT_URL_S3="+k.Endpoint,
	)
}

func rcloneConf(k createdKey) string {
	return lines(
		"["+profileName(k.Name)+"]",
		"type = s3",
		"provider = Other",
		"access_key_id = "+k.AccessKeyID,
		"secret_access_key = "+k.SecretAccessKey,
		"endpoint = "+k.Endpoint,
		"region = "+k.Region,
		"force_path_style = true",
		"acl = private",
	)
}

func awsCredentials(k createdKey) string {
	profile := profileName(k.Name)
	return lines(
		"# Append to ~/.aws/credentials, then: aws --profile "+profile+" s3 ls",
		"["+profile+"]",
		"aws_access_key_id = "+k.AccessKeyID,
		"aws_secret_access_key = "+k.SecretAccessKey,
		"region = "+k.Region,
		"endpoint_url = "+k.Endpoint,
	)
}
