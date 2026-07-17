// Package config loads the app-wizard runtime configuration from an optional
// wizard.yaml file overlaid by the environment (defaults → file → env). All
// values have local-dev-friendly defaults so `go run` works without any setup;
// secrets (OAuth client secret, session key, LLM key) come from the environment
// only, are rejected if present in the file, and are never logged.
package config

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	yaml "go.yaml.in/yaml/v3"
)

// XRDSourceMode selects where the schema pipeline reads repo files from.
type XRDSourceMode string

const (
	// SourceLocal reads files from RepoRoot on disk (dev/test).
	SourceLocal XRDSourceMode = "local"
	// SourceGitHub reads files from the GitHub repo via the API (prod).
	SourceGitHub XRDSourceMode = "github"
)

// Config is the fully-resolved runtime configuration.
type Config struct {
	// ListenAddr is the HTTP bind address (LISTEN_ADDR, default ":8080").
	ListenAddr string

	// AuthMode selects the authentication backend (AUTH_MODE):
	//   "github" (default) — real GitHub OAuth; login == PR token, opened as the user.
	//   "dev"              — LOCAL TESTING ONLY. Bypasses login (fake user) and
	//                        writes generated files to RepoRoot instead of opening
	//                        a real PR. Must never be set in a deployed environment.
	AuthMode string

	// GitHubClientID / GitHubClientSecret are the OAuth app credentials
	// (GITHUB_CLIENT_ID / GITHUB_CLIENT_SECRET) used for the login → PR token.
	GitHubClientID     string
	GitHubClientSecret string
	// OAuthRedirectURL is the callback URL registered with the OAuth app
	// (OAUTH_REDIRECT_URL). Defaults to a localhost callback for dev.
	OAuthRedirectURL string

	// RepoOwner / RepoName identify the GitOps repository PRs are opened
	// against (REPO_OWNER / REPO_NAME).
	RepoOwner string
	RepoName  string
	// RepoBaseBranch is the PR base branch (REPO_BASE_BRANCH, default "main").
	RepoBaseBranch string

	// XRDSource selects the schema source backend (XRD_SOURCE, local|github).
	XRDSource XRDSourceMode
	// RepoRoot is the on-disk repository root used by the local source and by
	// the crossplane renderer to locate composition files (REPO_ROOT).
	RepoRoot string

	// SessionKey is the secret used to authenticate/encrypt the session cookie
	// (SESSION_KEY). A random ephemeral key is generated when unset — fine for
	// dev, but sessions do not survive a restart.
	SessionKey []byte

	// XRDPath is the repo-relative path to the App XRD (XRD_PATH).
	XRDPath string
	// UIHintsPath is the on-disk path to ui-hints.yaml (UI_HINTS_PATH).
	UIHintsPath string
	// StacksPath is the repo-relative path to apps/stacks.yaml (STACKS_PATH).
	StacksPath string
	// CompositionPath / FunctionsPath / EnvConfigPath are repo-relative paths
	// used by the crossplane renderer.
	CompositionPath string
	FunctionsPath   string
	EnvConfigPath   string
	// FunctionsDevTargets maps a Crossplane Function name to a running gRPC
	// endpoint (host:port), parsed from FUNCTIONS_DEV_TARGETS
	// ("function-kcl=localhost:9443,function-auto-ready=localhost:9444,…").
	// When set, the renderer overlays the "Development" runtime onto the repo's
	// functions.yaml so `crossplane render` connects to those endpoints (the
	// in-pod function sidecars) instead of pulling+running images via Docker.
	FunctionsDevTargets map[string]string

	// --- LLM assists (Phase 3, FR-011). All optional. ---
	//
	// LLMAPIKey is the Anthropic API key (LLM_API_KEY). Never logged. When set,
	// LLM assists are available.
	LLMAPIKey string
	// LLMBaseURL overrides the Anthropic API base URL (LLM_BASE_URL). Optional;
	// lets the wizard target the platform AI Gateway. When empty the SDK default
	// (api.anthropic.com) is used. A non-empty base URL also marks assists as
	// available (supports a keyless gateway).
	LLMBaseURL string
	// LLMModel is the model id used for assists (LLM_MODEL, default
	// "claude-opus-4-8").
	LLMModel string

	// --- Agnostic-deployment knobs (SPEC-009). Introduced by the config-file
	// layer (T005); the behaviour that consumes them lands in later tasks:
	// Layout → T007, RenderEnabled → T009, Branding* → T008. Defaults reproduce
	// today's behaviour so they are inert until wired. ---

	// Layout is the PR file-layout template for a new app directory. Tokens
	// {stack} and {app} expand to the chosen stack and app name (LAYOUT).
	Layout string
	// RenderEnabled gates the crossplane render preview (RENDER_ENABLED). When
	// false the wizard still validates and opens PRs, without the preview.
	RenderEnabled bool
	// BrandingTitle / BrandingLogoURL / BrandingTheme drive the SPA chrome
	// (BRAND_TITLE / BRAND_LOGO_URL; theme is file-only). Neutral by default.
	BrandingTitle   string
	BrandingLogoURL string
	BrandingTheme   map[string]string
}

