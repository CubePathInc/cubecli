package skills

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/CubePathInc/cubecli/internal/version"
)

type entry struct {
	name, body string
	typ        byte
}

func tarball(t *testing.T, entries []entry) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, e := range entries {
		typ := e.typ
		if typ == 0 {
			typ = tar.TypeReg
		}
		hdr := &tar.Header{Name: e.name, Mode: 0644, Size: int64(len(e.body)), Typeflag: typ}
		if typ == tar.TypeSymlink {
			hdr.Linkname, hdr.Size = e.body, 0
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if typ == tar.TypeReg {
			_, _ = tw.Write([]byte(e.body))
		}
	}
	tw.Close()
	gz.Close()
	return buf.Bytes()
}

func releaseEntries(ver, min string) []entry {
	top := "cubepath-skills-v" + ver + "/"
	m, _ := json.Marshal(Manifest{Version: ver, MinCubecliVersion: min, Skills: []string{"cubepath-cli", "cubepath-vps"}})
	return []entry{
		{name: top + "manifest.json", body: string(m)},
		{name: top + "skills/cubepath-cli/SKILL.md", body: "---\nname: cubepath-cli\n---\ncli " + ver},
		{name: top + "skills/cubepath-cli/reference/vps.md", body: "ref"},
		{name: top + "skills/cubepath-vps/SKILL.md", body: "---\nname: cubepath-vps\n---\nvps " + ver},
	}
}

// serve publishes one release per tag; the last one is "latest".
func serve(t *testing.T, releases map[string][]byte, latest string, corrupt bool) {
	t.Helper()
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	for tag, archive := range releases {
		tag, archive := tag, archive
		name := "cubepath-skills-" + tag + ".tar.gz"
		sum := sha256.Sum256(archive)
		sums := hex.EncodeToString(sum[:]) + "  " + name + "\n"
		if corrupt {
			sums = strings.Repeat("0", 64) + "  " + name + "\n"
		}
		rel := map[string]interface{}{"tag_name": tag, "assets": []map[string]string{
			{"name": name, "browser_download_url": srv.URL + "/dl/" + name},
			{"name": "SHA256SUMS", "browser_download_url": srv.URL + "/dl/" + tag + "/SHA256SUMS"},
		}}
		body, _ := json.Marshal(rel)
		mux.HandleFunc("/releases/tags/"+tag, func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(body) })
		if tag == latest {
			mux.HandleFunc("/releases/latest", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(body) })
		}
		mux.HandleFunc("/dl/"+name, func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(archive) })
		mux.HandleFunc("/dl/"+tag+"/SHA256SUMS", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(sums)) })
	}
	old := releasesAPI
	releasesAPI = srv.URL + "/releases"
	t.Cleanup(func() { releasesAPI = old })
}

func actions(out []Outcome) string {
	var s []string
	for _, o := range out {
		s = append(s, o.Name+"="+o.Action)
	}
	return strings.Join(s, ",")
}

