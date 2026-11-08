package feedback

import (
	"net/http"
	"strings"
	"testing"

	"github.com/CubePathInc/cubecli/internal/cmdtest"
	"github.com/spf13/cobra"
)

func runFeedback(t *testing.T, stdin string, respond cmdtest.Responder, args ...string) (string, []cmdtest.Request, error) {
	t.Helper()
	c := NewCmd()
	c.SetIn(strings.NewReader(stdin))
	return cmdtest.Run(t, c, respond, args...)
}

func TestSendsMessage(t *testing.T) {
	out, reqs, err := runFeedback(t, "", func(string, string) (int, string) {
		return http.StatusCreated, `{"detail":"Gracias"}`
	}, "love", "the", "CLI")
	if err != nil {
		t.Fatal(err)
	}
	if len(reqs) != 1 {
		t.Fatalf("want 1 request, got %d", len(reqs))
	}
	r := reqs[0]
	if r.Method != http.MethodPost || r.Path != "/feedback" {
		t.Fatalf("got %s %s", r.Method, r.Path)
	}
	obj := r.Obj()
	if obj["message"] != "love the CLI" || len(obj) != 1 {
		t.Fatalf("body %s", r.Raw)
	}
	if !strings.Contains(out, "Gracias") {
		t.Fatalf("output %q", out)
	}
}

func TestTopicAndSentiment(t *testing.T) {
	_, reqs, err := runFeedback(t, "", nil, "--topic", "dns_domains", "--sentiment", "very_positive", "nice")
	if err != nil {
		t.Fatal(err)
	}
	obj := cmdtest.Last(reqs).Obj()
	if obj["topic"] != "dns_domains" || obj["sentiment"] != "very_positive" || obj["message"] != "nice" {
		t.Fatalf("body %s", cmdtest.Last(reqs).Raw)
	}
}

func TestOptionalKeysAbsent(t *testing.T) {
	_, reqs, _ := runFeedback(t, "", nil, "--topic", "vps", "x")
	if _, ok := cmdtest.Last(reqs).Obj()["sentiment"]; ok {
		t.Fatal("sentiment must be omitted")
	}
	_, reqs, _ = runFeedback(t, "", nil, "--sentiment", "negative", "x")
	if _, ok := cmdtest.Last(reqs).Obj()["topic"]; ok {
		t.Fatal("topic must be omitted")
	}
}

func TestAllValuesAccepted(t *testing.T) {
	for _, tp := range topics {
		if _, reqs, err := runFeedback(t, "", nil, "--topic", tp, "x"); err != nil || len(reqs) != 1 {
			t.Fatalf("topic %s: %v", tp, err)
		}
	}
	for _, s := range sentiments {
		if _, reqs, err := runFeedback(t, "", nil, "--sentiment", s, "x"); err != nil || len(reqs) != 1 {
			t.Fatalf("sentiment %s: %v", s, err)
		}
	}
}

func TestStdin(t *testing.T) {
	_, reqs, err := runFeedback(t, "line one\r\nline two\r\n\n", nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := cmdtest.Last(reqs).Obj()["message"]; got != "line one\nline two" {
		t.Fatalf("message %q", got)
	}
}

func TestArgsWinOverStdin(t *testing.T) {
	_, reqs, _ := runFeedback(t, "from stdin", nil, "from args")
	if got := cmdtest.Last(reqs).Obj()["message"]; got != "from args" {
		t.Fatalf("message %q", got)
	}
}

func TestValidationSendsNothing(t *testing.T) {
	tests := []struct {
		name  string
		stdin string
		args  []string
		want  string
	}{
		{"bad topic", "", []string{"--topic", "nope", "x"}, "invalid --topic"},
		{"bad sentiment", "", []string{"--sentiment", "meh", "x"}, "invalid --sentiment"},
		{"empty", "", nil, "empty"},
		{"whitespace args", "", []string{"   ", "\t"}, "empty"},
		{"whitespace stdin", " \n\r\n ", nil, "empty"},
		{"too long by runes", "", []string{strings.Repeat("é", 501)}, "too long"},
		{"angle brackets", "", []string{"<script>alert(1)</script>"}, "cannot contain"},
		{"angle bracket stdin", "a > b", nil, "cannot contain"},
		{"control char", "", []string{"bad\x07bell"}, "control character"},
		{"nul stdin", "a\x00b", nil, "control character"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, reqs, err := runFeedback(t, tc.stdin, nil, tc.args...)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want %q", err, tc.want)
			}
			if len(reqs) != 0 {
				t.Fatalf("sent %d requests", len(reqs))
			}
		})
	}
}

func TestLimitsAreInRunes(t *testing.T) {
	if _, reqs, err := runFeedback(t, "", nil, strings.Repeat("é", 500)); err != nil || len(reqs) != 1 {
		t.Fatalf("500 runes must pass: %v", err)
	}
	if _, reqs, err := runFeedback(t, "", nil, "tab\tand\nnewline"); err != nil || len(reqs) != 1 {
		t.Fatalf("tab and newline must pass: %v", err)
	}
}

func TestJSONOutput(t *testing.T) {
	out, _, err := runFeedback(t, "", func(string, string) (int, string) {
		return http.StatusCreated, `{"detail":"Thanks"}`
	}, "--json", "x")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"detail": "Thanks"`) && !strings.Contains(out, `"detail":"Thanks"`) {
		t.Fatalf("output %q", out)
	}
}

func TestErrors(t *testing.T) {
	for _, status := range []int{400, 401, 403} {
		_, _, err := runFeedback(t, "", func(string, string) (int, string) {
			return status, `{"detail":"server said no"}`
		}, "x")
		if err == nil || err.Error() != "server said no" {
			t.Fatalf("%d: %v", status, err)
		}
	}
}

func TestRateLimit(t *testing.T) {
	_, reqs, err := runFeedback(t, "", func(string, string) (int, string) {
		return http.StatusTooManyRequests, `{"detail":"Too many requests"}`
	}, "x")
	if err == nil || !strings.Contains(err.Error(), "Too many requests") {
		t.Fatalf("err = %v", err)
	}
	if len(reqs) != 1 {
		t.Fatalf("requests %d", len(reqs))
	}
}

func TestCompletion(t *testing.T) {
	c := NewCmd()
	for _, name := range []string{"topic", "sentiment"} {
		fn, ok := c.GetFlagCompletionFunc(name)
		if !ok {
			t.Fatalf("no completion for %s", name)
		}
		vals, dir := fn(c, nil, "")
		if len(vals) == 0 || dir != cobra.ShellCompDirectiveNoFileComp {
			t.Fatalf("%s: %v %v", name, vals, dir)
		}
	}
}
