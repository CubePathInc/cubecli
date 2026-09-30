package network

import (
	"net/http"
	"testing"

	"github.com/CubePathInc/cubecli/internal/cmdtest"
)

func TestBGPPeers(t *testing.T) {
	_, reqs, err := cmdtest.Run(t, NewCmd(), func(m, p string) (int, string) { return 201, `{"peer_id":"p1"}` },
		"bgp-peer", "create", "42", "--type", "vps", "--target", "123", "--remote-asn", "4200000000")
	if err != nil {
		t.Fatal(err)
	}
	r := cmdtest.Last(reqs)
	b := r.Obj()
	if r.Path != "/networks/42/bgp-peers" || b["peer_target"] != "123" || b["remote_asn"] != 4200000000.0 {
		t.Fatalf("got %s %v", r.Path, b)
	}
	if _, ok := b["max_prefix"]; ok {
		t.Fatal("max_prefix sent without --max-prefix")
	}

	_, reqs, err = cmdtest.Run(t, NewCmd(), nil, "bgp-peer", "update", "42", "p1", "--enabled=false")
	if err != nil {
		t.Fatal(err)
	}
	r = cmdtest.Last(reqs)
	if r.Method != http.MethodPatch || r.Path != "/networks/42/bgp-peers/p1" || len(r.Obj()) != 1 || r.Obj()["enabled"] != false {
		t.Fatalf("got %s %s %s", r.Method, r.Path, r.Raw)
	}

	_, reqs, _ = cmdtest.Run(t, NewCmd(), nil, "move-project", "42", "--project", "3")
	if r := cmdtest.Last(reqs); r.Path != "/networks/42/move-project" {
		t.Fatalf("got %s", r.Path)
	}
}
