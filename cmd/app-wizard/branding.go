package main

import (
	"net/http"

	"github.com/Smana/app-wizard/internal/api"
	"github.com/Smana/app-wizard/internal/config"
	"github.com/Smana/app-wizard/internal/httputil"
)

// brandingHandler serves GET /api/branding. Unauthenticated on purpose: the
// SPA needs the chrome before login. Links is coerced to an empty array so the
// client can iterate it without a null check.
func brandingHandler(b api.Branding) http.HandlerFunc {
	if b.Links == nil {
		b.Links = []api.Link{}
	}
	return func(w http.ResponseWriter, _ *http.Request) {
		httputil.WriteJSON(w, http.StatusOK, b)
	}
}

// brandingFromConfig maps the loaded config onto the wire type.
func brandingFromConfig(cfg *config.Config) api.Branding {
	out := api.Branding{
		Title:   cfg.BrandingTitle,
		LogoURL: cfg.BrandingLogoURL,
		Theme:   cfg.BrandingTheme,
		Links:   make([]api.Link, 0, len(cfg.Links)),
	}
	for _, l := range cfg.Links {
		out.Links = append(out.Links, api.Link{Label: l.Label, URL: l.URL})
	}
	return out
}
