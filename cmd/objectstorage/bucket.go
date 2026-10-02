package objectstorage

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/CubePathInc/cubecli/internal/cmdutil"
	"github.com/CubePathInc/cubecli/internal/output"
	"github.com/spf13/cobra"
)

func bucketCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "bucket",
		Aliases: []string{"buckets"},
		Short:   "Manage buckets",
	}
	cmd.AddCommand(
		bucketListCmd(),
		bucketGetCmd(),
		bucketCreateCmd(),
		bucketUpdateCmd(),
		bucketDeleteCmd(),
		bucketMetricsCmd(),
		bucketLifecycleCmd(),
		bucketObjectLockCmd(),
		bucketEncryptionCmd(),
	)
	return cmd
}

// --- list ---

func bucketListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List buckets",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			s := output.NewSpinner("Fetching buckets...")
			s.Start()
			resp, err := client.Get("/object-storage/buckets" + listQuery(cmd))
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			var buckets []struct {
				UUID           string            `json:"uuid"`
				Name           string            `json:"name"`
				Status         string            `json:"status"`
				ProjectID      *int              `json:"project_id"`
				Tier           tierSummary       `json:"tier"`
				Versioning     string            `json:"versioning"`
				Protected      bool              `json:"protected"`
				SizeBytes      int64             `json:"size_bytes"`
				ObjectsCount   int64             `json:"objects_count"`
				MonthlyCharges float64           `json:"monthly_charges"`
				CDNConnected   bool              `json:"cdn_connected"`
				Tags           map[string]string `json:"tags"`
				ObjectLock     objectLock        `json:"object_lock"`
			}
			if err := json.Unmarshal(resp, &buckets); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}

			t := output.NewTable("Buckets", []string{"UUID", "Name", "Status", "Tier", "Project", "Size", "Objects", "Versioning", "Lock", "CDN", "Protected", "This month", "Tags"})
			for _, b := range buckets {
				t.AddRow(
					b.UUID,
					b.Name,
					output.FormatStatus(b.Status),
					b.Tier.Name,
					intPtr(b.ProjectID),
					formatBytes(b.SizeBytes),
					formatCount(b.ObjectsCount),
					b.Versioning,
					formatLockColumn(b.ObjectLock),
					yesNo(b.CDNConnected),
					yesNo(b.Protected),
					formatUSD(b.MonthlyCharges),
					formatTags(b.Tags, maxTagsColumn),
				)
			}
			t.Render()
			return nil
		},
	}
	cmd.Example = `  cubecli s3 bucket list --tag env=prod --tag team`
	cmd.Flags().IntP("project", "p", 0, "Only buckets of this project ID")
	cmd.Flags().String("tier", "", "Only buckets of this tier (slug, uuid or ia)")
	addTagFilterFlag(cmd)
	return cmd
}

// --- get ---

func bucketGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "get <bucket>",
		Aliases: []string{"show"},
		Short:   "Show a bucket with its connection details, month usage and CDN status",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			s := output.NewSpinner("Fetching bucket...")
			s.Start()
			uuid, err := resolveBucket(client, args[0])
			var resp json.RawMessage
			if err == nil {
				resp, err = client.Get("/object-storage/buckets/" + uuid)
			}
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			var b struct {
				UUID           string            `json:"uuid"`
				Name           string            `json:"name"`
				Status         string            `json:"status"`
				SuspendReason  *string           `json:"suspend_reason"`
				ErrorMessage   *string           `json:"error_message"`
				ProjectID      *int              `json:"project_id"`
				Tier           tierSummary       `json:"tier"`
				LocationName   string            `json:"location_name"`
				Versioning     string            `json:"versioning"`
				Protected      bool              `json:"protected"`
				SizeBytes      int64             `json:"size_bytes"`
				ObjectsCount   int64             `json:"objects_count"`
				UsageUpdatedAt *string           `json:"usage_updated_at"`
				MonthlyCharges float64           `json:"monthly_charges"`
				Tags           map[string]string `json:"tags"`
				ObjectLock     objectLock        `json:"object_lock"`
				LockedKept     bool              `json:"locked_content_kept"`
				Encryption     *bucketEncryption `json:"encryption"`
				Connection     struct {
					Endpoint       string `json:"endpoint"`
					Region         string `json:"region"`
					PathStyleURL   string `json:"path_style_url"`
					VirtualHostURL string `json:"virtual_host_url"`
				} `json:"connection"`
				Usage *struct {
					Period         string  `json:"period"`
					StorageGiBMo   float64 `json:"storage_gib_month"`
					EgressBytes    int64   `json:"egress_bytes"`
					CDNBytes       int64   `json:"cdn_bytes"`
					ClassARequests int64   `json:"class_a_requests"`
					ClassBRequests int64   `json:"class_b_requests"`
				} `json:"usage"`
				CDN *struct {
					Status        string  `json:"status"`
					ZoneUUID      string  `json:"zone_uuid"`
					ZoneName      string  `json:"zone_name"`
					Domain        string  `json:"domain"`
					CustomDomain  *string `json:"custom_domain"`
					ZoneStatus    string  `json:"zone_status"`
					OriginEnabled bool    `json:"origin_enabled"`
				} `json:"cdn"`
			}
			if err := json.Unmarshal(resp, &b); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}

			info := output.NewTable("Bucket Info", []string{"Field", "Value"})
			info.AddRow("UUID", b.UUID)
			info.AddRow("Name", b.Name)
			info.AddRow("Status", output.FormatStatus(b.Status))
			if b.SuspendReason != nil && *b.SuspendReason != "" {
				info.AddRow("Suspend reason", *b.SuspendReason)
			}
			if b.ErrorMessage != nil && *b.ErrorMessage != "" {
				info.AddRow("Last error", *b.ErrorMessage)
			}
			info.AddRow("Tier", fmt.Sprintf("%s (%s)", b.Tier.Name, b.Tier.Slug))
			info.AddRow("Location", b.LocationName)
			info.AddRow("Project", intPtr(b.ProjectID))
			info.AddRow("Endpoint", b.Connection.Endpoint)
			info.AddRow("Region", b.Connection.Region)
			info.AddRow("Path style URL", b.Connection.PathStyleURL)
			info.AddRow("Virtual host URL", b.Connection.VirtualHostURL)
			info.AddRow("Versioning", b.Versioning)
			info.AddRow("Object Lock", formatLock(b.ObjectLock))
			info.AddRow("Encryption", formatEncryption(b.Encryption))
			if b.LockedKept {
				info.AddRow("Locked content kept", "yes (the last delete kept versions still under retention or legal hold)")
			}
			info.AddRow("Protected", yesNo(b.Protected))
			info.AddRow("Tags", formatTags(b.Tags, 0))
			info.AddRow("Size", formatBytes(b.SizeBytes))
			info.AddRow("Objects", formatCount(b.ObjectsCount))
			if b.UsageUpdatedAt != nil {
				info.AddRow("Size measured at", *b.UsageUpdatedAt)
			}
			info.AddRow("Charged this month", formatUSD(b.MonthlyCharges))
			info.Render()

			if b.Usage != nil {
				u := output.NewTable(fmt.Sprintf("Usage %s", b.Usage.Period), []string{"Storage (GiB-mo)", "Egress", "CDN", "Class A", "Class B"})
				u.AddRow(
					strconv.FormatFloat(b.Usage.StorageGiBMo, 'f', 4, 64),
					formatBytes(b.Usage.EgressBytes),
					formatBytes(b.Usage.CDNBytes),
					formatCount(b.Usage.ClassARequests),
					formatCount(b.Usage.ClassBRequests),
				)
				u.Render()
			} else {
				output.PrintWarning("Usage metrics are temporarily unavailable.")
			}

			if b.CDN != nil {
				c := output.NewTable("CDN", []string{"Field", "Value"})
				c.AddRow("Status", output.FormatStatus(b.CDN.Status))
				c.AddRow("Zone", fmt.Sprintf("%s (%s)", b.CDN.ZoneName, b.CDN.ZoneUUID))
				c.AddRow("Domain", b.CDN.Domain)
				if b.CDN.CustomDomain != nil && *b.CDN.CustomDomain != "" {
					c.AddRow("Custom domain", *b.CDN.CustomDomain)
				}
				c.AddRow("Zone status", output.FormatStatus(b.CDN.ZoneStatus))
				c.AddRow("Origin enabled", yesNo(b.CDN.OriginEnabled))
				c.Render()
			}
			return nil
		},
	}
}

// --- create ---

func bucketCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a bucket (billed hourly while it exists)",
		Long: `Create a bucket. Names are 3 to 63 characters of lowercase letters, numbers
and hyphens, and are unique across all CubePath customers.

The bucket is created asynchronously: it is usable once its status is active
(usually 10 to 20 seconds). Uploads may answer 503 for the first minutes.

Tags are labels to organize and filter buckets (at most 50; key up to 128 and
value up to 256 characters). They are managed with cubecli, the API and the
dashboard only: S3 bucket tagging calls are not supported.

Encryption at rest (AES-256) is on by default: --no-encryption creates the
bucket without it. It can be enabled later (bucket encryption enable), never
turned off.

--object-lock creates the bucket with Object Lock (WORM): object versions cannot
be deleted or overwritten until their retention date. It can only be turned on
now, never later, and implies versioning and deletion protection. It needs
--accept-object-lock-terms. An optional default retention applies to every new
version:

  governance  keys created with --bypass-governance can still delete
  compliance  nobody can delete or shorten it before its date, CubePath included
              (asks for confirmation unless --yes)`,
		Example: `  cubecli objectstorage bucket create photos --tier ia
  cubecli s3 bucket create backups --tier infrequent_access --project 12 --versioning
  cubecli s3 bucket create logs --tier ia --tag env=prod --tag team=data
  cubecli s3 bucket create scratch --tier ia --no-encryption
  cubecli s3 bucket create veeam --tier ia --object-lock --accept-object-lock-terms
  cubecli s3 bucket create archive --tier ia --object-lock --lock-mode governance --lock-days 30 --accept-object-lock-terms`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			tier, _ := cmd.Flags().GetString("tier")
			projectID, _ := cmd.Flags().GetInt("project")
			versioning, _ := cmd.Flags().GetBool("versioning")
			tagFlags, _ := cmd.Flags().GetStringArray("tag")
			tags, err := parseTags(tagFlags)
			if err != nil {
				return err
			}

			objectLockOn, _ := cmd.Flags().GetBool("object-lock")
			lockMode, _ := cmd.Flags().GetString("lock-mode")
			lockDays, _ := cmd.Flags().GetInt("lock-days")
			lockYears, _ := cmd.Flags().GetInt("lock-years")
			acceptTerms, _ := cmd.Flags().GetBool("accept-object-lock-terms")
			lockDefault, err := parseRetention(lockMode, lockDays, lockYears,
				cmd.Flags().Changed("lock-days"), cmd.Flags().Changed("lock-years"), "lock-")
			if err != nil {
				return err
			}
			if objectLockOn {
				if cmd.Flags().Changed("versioning") && !versioning {
					return fmt.Errorf("--object-lock requires versioning: drop --versioning=false")
				}
				if !acceptTerms {
					return fmt.Errorf("--object-lock requires --accept-object-lock-terms")
				}
				// Object Lock always creates the bucket with versioning enabled.
				versioning = true
			} else if lockDefault != nil {
				return fmt.Errorf("--lock-mode, --lock-days and --lock-years need --object-lock")
			}
			if lockMode == "compliance" {
				ok, err := confirmCompliance(cmd, fmt.Sprintf("Create bucket %s with a compliance default retention?", args[0]))
				if err != nil {
					return err
				}
				if !ok {
					output.PrintWarning("Aborted")
					return nil
				}
			}

			noEncryption, _ := cmd.Flags().GetBool("no-encryption")
			body := map[string]interface{}{
				"name":       args[0],
				"tier":       normalizeTier(tier),
				"versioning": versioning,
				"encryption": !noEncryption,
			}
			if objectLockOn {
				body["object_lock"] = true
				body["accept_object_lock_terms"] = true
				if lockDefault != nil {
					body["object_lock_default"] = lockDefault
				}
			}
			if projectID > 0 {
				body["project_id"] = projectID
			}
			if len(tags) > 0 {
				body["tags"] = tags
			}

			s := output.NewSpinner("Creating bucket...")
			s.Start()
			resp, err := client.Post("/object-storage/buckets", body)
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			var result struct {
				UUID     string `json:"uuid"`
				Name     string `json:"name"`
				Endpoint string `json:"endpoint"`
			}
			if err := json.Unmarshal(resp, &result); err == nil && result.UUID != "" {
				output.PrintSuccess(fmt.Sprintf("Bucket %s is being created: %s", result.Name, result.UUID))
				if result.Endpoint != "" {
					output.PrintInfo(fmt.Sprintf("Endpoint: %s", result.Endpoint))
				}
				if objectLockOn {
					output.PrintInfo("Object Lock is on: the bucket keeps versioning enabled and starts with deletion protection.")
				}
			} else {
				output.PrintSuccess("Bucket creation initiated")
			}
			return nil
		},
	}
	cmd.Flags().String("tier", "", "Storage tier: slug, uuid or ia (see 'objectstorage tiers')")
	cmd.Flags().IntP("project", "p", 0, "Project ID (default: the organization's first project)")
	cmd.Flags().Bool("versioning", false, "Enable object versioning")
	cmd.Flags().StringArray("tag", nil, "Tag as key=value (repeatable)")
	cmd.Flags().Bool("no-encryption", false, "Create the bucket without encryption at rest (it can be enabled later, never turned off)")
	cmd.Flags().Bool("object-lock", false, "Create the bucket with Object Lock (only possible now, never later)")
	cmd.Flags().String("lock-mode", "", "Default retention mode: governance or compliance")
	cmd.Flags().Int("lock-days", 0, "Default retention in days")
	cmd.Flags().Int("lock-years", 0, "Default retention in years")
	cmd.Flags().Bool("accept-object-lock-terms", false, "Accept the Object Lock terms (required with --object-lock)")
	cmd.Flags().BoolP("yes", "y", false, "Skip the compliance confirmation prompt")
	cmd.MarkFlagsMutuallyExclusive("lock-days", "lock-years")
	_ = cmd.MarkFlagRequired("tier")
	return cmd
}

