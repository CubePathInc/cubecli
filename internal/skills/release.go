// Package skills installs the CubePath agent skills (github.com/CubePathInc/skills)
// into the skill directories of the AI coding agents on this machine.
package skills

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/CubePathInc/cubecli/internal/version"
	"github.com/Masterminds/semver/v3"
)

// releasesAPI is the GitHub releases API of the skills repository.
// CUBECLI_SKILLS_RELEASES_API overrides it (a mirror, or a test server).
var releasesAPI = func() string {
	if v := os.Getenv("CUBECLI_SKILLS_RELEASES_API"); v != "" {
		return strings.TrimRight(v, "/")
	}
	return "https://api.github.com/repos/CubePathInc/skills/releases"
}()

const (
	maxArchiveBytes   = 10 << 20
	maxExtractedBytes = 20 << 20
	maxFiles          = 2000
)

var httpClient = &http.Client{Timeout: 60 * time.Second}

// Manifest is manifest.json at the root of a skills release.
type Manifest struct {
	Version           string   `json:"version"`
	MinCubecliVersion string   `json:"min_cubecli_version"`
	Skills            []string `json:"skills"`
}

// Bundle is a downloaded, verified and extracted release.
type Bundle struct {
	Manifest Manifest
	// Dir holds one directory per skill.
	Dir string
	tmp string
}

// Cleanup removes the extracted files.
func (b *Bundle) Cleanup() { os.RemoveAll(b.tmp) }

type release struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
	} `json:"assets"`
}

// LatestVersion returns the tag of the latest release, e.g. "v0.1.0".
func LatestVersion(ctx context.Context) (string, error) {
	rel, err := fetchRelease(ctx, "")
	if err != nil {
		return "", err
	}
	return rel.TagName, nil
}

// Download fetches a release ("" = latest), checks it against SHA256SUMS and
// extracts it to a temporary directory. The caller must call Cleanup.
func Download(ctx context.Context, tag string) (*Bundle, error) {
	rel, err := fetchRelease(ctx, tag)
	if err != nil {
		return nil, err
	}
	var archiveURL, archiveName, sumsURL string
	for _, a := range rel.Assets {
		switch {
		case a.Name == "SHA256SUMS":
			sumsURL = a.URL
		case strings.HasPrefix(a.Name, "cubepath-skills-") && strings.HasSuffix(a.Name, ".tar.gz"):
			archiveURL, archiveName = a.URL, a.Name
		}
	}
	if archiveURL == "" || sumsURL == "" {
		return nil, fmt.Errorf("release %s has no skills archive or SHA256SUMS", rel.TagName)
	}

	sums, err := fetch(ctx, sumsURL, 1<<16)
	if err != nil {
		return nil, fmt.Errorf("failed to download SHA256SUMS: %w", err)
	}
	want, err := checksumFor(sums, archiveName)
	if err != nil {
		return nil, err
	}
	archive, err := fetch(ctx, archiveURL, maxArchiveBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to download %s: %w", archiveName, err)
	}
	got := sha256.Sum256(archive)
	if hex.EncodeToString(got[:]) != want {
		return nil, fmt.Errorf("checksum mismatch for %s: refusing to install", archiveName)
	}

	tmp, err := os.MkdirTemp("", "cubecli-skills-")
	if err != nil {
		return nil, err
	}
	root, err := extract(archive, tmp)
	if err != nil {
		os.RemoveAll(tmp)
		return nil, err
	}

	var m Manifest
	data, err := os.ReadFile(filepath.Join(root, "manifest.json"))
	if err == nil {
		err = json.Unmarshal(data, &m)
	}
	if err != nil {
		os.RemoveAll(tmp)
		return nil, fmt.Errorf("invalid skills release: manifest.json: %w", err)
	}
	if err := checkCompatible(m); err != nil {
		os.RemoveAll(tmp)
		return nil, err
	}
	skillsDir := filepath.Join(root, "skills")
	for _, name := range m.Skills {
		if !validSkillName(name) {
			os.RemoveAll(tmp)
			return nil, fmt.Errorf("invalid skill name %q in manifest", name)
		}
		if _, err := os.Stat(filepath.Join(skillsDir, name, "SKILL.md")); err != nil {
			os.RemoveAll(tmp)
			return nil, fmt.Errorf("invalid skills release: %s has no SKILL.md", name)
		}
	}
	return &Bundle{Manifest: m, Dir: skillsDir, tmp: tmp}, nil
}

