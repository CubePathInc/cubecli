package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/CubePathInc/cubecli/internal/api"
)

func meServer(t *testing.T, status int, body string) *api.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/account/me" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return api.NewClient(srv.URL, "tok")
}

func TestAccountPlanFree(t *testing.T) {
	raw, ok := accountPlan(meServer(t, 200, `{"email":"a@b.c","plan":null}`))
	if !ok || string(raw) != "null" {
		t.Fatalf("got %q %v", raw, ok)
	}
	if got := planSummary(&raw); got != "Free" {
		t.Fatalf("summary %q", got)
	}
}

func TestAccountPlanPaid(t *testing.T) {
	raw, ok := accountPlan(meServer(t, 200, `{"plan":{"code":"pro","name":"Pro","status":"active","renews_at":"2026-11-01T00:00:00","cancel_at_period_end":true}}`))
	if !ok {
		t.Fatal("not ok")
	}
	if got := planSummary(&raw); got != "Pro, ends 2026-11-01" {
		t.Fatalf("summary %q", got)
	}
}

func TestAccountPlanMissingOrError(t *testing.T) {
	if _, ok := accountPlan(meServer(t, 200, `{"email":"a@b.c"}`)); ok {
		t.Fatal("an API without the plan block is not Free")
	}
	if _, ok := accountPlan(meServer(t, 401, `{"detail":"Unauthorized"}`)); ok {
		t.Fatal("error must not be ok")
	}
	if got := planSummary(nil); got != "" {
		t.Fatalf("summary %q", got)
	}
}
