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
	}, "create", "--name", "app-db", "--engine", "mysql", "--version", "8.4", "--plan", "plan-1", "--project", "12", "--backup-hour", "0", "--backup-retention", "14")
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
	if backup, _ := b["backup"].(map[string]interface{}); backup["hour_utc"] != 0.0 || backup["retention_days"] != 14.0 || len(backup) != 2 {
		t.Fatalf("backup %v", b["backup"])
	}
}

func TestCreateWithoutBackupFlags(t *testing.T) {
	_, reqs, err := cmdtest.Run(t, NewCmd(), func(m, p string) (int, string) {
		return 201, `{"uuid":"md-1","status":"provisioning"}`
	}, "create", "--name", "app-db", "--engine", "mysql", "--version", "8.4", "--plan", "plan-1", "--project", "12")
	if err != nil {
		t.Fatal(err)
	}
	b := cmdtest.Last(reqs).Obj()
	if _, ok := b["backup"]; ok {
		t.Fatalf("backup sent without flags: %v", b)
	}
	if _, ok := b["replicas"]; ok {
		t.Fatal("replicas sent without --replicas")
	}
}

func TestUpdateBackup(t *testing.T) {
	if _, reqs, err := cmdtest.Run(t, NewCmd(), nil, "update", "md-1"); err == nil || len(reqs) != 0 {
		t.Fatalf("want error, got %v %v", err, reqs)
	}
	for _, old := range []string{"--backup-schedule", "--backup-enabled"} {
		if _, _, err := cmdtest.Run(t, NewCmd(), nil, "update", "md-1", old, "x"); err == nil {
			t.Fatalf("%s should not exist", old)
		}
	}
	_, reqs, err := cmdtest.Run(t, NewCmd(), nil, "update", "md-1", "--backup-hour", "0")
	if err != nil {
		t.Fatal(err)
	}
	r := cmdtest.Last(reqs)
	backup, _ := r.Obj()["backup"].(map[string]interface{})
	if r.Method != http.MethodPatch || r.Path != "/managed-databases/md-1" || backup["hour_utc"] != 0.0 || len(backup) != 1 {
		t.Fatalf("got %+v", r)
	}
}

func TestShowNewResponse(t *testing.T) {
	body := `{"uuid":"md-1","project_id":12,"name":"app-db","label":null,"engine":"mysql","version":"8.4","topology":"replication","replicas":1,"status":"active",
"endpoint_host":"abcd1234.cubemdb.io","endpoint_port":3306,
"plan":{"uuid":"p","name":"mdb-1","description":null,"engine":"mysql","cpu":2,"memory_gb":4,"storage_gb":50,"max_replicas":1,"price_per_hour":0.01},
"location":{"id":1,"location_name":"bcn-1","description":null},
"backup_enabled":true,"backup_schedule_cron":"0 5 * * *","backup_hour_utc":5,"backup_retention_days":9,"billing_type":"hourly","protected":false,"updated_at":"2026-10-17T10:00:00",
"health":{"polled_at":1,"up":true,"role":"primary","version":"8.4.11","uptime_s":10,"connections":3,"max_connections":150,"threads_running":1,"data_bytes":null,"latency_ms":2,"error_kind":"","nodes_total":1,"nodes_up":1},
"metrics_available":true}`
	out, _, err := cmdtest.Run(t, NewCmd(), func(m, p string) (int, string) { return 200, body }, "show", "md-1")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"daily at 05:00 UTC, kept 9 days", "abcd1234.cubemdb.io:3306", "1/1 nodes up"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in %s", want, out)
		}
	}
	// No health yet (not measured) must not fail.
	out, _, err = cmdtest.Run(t, NewCmd(), func(m, p string) (int, string) {
		return 200, strings.Replace(body, `"health":{`, `"health_x":{`, 1)
	}, "show", "md-1")
	if err != nil || !strings.Contains(out, "not measured yet") {
		t.Fatalf("err %v out %s", err, out)
	}
}

func TestCredentialsTLSHint(t *testing.T) {
	out, _, err := cmdtest.Run(t, NewCmd(), func(m, p string) (int, string) {
		return 200, `{"host":"abcd1234.cubemdb.io","port":3306,"username":"admin","password":"pw","uri":"mysql://admin:pw@abcd1234.cubemdb.io:3306"}`
	}, "credentials", "md-1")
	if err != nil || !strings.Contains(out, "--ssl-mode=VERIFY_IDENTITY") {
		t.Fatalf("err %v out %s", err, out)
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
