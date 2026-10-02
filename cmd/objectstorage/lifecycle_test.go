package objectstorage

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLifecycleGetTable(t *testing.T) {
	out, reqs, err := run(t, "s3", "bucket", "lifecycle", "get", "photos")
	if err != nil {
		t.Fatal(err)
	}
	if req := last(reqs); req.Method != http.MethodGet || req.Path != "/object-storage/buckets/"+bucketUUID+"/lifecycle" {
		t.Fatalf("got %+v", req)
	}
	for _, want := range []string{"logs-30d", "prefix logs/", "delete after 30 days", "delete noncurrent versions after 7 days (keep 3)", "active", "generation 3, applied 3"} {
		if !strings.Contains(out, want) {
			t.Fatalf("stdout misses %q:\n%s", want, out)
		}
	}
}

func TestLifecycleSetShortcut(t *testing.T) {
	_, reqs, err := run(t, "s3", "bucket", "lifecycle", "set", "photos", "--expire-days", "30", "--prefix", "logs/", "--force")
	if err != nil {
		t.Fatal(err)
	}
	req := last(reqs)
	if req.Method != http.MethodPut || req.Path != "/object-storage/buckets/"+bucketUUID+"/lifecycle" {
		t.Fatalf("got %+v", req)
	}
	rules, _ := req.Body["rules"].([]interface{})
	if len(rules) != 1 {
		t.Fatalf("rules %v", req.Body)
	}
	rule := rules[0].(map[string]interface{})
	if rule["id"] != "expire-30d" || rule["enabled"] != true {
		t.Fatalf("rule %v", rule)
	}
	if rule["filter"].(map[string]interface{})["prefix"] != "logs/" || rule["expiration"].(map[string]interface{})["days"] != float64(30) {
		t.Fatalf("rule %v", rule)
	}
}

func TestLifecycleSetFromFileBothShapes(t *testing.T) {
	dir := t.TempDir()
	wrapped := filepath.Join(dir, "wrapped.json")
	bare := filepath.Join(dir, "bare.json")
	_ = os.WriteFile(wrapped, []byte(`{"rules":[{"id":"a","enabled":true,"expiration":{"days":1}}]}`), 0o600)
	_ = os.WriteFile(bare, []byte(`[{"id":"a","enabled":true,"expiration":{"days":1}},{"id":"b","enabled":false,"expiration":{"date":"2030-01-01"}}]`), 0o600)
	for file, n := range map[string]int{wrapped: 1, bare: 2} {
		_, reqs, err := run(t, "s3", "bucket", "lifecycle", "set", "photos", "--file", file, "--force")
		if err != nil {
			t.Fatal(err)
		}
		if rules, _ := last(reqs).Body["rules"].([]interface{}); len(rules) != n {
			t.Fatalf("%s: %v", file, last(reqs).Body)
		}
	}
}

func TestLifecycleSetWaitPollsUntilApplied(t *testing.T) {
	lifecyclePollInterval = 0
	out, reqs, err := run(t, "s3", "bucket", "lifecycle", "set", "photos", "--expire-days", "7", "--force", "--wait")
	if err != nil {
		t.Fatal(err)
	}
	if req := last(reqs); req.Method != http.MethodGet || !strings.HasSuffix(req.Path, "/lifecycle") {
		t.Fatalf("no poll after the PUT: %+v", req)
	}
	if !strings.Contains(out, "logs-30d") {
		t.Fatalf("stdout %s", out)
	}
}

func TestLifecycleRejectsBadInput(t *testing.T) {
	cases := [][]string{
		{"s3", "bucket", "lifecycle", "set", "photos", "--force"},
		{"s3", "bucket", "lifecycle", "set", "photos", "--expire-days", "0", "--force"},
		{"s3", "bucket", "lifecycle", "set", "photos", "--expire-days", "36501", "--force"},
		{"s3", "bucket", "lifecycle", "set", "photos", "--expire-days", "3", "--file", "x.json", "--force"},
	}
	for _, args := range cases {
		if _, reqs, err := run(t, args...); err == nil || len(reqs) != 0 {
			t.Fatalf("%v: err %v, %d requests", args, err, len(reqs))
		}
	}
	file := filepath.Join(t.TempDir(), "bad.json")
	_ = os.WriteFile(file, []byte(`{"foo":1}`), 0o600)
	if _, _, err := run(t, "s3", "bucket", "lifecycle", "set", "photos", "--file", file, "--force"); err == nil || !strings.Contains(err.Error(), "not a rules document") {
		t.Fatalf("err %v", err)
	}
}

func TestLifecycleDelete(t *testing.T) {
	_, reqs, err := run(t, "s3", "bucket", "lifecycle", "delete", "photos", "--force")
	if err != nil {
		t.Fatal(err)
	}
	if req := last(reqs); req.Method != http.MethodDelete || req.Path != "/object-storage/buckets/"+bucketUUID+"/lifecycle" {
		t.Fatalf("got %+v", req)
	}
}
