package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Smana/app-wizard/internal/api"
)

func TestBrandingHandlerCarriesLinks(t *testing.T) {
	h := brandingHandler(api.Branding{
		Title: "Console",
		Links: []api.Link{{Label: "Headlamp", URL: "https://h.example/c/main/apps/{namespace}/{name}"}},
	})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/branding", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var got api.Branding
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Title != "Console" || len(got.Links) != 1 || got.Links[0].URL != "https://h.example/c/main/apps/{namespace}/{name}" {
		t.Errorf("payload = %+v", got)
	}
}

func TestBrandingHandlerEmptyLinksIsArray(t *testing.T) {
	rec := httptest.NewRecorder()
	brandingHandler(api.Branding{Title: "X"}).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/branding", nil))
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if string(raw["links"]) != "[]" {
		t.Errorf("links = %s, want [] (never null — the SPA iterates it)", raw["links"])
	}
}
