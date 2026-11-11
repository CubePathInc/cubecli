package alert

import (
	"net/http"
	"testing"

	"github.com/CubePathInc/cubecli/internal/cmdtest"
)

const alertJSON = `{"id":"a1","project_id":12,"name":"cpu","target_type":"vps","target_id":"345","metric_type":"cpu","operator":"gt","threshold":90,"duration_seconds":300,"cooldown_seconds":600,"status":"enabled","actions":[{"id":"x","action_type":"notify","notificator_id":"n1","enabled":true}]}`

func TestCreateBody(t *testing.T) {
	_, reqs, err := cmdtest.Run(t, NewCmd(), func(m, p string) (int, string) { return 201, alertJSON },
		"create", "--project", "12", "--name", "cpu", "--target-type", "availability-group", "--target", "ag-1",
		"--metric", "CPU", "--operator", ">=", "--threshold", "90", "--notify", "n1", "--notify", "n2")
	if err != nil {
		t.Fatal(err)
	}
	r := cmdtest.Last(reqs)
	if r.Method != http.MethodPost || r.Path != "/triggers/" {
		t.Fatalf("got %s %s", r.Method, r.Path)
	}
	b := r.Obj()
	if b["target_type"] != "availability_group" || b["operator"] != "gte" || b["metric_type"] != "cpu" || b["threshold"] != 90.0 {
		t.Fatalf("body %v", b)
	}
	if _, ok := b["duration_seconds"]; ok {
		t.Fatal("duration sent without --duration")
	}
	actions, _ := b["actions"].([]interface{})
	if len(actions) != 2 {
		t.Fatalf("actions %v", b["actions"])
	}
	second, _ := actions[1].(map[string]interface{})
	if second["action_type"] != "notify" || second["notificator_id"] != "n2" || second["order"] != 1.0 {
		t.Fatalf("action %v", second)
	}
}

func TestCreateRejectsUnknownOperator(t *testing.T) {
	_, reqs, err := cmdtest.Run(t, NewCmd(), nil, "create", "--project", "1", "--name", "x", "--target", "1",
		"--metric", "cpu", "--operator", "~", "--threshold", "1", "--notify", "n1")
	if err == nil || len(reqs) != 0 {
		t.Fatalf("got %v %v", err, reqs)
	}
}

func TestUpdateOnlySendsChanged(t *testing.T) {
	_, reqs, err := cmdtest.Run(t, NewCmd(), func(m, p string) (int, string) { return 200, alertJSON },
		"update", "a1", "--status", "disabled", "--threshold", "0")
	if err != nil {
		t.Fatal(err)
	}
	r := cmdtest.Last(reqs)
	b := r.Obj()
	if r.Method != http.MethodPut || r.Path != "/triggers/a1" || len(b) != 2 || b["status"] != "disabled" || b["threshold"] != 0.0 {
		t.Fatalf("got %s %s %v", r.Method, r.Path, b)
	}
}

func TestNotificatorCreate(t *testing.T) {
	_, reqs, err := cmdtest.Run(t, NewCmd(), func(m, p string) (int, string) {
		return 201, `{"id":"n1","name":"me","type":"email","config":{"email":"a@b.c"},"enabled":true}`
	}, "notificator", "create", "--name", "me", "--type", "email")
	if err != nil {
		t.Fatal(err)
	}
	r := cmdtest.Last(reqs)
	b := r.Obj()
	if r.Path != "/triggers/notificators/" || b["type"] != "email" || b["enabled"] != true {
		t.Fatalf("got %+v", r)
	}
	if _, ok := b["config"]; ok {
		t.Fatal("email channels must not send a config")
	}
}

func TestHistoryAndDelete(t *testing.T) {
	_, reqs, err := cmdtest.Run(t, NewCmd(), func(m, p string) (int, string) { return 200, `[]` }, "history", "a1", "--limit", "10")
	if err != nil {
		t.Fatal(err)
	}
	if p := cmdtest.Last(reqs).Path; p != "/triggers/a1/history?limit=10" {
		t.Fatalf("got %s", p)
	}
	_, reqs, err = cmdtest.Run(t, NewCmd(), func(m, p string) (int, string) { return 204, " " }, "delete", "a1", "--force")
	if err != nil {
		t.Fatal(err)
	}
	if r := cmdtest.Last(reqs); r.Method != http.MethodDelete || r.Path != "/triggers/a1" {
		t.Fatalf("got %+v", r)
	}
}

// A baremetal target is the server uuid, sent in lower case; a number is refused
// before any request, on create and on update when the type changes.
func TestBaremetalTargetIsUUID(t *testing.T) {
	const u = "3f2504e0-4f89-41d3-9a0c-0305e82c3301"
	ok := func(m, p string) (int, string) { return 201, alertJSON }
	base := []string{"create", "--project", "12", "--name", "net", "--target-type", "baremetal", "--metric", "network_in", "--threshold", "90", "--notify", "n1"}
	_, reqs, err := cmdtest.Run(t, NewCmd(), ok, append(base, "--target", "3F2504E0-4F89-41D3-9A0C-0305E82C3301")...)
	if err != nil || cmdtest.Last(reqs).Obj()["target_id"] != u {
		t.Fatalf("create: %v %v", err, cmdtest.Last(reqs).Obj())
	}
	for _, args := range [][]string{
		append(base, "--target", "12"),
		{"update", "a1", "--target-type", "baremetal", "--target", "12"},
	} {
		_, reqs, err := cmdtest.Run(t, NewCmd(), ok, args...)
		if err == nil || len(reqs) != 0 {
			t.Fatalf("%v: got %v, %d requests", args, err, len(reqs))
		}
	}
}
