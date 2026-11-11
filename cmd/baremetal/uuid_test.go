package baremetal

import (
	"net/http"
	"strings"
	"testing"

	"github.com/CubePathInc/cubecli/internal/cmdtest"
)

const testUUID = "3f2504e0-4f89-41d3-9a0c-0305e82c3301"

// Every route of a server takes the number or the uuid, passed through as given
// (the uuid in lower case), and never reaches the API with anything else.
func TestRoutesTakeNumberOrUUID(t *testing.T) {
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
	for _, ref := range []struct{ typed, sent string }{{"9", "9"}, {testUUID, testUUID}, {strings.ToUpper(testUUID), testUUID}} {
		for _, c := range cases {
			args := make([]string, len(c.args)+1)
			args[0] = "--json"
			for i, a := range c.args {
				args[i+1] = strings.ReplaceAll(a, "%s", ref.typed)
			}
			_, reqs, err := cmdtest.Run(t, NewCmd(), func(m, p string) (int, string) { return 200, `[]` }, args...)
			if err != nil {
				t.Fatalf("%v: %v", args, err)
			}
			r := cmdtest.Last(reqs)
			if want := strings.ReplaceAll(c.path, "%s", ref.sent); r.Method != c.method || r.Path != want {
				t.Fatalf("%v: got %s %s, want %s %s", args, r.Method, r.Path, c.method, want)
			}
		}
	}
}

func TestInvalidReferenceSendsNothing(t *testing.T) {
	for _, bad := range []string{"abc", "x-1", "9/kvm", "3f2504e04f8941d39a0c0305e82c3301", "3f2504e0-4f89-41d3-9a0c-0305e82c33zz"} {
		for _, args := range [][]string{
			{"show", bad}, {"sensors", bad}, {"rescue", bad}, {"power", "start", bad},
			{"monitoring", "status", bad}, {"reinstall", "status", bad}, {"protection", bad, "--enable"}, {"kvm", bad},
		} {
			_, reqs, err := cmdtest.Run(t, NewCmd(), nil, args...)
			if err == nil || !strings.Contains(err.Error(), "invalid baremetal ID") {
				t.Fatalf("%v: got %v", args, err)
			}
			if len(reqs) != 0 {
				t.Fatalf("%v: sent %d requests", args, len(reqs))
			}
		}
	}
}

// /projects/ before the cutover (integer id, no uuid) and after it (id plus uuid).
const projectsOld = `[{"project":{"id":1,"name":"p"},"baremetals":[{"id":11,"hostname":"old","status":"active","monitoring_enable":true}]}]`
const projectsNew = `[{"project":{"id":1,"name":"p"},"baremetals":[{"id":9,"hostname":"a","status":"active"},{"id":11,"uuid":"` + testUUID + `","hostname":"new","status":"deploying","monitoring_enable":true}]}]`

func projects(body string) cmdtest.Responder {
	return func(m, p string) (int, string) { return 200, body }
}

func TestShowFindsByNumberOrUUID(t *testing.T) {
	for _, c := range []struct{ body, ref, host string }{
		{projectsOld, "11", "old"},
		{projectsNew, "11", "new"},
		{projectsNew, testUUID, "new"},
		{projectsNew, strings.ToUpper(testUUID), "new"},
	} {
		out, _, err := cmdtest.Run(t, NewCmd(), projects(c.body), "--json", "show", c.ref)
		if err != nil || !strings.Contains(out, `"hostname": "`+c.host+`"`) {
			t.Fatalf("%s %s: %v %s", c.body[:20], c.ref, err, out)
		}
		if !strings.Contains(out, `"id": 11`) {
			t.Fatalf("id must stay a number: %s", out)
		}
		if strings.Contains(c.body, testUUID) != strings.Contains(out, testUUID) {
			t.Fatalf("uuid in --json must follow the API: %s", out)
		}
	}
	// the old API has no uuid to match, and an unknown server is not found
	for _, c := range []struct{ body, ref string }{{projectsOld, testUUID}, {projectsNew, "10"}} {
		if _, _, err := cmdtest.Run(t, NewCmd(), projects(c.body), "show", c.ref); err == nil || !strings.Contains(err.Error(), "not found") {
			t.Fatalf("%s: got %v", c.ref, err)
		}
	}
}

func TestShowAcceptsStringID(t *testing.T) {
	body := `[{"project":{"id":1,"name":"p"},"baremetals":[{"id":"11","hostname":"s"}]}]`
	out, _, err := cmdtest.Run(t, NewCmd(), projects(body), "--json", "show", "11")
	if err != nil || !strings.Contains(out, `"id": 11`) {
		t.Fatalf("%v %s", err, out)
	}
	if _, _, err := cmdtest.Run(t, NewCmd(), projects(body), "list"); err != nil {
		t.Fatal(err)
	}
}

func TestListShowsStringAndUUIDServers(t *testing.T) {
	out, _, err := cmdtest.Run(t, NewCmd(), projects(projectsNew), "list")
	if err != nil || !strings.Contains(out, "new") || !strings.Contains(out, "11") {
		t.Fatalf("%v %s", err, out)
	}
}

func TestMonitoringStatusJSONHasUUIDWhenGiven(t *testing.T) {
	out, _, err := cmdtest.Run(t, NewCmd(), projects(projectsNew), "--json", "monitoring", "status", testUUID)
	if err != nil || !strings.Contains(out, `"uuid": "`+testUUID+`"`) || !strings.Contains(out, `"id": 11`) {
		t.Fatalf("%v %s", err, out)
	}
	out, _, err = cmdtest.Run(t, NewCmd(), projects(projectsOld), "--json", "monitoring", "status", "11")
	if err != nil || strings.Contains(out, "uuid") || !strings.Contains(out, `"id": 11`) {
		t.Fatalf("%v %s", err, out)
	}
}

func TestReinstallStatusByUUID(t *testing.T) {
	out, _, err := cmdtest.Run(t, NewCmd(), projects(projectsNew), "--json", "reinstall", "status", testUUID)
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
