package floatingip

import (
	"strings"
	"testing"

	"github.com/CubePathInc/cubecli/internal/cmdtest"
)

func TestAssignBaremetalNumberOrUUID(t *testing.T) {
	const u = "3f2504e0-4f89-41d3-9a0c-0305e82c3301"
	for _, c := range []struct{ typed, sent string }{{"9", "9"}, {u, u}, {strings.ToUpper(u), u}} {
		_, reqs, err := cmdtest.Run(t, NewCmd(), nil, "assign", "10.0.0.1", "--baremetal", c.typed)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := cmdtest.Last(reqs).Path, "/floating_ips/assign/baremetal/"+c.sent+"?address=10.0.0.1"; got != want {
			t.Fatalf("got %s want %s", got, want)
		}
	}
	_, reqs, err := cmdtest.Run(t, NewCmd(), nil, "assign", "10.0.0.1", "--baremetal", "nope")
	if err == nil || len(reqs) != 0 {
		t.Fatalf("got %v, %d requests", err, len(reqs))
	}
	if _, _, err := cmdtest.Run(t, NewCmd(), nil, "assign", "10.0.0.1"); err == nil {
		t.Fatal("one of --vps or --baremetal is required")
	}
	if _, _, err := cmdtest.Run(t, NewCmd(), nil, "assign", "10.0.0.1", "--vps", "3", "--baremetal", "9"); err == nil {
		t.Fatal("not both")
	}
	_, reqs, err = cmdtest.Run(t, NewCmd(), nil, "assign", "10.0.0.1", "--vps", "3")
	if err != nil || cmdtest.Last(reqs).Path != "/floating_ips/assign/vps/3?address=10.0.0.1" {
		t.Fatalf("vps path unchanged: %v", err)
	}
}
