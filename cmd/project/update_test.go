package project

import (
	"net/http"
	"testing"

	"github.com/CubePathInc/cubecli/internal/cmdtest"
)

func TestUpdate(t *testing.T) {
	_, reqs, err := cmdtest.Run(t, NewCmd(), nil, "update", "12", "--name", "prod")
	if err != nil {
		t.Fatal(err)
	}
	if r := cmdtest.Last(reqs); r.Method != http.MethodPut || r.Path != "/projects/12" || r.Obj()["name"] != "prod" {
		t.Fatalf("got %s %s %s", r.Method, r.Path, r.Raw)
	}
}
