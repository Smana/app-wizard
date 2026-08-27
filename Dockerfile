# syntax=docker/dockerfile:1
# App Wizard — single Go binary serving the embedded React SPA (SPEC-008).
#
# Stage 1: build the SPA into internal/web/dist.
# Stage 2: build the Go binary embedding that dist via go:embed.
# Stage 3: fetch the crossplane CLI + core engine (the render preview needs both).
# Stage 4: distroless runtime, non-root.

ARG APP_WIZARD_VERSION=v0.1.0

# The render preview runs `crossplane render` as a subprocess
# (internal/render/crossplane.go: Binary defaults to "crossplane"), so the CLI has to
# be IN the image — distroless ships nothing but what we copy. Without it the preview
# fails with:
#   crossplane render failed: exec: "crossplane": executable file not found in $PATH
#
# Kept in step with the Crossplane running on the cluster so the renderer behaves
# the way the cluster will — bump this whenever that Helm chart version moves.
ARG CROSSPLANE_VERSION=v2.4.0

# ---------- UI build ----------
FROM node:22-alpine AS ui
WORKDIR /src/ui
COPY ui/package.json ui/package-lock.json* ./
RUN npm ci || npm install
COPY ui/ ./
RUN npm run build

# ---------- Go build ----------
FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
# Overwrite the committed placeholder with the real SPA build output.
COPY --from=ui /src/internal/web/dist ./internal/web/dist
ARG TARGETOS
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w" -o /out/app-wizard ./cmd/app-wizard

# ---------- crossplane binaries ----------
# The render preview needs BOTH halves, under two different names on $PATH:
#
#   crossplane       the CLI / driver. `crossplane render` is here.
#   crossplane-core  the engine. `crossplane render` shells out to
#                    `<--crossplane-binary> internal render`, which ONLY the core
#                    binary implements; the CLI answers `unexpected argument
#                    internal` (exit 80).
#
# They ship from two different places, and both moved:
#   - releases.crossplane.io published a bare `crank` (the CLI) up to v2.3.4. From
#     v2.3.5 it publishes only the CORE binary, confusingly named `crossplane`.
#   - The CLI moved to cli.crossplane.io as a crossplane-cli.tar.gz bundle
#     (see upstream install.sh).
# Fetching `crank` from the old location 404s on anything >= v2.3.5.
#
# Integrity: the CORE binary is verified against its published .sha256 (a bare hex
# digest with no filename, so `sha256sum -c` cannot parse it and the compare is
# done by hand). The CLI bundle is NOT checksum-verified, deliberately — the
# crossplane.sha256 inside the tarball matches neither the binary it ships with
# nor the tarball itself, and upstream's own install.sh deletes that file without
# ever checking it (`rm "${BIN}.sha256"`). Asserting on it fails the build on a
# perfectly good download. Do not "fix" this by re-adding the check; the CLI's
# integrity gate is the `version --client` discriminator below.
FROM alpine:3.24 AS crossplane-bins
ARG TARGETOS
ARG TARGETARCH
ARG CROSSPLANE_VERSION
RUN set -eux; \
    apk add --no-cache curl; \
    \
    # --- CLI (driver) ---
    curl -fsSL -o /tmp/cli.tar.gz \
      "https://cli.crossplane.io/stable/${CROSSPLANE_VERSION}/bundle/${TARGETOS}_${TARGETARCH}/crossplane-cli.tar.gz"; \
    tar xzf /tmp/cli.tar.gz -C /tmp; \
    mv /tmp/crossplane /out-crossplane; \
    chmod 0755 /out-crossplane; \
    rm -f /tmp/crossplane.sha256; \
    \
    # --- core engine ---
    # Published directly since v2.3.5, so this no longer pulls the whole
    # controller image and copies its rootfs to dig a binary out of /nix/store.
    curl -fsSL -o /out-crossplane-core \
      "https://releases.crossplane.io/stable/${CROSSPLANE_VERSION}/bin/${TARGETOS}_${TARGETARCH}/crossplane"; \
    curl -fsSL -o /tmp/core.sha256 \
      "https://releases.crossplane.io/stable/${CROSSPLANE_VERSION}/bin/${TARGETOS}_${TARGETARCH}/crossplane.sha256"; \
    expected="$(cat /tmp/core.sha256)"; \
    actual="$(sha256sum /out-crossplane-core | cut -d' ' -f1)"; \
    [ "$expected" = "$actual" ] || { echo "core checksum mismatch: $actual != $expected" >&2; exit 1; }; \
    chmod 0755 /out-crossplane-core; \
    \
    # --- prove each artifact is the half we think it is ---
    # These are real discriminators, not smoke tests: only the CLI has
    # `version --client`, and only the core has `internal render`. Swapping the
    # two URLs would otherwise ship an image whose preview fails at runtime.
    /out-crossplane version --client; \
    /out-crossplane-core internal render --help >/dev/null

# ---------- Runtime ----------
FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /
COPY --from=build /out/app-wizard /app-wizard
# The renderer resolves "crossplane" (driver) and "crossplane-core" (engine) through
# $PATH; distroless sets PATH to /usr/local/bin:/usr/bin:/bin, so these land on it.
COPY --from=crossplane-bins /out-crossplane /usr/local/bin/crossplane
COPY --from=crossplane-bins /out-crossplane-core /usr/local/bin/crossplane-core
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/app-wizard"]
