# hclkit — task runner (Makefile)
#
# Mirror of the justfile: both expose the same target set with
# equivalent behavior. Keep the two in sync when editing recipes.

.DEFAULT_GOAL := help

PROJECT_NAME     ?= hclkit
PROJECT_OWNER    ?= donaldgifford
GO_PACKAGE       := github.com/$(PROJECT_OWNER)/$(PROJECT_NAME)
BUILD_DIR        := build
BIN_DIR          := $(BUILD_DIR)/bin
COVERAGE_OUT     := coverage.out
ALLOWED_LICENSES := Apache-2.0,MIT,BSD-2-Clause,BSD-3-Clause,ISC,MPL-2.0
GOIMPORTS_LOCAL  := github.com/$(PROJECT_OWNER)
COVERAGE_MIN     ?= 55

# Version info derived from git; falls back to dev when not in a repo or tag-less.
COMMIT_HASH := $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
VERSION     := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
BUILD_DATE  := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

##@ General

.PHONY: help
help: ## Show this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) }' $(MAKEFILE_LIST)

##@ Build

.PHONY: build
build: build-core ## Build everything (core)

.PHONY: build-core
build-core: ## Build the core CLI binary into build/bin/hclkit
	@mkdir -p $(BIN_DIR)
	@go build -ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT_HASH) -X main.date=$(BUILD_DATE)" \
		-o $(BIN_DIR)/$(PROJECT_NAME) ./cmd/$(PROJECT_NAME)
	@echo "✓ Core binaries built"

.PHONY: clean
clean: ## Remove build artifacts and the Go build cache
	@rm -rf $(BIN_DIR)/
	@rm -f $(COVERAGE_OUT)
	@go clean -cache
	@find . -name "*.test" -delete
	@echo "✓ Cleaned build artifacts"

##@ Run

.PHONY: run
run: build ## Build then run the CLI
	@$(BIN_DIR)/$(PROJECT_NAME)

##@ Test

.PHONY: test
test: ## Run all tests with the race detector
	@go test -v -race ./...

.PHONY: test-pkg
test-pkg: ## Test a single package: make test-pkg PKG=./pkg/foo
	@go test -v -race $(PKG)

.PHONY: test-integration
test-integration: ## Run integration tests (build tag: integration)
	@go test -v -race -tags=integration ./...

.PHONY: test-coverage
test-coverage: ## Run tests with a coverage profile written to coverage.out
	@go test -v -race -coverprofile=$(COVERAGE_OUT) ./...

.PHONY: test-report
test-report: ## Run tests and open the HTML coverage report
	@go test -coverprofile=$(COVERAGE_OUT) ./...
	@go tool cover -html=$(COVERAGE_OUT)

.PHONY: coverage-gate
coverage-gate: ## Fail if any internal/ or pkg/ package covers less than COVERAGE_MIN%
	@go test -cover ./internal/... ./pkg/... 2>&1 | awk -v min=$(COVERAGE_MIN) '\
		/coverage:/ { \
			if ($$0 ~ /no statements/) next; \
			pct = $$0; \
			sub(/.*coverage: /, "", pct); \
			sub(/% of statements.*/, "", pct); \
			if (pct + 0 < min + 0) { \
				printf "FAIL: %s at %s%% (min %s%%)\n", $$2, pct, min; \
				bad = 1; \
			} \
		} \
		END { exit bad }'

.PHONY: bench
bench: ## Run benchmarks
	@go test -run='^$$' -bench=. -benchmem ./...

##@ Lint & format

.PHONY: lint
lint: ## Run golangci-lint
	@golangci-lint run ./...

.PHONY: lint-fix
lint-fix: ## Run golangci-lint with --fix
	@golangci-lint run --fix ./...

.PHONY: lint-config
lint-config: ## Verify the golangci-lint configuration
	@golangci-lint config verify

.PHONY: lint-actions
lint-actions: ## Lint GitHub Actions workflows
	@actionlint

.PHONY: fmt
fmt: ## Format code with gofmt + goimports
	@gofmt -s -w .
	@goimports -w -local $(GOIMPORTS_LOCAL) .

##@ License compliance

.PHONY: license-check
license-check: ## Check dependency licenses against the allow list
	@go-licenses check ./... --allowed_licenses=$(ALLOWED_LICENSES)

.PHONY: license-report
license-report: ## Generate CSV report of all dependency licenses
	@go-licenses report ./... --template=.github/licenses-csv.tpl

##@ Release

.PHONY: release-check
release-check: ## Validate the goreleaser config
	@goreleaser check

.PHONY: release-local
release-local: ## Snapshot release locally (no publish, no sign)
	@goreleaser release --snapshot --clean --skip=publish --skip=sign

.PHONY: release
release: ## Tag and push a new release: make release TAG=v0.1.0
	@test -n "$(TAG)" || { echo "usage: make release TAG=v0.1.0"; exit 1; }
	@git tag -a $(TAG) -m "Release $(TAG)"
	@git push origin $(TAG)

##@ Composite gates

.PHONY: check
check: lint test ## Pre-commit gate: lint + test
	@echo "✓ Pre-commit checks passed"

.PHONY: ci
ci: lint test build license-check ## Full CI gate: lint + test + build + license-check
	@echo "✓ CI pipeline complete"
