package auth

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	skillscmd "github.com/CubePathInc/cubecli/cmd/skills"
	"github.com/CubePathInc/cubecli/internal/api"
	"github.com/CubePathInc/cubecli/internal/cmdutil"
	internalConfig "github.com/CubePathInc/cubecli/internal/config"
	"github.com/CubePathInc/cubecli/internal/oauth"
	"github.com/CubePathInc/cubecli/internal/output"
	"github.com/CubePathInc/cubecli/internal/skills"
	"github.com/spf13/cobra"
)

// NewCmd returns the `auth` command group.
func NewCmd() *cobra.Command {
	authCmd := &cobra.Command{
		Use:   "auth",
		Short: "Log in, log out and inspect credentials",
	}
	authCmd.AddCommand(NewLoginCmd(), NewLogoutCmd(), newStatusCmd())
	return authCmd
}

// NewLoginCmd returns `login`, registered both at the root and under `auth`.
func NewLoginCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "login [profile]",
		Short: "Log in to CubePath in the browser and store the session in a profile",
		Long: `Log in to CubePath through your browser and store the session in a profile.

Each profile holds the session of one organization, chosen on the consent
screen. Log in once per organization and switch with 'cubecli profile use'
or '--profile'.

Without a profile name, the active profile is used ('default' if none).

For CI and other non-interactive use, store an API token instead with
'--token', or set CUBE_API_TOKEN.`,
		Example: `  cubecli login
  cubecli login work
  cubecli login staging --api-url https://api.staging.cubepath.com
  cubecli login --no-browser      # over SSH, no local browser
  cubecli login ci --token        # store an API token`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name, err := loginProfileName(cmd, args)
			if err != nil {
				return err
			}
			apiURL, _ := cmd.Flags().GetString("api-url")
			useToken, _ := cmd.Flags().GetBool("token")
			noBrowser, _ := cmd.Flags().GetBool("no-browser")
			makeActive, _ := cmd.Flags().GetBool("use")

			if useToken {
				return TokenLogin(cmd, name, apiURL, makeActive)
			}
			return BrowserLogin(cmd, name, apiURL, noBrowser, makeActive)
		},
	}
	cmd.Flags().String("api-url", "", "API URL for this profile (kept from the existing profile if omitted)")
	cmd.Flags().Bool("token", false, "Store an API token instead of logging in with the browser")
	cmd.Flags().Bool("no-browser", false, "Print the login URL instead of opening a browser")
	cmd.Flags().Bool("use", false, "Make this the active profile")
	cmd.Flags().Bool("skip-skills", false, "Do not offer to install the CubePath skills for AI agents")
	return cmd
}

// loginProfileName resolves the target profile: positional argument, then the
// global --profile flag, then CUBE_PROFILE / the active profile / "default".
func loginProfileName(cmd *cobra.Command, args []string) (string, error) {
	flagName, _ := cmd.Flags().GetString("profile")
	if len(args) == 1 {
		name := strings.TrimSpace(args[0])
		if name == "" {
			return "", fmt.Errorf("profile name cannot be empty")
		}
		if flagName != "" && flagName != name {
			return "", fmt.Errorf("conflicting profile names: %q and --profile %q", name, flagName)
		}
		return name, nil
	}
	return internalConfig.LoadOrEmpty().ActiveProfileName(flagName), nil
}

