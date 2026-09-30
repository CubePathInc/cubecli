package firewall

import (
	"net/http"
	"testing"

	"github.com/CubePathInc/cubecli/internal/cmdtest"
)

func TestParseRule(t *testing.T) {
	r, err := parseRule("in tcp 80,443 2001:db8::/32 web and api")
	if err != nil {
		t.Fatal(err)
	}
	if r.Direction != "in" || r.Protocol != "tcp" || *r.Port != "80,443" || *r.Source != "2001:db8::/32" || *r.Comment != "web and api" {
		t.Fatalf("got %+v", r)
	}
	r, err = parseRule("IN icmp")
	if err != nil || r.Port != nil || r.Source != nil || r.Comment != nil || r.Direction != "in" {
		t.Fatalf("got %+v %v", r, err)
	}
	r, _ = parseRule("in tcp - 10.0.0.0/8")
	if r.Port != nil || *r.Source != "10.0.0.0/8" {
		t.Fatalf("got %+v", r)
	}
	for _, bad := range []string{"in", "sideways tcp 80"} {
		if _, err := parseRule(bad); err == nil {
			t.Fatalf("%q: want error", bad)
		}
	}
}

func TestCreateSendsProjectAndRules(t *testing.T) {
	_, reqs, err := cmdtest.Run(t, NewCmd(), func(m, p string) (int, string) {
		return 201, `{"id":15,"project_id":3,"name":"web","rules":[],"enabled":true,"vps_count":0}`
	}, "group", "create", "--project", "3", "--name", "web", "--rule", "in tcp 443")
	if err != nil {
		t.Fatal(err)
	}
	r := cmdtest.Last(reqs)
	if r.Method != http.MethodPost || r.Path != "/firewall/groups?project_id=3" {
		t.Fatalf("got %s %s", r.Method, r.Path)
	}
	rules, _ := r.Obj()["rules"].([]interface{})
	first, _ := rules[0].(map[string]interface{})
	if len(rules) != 1 || first["port"] != "443" || first["source"] != nil {
		t.Fatalf("rules %v", rules)
	}
}

func TestUpdateOnlyReplacesRulesWhenGiven(t *testing.T) {
	_, reqs, err := cmdtest.Run(t, NewCmd(), nil, "group", "update", "15", "--enabled=false")
	if err != nil {
		t.Fatal(err)
	}
	b := cmdtest.Last(reqs).Obj()
	if _, ok := b["rules"]; ok || b["enabled"] != false {
		t.Fatalf("body %v", b)
	}
}

func TestAssign(t *testing.T) {
	_, reqs, err := cmdtest.Run(t, NewCmd(), nil, "assign", "345", "--none")
	if err != nil {
		t.Fatal(err)
	}
	r := cmdtest.Last(reqs)
	ids, ok := r.Obj()["firewall_group_ids"].([]interface{})
	if r.Method != http.MethodPut || r.Path != "/firewall/vps/345/groups" || !ok || len(ids) != 0 {
		t.Fatalf("got %s %s %v", r.Method, r.Path, r.Obj())
	}
	if _, _, err := cmdtest.Run(t, NewCmd(), nil, "assign", "345"); err == nil {
		t.Fatal("want error without --group or --none")
	}
	_, reqs, _ = cmdtest.Run(t, NewCmd(), nil, "assign", "345", "--group", "3", "--group", "7")
	if ids, _ := cmdtest.Last(reqs).Obj()["firewall_group_ids"].([]interface{}); len(ids) != 2 || ids[1] != 7.0 {
		t.Fatalf("got %v", ids)
	}
}
