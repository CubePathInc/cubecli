package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

const (
	DefaultAPIURL      = "https://api.cubepath.com"
	DefaultProfileName = "default"
	configDir          = ".cubecli"
	configFile         = "config.json"
)

type Profile struct {
	APIToken string `json:"api_token"`
	APIURL   string `json:"api_url,omitempty"`
}

type Config struct {
	CurrentProfile string              `json:"current_profile"`
	Profiles       map[string]*Profile `json:"profiles"`
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

func Save(cfg *Config) error {
	if err := os.MkdirAll(Dir(), 0700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(Path(), data, 0600)
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
			return nil, "", fmt.Errorf("no profiles configured: set CUBE_API_TOKEN or run 'cubecli config setup'")
		}
		return nil, "", fmt.Errorf("profile %q not found (known: %s)", name, c.profileNamesList())
	}
	if p.APIToken == "" {
		return nil, "", fmt.Errorf("profile %q has no API token: run 'cubecli profile add %s'", name, name)
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
