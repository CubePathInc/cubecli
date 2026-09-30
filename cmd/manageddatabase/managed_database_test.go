package manageddatabase

import (
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/CubePathInc/cubecli/internal/cmdtest"
)

func TestCreateBody(t *testing.T) {
	_, reqs, err := cmdtest.Run(t, NewCmd(), func(m, p string) (int, string) {
		return 201, `{"uuid":"md-1","status":"provisioning"}`
	}, "create", "--name", "app-db", "--engine", "mysql", "--version", "8.0.39", "--plan", "plan-1", "--project", "12", "--backup-schedule", "0 3 * * *")
	if err != nil {
		t.Fatal(err)
	}
	r := cmdtest.Last(reqs)
	if r.Method != http.MethodPost || r.Path != "/managed-databases/" {
		t.Fatalf("got %s %s", r.Method, r.Path)
	}
	b := r.Obj()
	if b["project_id"] != 12.0 || b["plan_uuid"] != "plan-1" || b["engine"] != "mysql" {
		t.Fatalf("body %v", b)
	}
	if _, ok := b["replicas"]; ok {
		t.Fatal("replicas sent without --replicas")
	}
	if backup, _ := b["backup"].(map[string]interface{}); backup["schedule_cron"] != "0 3 * * *" {
		t.Fatalf("backup %v", b["backup"])
	}
}

func TestScaleNeedsExactlyOne(t *testing.T) {
	if _, reqs, err := cmdtest.Run(t, NewCmd(), nil, "scale", "md-1"); err == nil || len(reqs) != 0 {
		t.Fatalf("want error, got %v %v", err, reqs)
	}
	if _, _, err := cmdtest.Run(t, NewCmd(), nil, "scale", "md-1", "--replicas", "3", "--plan", "p"); err == nil {
		t.Fatal("want error with both flags")
	}
	_, reqs, err := cmdtest.Run(t, NewCmd(), nil, "scale", "md-1", "--replicas", "5")
	if err != nil {
		t.Fatal(err)
	}
	if r := cmdtest.Last(reqs); r.Path != "/managed-databases/md-1/scale" || r.Obj()["replicas"] != 5.0 {
		t.Fatalf("got %+v", r)
	}
}

func TestParseParams(t *testing.T) {
	got, err := parseParams([]string{"max_connections=500", "long_query_time=1.5", "slow_query_log=ON", "flag=true"})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]interface{}{"max_connections": int64(500), "long_query_time": 1.5, "slow_query_log": "ON", "flag": true}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v", got)
	}
	if _, err := parseParams([]string{"novalue"}); err == nil {
		t.Fatal("want error")
	}
}

func TestConfigSetSendsParams(t *testing.T) {
	_, reqs, err := cmdtest.Run(t, NewCmd(), nil, "config", "set", "md-1", "max_connections=500")
	if err != nil {
		t.Fatal(err)
	}
	r := cmdtest.Last(reqs)
	params, _ := r.Obj()["params"].(map[string]interface{})
	if r.Method != http.MethodPatch || r.Path != "/managed-databases/md-1/config" || params["max_connections"] != 500.0 {
		t.Fatalf("got %+v", r)
	}
}

func TestUserCreateShowsPassword(t *testing.T) {
	out, reqs, err := cmdtest.Run(t, NewCmd(), func(m, p string) (int, string) {
		return 201, `{"uuid":"u1","username":"app","password":"s3cretPassw0rd","status":"pending"}`
	}, "user", "create", "md-1", "app")
	if err != nil {
		t.Fatal(err)
	}
	r := cmdtest.Last(reqs)
	if r.Path != "/managed-databases/md-1/users" || r.Obj()["username"] != "app" {
		t.Fatalf("got %+v", r)
	}
	if _, ok := r.Obj()["password"]; ok {
		t.Fatal("password sent without --password")
	}
	if !strings.Contains(out, "s3cretPassw0rd") {
		t.Fatalf("password not printed: %s", out)
	}
}

func TestProtectionAndPlans(t *testing.T) {
	_, reqs, err := cmdtest.Run(t, NewCmd(), nil, "protection", "md-1", "--enable")
	if err != nil {
		t.Fatal(err)
	}
	if r := cmdtest.Last(reqs); r.Path != "/managed-databases/md-1/protection" || r.Obj()["enabled"] != true {
		t.Fatalf("got %+v", r)
	}
	if _, _, err := cmdtest.Run(t, NewCmd(), nil, "protection", "md-1"); err == nil {
		t.Fatal("want error without --enable/--disable")
	}
	_, reqs, err = cmdtest.Run(t, NewCmd(), func(m, p string) (int, string) { return 200, `[]` }, "plans", "--engine", "valkey")
	if err != nil {
		t.Fatal(err)
	}
	if r := cmdtest.Last(reqs); r.Path != "/managed-database-plans/?engine=valkey" {
		t.Fatalf("got %s", r.Path)
	}
}
