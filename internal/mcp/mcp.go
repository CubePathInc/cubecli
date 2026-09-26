// Package mcp connects the CubePath MCP server to the AI agents on this
// machine. Each agent runs the OAuth authorization itself on first use, so no
// credential is ever written by cubecli.
package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

// ServerName is the name the server is registered under in every agent.
const ServerName = "cubepath"

// DefaultURL is the production MCP endpoint.
const DefaultURL = "https://mcp.cubepath.com/mcp"

// ResolveURL returns the MCP endpoint that belongs to the API at apiURL, read
// from its protected resource metadata, or DefaultURL.
func ResolveURL(ctx context.Context, apiURL string) string {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	u := strings.TrimRight(apiURL, "/") + "/.well-known/oauth-protected-resource/mcp"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return DefaultURL
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return DefaultURL
	}
	defer resp.Body.Close()
	var pr struct {
		Resource string `json:"resource"`
	}
	if resp.StatusCode != http.StatusOK || json.NewDecoder(resp.Body).Decode(&pr) != nil {
		return DefaultURL
	}
	if !strings.HasPrefix(pr.Resource, "https://") && !strings.HasPrefix(pr.Resource, "http://127.0.0.1") && !strings.HasPrefix(pr.Resource, "http://localhost") {
		return DefaultURL
	}
	return pr.Resource
}

// Client is an AI agent the MCP server can be added to.
type Client struct {
	ID   string // value of --agent
	Name string
	// NextStep tells the user how to finish the authorization in the agent.
	NextStep string

	detect     func() bool
	configured func(url string) (bool, error)
	install    func(ctx context.Context, url string) error
	uninstall  func(ctx context.Context, url string) error
}

// Detected reports whether the agent seems to be installed.
func (c Client) Detected() bool { return c.detect() }

// Configured reports whether the CubePath server is already set up.
func (c Client) Configured(url string) (bool, error) { return c.configured(url) }

// Install adds the CubePath server to the agent.
func (c Client) Install(ctx context.Context, url string) error { return c.install(ctx, url) }

// Uninstall removes the CubePath server from the agent.
func (c Client) Uninstall(ctx context.Context, url string) error { return c.uninstall(ctx, url) }

