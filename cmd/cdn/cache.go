package cdn

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/CubePathInc/cubecli/internal/cmdutil"
	"github.com/CubePathInc/cubecli/internal/output"
	"github.com/spf13/cobra"
)

// addCacheCmd wires `cdn cache purge|purges`.
func addCacheCmd(parent *cobra.Command) {
	cacheCmd := &cobra.Command{
		Use:   "cache",
		Short: "Purge the edge cache of a CDN zone",
	}

	purgeCmd := &cobra.Command{
		Use:   "purge <zone_uuid> [path...]",
		Short: "Purge paths, path prefixes or the whole cache of a zone",
		Long: `Purge cached content on every edge location. Give up to 100 paths
starting with "/"; a trailing "*" purges every URL under a prefix. --everything
purges the whole zone. Both the system and the custom domain are purged.`,
		Example: `  cubecli cdn cache purge <zone_uuid> /assets/app.css "/images/*"
  cubecli cdn cache purge <zone_uuid> --everything`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			everything, _ := cmd.Flags().GetBool("everything")
			paths := args[1:]
			if everything == (len(paths) > 0) {
				return fmt.Errorf("give either paths or --everything")
			}
			body := map[string]interface{}{}
			if everything {
				body["everything"] = true
			} else {
				body["paths"] = paths
			}

			s := output.NewSpinner("Queuing cache purge...")
			s.Start()
			resp, err := client.Post(fmt.Sprintf("/cdn/zones/%s/purge-cache", args[0]), body)
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}
			var r struct {
				Detail    string `json:"detail"`
				PurgeUUID string `json:"purge_uuid"`
			}
			if err := json.Unmarshal(resp, &r); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}
			output.PrintSuccess(r.Detail)
			output.PrintInfo(fmt.Sprintf("Purge %s; follow it with: cubecli cdn cache purges %s", r.PurgeUUID, args[0]))
			return nil
		},
	}
	purgeCmd.Flags().Bool("everything", false, "Purge the whole cache of the zone")

	listCmd := &cobra.Command{
		Use:   "purges <zone_uuid>",
		Short: "Show the latest 20 purges and their progress per edge location",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			s := output.NewSpinner("Fetching purges...")
			s.Start()
			resp, err := client.Get(fmt.Sprintf("/cdn/zones/%s/purge-cache", args[0]))
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			type progress struct {
				Pop       string `json:"pop"`
				Expected  int    `json:"expected"`
				Completed int    `json:"completed"`
				Failed    int    `json:"failed"`
			}
			var purges []struct {
				PurgeUUID   string     `json:"purge_uuid"`
				Scope       string     `json:"scope"`
				Paths       []string   `json:"paths"`
				Status      string     `json:"status"`
				RequestedAt string     `json:"requested_at"`
				Nodes       progress   `json:"nodes"`
				Pops        []progress `json:"pops"`
			}
			if err := json.Unmarshal(resp, &purges); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}

			t := output.NewTable("Cache Purges", []string{"UUID", "Requested", "Scope", "Status", "Nodes", "PoPs"})
			for _, p := range purges {
				scope := "everything"
				if p.Scope != "everything" {
					scope = strings.Join(p.Paths, " ")
				}
				pops := make([]string, 0, len(p.Pops))
				for _, pop := range p.Pops {
					pops = append(pops, fmt.Sprintf("%s %d/%d", pop.Pop, pop.Completed, pop.Expected))
				}
				t.AddRow(p.PurgeUUID, p.RequestedAt, scope, output.FormatStatus(p.Status),
					fmt.Sprintf("%d/%d done, %d failed", p.Nodes.Completed, p.Nodes.Expected, p.Nodes.Failed),
					strings.Join(pops, ", "))
			}
			t.Render()
			return nil
		},
	}

	cacheCmd.AddCommand(purgeCmd, listCmd)
	parent.AddCommand(cacheCmd)
}

