// Package oauth implements the browser login of cubecli against the CubePath
// authorization server: RFC 9728/8414 discovery, RFC 7591 client registration,
// authorization code + PKCE (RFC 7636) over a loopback redirect (RFC 8252),
// resource indicators (RFC 8707), refresh and revocation (RFC 7009).
package oauth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/CubePathInc/cubecli/internal/version"
)

const (
	ClientName = "CubePath CLI"
	clientURI  = "https://github.com/CubePathInc/cubecli"

	callbackPath = "/callback"
)

// callbackPorts are the loopback ports registered for the client. Redirect URIs
// must match exactly, so the ports are fixed; the first free one is used.
var callbackPorts = []int{38271, 38272, 38273, 38274, 38275}

var httpClient = &http.Client{Timeout: 30 * time.Second}

// ErrBrowserLoginUnsupported means the API does not accept browser-login
// tokens: its protected resource metadata does not name the API itself.
var ErrBrowserLoginUnsupported = errors.New("browser login is not available for this API yet")

// ResourceIsAPI reports whether an OAuth resource identifier is the API at
// apiURL, i.e. whether tokens issued for it are accepted by the API.
func ResourceIsAPI(resource, apiURL string) bool {
	r, err1 := url.Parse(resource)
	a, err2 := url.Parse(apiURL)
	if err1 != nil || err2 != nil {
		return false
	}
	return r.Scheme == a.Scheme && strings.EqualFold(r.Host, a.Host) && strings.Trim(r.Path, "/") == ""
}

// ProtectedResource is the RFC 9728 document served by the API.
type ProtectedResource struct {
	Resource             string   `json:"resource"`
	AuthorizationServers []string `json:"authorization_servers"`
}

// ServerMetadata is the RFC 8414 document served by the authorization server.
type ServerMetadata struct {
	Issuer                string   `json:"issuer"`
	AuthorizationEndpoint string   `json:"authorization_endpoint"`
	TokenEndpoint         string   `json:"token_endpoint"`
	RegistrationEndpoint  string   `json:"registration_endpoint"`
	RevocationEndpoint    string   `json:"revocation_endpoint"`
	ScopesSupported       []string `json:"scopes_supported"`
}

// TokenResponse is a successful token endpoint response.
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
}

// Scopes returns the granted scopes as a list.
func (t *TokenResponse) Scopes() []string {
	return strings.Fields(t.Scope)
}

// ExpiresAt converts expires_in to an absolute time, relative to now.
func (t *TokenResponse) ExpiresAt() time.Time {
	return time.Now().Add(time.Duration(t.ExpiresIn) * time.Second)
}

// Error is an OAuth error response (RFC 6749 section 5.2).
type Error struct {
	StatusCode  int
	Code        string `json:"error"`
	Description string `json:"error_description"`
}

func (e *Error) Error() string {
	if e.Description != "" {
		return fmt.Sprintf("%s: %s", e.Code, e.Description)
	}
	if e.Code != "" {
		return e.Code
	}
	return fmt.Sprintf("authorization server returned HTTP %d", e.StatusCode)
}

// Discover resolves the authorization server of the API at apiURL.
func Discover(ctx context.Context, apiURL string) (*ProtectedResource, *ServerMetadata, error) {
	var pr ProtectedResource
	if err := getJSON(ctx, strings.TrimRight(apiURL, "/")+"/.well-known/oauth-protected-resource", &pr); err != nil {
		return nil, nil, ErrBrowserLoginUnsupported
	}
	if len(pr.AuthorizationServers) == 0 || pr.Resource == "" || !ResourceIsAPI(pr.Resource, apiURL) {
		return nil, nil, ErrBrowserLoginUnsupported
	}

	issuer := strings.TrimRight(pr.AuthorizationServers[0], "/")
	if err := requireSecureURL(issuer); err != nil {
		return nil, nil, err
	}
	var meta ServerMetadata
	if err := getJSON(ctx, issuer+"/.well-known/oauth-authorization-server", &meta); err != nil {
		return nil, nil, fmt.Errorf("failed to read authorization server metadata: %w", err)
	}
	// RFC 8414 section 3.3: the issuer in the document must be the one we asked.
	if strings.TrimRight(meta.Issuer, "/") != issuer {
		return nil, nil, fmt.Errorf("authorization server metadata issuer mismatch: %q != %q", meta.Issuer, issuer)
	}
	for _, u := range []string{meta.AuthorizationEndpoint, meta.TokenEndpoint} {
		if err := requireSecureURL(u); err != nil {
			return nil, nil, err
		}
	}
	return &pr, &meta, nil
}