// BrowserLogin runs the OAuth login and stores the session in profile `name`.
func BrowserLogin(cmd *cobra.Command, name, apiURL string, noBrowser, makeActive bool) error {
	cfg := internalConfig.LoadOrEmpty()
	existing := cfg.Profiles[name]
	if apiURL == "" && existing != nil {
		apiURL = existing.APIURL
	}
	target := &internalConfig.Profile{APIURL: apiURL}
	baseURL := internalConfig.APIURL(target)

	out := cmd.OutOrStdout()
	if cmdutil.IsJSON(cmd) {
		// Keep stdout clean for the JSON result.
		out = cmd.ErrOrStderr()
	}
	fmt.Fprintf(out, "Logging in profile %q (%s)\n", name, baseURL)

	res, err := oauth.Login(cmd.Context(), oauth.LoginOptions{
		APIURL: baseURL,
		LookupClient: func(issuer string) string {
			return cfg.OAuthClients[issuer]
		},
		NoBrowser: noBrowser,
		In:        os.Stdin,
		Out:       out,
		ClientRegistered: func(issuer, clientID string) {
			_ = internalConfig.Update(func(c *internalConfig.Config) error {
				if c.OAuthClients == nil {
					c.OAuthClients = map[string]string{}
				}
				c.OAuthClients[issuer] = clientID
				return nil
			})
		},
	})
	if err != nil {
		return err
	}

	creds := &internalConfig.OAuthCredentials{
		Issuer:          res.Meta.Issuer,
		TokenEndpoint:   res.Meta.TokenEndpoint,
		RevokeEndpoint:  res.Meta.RevocationEndpoint,
		ClientID:        res.ClientID,
		Resource:        res.Resource,
		AccessToken:     res.Token.AccessToken,
		RefreshToken:    res.Token.RefreshToken,
		AccessExpiresAt: res.Token.ExpiresAt(),
		Scopes:          res.Token.Scopes(),
	}
	creds.Email, creds.Organization = whoami(baseURL, res.Token.AccessToken)

	var previous *internalConfig.OAuthCredentials
	var active string
	err = internalConfig.Update(func(c *internalConfig.Config) error {
		if c.Profiles == nil {
			c.Profiles = map[string]*internalConfig.Profile{}
		}
		if old := c.Profiles[name]; old != nil && old.OAuth != nil {
			previous = old.OAuth
		}
		c.Profiles[name] = &internalConfig.Profile{APIURL: apiURL, OAuth: creds}
		if c.OAuthClients == nil {
			c.OAuthClients = map[string]string{}
		}
		c.OAuthClients[res.Meta.Issuer] = res.ClientID
		if makeActive || c.CurrentProfile == "" {
			c.CurrentProfile = name
		}
		active = c.CurrentProfile
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	// A re-login into a different organization leaves the old grant alive on the
	// server. It is no longer reachable from here, so revoke it.
	if previous != nil && previous.RefreshToken != creds.RefreshToken {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		_ = oauth.Revoke(ctx, previous.RevokeEndpoint, previous.ClientID, previous.RefreshToken, "refresh_token")
		cancel()
	}

	if cmdutil.IsJSON(cmd) {
		return output.PrintJSON(map[string]interface{}{
			"profile":      name,
			"auth":         "oauth",
			"email":        creds.Email,
			"organization": creds.Organization,
			"scopes":       creds.Scopes,
			"active":       active == name,
		})
	}

	who := creds.Email
	if who == "" {
		who = "CubePath"
	}
	if creds.Organization != "" {
		who = fmt.Sprintf("%s (%s)", who, creds.Organization)
	}
	output.PrintSuccess(fmt.Sprintf("Logged in as %s, profile %q", who, name))
	if !hasWriteScope(creds.Scopes) {
		output.PrintWarning("Read-only access granted. To create or change resources, run 'cubecli login " + name + "' again and allow write permissions.")
	}
	if active == name {
		output.PrintInfo(fmt.Sprintf("Active profile: %s", name))
	} else {
		output.PrintInfo(fmt.Sprintf("Run 'cubecli profile use %s' to switch, or pass --profile %s", name, name))
	}
	warnEnvToken()
	offerSkills(cmd)
	return nil
}

// TokenLogin asks for an API token, validates it and stores it in profile `name`.
func TokenLogin(cmd *cobra.Command, name, apiURL string, makeActive bool) error {
	cfg := internalConfig.LoadOrEmpty()
	existing := cfg.Profiles[name]
	if apiURL == "" && existing != nil {
		apiURL = existing.APIURL
	}

	fmt.Fprintf(cmd.ErrOrStderr(), "Enter the CubePath API token for profile %q: ", name)
	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		return fmt.Errorf("failed to read input")
	}
	token := strings.TrimSpace(scanner.Text())
	if token == "" {
		return fmt.Errorf("API token cannot be empty")
	}

	profile := &internalConfig.Profile{APIToken: token, APIURL: apiURL}

	s := output.NewSpinner("Validating API token...")
	s.Start()
	_, err := api.NewClient(internalConfig.APIURL(profile), token).Get("/sshkey/user/sshkeys")
	s.Stop()
	if err != nil {
		return err
	}

	var previous *internalConfig.OAuthCredentials
	var active string
	err = internalConfig.Update(func(c *internalConfig.Config) error {
		if c.Profiles == nil {
			c.Profiles = map[string]*internalConfig.Profile{}
		}
		if old := c.Profiles[name]; old != nil {
			previous = old.OAuth
		}
		c.Profiles[name] = profile
		if makeActive || c.CurrentProfile == "" {
			c.CurrentProfile = name
		}
		active = c.CurrentProfile
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}
	if previous != nil {
		RevokeSession(previous)
	}

	output.PrintSuccess(fmt.Sprintf("API token saved in profile %q", name))
	if active == name {
		output.PrintInfo(fmt.Sprintf("Active profile: %s", name))
	} else {
		output.PrintInfo(fmt.Sprintf("Run 'cubecli profile use %s' to switch", name))
	}
	warnEnvToken()
	offerSkills(cmd)
	return nil
}

// offerSkills asks once, after the first interactive login, whether to install
// the CubePath skills for the AI agents found on this machine.
func offerSkills(cmd *cobra.Command) {
	skip, _ := cmd.Flags().GetBool("skip-skills")
	if skip || cmdutil.IsJSON(cmd) || !cmdutil.StdinIsTerminal() {
		return
	}
	if internalConfig.LoadOrEmpty().SkillsPrompted {
		return
	}
	targets := skills.DefaultTargets()
	if len(targets) == 0 {
		return // no agent here; ask again after one is installed
	}
	markPrompted := func() {
		_ = internalConfig.Update(func(c *internalConfig.Config) error {
			c.SkillsPrompted = true
			return nil
		})
	}
	var names []string
	for _, t := range targets {
		if dir, err := t.Dir(false); err == nil {
			if installed, _ := skills.List(dir); len(installed) > 0 {
				markPrompted() // already installed, 'cubecli skills update' keeps them current
				return
			}
		}
		names = append(names, t.Name)
	}

	fmt.Println()
	ok := cmdutil.ConfirmDefaultYes(fmt.Sprintf("Install the CubePath skills for your AI agents (%s)?", strings.Join(names, "; ")))
	markPrompted()
	if !ok {
		output.PrintInfo("You can install them later with 'cubecli skills install'.")
		return
	}
	if err := skillscmd.Run(cmd, targets, false, "", false); err != nil {
		output.PrintWarning(fmt.Sprintf("Could not install the skills: %v. Retry with 'cubecli skills install'.", err))
	}
}

// NewLogoutCmd returns `logout`, registered both at the root and under `auth`.
func NewLogoutCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logout [profile]",
		Short: "Revoke the session of a profile and remove its credentials",
		Long: `Revoke the browser session of a profile on CubePath and remove its stored
credentials. The profile itself (name, API URL) is kept, so 'cubecli login
<profile>' logs it back in. Use 'cubecli profile delete' to remove it entirely.

API tokens are only removed locally: revoke them in the dashboard.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			all, _ := cmd.Flags().GetBool("all")
			if all && len(args) > 0 {
				return fmt.Errorf("pass a profile name or --all, not both")
			}

			var names []string
			if all {
				names = internalConfig.LoadOrEmpty().ProfileNames()
			} else {
				name, err := loginProfileName(cmd, args)
				if err != nil {
					return err
				}
				names = []string{name}
			}

			var revoke []*internalConfig.OAuthCredentials
			var loggedOut []string
			err := internalConfig.Update(func(c *internalConfig.Config) error {
				for _, n := range names {
					p, ok := c.Profiles[n]
					if !ok {
						if all {
							continue
						}
						return fmt.Errorf("profile %q not found", n)
					}
					if p.AuthMethod() == "" {
						continue
					}
					if p.OAuth != nil {
						revoke = append(revoke, p.OAuth)
					}
					p.OAuth = nil
					p.APIToken = ""
					loggedOut = append(loggedOut, n)
				}
				return nil
			})
			if err != nil {
				return err
			}
			for _, creds := range revoke {
				RevokeSession(creds)
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(map[string]interface{}{"logged_out": loggedOut})
			}
			if len(loggedOut) == 0 {
				output.PrintInfo("Nothing to log out")
				return nil
			}
			for _, n := range loggedOut {
				output.PrintSuccess(fmt.Sprintf("Logged out profile %q", n))
			}
			return nil
		},
	}
	cmd.Flags().Bool("all", false, "Log out every profile")
	return cmd
}

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show how each profile is authenticated",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := internalConfig.LoadOrEmpty()
			explicit, _ := cmd.Flags().GetString("profile")
			activeName := cfg.ActiveProfileName(explicit)
			envToken := os.Getenv("CUBE_API_TOKEN") != ""

			type row struct {
				Profile      string   `json:"profile"`
				Active       bool     `json:"active"`
				Auth         string   `json:"auth"`
				Email        string   `json:"email,omitempty"`
				Organization string   `json:"organization,omitempty"`
				APIURL       string   `json:"api_url"`
				Scopes       []string `json:"scopes,omitempty"`
				Access       string   `json:"access,omitempty"`
			}
			var rows []row
			for _, n := range cfg.ProfileNames() {
				p := cfg.Profiles[n]
				r := row{
					Profile: n,
					Active:  n == activeName && !envToken,
					Auth:    p.AuthMethod(),
					APIURL:  internalConfig.APIURL(p),
				}
				if r.Auth == "" {
					r.Auth = "logged out"
				}
				if p.OAuth != nil {
					r.Email = p.OAuth.Email
					r.Organization = p.OAuth.Organization
					r.Scopes = p.OAuth.Scopes
					r.Access = "read-only"
					if hasWriteScope(p.OAuth.Scopes) {
						r.Access = "read/write"
					}
				}
				rows = append(rows, r)
			}

			if cmdutil.IsJSON(cmd) {
				return output.PrintJSON(map[string]interface{}{
					"env_token": envToken,
					"profiles":  rows,
				})
			}
			if envToken {
				output.PrintWarning("CUBE_API_TOKEN is set: it overrides every profile.")
			}
			if len(rows) == 0 {
				output.PrintWarning("No profiles configured. Run 'cubecli login'.")
				return nil
			}
			t := output.NewTable("Profiles", []string{"Active", "Profile", "Auth", "Account", "Organization", "Access"})
			for _, r := range rows {
				active := ""
				if r.Active {
					active = "*"
				}
				t.AddRow(active, r.Profile, r.Auth, r.Email, r.Organization, r.Access)
			}
			t.Render()
			return nil
		},
	}
}

// whoami returns the account email and the organization the token is bound to.
// Best effort: a failure only means the status output is less descriptive.
func whoami(baseURL, accessToken string) (email, organization string) {
	raw, err := api.NewClient(baseURL, accessToken).Get("/account/me")
	if err != nil {
		return "", ""
	}
	var me struct {
		Email         string `json:"email"`
		Organizations []struct {
			Name     string `json:"name"`
			IsActive bool   `json:"is_active"`
		} `json:"organizations"`
	}
	if json.Unmarshal(raw, &me) != nil {
		return "", ""
	}
	for _, o := range me.Organizations {
		if o.IsActive {
			organization = o.Name
		}
	}
	return me.Email, organization
}

func hasWriteScope(scopes []string) bool {
	for _, s := range scopes {
		if strings.HasSuffix(s, ":write") {
			return true
		}
	}
	return false
}

// RevokeSession revokes a browser session on the server, warning instead of
// failing: the local credentials are removed either way.
func RevokeSession(creds *internalConfig.OAuthCredentials) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := oauth.Revoke(ctx, creds.RevokeEndpoint, creds.ClientID, creds.RefreshToken, "refresh_token"); err != nil {
		output.PrintWarning(fmt.Sprintf("Could not revoke the session on the server (%v). Disconnect it in the dashboard under Account > Connections.", err))
	}
}

func warnEnvToken() {
	if os.Getenv("CUBE_API_TOKEN") != "" {
		output.PrintWarning("CUBE_API_TOKEN is set and overrides every profile until you unset it.")
	}
}
