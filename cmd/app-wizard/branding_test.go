package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/Smana/app-wizard/internal/api"
	"github.com/Smana/app-wizard/internal/config"
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

// TestBrandingFromConfig: brandingFromConfig is the one untested hop between
// config.Config and the wire type — a swapped Label/URL, a dropped entry, or
// reordering would slip past brandingHandler's tests, since those construct
// api.Branding directly rather than going through this mapping.
func TestBrandingFromConfig(t *testing.T) {
	cases := map[string]struct {
		links []config.Link
		want  []api.Link
	}{
		"two links map in order with the same field values": {
			links: []config.Link{
				{Label: "Headlamp", URL: "https://h.example/{name}"},
				{Label: "Runbook", URL: "https://wiki.example/{stack}/{name}"},
			},
			want: []api.Link{
				{Label: "Headlamp", URL: "https://h.example/{name}"},
				{Label: "Runbook", URL: "https://wiki.example/{stack}/{name}"},
			},
		},
		"nil links become a non-nil empty slice": {
			links: nil,
			want:  []api.Link{},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := brandingFromConfig(&config.Config{Links: tc.links})
			if !reflect.DeepEqual(got.Links, tc.want) {
				t.Errorf("Links = %+v, want %+v", got.Links, tc.want)
			}
		})
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
