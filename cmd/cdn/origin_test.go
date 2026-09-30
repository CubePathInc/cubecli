package cdn

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/CubePathInc/cubecli/internal/api"
	"github.com/CubePathInc/cubecli/internal/cmdutil"
	"github.com/spf13/cobra"
)

const (
	zoneUUID   = "11111111-2222-4333-8444-555555555555"
	bucketUUID = "66666666-7777-4888-9999-000000000000"
)

type recorded struct {
	Method string
	Path   string
	Body   map[string]interface{}
}

// run executes `cdn <args>` against a fake API and returns the requests it sent.
func run(t *testing.T, args ...string) ([]recorded, error) {
	t.Helper()
	var mu sync.Mutex
	var reqs []recorded
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		if raw, _ := io.ReadAll(r.Body); len(raw) > 0 {
			_ = json.Unmarshal(raw, &body)
		}
		mu.Lock()
		reqs = append(reqs, recorded{r.Method, r.URL.Path, body})
		mu.Unlock()
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/object-storage/buckets":
			_, _ = w.Write([]byte(`[{"uuid":"` + bucketUUID + `","name":"photos"}]`))
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/origins"):
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"uuid":"o","name":"photos","address":"eu.cubestorage.io","object_storage_bucket_uuid":"` + bucketUUID + `"}`))
		default:
			_, _ = w.Write([]byte(`{"detail":"ok"}`))
		}
	}))
	defer srv.Close()

	root := &cobra.Command{Use: "cubecli", SilenceUsage: true, SilenceErrors: true}
	root.PersistentFlags().Bool("json", false, "")
	root.AddCommand(NewCmd())
	root.SetArgs(args)
	ctx := context.WithValue(context.Background(), cmdutil.ClientKey, api.NewClient(srv.URL, "tok"))

	oldStdout, oldStderr := os.Stdout, os.Stderr
	devnull, _ := os.Open(os.DevNull)
	os.Stdout, os.Stderr = devnull, devnull
	err := root.ExecuteContext(ctx)
	os.Stdout, os.Stderr = oldStdout, oldStderr
	_ = devnull.Close()
	return reqs, err
}

func TestOriginCreateWithBucketResolvesName(t *testing.T) {
	reqs, err := run(t, "cdn", "origin", "create", zoneUUID, "--name", "photos", "--bucket", "photos")
	if err != nil {
		t.Fatal(err)
	}
	req := reqs[len(reqs)-1]
	if req.Method != http.MethodPost || req.Path != "/cdn/zones/"+zoneUUID+"/origins" {
		t.Fatalf("got %+v", req)
	}
	if req.Body["object_storage_bucket_uuid"] != bucketUUID {
		t.Fatalf("body %v", req.Body)
	}
	for _, k := range []string{"address", "origin_url", "port", "protocol", "host_header", "verify_ssl", "health_check_path"} {
		if _, ok := req.Body[k]; ok {
			t.Fatalf("%s sent with --bucket: %v", k, req.Body)
		}
	}
}

func TestOriginCreateBucketExcludesAddress(t *testing.T) {
	for _, flag := range [][]string{{"--url", "https://x.example.com"}, {"--address", "1.2.3.4"}, {"--host-header", "x"}} {
		args := append([]string{"cdn", "origin", "create", zoneUUID, "--name", "p", "--bucket", "photos"}, flag...)
		reqs, err := run(t, args...)
		if err == nil {
			t.Fatalf("%v: expected an error", flag)
		}
		if len(reqs) != 0 {
			t.Fatalf("%v: sent %+v", flag, reqs)
		}
	}
}

func TestOriginCreateNeedsATarget(t *testing.T) {
	if _, err := run(t, "cdn", "origin", "create", zoneUUID, "--name", "p"); err == nil {
		t.Fatal("expected an error without --url, --address or --bucket")
	}
}

func TestOriginCreateWithAddress(t *testing.T) {
	reqs, err := run(t, "cdn", "origin", "create", zoneUUID, "--name", "web", "--address", "1.2.3.4", "--port", "443")
	if err != nil {
		t.Fatal(err)
	}
	b := reqs[len(reqs)-1].Body
	if b["address"] != "1.2.3.4" || b["verify_ssl"] != true || b["health_check_path"] != "/health" {
		t.Fatalf("body %v", b)
	}
	if _, ok := b["object_storage_bucket_uuid"]; ok {
		t.Fatalf("bucket sent without --bucket: %v", b)
	}
}