// Register creates a public client for this machine's loopback redirect URIs.
func Register(ctx context.Context, meta *ServerMetadata) (string, error) {
	if meta.RegistrationEndpoint == "" {
		return "", fmt.Errorf("the authorization server does not support client registration")
	}
	redirects := make([]string, len(callbackPorts))
	for i, p := range callbackPorts {
		redirects[i] = redirectURI(p)
	}
	body, _ := json.Marshal(map[string]interface{}{
		"client_name":                ClientName,
		"client_uri":                 clientURI,
		"redirect_uris":              redirects,
		"grant_types":                []string{"authorization_code", "refresh_token"},
		"response_types":             []string{"code"},
		"token_endpoint_auth_method": "none",
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, meta.RegistrationEndpoint, strings.NewReader(string(body)))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	var out struct {
		ClientID string `json:"client_id"`
	}
	if err := doJSON(req, &out); err != nil {
		return "", fmt.Errorf("client registration failed: %w", err)
	}
	if out.ClientID == "" {
		return "", fmt.Errorf("client registration returned no client_id")
	}
	return out.ClientID, nil
}

// Exchange trades an authorization code for tokens.
func Exchange(ctx context.Context, tokenEndpoint, clientID, code, redirect, verifier, resource string) (*TokenResponse, error) {
	return tokenRequest(ctx, tokenEndpoint, url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {redirect},
		"client_id":     {clientID},
		"code_verifier": {verifier},
		"resource":      {resource},
	})
}

// RefreshToken obtains a new access token. The server rotates the refresh token:
// the returned one replaces the old, which must never be presented again.
func RefreshToken(ctx context.Context, tokenEndpoint, clientID, refreshToken, resource string) (*TokenResponse, error) {
	return tokenRequest(ctx, tokenEndpoint, url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
		"client_id":     {clientID},
		"resource":      {resource},
	})
}

// Revoke invalidates the grant behind a token (RFC 7009).
func Revoke(ctx context.Context, revocationEndpoint, clientID, token, hint string) error {
	if revocationEndpoint == "" {
		return fmt.Errorf("the authorization server does not support token revocation")
	}
	form := url.Values{"token": {token}, "client_id": {clientID}}
	if hint != "" {
		form.Set("token_type_hint", hint)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, revocationEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return doJSON(req, nil)
}

func tokenRequest(ctx context.Context, endpoint string, form url.Values) (*TokenResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	var tok TokenResponse
	if err := doJSON(req, &tok); err != nil {
		return nil, err
	}
	if tok.AccessToken == "" || tok.RefreshToken == "" {
		return nil, fmt.Errorf("token response is missing access_token or refresh_token")
	}
	return &tok, nil
}

func getJSON(ctx context.Context, u string, out interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	return doJSON(req, out)
}

func doJSON(req *http.Request, out interface{}) error {
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", fmt.Sprintf("CubeCLI/%s", version.Version))
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("connection error: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return parseError(resp.StatusCode, body)
	}
	if out == nil || len(body) == 0 {
		return nil
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("invalid response from %s: %w", req.URL.Host, err)
	}
	return nil
}

func parseError(status int, body []byte) error {
	e := &Error{StatusCode: status}
	if json.Unmarshal(body, e) == nil && e.Code != "" {
		return e
	}
	// {"detail": "..."} from endpoints outside the OAuth spec.
	var d struct {
		Detail string `json:"detail"`
	}
	if json.Unmarshal(body, &d) == nil && d.Detail != "" {
		e.Description = d.Detail
	}
	return e
}

// requireSecureURL refuses plain http except to the local machine.
func requireSecureURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return fmt.Errorf("invalid authorization server URL %q", raw)
	}
	if u.Scheme == "https" {
		return nil
	}
	if u.Scheme == "http" && isLoopbackHost(u.Hostname()) {
		return nil
	}
	return fmt.Errorf("refusing insecure authorization server URL %q", raw)
}

func isLoopbackHost(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func redirectURI(port int) string {
	return fmt.Sprintf("http://127.0.0.1:%d%s", port, callbackPath)
}
