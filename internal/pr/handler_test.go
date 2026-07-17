package pr

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Smana/app-wizard/internal/auth"
	"github.com/Smana/app-wizard/internal/gitprovider"
)

// TestHandler_Unauthenticated keeps the plain 401 for a missing session.
func TestHandler_Unauthenticated(t *testing.T) {
	s := validService()
	providerFor := func(r *http.Request) (gitprovider.Provider, error) {
		return nil, auth.ErrUnauthenticated
	}
	h := s.Handler(providerFor, nil)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/pr", strings.NewReader("{}"))
	h(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("got %d, want 401", rec.Code)
	}
}
