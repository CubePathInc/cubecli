package dns

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/CubePathInc/cubecli/internal/cmdutil"
	"github.com/CubePathInc/cubecli/internal/output"
	"github.com/spf13/cobra"
)

func addSOACmd(parent *cobra.Command) {
	soaCmd := &cobra.Command{
		Use:   "soa",
		Short: "Manage DNS SOA records",
	}

	soaShowCmd := &cobra.Command{
		Use:   "show <zone_uuid>",
		Short: "Show SOA record for a zone",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			s := output.NewSpinner("Fetching SOA record...")
			s.Start()
			resp, err := client.Get(fmt.Sprintf("/dns/zones/%s/soa", args[0]))
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			var soa struct {
				PrimaryNS  string `json:"primary_ns"`
				Hostmaster string `json:"hostmaster"`
				Serial     int64  `json:"serial"`
				Refresh    int    `json:"refresh"`
				Retry      int    `json:"retry"`
				Expire     int    `json:"expire"`
				Minimum    int    `json:"minimum"`
			}
			if err := json.Unmarshal(resp, &soa); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}

			t := output.NewTable("SOA Record", []string{"Field", "Value"})
			t.AddRow("Primary NS", soa.PrimaryNS)
			t.AddRow("Hostmaster", soa.Hostmaster)
			t.AddRow("Serial", strconv.FormatInt(soa.Serial, 10))
			t.AddRow("Refresh", strconv.Itoa(soa.Refresh))
			t.AddRow("Retry", strconv.Itoa(soa.Retry))
			t.AddRow("Expire", strconv.Itoa(soa.Expire))
			t.AddRow("Minimum", strconv.Itoa(soa.Minimum))
			t.Render()
			return nil
		},
	}

	soaUpdateCmd := &cobra.Command{
		Use:   "update <zone_uuid>",
		Short: "Update SOA record for a zone",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			body := map[string]interface{}{}

			if cmd.Flags().Changed("refresh") {
				v, _ := cmd.Flags().GetInt("refresh")
				body["refresh"] = v
			}
			if cmd.Flags().Changed("retry") {
				v, _ := cmd.Flags().GetInt("retry")
				body["retry"] = v
			}
			if cmd.Flags().Changed("expire") {
				v, _ := cmd.Flags().GetInt("expire")
				body["expire"] = v
			}
			if cmd.Flags().Changed("minimum") {
				v, _ := cmd.Flags().GetInt("minimum")
				body["minimum"] = v
			}
			if cmd.Flags().Changed("hostmaster") {
				v, _ := cmd.Flags().GetString("hostmaster")
				body["hostmaster"] = v
			}

			if len(body) == 0 {
				return fmt.Errorf("no fields to update; provide at least one flag")
			}

			s := output.NewSpinner("Updating SOA record...")
			s.Start()
			resp, err := client.Put(fmt.Sprintf("/dns/zones/%s/soa", args[0]), body)
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			output.PrintSuccess("SOA record updated successfully")
			return nil
		},
	}

	soaUpdateCmd.Flags().Int("refresh", 0, "SOA refresh interval in seconds")
	soaUpdateCmd.Flags().Int("retry", 0, "SOA retry interval in seconds")
	soaUpdateCmd.Flags().Int("expire", 0, "SOA expire time in seconds")
	soaUpdateCmd.Flags().Int("minimum", 0, "SOA minimum TTL in seconds")
	soaUpdateCmd.Flags().String("hostmaster", "", "SOA hostmaster email")

	soaCmd.AddCommand(soaShowCmd, soaUpdateCmd)
	parent.AddCommand(soaCmd)
}
