package oauth

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/CubePathInc/cubecli/internal/config"
)

// refreshMargin is how long before expiry the access token is refreshed.
const refreshMargin = 60 * time.Second

// TokenSource serves the access token of an OAuth profile, refreshing it when it
// is about to expire or the API rejects it.
type TokenSource struct {
	profile string
	mu      sync.Mutex
	creds   config.OAuthCredentials
}

// NewTokenSource returns a token source for the named profile.
func NewTokenSource(profile string, creds *config.OAuthCredentials) *TokenSource {
	return &TokenSource{profile: profile, creds: *creds}
}

// Token returns a valid access token, refreshing it first if needed.
func (ts *TokenSource) Token() (string, error) {
	ts.mu.Lock()
	current := ts.creds.AccessToken
	fresh := time.Until(ts.creds.AccessExpiresAt) > refreshMargin
	ts.mu.Unlock()
	if fresh {
		return current, nil
	}
	return ts.Refresh(current)
}

// Refresh replaces the access token `stale`. It re-reads the profile under the
// config lock, so if another cubecli process already refreshed, its tokens are
// used instead of refreshing again.
func (ts *TokenSource) Refresh(stale string) (string, error) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	if ts.creds.AccessToken != stale && time.Until(ts.creds.AccessExpiresAt) > refreshMargin {
		return ts.creds.AccessToken, nil
	}

	var updated config.OAuthCredentials
	err := config.Update(func(cfg *config.Config) error {
		p, ok := cfg.Profiles[ts.profile]
		if !ok || p.OAuth == nil {
			return fmt.Errorf("profile %q is logged out: run 'cubecli login --profile %s'", ts.profile, ts.profile)
		}
		c := p.OAuth
		if c.AccessToken != stale && time.Until(c.AccessExpiresAt) > refreshMargin {
			updated = *c
			return nil
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		tok, err := RefreshToken(ctx, c.TokenEndpoint, c.ClientID, c.RefreshToken, c.Resource)
		if err != nil {
			var oerr *Error
			if errors.As(err, &oerr) && oerr.Code == "invalid_grant" {
				return fmt.Errorf("the session of profile %q expired or was revoked: run 'cubecli login --profile %s'", ts.profile, ts.profile)
			}
			return fmt.Errorf("failed to refresh the session: %w", err)
		}
		c.AccessToken = tok.AccessToken
		c.RefreshToken = tok.RefreshToken
		c.AccessExpiresAt = tok.ExpiresAt()
		if scopes := tok.Scopes(); len(scopes) > 0 {
			c.Scopes = scopes
		}
		updated = *c
		return nil
	})
	if err != nil {
		return "", err
	}
	ts.creds = updated
	return updated.AccessToken, nil
}
