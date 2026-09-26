package skills

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// Target is a skills directory read by one or more agents.
type Target struct {
	ID     string // value of --agent
	Name   string // who reads it, for humans
	parent string // config directory under $HOME, e.g. ".claude"
	probes []string
	bins   []string
}

// Targets lists every place skills can be installed. Paths per the agents' docs:
// Claude Code reads ~/.claude/skills; Codex, Gemini CLI and Cursor all read
// ~/.agents/skills (Cursor also reads ~/.claude/skills).
var Targets = []Target{
	{ID: "claude", Name: "Claude Code", parent: ".claude", probes: []string{".claude"}, bins: []string{"claude"}},
	{ID: "agents", Name: "Codex, Gemini CLI, Cursor", parent: ".agents",
		probes: []string{".codex", ".gemini", ".cursor", ".agents"}, bins: []string{"codex", "gemini", "cursor-agent", "cursor"}},
}

// TargetByID returns the target with the given --agent value.
func TargetByID(id string) (Target, bool) {
	for _, t := range Targets {
		if t.ID == id {
			return t, true
		}
	}
	return Target{}, false
}

// Dir is the skills directory: under $HOME, or under the current directory for
// a project install (committed with the repository).
func (t Target) Dir(project bool) (string, error) {
	base, err := os.UserHomeDir()
	if project {
		base, err = os.Getwd()
	}
	if err != nil {
		return "", err
	}
	return filepath.Join(base, t.parent, "skills"), nil
}

// Detected reports whether one of the agents reading this target seems to be
// installed.
func (t Target) Detected() bool {
	home, _ := os.UserHomeDir()
	for _, p := range t.probes {
		if fi, err := os.Stat(filepath.Join(home, p)); err == nil && fi.IsDir() {
			return true
		}
	}
	for _, b := range t.bins {
		if _, err := exec.LookPath(b); err == nil {
			return true
		}
	}
	return false
}

// ClaudePluginInstalled reports whether the CubePath plugin is installed in
// Claude Code, in which case copying the skills into ~/.claude/skills would
// load them twice.
func ClaudePluginInstalled() bool {
	home, _ := os.UserHomeDir()
	data, err := os.ReadFile(filepath.Join(home, ".claude", "plugins", "installed_plugins.json"))
	if err != nil {
		return false
	}
	var f struct {
		Plugins map[string]json.RawMessage `json:"plugins"`
	}
	if json.Unmarshal(data, &f) != nil {
		return false
	}
	for id := range f.Plugins {
		if strings.HasPrefix(id, "cubepath@") {
			return true
		}
	}
	return false
}

// DefaultTargets returns the detected targets, leaving out Claude Code when
// the plugin already provides the skills there.
func DefaultTargets() []Target {
	var out []Target
	for _, t := range Targets {
		if !t.Detected() {
			continue
		}
		if t.ID == "claude" && ClaudePluginInstalled() {
			continue
		}
		out = append(out, t)
	}
	return out
}

// markerFile marks a skill directory as installed by cubecli. Without it,
// cubecli never touches a directory, so a user's own skill with the same name
// is safe.
const markerFile = ".cubecli-skill.json"

type marker struct {
	Version string `json:"version"`
	Hash    string `json:"hash"`
}

// Installed is a skill found in a target directory.
type Installed struct {
	Name     string `json:"name"`
	Version  string `json:"version"`
	Modified bool   `json:"modified"` // edited since cubecli installed it
}

// List returns the cubecli-managed skills in dir.
func List(dir string) ([]Installed, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []Installed
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		m, ok := readMarker(filepath.Join(dir, e.Name()))
		if !ok {
			continue
		}
		h, _ := treeHash(filepath.Join(dir, e.Name()))
		out = append(out, Installed{Name: e.Name(), Version: m.Version, Modified: h != m.Hash})
	}
	return out, nil
}

// Outcome of installing or removing one skill.
type Outcome struct {
	Name   string `json:"name"`
	Action string `json:"action"` // installed, updated, unchanged, skipped, removed
	Reason string `json:"reason,omitempty"`
}

// Install copies every skill of the bundle into dir. Existing directories that
// cubecli did not create, or that the user modified, are skipped unless force.
func Install(b *Bundle, dir string, force bool) ([]Outcome, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	var out []Outcome
	for _, name := range b.Manifest.Skills {
		o, err := installOne(b, dir, name, force)
		if err != nil {
			return out, fmt.Errorf("%s: %w", name, err)
		}
		out = append(out, o)
	}
	return out, nil
}

