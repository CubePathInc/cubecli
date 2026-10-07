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

func TestCreateImageFlags(t *testing.T) {
	base := []string{"create", "--name", "web", "--plan", "gp.small", "--project", "7", "--location", "eu-bcn-1", "--ssh", "3"}
	bad := [][]string{
		{"--template", "ubuntu-24", "--snapshot", "6f1c"},
		{},
		{"--snapshot", "6f1c", "--cloudinit", "#cloud-config"},
	}
	for _, extra := range bad {
		_, reqs, err := cmdtest.Run(t, NewCmd(), nil, append(append([]string{}, base...), extra...)...)
		if err == nil || len(reqs) != 0 {
			t.Fatalf("%v: want a local error and no request, got err=%v reqs=%d", extra, err, len(reqs))
		}
	}

	_, reqs, err := cmdtest.Run(t, NewCmd(), nil, append(append([]string{}, base...), "--snapshot", "6f1c")...)
	if err != nil {
		t.Fatal(err)
	}
	r := cmdtest.Last(reqs)
	b := r.Obj()
	if _, ok := b["template_name"]; ok || b["snapshot_id"] != "6f1c" || r.Path != "/vps/create/7" {
		t.Fatalf("snapshot deploy: got %s %s", r.Path, r.Raw)
	}

	_, reqs, err = cmdtest.Run(t, NewCmd(), nil, append(append([]string{}, base...), "--template", "ubuntu-24")...)
	if err != nil {
		t.Fatal(err)
	}
	b = cmdtest.Last(reqs).Obj()
	if _, ok := b["snapshot_id"]; ok || b["template_name"] != "ubuntu-24" {
		t.Fatalf("template deploy: got %s", cmdtest.Last(reqs).Raw)
	}
}
