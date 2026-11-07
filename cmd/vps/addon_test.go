package vps

import (
	"net/http"
	"strings"
	"testing"

	"github.com/CubePathInc/cubecli/internal/cmdtest"
)

const addonServers = `[
  {"id": 7, "addons": []},
  {"id": 345, "addons": [
    {"uuid": "old-uuid", "slug": "vps-port-10g", "status": "cancelled"},
    {"uuid": "live-uuid", "slug": "vps-port-10g", "status": "active"}
  ]}
]`

func addonAPI(method, path string) (int, string) {
	if method == http.MethodGet && path == "/vps/" {
		return 200, addonServers
	}
	return 200, `{"detail":"10 Gbps port was removed from your server."}`
}

func TestAddonRemoveBySlugOrUUID(t *testing.T) {
	for _, ref := range []string{"vps-port-10g", "live-uuid"} {
		out, reqs, err := cmdtest.Run(t, NewCmd(), addonAPI, "addon", "remove", "345", ref, "--force")
		if err != nil {
			t.Fatalf("%s: %v", ref, err)
		}
		r := cmdtest.Last(reqs)
		if r.Method != http.MethodDelete || r.Path != "/vps/345/addons/live-uuid" {
			t.Fatalf("%s: got %s %s", ref, r.Method, r.Path)
		}
		if !strings.Contains(out, "was removed") {
			t.Fatalf("%s: output %q", ref, out)
		}
	}
}

func TestAddonRemoveSendsNothingWithoutALiveContract(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"unknown add-on", []string{"345", "other-addon"}, "has no add-on"},
		{"ended contract by uuid", []string{"345", "old-uuid"}, "has no add-on"},
		{"server without add-ons", []string{"7", "vps-port-10g"}, "has no add-on"},
		{"unknown server", []string{"999", "vps-port-10g"}, "not found"},
		{"bad id", []string{"abc", "vps-port-10g"}, "invalid VPS ID"},
	}
	for _, c := range cases {
		args := append([]string{"addon", "remove"}, c.args...)
		_, reqs, err := cmdtest.Run(t, NewCmd(), addonAPI, append(args, "--force")...)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Fatalf("%s: err %v", c.name, err)
		}
		for _, r := range reqs {
			if r.Method == http.MethodDelete {
				t.Fatalf("%s: sent a DELETE", c.name)
			}
		}
	}
}
