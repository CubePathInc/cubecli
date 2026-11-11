package baremetal

import (
	"net/http"
	"strings"
	"testing"

	"github.com/CubePathInc/cubecli/internal/cmdtest"
)

const (
	testUUID      = "3f2504e0-4f89-41d3-9a0c-0305e82c3301"
	deployingUUID = "9b1deb4d-3b7d-4bad-9bdd-2b0d7b3dcb6d"
)

// Every route of a server takes the uuid, sent in lower case, and never reaches
// the API with anything else.
func TestRoutesTakeUUID(t *testing.T) {
	cases := []struct {
		args   []string
		method string
		path   string // %s is the reference
	}{
		{[]string{"rescue", "%s"}, http.MethodPost, "/baremetal/%s/rescue"},
		{[]string{"reset-bmc", "%s", "--force"}, http.MethodPost, "/baremetal/%s/reset-bmc"},
		{[]string{"update", "%s", "--hostname", "h"}, http.MethodPatch, "/baremetal/update/%s"},
		{[]string{"ipmi", "%s"}, http.MethodPost, "/ipmi-proxy/create-session/%s"},
		{[]string{"power", "start", "%s"}, http.MethodPost, "/baremetal/%s/power/start_metal"},
		{[]string{"power", "stop", "%s"}, http.MethodPost, "/baremetal/%s/power/stop_metal"},
		{[]string{"power", "restart", "%s"}, http.MethodPost, "/baremetal/%s/power/restart_metal"},
		{[]string{"monitoring", "enable", "%s"}, http.MethodPut, "/baremetal/%s/monitoring?enable=true"},
		{[]string{"monitoring", "disable", "%s"}, http.MethodPut, "/baremetal/%s/monitoring?enable=false"},
		{[]string{"reinstall", "start", "%s", "--os", "u", "--hostname", "h", "--password", "p", "--force"}, http.MethodPost, "/baremetal/%s/reinstall"},
		{[]string{"reinstall", "cancel", "%s", "--force"}, http.MethodDelete, "/baremetal/%s/reinstall"},
		{[]string{"ssh-key", "add", "%s", "12"}, http.MethodPost, "/baremetal/%s/ssh-keys"},
		{[]string{"ssh-key", "remove", "%s", "12"}, http.MethodDelete, "/baremetal/%s/ssh-keys/12"},
		{[]string{"network", "attach", "%s", "--network", "4"}, http.MethodPost, "/baremetal/%s/network"},
		{[]string{"network", "detach", "%s", "--force"}, http.MethodDelete, "/baremetal/%s/network"},
		{[]string{"protection", "%s", "--enable"}, http.MethodPost, "/baremetal/%s/protection"},
		{[]string{"move-project", "%s", "--project", "7"}, http.MethodPost, "/baremetal/%s/move-project"},
		{[]string{"kvm", "%s"}, http.MethodGet, "/baremetal/%s/kvm"},
		{[]string{"os", "%s"}, http.MethodGet, "/baremetal/os/%s"},
	}
	for _, typed := range []string{testUUID, strings.ToUpper(testUUID)} {
		for _, c := range cases {
			args := make([]string, len(c.args)+1)
			args[0] = "--json"
			for i, a := range c.args {
				args[i+1] = strings.ReplaceAll(a, "%s", typed)
			}
			_, reqs, err := cmdtest.Run(t, NewCmd(), func(m, p string) (int, string) { return 200, `[]` }, args...)
			if err != nil {
				t.Fatalf("%v: %v", args, err)
			}
			r := cmdtest.Last(reqs)
			if want := strings.ReplaceAll(c.path, "%s", testUUID); r.Method != c.method || r.Path != want {
				t.Fatalf("%v: got %s %s, want %s %s", args, r.Method, r.Path, c.method, want)
			}
		}
	}
}

