package cdn

import (
	"net/http"
	"strings"
	"testing"

	"github.com/CubePathInc/cubecli/internal/cmdtest"
)

func TestUnderAttackOn(t *testing.T) {
	out, reqs, err := cmdtest.Run(t, NewCmd(), func(m, p string) (int, string) {
		return 200, `{"active":true,"until":"2026-11-26T12:30:00Z","source":"manual","level":3,"exempt_paths":["/api/*","/webhook/*"]}`
	}, "under-attack", "on", "z1", "--for", "90m", "--level", "3", "--exempt", "/api/*", "--exempt", "/webhook/*")
	if err != nil {
		t.Fatal(err)
	}
	r := cmdtest.Last(reqs)
	ex, _ := r.Obj()["exempt_paths"].([]interface{})
	if r.Method != http.MethodPost || r.Path != "/cdn/zones/z1/under-attack" || r.Obj()["duration_minutes"] != 90.0 || r.Obj()["level"] != 3.0 || len(ex) != 2 || r.Obj()["confirm_whole_zone"] != false {
		t.Fatalf("got %s %s %s", r.Method, r.Path, r.Raw)
	}
	if !strings.Contains(out, "ON") || !strings.Contains(out, "/api/*") {
		t.Fatalf("got %s", out)
	}
}

func TestUnderAttackWholeZone(t *testing.T) {
	_, reqs, err := cmdtest.Run(t, NewCmd(), nil, "under-attack", "on", "z1", "--for", "30m", "--whole-zone")
	if err != nil {
		t.Fatal(err)
	}
	r := cmdtest.Last(reqs)
	if r.Obj()["confirm_whole_zone"] != true || r.Obj()["level"] != 2.0 || r.Obj()["duration_minutes"] != 30.0 {
		t.Fatalf("got %s", r.Raw)
	}
}

func TestUnderAttackRefusesWithoutExempt(t *testing.T) {
	_, reqs, err := cmdtest.Run(t, NewCmd(), nil, "under-attack", "on", "z1", "--for", "30m")
	if err == nil || !strings.Contains(err.Error(), "--whole-zone") || len(reqs) != 0 {
		t.Fatalf("got %v %d", err, len(reqs))
	}
}

func TestUnderAttackRejectsBadFlags(t *testing.T) {
	for _, args := range [][]string{
		{"under-attack", "on", "z1", "--for", "1m", "--whole-zone"},
		{"under-attack", "on", "z1", "--for", "25h", "--whole-zone"},
		{"under-attack", "on", "z1", "--for", "30m", "--level", "4", "--whole-zone"},
		{"under-attack", "on", "z1", "--for", "30m", "--exempt", "api"},
		{"under-attack", "on", "z1", "--whole-zone"},
	} {
		if _, reqs, err := cmdtest.Run(t, NewCmd(), nil, args...); err == nil || len(reqs) != 0 {
			t.Fatalf("%v: got %v", args, err)
		}
	}
}

func TestUnderAttackOffAndStatus(t *testing.T) {
	_, reqs, err := cmdtest.Run(t, NewCmd(), nil, "under-attack", "off", "z1")
	if err != nil {
		t.Fatal(err)
	}
	if r := cmdtest.Last(reqs); r.Method != http.MethodDelete || r.Path != "/cdn/zones/z1/under-attack" {
		t.Fatalf("got %s %s", r.Method, r.Path)
	}
	out, reqs, err := cmdtest.Run(t, NewCmd(), func(m, p string) (int, string) {
		return 200, `{"active":false,"until":null,"source":null,"level":null,"exempt_paths":[]}`
	}, "under-attack", "status", "z1")
	if err != nil || cmdtest.Last(reqs).Method != http.MethodGet || !strings.Contains(out, "off") {
		t.Fatalf("got %v %s", err, out)
	}
}
