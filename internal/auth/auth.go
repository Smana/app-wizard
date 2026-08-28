// Package auth implements GitHub OAuth for the wizard (FR-004, T104). The
// user's access token is stored only in a signed+encrypted secure cookie
// session and is never logged. PRs are opened with this per-user token.
package auth

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/Smana/app-wizard/internal/api"
	"github.com/Smana/app-wizard/internal/gitprovider"
	"github.com/Smana/app-wizard/internal/httputil"
	"github.com/gorilla/sessions"
	"golang.org/x/oauth2"
	githuboauth "golang.org/x/oauth2/github"
)

const (
	sessionName    = "app-wizard-session"
	tokenKey       = "gh_token"
	stateKey       = "oauth_state"
	stateCookieTTL = 600 // seconds
)

// ProviderFactory builds a gitprovider.Provider from a user token. Injected so
// tests and main can supply GitHub or a fake. It returns an error because
// constructing the GitHub client can fail (go-github's NewClient is fallible
// since v89) — a construction failure must surface as a 5xx, not a nil Provider
// that panics on first use.
type ProviderFactory func(ctx context.Context, token string) (gitprovider.Provider, error)

// Auth holds OAuth configuration and the session store.
type Auth struct {
	oauthConfig *oauth2.Config
	store       sessions.Store
	factory     ProviderFactory
	logger      *slog.Logger
	// devProvider, when non-nil, activates AUTH_MODE=dev: login is bypassed
	// (Me returns a fake user) and this provider handles all requests. DEV ONLY.
	devProvider gitprovider.Provider
}

// Config configures the Auth handler.
type Config struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	SessionKey   []byte
	Factory      ProviderFactory
	Logger       *slog.Logger
	// DevProvider activates the dev bypass when non-nil (AUTH_MODE=dev). Must be
	// nil in any deployed environment.
	DevProvider gitprovider.Provider
}

// New builds an Auth handler.
func New(cfg Config) *Auth {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	store := sessions.NewCookieStore(cfg.SessionKey)
	store.Options = &sessions.Options{
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   86400 * 7,
	}
	return &Auth{
		oauthConfig: &oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			RedirectURL:  cfg.RedirectURL,
			Scopes:       []string{"repo", "read:user"},
			Endpoint:     githuboauth.Endpoint,
		},
		store:       store,
		factory:     cfg.Factory,
		logger:      logger,
		devProvider: cfg.DevProvider,
	}
}

// Login redirects the user to GitHub's authorization page.
func (a *Auth) Login(w http.ResponseWriter, r *http.Request) {
	state, err := randomState()
	if err != nil {
		a.writeError(w, http.StatusInternalServerError, "failed to init login")
		return
	}
	sess, _ := a.store.Get(r, sessionName)
	sess.Values[stateKey] = state
	sess.Options.MaxAge = stateCookieTTL
	if err := sess.Save(r, w); err != nil {
		a.writeError(w, http.StatusInternalServerError, "failed to save session")
		return
	}
	http.Redirect(w, r, a.oauthConfig.AuthCodeURL(state), http.StatusFound)
}

// Callback exchanges the auth code for a token and stores it in the session.
func (a *Auth) Callback(w http.ResponseWriter, r *http.Request) {
	sess, _ := a.store.Get(r, sessionName)
	wantState, _ := sess.Values[stateKey].(string)
	gotState := r.URL.Query().Get("state")
	if wantState == "" || gotState != wantState {
		a.writeError(w, http.StatusBadRequest, "invalid OAuth state")
		return
	}
	delete(sess.Values, stateKey)

	code := r.URL.Query().Get("code")
	if code == "" {
		a.writeError(w, http.StatusBadRequest, "missing OAuth code")
		return
	}
	token, err := a.oauthConfig.Exchange(r.Context(), code)
	if err != nil {
		// Never log the code or any token material.
		a.logger.Warn("oauth exchange failed")
		a.writeError(w, http.StatusBadGateway, "OAuth exchange failed")
		return
	}

	sess.Values[tokenKey] = token.AccessToken
	sess.Options.MaxAge = 86400 * 7
	if err := sess.Save(r, w); err != nil {
		a.writeError(w, http.StatusInternalServerError, "failed to persist session")
		return
	}
	http.Redirect(w, r, "/", http.StatusFound)
}

