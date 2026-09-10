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

// TestLoad_StacklessLayoutRejected: a layout with no {stack} token makes the
// per-stack inventory walk read the same directory for every stack, filing one
// app under N stacks (see internal/layout.Validate). Load must fail closed and
// name the missing token, not let it reach a running wizard.
func TestLoad_StacklessLayoutRejected(t *testing.T) {
	t.Setenv("WIZARD_CONFIG", "")
	t.Setenv("REPO_OWNER", "acme")
	t.Setenv("REPO_NAME", "gitops")
	t.Setenv("XRD_PATH", "xrds/app.yaml")
	t.Setenv("RENDER_ENABLED", "false")
	t.Setenv("LAYOUT", "workloads/{app}")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "{stack}") {
		t.Fatalf("expected an error naming the missing {stack} token, got %v", err)
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

// links: file-only list of {label,url}; url is a template over {namespace},
// {name}, {stack}. Validated at load so a typo fails at startup, not on click.
func TestLoad_Links(t *testing.T) {
	writeConfig(t, `
repo:
  owner: acme
  name: platform
schema:
  xrdPath: xrds/app.yaml
render:
  enabled: false
links:
  - label: Headlamp (aws-0)
    url: https://headlamp.example/c/main/apps/{namespace}/{name}
  - label: Runbook
    url: https://wiki.example/{stack}/{name}
`)
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.Links) != 2 {
		t.Fatalf("Links = %+v, want 2", cfg.Links)
	}
	if cfg.Links[0].Label != "Headlamp (aws-0)" || cfg.Links[0].URL != "https://headlamp.example/c/main/apps/{namespace}/{name}" {
		t.Errorf("Links[0] = %+v", cfg.Links[0])
	}
}

func TestLoad_LinksAbsentIsEmpty(t *testing.T) {
	writeConfig(t, "repo:\n  owner: acme\n  name: platform\nschema:\n  xrdPath: xrds/app.yaml\nrender:\n  enabled: false\n")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.Links) != 0 {
		t.Errorf("Links = %+v, want none", cfg.Links)
	}
}

func TestLoad_LinksRejected(t *testing.T) {
	base := "repo:\n  owner: acme\n  name: platform\nschema:\n  xrdPath: xrds/app.yaml\nrender:\n  enabled: false\n"
	// idx is the "links[N]" the error must name; empty means "links[0]" — the
	// default for every case whose offending entry is the only (first) one.
	cases := map[string]struct{ links, want, idx string }{
		"unknown placeholder":                  {"links:\n  - label: X\n    url: https://x.example/{cluster}/{name}\n", `{cluster}`, ""},
		"relative url":                         {"links:\n  - label: X\n    url: /apps/{name}\n", "absolute http", ""},
		"bad scheme":                           {"links:\n  - label: X\n    url: ftp://x.example/{name}\n", "absolute http", ""},
		"empty label":                          {"links:\n  - label: \"\"\n    url: https://x.example/{name}\n", "label", ""},
		"empty url":                            {"links:\n  - label: X\n    url: \"\"\n", "url", ""},
		"missing closing brace":                {"links:\n  - label: X\n    url: https://x.example/{name\n", "unbalanced", ""},
		"missing opening brace":                {"links:\n  - label: X\n    url: https://x.example/name}\n", "unbalanced", ""},
		"doubled braces":                       {"links:\n  - label: X\n    url: https://x.example/{{name}}\n", "unbalanced", ""},
		"empty placeholder":                    {"links:\n  - label: X\n    url: https://x.example/{}\n", "{}", ""},
		"placeholder in host":                  {"links:\n  - label: X\n    url: https://headlamp.{stack}.example/apps/{name}\n", "scheme, host, or port", ""},
		"placeholder in userinfo":              {"links:\n  - label: X\n    url: https://{name}@x.example/a\n", "scheme, host, or port", ""},
		"placeholder in port":                  {"links:\n  - label: X\n    url: https://x.example:{name}/a\n", "scheme, host, or port", ""},
		"literal brace needs percent-encoding": {"links:\n  - label: X\n    url: https://x.example/{name}}\n", "percent-encode", ""},
		"offending entry is not first": {
			"links:\n  - label: Good\n    url: https://x.example/{name}\n  - label: X\n    url: ftp://x.example/{name}\n",
			"absolute http",
			"links[1]",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			writeConfig(t, base+tc.links)
			_, err := Load()
			if err == nil {
				t.Fatalf("Load succeeded, want an error mentioning %q", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q does not mention %q", err, tc.want)
			}
			wantIdx := tc.idx
			if wantIdx == "" {
				wantIdx = "links[0]"
			}
			if !strings.Contains(err.Error(), wantIdx) {
				t.Errorf("error %q does not name the entry %q", err, wantIdx)
			}
		})
	}
}
