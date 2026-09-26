package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const testURL = "https://mcp.example.test/mcp"

func TestTOMLAddAndRemove(t *testing.T) {
	existing := "model = \"o3\"\n\n[mcp_servers.other]\nurl = \"https://other.test/mcp\"\n"
	added := tomlAddServer([]byte(existing), testURL)
	want := existing + "\n[mcp_servers.cubepath]\nurl = \"" + testURL + "\"\ntool_timeout_sec = 130\n"
	if string(added) != want {
		t.Fatalf("add:\n%s", added)
	}
	if !tomlHasServer(added) || string(tomlAddServer(added, testURL)) != string(added) {
		t.Fatal("add must be idempotent")
	}

	withEnv := string(added) + "\n[mcp_servers.cubepath.env]\nFOO = \"1\"\n\n[profiles.fast]\nmodel = \"x\"\n"
	removed := string(tomlRemoveServer([]byte(withEnv)))
	if strings.Contains(removed, "cubepath") || !strings.Contains(removed, "[mcp_servers.other]") || !strings.Contains(removed, "[profiles.fast]") {
		t.Fatalf("remove:\n%s", removed)
	}
	if string(tomlRemoveServer([]byte(existing))) != existing {
		t.Fatal("remove must not touch a file without the server")
	}
	if got := tomlRemoveServer(tomlAddServer(nil, testURL)); len(got) != 0 {
		t.Fatalf("expected an empty file, got %q", got)
	}
}

func TestJSONSetAndDelete(t *testing.T) {
	existing := `{"theme":"dark","mcpServers":{"other":{"command":"npx","args":["x"]}}}`
	out, err := jsonSetServer([]byte(existing), "mcpServers", map[string]string{"url": testURL})
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Theme      string                     `json:"theme"`
		MCPServers map[string]json.RawMessage `json:"mcpServers"`
	}
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatal(err)
	}
	var server struct {
		URL string `json:"url"`
	}
	_ = json.Unmarshal(doc.MCPServers["cubepath"], &server)
	if doc.Theme != "dark" || len(doc.MCPServers) != 2 || server.URL != testURL {
		t.Fatalf("set: %s", out)
	}

	out, err = jsonDeleteServer(out, "mcpServers")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "cubepath") || !strings.Contains(string(out), `"other"`) || !strings.Contains(string(out), `"theme"`) {
		t.Fatalf("delete: %s", out)
	}

	if _, err := jsonSetServer([]byte("{ // comment\n}"), "mcpServers", nil); err == nil {
		t.Fatal("invalid JSON must be left untouched")
	}
	if out, err := jsonSetServer(nil, "mcpServers", map[string]string{"url": testURL}); err != nil || !strings.Contains(string(out), testURL) {
		t.Fatalf("new file: %s %v", out, err)
	}
}

func TestCursorAndCodexFiles(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	ctx := context.Background()

	cursor, _ := ClientByID("cursor")
	_ = os.MkdirAll(filepath.Join(home, ".cursor"), 0755)
	_ = os.WriteFile(filepath.Join(home, ".cursor", "mcp.json"), []byte(`{"mcpServers":{"mine":{"url":"https://x.test"}}}`), 0600)
	if err := cursor.Install(ctx, testURL); err != nil {
		t.Fatal(err)
	}
	if ok, _ := cursor.Configured(testURL); !ok {
		t.Fatal("cursor not configured")
	}
	fi, _ := os.Stat(filepath.Join(home, ".cursor", "mcp.json"))
	if runtime.GOOS != "windows" && fi.Mode().Perm() != 0600 {
		t.Fatalf("permissions changed to %v", fi.Mode().Perm())
	}
	if err := cursor.Uninstall(ctx, testURL); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(home, ".cursor", "mcp.json"))
	if !strings.Contains(string(data), `"mine"`) || strings.Contains(string(data), "cubepath") {
		t.Fatalf("cursor after uninstall: %s", data)
	}

	codex, _ := ClientByID("codex")
	if ok, _ := codex.Configured(testURL); ok {
		t.Fatal("codex should not be configured yet")
	}
	if err := codex.Install(ctx, testURL); err != nil {
		t.Fatal(err)
	}
	if ok, _ := codex.Configured(testURL); !ok {
		t.Fatal("codex not configured")
	}
	if err := codex.Uninstall(ctx, testURL); err != nil {
		t.Fatal(err)
	}
	if ok, _ := codex.Configured(testURL); ok {
		t.Fatal("codex still configured")
	}
}

