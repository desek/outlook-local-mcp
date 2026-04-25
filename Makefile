.PHONY: build test lint fmt fmt-check vet tidy ci verify govulncheck security vuln-scan license-check clean snapshot goreleaser-check build-mcpb-binaries build-mcpb-local mcpb-validate mcpb-pack mcpb-local mcpb-clean docs-bundle

BINARY_NAME := outlook-local-mcp
BUILD_DIR := .
CMD_PATH := ./cmd/outlook-local-mcp/

build:
	go build -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_PATH)

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

ci: docs-bundle build vet fmt-check tidy lint test goreleaser-check mcpb-validate

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
	@echo "==> Regenerating docs bundle (internal/docs/files/)"
	@mkdir -p internal/docs/files
	@cp README.md internal/docs/files/readme.md
	@cp QUICKSTART.md internal/docs/files/quickstart.md
	@cp docs/troubleshooting.md internal/docs/files/troubleshooting.md
	@echo "==> Verifying slugs resolve"
	@CGO_ENABLED=0 go test ./internal/docs/... -run TestCatalog_AllSlugsResolve -v
	@echo "==> Enforcing 2 MiB size budget"
	@CGO_ENABLED=0 go test ./internal/docs/... -run TestBundleSizeUnder2MiB -v
	@echo "==> Running secret-pattern lint"
	@for pat in eyJ sk- client_secret refresh_token; do \
		if grep -rq "$$pat" internal/docs/files/; then \
			echo "ERROR: secret pattern '$$pat' found in docs bundle" && exit 1; \
		fi; \
	done
	@echo "==> docs-bundle OK"

clean:
	rm -f $(BUILD_DIR)/$(BINARY_NAME) $(BUILD_DIR)/$(BINARY_NAME)-* coverage.out $(BINARY_NAME).cdx.json $(BINARY_NAME).spdx.json $(BINARY_NAME).license.cdx.json
	rm -rf dist/

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
