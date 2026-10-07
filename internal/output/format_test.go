package output

import (
	"testing"

	"github.com/fatih/color"
)

func TestFormatValue(t *testing.T) {
	for in, want := range map[interface{}]string{1073741824.0: "1073741824", 1.5: "1.5", "ON": "ON", true: "true"} {
		if got := FormatValue(in); got != want {
			t.Fatalf("%v: got %s", in, got)
		}
	}
}

func TestFormatStatusSnapshotStates(t *testing.T) {
	old := color.NoColor
	color.NoColor = false
	defer func() { color.NoColor = old }()
	for status, c := range map[string]*color.Color{
		"available":  green,
		"pending":    dim,
		"converting": dim,
		"deleting":   dim,
		"failed":     red,
	} {
		if got, want := FormatStatus(status), c.Sprint(status); got != want {
			t.Fatalf("%s: got %q, want %q", status, got, want)
		}
	}
	if got := FormatStatus("unknown"); got != "unknown" {
		t.Fatalf("unknown: got %q", got)
	}
}
