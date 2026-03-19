package cdn

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/CubePathInc/cubecli/internal/cmdutil"
	"github.com/CubePathInc/cubecli/internal/output"
	"github.com/spf13/cobra"
)

func addWAFCmd(parent *cobra.Command) {
	wafCmd := &cobra.Command{
		Use:   "waf",
		Short: "Manage CDN WAF rules",
	}

	wafListCmd := &cobra.Command{
		Use:   "list <zone_uuid>",
		Short: "List WAF rules for a zone",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			zoneUUID := args[0]

			s := output.NewSpinner("Fetching WAF rules...")
			s.Start()
			resp, err := client.Get(fmt.Sprintf("/cdn/zones/%s/waf-rules", zoneUUID))
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			var rules []struct {
				UUID     string `json:"uuid"`
				Name     string `json:"name"`
				Type     string `json:"rule_type"`
				Priority int    `json:"priority"`
				Enabled  bool   `json:"enabled"`
			}
			if err := json.Unmarshal(resp, &rules); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}

			t := output.NewTable("WAF Rules", []string{"UUID", "Name", "Type", "Priority", "Enabled"})
			for _, r := range rules {
				t.AddRow(
					r.UUID,
					r.Name,
					r.Type,
					strconv.Itoa(r.Priority),
					strconv.FormatBool(r.Enabled),
				)
			}
			t.Render()
			return nil
		},
	}

	wafShowCmd := &cobra.Command{
		Use:   "show <zone_uuid> <rule_uuid>",
		Short: "Show WAF rule details",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			zoneUUID := args[0]
			ruleUUID := args[1]

			s := output.NewSpinner("Fetching WAF rule details...")
			s.Start()
			resp, err := client.Get(fmt.Sprintf("/cdn/zones/%s/waf-rules/%s", zoneUUID, ruleUUID))
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			var rule struct {
				UUID            string                 `json:"uuid"`
				Name            string                 `json:"name"`
				Type            string                 `json:"rule_type"`
				Priority        int                    `json:"priority"`
				Enabled         bool                   `json:"enabled"`
				ActionConfig    map[string]interface{} `json:"action_config"`
				MatchConditions map[string]interface{} `json:"match_conditions"`
			}
			if err := json.Unmarshal(resp, &rule); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}

			t := output.NewTable("WAF Rule Details", []string{"Field", "Value"})
			t.AddRow("UUID", rule.UUID)
			t.AddRow("Name", rule.Name)
			t.AddRow("Type", rule.Type)
			t.AddRow("Priority", strconv.Itoa(rule.Priority))
			t.AddRow("Enabled", strconv.FormatBool(rule.Enabled))

			if rule.ActionConfig != nil {
				actionJSON, _ := json.MarshalIndent(rule.ActionConfig, "", "  ")
				t.AddRow("Action Config", string(actionJSON))
			}
			if rule.MatchConditions != nil {
				matchJSON, _ := json.MarshalIndent(rule.MatchConditions, "", "  ")
				t.AddRow("Match Conditions", string(matchJSON))
			}
			t.Render()
			return nil
		},
	}

	wafCreateCmd := &cobra.Command{
		Use:   "create <zone_uuid>",
		Short: "Create a new WAF rule",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			zoneUUID := args[0]

			name, _ := cmd.Flags().GetString("name")
			ruleType, _ := cmd.Flags().GetString("type")
			priority, _ := cmd.Flags().GetInt("priority")
			actionStr, _ := cmd.Flags().GetString("action")
			matchStr, _ := cmd.Flags().GetString("match")
			disabled, _ := cmd.Flags().GetBool("disabled")

			var actionConfig map[string]interface{}
			if err := json.Unmarshal([]byte(actionStr), &actionConfig); err != nil {
				return fmt.Errorf("invalid JSON for --action: %w", err)
			}

			body := map[string]interface{}{
				"name":          name,
				"rule_type":     ruleType,
				"priority":      priority,
				"action_config": actionConfig,
				"enabled":       !disabled,
			}

			if matchStr != "" {
				var matchConditions map[string]interface{}
				if err := json.Unmarshal([]byte(matchStr), &matchConditions); err != nil {
					return fmt.Errorf("invalid JSON for --match: %w", err)
				}
				body["match_conditions"] = matchConditions
			}

			s := output.NewSpinner("Creating WAF rule...")
			s.Start()
			resp, err := client.Post(fmt.Sprintf("/cdn/zones/%s/waf-rules", zoneUUID), body)
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			var rule struct {
				UUID string `json:"uuid"`
				Name string `json:"name"`
				Type string `json:"rule_type"`
			}
			if err := json.Unmarshal(resp, &rule); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}

			t := output.NewTable("WAF Rule Created", []string{"Field", "Value"})
			t.AddRow("UUID", rule.UUID)
			t.AddRow("Name", rule.Name)
			t.AddRow("Type", rule.Type)
			t.Render()

			output.PrintSuccess("WAF rule created successfully")
			return nil
		},
	}

	wafUpdateCmd := &cobra.Command{
		Use:   "update <zone_uuid> <rule_uuid>",
		Short: "Update a WAF rule",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			zoneUUID := args[0]
			ruleUUID := args[1]

			body := map[string]interface{}{}
			if cmd.Flags().Changed("name") {
				name, _ := cmd.Flags().GetString("name")
				body["name"] = name
			}
			if cmd.Flags().Changed("priority") {
				priority, _ := cmd.Flags().GetInt("priority")
				body["priority"] = priority
			}
			if cmd.Flags().Changed("action") {
				actionStr, _ := cmd.Flags().GetString("action")
				var actionConfig map[string]interface{}
				if err := json.Unmarshal([]byte(actionStr), &actionConfig); err != nil {
					return fmt.Errorf("invalid JSON for --action: %w", err)
				}
				body["action_config"] = actionConfig
			}
			if cmd.Flags().Changed("match") {
				matchStr, _ := cmd.Flags().GetString("match")
				var matchConditions map[string]interface{}
				if err := json.Unmarshal([]byte(matchStr), &matchConditions); err != nil {
					return fmt.Errorf("invalid JSON for --match: %w", err)
				}
				body["match_conditions"] = matchConditions
			}

			if len(body) == 0 {
				return fmt.Errorf("at least one flag must be specified")
			}

			s := output.NewSpinner("Updating WAF rule...")
			s.Start()
			resp, err := client.Patch(fmt.Sprintf("/cdn/zones/%s/waf-rules/%s", zoneUUID, ruleUUID), body)
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			output.PrintSuccess("WAF rule updated successfully")
			return nil
		},
	}

	wafDeleteCmd := &cobra.Command{
		Use:   "delete <zone_uuid> <rule_uuid>",
		Short: "Delete a WAF rule",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			zoneUUID := args[0]
			ruleUUID := args[1]

			if !cmdutil.CheckForce(cmd, fmt.Sprintf("Are you sure you want to delete WAF rule %s?", ruleUUID)) {
				output.PrintWarning("Aborted")
				return nil
			}

			s := output.NewSpinner("Deleting WAF rule...")
			s.Start()
			resp, err := client.Delete(fmt.Sprintf("/cdn/zones/%s/waf-rules/%s", zoneUUID, ruleUUID))
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			output.PrintSuccess("WAF rule deleted successfully")
			return nil
		},
	}

	// WAF create flags
	wafCreateCmd.Flags().StringP("name", "n", "", "Name of the WAF rule")
	wafCreateCmd.Flags().StringP("type", "t", "", "WAF rule type")
	wafCreateCmd.Flags().IntP("priority", "p", 100, "Rule priority")
	wafCreateCmd.Flags().StringP("action", "a", "", "Action configuration (JSON string)")
	wafCreateCmd.Flags().StringP("match", "m", "", "Match conditions (JSON string)")
	wafCreateCmd.Flags().Bool("disabled", false, "Create rule in disabled state")
	wafCreateCmd.MarkFlagRequired("name")
	wafCreateCmd.MarkFlagRequired("type")
	wafCreateCmd.MarkFlagRequired("action")

	// WAF update flags
	wafUpdateCmd.Flags().StringP("name", "n", "", "New name for the WAF rule")
	wafUpdateCmd.Flags().IntP("priority", "p", 0, "New priority")
	wafUpdateCmd.Flags().StringP("action", "a", "", "New action configuration (JSON string)")
	wafUpdateCmd.Flags().StringP("match", "m", "", "New match conditions (JSON string)")

	// WAF delete flags
	wafDeleteCmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")

	wafCmd.AddCommand(wafListCmd, wafShowCmd, wafCreateCmd, wafUpdateCmd, wafDeleteCmd)
	parent.AddCommand(wafCmd)
}
