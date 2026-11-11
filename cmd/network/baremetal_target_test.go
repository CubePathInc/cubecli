package network

import (
	"strings"
	"testing"

	"github.com/CubePathInc/cubecli/internal/cmdtest"
)

const bmUUID = "3f2504e0-4f89-41d3-9a0c-0305e82c3301"

// A baremetal next hop or BGP peer is named by its uuid (sent in lower case); a
// server number is refused before any request.
func TestBaremetalTargetsTakeUUID(t *testing.T) {
	ok := func(m, p string) (int, string) { return 201, `{"peer_id":"p1"}` }
	_, reqs, err := cmdtest.Run(t, NewCmd(), ok, "route", "create", "42", "-d", "10.1.0.0/24", "-t", "baremetal", "-g", strings.ToUpper(bmUUID))
	if err != nil || cmdtest.Last(reqs).Obj()["next_hop_target"] != bmUUID {
		t.Fatalf("route: %v %v", err, cmdtest.Last(reqs).Obj())
	}
	_, reqs, err = cmdtest.Run(t, NewCmd(), ok, "bgp-peer", "create", "42", "--type", "baremetal", "--target", strings.ToUpper(bmUUID), "--remote-asn", "65010")
	if err != nil || cmdtest.Last(reqs).Obj()["peer_target"] != bmUUID {
		t.Fatalf("bgp: %v %v", err, cmdtest.Last(reqs).Obj())
	}
	for _, args := range [][]string{
		{"route", "create", "42", "-d", "10.1.0.0/24", "-t", "baremetal", "-g", "12"},
		{"bgp-peer", "create", "42", "--type", "baremetal", "--target", "12", "--remote-asn", "65010"},
	} {
		_, reqs, err := cmdtest.Run(t, NewCmd(), ok, args...)
		if err == nil || !strings.Contains(err.Error(), "must be a uuid") || len(reqs) != 0 {
			t.Fatalf("%v: got %v, %d requests", args, err, len(reqs))
		}
	}
	_, reqs, err = cmdtest.Run(t, NewCmd(), ok, "route", "create", "42", "-d", "10.1.0.0/24", "-t", "vps", "-g", "12")
	if err != nil || cmdtest.Last(reqs).Obj()["next_hop_target"] != "12" {
		t.Fatalf("vps route target changed: %v %v", err, cmdtest.Last(reqs).Obj())
	}
}
