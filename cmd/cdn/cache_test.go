package cdn

import (
	"net/http"
	"strings"
	"testing"

	"github.com/CubePathInc/cubecli/internal/cmdtest"
)

func TestPurge(t *testing.T) {
	_, reqs, err := cmdtest.Run(t, NewCmd(), func(m, p string) (int, string) {
		return 202, `{"detail":"Cache purge queued.","purge_uuid":"p1","status":"pending"}`
	}, "cache", "purge", "z1", "/a.css", "/img/*")
	if err != nil {
		t.Fatal(err)
	}
	r := cmdtest.Last(reqs)
	paths, _ := r.Obj()["paths"].([]interface{})
	if r.Method != http.MethodPost || r.Path != "/cdn/zones/z1/purge-cache" || len(paths) != 2 {
		t.Fatalf("got %s %s %s", r.Method, r.Path, r.Raw)
	}
	_, reqs, _ = cmdtest.Run(t, NewCmd(), func(m, p string) (int, string) { return 202, `{"purge_uuid":"p2"}` }, "cache", "purge", "z1", "--everything")
	if strings.TrimSpace(string(cmdtest.Last(reqs).Raw)) != `{"everything":true}` {
		t.Fatalf("got %s", cmdtest.Last(reqs).Raw)
	}
	for _, args := range [][]string{{"cache", "purge", "z1"}, {"cache", "purge", "z1", "/a", "--everything"}} {
		if _, reqs, err := cmdtest.Run(t, NewCmd(), nil, args...); err == nil || len(reqs) != 0 {
			t.Fatalf("%v: got %v", args, err)
		}
	}
}

func TestTokenAuth(t *testing.T) {
	out, reqs, err := cmdtest.Run(t, NewCmd(), func(m, p string) (int, string) {
		return 200, `{"detail":"Token Auth enabled.","token_auth_secret":"sekret"}`
	}, "token-auth", "enable", "z1", "--ip-binding")
	if err != nil {
		t.Fatal(err)
	}
	r := cmdtest.Last(reqs)
	if r.Method != http.MethodPatch || r.Obj()["token_auth_enabled"] != true || r.Obj()["token_auth_ip_binding"] != true || !strings.Contains(out, "sekret") {
		t.Fatalf("got %s %s %s", r.Method, r.Raw, out)
	}

	out, reqs, err = cmdtest.Run(t, NewCmd(), func(m, p string) (int, string) {
		return 200, `{"signed_url":"https://cdn.example.com/v.mp4?token=t&expires=1"}`
	}, "token-auth", "sign-url", "z1", "/v.mp4", "--expires-in", "60")
	if err != nil {
		t.Fatal(err)
	}
	r = cmdtest.Last(reqs)
	if r.Path != "/cdn/zones/z1/token-auth/sign-url" || r.Obj()["expires_in"] != 60.0 || !strings.Contains(out, "token=t") {
		t.Fatalf("got %s %s %s", r.Path, r.Raw, out)
	}
}
