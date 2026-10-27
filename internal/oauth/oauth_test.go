package oauth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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
	preflights    int // authorize calls without PKCE (nothing stored)
	authorizes    int // authorize calls with PKCE (consent requests created)
	codes         map[string]pending
	refresh       string // current refresh token
	previous      string // last rotated refresh token
	revoked       bool
	refreshes     int32
	revocations   []string
	resource      string
}

type pending struct {
	clientID, redirect, challenge, resource string
}

// fakeResource is the resource the fake API advertises. Set in newFakeServer
// to the server's own URL, as an API that accepts browser-login tokens does.
var fakeResource string

func newFakeServer(t *testing.T) *fakeServer {
	f := &fakeServer{t: t, clients: map[string][]string{}, codes: map[string]pending{}}
	mux := http.NewServeMux()
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	issuer := f.srv.URL
	fakeResource = f.srv.URL
	f.resource = fakeResource

	mux.HandleFunc("/.well-known/oauth-protected-resource", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]interface{}{"resource": f.resource, "authorization_servers": []string{issuer}})
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
		if q.Get("code_challenge") == "" {
			// Like identity: after client validation, errors redirect to the client.
			f.preflights++
			http.Redirect(w, r, q.Get("redirect_uri")+"?"+url.Values{
				"error": {"invalid_request"}, "error_description": {"PKCE is required."}, "state": {q.Get("state")},
			}.Encode(), http.StatusFound)
			return
		}
		if q.Get("scope") != "vps:read vps:write" || q.Get("resource") != fakeResource ||
			q.Get("code_challenge_method") != "S256" || q.Get("response_type") != "code" {
			t.Errorf("unexpected authorize query: %v", q)
		}
		// Consent is simulated: the browser is redirected straight to the callback.
		f.authorizes++
		code := fmt.Sprintf("code-%d", len(f.codes)+1)
		f.codes[code] = pending{q.Get("client_id"), q.Get("redirect_uri"), q.Get("code_challenge"), q.Get("resource")}
		http.Redirect(w, r, q.Get("redirect_uri")+"?"+url.Values{"code": {code}, "state": {q.Get("state")}}.Encode(), http.StatusFound)
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

// openAuthorize simulates the browser: it opens the authorize URL and follows
// the redirect to the loopback callback.
func openAuthorize(t *testing.T, authURL string) *http.Response {
	t.Helper()
	resp, err := http.Get(authURL)
	if err != nil {
		t.Fatalf("callback request failed: %v", err)
	}
	return resp
}

