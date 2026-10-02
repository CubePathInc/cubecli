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
	destUUID      = "dddddddd-1111-4222-8333-444444444444"
	ruleUUID      = "eeeeeeee-5555-4666-8777-888888888888"
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
		_, _ = w.Write([]byte(`[{"uuid":"` + bucketUUID + `","name":"photos","status":"active","tier":{"slug":"infrequent_access","name":"Infrequent Access"},"tags":{"team":"web","env":"prod"}},
			{"uuid":"` + otherUUID + `","name":"backups","status":"active","tier":{"name":"Infrequent Access"},"object_lock":{"enabled":true,"default_retention":{"mode":"governance","days":30,"years":null}}},
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
	case r.Method == http.MethodGet && r.URL.Path == "/object-storage/usage":
		_, _ = w.Write([]byte(`{"period":"2026-09","metrics_available":false,"total_cost":0.0012,"projected_cost":0.0013,"tiers":[{"tier":{"name":"Infrequent Access"},"storage_gib_month":null,"cost":0.0012,"free_tier":{"storage_gb_month":{"included":5,"used":null}}}],"buckets":[]}`))
	case r.Method == http.MethodPost && r.URL.Path == "/graphql":
		_, _ = w.Write([]byte(`{"data":{"objectStorageBucket":{"uuid":"` + bucketUUID + `","name":"photos","storageMeasuredAt":1759400000,
			"storage":{"start":1,"end":2,"step":3600,"series":[{"name":"size_bytes","unit":"BYTES","points":[{"ts":1,"value":1024},{"ts":2,"value":2048}]}]},
			"traffic":{"start":1,"end":2,"step":300,"series":[{"name":"egress_bytes","unit":"BYTES","points":[{"ts":1,"value":1048576},{"ts":2,"value":1048576}]},{"name":"class_b_requests","unit":"COUNT","points":[{"ts":1,"value":1000},{"ts":2,"value":234}]}]},
			"responses":{"start":1,"end":2,"step":300,"series":[{"name":"responses_4xx","unit":"COUNT","points":[{"ts":1,"value":3}]}]}}}}`))
	case r.Method == http.MethodGet && r.URL.Path == "/object-storage/buckets/"+bucketUUID+"/lifecycle":
		_, _ = w.Write([]byte(`{"bucket_uuid":"` + bucketUUID + `","status":"active","generation":3,"applied_generation":3,"error":null,"updated_at":"2026-10-02T10:00:00","notes":["Objects are removed within 48 hours of their due date."],"platform_rules":[],
			"rules":[{"id":"logs-30d","enabled":true,"filter":{"prefix":"logs/","tags":null,"object_size_greater_than":null,"object_size_less_than":null},"expiration":{"days":30,"date":null,"expired_object_delete_marker":null},"noncurrent_version_expiration":{"noncurrent_days":7,"newer_noncurrent_versions":3},"abort_incomplete_multipart_upload":null}]}`))
	case (r.Method == http.MethodPut || r.Method == http.MethodDelete) && r.URL.Path == "/object-storage/buckets/"+bucketUUID+"/lifecycle":
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"detail":"Lifecycle rules are being applied","generation":3,"notes":[]}`))
	case r.Method == http.MethodGet && r.URL.Path == "/object-storage/buckets/"+bucketUUID:
		_, _ = w.Write([]byte(`{"uuid":"` + bucketUUID + `","name":"photos","status":"active","tier":{"slug":"infrequent_access","name":"Infrequent Access"},"versioning":"off","object_lock":{"enabled":false,"default_retention":null},"encryption":{"algorithm":"AES256","scope":"new_objects"},"connection":{"endpoint":"https://eu.cubestorage.io","region":"eu"}}`))
	case r.Method == http.MethodGet && r.URL.Path == "/object-storage/tiers":
		_, _ = w.Write([]byte(`[{"uuid":"t1","slug":"infrequent_access","name":"Infrequent Access","region":"eu","endpoint":"https://eu.cubestorage.io","prices":{"storage_gb_month":0.004},"free_tier":{"requests":20000},"accepting_new":true}]`))
	case r.Method == http.MethodGet && r.URL.Path == "/object-storage/event-destinations":
		_, _ = w.Write([]byte(`[{"uuid":"` + destUUID + `","name":"uploads-hook","type":"webhook","url_masked":"https://example.com/***","notificator":null,"payload_format":"cubepath","status":"active","rules_count":1}]`))
	case (r.Method == http.MethodPost && (r.URL.Path == "/object-storage/event-destinations" || r.URL.Path == "/object-storage/event-destinations/"+destUUID+"/rotate-secret")):
		_, _ = w.Write([]byte(`{"destination":{"uuid":"` + destUUID + `","name":"uploads-hook","type":"webhook","url_masked":"https://example.com/***","payload_format":"cubepath","status":"active","rules_count":0},"signing_secret":"whsec_S3cretS3cretS3cretS3cretS3cr","previous_secret_expires_at":"2026-10-03T10:00:00"}`))
	case r.Method == http.MethodGet && r.URL.Path == "/object-storage/event-destinations/"+destUUID+"/deliveries":
		_, _ = w.Write([]byte(`{"deliveries":[{"ts":"2026-10-02T10:00:00.250","ts_ms":1790964001250,"event_id":"evt_1","delivery_id":"dlv_1","event_type":"object.created","bucket_uuid":"` + bucketUUID + `","bucket_name":"photos","rule_uuid":"` + ruleUUID + `","object_key":"incoming/a.jpg","attempt":2,"status":"failed","http_status":500,"latency_ms":120,"error":"HTTP 500"}],"next_before":1790964001250}`))
	case r.Method == http.MethodGet && r.URL.Path == "/object-storage/buckets/"+bucketUUID+"/event-rules":
		_, _ = w.Write([]byte(`[{"uuid":"` + ruleUUID + `","name":"on-created","bucket_uuid":"` + bucketUUID + `","destination":{"uuid":"` + destUUID + `","name":"uploads-hook","type":"webhook"},"events":["object.created"],"prefix":"incoming/","suffix":"","enabled":true,"status":"active","error_message":null}]`))
	case r.Method == http.MethodPost && r.URL.Path == "/object-storage/buckets/"+bucketUUID+"/event-rules":
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"uuid":"` + ruleUUID + `","name":"on-created","bucket_uuid":"` + bucketUUID + `","destination":{"uuid":"` + destUUID + `","name":"uploads-hook","type":"webhook"},"events":["object.created"],"prefix":"incoming/","suffix":".jpg","enabled":true,"status":"pending","error_message":null}`))
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

func TestBucketGetShowsEncryption(t *testing.T) {
	out, _, err := run(t, "s3", "bucket", "get", "photos")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "AES256 (new objects; older ones are being encrypted)") {
		t.Fatalf("output %s", out)
	}
	if got := formatEncryption(nil); got != "not applied yet" {
		t.Fatalf("nil: %s", got)
	}
	if got := formatEncryption(&bucketEncryption{Algorithm: "AES256", Scope: "all_objects"}); got != "AES256 (all objects)" {
		t.Fatalf("all: %s", got)
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

func TestBucketListTagFilterAndColumn(t *testing.T) {
	out, reqs, err := run(t, "s3", "bucket", "list", "--tag", "env=prod", "--tag", "team")
	if err != nil {
		t.Fatal(err)
	}
	if got := last(reqs).Path; got != "/object-storage/buckets?tag=env%3Dprod&tag=team" {
		t.Fatalf("path %s", got)
	}
	if !strings.Contains(out, "env=prod,team=web") {
		t.Fatalf("stdout misses the tags:\n%s", out)
	}
}

func TestBucketCreateTags(t *testing.T) {
	_, reqs, err := run(t, "s3", "bucket", "create", "photos", "--tier", "ia", "--tag", "env=prod", "--tag", "note=a=b", "--tag", "team")
	if err != nil {
		t.Fatal(err)
	}
	tags, _ := last(reqs).Body["tags"].(map[string]interface{})
	if len(tags) != 3 || tags["env"] != "prod" || tags["note"] != "a=b" || tags["team"] != "" {
		t.Fatalf("body %v", last(reqs).Body)
	}
	_, reqs, err = run(t, "s3", "bucket", "create", "photos", "--tier", "ia")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := last(reqs).Body["tags"]; ok {
		t.Fatalf("tags sent without --tag: %v", last(reqs).Body)
	}
}

func TestBucketUpdateTags(t *testing.T) {
	_, reqs, err := run(t, "s3", "bucket", "update", bucketUUID, "--tag", "env=dev")
	if err != nil {
		t.Fatal(err)
	}
	req := last(reqs)
	tags, ok := req.Body["tags"].(map[string]interface{})
	if req.Method != http.MethodPatch || !ok || len(tags) != 1 || tags["env"] != "dev" {
		t.Fatalf("got %+v", req)
	}
	if _, ok := req.Body["versioning"]; ok {
		t.Fatalf("versioning sent: %v", req.Body)
	}

	_, reqs, err = run(t, "s3", "bucket", "update", bucketUUID, "--clear-tags")
	if err != nil {
		t.Fatal(err)
	}
	if tags, ok := last(reqs).Body["tags"].(map[string]interface{}); !ok || len(tags) != 0 {
		t.Fatalf("clear body %v", last(reqs).Body)
	}

	_, reqs, err = run(t, "s3", "bucket", "update", bucketUUID, "--versioning", "enabled")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := last(reqs).Body["tags"]; ok {
		t.Fatalf("tags sent without tag flags: %v", last(reqs).Body)
	}

	for _, args := range [][]string{
		{"--tag", "env=dev", "--clear-tags"},
		{"--tag", "=prod"},
		{"--tag", " env=prod"},
		{"--tag", "env=a", "--tag", "env=b"},
	} {
		_, reqs, err := run(t, append([]string{"s3", "bucket", "update", bucketUUID}, args...)...)
		if err == nil {
			t.Errorf("%v: expected an error", args)
		}
		if len(reqs) != 0 {
			t.Errorf("%v: sent %d requests before validating", args, len(reqs))
		}
	}
}

func TestUsageTagFilter(t *testing.T) {
	_, reqs, err := run(t, "s3", "usage", "--tag", "env=prod")
	if err != nil {
		t.Fatal(err)
	}
	if got := last(reqs).Path; got != "/object-storage/usage?tag=env%3Dprod" {
		t.Fatalf("path %s", got)
	}
}

func TestKeyListIgnoresTagFilter(t *testing.T) {
	_, reqs, err := run(t, "s3", "key", "list", "--tier", "ia")
	if err != nil {
		t.Fatal(err)
	}
	if got := last(reqs).Path; got != "/object-storage/keys?tier=infrequent_access" {
		t.Fatalf("path %s", got)
	}
}

func TestFormatTags(t *testing.T) {
	if got := formatTags(nil, 40); got != "-" {
		t.Errorf("empty = %q", got)
	}
	if got := formatTags(map[string]string{"b": "2", "a": ""}, 40); got != "a=,b=2" {
		t.Errorf("sorted = %q", got)
	}
	if got := formatTags(map[string]string{"k": strings.Repeat("v", 50)}, 20); got != "k="+strings.Repeat("v", 15)+"..." {
		t.Errorf("truncated = %q", got)
	}
}

func TestBucketMetricsQueryAndTotals(t *testing.T) {
	out, reqs, err := run(t, "s3", "bucket", "metrics", "photos", "--range", "7d")
	if err != nil {
		t.Fatal(err)
	}
	req := last(reqs)
	if req.Method != http.MethodPost || req.Path != "/graphql" {
		t.Fatalf("got %+v", req)
	}
	vars, _ := req.Body["variables"].(map[string]interface{})
	if vars["uuid"] != bucketUUID || vars["range"] != "D7" {
		t.Fatalf("variables %v", vars)
	}
	query, _ := req.Body["query"].(string)
	for _, part := range []string{"storageMeasuredAt", "storage(range: $range)", "traffic(range: $range)", "responses(range: $range)"} {
		if !strings.Contains(query, part) {
			t.Fatalf("query misses %s: %s", part, query)
		}
	}
	for _, want := range []string{"2.0 KiB (latest)", "2.0 MiB", "1,234", "egress_bytes", "responses_4xx"} {
		if !strings.Contains(out, want) {
			t.Fatalf("stdout misses %q:\n%s", want, out)
		}
	}
}

func TestBucketMetricsOnlyRequestedParts(t *testing.T) {
	_, reqs, err := run(t, "s3", "bucket", "metrics", bucketUUID, "--part", "traffic", "--json")
	if err != nil {
		t.Fatal(err)
	}
	query, _ := last(reqs).Body["query"].(string)
	if !strings.Contains(query, "traffic(range: $range)") || strings.Contains(query, "storage") || strings.Contains(query, "responses") {
		t.Fatalf("query %s", query)
	}
	if vars, _ := last(reqs).Body["variables"].(map[string]interface{}); vars["range"] != "H24" {
		t.Fatalf("default range %v", vars["range"])
	}
}

func TestBucketMetricsRejectsBadFlags(t *testing.T) {
	if _, _, err := run(t, "s3", "bucket", "metrics", "photos", "--range", "2d"); err == nil || !strings.Contains(err.Error(), "invalid range") {
		t.Fatalf("err %v", err)
	}
	if _, _, err := run(t, "s3", "bucket", "metrics", "photos", "--part", "latency"); err == nil || !strings.Contains(err.Error(), "unknown part") {
		t.Fatalf("err %v", err)
	}
}

func TestBucketCreateObjectLock(t *testing.T) {
	_, reqs, err := run(t, "s3", "bucket", "create", "veeam", "--tier", "ia", "--object-lock",
		"--lock-mode", "governance", "--lock-days", "30", "--accept-object-lock-terms")
	if err != nil {
		t.Fatal(err)
	}
	b := last(reqs).Body
	if b["object_lock"] != true || b["accept_object_lock_terms"] != true || b["versioning"] != true {
		t.Fatalf("body %v", b)
	}
	rule, _ := b["object_lock_default"].(map[string]interface{})
	if rule["mode"] != "governance" || rule["days"] != float64(30) {
		t.Fatalf("object_lock_default %v", b["object_lock_default"])
	}
	if _, ok := rule["years"]; ok {
		t.Fatalf("years sent with --lock-days: %v", rule)
	}

	// Without a default retention.
	_, reqs, err = run(t, "s3", "bucket", "create", "veeam", "--tier", "ia", "--object-lock", "--accept-object-lock-terms")
	if err != nil {
		t.Fatal(err)
	}
	if b := last(reqs).Body; b["object_lock"] != true || b["versioning"] != true {
		t.Fatalf("body %v", b)
	} else if _, ok := b["object_lock_default"]; ok {
		t.Fatalf("object_lock_default sent without --lock-mode: %v", b)
	}

	// No lock fields on a normal bucket.
	_, reqs, _ = run(t, "s3", "bucket", "create", "photos", "--tier", "ia")
	if _, ok := last(reqs).Body["object_lock"]; ok {
		t.Fatalf("object_lock sent without --object-lock: %v", last(reqs).Body)
	}
}

func TestBucketCreateComplianceNeedsYes(t *testing.T) {
	base := []string{"s3", "bucket", "create", "archive", "--tier", "ia", "--object-lock",
		"--lock-mode", "compliance", "--lock-years", "7", "--accept-object-lock-terms"}
	// Without --yes it asks (or refuses without a terminal) and sends nothing.
	if _, reqs, _ := run(t, base...); len(reqs) != 0 {
		t.Fatalf("sent %d requests without --yes", len(reqs))
	}
	_, reqs, err := run(t, append(base, "--yes")...)
	if err != nil {
		t.Fatal(err)
	}
	rule, _ := last(reqs).Body["object_lock_default"].(map[string]interface{})
	if rule["mode"] != "compliance" || rule["years"] != float64(7) {
		t.Fatalf("object_lock_default %v", rule)
	}
}

func TestBucketCreateObjectLockRejectsBadInput(t *testing.T) {
	for _, args := range [][]string{
		{"--object-lock"}, // terms not accepted
		{"--object-lock", "--accept-object-lock-terms", "--versioning=false"},
		{"--lock-mode", "governance", "--lock-days", "3"}, // no --object-lock
		{"--object-lock", "--accept-object-lock-terms", "--lock-mode", "strict", "--lock-days", "3"},
		{"--object-lock", "--accept-object-lock-terms", "--lock-mode", "governance"},
		{"--object-lock", "--accept-object-lock-terms", "--lock-mode", "governance", "--lock-days", "0"},
		{"--object-lock", "--accept-object-lock-terms", "--lock-mode", "governance", "--lock-days", "3", "--lock-years", "1"},
	} {
		base := []string{"s3", "bucket", "create", "veeam", "--tier", "ia"}
		_, reqs, err := run(t, append(base, args...)...)
		if err == nil {
			t.Errorf("%v: expected an error", args)
		}
		if len(reqs) != 0 {
			t.Errorf("%v: sent %d requests before validating", args, len(reqs))
		}
	}
}

func TestBucketObjectLockSet(t *testing.T) {
	_, reqs, err := run(t, "s3", "bucket", "object-lock", "set", "photos", "--mode", "governance", "--years", "1", "--accept-object-lock-terms")
	if err != nil {
		t.Fatal(err)
	}
	req := last(reqs)
	if req.Method != http.MethodPut || req.Path != "/object-storage/buckets/"+bucketUUID+"/object-lock" {
		t.Fatalf("got %+v", req)
	}
	rule, _ := req.Body["default_retention"].(map[string]interface{})
	if rule["mode"] != "governance" || rule["years"] != float64(1) || req.Body["accept_object_lock_terms"] != true {
		t.Fatalf("body %v", req.Body)
	}

	_, reqs, err = run(t, "s3", "bucket", "object-lock", "set", bucketUUID, "--remove")
	if err != nil {
		t.Fatal(err)
	}
	req = last(reqs)
	if v, ok := req.Body["default_retention"]; !ok || v != nil || req.Body["accept_object_lock_terms"] != false {
		t.Fatalf("body %v", req.Body)
	}

	for _, args := range [][]string{
		{},
		{"--remove", "--mode", "governance"},
		{"--mode", "governance", "--days", "1", "--years", "1"},
	} {
		_, reqs, err := run(t, append([]string{"s3", "bucket", "object-lock", "set", bucketUUID}, args...)...)
		if err == nil || len(reqs) != 0 {
			t.Errorf("%v: expected an error and no request, got %v, %d requests", args, err, len(reqs))
		}
	}
	// Compliance without --yes asks (or refuses without a terminal) and sends nothing.
	if _, reqs, _ := run(t, "s3", "bucket", "object-lock", "set", bucketUUID, "--mode", "compliance", "--days", "10"); len(reqs) != 0 {
		t.Fatalf("sent %d requests without --yes", len(reqs))
	}
	_, reqs, err = run(t, "s3", "bucket", "object-lock", "set", bucketUUID, "--mode", "compliance", "--days", "10", "--accept-object-lock-terms", "--yes")
	if err != nil || last(reqs).Method != http.MethodPut {
		t.Fatalf("compliance with --yes: %v %+v", err, reqs)
	}
}

func TestBucketDeleteBypassGovernance(t *testing.T) {
	_, reqs, err := run(t, "s3", "bucket", "delete", bucketUUID, "--purge", "--bypass-governance", "--force")
	if err != nil {
		t.Fatal(err)
	}
	if got := last(reqs).Path; got != "/object-storage/buckets/"+bucketUUID+"?force=true&bypass_governance=true" {
		t.Fatalf("path %s", got)
	}
	if _, reqs, err := run(t, "s3", "bucket", "delete", bucketUUID, "--bypass-governance", "--force"); err == nil || len(reqs) != 0 {
		t.Fatal("expected an error for --bypass-governance without --purge")
	}
}

func TestKeyCreateBypassGovernance(t *testing.T) {
	_, reqs, err := run(t, "s3", "key", "create", "--name", "veeam", "--tier", "ia", "--bypass-governance", "--json")
	if err != nil {
		t.Fatal(err)
	}
	if last(reqs).Body["bypass_governance"] != true {
		t.Fatalf("body %v", last(reqs).Body)
	}
	_, reqs, _ = run(t, "s3", "key", "create", "--name", "web", "--tier", "ia", "--json")
	if _, ok := last(reqs).Body["bypass_governance"]; ok {
		t.Fatalf("bypass_governance sent without the flag: %v", last(reqs).Body)
	}
	if _, reqs, err := run(t, "s3", "key", "create", "--name", "ro", "--tier", "ia", "--permission", "read_only", "--bypass-governance"); err == nil || len(reqs) != 0 {
		t.Fatal("expected an error for a read_only key with --bypass-governance")
	}
}

func TestLockFormatting(t *testing.T) {
	d, y := 30, 7
	cases := []struct {
		l          objectLock
		col, field string
	}{
		{objectLock{}, "-", "off"},
		{objectLock{Enabled: true}, "on", "on, no default retention"},
		{objectLock{Enabled: true, DefaultRetention: &lockRetention{Mode: "governance", Days: &d}}, "governance 30d", "on, default retention governance 30d"},
		{objectLock{Enabled: true, DefaultRetention: &lockRetention{Mode: "compliance", Years: &y}}, "compliance 7y", "on, default retention compliance 7y"},
	}
	for _, c := range cases {
		if got := formatLockColumn(c.l); got != c.col {
			t.Errorf("formatLockColumn(%+v) = %q, want %q", c.l, got, c.col)
		}
		if got := formatLock(c.l); got != c.field {
			t.Errorf("formatLock(%+v) = %q, want %q", c.l, got, c.field)
		}
	}
}

func TestBucketListLockColumn(t *testing.T) {
	out, _, err := run(t, "s3", "bucket", "list")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Lock") || !strings.Contains(out, "governance 30d") {
		t.Fatalf("output %s", out)
	}
}
