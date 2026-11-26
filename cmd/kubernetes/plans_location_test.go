package kubernetes

import "testing"

func TestLocationCell(t *testing.T) {
	id := 5
	if got := locationCell(&id); got != "5" {
		t.Fatalf("got %q", got)
	}
	if got := locationCell(nil); got != "-" {
		t.Fatalf("got %q", got)
	}
}
