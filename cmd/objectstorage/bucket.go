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
		bucketCDNCmd(),
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
				UUID           string      `json:"uuid"`
				Name           string      `json:"name"`
				Status         string      `json:"status"`
				ProjectID      *int        `json:"project_id"`
				Tier           tierSummary `json:"tier"`
				Versioning     string      `json:"versioning"`
				Protected      bool        `json:"protected"`
				SizeBytes      int64       `json:"size_bytes"`
				ObjectsCount   int64       `json:"objects_count"`
				MonthlyCharges float64     `json:"monthly_charges"`
				CDNConnected   bool        `json:"cdn_connected"`
			}
			if err := json.Unmarshal(resp, &buckets); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}

			t := output.NewTable("Buckets", []string{"UUID", "Name", "Status", "Tier", "Project", "Size", "Objects", "Versioning", "CDN", "Protected", "This month"})
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
					yesNo(b.CDNConnected),
					yesNo(b.Protected),
					formatUSD(b.MonthlyCharges),
				)
			}
			t.Render()
			return nil
		},
	}
	cmd.Flags().IntP("project", "p", 0, "Only buckets of this project ID")
	cmd.Flags().String("tier", "", "Only buckets of this tier (slug, uuid or ia)")
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
				UUID           string      `json:"uuid"`
				Name           string      `json:"name"`
				Status         string      `json:"status"`
				SuspendReason  *string     `json:"suspend_reason"`
				ErrorMessage   *string     `json:"error_message"`
				ProjectID      *int        `json:"project_id"`
				Tier           tierSummary `json:"tier"`
				LocationName   string      `json:"location_name"`
				Versioning     string      `json:"versioning"`
				Protected      bool        `json:"protected"`
				SizeBytes      int64       `json:"size_bytes"`
				ObjectsCount   int64       `json:"objects_count"`
				UsageUpdatedAt *string     `json:"usage_updated_at"`
				MonthlyCharges float64     `json:"monthly_charges"`
				CreatedAt      string      `json:"created_at"`
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
			info.AddRow("Protected", yesNo(b.Protected))
			info.AddRow("Size", formatBytes(b.SizeBytes))
			info.AddRow("Objects", formatCount(b.ObjectsCount))
			if b.UsageUpdatedAt != nil {
				info.AddRow("Size measured at", *b.UsageUpdatedAt)
			}
			info.AddRow("Charged this month", formatUSD(b.MonthlyCharges))
			info.AddRow("Created", b.CreatedAt)
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
(usually 10 to 20 seconds). Uploads may answer 503 for the first minutes.`,
		Example: `  cubecli objectstorage bucket create photos --tier ia
  cubecli s3 bucket create backups --tier infrequent_access --project 12 --versioning`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			tier, _ := cmd.Flags().GetString("tier")
			projectID, _ := cmd.Flags().GetInt("project")
			versioning, _ := cmd.Flags().GetBool("versioning")

			body := map[string]interface{}{
				"name":       args[0],
				"tier":       normalizeTier(tier),
				"versioning": versioning,
			}
			if projectID > 0 {
				body["project_id"] = projectID
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
			} else {
				output.PrintSuccess("Bucket creation initiated")
			}
			return nil
		},
	}
	cmd.Flags().String("tier", "", "Storage tier: slug, uuid or ia (see 'objectstorage tiers')")
	cmd.Flags().IntP("project", "p", 0, "Project ID (default: the organization's first project)")
	cmd.Flags().Bool("versioning", false, "Enable object versioning")
	_ = cmd.MarkFlagRequired("tier")
	return cmd
}

// --- update ---

func bucketUpdateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update <bucket>",
		Short: "Change a bucket's versioning or deletion protection",
		Example: `  cubecli s3 bucket update photos --versioning enabled
  cubecli s3 bucket update photos --protected=false`,
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
			if len(body) == 0 {
				return fmt.Errorf("at least one of --versioning or --protected must be specified")
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

The bucket name stays reserved for your organization for 90 days.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			purge, _ := cmd.Flags().GetBool("purge")
			msg := fmt.Sprintf("Are you sure you want to delete bucket %s?", args[0])
			if purge {
				msg = fmt.Sprintf("Delete bucket %s and ALL its objects and versions? This cannot be undone.", args[0])
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
	cmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")
	return cmd
}

// --- cdn ---

func bucketCDNCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cdn",
		Short: "Serve a bucket publicly through the CubePath CDN",
	}
	cmd.AddCommand(bucketCDNConnectCmd(), bucketCDNDisconnectCmd())
	return cmd
}

func bucketCDNConnectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "connect <bucket>",
		Short: "Connect a bucket to a CDN zone (creates a billable zone the first time)",
		Long: `Connect a bucket to the CubePath CDN. The first connection creates a CDN zone
