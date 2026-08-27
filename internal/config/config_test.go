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

// TestLoad_UnconfiguredFailsClosed: with nothing configured, Load must NOT
// invent a target repository. It used to default to Smana/cloud-native-ref —
// this project's own origin repo — so a wizard whose config mount failed to land
// came up happily pointed at someone else's GitOps repo, which is where it opens
// pull requests.
func TestLoad_UnconfiguredFailsClosed(t *testing.T) {
	t.Setenv("WIZARD_CONFIG", "") // no file → default path is absent in CI
	t.Setenv("REPO_OWNER", "")
	t.Setenv("REPO_NAME", "")
	t.Setenv("XRD_PATH", "")

	_, err := Load()
	if err == nil {
		t.Fatal("Load succeeded with no repo/XRD configured; want a hard error")
	}
	for _, want := range []string{"repo.owner", "repo.name", "schema.xrdPath"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not name the missing key %q", err, want)
		}
	}
	if strings.Contains(err.Error(), "cloud-native-ref") {
		t.Errorf("error still references the origin repo: %v", err)
	}
}

// TestLoad_MinimalDefaults: with the un-defaultable keys supplied, everything
// else falls back to the neutral defaults.
func TestLoad_MinimalDefaults(t *testing.T) {
	t.Setenv("WIZARD_CONFIG", "")
	t.Setenv("REPO_OWNER", "acme")
	t.Setenv("REPO_NAME", "gitops")
	t.Setenv("XRD_PATH", "xrds/app.yaml")
	t.Setenv("RENDER_ENABLED", "false")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Layout != "apps/{stack}/{app}" {
		t.Errorf("Layout default = %q", cfg.Layout)
	}
	if cfg.BrandingTitle != "App Wizard" {
		t.Errorf("BrandingTitle default = %q", cfg.BrandingTitle)
	}
	if cfg.RepoBaseBranch != "main" {
		t.Errorf("RepoBaseBranch default = %q", cfg.RepoBaseBranch)
	}
}

// TestLoad_RenderPathsRequiredOnlyWhenEnabled: the render preview needs the
// composition/functions paths; the form and PR flow do not.
func TestLoad_RenderPathsRequiredOnlyWhenEnabled(t *testing.T) {
	t.Setenv("WIZARD_CONFIG", "")
	t.Setenv("REPO_OWNER", "acme")
	t.Setenv("REPO_NAME", "gitops")
	t.Setenv("XRD_PATH", "xrds/app.yaml")
	t.Setenv("COMPOSITION_PATH", "")
	t.Setenv("FUNCTIONS_PATH", "")

	t.Setenv("RENDER_ENABLED", "true")
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "compositionPath") {
		t.Fatalf("render on + no compositionPath should fail, got %v", err)
	}

	t.Setenv("RENDER_ENABLED", "false")
	if _, err := Load(); err != nil {
		t.Fatalf("render off should not require render paths: %v", err)
	}
}

// TestLoad_FileValues: wizard.yaml values flow into Config.
func TestLoad_FileValues(t *testing.T) {
	t.Setenv("REPO_OWNER", "")
	t.Setenv("REPO_NAME", "")
	t.Setenv("XRD_PATH", "")
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
	writeConfig(t, "repo:\n  owner: fromfile\n  name: platform\nschema:\n  xrdPath: xrds/app.yaml\n")
	t.Setenv("REPO_OWNER", "fromenv")
	t.Setenv("REPO_NAME", "")
	t.Setenv("XRD_PATH", "")
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
