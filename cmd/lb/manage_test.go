package lb

import (
	"net/http"
	"testing"

	"github.com/CubePathInc/cubecli/internal/cmdtest"
)

func TestParseBatchTarget(t *testing.T) {
	got, err := parseBatchTarget("vps:101:8080:50")
	if err != nil || got["target_type"] != "vps" || got["target_uuid"] != "101" || got["port"] != 8080 || got["weight"] != 50 {
		t.Fatalf("got %v %v", got, err)
	}
	got, err = parseBatchTarget("availability-group:3f1c")
	if err != nil || got["target_type"] != "availability_group" || got["port"] != nil {
		t.Fatalf("got %v %v", got, err)
	}
	for _, bad := range []string{"vps", "vps:", "vps:1:x", "vps:1:80:w", "a:b:c:d:e"} {
		if _, err := parseBatchTarget(bad); err == nil {
			t.Fatalf("%q: want error", bad)
		}
	}
}

func TestAddBatch(t *testing.T) {
	_, reqs, err := cmdtest.Run(t, NewCmd(), func(m, p string) (int, string) {
		return 201, `{"detail":"Successfully added 2 targets","targets":[]}`
	}, "target", "add-batch", "lb1", "l1", "--target", "vps:101", "--target", "vps:102:8080")
	if err != nil {
		t.Fatal(err)
	}
	r := cmdtest.Last(reqs)
	targets, _ := r.Obj()["targets"].([]interface{})
	if r.Method != http.MethodPost || r.Path != "/loadbalancer/lb1/listeners/l1/targets/batch" || len(targets) != 2 {
		t.Fatalf("got %s %s %s", r.Method, r.Path, r.Raw)
	}
}

func TestProtectionAndMove(t *testing.T) {
	_, reqs, _ := cmdtest.Run(t, NewCmd(), nil, "protection", "lb1", "--enable")
	if r := cmdtest.Last(reqs); r.Path != "/loadbalancer/lb1/protection" || r.Obj()["enabled"] != true {
		t.Fatalf("got %+v", r)
	}
	_, reqs, _ = cmdtest.Run(t, NewCmd(), nil, "move-project", "lb1", "-p", "5")
	if r := cmdtest.Last(reqs); r.Path != "/loadbalancer/lb1/move-project" || r.Obj()["project_id"] != 5.0 {
		t.Fatalf("got %+v", r)
	}
}
