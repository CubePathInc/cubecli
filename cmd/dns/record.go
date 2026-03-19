package dns

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/CubePathInc/cubecli/internal/cmdutil"
	"github.com/CubePathInc/cubecli/internal/output"
	"github.com/spf13/cobra"
)

func addRecordCmd(parent *cobra.Command) {
	recordCmd := &cobra.Command{
		Use:   "record",
		Short: "Manage DNS records",
	}

	recordListCmd := &cobra.Command{
		Use:   "list <zone_uuid>",
		Short: "List DNS records in a zone",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			path := fmt.Sprintf("/dns/zones/%s/records", args[0])
			recordType, _ := cmd.Flags().GetString("type")
			if recordType != "" {
				path = fmt.Sprintf("%s?record_type=%s", path, recordType)
			}

			s := output.NewSpinner("Fetching DNS records...")
			s.Start()
			resp, err := client.Get(path)
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			var records []struct {
				UUID     string `json:"uuid"`
				Name     string `json:"name"`
				Type     string `json:"record_type"`
				Content  string `json:"content"`
				TTL      int    `json:"ttl"`
				Priority *int   `json:"priority"`
			}
			if err := json.Unmarshal(resp, &records); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}

			t := output.NewTable("DNS Records", []string{"Name", "Type", "Content", "TTL", "Priority", "UUID"})
			for _, r := range records {
				priority := ""
				if r.Priority != nil {
					priority = strconv.Itoa(*r.Priority)
				}
				t.AddRow(r.Name, r.Type, r.Content, strconv.Itoa(r.TTL), priority, r.UUID)
			}
			t.Render()
			return nil
		},
	}

	recordCreateCmd := &cobra.Command{
		Use:   "create <zone_uuid>",
		Short: "Create a DNS record",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			name, _ := cmd.Flags().GetString("name")
			recordType, _ := cmd.Flags().GetString("type")
			content, _ := cmd.Flags().GetString("content")
			ttl, _ := cmd.Flags().GetInt("ttl")

			body := map[string]interface{}{
				"name":        name,
				"record_type": strings.ToUpper(recordType),
				"content":     content,
				"ttl":         ttl,
			}

			if cmd.Flags().Changed("priority") {
				v, _ := cmd.Flags().GetInt("priority")
				body["priority"] = v
			}
			if cmd.Flags().Changed("weight") {
				v, _ := cmd.Flags().GetInt("weight")
				body["weight"] = v
			}
			if cmd.Flags().Changed("port") {
				v, _ := cmd.Flags().GetInt("port")
				body["port"] = v
			}
			if cmd.Flags().Changed("comment") {
				v, _ := cmd.Flags().GetString("comment")
				body["comment"] = v
			}

			s := output.NewSpinner("Creating DNS record...")
			s.Start()
			resp, err := client.Post(fmt.Sprintf("/dns/zones/%s/records", args[0]), body)
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			output.PrintSuccess("DNS record created successfully")
			return nil
		},
	}

	recordUpdateCmd := &cobra.Command{
		Use:   "update <zone_uuid> <record_uuid>",
		Short: "Update a DNS record",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			body := map[string]interface{}{}

			if cmd.Flags().Changed("content") {
				v, _ := cmd.Flags().GetString("content")
				body["content"] = v
			}
			if cmd.Flags().Changed("ttl") {
				v, _ := cmd.Flags().GetInt("ttl")
				body["ttl"] = v
			}
			if cmd.Flags().Changed("priority") {
				v, _ := cmd.Flags().GetInt("priority")
				body["priority"] = v
			}
			if cmd.Flags().Changed("weight") {
				v, _ := cmd.Flags().GetInt("weight")
				body["weight"] = v
			}
			if cmd.Flags().Changed("port") {
				v, _ := cmd.Flags().GetInt("port")
				body["port"] = v
			}
			if cmd.Flags().Changed("comment") {
				v, _ := cmd.Flags().GetString("comment")
				body["comment"] = v
			}

			if len(body) == 0 {
				return fmt.Errorf("no fields to update; provide at least one flag")
			}

			s := output.NewSpinner("Updating DNS record...")
			s.Start()
			resp, err := client.Put(fmt.Sprintf("/dns/zones/%s/records/%s", args[0], args[1]), body)
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			output.PrintSuccess("DNS record updated successfully")
			return nil
		},
	}

	recordDeleteCmd := &cobra.Command{
		Use:   "delete <zone_uuid> <record_uuid>",
		Short: "Delete a DNS record",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			if !cmdutil.CheckForce(cmd, "Are you sure you want to delete this DNS record?") {
				output.PrintWarning("Aborted")
				return nil
			}

			s := output.NewSpinner("Deleting DNS record...")
			s.Start()
			resp, err := client.Delete(fmt.Sprintf("/dns/zones/%s/records/%s", args[0], args[1]))
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			output.PrintSuccess("DNS record deleted successfully")
			return nil
		},
	}

	recordListCmd.Flags().StringP("type", "t", "", "Filter by record type")

	recordCreateCmd.Flags().StringP("name", "n", "", "Record name")
	recordCreateCmd.Flags().StringP("type", "t", "", "Record type (A, AAAA, CNAME, MX, TXT, etc.)")
	recordCreateCmd.Flags().StringP("content", "c", "", "Record content")
	recordCreateCmd.Flags().Int("ttl", 3600, "Time to live in seconds")
	recordCreateCmd.Flags().Int("priority", 0, "Record priority (for MX, SRV)")
	recordCreateCmd.Flags().Int("weight", 0, "Record weight (for SRV)")
	recordCreateCmd.Flags().Int("port", 0, "Record port (for SRV)")
	recordCreateCmd.Flags().String("comment", "", "Record comment")
	recordCreateCmd.MarkFlagRequired("name")
	recordCreateCmd.MarkFlagRequired("type")
	recordCreateCmd.MarkFlagRequired("content")

	recordUpdateCmd.Flags().StringP("content", "c", "", "Record content")
	recordUpdateCmd.Flags().Int("ttl", 0, "Time to live in seconds")
	recordUpdateCmd.Flags().Int("priority", 0, "Record priority")
	recordUpdateCmd.Flags().Int("weight", 0, "Record weight")
	recordUpdateCmd.Flags().Int("port", 0, "Record port")
	recordUpdateCmd.Flags().String("comment", "", "Record comment")

	recordDeleteCmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")

	recordCmd.AddCommand(recordListCmd, recordCreateCmd, recordUpdateCmd, recordDeleteCmd)
	parent.AddCommand(recordCmd)
}
