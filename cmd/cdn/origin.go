package cdn

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/CubePathInc/cubecli/cmd/objectstorage"
	"github.com/CubePathInc/cubecli/internal/cmdutil"
	"github.com/CubePathInc/cubecli/internal/output"
	"github.com/spf13/cobra"
)

func addOriginCmd(parent *cobra.Command) {
	originCmd := &cobra.Command{
		Use:   "origin",
		Short: "Manage CDN origins",
	}

	originListCmd := &cobra.Command{
		Use:   "list <zone_uuid>",
		Short: "List CDN origins for a zone",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			zoneUUID := args[0]

			s := output.NewSpinner("Fetching CDN origins...")
			s.Start()
			resp, err := client.Get(fmt.Sprintf("/cdn/zones/%s/origins", zoneUUID))
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			var origins []struct {
				UUID     string `json:"uuid"`
				Name     string `json:"name"`
				Address  string `json:"address"`
				Port     int    `json:"port"`
				Protocol string `json:"protocol"`
				Weight   int    `json:"weight"`
				Priority int    `json:"priority"`
				Backup   bool   `json:"is_backup"`
				Health   string `json:"health_status"`
				CheckOn  *bool  `json:"health_check_enabled"`
				Check    string `json:"health_check_type"`
				Detail   string `json:"health_detail"`
				Enabled  bool   `json:"enabled"`
				Bucket   string `json:"object_storage_bucket_uuid"`
			}
			if err := json.Unmarshal(resp, &origins); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}

			t := output.NewTable("CDN Origins", []string{"UUID", "Name", "Address", "Port", "Protocol", "Weight", "Priority", "Backup", "Health", "Check", "Detail", "Enabled", "Bucket"})
			// The nodes keep one check per zone, the one of the first origin that has it on, and
			// apply it to every enabled origin: the API says "unknown" for an origin nobody
			// checks and for one no node has checked yet, and only the first is "not checked".
			zoneChecked := false
			for _, o := range origins {
				if o.Enabled && o.CheckOn != nil && *o.CheckOn {
					zoneChecked = true
				}
			}
			for _, o := range origins {
				if !zoneChecked && o.CheckOn != nil {
					o.Health = "not checked"
				}
				t.AddRow(
					o.UUID,
					o.Name,
					o.Address,
					strconv.Itoa(o.Port),
					o.Protocol,
					strconv.Itoa(o.Weight),
					strconv.Itoa(o.Priority),
					strconv.FormatBool(o.Backup),
					o.Health,
					o.Check,
					o.Detail,
					strconv.FormatBool(o.Enabled),
					o.Bucket,
				)
			}
			t.Render()
			return nil
		},
	}

	originCreateCmd := &cobra.Command{
		Use:   "create <zone_uuid>",
		Short: "Create a new CDN origin",
		Long: `Create a CDN origin. An origin is either an address (--url or --address) or
one of your Object Storage buckets (--bucket): the CDN then reads the bucket with a
read-only key it manages, and deleting the origin disconnects the bucket. A bucket
can be the origin of one zone at a time.

The health check is a TCP connect by default. --health-type http makes the edge request
--health-path and take the origin out when the answer is not one of --health-expect (if it
is the only origin of the zone, the zone answers 503). All the origins of a zone share one
check: the one of the first origin by priority that has its check enabled.`,
		Example: `  cubecli cdn origin create <zone_uuid> --name web --url https://origin.example.com
  cubecli cdn origin create <zone_uuid> --name web --url https://origin.example.com --health-type http --health-path /ping --health-expect 200
  cubecli cdn origin create <zone_uuid> --name photos --bucket photos`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			zoneUUID := args[0]

			name, _ := cmd.Flags().GetString("name")
			originURL, _ := cmd.Flags().GetString("url")
			address, _ := cmd.Flags().GetString("address")
			port, _ := cmd.Flags().GetInt("port")
			protocol, _ := cmd.Flags().GetString("protocol")
			weight, _ := cmd.Flags().GetInt("weight")
			priority, _ := cmd.Flags().GetInt("priority")
			backup, _ := cmd.Flags().GetBool("backup")
			healthPath, _ := cmd.Flags().GetString("health-path")
			noHealthCheck, _ := cmd.Flags().GetBool("no-health-check")
			noVerifySSL, _ := cmd.Flags().GetBool("no-verify-ssl")
			hostHeader, _ := cmd.Flags().GetString("host-header")
			basePath, _ := cmd.Flags().GetString("base-path")
			bucket, _ := cmd.Flags().GetString("bucket")

			if bucket == "" && originURL == "" && address == "" {
				return fmt.Errorf("one of --url, --address or --bucket is required")
			}

			body := map[string]interface{}{
				"name":                 name,
				"weight":               weight,
				"priority":             priority,
				"is_backup":            backup,
				"health_check_enabled": !noHealthCheck,
				"health_check_path":    healthPath,
				"verify_ssl":           !noVerifySSL,
				"enabled":              true,
			}
			if bucket != "" {
				// The API fills everything else of a bucket origin (connection, health check,
				// enabled) and refuses those fields: only name, weight, priority and backup go.
				for _, f := range append([]string{"health-path", "no-health-check"}, healthCheckFlags...) {
					if cmd.Flags().Changed(f) {
						return fmt.Errorf("--health-path, --no-health-check and the --health-* flags cannot be used with --bucket")
					}
				}
				for _, k := range []string{"health_check_enabled", "health_check_path", "verify_ssl", "enabled"} {
					delete(body, k)
				}
			}
			if err := addHealthCheckFlags(cmd, body); err != nil {
				return err
			}
			// An http check with no --health-path goes to "/" (the API decides when the path is
			// not sent); the "/health" of the flag is only what a tcp check stores.
			if body["health_check_type"] == "http" && !cmd.Flags().Changed("health-path") {
				delete(body, "health_check_path")
			}
			if originURL != "" {
				body["origin_url"] = originURL
			}
			if address != "" {
				body["address"] = address
			}
			if port > 0 {
				body["port"] = port
			}
			if protocol != "" {
				body["protocol"] = protocol
			}
			if hostHeader != "" {
				body["host_header"] = hostHeader
			}
			if basePath != "" {
				body["base_path"] = basePath
			}

			s := output.NewSpinner("Creating CDN origin...")
			s.Start()
			var resp json.RawMessage
			var err error
			if bucket != "" {
				var bucketUUID string
				bucketUUID, err = objectstorage.ResolveBucket(client, bucket)
				body["object_storage_bucket_uuid"] = bucketUUID
			}
			if err == nil {
				resp, err = client.Post(fmt.Sprintf("/cdn/zones/%s/origins", zoneUUID), body)
			}
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			var origin struct {
				UUID    string `json:"uuid"`
				Name    string `json:"name"`
				Address string `json:"address"`
				Bucket  string `json:"object_storage_bucket_uuid"`
			}
			if err := json.Unmarshal(resp, &origin); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}

			t := output.NewTable("CDN Origin Created", []string{"Field", "Value"})
			t.AddRow("UUID", origin.UUID)
			t.AddRow("Name", origin.Name)
			t.AddRow("Address", origin.Address)
			if origin.Bucket != "" {
				t.AddRow("Bucket", origin.Bucket)
			}
			t.Render()

			output.PrintSuccess("CDN origin created successfully")
			return nil
		},
	}

	originUpdateCmd := &cobra.Command{
		Use:   "update <zone_uuid> <origin_uuid>",
		Short: "Update a CDN origin",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			zoneUUID := args[0]
			originUUID := args[1]

			body := map[string]interface{}{}
			if cmd.Flags().Changed("name") {
				name, _ := cmd.Flags().GetString("name")
				body["name"] = name
			}
			if cmd.Flags().Changed("address") {
				address, _ := cmd.Flags().GetString("address")
				body["address"] = address
			}
			if cmd.Flags().Changed("port") {
				port, _ := cmd.Flags().GetInt("port")
				body["port"] = port
			}
			if cmd.Flags().Changed("protocol") {
				protocol, _ := cmd.Flags().GetString("protocol")
				body["protocol"] = protocol
			}
			if cmd.Flags().Changed("weight") {
				weight, _ := cmd.Flags().GetInt("weight")
				body["weight"] = weight
			}
			if cmd.Flags().Changed("priority") {
				priority, _ := cmd.Flags().GetInt("priority")
				body["priority"] = priority
			}
			if cmd.Flags().Changed("host-header") {
				hostHeader, _ := cmd.Flags().GetString("host-header")
				body["host_header"] = hostHeader
			}
			if cmd.Flags().Changed("base-path") {
				basePath, _ := cmd.Flags().GetString("base-path")
				body["base_path"] = basePath
			}

			if err := addHealthCheckFlags(cmd, body); err != nil {
				return err
			}

			if len(body) == 0 {
				return fmt.Errorf("at least one flag must be specified")
			}

			s := output.NewSpinner("Updating CDN origin...")
			s.Start()
			resp, err := client.Patch(fmt.Sprintf("/cdn/zones/%s/origins/%s", zoneUUID, originUUID), body)
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			output.PrintSuccess("CDN origin updated successfully")
			return nil
		},
	}

	originDeleteCmd := &cobra.Command{
		Use:   "delete <zone_uuid> <origin_uuid>",
		Short: "Delete a CDN origin",
		Long: `Delete a CDN origin. If the origin is an Object Storage bucket, this
disconnects the bucket: the CDN stops reading it and its public URLs stop working.`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			zoneUUID := args[0]
			originUUID := args[1]

			if !cmdutil.CheckForce(cmd, fmt.Sprintf("Are you sure you want to delete CDN origin %s?", originUUID)) {
				output.PrintWarning("Aborted")
				return nil
			}

			s := output.NewSpinner("Deleting CDN origin...")
			s.Start()
			resp, err := client.Delete(fmt.Sprintf("/cdn/zones/%s/origins/%s", zoneUUID, originUUID))
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			output.PrintSuccess("CDN origin deleted successfully")
			return nil
		},
	}

	// Origin create flags
	originCreateCmd.Flags().StringP("name", "n", "", "Name of the origin")
	originCreateCmd.Flags().StringP("url", "u", "", "Origin URL")
	originCreateCmd.Flags().StringP("address", "a", "", "Origin address")
	originCreateCmd.Flags().IntP("port", "p", 0, "Origin port")
	originCreateCmd.Flags().String("protocol", "", "Origin protocol")
	originCreateCmd.Flags().IntP("weight", "w", 100, "Origin weight")
	originCreateCmd.Flags().Int("priority", 1, "Origin priority")
	originCreateCmd.Flags().Bool("backup", false, "Mark origin as backup")
	originCreateCmd.Flags().String("health-path", "/health", "Health check path")
	originCreateCmd.Flags().Bool("no-health-check", false, "Disable health checks")
	addHealthCheckFlagDefs(originCreateCmd)
	originCreateCmd.Flags().Bool("no-verify-ssl", false, "Disable SSL verification")
	originCreateCmd.Flags().String("host-header", "", "Host header override")
	originCreateCmd.Flags().String("base-path", "", "Base path for the origin")
	originCreateCmd.Flags().String("bucket", "", "Object Storage bucket (name or uuid) to serve through this zone")
	originCreateCmd.MarkFlagRequired("name")
	for _, f := range []string{"url", "address", "port", "protocol", "host-header", "no-verify-ssl"} {
		originCreateCmd.MarkFlagsMutuallyExclusive("bucket", f)
	}

	// Origin update flags
	originUpdateCmd.Flags().StringP("name", "n", "", "New name for the origin")
	originUpdateCmd.Flags().StringP("address", "a", "", "New address")
	originUpdateCmd.Flags().IntP("port", "p", 0, "New port")
	originUpdateCmd.Flags().String("protocol", "", "New protocol")
	originUpdateCmd.Flags().IntP("weight", "w", 0, "New weight")
	originUpdateCmd.Flags().Int("priority", 0, "New priority")
	originUpdateCmd.Flags().String("host-header", "", "New host header override")
	originUpdateCmd.Flags().String("base-path", "", "New base path")
	addHealthCheckFlagDefs(originUpdateCmd)

	// Origin delete flags
	originDeleteCmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")

	originCmd.AddCommand(originListCmd, originCreateCmd, originUpdateCmd, originDeleteCmd)
	parent.AddCommand(originCmd)
}

