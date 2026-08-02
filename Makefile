.PHONY: build test lint fmt fmt-check vet tidy ci verify govulncheck security vuln-scan license-check clean snapshot goreleaser-check build-mcpb-binaries build-mcpb-local mcpb-validate mcpb-pack mcpb-local mcpb-clean docs-bundle crud-test surface-manifest surface-check

BINARY_NAME := outlook-local-mcp
BUILD_DIR := .
CMD_PATH := ./cmd/outlook-local-mcp/

build:
	go build -ldflags="-X main.commit=$$(git rev-parse --short HEAD) -X main.buildDate=$$(date -u +%Y-%m-%dT%H:%M:%SZ)" -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_PATH)

test:
	CGO_ENABLED=0 go test -coverprofile=coverage.out ./...

lint:
	golangci-lint run

fmt:
	gofmt -w .
	goimports -w .

fmt-check:
	@test -z "$$(gofmt -l .)" || (echo "Unformatted files:" && gofmt -l . && exit 1)

vet:
	go vet ./...

tidy:
	go mod tidy
	@git diff --exit-code go.mod go.sum || (echo "go.mod or go.sum not tidy" && exit 1)

ci: docs-bundle surface-check build vet fmt-check tidy lint test goreleaser-check mcpb-validate

# Regenerate the code-derived surface manifest that the website consumes. The
# generator reads the live verb builders and the configuration inventory and
# writes site/src/generated/surface.json; it needs no credentials or network
# (CR-0073 FR-7). This is the command a contributor runs after adding, renaming,
# or removing a verb, a domain, or a configuration variable.
surface-manifest:
	@echo "==> Regenerating surface manifest"
	@CGO_ENABLED=0 go run cmd/gen-surface/main.go
	@echo "==> surface manifest written to site/src/generated/surface.json"

# The drift check wired into `make ci`: regenerate the manifest and fail if the
# working tree changed as a result, so a stale manifest cannot merge (CR-0073
# FR-22, FR-25). A separate non-mutating guard, TestCommittedManifestMatchesRecord
# in internal/surface, runs in `make test`; this target adds the git-diff
# semantics the workflows use so the same check reads identically everywhere.
surface-check: surface-manifest
	@git diff --exit-code -- site/src/generated/surface.json \
		|| (echo "ERROR: surface manifest is stale; run 'make surface-manifest' and commit site/src/generated/surface.json" && exit 1)

verify:
	go mod verify

govulncheck:
	govulncheck ./...

security: verify govulncheck vuln-scan license-check

snapshot:
	goreleaser release --snapshot --clean

goreleaser-check:
	goreleaser check

vuln-scan: build
	syft scan $(BUILD_DIR)/$(BINARY_NAME) -o cyclonedx-json=$(BINARY_NAME).cdx.json
	grype sbom:$(BINARY_NAME).cdx.json --fail-on high

license-check:
	syft scan dir:. --override-default-catalogers gomod -o cyclonedx-json=$(BINARY_NAME).license.cdx.json
	grant check $(BINARY_NAME).license.cdx.json

docs-bundle:
	@echo "==> Verifying slugs resolve"
	@CGO_ENABLED=0 go test ./internal/docs/... -run TestCatalog_AllSlugsResolve -v
	@echo "==> Enforcing 2 MiB size budget"
	@CGO_ENABLED=0 go test ./internal/docs/... -run TestBundleSizeUnder2MiB -v
	@echo "==> Running secret-pattern lint"
	@for pat in eyJ sk- client_secret refresh_token; do \
		if grep -rq "$$pat" docs/*.md; then \
			echo "ERROR: secret pattern '$$pat' found in docs bundle" && exit 1; \
		fi; \
	done
	@echo "==> Regenerating llms.txt"
	@CGO_ENABLED=0 go run cmd/gen-llms/main.go
	@echo "==> Verifying llms.txt matches catalog"
	@CGO_ENABLED=0 go test ./internal/docs/... -run TestLLMsTxt_MatchesCatalog -v
	@echo "==> docs-bundle OK"

clean:
	rm -f $(BUILD_DIR)/$(BINARY_NAME) $(BUILD_DIR)/$(BINARY_NAME)-* coverage.out $(BINARY_NAME).cdx.json $(BINARY_NAME).spdx.json $(BINARY_NAME).license.cdx.json
	rm -rf dist/

# Headless CRUD lifecycle test runner. Wraps scripts/crud-test.sh which spawns
# claude -p, captures stream-json metrics to docs/bench/runs/{ts}/, appends a
# row to docs/bench/crud-runs.csv, and writes TEST-REPORT-{ts}.md.
# Override defaults with env: ACCOUNT (default), MODEL (claude-sonnet-5),
# THINKING (low|medium|high|xhigh|max).
crud-test:
	./scripts/crud-test.sh

# MCPB extension packaging targets
EXTENSION_DIR := extension
EXTENSION_BIN := $(EXTENSION_DIR)/bin

build-mcpb-binaries: snapshot
	@mkdir -p $(EXTENSION_BIN)
	cp dist/outlook-local-mcp-darwin-arm64 $(EXTENSION_BIN)/outlook-local-mcp-darwin-arm64
	cp dist/outlook-local-mcp-windows-amd64.exe $(EXTENSION_BIN)/outlook-local-mcp-win32-x64.exe
	chmod +x $(EXTENSION_BIN)/outlook-local-mcp-darwin-arm64

build-mcpb-local:
	CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 go build -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_PATH)
	@mkdir -p $(EXTENSION_BIN)
	cp $(BUILD_DIR)/$(BINARY_NAME) $(EXTENSION_BIN)/outlook-local-mcp-darwin-arm64
	chmod +x $(EXTENSION_BIN)/outlook-local-mcp-darwin-arm64

mcpb-validate:
	mcpb validate $(EXTENSION_DIR)/manifest.json

mcpb-pack: build-mcpb-binaries mcpb-validate
	mcpb pack $(EXTENSION_DIR) $(BINARY_NAME).mcpb

mcpb-local: build-mcpb-local mcpb-validate
	mcpb pack $(EXTENSION_DIR) $(BINARY_NAME).mcpb

mcpb-clean:
	rm -rf $(EXTENSION_BIN) *.mcpb
