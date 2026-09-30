package manageddatabase

import (
	"encoding/json"
	"fmt"

	"github.com/CubePathInc/cubecli/internal/cmdutil"
	"github.com/CubePathInc/cubecli/internal/output"
	"github.com/spf13/cobra"
)

// object is a logical database or a user of an instance.
type object struct {
	UUID      string `json:"uuid"`
	Name      string `json:"name"`
	Username  string `json:"username"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
}

// --- databases ---

func databaseCmd() *cobra.Command {
	dbCmd := &cobra.Command{
		Use:     "database",
		Aliases: []string{"db"},
		Short:   "Manage the logical databases of an instance (MySQL, PostgreSQL)",
	}

	listCmd := &cobra.Command{
		Use:   "list <md_uuid>",
		Short: "List logical databases",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			s := output.NewSpinner("Fetching databases...")
			s.Start()
			resp, err := client.Get(fmt.Sprintf("/managed-databases/%s/databases", args[0]))
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			var items []object
			if err := json.Unmarshal(resp, &items); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}
			t := output.NewTable("Databases", []string{"UUID", "Name", "Status", "Created"})
			for _, o := range items {
				t.AddRow(o.UUID, o.Name, output.FormatStatus(o.Status), o.CreatedAt)
			}
			t.Render()
			return nil
		},
	}

	createCmd := &cobra.Command{
		Use:   "create <md_uuid> <name>",
		Short: "Create a logical database",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			s := output.NewSpinner("Creating database...")
			s.Start()
			resp, err := client.Post(fmt.Sprintf("/managed-databases/%s/databases", args[0]), map[string]string{"name": args[1]})
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			var o object
			if json.Unmarshal(resp, &o) == nil && o.UUID != "" {
				output.PrintSuccess(fmt.Sprintf("Database '%s' creation started (%s)", o.Name, o.UUID))
			} else {
				output.PrintSuccess("Database creation started")
			}
			return nil
		},
	}

	deleteCmd := &cobra.Command{
		Use:   "delete <md_uuid> <db_uuid>",
		Short: "Delete a logical database and its data",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmdutil.CheckForce(cmd, "Delete this database? Its data will be destroyed.") {
				output.PrintWarning("Aborted")
				return nil
			}

			client := cmdutil.GetClient(cmd)
			s := output.NewSpinner("Deleting database...")
			s.Start()
			resp, err := client.Delete(fmt.Sprintf("/managed-databases/%s/databases/%s", args[0], args[1]))
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}
			output.PrintSuccess("Database deletion started")
			return nil
		},
	}
	deleteCmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")

	dbCmd.AddCommand(listCmd, createCmd, deleteCmd)
	return dbCmd
}

// --- users ---

func userCmd() *cobra.Command {
	userCmd := &cobra.Command{
		Use:   "user",
		Short: "Manage the database users of an instance",
	}

	listCmd := &cobra.Command{
		Use:   "list <md_uuid>",
		Short: "List database users",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			s := output.NewSpinner("Fetching users...")
			s.Start()
			resp, err := client.Get(fmt.Sprintf("/managed-databases/%s/users", args[0]))
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			var items []object
			if err := json.Unmarshal(resp, &items); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}
			t := output.NewTable("Database Users", []string{"UUID", "Username", "Status", "Created"})
			for _, o := range items {
				t.AddRow(o.UUID, o.Username, output.FormatStatus(o.Status), o.CreatedAt)
			}
			t.Render()
			return nil
		},
	}

	createCmd := &cobra.Command{
		Use:   "create <md_uuid> <username>",
		Short: "Create a database user",
		Long: `Create a database user. Without --password a random one is generated.
The password is shown only once: store it now.`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			body := map[string]interface{}{"username": args[1]}
			if pw, _ := cmd.Flags().GetString("password"); pw != "" {
				body["password"] = pw
			}

			s := output.NewSpinner("Creating user...")
			s.Start()
			resp, err := client.Post(fmt.Sprintf("/managed-databases/%s/users", args[0]), body)
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			var u struct {
				object
				Password string `json:"password"`
			}
			if err := json.Unmarshal(resp, &u); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}
			t := output.NewTable("Database User Created", []string{"Field", "Value"})
			t.AddRow("UUID", u.UUID)
			t.AddRow("Username", u.Username)
			t.AddRow("Password", u.Password)
			t.AddRow("Status", output.FormatStatus(u.Status))
			t.Render()
			output.PrintWarning("The password is shown only once. Store it now.")
			return nil
		},
	}
	createCmd.Flags().String("password", "", "Password (12-64 characters); generated when omitted")

	deleteCmd := &cobra.Command{
		Use:   "delete <md_uuid> <user_uuid>",
		Short: "Delete a database user",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmdutil.CheckForce(cmd, "Delete this database user?") {
				output.PrintWarning("Aborted")
				return nil
			}

			client := cmdutil.GetClient(cmd)
			s := output.NewSpinner("Deleting user...")
			s.Start()
			resp, err := client.Delete(fmt.Sprintf("/managed-databases/%s/users/%s", args[0], args[1]))
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}
			output.PrintSuccess("User deletion started")
			return nil
		},
	}
	deleteCmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")

	userCmd.AddCommand(listCmd, createCmd, deleteCmd)
	return userCmd
}