// AssistsAvailable reports whether LLM assists are configured: either an API
// key or a base URL (keyless gateway) is set.
func (c *Config) AssistsAvailable() bool {
	return c.LLMAPIKey != "" || c.LLMBaseURL != ""
}

// Load resolves configuration with precedence defaults → wizard.yaml → env
// (env wins, 12-factor). The config file is optional: when WIZARD_CONFIG is
// unset and the default path is absent, Load behaves exactly as the pure-env
// path did. Secrets are environment-only and are rejected if present in the
// file (FR-002, fail closed).
func Load() (*Config, error) {
	fc, err := loadFile()
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		ListenAddr:          pick("LISTEN_ADDR", "", ":8080"),
		AuthMode:            strings.ToLower(pick("AUTH_MODE", fc.Auth.Mode, "github")),
		GitHubClientID:      os.Getenv("GITHUB_CLIENT_ID"),
		GitHubClientSecret:  os.Getenv("GITHUB_CLIENT_SECRET"),
		OAuthRedirectURL:    pick("OAUTH_REDIRECT_URL", fc.Auth.RedirectURL, "http://localhost:8080/api/auth/callback"),
		RepoOwner:           pick("REPO_OWNER", fc.Repo.Owner, "Smana"),
		RepoName:            pick("REPO_NAME", fc.Repo.Name, "cloud-native-ref"),
		RepoBaseBranch:      pick("REPO_BASE_BRANCH", fc.Repo.BaseBranch, "main"),
		XRDSource:           XRDSourceMode(strings.ToLower(pick("XRD_SOURCE", "", string(SourceLocal)))),
		RepoRoot:            pick("REPO_ROOT", "", defaultRepoRoot()),
		XRDPath:             pick("XRD_PATH", fc.Schema.XRDPath, "infrastructure/base/crossplane/configuration/app-definition.yaml"),
		UIHintsPath:         pick("UI_HINTS_PATH", fc.Schema.UIHintsPath, ""),
		StacksPath:          pick("STACKS_PATH", fc.Schema.StacksPath, "apps/stacks.yaml"),
		CompositionPath:     pick("COMPOSITION_PATH", fc.Render.CompositionPath, "infrastructure/base/crossplane/configuration/app-composition.yaml"),
		FunctionsPath:       pick("FUNCTIONS_PATH", fc.Render.FunctionsPath, "infrastructure/base/crossplane/configuration/functions.yaml"),
		EnvConfigPath:       pick("ENVCONFIG_PATH", fc.Render.EnvConfigPath, "infrastructure/base/crossplane/configuration/environmentconfig.yaml"),
		LLMAPIKey:           os.Getenv("LLM_API_KEY"),
		LLMBaseURL:          pick("LLM_BASE_URL", fc.Assists.BaseURL, ""),
		LLMModel:            pick("LLM_MODEL", fc.Assists.Model, "claude-opus-4-8"),
		FunctionsDevTargets: parseKVList(pick("FUNCTIONS_DEV_TARGETS", fc.Render.FunctionsDevTargets, "")),
		Layout:              pick("LAYOUT", fc.Layout, "apps/{stack}/{app}"),
		RenderEnabled:       pickBool("RENDER_ENABLED", fc.Render.Enabled, true),
		BrandingTitle:       pick("BRAND_TITLE", fc.Branding.Title, "App Wizard"),
		BrandingLogoURL:     pick("BRAND_LOGO_URL", fc.Branding.LogoURL, ""),
		BrandingTheme:       fc.Branding.Theme,
	}

	if cfg.XRDSource != SourceLocal && cfg.XRDSource != SourceGitHub {
		cfg.XRDSource = SourceLocal
	}

	if cfg.UIHintsPath == "" {
		// ui-hints.yaml defaults beside the working directory.
		cfg.UIHintsPath = filepath.Join(defaultRepoRoot(), "ui-hints.yaml")
	}

	if key := os.Getenv("SESSION_KEY"); key != "" {
		cfg.SessionKey = []byte(key)
	} else {
		b := make([]byte, 32)
		if _, err := rand.Read(b); err != nil {
			return nil, err
		}
		cfg.SessionKey = []byte(hex.EncodeToString(b))
	}

	return cfg, nil
}

// pick returns the first non-empty of: env[key], fileVal, def. This is how the
// defaults → file → env precedence is applied per string field.
func pick(key, fileVal, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	if fileVal != "" {
		return fileVal
	}
	return def
}

