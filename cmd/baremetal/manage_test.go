package baremetal

import (
	"net/http"
	"strings"
	"testing"

	"github.com/CubePathInc/cubecli/internal/cmdtest"
)

func TestManageRoutes(t *testing.T) {
	cases := []struct {
		args   []string
		method string
		path   string
		body   string
	}{
		{[]string{"ssh-key", "add", testUUID, "12"}, http.MethodPost, "/baremetal/" + testUUID + "/ssh-keys", `[12]`},
		{[]string{"ssh-key", "remove", testUUID, "12"}, http.MethodDelete, "/baremetal/" + testUUID + "/ssh-keys/12", ""},
		{[]string{"network", "attach", testUUID, "--network", "4"}, http.MethodPost, "/baremetal/" + testUUID + "/network", `{"network_id":4}`},
		{[]string{"network", "detach", testUUID, "--force"}, http.MethodDelete, "/baremetal/" + testUUID + "/network", ""},
		{[]string{"protection", testUUID, "--enable"}, http.MethodPost, "/baremetal/" + testUUID + "/protection", `{"enabled":true}`},
		{[]string{"move-project", testUUID, "--project", "7"}, http.MethodPost, "/baremetal/" + testUUID + "/move-project", `{"project_id":7}`},
		{[]string{"kvm", testUUID}, http.MethodGet, "/baremetal/" + testUUID + "/kvm", ""},
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

func TestOSAndModels(t *testing.T) {
	out, reqs, err := cmdtest.Run(t, NewCmd(), func(m, p string) (int, string) {
		return 200, `[{"id":1,"os_name":"ubuntu-24","operating_system":"Ubuntu 24.04","disk_layouts":[{"disk_layout_name":"raid1","raid_type":"RAID1","disk_type":"NVME","disk_count":2}]}]`
	}, "os", testUUID)
	if err != nil {
		t.Fatal(err)
	}
	if cmdtest.Last(reqs).Path != "/baremetal/os/"+testUUID || !strings.Contains(out, "raid1") {
		t.Fatalf("got %s %s", cmdtest.Last(reqs).Path, out)
	}

	out, reqs, err = cmdtest.Run(t, NewCmd(), func(m, p string) (int, string) {
		return 200, `{"locations":[{"location_name":"eu-bcn-1","models":[{"model_name":"c1.metal.lite","price":239.0,"port":1,"stock_available":4}]}]}`
	}, "model", "list")
	if err != nil {
		t.Fatal(err)
	}
	if cmdtest.Last(reqs).Path != "/baremetal/models" || !strings.Contains(out, "$239.00") || !strings.Contains(out, "1 Gbps") {
		t.Fatalf("got %s %s", cmdtest.Last(reqs).Path, out)
	}
}
