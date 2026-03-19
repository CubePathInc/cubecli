package cmd

import (
	"context"

	"github.com/CubePathInc/cubecli/cmd/baremetal"
	"github.com/CubePathInc/cubecli/cmd/cdn"
	configcmd "github.com/CubePathInc/cubecli/cmd/config"
	"github.com/CubePathInc/cubecli/cmd/ddosattack"
	"github.com/CubePathInc/cubecli/cmd/dns"
	"github.com/CubePathInc/cubecli/cmd/floatingip"
	"github.com/CubePathInc/cubecli/cmd/lb"
	"github.com/CubePathInc/cubecli/cmd/location"
	"github.com/CubePathInc/cubecli/cmd/network"
	"github.com/CubePathInc/cubecli/cmd/project"
	"github.com/CubePathInc/cubecli/cmd/sshkey"
	"github.com/CubePathInc/cubecli/cmd/vps"
	"github.com/CubePathInc/cubecli/internal/api"
	"github.com/CubePathInc/cubecli/internal/cmdutil"
	internalConfig "github.com/CubePathInc/cubecli/internal/config"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "cubecli",
	Short: "CubePath Cloud CLI",
	Long:  "CubeCLI - The official command-line interface for CubePath Cloud",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Walk up to find the root subcommand (direct child of root)
		root := cmd
		for root.Parent() != nil && root.Parent().Parent() != nil {
			root = root.Parent()
		}
		rootName := root.Name()

		// Skip auth for commands that don't need it
		switch rootName {
		case "config", "version", "update", "completion", "help", "cubecli":
			return nil
		}

		cfg, err := internalConfig.Load()
		if err != nil {
			return err
		}

		client := api.NewClient(internalConfig.APIURL(), cfg.APIToken)
		cmd.SetContext(context.WithValue(cmd.Context(), cmdutil.ClientKey, client))
		return nil
	},
	SilenceUsage:  true,
	SilenceErrors: true,
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().Bool("json", false, "Output in JSON format")
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "Enable verbose output")

	rootCmd.AddCommand(
		configcmd.NewCmd(),
		sshkey.NewCmd(),
		project.NewCmd(),
		network.NewCmd(),
		location.NewCmd(),
		vps.NewCmd(),
		baremetal.NewCmd(),
		floatingip.NewCmd(),
		ddosattack.NewCmd(),
		dns.NewCmd(),
		lb.NewCmd(),
		cdn.NewCmd(),
	)
}
