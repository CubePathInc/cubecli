package oauth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// fakeServer serves the API metadata and a strict authorization server: exact
// redirect_uri match, PKCE S256, and refresh token rotation with reuse
// detection that revokes the grant.
type fakeServer struct {
	t   *testing.T
	srv *httptest.Server

	mu            sync.Mutex
	clients       map[string][]string // client_id -> redirect_uris
	registrations int
	codes         map[string]pending
	refresh       string // current refresh token
	previous      string // last rotated refresh token
	revoked       bool
	refreshes     int32
	revocations   []string
}

type pending struct {
	clientID, redirect, challenge, resource string
}

const fakeResource = "https://mcp.example.test/mcp"

func newFakeServer(t *testing.T) *fakeServer {
	f := &fakeServer{t: t, clients: map[string][]string{}, codes: map[string]pending{}}
	mux := http.NewServeMux()
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	issuer := f.srv.URL

	mux.HandleFunc("/.well-known/oauth-protected-resource", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]interface{}{"resource": fakeResource, "authorization_servers": []string{issuer}})
	})
	mux.HandleFunc("/.well-known/oauth-authorization-server", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]interface{}{
			"issuer":                 issuer,
			"authorization_endpoint": issuer + "/oauth/authorize",
			"token_endpoint":         issuer + "/oauth/token",
			"registration_endpoint":  issuer + "/oauth/register",
			"revocation_endpoint":    issuer + "/oauth/revoke",
			"scopes_supported":       []string{"vps:read", "vps:write"},
		})
	})
	mux.HandleFunc("/oauth/register", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			RedirectURIs []string `json:"redirect_uris"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		f.mu.Lock()
		f.registrations++
		id := fmt.Sprintf("client-%d", f.registrations)
		f.clients[id] = body.RedirectURIs
		f.mu.Unlock()
		writeJSON(w, 201, map[string]interface{}{"client_id": id})
	})
	mux.HandleFunc("/oauth/authorize", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		f.mu.Lock()
		defer f.mu.Unlock()
		uris, ok := f.clients[q.Get("client_id")]
		if !ok {
			writeJSON(w, 400, map[string]string{"error": "invalid_client", "error_description": "Unknown client_id."})
			return
		}
		if !contains(uris, q.Get("redirect_uri")) {
			writeJSON(w, 400, map[string]string{"error": "invalid_request"})
			return
		}
		if q.Get("scope") != "vps:read vps:write" || q.Get("resource") != fakeResource {
			t.Errorf("unexpected scope/resource: %q %q", q.Get("scope"), q.Get("resource"))
		}
		// Consent is simulated: the "consent page" URL carries what the dashboard
		// would eventually redirect to.
		code := fmt.Sprintf("code-%d", len(f.codes)+1)
		f.codes[code] = pending{q.Get("client_id"), q.Get("redirect_uri"), q.Get("code_challenge"), q.Get("resource")}
		back := q.Get("redirect_uri") + "?" + url.Values{"code": {code}, "state": {q.Get("state")}}.Encode()
		http.Redirect(w, r, issuer+"/consent?back="+url.QueryEscape(back), http.StatusFound)
	})
	mux.HandleFunc("/oauth/token", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		f.mu.Lock()
		defer f.mu.Unlock()
		switch r.Form.Get("grant_type") {
		case "authorization_code":
			p, ok := f.codes[r.Form.Get("code")]
			delete(f.codes, r.Form.Get("code"))
			challenge := s256(r.Form.Get("code_verifier"))
			if !ok || p.redirect != r.Form.Get("redirect_uri") || p.clientID != r.Form.Get("client_id") ||
				p.challenge != challenge || p.resource != r.Form.Get("resource") {
				writeJSON(w, 400, map[string]string{"error": "invalid_grant"})
				return
			}
		case "refresh_token":
			atomic.AddInt32(&f.refreshes, 1)
			presented := r.Form.Get("refresh_token")
			if presented == f.previous && presented != "" {
				f.revoked = true
			}
			if f.revoked || presented != f.refresh {
				writeJSON(w, 400, map[string]string{"error": "invalid_grant"})
				return
			}
			time.Sleep(50 * time.Millisecond) // widen the race window
		default:
			writeJSON(w, 400, map[string]string{"error": "unsupported_grant_type"})
			return
		}
		f.previous = f.refresh
		f.refresh = fmt.Sprintf("rt-%d", time.Now().UnixNano())
		writeJSON(w, 200, map[string]interface{}{
			"access_token": "at-" + f.refresh, "token_type": "Bearer", "expires_in": 3600,
			"refresh_token": f.refresh, "scope": "vps:read",
		})
	})
	mux.HandleFunc("/oauth/revoke", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		f.mu.Lock()
		f.revocations = append(f.revocations, r.Form.Get("token"))
		f.mu.Unlock()
		w.WriteHeader(200)
	})
	return f
}

type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func ioPipe() (*io.PipeReader, *io.PipeWriter) { return io.Pipe() }

func contains(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// followConsent simulates the user approving in the browser: it takes the consent
// URL and delivers the final redirect to the loopback callback.
func followConsent(t *testing.T, consentURL string) *http.Response {
	t.Helper()
	u, err := url.Parse(consentURL)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.Get(u.Query().Get("back"))
	if err != nil {
		t.Fatalf("callback request failed: %v", err)
	}
	return resp
}

func TestLoginBrowserFlowRegistersOnceAndReusesClient(t *testing.T) {
	f := newFakeServer(t)
	openBrowser = func(u string) error {
		go func() { followConsent(t, u).Body.Close() }()
		return nil
	}
	t.Cleanup(func() { openBrowser = func(string) error { return nil } })

	stored := ""
	opts := LoginOptions{
		APIURL:           f.srv.URL,
		LookupClient:     func(string) string { return stored },
		Out:              &bytes.Buffer{},
		ClientRegistered: func(_, id string) { stored = id },
	}
	res, err := Login(context.Background(), opts)
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if res.ClientID != "client-1" || stored != "client-1" || res.Token.RefreshToken == "" || res.Resource != fakeResource {
		t.Fatalf("unexpected result: %+v stored=%q", res, stored)
	}

	if _, err := Login(context.Background(), opts); err != nil {
		t.Fatalf("second login: %v", err)
	}
	if f.registrations != 1 {
		t.Fatalf("expected the client to be reused, got %d registrations", f.registrations)
	}
}

func TestLoginReRegistersWhenStoredClientIsGone(t *testing.T) {
	f := newFakeServer(t)
	openBrowser = func(u string) error {
		go func() { followConsent(t, u).Body.Close() }()
		return nil
	}
	t.Cleanup(func() { openBrowser = func(string) error { return nil } })

	var registered string
	res, err := Login(context.Background(), LoginOptions{
		APIURL:           f.srv.URL,
		LookupClient:     func(string) string { return "pruned-client" },
		Out:              &bytes.Buffer{},
		ClientRegistered: func(_, id string) { registered = id },
	})
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if res.ClientID != "client-1" || registered != "client-1" {
		t.Fatalf("expected a fresh registration, got %q / %q", res.ClientID, registered)
	}
}

func TestLoginNoBrowserPastedURL(t *testing.T) {
	f := newFakeServer(t)
	openBrowser = func(string) error { t.Error("must not open a browser"); return nil }
	t.Cleanup(func() { openBrowser = func(string) error { return nil } })

	// The consent URL is printed; capture it and "paste" the final redirect.
	out := &syncBuffer{}
	pr, pw := ioPipe()
	go func() {
		for i := 0; i < 200; i++ {
			if s := out.String(); strings.Contains(s, "paste it here") {
				line := s[strings.Index(s, "http"):]
				line = strings.TrimSpace(line[:strings.Index(line, "\n")])
				back, _ := url.Parse(line)
				fmt.Fprintln(pw, back.Query().Get("back"))
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}()
	res, err := Login(context.Background(), LoginOptions{APIURL: f.srv.URL, NoBrowser: true, In: pr, Out: out})
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if res.Token.AccessToken == "" {
		t.Fatal("no token")
	}
}

func TestParseCallbackRejectsWrongState(t *testing.T) {
	if _, err := parseCallback(url.Values{"code": {"x"}, "state": {"evil"}}, "good"); err == nil {
		t.Fatal("expected state mismatch")
	}
	if _, err := parseCallback(url.Values{"error": {"access_denied"}, "state": {"good"}}, "good"); err == nil ||
		!strings.Contains(err.Error(), "denied") {
		t.Fatalf("expected access denied, got %v", err)
	}
	if code, err := parseCallback(url.Values{"code": {"abc"}, "state": {"good"}}, "good"); err != nil || code != "abc" {
		t.Fatalf("got %q %v", code, err)
	}
}

func TestRequireSecureURL(t *testing.T) {
	for _, ok := range []string{"https://identity.cubepath.com", "http://127.0.0.1:8004", "http://localhost:8004"} {
		if err := requireSecureURL(ok); err != nil {
			t.Errorf("%s: %v", ok, err)
		}
	}
	for _, bad := range []string{"http://identity.cubepath.com", "http://127.0.0.1.evil.test", "ftp://x"} {
		if err := requireSecureURL(bad); err == nil {
			t.Errorf("%s: expected refusal", bad)
		}
	}
}