(--zone-name and --plan required) that is billed like any CDN zone. A bucket
that was connected before reuses its zone and ignores these flags.

Traffic from the bucket to the CDN is not billed as egress; the edges' requests
are class B requests of the bucket.`,
		Example: `  cubecli s3 bucket cdn connect photos --zone-name photos --plan <plan>
  cubecli s3 bucket cdn connect photos --zone-name photos --plan <plan> --custom-domain cdn.example.com`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			body := map[string]interface{}{}
			if v, _ := cmd.Flags().GetString("zone-name"); v != "" {
				body["zone_name"] = v
			}
			if v, _ := cmd.Flags().GetString("plan"); v != "" {
				body["plan_name"] = v
			}
			if v, _ := cmd.Flags().GetString("custom-domain"); v != "" {
				body["custom_domain"] = v
			}

			s := output.NewSpinner("Connecting bucket to the CDN...")
			s.Start()
			uuid, err := resolveBucket(client, args[0])
			var resp json.RawMessage
			if err == nil {
				resp, err = client.Post("/object-storage/buckets/"+uuid+"/cdn", body)
			}
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			var result struct {
				ZoneUUID     string  `json:"zone_uuid"`
				ZoneName     string  `json:"zone_name"`
				Domain       string  `json:"domain"`
				CustomDomain *string `json:"custom_domain"`
				ReusedZone   bool    `json:"reused_zone"`
			}
			if err := json.Unmarshal(resp, &result); err != nil || result.ZoneUUID == "" {
				output.PrintSuccess("CDN connection started")
				return nil
			}
			if result.ReusedZone {
				output.PrintSuccess(fmt.Sprintf("CDN connection started, reusing zone %s (%s)", result.ZoneName, result.ZoneUUID))
			} else {
				output.PrintSuccess(fmt.Sprintf("CDN connection started, zone %s created (%s)", result.ZoneName, result.ZoneUUID))
			}
			output.PrintInfo(fmt.Sprintf("Public URL: https://%s/<object key>", result.Domain))
			if result.CustomDomain != nil && *result.CustomDomain != "" {
				output.PrintInfo(fmt.Sprintf("Custom domain: %s (point it to the zone and request SSL with 'cdn zone request-ssl')", *result.CustomDomain))
			}
			return nil
		},
	}
	cmd.Flags().String("zone-name", "", "Name of the CDN zone to create (first connection only)")
	cmd.Flags().String("plan", "", "CDN plan name (first connection only; see 'cdn plan list')")
	cmd.Flags().String("custom-domain", "", "Optional custom domain for the new zone")
	return cmd
}

func bucketCDNDisconnectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "disconnect <bucket>",
		Short: "Disconnect a bucket from its CDN zone (the zone is kept)",
		Long: `Disconnect a bucket from the CDN. The CDN stops reading the bucket, but the
zone keeps existing (and billing) with its plan, domain and rules: delete it
with 'cdn zone delete' if it is no longer needed, or reconnect the bucket later.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmdutil.CheckForce(cmd, fmt.Sprintf("Disconnect bucket %s from the CDN? Its public URLs stop working.", args[0])) {
				output.PrintWarning("Aborted")
				return nil
			}

			client := cmdutil.GetClient(cmd)

			s := output.NewSpinner("Disconnecting bucket from the CDN...")
			s.Start()
			uuid, err := resolveBucket(client, args[0])
			var resp json.RawMessage
			if err == nil {
				resp, err = client.Delete("/object-storage/buckets/" + uuid + "/cdn")
			}
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			output.PrintSuccess("CDN disconnection started")
			output.PrintInfo("The CDN zone is kept and still billed; delete it with 'cdn zone delete' if unwanted.")
			return nil
		},
	}
	cmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")
	return cmd
}

func intPtr(v *int) string {
	if v == nil {
		return "-"
	}
	return strconv.Itoa(*v)
}
