package objectstorage

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/CubePathInc/cubecli/internal/api"
	"github.com/CubePathInc/cubecli/internal/cmdutil"
	"github.com/spf13/cobra"
)

const (
	bucketUUID = "11111111-2222-4333-8444-555555555555"
	otherUUID  = "66666666-7777-4888-9999-000000000000"
	keyUUID    = "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee"
	// uuidNamed is a valid bucket name that is also uuid-shaped.
	uuidNamed     = "12345678-1234-1234-1234-123456789012"
	uuidNamedUUID = "bbbbbbbb-cccc-4ddd-8eee-ffffffffffff"
)

type recorded struct {
	Method string
	Path   string // path plus raw query
	Body   map[string]interface{}
}

// fakeAPI answers the Object Storage routes with canned JSON and records every request.
type fakeAPI struct {
	mu       sync.Mutex
	requests []recorded
}

func (f *fakeAPI) handler(w http.ResponseWriter, r *http.Request) {
	var body map[string]interface{}
	if raw, _ := io.ReadAll(r.Body); len(raw) > 0 {
		_ = json.Unmarshal(raw, &body)
	}
	path := r.URL.Path
	if r.URL.RawQuery != "" {
		path += "?" + r.URL.RawQuery
	}
	f.mu.Lock()
	f.requests = append(f.requests, recorded{r.Method, path, body})
	f.mu.Unlock()

	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/object-storage/buckets":
		_, _ = w.Write([]byte(`[{"uuid":"` + bucketUUID + `","name":"photos","status":"active","tier":{"name":"Infrequent Access"}},
			{"uuid":"` + otherUUID + `","name":"backups","status":"active","tier":{"name":"Infrequent Access"}},
			{"uuid":"` + uuidNamedUUID + `","name":"` + uuidNamed + `","status":"active","tier":{"name":"Infrequent Access"}}]`))
	case r.Method == http.MethodGet && r.URL.Path == "/object-storage/keys":
		_, _ = w.Write([]byte(`[{"uuid":"` + keyUUID + `","name":"web","access_key_id":"CP7Q2M9XK4B1N8R5T3W6","permission":"read_only","bucket_scope":null,"status":"active"},
			{"uuid":"` + otherUUID + `","name":"orphan","access_key_id":"CPORPHANKEY000000000","permission":"read_only","bucket_scope":[],"status":"active"}]`))
	case r.Method == http.MethodPost && r.URL.Path == "/object-storage/keys":
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"detail":"Access key created. Copy the secret now: it will not be shown again.","uuid":"` + keyUUID + `","name":"My Backups","access_key_id":"CP7Q2M9XK4B1N8R5T3W6","secret_access_key":"s3cr3t","permission":"read_write","bucket_scope":null,"region":"eu","endpoint":"https://eu.cubestorage.io","status":"pending","expires_at":null}`))
	case r.Method == http.MethodPost && r.URL.Path == "/object-storage/buckets":
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"detail":"Bucket is being created","uuid":"` + bucketUUID + `","name":"photos","status":"pending","endpoint":"https://eu.cubestorage.io"}`))
	case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/cdn"):
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"detail":"CDN connection started","zone_uuid":"z","zone_name":"photos","domain":"photos.cubecdn.io","custom_domain":null,"reused_zone":false,"status":"connecting"}`))
	case r.Method == http.MethodGet && r.URL.Path == "/object-storage/usage":
		_, _ = w.Write([]byte(`{"period":"2026-09","metrics_available":false,"total_cost":0.0012,"projected_cost":0.0013,"tiers":[{"tier":{"name":"Infrequent Access"},"storage_gib_month":null,"cost":0.0012,"free_tier":{"storage_gb_month":{"included":5,"used":null}}}],"buckets":[]}`))
	case r.Method == http.MethodGet && r.URL.Path == "/object-storage/tiers":
		_, _ = w.Write([]byte(`[{"slug":"infrequent_access","name":"Infrequent Access","prices":{"storage_gb_month":0.004},"free_tier":{"requests":20000},"accepting_new":true}]`))
	default:
		_, _ = w.Write([]byte(`{"detail":"ok"}`))
	}
}

// run executes `objectstorage <args>` against a fake API and returns what it
// printed to stdout and the requests it sent.
func run(t *testing.T, args ...string) (string, []recorded, error) {
	t.Helper()
	f := &fakeAPI{}
	srv := httptest.NewServer(http.HandlerFunc(f.handler))
	defer srv.Close()

	root := &cobra.Command{Use: "cubecli", SilenceUsage: true, SilenceErrors: true}
	root.PersistentFlags().Bool("json", false, "")
	root.AddCommand(NewCmd())
	root.SetArgs(args)
	ctx := context.WithValue(context.Background(), cmdutil.ClientKey, api.NewClient(srv.URL, "tok"))

	oldStdout, oldStderr := os.Stdout, os.Stderr
	r, w, _ := os.Pipe()
	os.Stdout = w
	devnull, _ := os.Open(os.DevNull)
	os.Stderr = devnull
	done := make(chan string)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		done <- buf.String()
	}()
	err := root.ExecuteContext(ctx)
	_ = w.Close()
	os.Stdout, os.Stderr = oldStdout, oldStderr
	_ = devnull.Close()
	out := <-done
	return out, f.requests, err
}

func last(reqs []recorded) recorded {
	if len(reqs) == 0 {
		return recorded{}
	}
	return reqs[len(reqs)-1]
}

func TestNormalizeTier(t *testing.T) {
	cases := map[string]string{
		"ia":                "infrequent_access",
		"IA":                "infrequent_access",
		"infrequent_access": "infrequent_access",
		"standard":          "standard",
		bucketUUID:          bucketUUID,
	}
	for in, want := range cases {
		if got := normalizeTier(in); got != want {
			t.Errorf("normalizeTier(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestAliasS3(t *testing.T) {
	_, reqs, err := run(t, "s3", "tiers", "--json")
	if err != nil {
		t.Fatal(err)
	}
	if last(reqs).Path != "/object-storage/tiers" {
		t.Fatalf("got %+v", reqs)
	}
}

func TestBucketCreateSendsTierSlug(t *testing.T) {
	_, reqs, err := run(t, "objectstorage", "bucket", "create", "photos", "--tier", "ia", "--project", "12", "--versioning")
	if err != nil {
		t.Fatal(err)
	}
	req := last(reqs)
	if req.Method != http.MethodPost || req.Path != "/object-storage/buckets" {
		t.Fatalf("got %+v", req)
	}
	if req.Body["name"] != "photos" || req.Body["tier"] != "infrequent_access" ||
		req.Body["project_id"] != float64(12) || req.Body["versioning"] != true {
		t.Fatalf("body %v", req.Body)
	}
}

func TestBucketCreateRequiresTier(t *testing.T) {
	if _, _, err := run(t, "s3", "bucket", "create", "photos"); err == nil {
		t.Fatal("expected an error without --tier")
	}
}

func TestBucketListFilters(t *testing.T) {
	_, reqs, err := run(t, "s3", "bucket", "list", "--project", "7", "--tier", "ia")
	if err != nil {
		t.Fatal(err)
	}
	if got := last(reqs).Path; got != "/object-storage/buckets?project_id=7&tier=infrequent_access" {
		t.Fatalf("path %s", got)
	}
}

func TestBucketGetResolvesName(t *testing.T) {
	_, reqs, err := run(t, "s3", "bucket", "get", "backups", "--json")
	if err != nil {
		t.Fatal(err)
	}
	if got := last(reqs).Path; got != "/object-storage/buckets/"+otherUUID {
		t.Fatalf("path %s", got)
	}
}

func TestBucketGetUUIDShapedNameWinsOverUUID(t *testing.T) {
	_, reqs, err := run(t, "s3", "bucket", "get", uuidNamed, "--json")
	if err != nil {
		t.Fatal(err)
	}
	if got := last(reqs).Path; got != "/object-storage/buckets/"+uuidNamedUUID {
		t.Fatalf("path %s", got)
	}
	_, reqs, err = run(t, "s3", "bucket", "get", bucketUUID, "--json")
	if err != nil {
		t.Fatal(err)
	}
	if got := last(reqs).Path; got != "/object-storage/buckets/"+bucketUUID {
		t.Fatalf("path %s", got)
	}
}

func TestKeyListEmptyScopeIsNotBlank(t *testing.T) {
	out, _, err := run(t, "s3", "key", "list")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "none (buckets deleted)") {
		t.Fatalf("output %s", out)
	}
}

func TestBucketGetUnknownName(t *testing.T) {
	_, _, err := run(t, "s3", "bucket", "get", "nope")
	if err == nil || !strings.Contains(err.Error(), `bucket "nope" not found`) {
		t.Fatalf("err %v", err)
	}
}

func TestBucketDeleteForceSkipsPromptWithoutPurge(t *testing.T) {
	_, reqs, err := run(t, "s3", "bucket", "delete", bucketUUID, "--force")
	if err != nil {
		t.Fatal(err)
	}
	req := last(reqs)
	if req.Method != http.MethodDelete || req.Path != "/object-storage/buckets/"+bucketUUID {
		t.Fatalf("got %+v", req)
	}
}

func TestBucketDeletePurgeSendsForceQuery(t *testing.T) {
	_, reqs, err := run(t, "s3", "bucket", "delete", "photos", "--purge", "--force")
	if err != nil {
		t.Fatal(err)
	}
	if got := last(reqs).Path; got != "/object-storage/buckets/"+bucketUUID+"?force=true" {
		t.Fatalf("path %s", got)
	}
}

func TestBucketUpdate(t *testing.T) {
	_, reqs, err := run(t, "s3", "bucket", "update", bucketUUID, "--versioning", "enabled", "--protected=false")
	if err != nil {
		t.Fatal(err)
	}
	req := last(reqs)
	if req.Method != http.MethodPatch || req.Body["versioning"] != "enabled" || req.Body["protected"] != false {
		t.Fatalf("got %+v", req)
	}
	if _, _, err := run(t, "s3", "bucket", "update", bucketUUID); err == nil {
		t.Fatal("expected an error without fields")
	}
}

func TestBucketCDNConnectAndDisconnect(t *testing.T) {
	_, reqs, err := run(t, "s3", "bucket", "cdn", "connect", "photos", "--zone-name", "photos", "--plan", "starter")
	if err != nil {
		t.Fatal(err)
	}
	req := last(reqs)
	if req.Method != http.MethodPost || req.Path != "/object-storage/buckets/"+bucketUUID+"/cdn" ||
		req.Body["zone_name"] != "photos" || req.Body["plan_name"] != "starter" {
		t.Fatalf("got %+v", req)
	}
	if _, ok := req.Body["custom_domain"]; ok {
		t.Fatalf("custom_domain sent without the flag: %v", req.Body)
	}

	_, reqs, err = run(t, "s3", "bucket", "cdn", "disconnect", bucketUUID, "-f")
	if err != nil {
		t.Fatal(err)
	}
	if req := last(reqs); req.Method != http.MethodDelete || req.Path != "/object-storage/buckets/"+bucketUUID+"/cdn" {
		t.Fatalf("got %+v", req)
	}
}

func TestKeyCreateBody(t *testing.T) {
	_, reqs, err := run(t, "s3", "key", "create", "--name", "web", "--tier", "ia",
		"--bucket", "photos", "--bucket", otherUUID, "--permission", "read_only", "--expires-at", "2030-01-02", "--json")
	if err != nil {
		t.Fatal(err)
	}
	req := last(reqs)
	if req.Method != http.MethodPost || req.Path != "/object-storage/keys" {
		t.Fatalf("got %+v", req)
	}
	b := req.Body
	if b["name"] != "web" || b["tier"] != "infrequent_access" || b["permission"] != "read_only" || b["expires_at"] != "2030-01-02T00:00:00" {
		t.Fatalf("body %v", b)
	}
	uuids, _ := b["bucket_uuids"].([]interface{})
	if len(uuids) != 2 || uuids[0] != bucketUUID || uuids[1] != otherUUID {
		t.Fatalf("bucket_uuids %v", b["bucket_uuids"])
	}
	if _, ok := b["project_id"]; ok {
		t.Fatalf("project_id sent without --project: %v", b)
	}
}

func TestKeyCreateAllBucketsOmitsScope(t *testing.T) {
	_, reqs, err := run(t, "s3", "key", "create", "--name", "web", "--tier", "infrequent_access", "--json")
	if err != nil {
		t.Fatal(err)
	}
	b := last(reqs).Body
	if _, ok := b["bucket_uuids"]; ok || b["permission"] != "read_write" {
		t.Fatalf("body %v", b)
	}
}

func TestKeyCreateRejectsBadInput(t *testing.T) {
	for _, args := range [][]string{
		{"--permission", "admin"},
		{"--output", "yaml"},
		{"--expires-at", "tomorrow"},
		{"--expires-in", "-1h"},
	} {
		base := []string{"s3", "key", "create", "--name", "k", "--tier", "ia"}
		_, reqs, err := run(t, append(base, args...)...)
		if err == nil {
			t.Errorf("%v: expected an error", args)
		}
		if len(reqs) != 0 {
			t.Errorf("%v: sent %d requests before validating", args, len(reqs))
		}
	}
}

func TestKeyCreateOutputEnvIsOnlyCredentials(t *testing.T) {
	out, _, err := run(t, "s3", "key", "create", "--name", "My Backups", "--tier", "ia", "--output", "env")
	if err != nil {
		t.Fatal(err)
	}
	want := `# CubePath Object Storage: My Backups
