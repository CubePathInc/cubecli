package output

import "testing"

func TestFormatValue(t *testing.T) {
	for in, want := range map[interface{}]string{1073741824.0: "1073741824", 1.5: "1.5", "ON": "ON", true: "true"} {
		if got := FormatValue(in); got != want {
			t.Fatalf("%v: got %s", in, got)
		}
	}
}
