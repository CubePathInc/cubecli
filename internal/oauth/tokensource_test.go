package oauth

import (
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/CubePathInc/cubecli/internal/config"
)

// Several cubecli processes with the same expired token must result in ONE
// refresh. A second refresh with the rotated token would make the server revoke
// the grant and log the user out everywhere.
func TestConcurrentRefreshRotatesOnce(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", t.TempDir())
	f := newFakeServer(t)
	f.refresh = "rt-initial"

	creds := &config.OAuthCredentials{
		Issuer:          f.srv.URL,
		TokenEndpoint:   f.srv.URL + "/oauth/token",
		ClientID:        "client-1",
		Resource:        fakeResource,
		AccessToken:     "at-expired",
		RefreshToken:    "rt-initial",
		AccessExpiresAt: time.Now().Add(-time.Minute),
	}
	if err := config.Save(&config.Config{
		CurrentProfile: "work",
		Profiles:       map[string]*config.Profile{"work": {OAuth: creds}},
	}); err != nil {
		t.Fatal(err)
	}

	const workers = 8
	tokens := make([]string, workers)
	errs := make([]error, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			// A TokenSource per worker, like separate processes: each starts from
			// the same stale on-disk state.
			tokens[i], errs[i] = NewTokenSource("work", creds).Token()
		}(i)
	}
	wg.Wait()

	for i := 0; i < workers; i++ {
		if errs[i] != nil {
			t.Fatalf("worker %d: %v", i, errs[i])
		}
		if tokens[i] != tokens[0] {
			t.Fatalf("workers got different tokens: %q vs %q", tokens[i], tokens[0])
		}
	}
	if n := atomic.LoadInt32(&f.refreshes); n != 1 {
		t.Fatalf("expected exactly 1 refresh, got %d", n)
	}
	if f.revoked {
		t.Fatal("grant was revoked by refresh token reuse")
	}

	saved, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got := saved.Profiles["work"].OAuth; got.AccessToken != tokens[0] || got.RefreshToken != f.refresh {
		t.Fatalf("rotated tokens not persisted: %+v", got)
	}
}

func TestFreshTokenIsNotRefreshed(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	ts := NewTokenSource("work", &config.OAuthCredentials{AccessToken: "at", AccessExpiresAt: time.Now().Add(time.Hour)})
	tok, err := ts.Token()
	if err != nil || tok != "at" {
		t.Fatalf("got %q %v", tok, err)
	}
}

func TestRevokedGrantAsksToLogIn(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	f := newFakeServer(t)
	f.refresh = "rt-current"
	creds := &config.OAuthCredentials{
		TokenEndpoint: f.srv.URL + "/oauth/token", ClientID: "c", Resource: fakeResource,
		AccessToken: "at", RefreshToken: "rt-stolen-and-rotated", AccessExpiresAt: time.Now().Add(-time.Minute),
	}
	if err := config.Save(&config.Config{Profiles: map[string]*config.Profile{"work": {OAuth: creds}}}); err != nil {
		t.Fatal(err)
	}
	_, err := NewTokenSource("work", creds).Token()
	if err == nil || !strings.Contains(err.Error(), "cubecli login --profile work") {
		t.Fatalf("expected a login hint, got %v", err)
	}
}