AWS_ACCESS_KEY_ID=CP7Q2M9XK4B1N8R5T3W6
AWS_SECRET_ACCESS_KEY=s3cr3t
AWS_REGION=eu
AWS_DEFAULT_REGION=eu
AWS_ENDPOINT_URL=https://eu.cubestorage.io
AWS_ENDPOINT_URL_S3=https://eu.cubestorage.io
`
	if out != want {
		t.Fatalf("stdout:\n%s\nwant:\n%s", out, want)
	}
}

func TestCredentialFiles(t *testing.T) {
	k := createdKey{Name: "My Backups!", AccessKeyID: "CPKEY", SecretAccessKey: "SECRET", Region: "eu", Endpoint: "https://eu.cubestorage.io"}

	rc := rcloneConf(k)
	wantRclone := `[cubepath-my-backups]
type = s3
provider = Other
access_key_id = CPKEY
secret_access_key = SECRET
endpoint = https://eu.cubestorage.io
region = eu
force_path_style = true
acl = private
`
	if rc != wantRclone {
		t.Errorf("rclone:\n%s", rc)
	}

	aws := awsCredentials(k)
	wantAWS := `# Append to ~/.aws/credentials, then: aws --profile cubepath-my-backups s3 ls
[cubepath-my-backups]
aws_access_key_id = CPKEY
aws_secret_access_key = SECRET
region = eu
endpoint_url = https://eu.cubestorage.io
`
	if aws != wantAWS {
		t.Errorf("aws:\n%s", aws)
	}

	if got := profileName("---"); got != "cubepath-storage" {
		t.Errorf("profileName fallback %q", got)
	}
}

func TestKeyDeleteResolvesAccessKeyIDAndName(t *testing.T) {
	for _, ref := range []string{"CP7Q2M9XK4B1N8R5T3W6", "web", keyUUID} {
		_, reqs, err := run(t, "s3", "key", "delete", ref, "--force")
		if err != nil {
			t.Fatalf("%s: %v", ref, err)
		}
		if req := last(reqs); req.Method != http.MethodDelete || req.Path != "/object-storage/keys/"+keyUUID {
			t.Fatalf("%s: got %+v", ref, req)
		}
	}
}

func TestUsageQueryAndMissingMetrics(t *testing.T) {
	out, reqs, err := run(t, "s3", "usage", "--period", "2026-08", "--tier", "ia")
	if err != nil {
		t.Fatal(err)
	}
	if got := last(reqs).Path; got != "/object-storage/usage?period=2026-08&tier=infrequent_access" {
		t.Fatalf("path %s", got)
	}
	if !strings.Contains(out, "$0.0012") {
		t.Fatalf("stdout misses the cost:\n%s", out)
	}
}

func TestParseExpiry(t *testing.T) {
	now := time.Date(2026, 9, 29, 10, 0, 0, 0, time.UTC)
	cases := []struct{ at, in, want string }{
		{"", "", ""},
		{"", "30d", "2026-10-29T10:00:00"},
		{"", "36h", "2026-09-30T22:00:00"},
		{"2026-12-01", "", "2026-12-01T00:00:00"},
		{"2026-12-01T08:30:00", "", "2026-12-01T08:30:00"},
		{"2026-12-01T10:30:00+02:00", "", "2026-12-01T08:30:00"},
	}
	for _, c := range cases {
		got, err := parseExpiry(c.at, c.in, now)
		if err != nil || got != c.want {
			t.Errorf("parseExpiry(%q, %q) = %q, %v; want %q", c.at, c.in, got, err, c.want)
		}
	}
}

func TestFormatting(t *testing.T) {
	if got := formatBytes(0); got != "0 B" {
		t.Errorf("formatBytes(0) = %q", got)
	}
	if got := formatBytes(3 * gib / 2); got != "1.50 GiB" {
		t.Errorf("formatBytes(1.5 GiB) = %q", got)
	}
	if got := formatCount(1234567); got != "1,234,567" {
		t.Errorf("formatCount = %q", got)
	}
	if got := formatUSD(0.00123); got != "$0.0012" {
		t.Errorf("formatUSD small = %q", got)
	}
	if got := formatUSD(12.5); got != "$12.50" {
		t.Errorf("formatUSD = %q", got)
	}
	if got := formatPrice(0.004); got != "$0.004" {
		t.Errorf("formatPrice = %q", got)
	}
}
