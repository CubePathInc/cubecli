package oauth

import (
	"bufio"
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

const loginTimeout = 5 * time.Minute

// LoginOptions configures an interactive browser login.
type LoginOptions struct {
	APIURL string
	// LookupClient returns the client_id this machine previously registered on the
	// given issuer, or "". Empty, or unknown to the server, triggers a new
	// registration.
	LookupClient func(issuer string) string
	// NoBrowser prints the URL instead of opening it, and asks for the URL the
	// browser ended up on. For machines without a browser (SSH sessions).
	NoBrowser bool
	In        io.Reader
	Out       io.Writer
	// ClientRegistered is called when a new client is registered.
	ClientRegistered func(issuer, clientID string)
}

// LoginResult is the outcome of a successful login.
type LoginResult struct {
	Meta     *ServerMetadata
	Resource string
	ClientID string
	Token    *TokenResponse
}

// Login runs the authorization code + PKCE flow through the user's browser.
func Login(ctx context.Context, opts LoginOptions) (*LoginResult, error) {
	ctx, cancel := context.WithTimeout(ctx, loginTimeout)
	defer cancel()

	pr, meta, err := Discover(ctx, opts.APIURL)
	if err != nil {
		return nil, err
	}

	var ln net.Listener
	port := callbackPorts[0]
	if !opts.NoBrowser {
		ln, port, err = listenLoopback()
		if err != nil {
			return nil, err
		}
		defer ln.Close()
	}
	redirect := redirectURI(port)

	storedClient := ""
	if opts.LookupClient != nil {
		storedClient = opts.LookupClient(meta.Issuer)
	}
	clientID := storedClient
	register := func() error {
		id, err := Register(ctx, meta)
		if err != nil {
			return err
		}
		clientID = id
		if opts.ClientRegistered != nil {
			opts.ClientRegistered(meta.Issuer, id)
		}
		return nil
	}
	if clientID == "" {
		if err := register(); err != nil {
			return nil, err
		}
	}

	verifier, challenge := newPKCE()
	state := randomString(16)

	consentURL, err := startAuthorization(ctx, meta, pr.Resource, clientID, redirect, state, challenge)
	var oerr *Error
	if errors.As(err, &oerr) && oerr.Code == "invalid_client" && storedClient != "" {
		// The stored client no longer exists server side (pruned). Register again.
		if err := register(); err != nil {
			return nil, err
		}
		consentURL, err = startAuthorization(ctx, meta, pr.Resource, clientID, redirect, state, challenge)
	}
	if err != nil {
		return nil, err
	}

	var code string
	if opts.NoBrowser {
		code, err = pasteCallback(ctx, opts, consentURL, state)
	} else {
		code, err = browserCallback(ctx, opts, ln, consentURL, state)
	}
	if err != nil {
		return nil, err
	}

	tok, err := Exchange(ctx, meta.TokenEndpoint, clientID, code, redirect, verifier, pr.Resource)
	if err != nil {
		return nil, fmt.Errorf("token exchange failed: %w", err)
	}
	return &LoginResult{Meta: meta, Resource: pr.Resource, ClientID: clientID, Token: tok}, nil
}

// startAuthorization calls the authorization endpoint and returns the consent
// page it redirects to, so an unknown client is detected here rather than in
// the browser.
func startAuthorization(ctx context.Context, meta *ServerMetadata, resource, clientID, redirect, state, challenge string) (string, error) {
	q := url.Values{
		"response_type":         {"code"},
		"client_id":             {clientID},
		"redirect_uri":          {redirect},
		"state":                 {state},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
		"resource":              {resource},
	}
	// Request every scope; the user picks on the consent screen.
	if len(meta.ScopesSupported) > 0 {
		q.Set("scope", strings.Join(meta.ScopesSupported, " "))
	}
	authURL := meta.AuthorizationEndpoint + "?" + q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, authURL, nil)
	if err != nil {
		return "", err
	}
	noRedirect := *httpClient
	noRedirect.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := noRedirect.Do(req)
	if err != nil {
		return "", fmt.Errorf("connection error: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))

	if resp.StatusCode < 300 || resp.StatusCode >= 400 {
		return "", parseError(resp.StatusCode, body)
	}
	loc, err := resp.Location()
	if err != nil {
		return "", fmt.Errorf("authorization endpoint returned a redirect without location")
	}
	// Errors after client validation come back as a redirect to our own URI.
	if strings.HasPrefix(loc.String(), redirect) {
		if code := loc.Query().Get("error"); code != "" {
			return "", &Error{Code: code, Description: loc.Query().Get("error_description")}
		}
		return "", fmt.Errorf("unexpected redirect from the authorization endpoint")
	}
	if err := requireSecureURL(loc.String()); err != nil {
		return "", fmt.Errorf("refusing to open the consent page: %w", err)
	}
	return loc.String(), nil
}

func listenLoopback() (net.Listener, int, error) {
	for _, p := range callbackPorts {
		ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", p))
		if err == nil {
			return ln, p, nil
		}
	}
	return nil, 0, fmt.Errorf("no free local port for the login callback (tried %d-%d); retry, or use --no-browser",
		callbackPorts[0], callbackPorts[len(callbackPorts)-1])
}

type callbackResult struct {
	code string
	err  error
}

func browserCallback(ctx context.Context, opts LoginOptions, ln net.Listener, consentURL, state string) (string, error) {
	results := make(chan callbackResult, 1)
	mux := http.NewServeMux()
	mux.HandleFunc(callbackPath, func(w http.ResponseWriter, r *http.Request) {
		code, err := parseCallback(r.URL.Query(), state)
		if err != nil {
			writePage(w, http.StatusBadRequest, "Login failed", err.Error())
		} else {
			writePage(w, http.StatusOK, "You are logged in", "You can close this window and return to the terminal.")
		}
		select {
		case results <- callbackResult{code, err}:
		default:
		}
	})
	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	go func() { _ = srv.Serve(ln) }()
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	fmt.Fprintln(opts.Out, "Opening your browser to log in to CubePath...")
	fmt.Fprintf(opts.Out, "If it does not open, visit:\n\n  %s\n\n", consentURL)
	if err := openBrowser(consentURL); err != nil {
		fmt.Fprintln(opts.Out, "Could not open a browser automatically.")
	}
	fmt.Fprintln(opts.Out, "Waiting for authorization...")

	select {
	case res := <-results:
		return res.code, res.err
	case <-ctx.Done():
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return "", fmt.Errorf("timed out waiting for the browser authorization")
		}
		return "", ctx.Err()
	}
}

func pasteCallback(ctx context.Context, opts LoginOptions, consentURL, state string) (string, error) {
	fmt.Fprintf(opts.Out, "Open this URL in a browser and approve the access:\n\n  %s\n\n", consentURL)
	fmt.Fprintln(opts.Out, "The browser will then fail to load a 127.0.0.1 page. That is expected:")
	fmt.Fprint(opts.Out, "copy the full URL from its address bar and paste it here: ")

	lines := make(chan string, 1)
	go func() {
		s := bufio.NewScanner(opts.In)
		s.Buffer(make([]byte, 0, 64*1024), 64*1024)
		if s.Scan() {
			lines <- s.Text()
		}
		close(lines)
	}()
	select {
	case line, ok := <-lines:
		if !ok {
			return "", fmt.Errorf("no URL provided")
		}
		u, err := url.Parse(strings.TrimSpace(line))
		if err != nil || u.RawQuery == "" {
			return "", fmt.Errorf("that does not look like the redirect URL")
		}
		return parseCallback(u.Query(), state)
	case <-ctx.Done():
		return "", fmt.Errorf("timed out waiting for the redirect URL")
	}
}

func parseCallback(q url.Values, state string) (string, error) {
	if subtle.ConstantTimeCompare([]byte(q.Get("state")), []byte(state)) != 1 {
		return "", fmt.Errorf("state mismatch: the response does not belong to this login attempt")
	}
	if code := q.Get("error"); code != "" {
		if code == "access_denied" {
			return "", fmt.Errorf("access was denied in the browser")
		}
		return "", &Error{Code: code, Description: q.Get("error_description")}
	}
	code := q.Get("code")
	if code == "" {
		return "", fmt.Errorf("the response has no authorization code")
	}
	return code, nil
}

func writePage(w http.ResponseWriter, status int, title, msg string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	fmt.Fprintf(w, `<!doctype html><html><head><meta charset="utf-8"><title>CubePath CLI</title>
<style>body{font-family:system-ui,sans-serif;display:flex;align-items:center;justify-content:center;height:100vh;margin:0;background:#0b0b0f;color:#e5e5e5}
div{text-align:center;max-width:32rem;padding:1rem}h1{font-size:1.4rem}p{color:#a3a3a3}</style></head>
<body><div><h1>%s</h1><p>%s</p></div></body></html>`, html.EscapeString(title), html.EscapeString(msg))
}

// openBrowser is a variable so tests can replace it.
var openBrowser = func(u string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", u)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", u)
	default:
		cmd = exec.Command("xdg-open", u)
	}
	return cmd.Start()
}
