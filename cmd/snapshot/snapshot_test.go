package snapshot

import (
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/CubePathInc/cubecli/internal/cmdtest"
)

const snapshotJSON = `{
  "uuid": "6f1c-aaaa", "name": "web-01-2026-10-07", "description": null,
  "status": "available", "origin": "vps", "os_type": "linux",
  "disk_gb": 80, "estimated_gb": 80, "billable_gb": 80,
  "location": {"name": "eu-bcn-1", "description": "Barcelona"},
  "store_location": {"name": "eu-bcn-1", "description": "Barcelona"},
  "project_id": 655,
  "source_vps": {"id": 20467, "name": "glowbit", "deleted": false},
  "template": {"template_name": "ubuntu-24", "name": "Ubuntu 24", "operating_system": "ubuntu"},
  "price_gb_month": 0.03, "monthly_cost": 2.4, "hourly_cost": 0.003288,
  "deploying_count": 1,
  "created_at": "2026-10-07T10:00:00Z", "available_at": "2026-10-07T10:03:12Z"
}`

const pendingJSON = `{
  "uuid": "77aa-bbbb", "name": "db", "description": "before upgrade",
  "status": "pending", "origin": "backup", "os_type": null,
  "disk_gb": null, "estimated_gb": 40, "billable_gb": null,
  "location": {"name": "us-mia-1", "description": null},
  "store_location": {"name": "eu-bcn-1", "description": "Barcelona"},
  "project_id": 7,
  "source_vps": {"id": null, "name": "old-db", "deleted": true},
  "template": {"template_name": "debian-12", "name": "Debian 12", "operating_system": "debian"},
  "price_gb_month": 0.03, "monthly_cost": 1.2, "hourly_cost": 0.001644,
  "deploying_count": 0,
  "created_at": "2026-10-07T11:00:00Z", "available_at": null
}`

func TestParseList(t *testing.T) {
	r, err := parseList([]byte(`{"snapshots": [` + snapshotJSON + `,` + pendingJSON + `], "total": 5}`))
	if err != nil {
		t.Fatal(err)
	}
	if r.Total != 5 || len(r.Snapshots) != 2 {
		t.Fatalf("got total=%d len=%d", r.Total, len(r.Snapshots))
	}
	a, p := r.Snapshots[0], r.Snapshots[1]
	if a.UUID != "6f1c-aaaa" || *a.DiskGB != 80 || *a.BillableGB != 80 || a.MonthlyCost != 2.4 || a.DeployingCount != 1 {
		t.Fatalf("available snapshot parsed wrong: %+v", a)
	}
	if a.sizeGB() != "80 GB" || a.sourceVPS() != "glowbit (20467)" || a.osName() != "Ubuntu 24 (linux)" || *a.AvailableAt == "" {
		t.Fatalf("available snapshot rendered wrong: %q %q %q", a.sizeGB(), a.sourceVPS(), a.osName())
	}
	// Nulls before the conversion finishes and a deleted source VPS.
	if p.DiskGB != nil || p.BillableGB != nil || p.OSType != nil || p.AvailableAt != nil || p.SourceVPS.ID != nil {
		t.Fatalf("pending snapshot nulls parsed wrong: %+v", p)
	}
	if p.sizeGB() != "~40 GB" || p.sourceVPS() != "old-db (deleted)" || p.osName() != "Debian 12" || locationName(p.Location) != "us-mia-1" {
		t.Fatalf("pending snapshot rendered wrong: %q %q %q", p.sizeGB(), p.sourceVPS(), p.osName())
	}
	if a.origin() != "server" || p.origin() != "backup" || (Snapshot{}).origin() != "-" {
		t.Fatalf("origin rendered wrong: %q %q", a.origin(), p.origin())
	}
	if *p.Description != "before upgrade" || p.Location.Description != nil {
		t.Fatalf("descriptions parsed wrong: %+v", p)
	}
}

