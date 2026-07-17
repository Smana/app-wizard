# Configuration reference

The wizard is configured by an optional **`wizard.yaml`** file overlaid by the
**environment**. Precedence is **defaults → `wizard.yaml` → environment** (env
wins, 12-factor). The file path comes from `WIZARD_CONFIG` (default
`/config/wizard.yaml`); if it is unset and the default path is absent, the
wizard runs on pure environment/defaults. An explicitly-set-but-missing
`WIZARD_CONFIG` is a hard error — never a silent fallback.

See [`examples/wizard.yaml`](../examples/wizard.yaml) for a complete file.

## Secrets are environment-only

Credentials are **never** read from `wizard.yaml` — supplying any secret-bearing
key there fails the load closed. Set them in the environment:

| Env var | Purpose |
|---------|---------|
| `GITHUB_CLIENT_SECRET` | GitHub OAuth app secret (`github` auth mode) |
| `SESSION_KEY` | Signs the session cookie (a random ephemeral key is generated when unset — sessions won't survive a restart) |
| `LLM_API_KEY` | LLM assist API key (enables assists) |

## `wizard.yaml` keys

Every key is optional; the default reproduces a cloud-native-ref-style deployment.
Each has an environment override (env wins).

| `wizard.yaml` | Env override | Default | Purpose |
|---------------|--------------|---------|---------|
| `repo.owner` | `REPO_OWNER` | `Smana` | GitOps repo owner PRs open against |
| `repo.name` | `REPO_NAME` | `cloud-native-ref` | GitOps repo name |
| `repo.baseBranch` | `REPO_BASE_BRANCH` | `main` | PR base branch |
| `schema.xrdPath` | `XRD_PATH` | `infrastructure/base/crossplane/configuration/app-definition.yaml` | XRD the form is generated from (resolved under `REPO_ROOT`) |
| `schema.stacksPath` | `STACKS_PATH` | `apps/stacks.yaml` | Stack registry (dropdown) |
| `schema.uiHintsPath` | `UI_HINTS_PATH` | `<repo-root>/ui-hints.yaml` | Presentation overlay |
| `layout` | `LAYOUT` | `apps/{stack}/{app}` | PR file-layout template (`{stack}`/`{app}` tokens; last segment is the app dir) |
| `render.enabled` | `RENDER_ENABLED` | `true` | Crossplane render preview + PR comment. When `false`, validation + PR still work |
| `render.compositionPath` | `COMPOSITION_PATH` | `.../app-composition.yaml` | Composition for `crossplane render` |
| `render.functionsPath` | `FUNCTIONS_PATH` | `.../functions.yaml` | Functions for `crossplane render` |
| `render.envConfigPath` | `ENVCONFIG_PATH` | `.../environmentconfig.yaml` | EnvironmentConfig for `crossplane render` |
| `render.functionsDevTargets` | `FUNCTIONS_DEV_TARGETS` | — | `name=host:port,…` in-pod function gRPC endpoints (sidecar render) |
| `branding.title` | `BRAND_TITLE` | `App Wizard` | App title (header + document title) |
| `branding.logoUrl` | `BRAND_LOGO_URL` | — | Header logo URL (no logo when unset) |
| `branding.theme` | — (file-only) | — | CSS custom properties applied to `:root` (keys map to the wizard's `--var` names) |
| `assists.model` | `LLM_MODEL` | `claude-opus-4-8` | Assist model id |
| `assists.baseUrl` | `LLM_BASE_URL` | — | Anthropic-compatible endpoint (also marks assists available) |
| `auth.mode` | `AUTH_MODE` | `github` | `github` (OAuth) or `dev` (local bypass) |
| `auth.redirectUrl` | `OAUTH_REDIRECT_URL` | `http://localhost:8080/api/auth/callback` | OAuth callback |

Other environment-only knobs: `LISTEN_ADDR` (default `:8080`), `REPO_ROOT`
(on-disk repo root the local source and renderer read from), `XRD_SOURCE`
(`local`/`github`), `GITHUB_CLIENT_ID`.

## The claim GVK is not configured

The claim's `apiVersion` and `kind` are **derived from the XRD** — `spec.group` +
the first served version, and `spec.claimNames.kind` (or `spec.names.kind`). Point
`schema.xrdPath` at your XRD and the wizard follows it; there is nothing to set.
