package bmref

import "testing"

func TestParse(t *testing.T) {
	ok := map[string]string{
		"3F2504E0-4F89-41D3-9A0C-0305E82C3301": "3f2504e0-4f89-41d3-9a0c-0305e82c3301",
		"3f2504e0-4f89-41d3-9a0c-0305e82c3301": "3f2504e0-4f89-41d3-9a0c-0305e82c3301",
	}
	for in, want := range ok {
		got, err := Parse(in)
		if err != nil || got != want {
			t.Errorf("Parse(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, in := range []string{"", "9", "0123", "abc", "-1", "1.5", " 9", "9/protection", "9?x=1", "3f2504e04f8941d39a0c0305e82c3301", "3f2504e0-4f89-41d3-9a0c-0305e82c330", "3f2504e0-4f89-41d3-9a0c-0305e82c33zz", "../9", " 3f2504e0-4f89-41d3-9a0c-0305e82c3301"} {
		if got, err := Parse(in); err == nil {
			t.Errorf("Parse(%q) = %q, want error", in, got)
		}
	}
}

func TestTarget(t *testing.T) {
	u := "3f2504e0-4f89-41d3-9a0c-0305e82c3301"
	if got, err := Target("baremetal", "3F2504E0-4F89-41D3-9A0C-0305E82C3301"); err != nil || got != u {
		t.Errorf("baremetal uuid: %q %v", got, err)
	}
	if _, err := Target("baremetal", "9"); err == nil {
		t.Error("a baremetal number must be refused")
	}
	if got, err := Target("vps", "9"); err != nil || got != "9" {
		t.Errorf("a vps target must pass unchanged: %q %v", got, err)
	}
	if got, err := Target("ip", "10.0.0.5"); err != nil || got != "10.0.0.5" {
		t.Errorf("an ip target must pass unchanged: %q %v", got, err)
	}
}

func TestMatches(t *testing.T) {
	u := "3f2504e0-4f89-41d3-9a0c-0305e82c3301"
	cases := []struct {
		ref, id string
		want    bool
	}{
		{u, u, true},
		{u, "3F2504E0-4F89-41D3-9A0C-0305E82C3301", true},
		{u, "other", false},
		{"", "", false},
	}
	for _, c := range cases {
		if got := Matches(c.ref, c.id); got != c.want {
			t.Errorf("Matches(%q, %q) = %v", c.ref, c.id, got)
		}
	}
}
