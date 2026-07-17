.PHONY: dev ui-build build test

# Run the wizard locally against examples/ — dev auth (no GitHub), render preview
# off, so it works offline with no Crossplane cluster. Open http://localhost:8080.
dev: ui-build
	WIZARD_CONFIG=examples/wizard.yaml REPO_ROOT=. go run ./cmd/app-wizard

# Build the SPA into internal/web/dist so the Go binary embeds it.
ui-build:
	cd ui && npm ci && npm run build

# Build the single binary (SPA embedded).
build: ui-build
	CGO_ENABLED=0 go build -trimpath -o app-wizard ./cmd/app-wizard

# Backend + frontend test suites.
test:
	go test ./...
	cd ui && npm test
