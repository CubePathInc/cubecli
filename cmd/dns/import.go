package dns

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"

	"github.com/CubePathInc/cubecli/internal/cmdutil"
	"github.com/CubePathInc/cubecli/internal/output"
	"github.com/spf13/cobra"
)

// renderImport prints a DNSImportResult (zone file import or public DNS scan).
func renderImport(resp json.RawMessage) error {
	var r struct {
		Imported int      `json:"imported"`
		Skipped  int      `json:"skipped"`
		Errors   []string `json:"errors"`
		Records  []struct {
			Name       string `json:"name"`
			RecordType string `json:"record_type"`
			Content    string `json:"content"`
			TTL        int    `json:"ttl"`
		} `json:"records"`
	}
	if err := json.Unmarshal(resp, &r); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}
	if len(r.Records) > 0 {
		t := output.NewTable("Imported Records", []string{"Name", "Type", "Content", "TTL"})
		for _, rec := range r.Records {
			t.AddRow(rec.Name, rec.RecordType, rec.Content, strconv.Itoa(rec.TTL))
		}
		t.Render()
	}
	for _, e := range r.Errors {
		output.PrintWarning(e)
	}
	output.PrintSuccess(fmt.Sprintf("%d records imported, %d skipped", r.Imported, r.Skipped))
	return nil
}

// addZoneExtraCmds adds `dns zone import|move-project` and `dns regions`.
func addZoneExtraCmds(zoneCmd, dnsCmd *cobra.Command) {
	importCmd := &cobra.Command{
		Use:   "import <zone_uuid> <zone_file>",
		Short: "Import records from a BIND zone file into an existing zone",
		Long: `Import records from a BIND zone file (max 1 MB). NS records are skipped,
duplicates and conflicting CNAMEs are reported and skipped, and the file's SOA
timers are applied to the zone.`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			data, err := os.ReadFile(args[1])
			if err != nil {
				return fmt.Errorf("failed to read zone file: %w", err)
			}

			s := output.NewSpinner("Importing zone file...")
			s.Start()
			resp, err := client.PostFile(fmt.Sprintf("/dns/zones/%s/import", args[0]), "file", filepath.Base(args[1]), data)
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}
			return renderImport(resp)
		},
	}

	moveCmd := cmdutil.MoveProjectCmd("zone_uuid", "DNS zone", cmdutil.StringPath("/dns/zones/%s/move-project"))

	regionsCmd := &cobra.Command{
		Use:   "regions",
		Short: "List the GeoDNS regions records can target",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			s := output.NewSpinner("Fetching regions...")
			s.Start()
			resp, err := client.Get("/dns/regions")
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			var regions []struct {
				Code string `json:"code"`
				Name string `json:"name"`
			}
			if err := json.Unmarshal(resp, &regions); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}
			t := output.NewTable("GeoDNS Regions", []string{"Code", "Name"})
			for _, r := range regions {
				t.AddRow(r.Code, r.Name)
			}
			t.Render()
			return nil
		},
	}

	zoneCmd.AddCommand(importCmd, moveCmd)
	dnsCmd.AddCommand(regionsCmd)
}

// createFromSource creates a zone and fills it from a zone file or from a
// scan of its current public DNS.
func createFromSource(cmd *cobra.Command, domain string, projectID int, zoneFile string, scan bool) error {
	client := cmdutil.GetClient(cmd)
	q := url.Values{}
	q.Set("domain", domain)
	q.Set("project_id", strconv.Itoa(projectID))

	var resp json.RawMessage
	var err error
	s := output.NewSpinner("Creating DNS zone...")
	s.Start()
	if zoneFile != "" {
		var data []byte
		if data, err = os.ReadFile(zoneFile); err != nil {
			s.Stop()
			return fmt.Errorf("failed to read zone file: %w", err)
		}
		resp, err = client.PostFile("/dns/zones/upload?"+q.Encode(), "file", filepath.Base(zoneFile), data)
	} else if scan {
		resp, err = client.Post("/dns/zones/scan?"+q.Encode(), nil)
	}
	s.Stop()
	if err != nil {
		return err
	}

	if cmdutil.IsJSON(cmd) {
		return output.PrintJSON(json.RawMessage(resp))
	}
	output.PrintSuccess("DNS zone created successfully")
	return renderImport(resp)
}