// Me returns the authenticated user (401 if unauthenticated).
func (a *Auth) Me(w http.ResponseWriter, r *http.Request) {
	if a.devProvider != nil { // AUTH_MODE=dev
		u, err := a.devProvider.CurrentUser(r.Context())
		if err != nil {
			a.writeError(w, http.StatusBadGateway, "failed to fetch user")
			return
		}
		httputil.WriteJSON(w, http.StatusOK, api.User{Login: u.Login, AvatarURL: u.AvatarURL, Name: u.Name})
		return
	}
	token, ok := a.Token(r)
	if !ok {
		a.writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}
	provider, err := a.factory(r.Context(), token)
	if err != nil {
		a.writeError(w, http.StatusInternalServerError, "failed to build git provider")
		return
	}
	u, err := provider.CurrentUser(r.Context())
	if err != nil {
		// A rejected token is a stale session, not a broken upstream. Returning
		// 502 here left the UI on "Something went wrong / failed to fetch user"
		// with no way forward: the cookie is signed by SESSION_KEY, which
		// outlives the cluster, so a session from a previous OAuth app or a
		// previous cluster decrypts perfectly and carries a token GitHub no
		// longer honours. Observed on gcp-0, 2026-08-28.
		//
		// Drop the session and answer 401 so the client offers a sign-in.
		if errors.Is(err, gitprovider.ErrUnauthorized) {
			a.clearSession(w, r)
			a.logger.Warn("session token rejected by the provider; session cleared")
			a.writeError(w, http.StatusUnauthorized, "not authenticated")
			return
		}
		// Anything else IS a gateway failure, and it used to vanish silently --
		// this handler logged nothing at all, so the 502 was undiagnosable from
		// the pod logs. Never log the error's body; the message is enough.
		a.logger.Warn("failed to fetch user", "err", err)
		a.writeError(w, http.StatusBadGateway, "failed to fetch user")
		return
	}
	// The login token IS the PR token in this wizard.
	httputil.WriteJSON(w, http.StatusOK, api.User{Login: u.Login, AvatarURL: u.AvatarURL, Name: u.Name})
}

// Logout clears the session.
func (a *Auth) Logout(w http.ResponseWriter, r *http.Request) {
	a.clearSession(w, r)
	w.WriteHeader(http.StatusNoContent)
}

// clearSession expires the session cookie. Shared by Logout and by Me when the
// provider rejects the stored token, so a stale session is dropped by the same
// code path a deliberate sign-out uses.
func (a *Auth) clearSession(w http.ResponseWriter, r *http.Request) {
	sess, _ := a.store.Get(r, sessionName)
	sess.Options.MaxAge = -1
	sess.Values = map[any]any{}
	_ = sess.Save(r, w)
}

// Token returns the session's GitHub token, if present.
func (a *Auth) Token(r *http.Request) (string, bool) {
	sess, err := a.store.Get(r, sessionName)
	if err != nil {
		return "", false
	}
	token, ok := sess.Values[tokenKey].(string)
	if !ok || token == "" {
		return "", false
	}
	return token, true
}

// ProviderForRequest builds a gitprovider for the request's authenticated user.
func (a *Auth) ProviderForRequest(r *http.Request) (gitprovider.Provider, error) {
	if a.devProvider != nil { // AUTH_MODE=dev
		return a.devProvider, nil
	}
	token, ok := a.Token(r)
	if !ok {
		return nil, ErrUnauthenticated
	}
	return a.factory(r.Context(), token)
}

// ErrUnauthenticated is returned when no user token is in the session. It is the
// httputil sentinel so handlers can tell it apart from a provider-construction
// failure without importing this package.
var ErrUnauthenticated = httputil.ErrUnauthenticated

func (a *Auth) writeError(w http.ResponseWriter, status int, msg string) {
	httputil.WriteError(w, status, msg)
}
