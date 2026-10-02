package objectstorage

import (
	"encoding/json"
	"net/url"
	"strings"
	"testing"
	"time"
)

func withCredentials(t *testing.T) {
	t.Helper()
	t.Setenv("AWS_ACCESS_KEY_ID", "CP7Q2M9XK4B1N8R5T3W6")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "s3cr3t")
	old := presignNow
	presignNow = func() time.Time { return time.Date(2026, 10, 2, 10, 0, 0, 0, time.UTC) }
	t.Cleanup(func() { presignNow = old })
}

func TestPresignUsesTheBucketsTier(t *testing.T) {
	withCredentials(t)
	out, reqs, err := run(t, "s3", "presign", "photos/a/informe 1.pdf", "--expires", "6h")
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(strings.TrimSpace(out))
	if err != nil {
		t.Fatal(err)
	}
	if u.Host != "eu.cubestorage.io" || u.EscapedPath() != "/photos/a/informe%201.pdf" {
		t.Fatalf("url %s", out)
	}
	q := u.Query()
	if q.Get("X-Amz-Expires") != "21600" || q.Get("X-Amz-Credential") != "CP7Q2M9XK4B1N8R5T3W6/20261002/eu/s3/aws4_request" {
		t.Fatalf("query %v", q)
	}
	for _, r := range reqs {
		if r.Method != "GET" {
			t.Fatalf("unexpected %s %s", r.Method, r.Path)
		}
		if strings.Contains(r.Path, "s3cr3t") {
			t.Fatal("secret sent to the API")
		}
	}
}

func TestPresignJSON(t *testing.T) {
	withCredentials(t)
	out, _, err := run(t, "s3", "presign", "photos/a.txt", "--tier", "ia", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]string
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	if got["expires_at"] != "2026-10-02T11:00:00Z" || !strings.HasPrefix(got["url"], "https://eu.cubestorage.io/photos/a.txt?") {
		t.Fatalf("%v", got)
	}
}

func TestPresignWithEndpointSendsNothing(t *testing.T) {
	withCredentials(t)
	out, reqs, err := run(t, "s3", "presign", "photos/a.txt", "--endpoint", "http://localhost:9000")
	if err != nil {
		t.Fatal(err)
	}
	if len(reqs) != 0 {
		t.Fatalf("requests sent: %v", reqs)
	}
	if !strings.HasPrefix(out, "http://localhost:9000/photos/a.txt?") || !strings.Contains(out, "%2Feu%2Fs3%2F") {
		t.Fatalf("url %s", out)
	}
}

func TestPresignRefusesMoreThan24Hours(t *testing.T) {
	withCredentials(t)
	_, _, err := run(t, "s3", "presign", "photos/a.txt", "--expires", "25h", "--endpoint", "https://eu.cubestorage.io")
	if err == nil || err.Error() != "presigned URLs on CubePath Object Storage last at most 24 hours" {
		t.Fatalf("err %v", err)
	}
	if _, _, err := run(t, "s3", "presign", "photos/a.txt", "--expires", "24h", "--endpoint", "https://eu.cubestorage.io"); err != nil {
		t.Fatalf("24h refused: %v", err)
	}
}

func TestPresignBadRefsAndCredentials(t *testing.T) {
	withCredentials(t)
	for _, ref := range []string{"photos", "photos/", "/a.txt", "photos//a.txt", "photos/a//b", "photos/dir/"} {
		if _, _, err := run(t, "s3", "presign", ref, "--endpoint", "https://eu.cubestorage.io"); err == nil {
			t.Errorf("%q accepted", ref)
		}
	}
	t.Setenv("AWS_SECRET_ACCESS_KEY", "")
	if _, _, err := run(t, "s3", "presign", "photos/a.txt", "--endpoint", "https://eu.cubestorage.io"); err == nil {
		t.Error("missing secret accepted")
	}
}

func TestPresignUnknownBucketNeedsATier(t *testing.T) {
	withCredentials(t)
	// One tier only: it is used even for a bucket that is not listed
	if _, _, err := run(t, "s3", "presign", "elsewhere/a.txt"); err != nil {
		t.Fatalf("err %v", err)
	}
	if _, _, err := run(t, "s3", "presign", "photos/a.txt", "--tier", "standard"); err == nil {
		t.Fatal("unknown tier accepted")
	}
}
