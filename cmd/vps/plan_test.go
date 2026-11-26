package vps

import (
	"encoding/json"
	"testing"
)

func TestVPSPlanRowPrices(t *testing.T) {
	var discounted, plain vpsPlanRow
	if err := json.Unmarshal([]byte(`{"plan_name":"gp.nano","price_per_hour":"0.07639","effective_price_per_hour":0.03819,"discount_percent":12.5,"discount_label":"x"}`), &discounted); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(`{"plan_name":"gp.nano","price_per_hour":"0.07639","effective_price_per_hour":null,"discount_percent":null}`), &plain); err != nil {
		t.Fatal(err)
	}
	text, hourly, disc := discounted.prices()
	if text != "0.03819" || hourly != 0.03819 || disc != "-12.5%" {
		t.Fatalf("discounted: %s %v %s", text, hourly, disc)
	}
	text, hourly, disc = plain.prices()
	if text != "0.07639" || hourly != 0.07639 || disc != "" {
		t.Fatalf("plain: %s %v %s", text, hourly, disc)
	}
}

func TestPlanRowPricesWholePercent(t *testing.T) {
	var p vpsPlanRow
	if err := json.Unmarshal([]byte(`{"plan_name":"gp.nano","price_per_hour":0.07639,"effective_price_per_hour":0.03819,"discount_percent":50.0}`), &p); err != nil {
		t.Fatal(err)
	}
	if _, _, disc := p.prices(); disc != "-50%" {
		t.Fatalf("discount = %q, want -50%%", disc)
	}
}
