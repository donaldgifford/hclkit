# CLAUDE.md

Per-repo orientation for `donaldgifford/hclkit`. This file is a
Go-shaped overlay on top of the universal homelab `CLAUDE.md` (see
[homelab/docs](https://github.com/donaldgifford/docs)); the universals
apply here too — only Go-specific guidance is captured below.

## What this is

`hclkit` is a Go binary / library scaffold maintained as part of the
homelab fleet:

- Single binary under `cmd/hclkit/`; library code under
  `internal/` (private to the module).
- Released as multi-arch (linux+darwin × amd64+arm64) archives via
  `goreleaser`, with SBOMs (`syft`) and signed checksums (`gpg`).
- CI runs on GitHub Actions (`.github/workflows/`); the repo is the
  GitHub-side mirror of a Forgejo source of truth.
- Documentation lifecycle (RFC / ADR / DESIGN / IMPL / PLAN / INV) is
  managed by `docz` under `docs/` and published via MkDocs +
  Backstage TechDocs (`catalog-info.yaml`, `mkdocs.yml`).

## Layout

```
cmd/hclkit/             main package — cobra subcommands, kept thin; logic lives in the library
pkg/hclkit/             public library API (Loader, Diagnostics, options, EvalCtxBuilder); consumers import this
pkg/hclkit/funcs/       std HCL function bundle (case helpers, now, env) — imports only hcl/cty+stdlib
pkg/hclkit/varsfile/    variable-block decode + vars-file resolution primitives
pkg/hclkit/ctytypes/    refined decode helpers (Duration, Enum) with HCL-position diagnostics
pkg/hclkit/partial/     hcldec spec decode w/ retained exprs + ordered block walks
internal/parser/        hclparse wrapper (extension dispatch, file map for diagnostics)
internal/testutil/      test-only golden/fixture helpers — import from _test.go files only
docs/                   docz-managed: rfc/ adr/ design/ impl/ plan/ investigation/
scripts/                repo automation (e.g. labels.sh for GitHub label sync)
.github/workflows/      CI (ci, security, codeql, trufflehog, release, changelog, license-check, pr-labels, dependabot-severity-label, changelog-regen)
.goreleaser.yml         release config (multi-arch archives + SBOMs + signed checksums)
.golangci.yml           lint config (Uber-style; v2 schema)
.codecov.yml            coverage gate (project target 60%, threshold 40%; ignores main.go/docs/scripts/examples)
.docz.yaml              docz config (six doc types, MkDocs wiki integration)
mkdocs.yml              wiki site config
cliff.toml              git-cliff config for CHANGELOG.md
catalog-info.yaml       Backstage entity descriptor
mise.toml               pinned go + lint/format/security/release toolchain
justfile                `just` task runner — `just` (no args) for the menu
Makefile                mirror of the justfile target set (`make help`); keep in sync
renovate.json5          extends donaldgifford/renovate-config (go + docker + mise + ci)
.forge-lock.hcl         fleet lock file (homelab-wide)
```

## Workflows

### Build + run

- `just build` — builds to `build/bin/hclkit` with `-ldflags` for
  `main.version` / `main.commit` / `main.date`.
- `just run` — builds then runs the resulting binary.
- `just clean` — removes `build/bin/`, `coverage.out`, and the Go
  build cache.

### Test

- `just test` — race detector, no coverage.
- `just test-coverage` — race + writes `coverage.out`.
- `just coverage-gate` — fails if any `internal/...` or `pkg/...`
  package covers less than 55% (the `coverage_min` in the justfile).
  Tighter than the Codecov project gate (60% w/ 40% threshold).
- `just test-pkg ./internal/foo` — single package.
- `just test-integration` — `//go:build integration` tests (the
  end-to-end consumer-pattern tests under `examples/`).
- `just bench` — benchmarks across the repo (`./...`). The load/decode
  benchmarks live in `pkg/hclkit/bench_test.go` over
  `pkg/hclkit/testdata/bench/` fixtures (forge-blueprint and
  repo-guardian-policy shapes).

### Lint + format

- `just lint` — `golangci-lint run ./...` (Uber-style config; covers
  errcheck, errorlint, gocritic, gosec, revive, etc.).
- `just lint-fix` — same with `--fix`.
- `just lint-config` — `golangci-lint config verify`.
- `just lint-actions` — `actionlint` over `.github/workflows/`.
- `just fmt` — `gofmt -s -w .` + `goimports -w -local github.com/donaldgifford .`.
- `just lint` does **not** run yamllint/markdownlint/prettier today;
  those tools are pinned in `mise.toml` but need to be invoked
  directly (or wired into the recipe).

### License compliance

- `just license-check` — `go-licenses check ./...` against the allow
  list (`Apache-2.0,MIT,BSD-2-Clause,BSD-3-Clause,ISC,MPL-2.0`).
- `just license-report` — CSV via `.github/licenses-csv.tpl`.

### Release

- **Releases are label-driven, not manual.** Merging a PR to main
  fires `release.yml`: `pr-semver-bump` reads the merged PR's semver
  label, pushes the bumped `v*` tag, and goreleaser publishes
  multi-arch archives, SBOMs, and a GPG-signed checksum. Every PR
  must carry exactly one of `major`/`minor`/`patch`/`dont-release`
  (enforced by the required-labels check); use `dont-release` for
  docs-only or no-release PRs.
- `just release-check` — `goreleaser check`.
- `just release-local` — snapshot build (`--snapshot --skip=publish
  --skip=sign`); artifacts land in `dist/`.
- `just release v0.1.0` — manual tag + push. **Escape hatch only:**
  `release.yml` does not fire on tag pushes, so a manual tag
  publishes nothing and can collide with the next auto-bump. Prefer
  the label flow.

### Composite gates

- `just check` — lint + test (pre-commit).
- `just ci` — lint + test + build + license-check (full CI gate).

### Documentation

- Authored via `docz`; one type per directory under `docs/`.
- `docz` is pinned in `mise.toml` (`github:donaldgifford/docz`).
- `.docz.yaml` controls type config and `index.auto_update: true`
  keeps per-type `README.md` indexes in sync.
- `mkdocs.yml` consumes the same tree for Backstage TechDocs.
- Changelogs are generated by `git-cliff` (see `cliff.toml`) via the
  `changelog.yml` / `changelog-regen.yml` workflows.

## Go-specific conventions

- **`go.mod` go directive matches `mise.toml`** (currently `go 1.26.3`).
  Bump both together — Renovate's Go updater handles `go.mod`; bump
  `mise.toml` in the same commit.
- **No `vendor/`**. Modules are resolved at build time.
- **`internal/` is a hard wall** — packages there can't be imported by
  other modules. The public API lives in `pkg/hclkit` (per RFC-0001;
  hclkit is a fleet library, so a public surface is the point);
  implementation details stay under `internal/`.
- **`Diagnostics` implements both `error` and `io.WriterTo`** — the
  embed makes discarded `Load*` returns trip errcheck in consumers;
  don't refactor the embed into a wrapped field.
- **`slog` for structured logs**, not `log` or third-party loggers.
  Default handler is set in `main()` so library code doesn't have to
  thread loggers.
- **No `init()` for behavior**. `init()` runs at import time — it breaks
  test isolation and surprises future-you. Wire dependencies in `main()`.
- **Tests live next to the code** (`foo_test.go` alongside `foo.go`).
  Integration tests that need external services go under
  `//go:build integration` and run via `go test -tags=integration ./...`.
- **Errors wrap with `%w`**: `fmt.Errorf("loading config: %w", err)`.
  Top of the call stack handles via `errors.Is` / `errors.As`.
- **Import grouping**: `gci` enforces `standard → default →
  prefix(github.com/donaldgifford)`. Run `just fmt` rather than
  hand-ordering imports.

## CI

`.github/workflows/` (no Forgejo workflows currently checked in):

- `ci.yml` — labeler + lint + test (with `just test-coverage` /
  `just coverage-gate`) + Codecov upload + govulncheck + Trivy fs
  scan + snapshot goreleaser build with SBOM scan (anchore).
- `security.yml`, `codeql.yml`, `trufflehog.yml` — supplementary
  security scans.
- `license-check.yml` — `go-licenses` against the allow list.
- `changelog.yml` / `changelog-regen.yml` — `git-cliff`-driven.
- `pr-labels.yml` + `.github/labeler.yml` — branch-prefix + path-glob
  labeling. Labels are provisioned via `scripts/labels.sh`.
- `release.yml` — fires on push to main; bumps the version from the
  merged PR's semver label, pushes the tag, and runs `goreleaser
  release --clean` against `.goreleaser.yml`.
- `dependabot-severity-label.yml` — augments Dependabot PRs with
  severity labels (Dependabot config lives in `.github/dependabot.yml`).

## Gotchas

- **`go mod tidy` on first scaffold**: the homelab post-create hook
  runs it. If you skip hooks (`--no-hooks`), run it manually before
  the first `just build` or imports will be unresolved.
- **`goreleaser` v2 config**: `archives[].format` (v1) is now
  `archives[].formats` (slice). If you copy a pre-v2 config from
  elsewhere, validate with `just release-check`.
- **No Dockerfile / container image in-repo** — deliberately deferred
  per DESIGN-0001 open question 4 until a CI integration asks. If you
  add one, mirror the SBOM/signing behavior of `goreleaser` so
  release artifacts stay consistent.
- **Coverage gates differ**: `just coverage-gate` enforces 55% per
  `internal/...` / `pkg/...` package; Codecov enforces 60%
  project-wide w/ 40% threshold. Don't mistake one passing for the
  other.
- **Build output lives under `build/bin/`**, not `bin/`.
- **`coverage.out`, not `coverage.txt`**. The justfile writes
  `coverage.out` and `.gitignore` covers both; CI uploads
  `coverage.out` to Codecov.
- **goreleaser signing requires `GPG_PRIVATE_KEY` + `GPG_FINGERPRINT`**
  in repo Secrets; the release job fails at key import otherwise
  (both set as of 2026-08-01). Use `just release-local` for a
  signing-free snapshot.
- **Committing a regenerated `CHANGELOG.md` on a branch can conflict
  with main's auto-sync commit** — and a conflicted PR silently stops
  triggering `pull_request` workflows (no runs, no failures). If PR
  checks vanish, check mergeability first; merge main in and re-run
  `git-cliff -o CHANGELOG.md` to resolve.
- **PR labels must exist before `pr-labels.yml` can apply them**.
  Run `scripts/labels.sh` once per repo (or after editing
  `.github/labeler.yml`) to provision them via `gh`.

## Renovate

- `go.mod` updates are PR'd by Renovate's Go module manager.
- `mise.toml` versions are handled by a custom regex manager
  configured upstream in `donaldgifford/renovate-config`.
- `renovate.json5` only extends the shared presets — keep
  repo-specific tuning there, not inline.