// pickBool applies the same precedence to a bool. The file value is a *bool so
// an explicit `false` in the file is distinguishable from "unset".
func pickBool(key string, fileVal *bool, def bool) bool {
	if v := os.Getenv(key); v != "" {
		return v == "1" || strings.EqualFold(v, "true")
	}
	if fileVal != nil {
		return *fileVal
	}
	return def
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// fileConfig mirrors wizard.yaml. Only non-secret knobs live here; secrets are
// environment-only (loadFile rejects them). Strict decoding (KnownFields) makes
// an unknown key — a typo or a secret — fail the load rather than be ignored.
type fileConfig struct {
	Repo struct {
		Owner      string `yaml:"owner"`
		Name       string `yaml:"name"`
		BaseBranch string `yaml:"baseBranch"`
	} `yaml:"repo"`
	Schema struct {
		XRDPath     string `yaml:"xrdPath"`
		UIHintsPath string `yaml:"uiHintsPath"`
		StacksPath  string `yaml:"stacksPath"`
	} `yaml:"schema"`
	Layout string `yaml:"layout"`
	Render struct {
		Enabled             *bool  `yaml:"enabled"`
		CompositionPath     string `yaml:"compositionPath"`
		FunctionsPath       string `yaml:"functionsPath"`
		EnvConfigPath       string `yaml:"envConfigPath"`
		FunctionsDevTargets string `yaml:"functionsDevTargets"`
	} `yaml:"render"`
	Branding struct {
		Title   string            `yaml:"title"`
		LogoURL string            `yaml:"logoUrl"`
		Theme   map[string]string `yaml:"theme"`
	} `yaml:"branding"`
	Assists struct {
		Model   string `yaml:"model"`
		BaseURL string `yaml:"baseUrl"`
	} `yaml:"assists"`
	Auth struct {
		Mode        string `yaml:"mode"`
		RedirectURL string `yaml:"redirectUrl"`
	} `yaml:"auth"`
}

// loadFile reads wizard.yaml. Path from WIZARD_CONFIG, default
// /config/wizard.yaml. Absent + unset ⇒ zero fileConfig (pure-env path). Absent
// but WIZARD_CONFIG explicitly set ⇒ error (a mount that didn't land is a bug,
// not a silent fallback to the wrong repo/XRD defaults on someone else's platform).
func loadFile() (fileConfig, error) {
	var fc fileConfig
	path := os.Getenv("WIZARD_CONFIG")
	explicit := path != ""
	if !explicit {
		path = "/config/wizard.yaml"
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) && !explicit {
			return fc, nil
		}
		return fc, fmt.Errorf("read config %s: %w", path, err)
	}
	if err := rejectSecrets(data, path); err != nil {
		return fc, err
	}
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&fc); err != nil {
		return fc, fmt.Errorf("parse config %s: %w", path, err)
	}
	return fc, nil
}

// secretKeys are configuration keys that must never appear in wizard.yaml —
// they carry credentials and are environment-only (NFR-002). Compared after
// lowercasing and stripping '_' and '-', so githubClientSecret,
// GITHUB_CLIENT_SECRET, and github-client-secret all match.
var secretKeys = map[string]bool{
	"githubclientsecret": true,
	"sessionkey":         true,
	"llmapikey":          true,
	"clientsecret":       true,
	"secret":             true,
	"secrets":            true,
}

// rejectSecrets fails the load if any secret-bearing key appears anywhere in the
// file, with a clear message pointing at the environment. This is the explicit,
// friendly guard; strict decoding is the backstop for everything else.
func rejectSecrets(data []byte, path string) error {
	var raw any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("parse config %s: %w", path, err)
	}
	var found string
	var walk func(node any)
	walk = func(node any) {
		if found != "" {
			return
		}
		m, ok := node.(map[string]any)
		if !ok {
			return
		}
		for k, v := range m {
			norm := strings.NewReplacer("_", "", "-", "").Replace(strings.ToLower(k))
			if secretKeys[norm] {
				found = k
				return
			}
			walk(v)
		}
	}
	walk(raw)
	if found != "" {
		return fmt.Errorf("config %s: secret-bearing key %q is not allowed in the file — supply secrets via the environment (GITHUB_CLIENT_SECRET, SESSION_KEY, LLM_API_KEY)", path, found)
	}
	return nil
}

// parseKVList parses "k=v,k=v" into a map. Empty/malformed entries are skipped.
// Returns nil for empty input so callers can test presence with len()>0.
func parseKVList(s string) map[string]string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	out := map[string]string{}
	for _, pair := range strings.Split(s, ",") {
		k, v, ok := strings.Cut(strings.TrimSpace(pair), "=")
		k, v = strings.TrimSpace(k), strings.TrimSpace(v)
		if ok && k != "" && v != "" {
			out[k] = v
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// defaultRepoRoot best-efforts the repo root for local dev: the current
// working directory. Overridable via REPO_ROOT.
func defaultRepoRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return wd
}
