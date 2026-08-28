package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Smana/app-wizard/internal/gitprovider"
)

func newTestAuth(dev gitprovider.Provider) *Auth {
	return New(Config{
		ClientID:    "gh-client",
		RedirectURL: "http://localhost:8080/api/auth/callback",
		SessionKey:  []byte("0123456789abcdef0123456789abcdef"), // pragma: allowlist secret
		Factory: func(ctx context.Context, token string) (gitprovider.Provider, error) {
			return &gitprovider.FakeProvider{User: gitprovider.User{Login: "gh-user"}}, nil
		},
		DevProvider: dev,
	})
}

// seedAuthToken returns a request carrying a session with a github token, so
// github-mode ProviderForRequest can be exercised without the OAuth round-trip.
func seedAuthToken(t *testing.T, a *Auth, token string) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/pr", nil)
	sess, _ := a.store.Get(req, sessionName)
	sess.Values[tokenKey] = token
	rec := httptest.NewRecorder()
	if err := sess.Save(req, rec); err != nil {
		t.Fatalf("save session: %v", err)
	}
	out := httptest.NewRequest(http.MethodPost, "/api/pr", nil)
	for _, c := range rec.Result().Cookies() {
		out.AddCookie(c)
	}
	return out
}

// TestGitHubModeProviderForRequest confirms a session token yields a provider,
// and no token yields ErrUnauthenticated.
func TestGitHubModeProviderForRequest(t *testing.T) {
	a := newTestAuth(nil)

	// No session → ErrUnauthenticated.
	req := httptest.NewRequest(http.MethodPost, "/api/pr", nil)
	if _, err := a.ProviderForRequest(req); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("no session: got %v, want ErrUnauthenticated", err)
	}

	// With token → provider, no error.
	req = seedAuthToken(t, a, "gh-token")
	p, err := a.ProviderForRequest(req)
	if err != nil || p == nil {
		t.Fatalf("with token: provider=%v err=%v", p, err)
	}
}

// TestDevModeBypass confirms AUTH_MODE=dev still returns the dev provider
// without a session (login bypass unchanged).
func TestDevModeBypass(t *testing.T) {
	dev := &gitprovider.FakeProvider{User: gitprovider.User{Login: "dev-user"}}
	a := newTestAuth(dev)

	req := httptest.NewRequest(http.MethodPost, "/api/pr", nil) // no session
	p, err := a.ProviderForRequest(req)
	if err != nil {
		t.Fatalf("dev bypass: unexpected error %v", err)
	}
	if p != dev {
		t.Fatalf("dev bypass: expected the dev provider")
	}
}

// newTestAuthWithProviderErr builds github-mode Auth whose factory yields a
// provider failing CurrentUser with err.
func newTestAuthWithProviderErr(err error) *Auth {
	return New(Config{
		ClientID:    "gh-client",
		RedirectURL: "http://localhost:8080/api/auth/callback",
		SessionKey:  []byte("0123456789abcdef0123456789abcdef"), // pragma: allowlist secret
		Factory: func(ctx context.Context, token string) (gitprovider.Provider, error) {
			return &gitprovider.FakeProvider{CurrentUserErr: err}, nil
		},
	})
}

// TestMeStaleSessionClears covers the case that left the UI stuck on
// "failed to fetch user" with no way forward: the cookie is signed by
// SESSION_KEY, which outlives the cluster, so a session issued by a previous
// OAuth app decrypts perfectly and carries a token GitHub no longer honours.
//
// That must read as "sign in again" (401 + cookie dropped), not as a broken
// upstream (502), because only one of those is actionable by the user.
func TestMeStaleSessionClears(t *testing.T) {
	a := newTestAuthWithProviderErr(fmt.Errorf("%w: 401", gitprovider.ErrUnauthorized))
	req := seedAuthToken(t, a, "stale-token")
	rec := httptest.NewRecorder()

	a.Me(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	// The stale cookie must be expired, or every reload repeats the failure.
	var cleared bool
	for _, c := range rec.Result().Cookies() {
		if c.Name == sessionName && c.MaxAge < 0 {
			cleared = true
		}
	}
	if !cleared {
		t.Fatalf("session cookie was not expired; cookies=%v", rec.Result().Cookies())
	}
}

// TestMeUpstreamFailureStays502 is the other half: an error that is NOT a
// credential rejection must remain a gateway failure, and must NOT drop the
// user's session. Signing in again cannot fix GitHub being unreachable.
func TestMeUpstreamFailureStays502(t *testing.T) {
	a := newTestAuthWithProviderErr(errors.New("dial tcp: connection refused"))
	req := seedAuthToken(t, a, "good-token")
	rec := httptest.NewRecorder()

	a.Me(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusBadGateway)
	}
	for _, c := range rec.Result().Cookies() {
		if c.Name == sessionName && c.MaxAge < 0 {
			t.Fatal("session was cleared on a transport failure; re-login cannot fix that")
		}
	}
}