func TestParseGetWithEstimates(t *testing.T) {
	body := strings.TrimSuffix(strings.TrimSpace(snapshotJSON), "}") +
		`, "deploy_estimates": [{"location_name": "eu-bcn-1", "remote": false, "minutes": 20}, {"location_name": "us-mia-1", "remote": true, "minutes": 80}]}`
	s, err := parseSnapshot([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if len(s.DeployEstimates) != 2 || s.DeployEstimates[1] != (DeployEstimate{"us-mia-1", true, 80}) || s.DeployEstimates[0].Remote {
		t.Fatalf("estimates parsed wrong: %+v", s.DeployEstimates)
	}
	if s.HourlyCost != 0.003288 || s.PriceGBMonth != 0.03 || *s.ProjectID != 655 || s.StoreLocation.Name != "eu-bcn-1" {
		t.Fatalf("snapshot parsed wrong: %+v", s)
	}
}

func TestParseQuota(t *testing.T) {
	q, err := parseQuota([]byte(`{"count": 2, "count_max": 10, "gb": 120, "gb_max": 500, "price_gb_month": 0.03}`))
	if err != nil {
		t.Fatal(err)
	}
	if q != (Quota{Count: 2, CountMax: 10, GB: 120, GBMax: 500, PriceGBMonth: 0.03}) {
		t.Fatalf("got %+v", q)
	}
	if _, err := parseQuota([]byte(`not json`)); err == nil {
		t.Fatal("want error for invalid JSON")
	}
}

func respond(method, path string) (int, string) {
	switch {
	case method == http.MethodGet && path == "/snapshots/quota":
		return 200, `{"count": 1, "count_max": 10, "gb": 80, "gb_max": 500, "price_gb_month": 0.03}`
	case method == http.MethodGet && strings.HasPrefix(path, "/snapshots?"):
		return 200, `{"snapshots": [` + snapshotJSON + `,` + pendingJSON + `], "total": 2}`
	case method == http.MethodGet && strings.HasPrefix(path, "/snapshots/"):
		return 200, snapshotJSON
	case method == http.MethodPost && path == "/snapshots":
		return 202, `{"detail": "Snapshot requested. It will be available in a few minutes.", "snapshot": ` + pendingJSON + `}`
	}
	return 0, ""
}

func TestListQueryAndTable(t *testing.T) {
	out, reqs, err := cmdtest.Run(t, NewCmd(), respond, "list", "--project", "655", "--status", "available", "--vps", "20467")
	if err != nil {
		t.Fatal(err)
	}
	r := cmdtest.Last(reqs)
	if r.Method != http.MethodGet || r.Path != "/snapshots?limit=50&project_id=655&source_vps_id=20467&status=available" {
		t.Fatalf("got %s %s", r.Method, r.Path)
	}
	for _, want := range []string{"web-01-2026-10-07", "glowbit (20467)", "old-db (deleted)", "~40 GB", "$2.40", "eu-bcn-1"} {
		if !strings.Contains(out, want) {
			t.Fatalf("table misses %q:\n%s", want, out)
		}
	}
}

func TestListRejectsBadFlags(t *testing.T) {
	for _, args := range [][]string{
		{"list", "--status", "deleted"},
		{"list", "--limit", "0"},
		{"list", "--limit", "101"},
		{"list", "--offset", "-1"},
	} {
		_, reqs, err := cmdtest.Run(t, NewCmd(), respond, args...)
		if err == nil || len(reqs) != 0 {
			t.Fatalf("%v: want a local error and no request, got err=%v reqs=%d", args, err, len(reqs))
		}
	}
}

func TestGetAndQuota(t *testing.T) {
	out, reqs, err := cmdtest.Run(t, NewCmd(), respond, "get", "6f1c-aaaa")
	if err != nil {
		t.Fatal(err)
	}
	if r := cmdtest.Last(reqs); r.Path != "/snapshots/6f1c-aaaa" {
		t.Fatalf("got %s", r.Path)
	}
	if !strings.Contains(out, "glowbit") || !strings.Contains(out, "$2.40/month") {
		t.Fatalf("get output:\n%s", out)
	}
	if !hasLine(out, "Origin", "server") {
		t.Fatalf("get output misses the origin:\n%s", out)
	}

	out, reqs, err = cmdtest.Run(t, NewCmd(), respond, "quota")
	if err != nil {
		t.Fatal(err)
	}
	if r := cmdtest.Last(reqs); r.Path != "/snapshots/quota" {
		t.Fatalf("got %s", r.Path)
	}
	if !strings.Contains(out, "1 / 10") || !strings.Contains(out, "80 / 500 GB") {
		t.Fatalf("quota output:\n%s", out)
	}
	if strings.Contains(out, "enabled") || strings.Contains(out, "disabled") {
		t.Fatalf("quota must not show an on/off state:\n%s", out)
	}
}

// hasLine reports whether one line of out contains every part.
func hasLine(out string, parts ...string) bool {
	for _, line := range strings.Split(out, "\n") {
		ok := true
		for _, p := range parts {
			ok = ok && strings.Contains(line, p)
		}
		if ok {
			return true
		}
	}
	return false
}

func TestGetJSONPassesThrough(t *testing.T) {
	out, _, err := cmdtest.Run(t, NewCmd(), respond, "quota", "--json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"count_max": 10`) {
		t.Fatalf("json output:\n%s", out)
	}
}

func TestCreateBody(t *testing.T) {
	out, reqs, err := cmdtest.Run(t, NewCmd(), respond, "create", "--vps", "20467", "--backup", "991", "--name", "web-01", "--description", "golden")
	if err != nil {
		t.Fatal(err)
	}
	r := cmdtest.Last(reqs)
	b := r.Obj()
	if r.Method != http.MethodPost || r.Path != "/snapshots" || b["vps_id"] != 20467.0 || b["backup_id"] != 991.0 || b["name"] != "web-01" || b["description"] != "golden" {
		t.Fatalf("got %s %s %s", r.Method, r.Path, r.Raw)
	}
	if !strings.Contains(out, "77aa-bbbb") {
		t.Fatalf("create output misses the uuid:\n%s", out)
	}

	_, reqs, err = cmdtest.Run(t, NewCmd(), respond, "create", "--vps", "1", "--backup", "2", "--name", "x")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := cmdtest.Last(reqs).Obj()["description"]; ok {
		t.Fatal("description sent without --description")
	}
	if _, _, err := cmdtest.Run(t, NewCmd(), respond, "create", "--vps", "1", "--backup", "0", "--name", "x"); err == nil {
		t.Fatal("want error for --backup 0")
	}
	for _, v := range []string{"0", "-3"} {
		if _, reqs, err := cmdtest.Run(t, NewCmd(), respond, "create", "--vps", v, "--name", "x"); err == nil || len(reqs) != 0 {
			t.Fatalf("--vps %s: want a local error and no request", v)
		}
	}
	if _, _, err := cmdtest.Run(t, NewCmd(), respond, "create", "--backup", "2", "--name", "x"); err == nil {
		t.Fatal("want error without --vps")
	}
	if _, _, err := cmdtest.Run(t, NewCmd(), respond, "create", "--vps", "1", "--backup", "2", "--name", "  "); err == nil {
		t.Fatal("want error for a blank name")
	}
}

func TestCreateFromVPSNow(t *testing.T) {
	out, reqs, err := cmdtest.Run(t, NewCmd(), respond, "create", "--vps", "20467", "--name", "web-01-now")
	if err != nil {
		t.Fatal(err)
	}
	r := cmdtest.Last(reqs)
	b := r.Obj()
	if r.Method != http.MethodPost || r.Path != "/snapshots" || b["vps_id"] != 20467.0 || b["name"] != "web-01-now" {
		t.Fatalf("got %s %s %s", r.Method, r.Path, r.Raw)
	}
	if _, ok := b["backup_id"]; ok {
		t.Fatalf("backup_id sent without --backup: %s", r.Raw)
	}
	if !strings.Contains(out, "77aa-bbbb") {
		t.Fatalf("create output misses the uuid:\n%s", out)
	}
}

func TestUpdateRoutes(t *testing.T) {
	cases := []struct {
		args []string
		body string
	}{
		{[]string{"update", "u1", "--name", "new"}, `{"name":"new"}`},
		{[]string{"update", "u1", "--description", ""}, `{"description":null}`},
		{[]string{"update", "u1", "--project", "9", "--description", "d"}, `{"description":"d","project_id":9}`},
		{[]string{"rename", "u1", "renamed"}, `{"name":"renamed"}`},
		{[]string{"move-project", "u1", "--project", "7"}, `{"project_id":7}`},
		{[]string{"move", "u1", "-p", "8"}, `{"project_id":8}`},
	}
	for _, c := range cases {
		_, reqs, err := cmdtest.Run(t, NewCmd(), nil, c.args...)
		if err != nil {
			t.Fatalf("%v: %v", c.args, err)
		}
		r := cmdtest.Last(reqs)
		if r.Method != http.MethodPatch || r.Path != "/snapshots/u1" || strings.TrimSpace(string(r.Raw)) != c.body {
			t.Fatalf("%v: got %s %s %s", c.args, r.Method, r.Path, r.Raw)
		}
	}
	if _, reqs, err := cmdtest.Run(t, NewCmd(), nil, "update", "u1"); err == nil || len(reqs) != 0 {
		t.Fatal("want a local error when nothing changes")
	}
}

func TestDeleteWithForce(t *testing.T) {
	_, reqs, err := cmdtest.Run(t, NewCmd(), nil, "delete", "u1", "--force")
	if err != nil {
		t.Fatal(err)
	}
	if r := cmdtest.Last(reqs); r.Method != http.MethodDelete || r.Path != "/snapshots/u1" {
		t.Fatalf("got %s %s", r.Method, r.Path)
	}
}

// withStdin feeds input to the confirmation prompt for the duration of fn.
func withStdin(t *testing.T, input string, fn func()) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.WriteString(input); err != nil {
		t.Fatal(err)
	}
	_ = w.Close()
	old := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = old; _ = r.Close() }()
	fn()
}

func TestDeleteAsksForConfirmation(t *testing.T) {
	for _, input := range []string{"", "n\n", "no\n"} {
		withStdin(t, input, func() {
			out, reqs, err := cmdtest.Run(t, NewCmd(), nil, "delete", "u1")
			if err != nil {
				t.Fatalf("%q: %v", input, err)
			}
			if !strings.Contains(out, "delete snapshot u1") || !strings.Contains(out, "Aborted") {
				t.Fatalf("%q: output %q", input, out)
			}
			if len(reqs) != 0 {
				t.Fatalf("%q: sent %d requests, want none", input, len(reqs))
			}
		})
	}
	withStdin(t, "y\n", func() {
		_, reqs, err := cmdtest.Run(t, NewCmd(), nil, "delete", "u1")
		if err != nil {
			t.Fatal(err)
		}
		if r := cmdtest.Last(reqs); r.Method != http.MethodDelete || r.Path != "/snapshots/u1" {
			t.Fatalf("got %s %s", r.Method, r.Path)
		}
	})
}
