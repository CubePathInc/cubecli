package cdn

import (
	"net/http"
	"strings"
	"testing"

	"github.com/CubePathInc/cubecli/internal/cmdtest"
)

func TestUnderAttackOn(t *testing.T) {
	out, reqs, err := cmdtest.Run(t, NewCmd(), func(m, p string) (int, string) {
		return 200, `{"active":true,"until":"2026-11-26T12:30:00Z","source":"manual","level":3}`
	}, "under-attack", "on", "z1", "--duration", "90m", "--level", "3")
	if err != nil {
		t.Fatal(err)
	}
	r := cmdtest.Last(reqs)
	if r.Method != http.MethodPost || r.Path != "/cdn/zones/z1/under-attack" || r.Obj()["duration_minutes"] != 90.0 || r.Obj()["level"] != 3.0 || len(r.Obj()) != 2 {
		t.Fatalf("got %s %s %s", r.Method, r.Path, r.Raw)
	}
	if !strings.Contains(out, "ON") || !strings.Contains(out, "2026-11-26T12:30:00Z") {
		t.Fatalf("got %s", out)
	}
}

func TestUnderAttackOnWithoutEnd(t *testing.T) {
	out, reqs, err := cmdtest.Run(t, NewCmd(), func(m, p string) (int, string) {
		return 200, `{"active":true,"until":null,"source":"manual","level":2}`
	}, "under-attack", "on", "z1")
	if err != nil {
		t.Fatal(err)
	}
	r := cmdtest.Last(reqs)
	if _, has := r.Obj()["duration_minutes"]; has || r.Obj()["level"] != 2.0 || len(r.Obj()) != 1 {
		t.Fatalf("got %s", r.Raw)
	}
	if !strings.Contains(out, "no end") {
		t.Fatalf("got %s", out)
	}
}

func TestUnderAttackRejectsBadFlags(t *testing.T) {
	for _, args := range [][]string{
		{"under-attack", "on", "z1", "--duration", "1m"},
		{"under-attack", "on", "z1", "--duration", "25h"},
		{"under-attack", "on", "z1", "--duration", "90s"},
		{"under-attack", "on", "z1", "--level", "4"},
		{"under-attack", "on", "z1", "--exempt", "/api/*"},
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
		return 200, `{"active":false,"until":null,"source":null,"level":null}`
	}, "under-attack", "status", "z1")
	if err != nil || cmdtest.Last(reqs).Method != http.MethodGet || !strings.Contains(out, "off") {
		t.Fatalf("got %v %s", err, out)
	}
}
