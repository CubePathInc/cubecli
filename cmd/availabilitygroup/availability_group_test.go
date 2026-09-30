package availabilitygroup

import (
	"testing"

	"github.com/CubePathInc/cubecli/internal/cmdtest"
)

func TestMoveProject(t *testing.T) {
	_, reqs, err := cmdtest.Run(t, NewCmd(), nil, "move-project", "ag-1", "--project", "7")
	if err != nil {
		t.Fatal(err)
	}
	if r := cmdtest.Last(reqs); r.Path != "/vps/availability-groups/ag-1/move-project" || r.Obj()["project_id"] != 7.0 {
		t.Fatalf("got %s %s", r.Path, r.Raw)
	}
}