func checkCompatible(m Manifest) error {
	if m.MinCubecliVersion == "" || version.Version == "dev" {
		return nil
	}
	current, err := semver.NewVersion(version.Version)
	if err != nil {
		return nil
	}
	min, err := semver.NewVersion(m.MinCubecliVersion)
	if err != nil {
		return nil
	}
	if current.LessThan(min) {
		return fmt.Errorf("these skills need cubecli %s or later (you have %s): run 'cubecli update' first", m.MinCubecliVersion, version.Version)
	}
	return nil
}

func fetchRelease(ctx context.Context, tag string) (*release, error) {
	u := releasesAPI + "/latest"
	if tag != "" {
		if !strings.HasPrefix(tag, "v") {
			tag = "v" + tag
		}
		u = releasesAPI + "/tags/" + tag
	}
	data, err := fetch(ctx, u, 1<<20)
	if err != nil {
		return nil, fmt.Errorf("failed to look up the skills release: %w", err)
	}
	var rel release
	if err := json.Unmarshal(data, &rel); err != nil || rel.TagName == "" {
		return nil, fmt.Errorf("unexpected response from the releases API")
	}
	return &rel, nil
}

func fetch(ctx context.Context, u string, limit int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", fmt.Sprintf("CubeCLI/%s", version.Version))
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, errors.New("not found")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("response larger than %d bytes", limit)
	}
	return data, nil
}

func checksumFor(sums []byte, name string) (string, error) {
	sc := bufio.NewScanner(bytes.NewReader(sums))
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) == 2 && strings.TrimPrefix(fields[1], "*") == name {
			return strings.ToLower(fields[0]), nil
		}
	}
	return "", fmt.Errorf("SHA256SUMS has no entry for %s", name)
}

// extract unpacks a .tar.gz into dir and returns the single top-level directory.
// Only regular files and directories are accepted, and every path must stay
// inside dir: no absolute paths, no "..", no links.
func extract(archive []byte, dir string) (string, error) {
	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return "", fmt.Errorf("invalid skills archive: %w", err)
	}
	tr := tar.NewReader(gz)
	var top string
	var total int64
	files := 0
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("invalid skills archive: %w", err)
		}
		name := path.Clean(hdr.Name)
		if path.IsAbs(hdr.Name) || name == ".." || strings.HasPrefix(name, "../") || strings.Contains(hdr.Name, "\\") {
			return "", fmt.Errorf("invalid skills archive: unsafe path %q", hdr.Name)
		}
		first := strings.SplitN(name, "/", 2)[0]
		if top == "" {
			top = first
		} else if first != top {
			return "", fmt.Errorf("invalid skills archive: more than one top-level directory")
		}
		target := filepath.Join(dir, filepath.FromSlash(name))
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return "", err
			}
		case tar.TypeReg:
			files++
			total += hdr.Size
			if files > maxFiles || total > maxExtractedBytes {
				return "", fmt.Errorf("invalid skills archive: too large")
			}
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return "", err
			}
			f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0644)
			if err != nil {
				return "", err
			}
			_, err = io.Copy(f, io.LimitReader(tr, hdr.Size))
			f.Close()
			if err != nil {
				return "", err
			}
		case tar.TypeXGlobalHeader, tar.TypeXHeader:
			continue
		default:
			return "", fmt.Errorf("invalid skills archive: %q is not a regular file", hdr.Name)
		}
	}
	if top == "" || top == "." {
		return "", fmt.Errorf("invalid skills archive: empty")
	}
	return filepath.Join(dir, top), nil
}

func validSkillName(name string) bool {
	if name == "" || len(name) > 64 {
		return false
	}
	for _, r := range name {
		lower, digit := r >= 'a' && r <= 'z', r >= '0' && r <= '9'
		if !lower && !digit && r != '-' {
			return false
		}
	}
	return !strings.HasPrefix(name, "-") && !strings.HasSuffix(name, "-")
}