// addTokenAuthCmd wires `cdn token-auth`: signed URLs checked at the edge.
func addTokenAuthCmd(parent *cobra.Command) {
	taCmd := &cobra.Command{
		Use:   "token-auth",
		Short: "Protect a CDN zone with signed URLs",
		Long: `With Token Auth on, the edge only serves URLs signed with the zone's secret
(HMAC-SHA256 of the path and expiry, optionally bound to the client IP).`,
	}

	secretResult := func(cmd *cobra.Command, resp json.RawMessage, fallback string) error {
		if cmdutil.IsJSON(cmd) {
			return output.PrintJSON(json.RawMessage(resp))
		}
		var r struct {
			Detail string `json:"detail"`
			Secret string `json:"token_auth_secret"`
		}
		_ = json.Unmarshal(resp, &r)
		if r.Secret == "" {
			output.PrintSuccess(fallback)
			return nil
		}
		t := output.NewTable("Token Auth Secret", []string{"Field", "Value"})
		t.AddRow("Secret", r.Secret)
		t.Render()
		output.PrintWarning("The secret is shown only once. Store it now.")
		return nil
	}

	enableCmd := &cobra.Command{
		Use:   "enable <zone_uuid>",
		Short: "Turn Token Auth on (the first time prints the secret)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			body := map[string]interface{}{"token_auth_enabled": true}
			if cmd.Flags().Changed("ip-binding") {
				v, _ := cmd.Flags().GetBool("ip-binding")
				body["token_auth_ip_binding"] = v
			}
			s := output.NewSpinner("Enabling Token Auth...")
			s.Start()
			resp, err := client.Patch(fmt.Sprintf("/cdn/zones/%s", args[0]), body)
			s.Stop()
			if err != nil {
				return err
			}
			return secretResult(cmd, resp, "Token Auth enabled")
		},
	}
	enableCmd.Flags().Bool("ip-binding", false, "Bind signed URLs to the client IP (--ip-binding=false to unbind)")

	disableCmd := &cobra.Command{
		Use:   "disable <zone_uuid>",
		Short: "Turn Token Auth off (the secret is kept)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			return cmdutil.RunDetail(cmd, "Disabling Token Auth...", "Token Auth disabled", func() (json.RawMessage, error) {
				return client.Patch(fmt.Sprintf("/cdn/zones/%s", args[0]), map[string]interface{}{"token_auth_enabled": false})
			})
		},
	}

	rotateCmd := &cobra.Command{
		Use:   "rotate-secret <zone_uuid>",
		Short: "Replace the secret; every URL signed before stops working",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmdutil.CheckForce(cmd, "Rotate the secret? Every URL signed with the current one stops working.") {
				output.PrintWarning("Aborted")
				return nil
			}
			client := cmdutil.GetClient(cmd)
			s := output.NewSpinner("Rotating secret...")
			s.Start()
			resp, err := client.Post(fmt.Sprintf("/cdn/zones/%s/token-auth/rotate-secret", args[0]), nil)
			s.Stop()
			if err != nil {
				return err
			}
			return secretResult(cmd, resp, "Secret rotated")
		},
	}
	rotateCmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")

	signCmd := &cobra.Command{
		Use:     "sign-url <zone_uuid> <path>",
		Short:   "Sign a URL of the zone",
		Example: `  cubecli cdn token-auth sign-url <zone_uuid> /videos/clip.mp4 --expires-in 3600`,
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)
			body := map[string]interface{}{"path": args[1]}
			if cmd.Flags().Changed("expires-in") {
				v, _ := cmd.Flags().GetInt("expires-in")
				body["expires_in"] = v
			}
			if ip, _ := cmd.Flags().GetString("client-ip"); ip != "" {
				body["client_ip"] = ip
			}

			s := output.NewSpinner("Signing URL...")
			s.Start()
			resp, err := client.Post(fmt.Sprintf("/cdn/zones/%s/token-auth/sign-url", args[0]), body)
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}
			var r struct {
				SignedURL string `json:"signed_url"`
			}
			if err := json.Unmarshal(resp, &r); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}
			fmt.Println(r.SignedURL)
			return nil
		},
	}
	signCmd.Flags().Int("expires-in", 3600, "Seconds the URL stays valid (60-604800)")
	signCmd.Flags().String("client-ip", "", "Client IP (required when the zone binds tokens to IPs)")

	taCmd.AddCommand(enableCmd, disableCmd, rotateCmd, signCmd)
	parent.AddCommand(taCmd)
}