// healthCheckFlags are the flags of the origin health check; they are sent only when set, so the
// API applies its own defaults.
var healthCheckFlags = []string{"health-type", "health-method", "health-expect", "health-interval", "health-threshold"}

func addHealthCheckFlagDefs(cmd *cobra.Command) {
	cmd.Flags().String("health-type", "", "Health check type: tcp (default) or http")
	cmd.Flags().String("health-method", "", "Method of an http health check: GET (default) or HEAD")
	cmd.Flags().String("health-expect", "", `Statuses or classes an http health check accepts, up to 8 separated by spaces (default "2xx 3xx"), e.g. "200 301"`)
	cmd.Flags().Int("health-interval", 0, "Seconds between health checks from each edge node, 5 to 300 (default 10)")
	cmd.Flags().Int("health-threshold", 0, "Results in a row that change the health status, 1 to 10 (default 3)")
}

// addHealthCheckFlags copies the health check flags that were set into the request body.
func addHealthCheckFlags(cmd *cobra.Command, body map[string]interface{}) error {
	if cmd.Flags().Changed("health-type") {
		v, _ := cmd.Flags().GetString("health-type")
		if v != "tcp" && v != "http" {
			return fmt.Errorf("--health-type must be tcp or http")
		}
		body["health_check_type"] = v
	}
	if cmd.Flags().Changed("health-method") {
		v, _ := cmd.Flags().GetString("health-method")
		if v != "GET" && v != "HEAD" {
			return fmt.Errorf("--health-method must be GET or HEAD")
		}
		body["health_check_method"] = v
	}
	if cmd.Flags().Changed("health-expect") {
		v, _ := cmd.Flags().GetString("health-expect")
		body["health_check_expected_status"] = v
	}
	if cmd.Flags().Changed("health-interval") {
		v, _ := cmd.Flags().GetInt("health-interval")
		body["health_check_interval"] = v
	}
	if cmd.Flags().Changed("health-threshold") {
		v, _ := cmd.Flags().GetInt("health-threshold")
		body["health_check_threshold"] = v
	}
	return nil
}
