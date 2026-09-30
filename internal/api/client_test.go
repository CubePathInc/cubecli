package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

type fakeSource struct {
	token     string
	refreshed int
}

func (f *fakeSource) Token() (string, error) { return f.token, nil }

func (f *fakeSource) Refresh(stale string) (string, error) {
	f.refreshed++
	f.token = "fresh"
	return f.token, nil
}

func TestUnauthorizedRefreshesOnceAndRetries(t *testing.T) {
	var bodies []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 64)
		n, _ := r.Body.Read(buf)
		bodies = append(bodies, string(buf[:n]))
		if r.Header.Get("Authorization") != "Bearer fresh" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"detail":"The access token has expired."}`))
			return
		}
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	src := &fakeSource{token: "stale"}
	out, err := NewClientWithAuth(srv.URL, src).Post("/x", map[string]int{"a": 1})
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != `{"ok":true}` || src.refreshed != 1 {
		t.Fatalf("out=%s refreshed=%d", out, src.refreshed)
	}
	if len(bodies) != 2 || bodies[0] != bodies[1] || bodies[1] != `{"a":1}` {
		t.Fatalf("body not replayed: %q", bodies)
	}
}

func TestStaticTokenKeepsUnauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"detail":"Invalid API token"}`))
	}))
	defer srv.Close()

	_, err := NewClient(srv.URL, "tok").Get("/x")
	apiErr, ok := err.(*APIError)
	if !ok || apiErr.StatusCode != 401 || apiErr.Detail != "Invalid API token" {
		t.Fatalf("got %v", err)
	}
}

func TestPostFileSendsMultipart(t *testing.T) {
	var ctype, field, name, content string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctype = r.Header.Get("Content-Type")
		if f, h, err := r.FormFile("file"); err == nil {
			buf := make([]byte, 64)
			n, _ := f.Read(buf)
			field, name, content = "file", h.Filename, string(buf[:n])
		}
		_, _ = w.Write([]byte(`{"imported":1}`))
	}))
	defer srv.Close()

	out, err := NewClient(srv.URL, "tok").PostFile("/dns/zones/z/import", "file", "z.zone", []byte("www A 1.2.3.4"))
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != `{"imported":1}` || field != "file" || name != "z.zone" || content != "www A 1.2.3.4" {
		t.Fatalf("out=%s ctype=%s field=%s name=%s content=%q", out, ctype, field, name, content)
	}
}
