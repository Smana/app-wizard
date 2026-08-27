// Package httputil provides small shared HTTP response helpers used across the
// app-wizard backend handlers, replacing the previously duplicated per-package
// writeJSON/writeError copies.
package httputil

import (
	"encoding/json"
	"net/http"

	"errors"
	"log/slog"

	"github.com/Smana/app-wizard/internal/api"
)

// WriteJSON writes v as a JSON body with the given status code.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// WriteError writes an api.ErrorResponse{Error: msg} with the given status code.
func WriteError(w http.ResponseWriter, status int, msg string) {
	WriteJSON(w, status, api.ErrorResponse{Error: msg})
}

// ErrUnauthenticated marks a request with no usable user session. It lives here
// rather than in internal/auth so the handlers that consume a provider factory
// can distinguish it without importing auth.
var ErrUnauthenticated = errors.New("not authenticated")

// WriteProviderError turns a "build me the provider for this request" failure
// into the right response.
//
// These call sites used to map EVERY error to 401 "not authenticated". Once
// building the GitHub client became fallible, that meant a client-construction
// failure told a signed-in user they were signed out and bounced them to the
// login screen, with the real cause discarded — the exact silent outcome that
// making the factory fallible was supposed to prevent. Only a genuinely absent
// session is a 401; anything else is a server-side fault worth logging.
func WriteProviderError(w http.ResponseWriter, logger *slog.Logger, err error) {
	if errors.Is(err, ErrUnauthenticated) {
		WriteError(w, http.StatusUnauthorized, "not authenticated")
		return
	}
	if logger != nil {
		logger.Error("could not build git provider for request", "err", err.Error())
	}
	WriteError(w, http.StatusInternalServerError, "could not build git provider")
}
