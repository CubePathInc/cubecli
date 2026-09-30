package vps

import (
	"net/http"
	"strings"
	"testing"

	"github.com/CubePathInc/cubecli/internal/cmdtest"
)

func TestSSHKeyAddSendsArray(t *testing.T) {
	_, reqs, err := cmdtest.Run(t, NewCmd(), nil, "ssh-key", "add", "345", "12", "47")
	if err != nil {
		t.Fatal(err)
	}
	r := cmdtest.Last(reqs)
	ids, ok := r.Body.([]interface{})
	if r.Method != http.MethodPost || r.Path != "/vps/345/ssh-keys" || !ok || len(ids) != 2 || ids[1] != 47.0 {
		t.Fatalf("got %s %s %s", r.Method, r.Path, r.Raw)
	}
	if _, _, err := cmdtest.Run(t, NewCmd(), nil, "ssh-key", "add", "345", "abc"); err == nil {
		t.Fatal("want error for a non-numeric key id")
	}
}

func TestManageRoutes(t *testing.T) {
	cases := []struct {
		args   []string
		method string
		path   string
		body   string
	}{
		{[]string{"ssh-key", "remove", "345", "12"}, http.MethodDelete, "/vps/345/ssh-keys/12", ""},
		{[]string{"network", "attach", "345", "--network", "9"}, http.MethodPost, "/vps/345/network", `{"network_id":9}`},
		{[]string{"network", "detach", "345", "--force"}, http.MethodDelete, "/vps/345/network", ""},
		{[]string{"protection", "345", "--disable"}, http.MethodPost, "/vps/345/protection", `{"enabled":false}`},
		{[]string{"move-project", "345", "--project", "7"}, http.MethodPost, "/vps/345/move-project", `{"project_id":7}`},
		{[]string{"console", "345"}, http.MethodPost, "/vps/345/vnc-url", ""},
	}
	for _, c := range cases {
		_, reqs, err := cmdtest.Run(t, NewCmd(), nil, c.args...)
		if err != nil {
			t.Fatalf("%v: %v", c.args, err)
		}
		r := cmdtest.Last(reqs)
		if r.Method != c.method || r.Path != c.path || strings.TrimSpace(string(r.Raw)) != c.body {
			t.Fatalf("%v: got %s %s %s", c.args, r.Method, r.Path, r.Raw)
		}
	}
}
