package cmd

import (
	"fmt"

	"github.com/CubePathInc/cubecli/internal/version"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Show CubeCLI version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("CubeCLI %s\n", version.Version)
			fmt.Printf("Commit: %s\n", version.Commit)
			fmt.Printf("Built:  %s\n", version.Date)
		},
	})
}