func TestLoginBrowserFlowRegistersOnceAndReusesClient(t *testing.T) {
	f := newFakeServer(t)
	openBrowser = func(u string) error {
		go func() { openAuthorize(t, u).Body.Close() }()
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
		go func() { openAuthorize(t, u).Body.Close() }()
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

	// The authorize URL is printed; "open" it without following the redirect and
	// paste the callback URL it sends the browser to.
	out := &syncBuffer{}
	pr, pw := ioPipe()
	go func() {
		for i := 0; i < 200; i++ {
			if s := out.String(); strings.Contains(s, "paste it here") {
				line := s[strings.Index(s, "http"):]
				line = strings.TrimSpace(line[:strings.Index(line, "\n")])
				if !strings.HasPrefix(line, f.srv.URL+"/oauth/authorize?") {
					t.Errorf("printed URL is not the authorize URL: %s", line)
				}
				noFollow := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
				resp, err := noFollow.Get(line)
				if err != nil {
					t.Error(err)
					return
				}
				resp.Body.Close()
				fmt.Fprintln(pw, resp.Header.Get("Location"))
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
	if f.preflights != 1 || f.authorizes != 1 {
		t.Fatalf("expected 1 preflight and 1 authorization, got %d and %d", f.preflights, f.authorizes)
	}
}

func TestLoginRefusedWhenAPIDoesNotAcceptTheTokens(t *testing.T) {
	f := newFakeServer(t)
	f.resource = "https://mcp.example.test/mcp" // tokens would only be valid for the MCP server
	openBrowser = func(string) error { t.Error("must not open a browser"); return nil }
	t.Cleanup(func() { openBrowser = func(string) error { return nil } })

	_, err := Login(context.Background(), LoginOptions{APIURL: f.srv.URL, Out: &bytes.Buffer{}})
	if !errors.Is(err, ErrBrowserLoginUnsupported) {
		t.Fatalf("expected ErrBrowserLoginUnsupported, got %v", err)
	}
	if f.registrations != 0 {
		t.Fatal("no client should be registered")
	}
}

func TestResourceIsAPI(t *testing.T) {
	for _, c := range []struct {
		resource, api string
		want          bool
	}{
		{"https://api.cubepath.com", "https://api.cubepath.com", true},
		{"https://api.cubepath.com/", "https://api.cubepath.com", true},
		{"https://mcp.cubepath.com/mcp", "https://api.cubepath.com", false},
		{"https://api.cubepath.com/mcp", "https://api.cubepath.com", false},
		{"http://api.cubepath.com", "https://api.cubepath.com", false},
	} {
		if got := ResourceIsAPI(c.resource, c.api); got != c.want {
			t.Errorf("ResourceIsAPI(%q, %q) = %v", c.resource, c.api, got)
		}
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

func TestLoginOpensTheAuthorizeURLInTheBrowser(t *testing.T) {
	f := newFakeServer(t)
	var opened string
	openBrowser = func(u string) error {
		opened = u
		go func() { openAuthorize(t, u).Body.Close() }()
		return nil
	}
	t.Cleanup(func() { openBrowser = func(string) error { return nil } })

	out := &bytes.Buffer{}
	if _, err := Login(context.Background(), LoginOptions{APIURL: f.srv.URL, Out: out}); err != nil {
		t.Fatalf("login: %v", err)
	}
	u, err := url.Parse(opened)
	if err != nil {
		t.Fatal(err)
	}
	if got := u.Scheme + "://" + u.Host + u.Path; got != f.srv.URL+"/oauth/authorize" {
		t.Fatalf("browser opened %q, want the authorization endpoint", got)
	}
	q := u.Query()
	for k, want := range map[string]string{
		"response_type": "code", "client_id": "client-1", "code_challenge_method": "S256",
		"scope": "vps:read vps:write", "resource": fakeResource,
	} {
		if q.Get(k) != want {
			t.Errorf("%s = %q, want %q", k, q.Get(k), want)
		}
	}
	if !strings.HasPrefix(q.Get("redirect_uri"), "http://127.0.0.1:") || q.Get("state") == "" || q.Get("code_challenge") == "" {
		t.Errorf("missing redirect_uri/state/code_challenge: %v", q)
	}
	if !strings.Contains(out.String(), opened) {
		t.Error("the authorize URL should also be printed in case the browser does not open")
	}
	// The preflight must not create a consent request: only the browser does.
	if f.preflights != 1 || f.authorizes != 1 {
		t.Fatalf("expected 1 preflight and 1 authorization, got %d and %d", f.preflights, f.authorizes)
	}
}

func TestLoginBrowserCallbackErrors(t *testing.T) {
	f := newFakeServer(t)
	for name, tc := range map[string]struct {
		query string // callback query, with %s for the real state
		want  string
	}{
		"denied":   {"error=access_denied&state=%s", "denied"},
		"mismatch": {"code=x&state=other", "state mismatch"},
	} {
		t.Run(name, func(t *testing.T) {
			openBrowser = func(u string) error {
				go func() {
					au, _ := url.Parse(u)
					cb := au.Query().Get("redirect_uri") + "?" + strings.Replace(tc.query, "%s", au.Query().Get("state"), 1)
					if resp, err := http.Get(cb); err == nil {
						resp.Body.Close()
					}
				}()
				return nil
			}
			t.Cleanup(func() { openBrowser = func(string) error { return nil } })
			_, err := Login(context.Background(), LoginOptions{APIURL: f.srv.URL, Out: &bytes.Buffer{}})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected error containing %q, got %v", tc.want, err)
			}
		})
	}
}

func TestPreflight(t *testing.T) {
	const redirect = "http://127.0.0.1:38271/callback"
	cases := map[string]struct {
		handler  http.HandlerFunc
		wantCode string // OAuth error code expected, "" for success
		wantErr  bool
	}{
		"client ok": {handler: func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, redirect+"?error=invalid_request&state=s", http.StatusFound)
		}},
		"invalid client": {handler: func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, 400, map[string]string{"error": "invalid_client"})
		}, wantCode: "invalid_client"},
		"other oauth error": {handler: func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, redirect+"?error=invalid_scope&state=s", http.StatusFound)
		}, wantCode: "invalid_scope"},
		"redirect elsewhere": {handler: func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "https://evil.example/consent", http.StatusFound)
		}, wantErr: true},
		"redirect to us without error": {handler: func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, redirect+"?code=x&state=s", http.StatusFound)
		}, wantErr: true},
		"ok page": {handler: func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(200)
		}, wantErr: true},
		"server error": {handler: func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(500)
		}, wantErr: true},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			var query url.Values
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				query = r.URL.Query()
				tc.handler(w, r)
			}))
			defer srv.Close()
			meta := &ServerMetadata{AuthorizationEndpoint: srv.URL + "/oauth/authorize", ScopesSupported: []string{"vps:read"}}
			err := preflight(context.Background(), meta, fakeResource, "cid", redirect, "s")
			if query.Get("code_challenge") != "" || query.Get("code_challenge_method") != "" {
				t.Error("the preflight must not send a PKCE challenge")
			}
			if query.Get("client_id") != "cid" || query.Get("redirect_uri") != redirect || query.Get("state") != "s" || query.Get("resource") != fakeResource {
				t.Errorf("unexpected preflight query: %v", query)
			}
			var oerr *Error
			switch {
			case tc.wantCode != "":
				if !errors.As(err, &oerr) || oerr.Code != tc.wantCode {
					t.Fatalf("expected OAuth error %q, got %v", tc.wantCode, err)
				}
			case tc.wantErr:
				if err == nil {
					t.Fatal("expected an error")
				}
			case err != nil:
				t.Fatalf("expected success, got %v", err)
			}
		})
	}
}
