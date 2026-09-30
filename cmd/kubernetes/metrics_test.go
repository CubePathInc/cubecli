package kubernetes

import (
	"strings"
	"testing"

	"github.com/CubePathInc/cubecli/internal/cmdtest"
)

func TestMetrics(t *testing.T) {
	out, reqs, err := cmdtest.Run(t, NewCmd(), func(m, p string) (int, string) {
		return 200, `{"start":1,"end":2,"step":60,"metrics":{"nodes_ready":[[1,3],[2,2]],"pods_failed":[]}}`
	}, "metrics", "c1", "--range", "24h")
	if err != nil {
		t.Fatal(err)
	}
	if p := cmdtest.Last(reqs).Path; p != "/kubernetes/c1/metrics?time_range=24h" {
		t.Fatalf("got %s", p)
	}
	if !strings.Contains(out, "nodes_ready") || !strings.Contains(out, "pods_failed") {
		t.Fatalf("out %s", out)
	}

	_, reqs, _ = cmdtest.Run(t, NewCmd(), func(m, p string) (int, string) { return 200, `{"metrics":{}}` }, "metrics", "c1", "--node", "k8s-node-1")
	if p := cmdtest.Last(reqs).Path; p != "/kubernetes/c1/nodes/k8s-node-1/metrics?time_range=1h" {
		t.Fatalf("got %s", p)
	}

	_, reqs, _ = cmdtest.Run(t, NewCmd(), nil, "protection", "c1", "--disable")
	if r := cmdtest.Last(reqs); r.Path != "/kubernetes/c1/protection" || r.Obj()["enabled"] != false {
		t.Fatalf("got %+v", r)
	}
}
