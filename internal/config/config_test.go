package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeConfig writes a wizard.yaml into a temp dir and points WIZARD_CONFIG at
// it for the duration of the test.
func writeConfig(t *testing.T, body string) {
	t.Helper()
	p := filepath.Join(t.TempDir(), "wizard.yaml")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Setenv("WIZARD_CONFIG", p)
}

// TestLoad_PureEnvDefaults: with no config file, Load reproduces the historical
// env/defaults behaviour, plus the new SPEC-009 defaults.
func TestLoad_PureEnvDefaults(t *testing.T) {
	t.Setenv("WIZARD_CONFIG", "") // no file → default path is absent in CI
	t.Setenv("REPO_OWNER", "")
	t.Setenv("REPO_NAME", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.RepoOwner != "Smana" || cfg.RepoName != "cloud-native-ref" {
		t.Errorf("repo defaults = %s/%s", cfg.RepoOwner, cfg.RepoName)
	}
	if cfg.Layout != "apps/{stack}/{app}" {
		t.Errorf("Layout default = %q", cfg.Layout)
	}
	if !cfg.RenderEnabled {
		t.Errorf("RenderEnabled default = false, want true")
	}
	if cfg.BrandingTitle != "App Wizard" {
		t.Errorf("BrandingTitle default = %q", cfg.BrandingTitle)
	}
}

// TestLoad_FileValues: wizard.yaml values flow into Config.
func TestLoad_FileValues(t *testing.T) {
	t.Setenv("REPO_OWNER", "")
	t.Setenv("LAYOUT", "")
	writeConfig(t, `
repo:
  owner: acme
  name: platform
  baseBranch: trunk
schema:
  xrdPath: xrds/service.yaml
  stacksPath: stacks.yaml
layout: "workloads/{stack}/{app}"
render:
  enabled: false
branding:
  title: Platform Console
  logoUrl: /brand.svg
  theme:
    color-primary: "#0af"
`)
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.RepoOwner != "acme" || cfg.RepoName != "platform" || cfg.RepoBaseBranch != "trunk" {
		t.Errorf("repo = %s/%s@%s", cfg.RepoOwner, cfg.RepoName, cfg.RepoBaseBranch)
	}
	if cfg.XRDPath != "xrds/service.yaml" || cfg.StacksPath != "stacks.yaml" {
		t.Errorf("schema paths = %s / %s", cfg.XRDPath, cfg.StacksPath)
	}
	if cfg.Layout != "workloads/{stack}/{app}" {
		t.Errorf("Layout = %q", cfg.Layout)
	}
	if cfg.RenderEnabled {
		t.Errorf("RenderEnabled = true, want false from file")
	}
	if cfg.BrandingTitle != "Platform Console" || cfg.BrandingLogoURL != "/brand.svg" {
		t.Errorf("branding = %q / %q", cfg.BrandingTitle, cfg.BrandingLogoURL)
	}
	if cfg.BrandingTheme["color-primary"] != "#0af" {
		t.Errorf("theme = %v", cfg.BrandingTheme)
	}
}

// TestLoad_EnvOverridesFile: an env var wins over the file value.
func TestLoad_EnvOverridesFile(t *testing.T) {
	writeConfig(t, "repo:\n  owner: fromfile\n")
	t.Setenv("REPO_OWNER", "fromenv")
	t.Setenv("RENDER_ENABLED", "false")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.RepoOwner != "fromenv" {
		t.Errorf("RepoOwner = %q, want env to win", cfg.RepoOwner)
	}
	if cfg.RenderEnabled {
		t.Errorf("RENDER_ENABLED=false env ignored")
	}
}

// TestLoad_SecretInFileRejected: a secret-bearing key fails the load closed.
func TestLoad_SecretInFileRejected(t *testing.T) {
	writeConfig(t, "auth:\n  mode: github\n  githubClientSecret: leaked\n")
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "secret") {
		t.Fatalf("expected secret-rejection error, got %v", err)
	}
}

// TestLoad_UnknownKeyRejected: strict decoding rejects a typo/unknown key.
func TestLoad_UnknownKeyRejected(t *testing.T) {
	writeConfig(t, "repo:\n  ownr: typo\n")
	if _, err := Load(); err == nil {
		t.Fatalf("expected strict-decode error for unknown key, got nil")
	}
}

// TestLoad_ExplicitMissingFileErrors: an explicitly-set WIZARD_CONFIG that does
// not exist is an error, not a silent fallback to defaults.
func TestLoad_ExplicitMissingFileErrors(t *testing.T) {
	t.Setenv("WIZARD_CONFIG", filepath.Join(t.TempDir(), "nope.yaml"))
	if _, err := Load(); err == nil {
		t.Fatalf("expected error for missing explicit config, got nil")
	}
}
