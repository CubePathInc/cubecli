package org

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/CubePathInc/cubecli/internal/cmdtest"
)

const activePlan = `{"subscription":{"uuid":"s1","status":"active","plan":{"code":"business","name":"Business"},
"billing_interval":"monthly","amount_usd":"199.00","current_period_start":"2026-10-01T00:00:00",
"current_period_end":"2026-11-01T00:00:00","past_due_since":null,"cancel_at_period_end":false,
"next_plan":{"code":"pro","name":"Pro"},"next_billing_interval":"yearly","channel":"invoice","source":"self_serve"},
"invoices":[{"invoice_number":"INV-2026-0042","status":"paid","amount":199,"currency":"USD","due_date":"2026-10-01",
"kind":"first_partial","period_start":"2026-10-01","period_end":"2026-11-01"}],
"credit":{"monthly_credit_usd":"25.00","this_month_granted_usd":"25.00","active_grants":[]}}`

func TestPlanCallsEndpoint(t *testing.T) {
	out, reqs, err := cmdtest.Run(t, NewCmd(), func(m, p string) (int, string) { return 200, activePlan }, "plan")
	if err != nil {
		t.Fatal(err)
	}
	r := cmdtest.Last(reqs)
	if len(reqs) != 1 || r.Method != http.MethodGet || r.Path != "/organization/plan" {
		t.Fatalf("requests %v", reqs)
	}
	for _, want := range []string{
		"Business (business)", "$199.00 per month", "2026-10-01 to 2026-11-01", "Renews", "2026-11-01",
		"to Pro (yearly) on 2026-11-01", "$25.00", "INV-2026-0042", "first_partial",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
}

func TestPlanJSONIsPassthrough(t *testing.T) {
	out, _, err := cmdtest.Run(t, NewCmd(), func(m, p string) (int, string) { return 200, activePlan }, "plan", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]interface{}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, out)
	}
	sub, _ := got["subscription"].(map[string]interface{})
	if sub["amount_usd"] != "199.00" || sub["next_billing_interval"] != "yearly" {
		t.Fatalf("subscription %v", sub)
	}
}

func TestPlanFree(t *testing.T) {
	out, _, err := cmdtest.Run(t, NewCmd(), func(m, p string) (int, string) {
		return 200, `{"subscription":null,"invoices":[],"credit":{"monthly_credit_usd":0,"this_month_granted_usd":0,"active_grants":[]}}`
	}, "plan")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Free") || !strings.Contains(out, PlanPage) {
		t.Fatalf("output:\n%s", out)
	}
}

func TestPlanCancelScheduled(t *testing.T) {
	body := strings.Replace(activePlan, `"cancel_at_period_end":false`, `"cancel_at_period_end":true`, 1)
	body = strings.Replace(body, `"next_plan":{"code":"pro","name":"Pro"},"next_billing_interval":"yearly"`, `"next_plan":null,"next_billing_interval":null`, 1)
	out, _, err := cmdtest.Run(t, NewCmd(), func(m, p string) (int, string) { return 200, body }, "plan")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "2026-11-01, does not renew") || strings.Contains(out, "Scheduled change") {
		t.Fatalf("output:\n%s", out)
	}
}

func TestPlanPastDueWarns(t *testing.T) {
	body := strings.Replace(activePlan, `"status":"active"`, `"status":"past_due"`, 1)
	body = strings.Replace(body, `"past_due_since":null`, `"past_due_since":"2026-10-05T00:00:00"`, 1)
	out, _, err := cmdtest.Run(t, NewCmd(), func(m, p string) (int, string) { return 200, body }, "plan")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Past due since") || !strings.Contains(out, "overdue") {
		t.Fatalf("output:\n%s", out)
	}
}

func TestPlanAPIError(t *testing.T) {
	_, _, err := cmdtest.Run(t, NewCmd(), func(m, p string) (int, string) { return 403, `{"detail":"Permission denied"}` }, "plan")
	if err == nil {
		t.Fatal("expected an error")
	}
}

func TestScheduledChange(t *testing.T) {
	yearly, monthly := "yearly", "monthly"
	cur := planRef{Code: "business", Name: "Business"}
	cases := []struct {
		next     *planRef
		interval *string
		want     string
	}{
		{nil, nil, ""},
		{&planRef{Code: "business", Name: "Business"}, &monthly, ""},
		{&planRef{Code: "pro", Name: "Pro"}, nil, "to Pro (monthly)"},
		{nil, &yearly, "to Business (yearly)"},
		{&planRef{Code: "pro", Name: "Pro"}, &yearly, "to Pro (yearly)"},
	}
	for _, c := range cases {
		if got := scheduledChange(cur, "monthly", c.next, c.interval); got != c.want {
			t.Errorf("scheduledChange(%v, %v) = %q, want %q", c.next, c.interval, got, c.want)
		}
	}
}

func TestAccountPlanSummary(t *testing.T) {
	cases := map[string]string{
		`null`: "Free",
		`{"code":"pro","name":"Pro","status":"active","renews_at":"2026-11-01T00:00:00","cancel_at_period_end":false}`: "Pro, renews 2026-11-01",
		`{"code":"pro","name":"Pro","status":"active","renews_at":"2026-11-01","cancel_at_period_end":true}`:           "Pro, ends 2026-11-01",
		`{"code":"pro","name":"Pro","status":"past_due","renews_at":"2026-11-01","cancel_at_period_end":false}`:        "Pro (past_due), renews 2026-11-01",
		`{"code":"enterprise","name":"Enterprise","status":"active","renews_at":null,"cancel_at_period_end":false}`:    "Enterprise",
	}
	for raw, want := range cases {
		var p *AccountPlan
		if err := json.Unmarshal([]byte(raw), &p); err != nil {
			t.Fatal(err)
		}
		if got := p.Summary(); got != want {
			t.Errorf("%s: got %q, want %q", raw, got, want)
		}
	}
}

func TestUSDAcceptsStringAndNumber(t *testing.T) {
	var v struct{ A, B, C USD }
	if err := json.Unmarshal([]byte(`{"A":"49.5","B":49,"C":null}`), &v); err != nil {
		t.Fatal(err)
	}
	if v.A.String() != "$49.50" || v.B.String() != "$49.00" || v.C.String() != "-" {
		t.Fatalf("%v", v)
	}
}