// Clients lists the supported agents.
var Clients = []Client{
	{
		ID: "claude", Name: "Claude Code",
		NextStep: "In Claude Code, run /mcp and choose cubepath to authorize it in the browser.",
		detect:   func() bool { return hasBinary("claude") },
		configured: func(string) (bool, error) {
			return exec.Command("claude", "mcp", "get", ServerName).Run() == nil, nil
		},
		install: func(ctx context.Context, url string) error {
			return run(ctx, "claude", "mcp", "add", "--transport", "http", "--scope", "user", ServerName, url)
		},
		uninstall: func(ctx context.Context, _ string) error {
			return run(ctx, "claude", "mcp", "remove", "--scope", "user", ServerName)
		},
	},
	{
		ID: "codex", Name: "Codex",
		NextStep: "Run 'codex mcp login cubepath' to authorize it in the browser.",
		detect:   func() bool { return hasBinary("codex") || homeDirExists(".codex") },
		configured: func(url string) (bool, error) {
			data, err := readHome(".codex", "config.toml")
			return tomlHasServer(data) || strings.Contains(string(data), url), err
		},
		install: func(_ context.Context, url string) error {
			return updateHomeFile(0600, func(data []byte) ([]byte, error) {
				return tomlAddServer(data, url), nil
			}, ".codex", "config.toml")
		},
		uninstall: func(_ context.Context, _ string) error {
			return updateHomeFile(0600, func(data []byte) ([]byte, error) {
				return tomlRemoveServer(data), nil
			}, ".codex", "config.toml")
		},
	},
	{
		ID: "gemini", Name: "Gemini CLI",
		NextStep: "In Gemini CLI, run /mcp auth cubepath to authorize it in the browser.",
		detect:   func() bool { return hasBinary("gemini") || homeDirExists(".gemini") },
		configured: func(url string) (bool, error) {
			return jsonHasServer("mcpServers", url, ".gemini", "settings.json")
		},
		install: func(ctx context.Context, url string) error {
			// The timeout leaves room for tools that wait up to 120 s for a
			// resource to settle.
			err := updateHomeFile(0644, func(data []byte) ([]byte, error) {
				return jsonSetServer(data, "mcpServers", map[string]interface{}{"httpUrl": url, "timeout": 130000})
			}, ".gemini", "settings.json")
			if err != nil && hasBinary("gemini") {
				// settings.json may contain comments; let Gemini CLI edit it.
				return run(ctx, "gemini", "mcp", "add", "--scope", "user", "--transport", "http", "--timeout", "130000", ServerName, url)
			}
			return err
		},
		uninstall: func(ctx context.Context, _ string) error {
			err := updateHomeFile(0644, func(data []byte) ([]byte, error) {
				return jsonDeleteServer(data, "mcpServers")
			}, ".gemini", "settings.json")
			if err != nil && hasBinary("gemini") {
				return run(ctx, "gemini", "mcp", "remove", "--scope", "user", ServerName)
			}
			return err
		},
	},
	{
		ID: "cursor", Name: "Cursor",
		NextStep: "Reload Cursor; it opens the browser to authorize cubepath.",
		detect:   func() bool { return homeDirExists(".cursor") || hasBinary("cursor") },
		configured: func(url string) (bool, error) {
			return jsonHasServer("mcpServers", url, ".cursor", "mcp.json")
		},
		install: func(_ context.Context, url string) error {
			return updateHomeFile(0644, func(data []byte) ([]byte, error) {
				return jsonSetServer(data, "mcpServers", map[string]string{"url": url})
			}, ".cursor", "mcp.json")
		},
		uninstall: func(_ context.Context, _ string) error {
			return updateHomeFile(0644, func(data []byte) ([]byte, error) {
				return jsonDeleteServer(data, "mcpServers")
			}, ".cursor", "mcp.json")
		},
	},
	{
		ID: "vscode", Name: "VS Code",
		NextStep: "In VS Code, start the cubepath server from the MCP view; it opens the browser to authorize it.",
		detect:   func() bool { return hasBinary("code") },
		configured: func(url string) (bool, error) {
			ok, err := jsonHasServer("servers", url, vscodeUserDir(), "mcp.json")
			if err != nil {
				return false, nil // mcp.json allows comments; let `code` deal with it
			}
			return ok, nil
		},
		install: func(ctx context.Context, url string) error {
			spec, _ := json.Marshal(map[string]string{"name": ServerName, "type": "http", "url": url})
			return run(ctx, "code", "--add-mcp", string(spec))
		},
		uninstall: func(_ context.Context, _ string) error {
			err := updateHomeFile(0644, func(data []byte) ([]byte, error) {
				return jsonDeleteServer(data, "servers")
			}, vscodeUserDir(), "mcp.json")
			if err != nil {
				return fmt.Errorf("%w; remove the cubepath entry from VS Code's mcp.json by hand", err)
			}
			return nil
		},
	},
}

// ClientByID returns the client with the given --agent value.
func ClientByID(id string) (Client, bool) {
	for _, c := range Clients {
		if c.ID == id {
			return c, true
		}
	}
	return Client{}, false
}

// Detected returns the clients installed on this machine.
func Detected() []Client {
	var out []Client
	for _, c := range Clients {
		if c.Detected() {
			out = append(out, c)
		}
	}
	return out
}

func run(ctx context.Context, name string, args ...string) error {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %s", name, strings.Join(args[:2], " "), lastLine(string(out), err))
	}
	return nil
}

// lastLine keeps the last non-empty line of a command's output, which is
// where CLIs put the error, instead of a full stack trace.
func lastLine(out string, err error) string {
	lines := strings.Split(strings.TrimSpace(out), "\n")
	msg := strings.TrimSpace(lines[len(lines)-1])
	if msg == "" {
		return err.Error()
	}
	if len(msg) > 300 {
		msg = msg[:300] + "..."
	}
	return msg
}

func hasBinary(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func homeDirExists(name string) bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	fi, err := os.Stat(filepath.Join(home, name))
	return err == nil && fi.IsDir()
}

// vscodeUserDir is VS Code's user settings directory, relative to $HOME.
func vscodeUserDir() string {
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join("Library", "Application Support", "Code", "User")
	case "windows":
		return filepath.Join("AppData", "Roaming", "Code", "User")
	default:
		return filepath.Join(".config", "Code", "User")
	}
}

func homePath(parts ...string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(append([]string{home}, parts...)...), nil
}

// readHome returns the file's content, or nil if it does not exist.
func readHome(parts ...string) ([]byte, error) {
	p, err := homePath(parts...)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	return data, err
}

