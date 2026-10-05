package dns

import (
	"net/http"
	"strings"
	"testing"

	"github.com/CubePathInc/cubecli/internal/cmdtest"
)

const hcWithSummary = `{"uuid":"h1","record_uuid":"r1","name":"web","check_type":"https","interval_secs":60,"timeout_secs":5,"enabled":true,"last_status":"healthy","last_check_at":"2026-10-05T10:00:00Z",
"status_summary":{"status":"degraded","pops":[{"pop":"ams01","status":"unhealthy"},{"pop":"bcn01","status":"healthy"}],"last_change_at":"2026-10-05T09:00:00Z"},
"uptime_24h":99.5}`

const hcHistory = `{"check_uuid":"h1","time_range":"30d","retention_days":30,"clamped":true,
"history_starts_at":"2026-09-20T10:00:00Z","start":"2026-09-05T10:00:00Z","end":"2026-10-05T10:00:00Z","bucket_secs":86400,
"overall":{"status":"degraded","uptime_pct":99.82,"coverage_pct":100.0,"outage_secs":0,"last_change_at":"2026-10-05T09:00:00Z"},
"pops":[{"pop":"ams01","region":"eu-west","status":"unhealthy","uptime_pct":97.1,"last_error_kind":"http_status_mismatch","last_change_at":"2026-10-05T09:00:00Z"},
        {"pop":"bcn01","region":"eu-south","status":"unhealthy","uptime_pct":99.9,"last_error_kind":"tls","last_change_at":"2026-10-05T08:00:00Z"},
        {"pop":"hou01","region":"us-central","status":"healthy","uptime_pct":null,"last_error_kind":null,"last_change_at":null}],
"buckets":[],"pop_buckets":{},"markers":[],
"incidents":[{"pop":"ams01","started_at":"2026-10-05T09:00:00Z","resolved_at":null,"duration_secs":null,"error_kind":"http_status_mismatch","http_status":503},
             {"pop":"ams01","started_at":"2026-10-01T08:00:00Z","resolved_at":"2026-10-01T09:30:00Z","duration_secs":5400,"error_kind":"connection_failed","http_status":null},
             {"pop":"bcn01","started_at":"2026-09-30T08:00:00Z","resolved_at":"2026-09-30T08:00:45Z","duration_secs":45,"error_kind":"tls","http_status":null}]}`

func TestHealthCheckAliases(t *testing.T) {
	for _, group := range []string{"healthcheck", "hc", "health-check"} {
		for _, sub := range []string{"get", "show"} {
			out, reqs, err := cmdtest.Run(t, NewCmd(), func(m, p string) (int, string) { return 200, hcWithSummary }, group, sub, "z1", "r1")
			if err != nil {
				t.Fatalf("%s %s: %v", group, sub, err)
			}
			if r := cmdtest.Last(reqs); r.Method != http.MethodGet || r.Path != "/dns/zones/z1/records/r1/health-check" {
				t.Fatalf("%s %s: got %s %s", group, sub, r.Method, r.Path)
			}
			for _, want := range []string{"degraded (down at ams01)", "ams01 unhealthy", "99.50%", "2026-10-05T09:00:00Z"} {
				if !strings.Contains(out, want) {
					t.Fatalf("%s %s: missing %q in %s", group, sub, want, out)
				}
			}
		}
	}
}

func TestHealthCheckListSummaryAndFallback(t *testing.T) {
	legacy := `{"uuid":"h2","record_uuid":"r2","name":"old","check_type":"ping","interval_secs":30,"enabled":false,"last_status":"unknown"}`
	out, reqs, err := cmdtest.Run(t, NewCmd(), func(m, p string) (int, string) { return 200, "[" + hcWithSummary + "," + legacy + "]" }, "hc", "list", "z1")
	if err != nil {
		t.Fatal(err)
	}
	if p := cmdtest.Last(reqs).Path; p != "/dns/zones/z1/health-checks" {
		t.Fatalf("got %s", p)
	}
	for _, want := range []string{"degraded (down at ams01)", "99.50%", "unknown", "ping (record value)"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in %s", want, out)
		}
	}
}

func TestHealthCheckListJSONPassthrough(t *testing.T) {
	out, _, err := cmdtest.Run(t, NewCmd(), func(m, p string) (int, string) { return 200, "[" + hcWithSummary + "]" }, "hc", "list", "z1", "--json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"status_summary"`) || !strings.Contains(out, `"uptime_24h"`) {
		t.Fatalf("out %s", out)
	}
}

