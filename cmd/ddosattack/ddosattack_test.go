package ddosattack

import (
	"strings"
	"testing"

	"github.com/CubePathInc/cubecli/internal/cmdtest"
)

func TestList(t *testing.T) {
	out, _, err := cmdtest.Run(t, NewCmd(), func(m, p string) (int, string) {
		return 200, `[{"attack_id":4211,"ip_address":"194.26.100.205","duration":340,"packets_second_peak":1250000,"gbps_peak":9.4,"status":"ended"}]`
	}, "list")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "9.4") {
		t.Fatalf("out %s", out)
	}
}

func TestDetailsAndTraffic(t *testing.T) {
	for sub, path := range map[string]string{"details": "/ddos-attacks/attacks/4211/details", "traffic": "/ddos-attacks/attacks/4211/traffic-graph"} {
		out, reqs, err := cmdtest.Run(t, NewCmd(), func(m, p string) (int, string) { return 200, `{"data":[]}` }, sub, "4211")
		if err != nil {
			t.Fatal(err)
		}
		if p := cmdtest.Last(reqs).Path; p != path || !strings.Contains(out, `"data"`) {
			t.Fatalf("%s: got %s %s", sub, p, out)
		}
	}
	if _, _, err := cmdtest.Run(t, NewCmd(), nil, "details", "x"); err == nil {
		t.Fatal("want error for a non-numeric id")
	}
}
