package cdn

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/CubePathInc/cubecli/internal/cmdutil"
	"github.com/CubePathInc/cubecli/internal/output"
	"github.com/spf13/cobra"
)

func addRuleCmd(parent *cobra.Command) {
	ruleCmd := &cobra.Command{
		Use:   "rule",
		Short: "Manage CDN rules",
	}

	ruleListCmd := &cobra.Command{
		Use:   "list <zone_uuid>",
		Short: "List CDN rules for a zone",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			zoneUUID := args[0]

			s := output.NewSpinner("Fetching CDN rules...")
			s.Start()
			resp, err := client.Get(fmt.Sprintf("/cdn/zones/%s/rules", zoneUUID))
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

			t := output.NewTable("CDN Rules", []string{"UUID", "Name", "Type", "Priority", "Enabled"})
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

	ruleShowCmd := &cobra.Command{
		Use:   "show <zone_uuid> <rule_uuid>",
		Short: "Show CDN rule details",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			zoneUUID := args[0]
			ruleUUID := args[1]

			s := output.NewSpinner("Fetching CDN rule details...")
			s.Start()
			resp, err := client.Get(fmt.Sprintf("/cdn/zones/%s/rules/%s", zoneUUID, ruleUUID))
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

			t := output.NewTable("CDN Rule Details", []string{"Field", "Value"})
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

	ruleCreateCmd := &cobra.Command{
		Use:   "create <zone_uuid>",
		Short: "Create a new CDN rule",
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

			s := output.NewSpinner("Creating CDN rule...")
			s.Start()
			resp, err := client.Post(fmt.Sprintf("/cdn/zones/%s/rules", zoneUUID), body)
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

			t := output.NewTable("CDN Rule Created", []string{"Field", "Value"})
			t.AddRow("UUID", rule.UUID)
			t.AddRow("Name", rule.Name)
			t.AddRow("Type", rule.Type)
			t.Render()

			output.PrintSuccess("CDN rule created successfully")
			return nil
		},
	}

	ruleUpdateCmd := &cobra.Command{
		Use:   "update <zone_uuid> <rule_uuid>",
		Short: "Update a CDN rule",
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

			s := output.NewSpinner("Updating CDN rule...")
			s.Start()
			resp, err := client.Patch(fmt.Sprintf("/cdn/zones/%s/rules/%s", zoneUUID, ruleUUID), body)
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			output.PrintSuccess("CDN rule updated successfully")
			return nil
		},
	}

	ruleDeleteCmd := &cobra.Command{
		Use:   "delete <zone_uuid> <rule_uuid>",
		Short: "Delete a CDN rule",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			zoneUUID := args[0]
			ruleUUID := args[1]

			if !cmdutil.CheckForce(cmd, fmt.Sprintf("Are you sure you want to delete CDN rule %s?", ruleUUID)) {
				output.PrintWarning("Aborted")
				return nil
			}

			s := output.NewSpinner("Deleting CDN rule...")
			s.Start()
			resp, err := client.Delete(fmt.Sprintf("/cdn/zones/%s/rules/%s", zoneUUID, ruleUUID))
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			output.PrintSuccess("CDN rule deleted successfully")
			return nil
		},
	}

	// Rule create flags
	ruleCreateCmd.Flags().StringP("name", "n", "", "Name of the rule")
	ruleCreateCmd.Flags().StringP("type", "t", "", "Rule type (cache, cache_bypass, redirect, header_request, header_response)")
	ruleCreateCmd.Flags().IntP("priority", "p", 100, "Rule priority")
	ruleCreateCmd.Flags().StringP("action", "a", "", "Action configuration (JSON string)")
	ruleCreateCmd.Flags().StringP("match", "m", "", "Match conditions (JSON string)")
	ruleCreateCmd.Flags().Bool("disabled", false, "Create rule in disabled state")
	ruleCreateCmd.MarkFlagRequired("name")
	ruleCreateCmd.MarkFlagRequired("type")
	ruleCreateCmd.MarkFlagRequired("action")

	// Rule update flags
	ruleUpdateCmd.Flags().StringP("name", "n", "", "New name for the rule")
	ruleUpdateCmd.Flags().IntP("priority", "p", 0, "New priority")
	ruleUpdateCmd.Flags().StringP("action", "a", "", "New action configuration (JSON string)")
	ruleUpdateCmd.Flags().StringP("match", "m", "", "New match conditions (JSON string)")

	// Rule delete flags
	ruleDeleteCmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")

	ruleCmd.AddCommand(ruleListCmd, ruleShowCmd, ruleCreateCmd, ruleUpdateCmd, ruleDeleteCmd)
	parent.AddCommand(ruleCmd)
}
