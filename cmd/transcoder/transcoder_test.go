package transcoder

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/CubePathInc/cubecli/internal/cmdtest"
)

const jobJSON = `{"uuid":"j1","status":"queued","input":{"source":"url","url":"https://x/in.mp4"},"output":{"s3":{"bucket":"b","path":"out/"}},"spec":{"outputs":[{"type":"hls"}]},"progress":0}`

func TestCreateFromFlags(t *testing.T) {
	_, reqs, err := cmdtest.Run(t, NewCmd(), func(m, p string) (int, string) { return 201, jobJSON },
		"create", "--input-url", "https://x/in.mp4", "--output-bucket", "b", "--output-path", "out/",
		"--output-secret-key", "sk", "--format", "hls", "--spec", `{"type":"file","codec":"h264"}`)
	if err != nil {
		t.Fatal(err)
	}
	r := cmdtest.Last(reqs)
	if r.Method != http.MethodPost || r.Path != "/transcoder/jobs" {
		t.Fatalf("got %s %s", r.Method, r.Path)
	}
	b := r.Obj()
	in, _ := b["input"].(map[string]interface{})
	if in["source"] != "url" || in["url"] != "https://x/in.mp4" {
		t.Fatalf("input %v", in)
	}
	out, _ := b["output"].(map[string]interface{})
	s3, _ := out["s3"].(map[string]interface{})
	if s3["bucket"] != "b" || s3["secret_key"] != "sk" {
		t.Fatalf("output %v", out)
	}
	outputs, _ := b["outputs"].([]interface{})
	spec, _ := outputs[1].(map[string]interface{})
	if len(outputs) != 2 || spec["codec"] != "h264" {
		t.Fatalf("outputs %v", outputs)
	}
}

func TestCreateValidation(t *testing.T) {
	cases := [][]string{
		{"create", "--output-bucket", "b", "--format", "hls"},                                            // no input
		{"create", "--input-url", "u", "--input-bucket", "x", "--output-bucket", "b", "--format", "hls"}, // both inputs
		{"create", "--input-url", "u", "--format", "hls"},                                                // no output
		{"create", "--input-url", "u", "--output-bucket", "b"},                                           // no outputs
	}
	for _, args := range cases {
		if _, reqs, err := cmdtest.Run(t, NewCmd(), nil, args...); err == nil || len(reqs) != 0 {
			t.Fatalf("%v: got %v %v", args, err, reqs)
		}
	}
}

func TestBatchFromFile(t *testing.T) {
	f := filepath.Join(t.TempDir(), "batch.json")
	if err := os.WriteFile(f, []byte(`{"output":{"s3":{"bucket":"b"}},"outputs":[{"type":"hls"}],"inputs":[{"url":"https://x/1.mp4"}]}`), 0600); err != nil {
		t.Fatal(err)
	}
	_, reqs, err := cmdtest.Run(t, NewCmd(), func(m, p string) (int, string) {
		return 201, `{"batch_id":"bt","job_ids":["j1"],"count":1}`
	}, "batch", "--file", f)
	if err != nil {
		t.Fatal(err)
	}
	r := cmdtest.Last(reqs)
	if r.Path != "/transcoder/jobs/batch" || r.Obj()["inputs"] == nil {
		t.Fatalf("got %+v", r)
	}
}

func TestListAndCancel(t *testing.T) {
	_, reqs, err := cmdtest.Run(t, NewCmd(), func(m, p string) (int, string) { return 200, `{"jobs":[` + jobJSON + `]}` }, "list", "--batch", "bt", "--limit", "10")
	if err != nil {
		t.Fatal(err)
	}
	if p := cmdtest.Last(reqs).Path; p != "/transcoder/jobs?batch_id=bt&limit=10&offset=0" {
		t.Fatalf("got %s", p)
	}
	_, reqs, err = cmdtest.Run(t, NewCmd(), nil, "cancel", "j1", "--force")
	if err != nil {
		t.Fatal(err)
	}
	if r := cmdtest.Last(reqs); r.Method != http.MethodDelete || r.Path != "/transcoder/jobs/j1" {
		t.Fatalf("got %+v", r)
	}
}
