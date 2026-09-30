package alert

import (
	"encoding/json"
	"fmt"

	"github.com/CubePathInc/cubecli/internal/cmdutil"
	"github.com/CubePathInc/cubecli/internal/output"
	"github.com/spf13/cobra"
)

type notificatorOut struct {
	ID      string            `json:"id"`
	Name    string            `json:"name"`
	Type    string            `json:"type"`
	Config  map[string]string `json:"config"`
	Enabled bool              `json:"enabled"`
}

func (n notificatorOut) destination() string {
	if v := n.Config["webhook_url"]; v != "" {
		return v
	}
	return n.Config["email"]
}

func notificatorCmd() *cobra.Command {
	nCmd := &cobra.Command{
		Use:     "notificator",
		Aliases: []string{"channel"},
		Short:   "Manage alert notification channels (Slack, Discord, email; max 5)",
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List notification channels",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			s := output.NewSpinner("Fetching notificators...")
			s.Start()
			resp, err := client.Get("/triggers/notificators/")
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			var items []notificatorOut
			if err := json.Unmarshal(resp, &items); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}
			t := output.NewTable("Notificators", []string{"ID", "Name", "Type", "Destination", "Enabled"})
			for _, n := range items {
				t.AddRow(n.ID, n.Name, n.Type, n.destination(), cmdutil.YesNo(n.Enabled))
			}
			t.Render()
			return nil
		},
	}

	showCmd := &cobra.Command{
		Use:   "show <notificator_id>",
		Short: "Show a notification channel",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			s := output.NewSpinner("Fetching notificator...")
			s.Start()
			resp, err := client.Get("/triggers/notificators/" + args[0])
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}
			return renderNotificator("Notificator", resp)
		},
	}

	createCmd := &cobra.Command{
		Use:   "create",
		Short: "Create a notification channel",
		Long: `Create a notification channel. Slack and Discord need --webhook-url; email
channels always notify the address of the account that creates them.`,
		Example: `  cubecli alert notificator create --name ops --type slack --webhook-url https://hooks.slack.com/services/...
  cubecli alert notificator create --name me --type email`,
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			name, _ := cmd.Flags().GetString("name")
			typ, _ := cmd.Flags().GetString("type")
			webhook, _ := cmd.Flags().GetString("webhook-url")
			disabled, _ := cmd.Flags().GetBool("disabled")

			body := map[string]interface{}{
				"name":    name,
				"type":    typ,
				"enabled": !disabled,
			}
			if webhook != "" {
				body["config"] = map[string]string{"webhook_url": webhook}
			}

			s := output.NewSpinner("Creating notificator...")
			s.Start()
			resp, err := client.Post("/triggers/notificators/", body)
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}
			if err := renderNotificator("Notificator Created", resp); err != nil {
				return err
			}
			output.PrintSuccess("Notificator created successfully")
			return nil
		},
	}
	createCmd.Flags().StringP("name", "n", "", "Name")
	createCmd.Flags().String("type", "", "Type: slack, discord or email")
	createCmd.Flags().String("webhook-url", "", "Webhook URL (slack and discord)")
	createCmd.Flags().Bool("disabled", false, "Create it disabled")
	_ = createCmd.MarkFlagRequired("name")
	_ = createCmd.MarkFlagRequired("type")

	updateCmd := &cobra.Command{
		Use:   "update <notificator_id>",
		Short: "Update a notification channel",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			body := map[string]interface{}{}
			if cmd.Flags().Changed("name") {
				v, _ := cmd.Flags().GetString("name")
				body["name"] = v
			}
			if cmd.Flags().Changed("webhook-url") {
				v, _ := cmd.Flags().GetString("webhook-url")
				body["config"] = map[string]string{"webhook_url": v}
			}
			if cmd.Flags().Changed("enabled") {
				v, _ := cmd.Flags().GetBool("enabled")
				body["enabled"] = v
			}
			if len(body) == 0 {
				return fmt.Errorf("at least one of --name, --webhook-url or --enabled must be specified")
			}

			s := output.NewSpinner("Updating notificator...")
			s.Start()
			resp, err := client.Put("/triggers/notificators/"+args[0], body)
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}
			output.PrintSuccess("Notificator updated successfully")
			return nil
		},
	}
	updateCmd.Flags().StringP("name", "n", "", "New name")
	updateCmd.Flags().String("webhook-url", "", "New webhook URL (slack and discord)")
	updateCmd.Flags().Bool("enabled", true, "Enable or disable (--enabled=false)")

	deleteCmd := &cobra.Command{
		Use:   "delete <notificator_id>",
		Short: "Delete a notification channel (it must not be used by any alert)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmdutil.CheckForce(cmd, "Delete this notificator?") {
				output.PrintWarning("Aborted")
				return nil
			}

			client := cmdutil.GetClient(cmd)
			s := output.NewSpinner("Deleting notificator...")
			s.Start()
			resp, err := client.Delete("/triggers/notificators/" + args[0])
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}
			output.PrintSuccess("Notificator deleted successfully")
			return nil
		},
	}
	deleteCmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")

	nCmd.AddCommand(listCmd, showCmd, createCmd, updateCmd, deleteCmd)
	return nCmd
}

func renderNotificator(title string, resp json.RawMessage) error {
	var n notificatorOut
	if err := json.Unmarshal(resp, &n); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}
	t := output.NewTable(title, []string{"Field", "Value"})
	t.AddRow("ID", n.ID)
	t.AddRow("Name", n.Name)
	t.AddRow("Type", n.Type)
	t.AddRow("Destination", n.destination())
	t.AddRow("Enabled", cmdutil.YesNo(n.Enabled))
	t.Render()
	return nil
}