func TestHealthCheckSetAllFields(t *testing.T) {
	_, reqs, err := cmdtest.Run(t, NewCmd(), func(m, p string) (int, string) { return 200, hcWithSummary },
		"hc", "set", "z1", "r1", "--name", "db", "--type", "tcp", "--target", "db.example.com", "--port", "5432",
		"--path", "/x", "--expected-status", "204", "--interval", "30", "--timeout", "3",
		"--healthy-threshold", "4", "--unhealthy-threshold", "5", "--disabled")
	if err != nil {
		t.Fatal(err)
	}
	r := cmdtest.Last(reqs)
	if r.Method != http.MethodPut || r.Path != "/dns/zones/z1/records/r1/health-check" {
		t.Fatalf("got %s %s", r.Method, r.Path)
	}
	want := map[string]interface{}{
		"name": "db", "check_type": "tcp", "target": "db.example.com", "port": 5432.0, "path": "/x",
		"expected_status": 204.0, "interval_secs": 30.0, "timeout_secs": 3.0,
		"healthy_threshold": 4.0, "unhealthy_threshold": 5.0, "enabled": false,
	}
	b := r.Obj()
	for k, v := range want {
		if b[k] != v {
			t.Fatalf("%s: got %v want %v (body %v)", k, b[k], v, b)
		}
	}
	if len(b) != len(want) {
		t.Fatalf("unexpected fields %v", b)
	}
}

func TestHealthCheckDelete(t *testing.T) {
	_, reqs, err := cmdtest.Run(t, NewCmd(), nil, "hc", "delete", "z1", "r1", "--force")
	if err != nil {
		t.Fatal(err)
	}
	if r := cmdtest.Last(reqs); r.Method != http.MethodDelete || r.Path != "/dns/zones/z1/records/r1/health-check" {
		t.Fatalf("got %s %s", r.Method, r.Path)
	}
}

func TestHealthCheckHistory(t *testing.T) {
	out, reqs, err := cmdtest.Run(t, NewCmd(), func(m, p string) (int, string) { return 200, hcHistory },
		"healthcheck", "history", "z1", "r1", "--range", "30d", "--incidents", "2")
	if err != nil {
		t.Fatal(err)
	}
	if r := cmdtest.Last(reqs); r.Method != http.MethodGet || r.Path != "/dns/zones/z1/records/r1/health-check/history?time_range=30d" {
		t.Fatalf("got %s %s", r.Method, r.Path)
	}
	for _, want := range []string{
		"Health Check History (30d)", "degraded (down at ams01, bcn01)", "99.82%", "100.00%", "range shortened",
		"2026-09-20T10:00:00Z", "eu-west", "http_status_mismatch (HTTP 503)", "us-central",
		"Recent Incidents (2 of 3)", "ongoing", "connection_failed", "1h 30m",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in %s", want, out)
		}
	}
	if strings.Contains(out, "2026-09-30T08:00:45Z") {
		t.Fatalf("incident limit not applied: %s", out)
	}
	// Locations only: no node counts or node numbers.
	for _, unwanted := range []string{"Down", "nodes", "#1"} {
		if strings.Contains(out, unwanted) {
			t.Fatalf("unexpected %q in %s", unwanted, out)
		}
	}
}

func TestHealthCheckHistoryDefaultsAndJSON(t *testing.T) {
	out, reqs, err := cmdtest.Run(t, NewCmd(), func(m, p string) (int, string) { return 200, hcHistory }, "hc", "history", "z1", "r1", "--json")
	if err != nil {
		t.Fatal(err)
	}
	if p := cmdtest.Last(reqs).Path; p != "/dns/zones/z1/records/r1/health-check/history?time_range=24h" {
		t.Fatalf("got %s", p)
	}
	if !strings.Contains(out, `"incidents"`) {
		t.Fatalf("out %s", out)
	}
}

func TestHealthCheckHistoryRejectsBadRange(t *testing.T) {
	_, reqs, err := cmdtest.Run(t, NewCmd(), nil, "hc", "history", "z1", "r1", "--range", "1y")
	if err == nil || !strings.Contains(err.Error(), "24h, 7d, 30d, 90d") {
		t.Fatalf("got %v", err)
	}
	if len(reqs) != 0 {
		t.Fatalf("sent %d requests", len(reqs))
	}
}

func TestHealthCheckHistoryNoIncidents(t *testing.T) {
	body := `{"time_range":"24h","retention_days":7,"overall":{"status":"healthy","uptime_pct":100,"coverage_pct":100,"outage_secs":0},"pops":[],"incidents":[]}`
	out, _, err := cmdtest.Run(t, NewCmd(), func(m, p string) (int, string) { return 200, body }, "hc", "history", "z1", "r1")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "No incidents in this window") || !strings.Contains(out, "7 days") {
		t.Fatalf("out %s", out)
	}
}

func TestFormatSecs(t *testing.T) {
	cases := map[int64]string{0: "0s", 45: "45s", 125: "2m 5s", 5400: "1h 30m", 90000: "1d 1h"}
	for in, want := range cases {
		if got := formatSecs(in); got != want {
			t.Fatalf("formatSecs(%d) = %q, want %q", in, got, want)
		}
	}
}