// updateHomeFile rewrites a file under $HOME through fn, atomically, keeping
// its permissions. A missing file starts empty and is created with mode.
func updateHomeFile(mode os.FileMode, fn func([]byte) ([]byte, error), parts ...string) error {
	p, err := homePath(parts...)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(p)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if fi, err := os.Stat(p); err == nil {
		mode = fi.Mode().Perm()
	}
	updated, err := fn(data)
	if err != nil {
		return fmt.Errorf("%s: %w", p, err)
	}
	if string(updated) == string(data) {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(p), filepath.Base(p)+".*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(updated); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), p)
}

// JSON config files (Cursor, VS Code, Gemini CLI).

func jsonHasServer(key, url string, parts ...string) (bool, error) {
	data, err := readHome(parts...)
	if err != nil || len(strings.TrimSpace(string(data))) == 0 {
		return false, err
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(data, &doc); err != nil {
		return false, err
	}
	var servers map[string]json.RawMessage
	if raw, ok := doc[key]; ok {
		if err := json.Unmarshal(raw, &servers); err != nil {
			return false, err
		}
	}
	if _, ok := servers[ServerName]; ok {
		return true, nil
	}
	for _, s := range servers {
		if strings.Contains(string(s), `"`+url+`"`) {
			return true, nil
		}
	}
	return false, nil
}

func jsonSetServer(data []byte, key string, server interface{}) ([]byte, error) {
	doc, servers, err := jsonServers(data, key)
	if err != nil {
		return nil, err
	}
	if _, ok := servers[ServerName]; ok {
		return data, nil
	}
	servers[ServerName], _ = json.Marshal(server)
	return jsonWrite(doc, servers, key)
}

func jsonDeleteServer(data []byte, key string) ([]byte, error) {
	if len(strings.TrimSpace(string(data))) == 0 {
		return data, nil
	}
	doc, servers, err := jsonServers(data, key)
	if err != nil {
		return nil, err
	}
	if _, ok := servers[ServerName]; !ok {
		return data, nil
	}
	delete(servers, ServerName)
	return jsonWrite(doc, servers, key)
}

func jsonServers(data []byte, key string) (map[string]json.RawMessage, map[string]json.RawMessage, error) {
	doc := map[string]json.RawMessage{}
	if len(strings.TrimSpace(string(data))) > 0 {
		if err := json.Unmarshal(data, &doc); err != nil {
			return nil, nil, fmt.Errorf("not valid JSON, leaving it untouched: %w", err)
		}
	}
	servers := map[string]json.RawMessage{}
	if raw, ok := doc[key]; ok {
		if err := json.Unmarshal(raw, &servers); err != nil {
			return nil, nil, fmt.Errorf("%q is not an object, leaving the file untouched", key)
		}
	}
	return doc, servers, nil
}

func jsonWrite(doc, servers map[string]json.RawMessage, key string) ([]byte, error) {
	raw, err := json.Marshal(servers)
	if err != nil {
		return nil, err
	}
	doc[key] = raw
	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(out, '\n'), nil
}

// Codex config.toml.

var tomlHeader = regexp.MustCompile(`(?m)^[ \t]*\[mcp_servers\.(?:cubepath|"cubepath")\][ \t]*(?:#.*)?$`)

func tomlHasServer(data []byte) bool {
	return tomlHeader.Match(data)
}

func tomlAddServer(data []byte, url string) []byte {
	if tomlHasServer(data) {
		return data
	}
	block := fmt.Sprintf("[mcp_servers.%s]\nurl = %q\ntool_timeout_sec = 130\n", ServerName, url)
	s := string(data)
	switch {
	case s == "":
		return []byte(block)
	case strings.HasSuffix(s, "\n\n"):
		return []byte(s + block)
	case strings.HasSuffix(s, "\n"):
		return []byte(s + "\n" + block)
	default:
		return []byte(s + "\n\n" + block)
	}
}

// tomlRemoveServer deletes the [mcp_servers.cubepath] table, up to the next
// table header, together with its sub-tables such as [mcp_servers.cubepath.env].
func tomlRemoveServer(data []byte) []byte {
	if !tomlHasServer(data) {
		return data
	}
	lines := strings.SplitAfter(string(data), "\n")
	var out []string
	skipping := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") {
			name := strings.Trim(strings.SplitN(trimmed, "]", 2)[0], "[] ")
			skipping = name == "mcp_servers.cubepath" || name == `mcp_servers."cubepath"` ||
				strings.HasPrefix(name, "mcp_servers.cubepath.") || strings.HasPrefix(name, `mcp_servers."cubepath".`)
		}
		if !skipping {
			out = append(out, line)
		}
	}
	rest := strings.TrimRight(strings.Join(out, ""), "\n")
	if rest == "" {
		return nil
	}
	return []byte(rest + "\n")
}