// A server number, or anything that is not a uuid, is refused before any request.
func TestInvalidReferenceSendsNothing(t *testing.T) {
	for _, bad := range []string{"9", "009", "abc", "x-1", "9/kvm", "3f2504e04f8941d39a0c0305e82c3301", "3f2504e0-4f89-41d3-9a0c-0305e82c33zz"} {
		for _, args := range [][]string{
			{"show", bad}, {"sensors", bad}, {"rescue", bad}, {"power", "start", bad},
			{"monitoring", "status", bad}, {"reinstall", "status", bad}, {"protection", bad, "--enable"}, {"kvm", bad},
			{"ssh-key", "add", bad, "12"}, {"update", bad, "--hostname", "h"},
		} {
			_, reqs, err := cmdtest.Run(t, NewCmd(), nil, args...)
			if err == nil || !strings.Contains(err.Error(), "baremetal reference must be a uuid") {
				t.Fatalf("%v: got %v", args, err)
			}
			if len(reqs) != 0 {
				t.Fatalf("%v: sent %d requests", args, len(reqs))
			}
		}
	}
}

// /projects/ after the change: id is the uuid, and uuid carries the same value.
const projectsBody = `[{"project":{"id":1,"name":"p"},"baremetals":[{"id":"aaaaaaaa-0000-4000-8000-000000000000","uuid":"aaaaaaaa-0000-4000-8000-000000000000","hostname":"a","status":"active"},{"id":"` + testUUID + `","uuid":"` + testUUID + `","hostname":"new","status":"deploying","monitoring_enable":true}]}]`

func projects(body string) cmdtest.Responder {
	return func(m, p string) (int, string) { return 200, body }
}

func TestShowFindsByUUID(t *testing.T) {
	for _, ref := range []string{testUUID, strings.ToUpper(testUUID)} {
		out, _, err := cmdtest.Run(t, NewCmd(), projects(projectsBody), "--json", "show", ref)
		if err != nil || !strings.Contains(out, `"hostname": "new"`) {
			t.Fatalf("%s: %v %s", ref, err, out)
		}
		if !strings.Contains(out, `"id": "`+testUUID+`"`) || !strings.Contains(out, `"uuid": "`+testUUID+`"`) {
			t.Fatalf("id and uuid must both be the uuid: %s", out)
		}
	}
	out, _, err := cmdtest.Run(t, NewCmd(), projects(projectsBody), "show", testUUID)
	if err != nil || !strings.Contains(out, testUUID) {
		t.Fatalf("table must show the uuid: %v %s", err, out)
	}
	if _, _, err := cmdtest.Run(t, NewCmd(), projects(projectsBody), "show", "bbbbbbbb-0000-4000-8000-000000000000"); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("unknown server: got %v", err)
	}
}

func TestListShowsUUIDInIDColumn(t *testing.T) {
	out, _, err := cmdtest.Run(t, NewCmd(), projects(projectsBody), "list")
	if err != nil || !strings.Contains(out, "new") || !strings.Contains(out, testUUID) {
		t.Fatalf("%v %s", err, out)
	}
}

func TestMonitoringStatusByUUID(t *testing.T) {
	out, _, err := cmdtest.Run(t, NewCmd(), projects(projectsBody), "--json", "monitoring", "status", testUUID)
	if err != nil || !strings.Contains(out, `"id": "`+testUUID+`"`) || !strings.Contains(out, `"monitoring_enable": true`) {
		t.Fatalf("%v %s", err, out)
	}
}

func TestReinstallStatusByUUID(t *testing.T) {
	out, _, err := cmdtest.Run(t, NewCmd(), projects(projectsBody), "--json", "reinstall", "status", testUUID)
	if err != nil || !strings.Contains(out, `"is_reinstalling": true`) {
		t.Fatalf("%v %s", err, out)
	}
}

func TestSensorsSendsReferenceToGraphQL(t *testing.T) {
	_, reqs, err := cmdtest.Run(t, NewCmd(), func(m, p string) (int, string) {
		return 200, `{"data":{"baremetal":{"sensors":{"temperatures":[],"fans":[]}}}}`
	}, "sensors", strings.ToUpper(testUUID))
	if err != nil {
		t.Fatal(err)
	}
	vars, _ := cmdtest.Last(reqs).Obj()["variables"].(map[string]interface{})
	if vars["id"] != testUUID {
		t.Fatalf("variables %v", vars)
	}
}