func TestInstallUpdateUninstall(t *testing.T) {
	serve(t, map[string][]byte{
		"v0.1.0": tarball(t, releaseEntries("0.1.0", "")),
		"v0.2.0": tarball(t, releaseEntries("0.2.0", "")),
	}, "v0.2.0", false)
	dir := t.TempDir()

	b, err := Download(context.Background(), "v0.1.0")
	if err != nil {
		t.Fatal(err)
	}
	out, err := Install(b, dir, false)
	b.Cleanup()
	if err != nil || actions(out) != "cubepath-cli=installed,cubepath-vps=installed" {
		t.Fatalf("install: %v %v", actions(out), err)
	}
	if data, _ := os.ReadFile(filepath.Join(dir, "cubepath-cli", "reference", "vps.md")); string(data) != "ref" {
		t.Fatal("files not copied")
	}

	// A user edits one skill: the update must leave it alone.
	_ = os.WriteFile(filepath.Join(dir, "cubepath-vps", "SKILL.md"), []byte("my notes"), 0644)
	b, err = Download(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	defer b.Cleanup()
	out, _ = Install(b, dir, false)
	if actions(out) != "cubepath-cli=updated,cubepath-vps=skipped" {
		t.Fatalf("update: %v", actions(out))
	}
	list, _ := List(dir)
	if len(list) != 2 || list[0].Version != "0.2.0" || !list[1].Modified {
		t.Fatalf("list: %+v", list)
	}

	// Leftover staging/backup directories must not remain.
	entries, _ := os.ReadDir(dir)
	if len(entries) != 2 {
		t.Fatalf("unexpected entries: %v", entries)
	}

	out, _ = Uninstall(dir, false)
	if actions(out) != "cubepath-cli=removed,cubepath-vps=skipped" {
		t.Fatalf("uninstall: %v", actions(out))
	}
	out, _ = Uninstall(dir, true)
	if actions(out) != "cubepath-vps=removed" {
		t.Fatalf("forced uninstall: %v", actions(out))
	}
}

func TestNeverTouchesUnmanagedSkill(t *testing.T) {
	serve(t, map[string][]byte{"v0.1.0": tarball(t, releaseEntries("0.1.0", ""))}, "v0.1.0", false)
	dir := t.TempDir()
	mine := filepath.Join(dir, "cubepath-cli")
	_ = os.MkdirAll(mine, 0755)
	_ = os.WriteFile(filepath.Join(mine, "SKILL.md"), []byte("mine"), 0644)

	b, err := Download(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	defer b.Cleanup()
	out, _ := Install(b, dir, false)
	if actions(out) != "cubepath-cli=skipped,cubepath-vps=installed" {
		t.Fatalf("got %v", actions(out))
	}
	if data, _ := os.ReadFile(filepath.Join(mine, "SKILL.md")); string(data) != "mine" {
		t.Fatal("user's skill was overwritten")
	}
	if out, _ := Uninstall(dir, true); actions(out) != "cubepath-vps=removed" {
		t.Fatalf("uninstall touched an unmanaged skill: %v", actions(out))
	}
	if out, _ := Install(b, dir, true); !strings.Contains(actions(out), "cubepath-cli=installed") {
		t.Fatalf("--force should replace it: %v", actions(out))
	}
}

func TestChecksumMismatchRefused(t *testing.T) {
	serve(t, map[string][]byte{"v0.1.0": tarball(t, releaseEntries("0.1.0", ""))}, "v0.1.0", true)
	if _, err := Download(context.Background(), ""); err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("expected checksum error, got %v", err)
	}
}

func TestUnsafeArchivesRefused(t *testing.T) {
	cases := map[string][]entry{
		"traversal": append(releaseEntries("0.1.0", ""), entry{name: "cubepath-skills-v0.1.0/../../evil", body: "x"}),
		"absolute":  append(releaseEntries("0.1.0", ""), entry{name: "/tmp/evil", body: "x"}),
		"symlink":   append(releaseEntries("0.1.0", ""), entry{name: "cubepath-skills-v0.1.0/skills/cubepath-cli/link", body: "/etc/passwd", typ: tar.TypeSymlink}),
		"two roots": append(releaseEntries("0.1.0", ""), entry{name: "other/file", body: "x"}),
	}
	for name, entries := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := extract(tarball(t, entries), t.TempDir()); err == nil {
				t.Fatal("expected the archive to be refused")
			}
		})
	}
}

func TestMinCubecliVersion(t *testing.T) {
	serve(t, map[string][]byte{"v0.1.0": tarball(t, releaseEntries("0.1.0", "1.6.0"))}, "v0.1.0", false)
	old := version.Version
	t.Cleanup(func() { version.Version = old })

	version.Version = "1.5.0"
	if _, err := Download(context.Background(), ""); err == nil || !strings.Contains(err.Error(), "cubecli update") {
		t.Fatalf("expected an upgrade hint, got %v", err)
	}
	version.Version = "v1.6.0"
	b, err := Download(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	b.Cleanup()
}

func TestInvalidSkillNameInManifest(t *testing.T) {
	e := releaseEntries("0.1.0", "")
	m, _ := json.Marshal(Manifest{Version: "0.1.0", Skills: []string{"../escape"}})
	e[0].body = string(m)
	serve(t, map[string][]byte{"v0.1.0": tarball(t, e)}, "v0.1.0", false)
	if _, err := Download(context.Background(), ""); err == nil {
		t.Fatal("expected invalid name error")
	}
}
