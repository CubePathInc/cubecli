package kubernetes

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/CubePathInc/cubecli/internal/cmdutil"
	"github.com/CubePathInc/cubecli/internal/output"
	"github.com/spf13/cobra"
)

func newAddonCmd() *cobra.Command {
	addonCmd := &cobra.Command{
		Use:   "addon",
		Short: "Manage Kubernetes addons",
	}

	addonCmd.AddCommand(
		addonListCmd(),
		addonShowCmd(),
		addonInstalledCmd(),
		addonInstallCmd(),
		addonUninstallCmd(),
	)

	return addonCmd
}

// --- list ---

func addonListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "Browse available addons",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			s := output.NewSpinner("Fetching addons...")
			s.Start()
			resp, err := client.Get("/kubernetes/addons")
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			var addons []struct {
				Name           string `json:"name"`
				Slug           string `json:"slug"`
				Category       string `json:"category"`
				DefaultVersion string `json:"default_version"`
				MinK8sVersion  string `json:"min_k8s_version"`
			}
			if err := json.Unmarshal(resp, &addons); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}

			t := output.NewTable("Available Addons", []string{"Name", "Slug", "Category", "Version", "Min K8s"})
			for _, a := range addons {
				t.AddRow(a.Name, a.Slug, a.Category, a.DefaultVersion, a.MinK8sVersion)
			}
			t.Render()
			return nil
		},
	}
}

// --- show ---

func addonShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <slug>",
		Short: "Show addon details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			s := output.NewSpinner("Fetching addon details...")
			s.Start()
			resp, err := client.Get("/kubernetes/addons/" + args[0])
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			var addon struct {
				Name             string `json:"name"`
				Slug             string `json:"slug"`
				Description      string `json:"description"`
				Category         string `json:"category"`
				HelmRepoName    string `json:"helm_repo_name"`
				HelmRepoURL     string `json:"helm_repo_url"`
				HelmChart        string `json:"helm_chart"`
				DefaultVersion   string `json:"default_version"`
				Namespace        string `json:"namespace"`
				IconURL          string `json:"icon_url"`
				DocumentationURL string `json:"documentation_url"`
				Keywords         string `json:"keywords"`
				MinK8sVersion    string `json:"min_k8s_version"`
			}
			if err := json.Unmarshal(resp, &addon); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}

			fmt.Println()
			fmt.Printf("  Name:          %s\n", addon.Name)
			fmt.Printf("  Slug:          %s\n", addon.Slug)
			fmt.Printf("  Category:      %s\n", addon.Category)
			if addon.Description != "" {
				fmt.Printf("  Description:   %s\n", addon.Description)
			}
			fmt.Printf("  Helm Chart:    %s/%s\n", addon.HelmRepoName, addon.HelmChart)
			fmt.Printf("  Helm Repo:     %s\n", addon.HelmRepoURL)
			fmt.Printf("  Version:       %s\n", addon.DefaultVersion)
			fmt.Printf("  Namespace:     %s\n", addon.Namespace)
			if addon.MinK8sVersion != "" {
				fmt.Printf("  Min K8s:       %s\n", addon.MinK8sVersion)
			}
			if addon.DocumentationURL != "" {
				fmt.Printf("  Docs:          %s\n", addon.DocumentationURL)
			}
			if addon.Keywords != "" {
				fmt.Printf("  Keywords:      %s\n", addon.Keywords)
			}
			fmt.Println()

			return nil
		},
	}
}

// --- installed ---

func addonInstalledCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "installed <cluster_uuid>",
		Short: "List addons installed on a cluster",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			s := output.NewSpinner("Fetching installed addons...")
			s.Start()
			resp, err := client.Get("/kubernetes/" + args[0] + "/addons")
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			var installed []struct {
				UUID    string `json:"uuid"`
				Status  string `json:"status"`
				Version string `json:"installed_version"`
				Addon   struct {
					Name string `json:"name"`
					Slug string `json:"slug"`
				} `json:"addon"`
				InstalledAt string `json:"installed_at"`
			}
			if err := json.Unmarshal(resp, &installed); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}

			if len(installed) == 0 {
				fmt.Println("No addons installed on this cluster.")
				return nil
			}

			t := output.NewTable("Installed Addons", []string{"UUID", "Name", "Slug", "Status", "Version", "Installed At"})
			for _, a := range installed {
				t.AddRow(
					a.UUID,
					a.Addon.Name,
					a.Addon.Slug,
					output.FormatStatus(a.Status),
					a.Version,
					a.InstalledAt,
				)
			}
			t.Render()
			return nil
		},
	}
}

// --- install ---

func addonInstallCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install <cluster_uuid> <addon_slug>",
		Short: "Install an addon on a cluster",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmdutil.GetClient(cmd)

			body := map[string]interface{}{}
			valuesFlag, _ := cmd.Flags().GetString("values")
			if valuesFlag != "" {
				var customValues map[string]interface{}
				// Try reading as file first
				if !strings.HasPrefix(valuesFlag, "{") {
					data, err := os.ReadFile(valuesFlag)
					if err == nil {
						valuesFlag = string(data)
					}
				}
				if err := json.Unmarshal([]byte(valuesFlag), &customValues); err != nil {
					return fmt.Errorf("invalid JSON values: %w", err)
				}
				body["custom_values"] = customValues
			}

			s := output.NewSpinner("Installing addon...")
			s.Start()
			resp, err := client.Post("/kubernetes/"+args[0]+"/addons/"+args[1]+"/install", body)
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			var result struct {
				Detail string `json:"detail"`
			}
			if err := json.Unmarshal(resp, &result); err != nil {
				return err
			}
			if result.Detail != "" {
				output.PrintSuccess(result.Detail)
			} else {
				output.PrintSuccess("Addon installation initiated")
			}
			return nil
		},
	}
	cmd.Flags().String("values", "", "Custom Helm values (JSON string or file path)")
	return cmd
}

// --- uninstall ---

func addonUninstallCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "uninstall <cluster_uuid> <addon_uuid>",
		Short: "Uninstall an addon from a cluster",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmdutil.CheckForce(cmd, fmt.Sprintf("Are you sure you want to uninstall addon %s?", args[1])) {
				return nil
			}

			client := cmdutil.GetClient(cmd)

			s := output.NewSpinner("Uninstalling addon...")
			s.Start()
			resp, err := client.Delete("/kubernetes/" + args[0] + "/addons/" + args[1])
			s.Stop()
			if err != nil {
				return err
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(json.RawMessage(resp))
			}

			output.PrintSuccess("Addon uninstall initiated")
			return nil
		},
	}
	cmd.Flags().BoolP("force", "f", false, "Skip confirmation")
	return cmd
}
