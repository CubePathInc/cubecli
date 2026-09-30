package dns

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/CubePathInc/cubecli/internal/cmdtest"
)

const importResult = `{"imported":1,"skipped":0,"errors":[],"records":[{"name":"www.example.com","record_type":"A","content":"1.2.3.4","ttl":3600}]}`

func TestImportUploadsMultipart(t *testing.T) {
	f := filepath.Join(t.TempDir(), "example.com.zone")
	if err := os.WriteFile(f, []byte("www 3600 IN A 1.2.3.4\n"), 0600); err != nil {
		t.Fatal(err)
	}
	out, reqs, err := cmdtest.Run(t, NewCmd(), func(m, p string) (int, string) { return 200, importResult }, "zone", "import", "z1", f)
	if err != nil {
		t.Fatal(err)
	}
	r := cmdtest.Last(reqs)
	if r.Method != http.MethodPost || r.Path != "/dns/zones/z1/import" || !strings.HasPrefix(r.ContentType, "multipart/form-data") {
		t.Fatalf("got %s %s %s", r.Method, r.Path, r.ContentType)
	}
	if !strings.Contains(string(r.Raw), `name="file"; filename="example.com.zone"`) || !strings.Contains(string(r.Raw), "www 3600 IN A 1.2.3.4") {
		t.Fatalf("body %s", r.Raw)
	}
	if !strings.Contains(out, "www.example.com") {
		t.Fatalf("out %s", out)
	}
}

func TestCreateWithScan(t *testing.T) {
	_, reqs, err := cmdtest.Run(t, NewCmd(), func(m, p string) (int, string) { return 201, importResult }, "zone", "create", "example.com", "-p", "12", "--scan")
	if err != nil {
		t.Fatal(err)
	}
	if r := cmdtest.Last(reqs); r.Method != http.MethodPost || r.Path != "/dns/zones/scan?domain=example.com&project_id=12" {
		t.Fatalf("got %s %s", r.Method, r.Path)
	}
	if _, _, err := cmdtest.Run(t, NewCmd(), nil, "zone", "create", "example.com", "-p", "12", "--scan", "--zone-file", "x"); err == nil {
		t.Fatal("want error with both sources")
	}
}

func TestHealthCheckSet(t *testing.T) {
	_, reqs, err := cmdtest.Run(t, NewCmd(), func(m, p string) (int, string) {
		return 200, `{"uuid":"h1","record_uuid":"r1","name":"web","check_type":"https","interval_secs":60,"timeout_secs":5,"enabled":true,"last_status":"unknown"}`
	}, "health-check", "set", "z1", "r1", "--name", "web", "--type", "https", "--path", "/health")
	if err != nil {
		t.Fatal(err)
	}
	r := cmdtest.Last(reqs)
	b := r.Obj()
	if r.Method != http.MethodPut || r.Path != "/dns/zones/z1/records/r1/health-check" || b["check_type"] != "https" || b["path"] != "/health" || b["enabled"] != true {
		t.Fatalf("got %s %s %v", r.Method, r.Path, b)
	}
	if _, ok := b["port"]; ok {
		t.Fatal("port sent without --port")
	}
}

func TestMoveProjectAndRegions(t *testing.T) {
	_, reqs, _ := cmdtest.Run(t, NewCmd(), nil, "zone", "move-project", "z1", "--project", "4")
	if r := cmdtest.Last(reqs); r.Path != "/dns/zones/z1/move-project" || r.Obj()["project_id"] != 4.0 {
		t.Fatalf("got %+v", r)
	}
	_, reqs, _ = cmdtest.Run(t, NewCmd(), func(m, p string) (int, string) { return 200, `[{"code":"global","name":"Global"}]` }, "regions")
	if p := cmdtest.Last(reqs).Path; p != "/dns/regions" {
		t.Fatalf("got %s", p)
	}
}
