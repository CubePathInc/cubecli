package ddosmitigation

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/CubePathInc/cubecli/internal/cmdtest"
	"github.com/spf13/cobra"
)

const defaultProfile = `{"network":"194.26.100.205","tcp_validation_level":1,"udp_threshold_pps":1000,"tcp_threshold_pps":0,"icmp_threshold_mbps":0,"syn_flood_block_secs":60,"always_on_mitigation":1,"symmetric_routing":0}`

func TestProfileUpdateMergesCurrent(t *testing.T) {
	_, reqs, err := cmdtest.Run(t, NewCmd(), func(m, p string) (int, string) {
		if m == http.MethodGet {
			return 200, defaultProfile
		}
		return 200, ""
	}, "profile", "update", "194.26.100.205", "--udp-threshold-pps", "5000", "--country-mode", "1")
	if err != nil {
		t.Fatal(err)
	}
	if len(reqs) != 2 || reqs[1].Method != http.MethodPut || reqs[1].Path != "/ddos-mitigation/profiles/194.26.100.205" {
		t.Fatalf("got %+v", reqs)
	}
	b := reqs[1].Obj()
	if b["udp_threshold_pps"] != 5000.0 || b["country_mode"] != 1.0 || b["tcp_validation_level"] != 1.0 || b["syn_flood_block_secs"] != 60.0 {
		t.Fatalf("body %v", b)
	}
	for _, k := range []string{"tcp_threshold_pps", "icmp_threshold_mbps", "always_on_mitigation", "symmetric_routing"} {
		if _, ok := b[k]; ok {
			t.Fatalf("%s should be left to the server: %v", k, b)
		}
	}
}

func TestProfileUpdateNeedsAFlag(t *testing.T) {
	if _, reqs, err := cmdtest.Run(t, NewCmd(), nil, "profile", "update", "1.2.3.4"); err == nil || len(reqs) != 0 {
		t.Fatalf("got %v %v", err, reqs)
	}
}

func TestSubnetPathsKeepTheSlash(t *testing.T) {
	_, reqs, err := cmdtest.Run(t, NewCmd(), func(m, p string) (int, string) { return 200, `{"rules":[]}` }, "rule", "list", "194.26.100.0/24")
	if err != nil {
		t.Fatal(err)
	}
	if p := cmdtest.Last(reqs).Path; p != "/ddos-mitigation/firewall-rules/194.26.100.0/24" {
		t.Fatalf("got %s", p)
	}
}

func TestRuleCreateAndBulkDelete(t *testing.T) {
	_, reqs, err := cmdtest.Run(t, NewCmd(), nil, "rule", "create", "1.2.3.4", "--protocol", "udp", "--port", "53", "--action", "60", "--udp", "5000")
	if err != nil {
		t.Fatal(err)
	}
	b := cmdtest.Last(reqs).Obj()
	if b["protocol"] != 17.0 || b["dst_port"] != 53.0 || b["action"] != 60.0 || b["udp"] != 5000.0 || b["network"] != "1.2.3.4" {
		t.Fatalf("body %v", b)
	}
	if _, ok := b["tcp_syn"]; ok {
		t.Fatal("unset rate field sent")
	}

	_, reqs, err = cmdtest.Run(t, NewCmd(), nil, "rule", "delete-matching", "1.2.3.0/24", "--protocol", "tcp", "--port", "22", "--force")
	if err != nil {
		t.Fatal(err)
	}
	r := cmdtest.Last(reqs)
	if r.Method != http.MethodDelete || r.Path != "/ddos-mitigation/firewall-rules/bulk?dst_port=22&network=1.2.3.0%2F24&protocol=6" {
		t.Fatalf("got %s %s", r.Method, r.Path)
	}
}

func TestParseProtocol(t *testing.T) {
	for in, want := range map[string]int{"any": 0, "TCP": 6, "udp": 17, "icmp": 1, "47": 47} {
		if got, err := parseProtocol(in); err != nil || got != want {
			t.Fatalf("%s: got %d %v", in, got, err)
		}
	}
	if _, err := parseProtocol("sctp"); err == nil {
		t.Fatal("want error")
	}
}

func TestAssignmentsReplace(t *testing.T) {
	_, reqs, err := cmdtest.Run(t, NewCmd(), nil, "profile", "set-countries", "1.2.3.4", "cn", "ru")
	if err != nil {
		t.Fatal(err)
	}
	codes, _ := cmdtest.Last(reqs).Obj()["iso_codes"].([]interface{})
	if len(codes) != 2 || codes[0] != "CN" {
		t.Fatalf("got %v", codes)
	}
	_, reqs, err = cmdtest.Run(t, NewCmd(), nil, "profile", "set-asns", "1.2.3.4")
	if err != nil {
		t.Fatal(err)
	}
	if asns, ok := cmdtest.Last(reqs).Obj()["asns"].([]interface{}); !ok || len(asns) != 0 {
		t.Fatalf("clearing must send an empty list, got %v", cmdtest.Last(reqs).Obj())
	}
}

func TestPrefixListCreatePrintsUUID(t *testing.T) {
	out, reqs, err := cmdtest.Run(t, NewCmd(), func(m, p string) (int, string) {
		if m == http.MethodGet {
			return 200, `{"prefix_lists":[{"uuid":"g","name":"scanners","is_global":true},{"uuid":"mine","name":"scanners","is_global":false}]}`
		}
		return 201, `{"detail":"Prefix list created successfully"}`
	}, "prefix-list", "create", "scanners")
	if err != nil {
		t.Fatal(err)
	}
	if reqs[0].Method != http.MethodPost || reqs[0].Obj()["name"] != "scanners" {
		t.Fatalf("got %+v", reqs[0])
	}
	if !strings.Contains(out, "UUID: mine") || strings.Contains(out, "UUID: g") {
		t.Fatalf("out %s", out)
	}
}

func TestTimeWindow(t *testing.T) {
	cmd := &cobra.Command{}
	addWindowFlags(cmd)
	now := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	_ = cmd.Flags().Set("since", "15m")
	start, end, err := timeWindow(cmd, now)
	if err != nil || start != "2026-09-30T11:45:00Z" || end != "2026-09-30T12:00:00Z" {
		t.Fatalf("got %s %s %v", start, end, err)
	}
	_ = cmd.Flags().Set("start", "2026-09-30T13:00:00Z")
	if _, _, err := timeWindow(cmd, now); err == nil {
		t.Fatal("want error when start is after end")
	}
}
