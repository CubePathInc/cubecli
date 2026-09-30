package sshkey

import (
	"net/http"
	"testing"

	"github.com/CubePathInc/cubecli/internal/cmdtest"
)

func TestUpdate(t *testing.T) {
	_, reqs, err := cmdtest.Run(t, NewCmd(), nil, "update", "42", "--name", "laptop")
	if err != nil {
		t.Fatal(err)
	}
	if r := cmdtest.Last(reqs); r.Method != http.MethodPut || r.Path != "/sshkey/42" || r.Obj()["name"] != "laptop" {
		t.Fatalf("got %s %s %s", r.Method, r.Path, r.Raw)
	}
}
