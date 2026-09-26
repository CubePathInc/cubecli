package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

const (
	DefaultAPIURL      = "https://api.cubepath.com"
	DefaultProfileName = "default"
	configDir          = ".cubecli"
	configFile         = "config.json"
)

// Profile holds the credentials for one account/organization. Exactly one of
// APIToken (static token, e.g. for CI) or OAuth (browser login) is set.
type Profile struct {
	APIToken string            `json:"api_token,omitempty"`
	APIURL   string            `json:"api_url,omitempty"`
	OAuth    *OAuthCredentials `json:"oauth,omitempty"`
}

// OAuthCredentials is what `cubecli login` stores. The access token is short
// lived and refreshed transparently; the refresh token rotates on every use.
type OAuthCredentials struct {
	Issuer          string    `json:"issuer"`
	TokenEndpoint   string    `json:"token_endpoint"`
	RevokeEndpoint  string    `json:"revocation_endpoint,omitempty"`
	ClientID        string    `json:"client_id"`
	Resource        string    `json:"resource"`
	AccessToken     string    `json:"access_token"`
	RefreshToken    string    `json:"refresh_token"`
	AccessExpiresAt time.Time `json:"access_expires_at"`
	Scopes          []string  `json:"scopes,omitempty"`
	Email           string    `json:"email,omitempty"`
	Organization    string    `json:"organization,omitempty"`
}

// AuthMethod reports how a profile authenticates: "oauth", "token" or "".
func (p *Profile) AuthMethod() string {
	switch {
	case p == nil:
		return ""
	case p.OAuth != nil:
		return "oauth"
	case p.APIToken != "":
		return "token"
	}
	return ""
}

type Config struct {
	CurrentProfile string              `json:"current_profile"`
	Profiles       map[string]*Profile `json:"profiles"`
	// OAuthClients maps an authorization server issuer to the client_id this
	// machine registered there, so every login reuses one client instead of
	// registering a new one each time.
	OAuthClients map[string]string `json:"oauth_clients,omitempty"`
	// SkillsPrompted records that `cubecli login` already offered to install
	// the agent skills, so it asks only once.
	SkillsPrompted bool `json:"skills_prompted,omitempty"`
}

// legacyConfig matches the pre-profiles config shape for auto-migration.
type legacyConfig struct {
	APIToken string `json:"api_token"`
}

func Dir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, configDir)
}

func Path() string {
	return filepath.Join(Dir(), configFile)
}

// Load reads and parses the config file, migrating the legacy format on the fly.
// Returns an error if the file is missing or empty; callers that want to fall back
// to env vars should use ActiveProfile instead.
func Load() (*Config, error) {
	data, err := os.ReadFile(Path())
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("invalid config file: %w", err)
	}

	if len(cfg.Profiles) == 0 {
		var legacy legacyConfig
		if err := json.Unmarshal(data, &legacy); err == nil && legacy.APIToken != "" {
			cfg = Config{
				CurrentProfile: DefaultProfileName,
				Profiles: map[string]*Profile{
					DefaultProfileName: {APIToken: legacy.APIToken},
				},
			}
			_ = Save(&cfg)
		}
	}

	if cfg.Profiles == nil {
		cfg.Profiles = map[string]*Profile{}
	}

	return &cfg, nil
}

// LoadOrEmpty returns the parsed config or an empty one if no file exists yet.
func LoadOrEmpty() *Config {
	cfg, err := Load()
	if err != nil {
		return &Config{Profiles: map[string]*Profile{}}
	}
	return cfg
}

// Save writes the config atomically (temp file + rename), so a concurrent reader
// never sees a half-written file.
func Save(cfg *Config) error {
	if err := os.MkdirAll(Dir(), 0700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	tmp, err := os.CreateTemp(Dir(), configFile+".*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if err := tmp.Chmod(0600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), Path())
}

// Update runs fn on a freshly loaded config while holding the config lock and
// saves the result. Use it for any read-modify-write that can race with another
// cubecli process, such as refreshing OAuth tokens.
func Update(fn func(cfg *Config) error) error {
	unlock, err := Lock()
	if err != nil {
		return err
	}
	defer unlock()

	cfg := LoadOrEmpty()
	if err := fn(cfg); err != nil {
		return err
	}
	return Save(cfg)
}

// ActiveProfileName resolves which profile should be used, in order:
// explicit name > CUBE_PROFILE env > cfg.CurrentProfile > "default".
func (c *Config) ActiveProfileName(explicit string) string {
	if explicit != "" {
		return explicit
	}
	if env := os.Getenv("CUBE_PROFILE"); env != "" {
		return env
	}
	if c.CurrentProfile != "" {
		return c.CurrentProfile
	}
	return DefaultProfileName
}

// ActiveProfile returns the profile that should serve the current invocation.
// If CUBE_API_TOKEN is set, it synthesizes an ephemeral profile so env-based
// auth keeps working even without a config file.
func (c *Config) ActiveProfile(explicit string) (*Profile, string, error) {
	if token := os.Getenv("CUBE_API_TOKEN"); token != "" {
		return &Profile{APIToken: token}, "env", nil
	}

	name := c.ActiveProfileName(explicit)
	p, ok := c.Profiles[name]
	if !ok {
		if len(c.Profiles) == 0 {
			return nil, "", fmt.Errorf("no profiles configured: run 'cubecli login' or set CUBE_API_TOKEN")
		}
		return nil, "", fmt.Errorf("profile %q not found (known: %s)", name, c.profileNamesList())
	}
	if p.AuthMethod() == "" {
		return nil, "", fmt.Errorf("profile %q has no credentials: run 'cubecli login --profile %s'", name, name)
	}
	return p, name, nil
}

func (c *Config) ProfileNames() []string {
	names := make([]string, 0, len(c.Profiles))
	for n := range c.Profiles {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

func (c *Config) profileNamesList() string {
	names := c.ProfileNames()
	out := ""
	for i, n := range names {
		if i > 0 {
			out += ", "
		}
		out += n
	}
	return out
}

// APIURL returns the base URL to hit for a given profile, honouring CUBE_API_URL.
func APIURL(p *Profile) string {
	if url := os.Getenv("CUBE_API_URL"); url != "" {
		return url
	}
	if p != nil && p.APIURL != "" {
		return p.APIURL
	}
	return DefaultAPIURL
}
