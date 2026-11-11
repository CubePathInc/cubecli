package bmref

import (
	"encoding/json"
	"testing"
)

func TestParse(t *testing.T) {
	ok := map[string]string{
		"9":                                    "9",
		"0123":                                 "0123",
		"3F2504E0-4F89-41D3-9A0C-0305E82C3301": "3f2504e0-4f89-41d3-9a0c-0305e82c3301",
		"3f2504e0-4f89-41d3-9a0c-0305e82c3301": "3f2504e0-4f89-41d3-9a0c-0305e82c3301",
	}
	for in, want := range ok {
		got, err := Parse(in)
		if err != nil || got != want {
			t.Errorf("Parse(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, in := range []string{"", "abc", "-1", "1.5", " 9", "9/protection", "9?x=1", "3f2504e04f8941d39a0c0305e82c3301", "3f2504e0-4f89-41d3-9a0c-0305e82c330", "3f2504e0-4f89-41d3-9a0c-0305e82c33zz", "../9"} {
		if got, err := Parse(in); err == nil {
			t.Errorf("Parse(%q) = %q, want error", in, got)
		}
	}
}

func TestFlexID(t *testing.T) {
	var v struct {
		ID FlexID `json:"id"`
	}
	for in, want := range map[string]string{`{"id":9}`: "9", `{"id":"9"}`: "9", `{"id":null}`: "", `{}`: "", `{"id":"3f2504e0-4f89-41d3-9a0c-0305e82c3301"}`: "3f2504e0-4f89-41d3-9a0c-0305e82c3301"} {
		v.ID = ""
		if err := json.Unmarshal([]byte(in), &v); err != nil || string(v.ID) != want {
			t.Errorf("%s: got %q, %v; want %q", in, v.ID, err, want)
		}
	}
	if err := json.Unmarshal([]byte(`{"id":true}`), &v); err == nil {
		t.Error("a boolean id must fail")
	}
	b, _ := json.Marshal(struct {
		ID FlexID `json:"id"`
	}{"9"})
	if string(b) != `{"id":9}` {
		t.Errorf("a numeric id must print as a number, got %s", b)
	}
}

func TestMatches(t *testing.T) {
	u := "3f2504e0-4f89-41d3-9a0c-0305e82c3301"
	cases := []struct {
		ref  string
		id   FlexID
		uuid string
		want bool
	}{
		{"9", "9", "", true},
		{"9", "10", "", false},
		{u, "9", u, true},
		{u, "9", "", false}, // old API: no uuid to match
		{u, "9", "other", false},
		{"", "", "", false},
	}
	for _, c := range cases {
		if got := Matches(c.ref, c.id, c.uuid); got != c.want {
			t.Errorf("Matches(%q, %q, %q) = %v", c.ref, c.id, c.uuid, got)
		}
	}
}