func installOne(b *Bundle, dir, name string, force bool) (Outcome, error) {
	dest := filepath.Join(dir, name)
	action := "installed"
	if fi, err := os.Lstat(dest); err == nil {
		m, managed := readMarker(dest)
		switch {
		case !fi.IsDir() || fi.Mode()&os.ModeSymlink != 0:
			if !force {
				return Outcome{name, "skipped", "exists and is not a directory cubecli installed"}, nil
			}
		case !managed:
			if !force {
				return Outcome{name, "skipped", "a skill with this name exists and was not installed by cubecli (use --force to replace it)"}, nil
			}
		default:
			if h, _ := treeHash(dest); h != m.Hash && !force {
				return Outcome{name, "skipped", "modified since it was installed (use --force to overwrite)"}, nil
			}
			if m.Version == b.Manifest.Version {
				action = "unchanged"
			} else {
				action = "updated"
			}
		}
	}

	// Build the new copy next to the destination, then swap it in, so a failure
	// never leaves a half-written skill behind.
	staging, err := os.MkdirTemp(dir, "."+name+".new-")
	if err != nil {
		return Outcome{}, err
	}
	defer os.RemoveAll(staging)
	if err := copyTree(filepath.Join(b.Dir, name), staging); err != nil {
		return Outcome{}, err
	}
	h, err := treeHash(staging)
	if err != nil {
		return Outcome{}, err
	}
	data, _ := json.MarshalIndent(marker{Version: b.Manifest.Version, Hash: h}, "", "  ")
	if err := os.WriteFile(filepath.Join(staging, markerFile), data, 0644); err != nil {
		return Outcome{}, err
	}
	if err := os.Chmod(staging, 0755); err != nil {
		return Outcome{}, err
	}

	old := ""
	if _, err := os.Lstat(dest); err == nil {
		old = filepath.Join(dir, "."+name+".old-"+strings.TrimPrefix(filepath.Base(staging), "."+name+".new-"))
		if err := os.Rename(dest, old); err != nil {
			return Outcome{}, err
		}
	}
	if err := os.Rename(staging, dest); err != nil {
		if old != "" {
			_ = os.Rename(old, dest)
		}
		return Outcome{}, err
	}
	if old != "" {
		os.RemoveAll(old)
	}
	return Outcome{Name: name, Action: action}, nil
}

// Uninstall removes the cubecli-managed skills in dir. Modified ones are kept
// unless force.
func Uninstall(dir string, force bool) ([]Outcome, error) {
	installed, err := List(dir)
	if err != nil {
		return nil, err
	}
	var out []Outcome
	for _, s := range installed {
		if s.Modified && !force {
			out = append(out, Outcome{s.Name, "skipped", "modified since it was installed (use --force to remove it)"})
			continue
		}
		if err := os.RemoveAll(filepath.Join(dir, s.Name)); err != nil {
			return out, err
		}
		out = append(out, Outcome{Name: s.Name, Action: "removed"})
	}
	return out, nil
}

func readMarker(dir string) (marker, bool) {
	data, err := os.ReadFile(filepath.Join(dir, markerFile))
	if err != nil {
		return marker{}, false
	}
	var m marker
	if json.Unmarshal(data, &m) != nil || m.Hash == "" {
		return marker{}, false
	}
	return m, true
}

// treeHash hashes the relative paths and contents of every regular file under
// dir, except the marker.
func treeHash(dir string) (string, error) {
	var files []string
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type().IsRegular() && d.Name() != markerFile {
			rel, _ := filepath.Rel(dir, p)
			files = append(files, filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Strings(files)
	h := sha256.New()
	for _, rel := range files {
		f, err := os.Open(filepath.Join(dir, filepath.FromSlash(rel)))
		if err != nil {
			return "", err
		}
		fmt.Fprintf(h, "%s\x00", rel)
		_, err = io.Copy(h, f)
		f.Close()
		if err != nil {
			return "", err
		}
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		target := filepath.Join(dst, rel)
		switch {
		case d.IsDir():
			return os.MkdirAll(target, 0755)
		case d.Type().IsRegular():
			in, err := os.Open(p)
			if err != nil {
				return err
			}
			defer in.Close()
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, in); err != nil {
				out.Close()
				return err
			}
			return out.Close()
		default:
			return nil // extract() only produces files and directories
		}
	})
}
