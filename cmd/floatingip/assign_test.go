package floatingip

import (
	"strings"
	"testing"

	"github.com/CubePathInc/cubecli/internal/cmdtest"
)

func TestAssignBaremetalUUID(t *testing.T) {
	const u = "3f2504e0-4f89-41d3-9a0c-0305e82c3301"
	for _, typed := range []string{u, strings.ToUpper(u)} {
		_, reqs, err := cmdtest.Run(t, NewCmd(), nil, "assign", "10.0.0.1", "--baremetal", typed)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := cmdtest.Last(reqs).Path, "/floating_ips/assign/baremetal/"+u+"?address=10.0.0.1"; got != want {
			t.Fatalf("got %s want %s", got, want)
		}
	}
	for _, bad := range []string{"nope", "9"} {
		_, reqs, err := cmdtest.Run(t, NewCmd(), nil, "assign", "10.0.0.1", "--baremetal", bad)
		if err == nil || !strings.Contains(err.Error(), "must be a uuid") || len(reqs) != 0 {
			t.Fatalf("%s: got %v, %d requests", bad, err, len(reqs))
		}
	}
	if _, _, err := cmdtest.Run(t, NewCmd(), nil, "assign", "10.0.0.1"); err == nil {
		t.Fatal("one of --vps or --baremetal is required")
	}
	if _, _, err := cmdtest.Run(t, NewCmd(), nil, "assign", "10.0.0.1", "--vps", "3", "--baremetal", u); err == nil {
		t.Fatal("not both")
	}
	_, reqs, err := cmdtest.Run(t, NewCmd(), nil, "assign", "10.0.0.1", "--vps", "3")
	if err != nil || cmdtest.Last(reqs).Path != "/floating_ips/assign/vps/3?address=10.0.0.1" {
		t.Fatalf("vps path unchanged: %v", err)
	}
}