// fakeBinaries puts scripts named after the agents' CLIs first in PATH. Each
// appends its arguments to calls.log.
func fakeBinaries(t *testing.T, names ...string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("shell scripts")
	}
	dir := t.TempDir()
	log := filepath.Join(dir, "calls.log")
	for _, n := range names {
		script := "#!/bin/sh\necho \"" + n + " $*\" >> " + log + "\n" +
			"if [ \"$2\" = get ]; then exit 1; fi\n"
		if err := os.WriteFile(filepath.Join(dir, n), []byte(script), 0755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir)
	return log
}

func TestCommandClients(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	log := fakeBinaries(t, "claude", "code")
	ctx := context.Background()

	var ids []string
	for _, c := range Detected() {
		ids = append(ids, c.ID)
	}
	if strings.Join(ids, ",") != "claude,vscode" {
		t.Fatalf("detected: %v", ids)
	}
	for _, id := range ids {
		c, _ := ClientByID(id)
		if ok, _ := c.Configured(testURL); ok {
			t.Fatalf("%s should not be configured", id)
		}
		if err := c.Install(ctx, testURL); err != nil {
			t.Fatalf("%s: %v", id, err)
		}
	}
	calls, _ := os.ReadFile(log)
	for _, want := range []string{
		"claude mcp add --transport http --scope user cubepath " + testURL,
		`code --add-mcp {"name":"cubepath","type":"http","url":"` + testURL + `"}`,
	} {
		if !strings.Contains(string(calls), want) {
			t.Errorf("missing call %q in:\n%s", want, calls)
		}
	}

}

func TestGeminiSettings(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("PATH", t.TempDir()) // no gemini binary: the file is edited directly
	ctx := context.Background()
	gemini, _ := ClientByID("gemini")
	settings := filepath.Join(home, ".gemini", "settings.json")
	_ = os.MkdirAll(filepath.Dir(settings), 0755)
	_ = os.WriteFile(settings, []byte(`{"security":{"auth":{"selectedType":"oauth-personal"}}}`), 0644)

	if err := gemini.Install(ctx, testURL); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(settings)
	if !strings.Contains(string(data), `"httpUrl": "`+testURL+`"`) || !strings.Contains(string(data), `"timeout": 130000`) || !strings.Contains(string(data), "oauth-personal") {
		t.Fatalf("settings: %s", data)
	}
	if err := gemini.Uninstall(ctx, testURL); err != nil {
		t.Fatal(err)
	}
	if ok, _ := gemini.Configured(testURL); ok {
		t.Fatal("still configured")
	}

	_ = os.WriteFile(settings, []byte(`{"mcpServers":{"cp":{"httpUrl":"`+testURL+`"}}}`), 0644)
	if ok, _ := gemini.Configured(testURL); !ok {
		t.Fatal("a server with the same URL under another name counts as configured")
	}
}

func TestResolveURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/.well-known/oauth-protected-resource/mcp" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"resource":"https://mcp.staging.test/mcp"}`))
	}))
	defer srv.Close()
	if got := ResolveURL(context.Background(), srv.URL); got != "https://mcp.staging.test/mcp" {
		t.Fatalf("got %s", got)
	}
	if got := ResolveURL(context.Background(), "http://127.0.0.1:1"); got != DefaultURL {
		t.Fatalf("fallback: %s", got)
	}
}
