package objectstorage

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/CubePathInc/cubecli/internal/api"
	"github.com/CubePathInc/cubecli/internal/cmdutil"
	"github.com/CubePathInc/cubecli/internal/output"
	"github.com/CubePathInc/cubecli/internal/s3sign"
	"github.com/spf13/cobra"
)

// maxPresignExpiry is the longest a presigned URL works on CubePath Object Storage: the
// endpoint refuses a longer X-Amz-Expires.
const maxPresignExpiry = 24 * time.Hour

// SkipAuthUnlessFlagAnnotation names a flag that, when set, lets the command run without a
// logged-in profile (the root command reads it before building the API client).
const SkipAuthUnlessFlagAnnotation = "cubecli/skip-auth-with-flag"

// presignNow is replaced in tests.
var presignNow = time.Now

func presignCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "presign <bucket>/<key>",
		Short: "Create a temporary download link for an object, signed locally",
		Long: `Create a presigned GET URL for one object. It is signed on this machine with
an access key of yours: the secret is never sent anywhere.

Anyone with the URL can download the object until it expires (at most 24 hours).
The file is always downloaded as an attachment, and every download counts as
egress of the bucket. To cut every URL signed with a key before it expires,
delete that access key.

Credentials come from --access-key/--secret-key or the AWS_ACCESS_KEY_ID and
AWS_SECRET_ACCESS_KEY environment variables (preferred: flags end up in the
shell history). The endpoint and region come from the bucket's tier, which
needs a logged-in profile; with --endpoint the command works offline.`,
		Example: `  AWS_ACCESS_KEY_ID=CP... AWS_SECRET_ACCESS_KEY=... cubecli s3 presign photos/2026/report.pdf --expires 1h
  cubecli s3 presign backups/db.sql.gz --expires 24h --tier ia --json
  cubecli s3 presign photos/a.txt --endpoint https://eu.cubestorage.io --region eu`,
		Args:        cobra.ExactArgs(1),
		Annotations: map[string]string{SkipAuthUnlessFlagAnnotation: "endpoint"},
		RunE: func(cmd *cobra.Command, args []string) error {
			bucket, key, err := splitObjectRef(args[0])
			if err != nil {
				return err
			}
			expires, _ := cmd.Flags().GetDuration("expires")
			if expires > maxPresignExpiry {
				return fmt.Errorf("presigned URLs on CubePath Object Storage last at most 24 hours")
			}
			if expires < time.Second {
				return fmt.Errorf("--expires must be at least 1s")
			}
			accessKey, _ := cmd.Flags().GetString("access-key")
			secretKey, _ := cmd.Flags().GetString("secret-key")
			if accessKey == "" {
				accessKey = os.Getenv("AWS_ACCESS_KEY_ID")
			}
			if secretKey == "" {
				secretKey = os.Getenv("AWS_SECRET_ACCESS_KEY")
			}
			if accessKey == "" || secretKey == "" {
				return fmt.Errorf("an access key is needed: pass --access-key and --secret-key or set AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY (create one with: cubecli s3 key create)")
			}

			endpoint, _ := cmd.Flags().GetString("endpoint")
			region, _ := cmd.Flags().GetString("region")
			if endpoint == "" {
				tier, _ := cmd.Flags().GetString("tier")
				endpoint, region, err = tierEndpoint(cmdutil.GetClient(cmd), bucket, tier, region)
				if err != nil {
					return err
				}
			} else if region == "" {
				region = "eu"
			}

			now := presignNow()
			signed, err := s3sign.PresignGet(s3sign.Request{
				Endpoint:  strings.TrimRight(endpoint, "/"),
				Region:    region,
				Path:      s3sign.PathStyle(bucket, key),
				AccessKey: accessKey,
				SecretKey: secretKey,
				Expires:   expires,
				Now:       now,
			})
			if err != nil {
				return err
			}
			expiresAt := now.UTC().Add(expires).Truncate(time.Second).Format(time.RFC3339)

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(map[string]string{"url": signed, "expires_at": expiresAt})
			}
			fmt.Println(signed)
			return nil
		},
	}
	cmd.Flags().Duration("expires", time.Hour, "How long the URL works, as a Go duration (1m, 6h, 24h); at most 24h")
	cmd.Flags().String("tier", "", "Tier of the bucket (slug, uuid or ia); default: the bucket's tier")
	cmd.Flags().String("endpoint", "", "S3 endpoint URL; skips the API lookup, so no login is needed")
	cmd.Flags().String("region", "", "Signing region (default: the tier's region, or eu with --endpoint)")
	cmd.Flags().String("access-key", "", "Access key ID (default: $AWS_ACCESS_KEY_ID)")
	cmd.Flags().String("secret-key", "", "Secret access key (default: $AWS_SECRET_ACCESS_KEY)")
	return cmd
}

// splitObjectRef splits "bucket/key/with/slashes". Keys that start with "/" or contain "//"
// are refused: the endpoint strips a leading slash (another object answers) and rejects "//".
func splitObjectRef(ref string) (string, string, error) {
	bucket, key, ok := strings.Cut(ref, "/")
	if !ok || bucket == "" || key == "" {
		return "", "", fmt.Errorf("expected <bucket>/<key>, got %q", ref)
	}
	if strings.HasPrefix(key, "/") || strings.Contains(key, "//") {
		return "", "", fmt.Errorf("object keys that start with '/' or contain '//' cannot be shared with a presigned URL")
	}
	if strings.HasSuffix(key, "/") {
		return "", "", fmt.Errorf("%q is a folder: presigned URLs point at one object", key)
	}
	return bucket, key, nil
}

// tierEndpoint finds the public endpoint and region to sign for: the --tier given, else the
// bucket's tier, else the only tier there is.
func tierEndpoint(client *api.Client, bucket, tier, region string) (string, string, error) {
	resp, err := client.Get("/object-storage/tiers")
	if err != nil {
		return "", "", err
	}
	var tiers []struct {
		UUID     string `json:"uuid"`
		Slug     string `json:"slug"`
		Region   string `json:"region"`
		Endpoint string `json:"endpoint"`
	}
	if err := json.Unmarshal(resp, &tiers); err != nil {
		return "", "", fmt.Errorf("failed to parse response: %w", err)
	}

	want := normalizeTier(tier)
	if want == "" {
		want, err = bucketTier(client, bucket)
		if err != nil {
			return "", "", err
		}
	}
	if want == "" && len(tiers) == 1 {
		want = tiers[0].Slug
	}
	if want == "" {
		return "", "", fmt.Errorf("bucket %q is not one of yours: pass --tier or --endpoint", bucket)
	}
	for _, t := range tiers {
		if t.Slug == want || t.UUID == want {
			if t.Endpoint == "" {
				return "", "", fmt.Errorf("tier %q has no public endpoint", t.Slug)
			}
			if region == "" {
				region = t.Region
			}
			return t.Endpoint, region, nil
		}
	}
	return "", "", fmt.Errorf("tier %q not found (see: cubecli s3 tiers)", want)
}

// bucketTier returns the tier slug of one of the caller's buckets, "" when it is not listed.
func bucketTier(client *api.Client, bucket string) (string, error) {
	resp, err := client.Get("/object-storage/buckets")
	if err != nil {
		return "", err
	}
	var buckets []struct {
		Name string      `json:"name"`
		Tier tierSummary `json:"tier"`
	}
	if err := json.Unmarshal(resp, &buckets); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}
	for _, b := range buckets {
		if b.Name == bucket {
			return b.Tier.Slug, nil
		}
	}
	return "", nil
}
