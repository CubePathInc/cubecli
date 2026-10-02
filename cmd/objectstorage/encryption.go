package objectstorage

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/CubePathInc/cubecli/internal/cmdutil"
	"github.com/CubePathInc/cubecli/internal/output"
)

func bucketEncryptionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "encryption",
		Short: "Manage a bucket's encryption at rest",
		Long: `Encryption at rest (AES-256) is chosen when the bucket is created and is on by
default. A bucket created without it can have it enabled later; once enabled it
cannot be turned off.`,
	}
	cmd.AddCommand(bucketEncryptionEnableCmd())
	return cmd
}

func bucketEncryptionEnableCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "enable <bucket>",
		Short: "Enable encryption at rest on a bucket (cannot be undone)",
		Long: `Enable encryption at rest (AES-256) on a bucket. New objects are encrypted at
once and the objects already stored are encrypted in the background; their
content, metadata, tags and ETag stay the same, their last modified date changes.
In a versioned bucket only the current versions are encrypted: older versions
stay as they are. Encryption cannot be turned off afterwards.`,
		Example: `  cubecli s3 bucket encryption enable photos
  cubecli s3 bucket encryption enable photos --force`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			question := fmt.Sprintf("Enable encryption at rest on bucket %s? It cannot be turned off afterwards.", args[0])
			if !cmdutil.CheckForce(cmd, question) {
				output.PrintWarning("Aborted")
				return nil
			}
			client := cmdutil.GetClient(cmd)
			s := output.NewSpinner("Enabling encryption at rest...")
			s.Start()
			uuid, err := resolveBucket(client, args[0])
			var resp json.RawMessage
			if err == nil {
				resp, err = client.Put(fmt.Sprintf("/object-storage/buckets/%s/encryption", uuid), map[string]bool{"enabled": true})
			}
			s.Stop()
			if err != nil {
				return err
			}
			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(resp)
			}
			var result struct {
				Detail         string `json:"detail"`
				ReencryptJobID *int   `json:"reencrypt_job_id"`
			}
			_ = json.Unmarshal(resp, &result)
			if result.Detail != "" {
				output.PrintSuccess(result.Detail)
			} else {
				output.PrintSuccess(fmt.Sprintf("Encryption at rest is on for bucket %s", args[0]))
			}
			if result.ReencryptJobID != nil {
				output.PrintInfo("The objects already stored are being encrypted in the background.")
			}
			return nil
		},
	}
	cmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")
	return cmd
}