// --- update ---

func bucketUpdateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update <bucket>",
		Short: "Change a bucket's versioning, deletion protection or tags",
		Long: `Change a bucket's versioning, deletion protection or tags.

--tag replaces every tag of the bucket with the ones given; --clear-tags
removes them all. Tags not given are not kept.`,
		Example: `  cubecli s3 bucket update photos --versioning enabled
  cubecli s3 bucket update photos --protected=false
  cubecli s3 bucket update photos --tag env=prod --tag team=web
  cubecli s3 bucket update photos --clear-tags`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			body := map[string]interface{}{}
			if cmd.Flags().Changed("versioning") {
				v, _ := cmd.Flags().GetString("versioning")
				body["versioning"] = v
			}
			if cmd.Flags().Changed("protected") {
				p, _ := cmd.Flags().GetBool("protected")
				body["protected"] = p
			}
			clearTags, _ := cmd.Flags().GetBool("clear-tags")
			if cmd.Flags().Changed("tag") {
				if clearTags {
					return fmt.Errorf("--tag and --clear-tags cannot be used together")
				}
				tagFlags, _ := cmd.Flags().GetStringArray("tag")
				tags, err := parseTags(tagFlags)
				if err != nil {
					return err
				}
				body["tags"] = tags
			} else if clearTags {
				body["tags"] = map[string]string{}
			}
			if len(body) == 0 {
				return fmt.Errorf("at least one of --versioning, --protected, --tag or --clear-tags must be specified")
			}

			s := output.NewSpinner("Updating bucket...")
			s.Start()
			uuid, err := resolveBucket(client, args[0])
			var resp json.RawMessage
			if err == nil {
				resp, err = client.Patch("/object-storage/buckets/"+uuid, body)
			}
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			output.PrintSuccess("Bucket updated")
			return nil
		},
	}
	cmd.Flags().String("versioning", "", "enabled or suspended (versioning cannot be turned off once enabled)")
	cmd.Flags().Bool("protected", false, "Deletion protection: --protected or --protected=false")
	cmd.Flags().StringArray("tag", nil, "Tag as key=value (repeatable); replaces every tag of the bucket")
	cmd.Flags().Bool("clear-tags", false, "Remove every tag of the bucket")
	return cmd
}

// --- delete ---

func bucketDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <bucket>",
		Short: "Delete a bucket",
		Long: `Delete a bucket. Without --purge only an empty bucket is deleted; otherwise
the bucket stays active and shows the error. With --purge every object, version
and pending upload is deleted first, which cannot be undone.

On a bucket with Object Lock, versions still under retention or legal hold are
kept: the bucket stays with "Locked content kept" until their retention ends,
and keeps being billed. --bypass-governance (with --purge) also deletes the
versions under governance retention; compliance versions can never be deleted
early.

The bucket name stays reserved for your organization for 90 days.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			purge, _ := cmd.Flags().GetBool("purge")
			bypass, _ := cmd.Flags().GetBool("bypass-governance")
			if bypass && !purge {
				return fmt.Errorf("--bypass-governance can only be used together with --purge")
			}
			msg := fmt.Sprintf("Are you sure you want to delete bucket %s?", args[0])
			if purge {
				msg = fmt.Sprintf("Delete bucket %s and ALL its objects and versions? This cannot be undone.", args[0])
			}
			if bypass {
				msg = fmt.Sprintf("Delete bucket %s and ALL its objects and versions, including those under governance retention? This cannot be undone.", args[0])
			}
			if !cmdutil.CheckForce(cmd, msg) {
				output.PrintWarning("Aborted")
				return nil
			}

			client := cmdutil.GetClient(cmd)

			path := ""
			s := output.NewSpinner("Deleting bucket...")
			s.Start()
			uuid, err := resolveBucket(client, args[0])
			var resp json.RawMessage
			if err == nil {
				path = "/object-storage/buckets/" + uuid
				if purge {
					path += "?force=true"
				}
				if bypass {
					path += "&bypass_governance=true"
				}
				resp, err = client.Delete(path)
			}
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			output.PrintSuccess("Bucket deletion started")
			if !purge {
				output.PrintInfo("If the bucket is not empty it stays active with an error; delete its objects or use --purge.")
			}
			return nil
		},
	}
	cmd.Flags().Bool("purge", false, "Also delete every object and version in the bucket (the API's force delete)")
	cmd.Flags().Bool("bypass-governance", false, "With --purge on a bucket with Object Lock: also delete versions under governance retention")
	cmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")
	return cmd
}

func intPtr(v *int) string {
	if v == nil {
		return "-"
	}
	return strconv.Itoa(*v)
}
