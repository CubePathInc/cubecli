package s3sign

import (
	"net/url"
	"strings"
	"testing"
	"time"
)

// The example of "Authenticating Requests: Using Query Parameters (AWS Signature Version 4)"
// in the Amazon S3 API reference.
func TestPresignGetAWSExample(t *testing.T) {
	got, err := PresignGet(Request{
		Endpoint:  "https://examplebucket.s3.amazonaws.com",
		Region:    "us-east-1",
		Path:      "/test.txt",
		AccessKey: "AKIAIOSFODNN7EXAMPLE",
		SecretKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
		Expires:   86400 * time.Second,
		Now:       time.Date(2013, 5, 24, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	want := "https://examplebucket.s3.amazonaws.com/test.txt?X-Amz-Algorithm=AWS4-HMAC-SHA256" +
		"&X-Amz-Credential=AKIAIOSFODNN7EXAMPLE%2F20130524%2Fus-east-1%2Fs3%2Faws4_request" +
		"&X-Amz-Date=20130524T000000Z&X-Amz-Expires=86400&X-Amz-SignedHeaders=host" +
		"&X-Amz-Signature=aeeed9bbccd4d02ee5c0109b86d86835f995330da4c265957d157751f604d404"
	if got != want {
		t.Fatalf("got  %s\nwant %s", got, want)
	}
}

func TestPresignGetPathStyleUTF8(t *testing.T) {
	got, err := PresignGet(Request{
		Endpoint:  "https://eu.cubestorage.io",
		Region:    "eu",
		Path:      PathStyle("photos", "informes/año 2026+final.pdf"),
		AccessKey: "CP7Q2M9XK4B1N8R5T3W6",
		SecretKey: "s3cr3t",
		Expires:   time.Hour,
		Now:       time.Date(2026, 10, 2, 10, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(got)
	if err != nil {
		t.Fatal(err)
	}
	if u.EscapedPath() != "/photos/informes/a%C3%B1o%202026%2Bfinal.pdf" {
		t.Fatalf("path %q", u.EscapedPath())
	}
	if u.Path != "/photos/informes/año 2026+final.pdf" {
		t.Fatalf("decoded path %q", u.Path)
	}
	q := u.Query()
	if q.Get("X-Amz-Credential") != "CP7Q2M9XK4B1N8R5T3W6/20261002/eu/s3/aws4_request" || q.Get("X-Amz-Expires") != "3600" {
		t.Fatalf("query %v", q)
	}
	if len(q.Get("X-Amz-Signature")) != 64 {
		t.Fatalf("signature %q", q.Get("X-Amz-Signature"))
	}
	// Signing the same request twice gives the same URL
	again, _ := PresignGet(Request{
		Endpoint: "https://eu.cubestorage.io", Region: "eu", Path: PathStyle("photos", "informes/año 2026+final.pdf"),
		AccessKey: "CP7Q2M9XK4B1N8R5T3W6", SecretKey: "s3cr3t", Expires: time.Hour,
		Now: time.Date(2026, 10, 2, 10, 0, 0, 0, time.UTC),
	})
	if again != got {
		t.Fatal("not deterministic")
	}
}

func TestPresignGetKeepsNonDefaultPort(t *testing.T) {
	got, err := PresignGet(Request{
		Endpoint: "http://localhost:9000", Region: "eu", Path: "/b/k", AccessKey: "A", SecretKey: "S",
		Expires: time.Minute, Now: time.Now(),
	})
	if err != nil || !strings.HasPrefix(got, "http://localhost:9000/b/k?") {
		t.Fatalf("%v %s", err, got)
	}
}

func TestPresignGetRejectsBadInput(t *testing.T) {
	for _, endpoint := range []string{"eu.cubestorage.io", "ftp://x", ""} {
		if _, err := PresignGet(Request{Endpoint: endpoint, Region: "eu", Path: "/b/k", Expires: time.Minute}); err == nil {
			t.Errorf("endpoint %q accepted", endpoint)
		}
	}
	if _, err := PresignGet(Request{Endpoint: "https://x", Region: "eu", Path: "/b/k"}); err == nil {
		t.Error("zero expiry accepted")
	}
}
