package cmd

import (
	"fmt"
	"runtime"

	"github.com/CubePathInc/cubecli/internal/output"
	"github.com/CubePathInc/cubecli/internal/version"
	selfupdate "github.com/creativeprojects/go-selfupdate"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(&cobra.Command{
		Use:   "update",
		Short: "Update CubeCLI to the latest version",
		RunE: func(cmd *cobra.Command, args []string) error {
			s := output.NewSpinner("Checking for updates...")
			s.Start()

			source, err := selfupdate.NewGitHubSource(selfupdate.GitHubConfig{})
			if err != nil {
				s.Stop()
				return fmt.Errorf("failed to create update source: %w", err)
			}

			updater, err := selfupdate.NewUpdater(selfupdate.Config{
				Source: source,
			})
			if err != nil {
				s.Stop()
				return fmt.Errorf("failed to create updater: %w", err)
			}

			latest, found, err := updater.DetectLatest(cmd.Context(), selfupdate.ParseSlug("CubePathInc/cubecli"))
			if err != nil {
				s.Stop()
				return fmt.Errorf("failed to check for updates: %w", err)
			}

			if !found {
				s.Stop()
				output.PrintInfo("No releases found")
				return nil
			}

			currentVersion := version.Version
			if currentVersion == "dev" {
				currentVersion = "0.0.0"
			}

			if latest.LessOrEqual(currentVersion) {
				s.Stop()
				output.PrintSuccess(fmt.Sprintf("Already up to date (v%s)", version.Version))
				return nil
			}

			s.Stop()
			output.PrintInfo(fmt.Sprintf("Updating from v%s to v%s...", version.Version, latest.Version()))

			s = output.NewSpinner("Downloading update...")
			s.Start()

			_, err = updater.UpdateSelf(cmd.Context(), currentVersion, selfupdate.ParseSlug("CubePathInc/cubecli"))
			s.Stop()

			if err != nil {
				return fmt.Errorf("failed to update: %w", err)
			}

			output.PrintSuccess(fmt.Sprintf("Updated to v%s (%s/%s)", latest.Version(), runtime.GOOS, runtime.GOARCH))
			return nil
		},
	})
}
