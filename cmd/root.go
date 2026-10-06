package cmd

import (
	"context"
	"fmt"

	"github.com/CubePathInc/cubecli/cmd/alert"
	authcmd "github.com/CubePathInc/cubecli/cmd/auth"
	"github.com/CubePathInc/cubecli/cmd/availabilitygroup"
	"github.com/CubePathInc/cubecli/cmd/baremetal"
	"github.com/CubePathInc/cubecli/cmd/cdn"
	configcmd "github.com/CubePathInc/cubecli/cmd/config"
	"github.com/CubePathInc/cubecli/cmd/ddosattack"
	"github.com/CubePathInc/cubecli/cmd/ddosmitigation"
	"github.com/CubePathInc/cubecli/cmd/dns"
	"github.com/CubePathInc/cubecli/cmd/firewall"
	"github.com/CubePathInc/cubecli/cmd/floatingip"
	"github.com/CubePathInc/cubecli/cmd/kubernetes"
	"github.com/CubePathInc/cubecli/cmd/lb"
	"github.com/CubePathInc/cubecli/cmd/location"
	"github.com/CubePathInc/cubecli/cmd/manageddatabase"
	mcpcmd "github.com/CubePathInc/cubecli/cmd/mcp"
	"github.com/CubePathInc/cubecli/cmd/natgateway"
	"github.com/CubePathInc/cubecli/cmd/network"
	"github.com/CubePathInc/cubecli/cmd/objectstorage"
	orgcmd "github.com/CubePathInc/cubecli/cmd/org"
	"github.com/CubePathInc/cubecli/cmd/profile"
	"github.com/CubePathInc/cubecli/cmd/project"
	skillscmd "github.com/CubePathInc/cubecli/cmd/skills"
	"github.com/CubePathInc/cubecli/cmd/sshkey"
	"github.com/CubePathInc/cubecli/cmd/transcoder"
	"github.com/CubePathInc/cubecli/cmd/vps"
	"github.com/CubePathInc/cubecli/internal/api"
	"github.com/CubePathInc/cubecli/internal/cmdutil"
	internalConfig "github.com/CubePathInc/cubecli/internal/config"
	"github.com/CubePathInc/cubecli/internal/oauth"
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
		case "config", "profile", "auth", "login", "logout", "skills", "mcp", "docs", "version", "update", "completion", "help", "cubecli":
			return nil
		}

		// A command that can work offline with one of its flags set (s3 presign --endpoint)
		if flag := cmd.Annotations[objectstorage.SkipAuthUnlessFlagAnnotation]; flag != "" && cmd.Flags().Changed(flag) {
			return nil
		}

		cfg := internalConfig.LoadOrEmpty()
		explicit, _ := cmd.Flags().GetString("profile")
		p, name, err := cfg.ActiveProfile(explicit)
		if err != nil {
			return err
		}

		var client *api.Client
		if p.OAuth != nil && !oauth.ResourceIsAPI(p.OAuth.Resource, internalConfig.APIURL(p)) {
			return fmt.Errorf("profile %q uses a browser login that the CubePath API does not accept yet; log it in with an API token: cubecli login %s --token", name, name)
		}
		if p.OAuth != nil {
			client = api.NewClientWithAuth(internalConfig.APIURL(p), oauth.NewTokenSource(name, p.OAuth))
		} else {
			client = api.NewClient(internalConfig.APIURL(p), p.APIToken)
		}
		ctx := cmd.Context()
		ctx = context.WithValue(ctx, cmdutil.ClientKey, client)
		ctx = context.WithValue(ctx, cmdutil.ActiveProfileKey, name)
		cmd.SetContext(ctx)
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
	rootCmd.PersistentFlags().String("profile", "", "Profile to use (overrides CUBE_PROFILE and current profile)")

	rootCmd.AddCommand(
		authcmd.NewLoginCmd(),
		authcmd.NewLogoutCmd(),
		authcmd.NewCmd(),
		configcmd.NewCmd(),
		profile.NewCmd(),
		orgcmd.NewCmd(),
		sshkey.NewCmd(),
		project.NewCmd(),
		network.NewCmd(),
		natgateway.NewCmd(),
		location.NewCmd(),
		vps.NewCmd(),
		baremetal.NewCmd(),
		availabilitygroup.NewCmd(),
		floatingip.NewCmd(),
		ddosattack.NewCmd(),
		ddosmitigation.NewCmd(),
		firewall.NewCmd(),
		dns.NewCmd(),
		lb.NewCmd(),
		cdn.NewCmd(),
		kubernetes.NewCmd(),
		objectstorage.NewCmd(),
		manageddatabase.NewCmd(),
		transcoder.NewCmd(),
		alert.NewCmd(),
		skillscmd.NewCmd(),
		mcpcmd.NewCmd(),
	)
}
