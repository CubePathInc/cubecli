package cdn

import (
	"net/http"
	"strings"
	"testing"

	"github.com/CubePathInc/cubecli/internal/cmdtest"
)

func TestEventsQueryAndOutput(t *testing.T) {
	out, reqs, err := cmdtest.Run(t, NewCmd(), func(m, p string) (int, string) {
		return 200, `{"events":[{"timestamp":"2026-11-25T10:00:00Z","action":"blocked","reason":"blocked_ip","rule_name":"office","status":403,"client_ip":null,"country":"ES","asn":1,"host":"a.example.com","method":"GET","path":"/x","request_id":"r1"}],"next_cursor":"c2","retention_days":30,"visitor_data_visible":false}`
	}, "events", "z1", "--minutes", "1440", "--action", "blocked", "--country", "es", "--asn", "64500", "--path-prefix", "/admin")
	if err != nil {
		t.Fatal(err)
	}
	r := cmdtest.Last(reqs)
	for _, want := range []string{"minutes=1440", "action=blocked", "country=ES", "asn=64500", "path_prefix=%2Fadmin", "limit=50"} {
		if r.Method != http.MethodGet || !strings.Contains(r.Path, want) || !strings.HasPrefix(r.Path, "/cdn/zones/z1/security-events?") {
			t.Fatalf("missing %s in %s", want, r.Path)
		}
	}
	if !strings.Contains(out, "blocked_ip") || !strings.Contains(out, "--cursor c2") {
		t.Fatalf("got %s", out)
	}
}

func TestEventsAllFollowsTheCursor(t *testing.T) {
	calls := 0
	_, reqs, err := cmdtest.Run(t, NewCmd(), func(m, p string) (int, string) {
		calls++
		if strings.Contains(p, "cursor=c2") {
			return 200, `{"events":[],"next_cursor":null}`
		}
		return 200, `{"events":[],"next_cursor":"c2"}`
	}, "events", "z1", "--all")
	if err != nil || len(reqs) != 2 {
		t.Fatalf("calls %d err %v", calls, err)
	}
}

func TestEventsRejectsBadFlags(t *testing.T) {
	for _, args := range [][]string{{"events", "z1", "--action", "nope"}, {"events", "z1", "--minutes", "0"}, {"events", "z1", "--limit", "500"}} {
		if _, reqs, err := cmdtest.Run(t, NewCmd(), nil, args...); err == nil || len(reqs) != 0 {
			t.Fatalf("%v: got %v", args, err)
		}
	}
}

func TestZoneUpdateManagedWaf(t *testing.T) {
	_, reqs, err := cmdtest.Run(t, NewCmd(), nil, "zone", "update", "z1", "--waf-mode", "log", "--waf-level", "2", "--waf-exclude-rule-ids", "942100,941100", "--waf-exclude-categories", "")
	if err != nil {
		t.Fatal(err)
	}
	r := cmdtest.Last(reqs)
	waf, _ := r.Obj()["managed_waf"].(map[string]interface{})
	ids, _ := waf["exclude_rule_ids"].([]interface{})
	cats, _ := waf["exclude_categories"].([]interface{})
	if r.Method != http.MethodPatch || r.Path != "/cdn/zones/z1" || waf["mode"] != "log" || waf["level"] != 2.0 || len(ids) != 2 || cats == nil || len(cats) != 0 {
		t.Fatalf("got %s %s", r.Method, r.Raw)
	}
	// An empty value clears the list of rule ids, as the help says.
	_, reqs, err = cmdtest.Run(t, NewCmd(), nil, "zone", "update", "z1", "--waf-exclude-rule-ids", "")
	if err != nil {
		t.Fatal(err)
	}
	r = cmdtest.Last(reqs)
	waf, _ = r.Obj()["managed_waf"].(map[string]interface{})
	ids, ok := waf["exclude_rule_ids"].([]interface{})
	if !ok || len(ids) != 0 || len(waf) != 1 {
		t.Fatalf("clearing the ids: got %s", r.Raw)
	}
	for _, args := range [][]string{
		{"zone", "update", "z1", "--waf-exclude-rule-ids", "abc"},
		{"zone", "update", "z1", "--waf-mode", "block"},
		{"zone", "update", "z1", "--waf-level", "3"},
		{"zone", "update", "z1", "--waf-exclude-rule-ids", "0"},
		{"zone", "update", "z1", "--waf-exclude-rule-ids", "5,5"},
		{"zone", "update", "z1", "--waf-exclude-categories", "SQLi"},
	} {
		if _, reqs, err := cmdtest.Run(t, NewCmd(), nil, args...); err == nil || len(reqs) != 0 {
			t.Fatalf("%v: got %v", args, err)
		}
	}
}

func TestMetricsWaf(t *testing.T) {
	out, reqs, err := cmdtest.Run(t, NewCmd(), func(m, p string) (int, string) {
		return 200, `{"ruleset":"core@2026.11-1","evaluated":10,"flagged":2,"would_block":1,"skipped":0,"top_rules":[{"rule_id":"942100","category":"sqli","hits":2}],"top_paths":[],"top_countries":[]}`
	}, "metrics", "waf", "z1", "--minutes", "120")
	if err != nil {
		t.Fatal(err)
	}
	if r := cmdtest.Last(reqs); r.Path != "/cdn/zones/z1/metrics/waf?minutes=120" || !strings.Contains(out, "942100") {
		t.Fatalf("got %s %s", r.Path, out)
	}
}
