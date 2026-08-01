---
id: IMPL-0001
title: "hclkit v0 library and validator binary"
status: In Progress
author: Donald Gifford
created: 2026-07-02
---
<!-- markdownlint-disable-file MD025 MD041 -->

# IMPL 0001: hclkit v0 library and validator binary

**Status:** In Progress
**Author:** Donald Gifford
**Date:** 2026-07-02

<!--toc:start-->
- [Objective](#objective)
- [Scope](#scope)
  - [In Scope](#in-scope)
  - [Out of Scope](#out-of-scope)
- [Implementation Phases](#implementation-phases)
  - [Phase 1: Library skeleton and first consumer](#phase-1-library-skeleton-and-first-consumer)
    - [Tasks](#tasks)
    - [Success Criteria](#success-criteria)
  - [Phase 2: Low-friction adopters](#phase-2-low-friction-adopters)
  - [Phase 3: EvalContext, vars-file, refined types, and partial-decode](#phase-3-evalcontext-vars-file-refined-types-and-partial-decode)
    - [Tasks](#tasks-1)
    - [Success Criteria](#success-criteria-1)
  - [Phase 4: Cross-block validation, the lint binary, and v1.0](#phase-4-cross-block-validation-the-lint-binary-and-v10)
    - [Tasks](#tasks-2)
    - [Success Criteria](#success-criteria-2)
- [File Changes](#file-changes)
- [Testing Plan](#testing-plan)
- [Dependencies](#dependencies)
- [Open Questions](#open-questions)
  - [1. CLI command framework](#1-cli-command-framework)
  - [2. Coverage-gate scope for pkg/hclkit](#2-coverage-gate-scope-for-pkghclkit)
  - [3. Machine-parseable diagnostic prefix](#3-machine-parseable-diagnostic-prefix)
  - [4. Adopter-gate mechanics](#4-adopter-gate-mechanics)
  - [5. Release tagging cadence](#5-release-tagging-cadence)
  - [6. Fixture sourcing for the partial-decode gate](#6-fixture-sourcing-for-the-partial-decode-gate)
  - [7. hcldec.Spec loader entry point (decide before Phase 3)](#7-hcldecspec-loader-entry-point-decide-before-phase-3)
- [References](#references)
<!--toc:end-->

## Objective

Implement the `hclkit` v0 library and validator binary specified in
DESIGN-0001: the nine mechanism-only primitives (Loader + diagnostics,
EvalContext assembly, standard function bundle, vars-file decode,
refined Duration/Enum `cty` types, cross-block reference validation,
uniqueness validation, and `hcldec`/`hclsyntax` partial-decode
helpers) plus the `hclkit` CLI (`fmt`, `validate`, `lint`, `version`).

The four phases mirror RFC-0001's rollout plan and end with a v1.0.0
tag, after which SemVer applies. Breaking API changes are permitted
until Phase 4 completes.

**Implements:** [DESIGN-0001](../design/0001-hclkit-v0-library-and-validator-binary.md)
(which details [RFC-0001](../rfc/0001-build-hclkit-as-a-mechanism-only-hcl-library.md))

## Scope

### In Scope

- The `pkg/hclkit` public API and its subpackages (`funcs`,
  `varsfile`, `ctytypes`, `partial`, `validate`) per DESIGN-0001's
  package layout.
- The `hclkit` binary: `fmt`, `validate`, `lint --schema`, `version`,
  with the four reserved-but-unimplemented flags documented.
- `internal/parser` and `internal/testutil` implementation packages.
- `examples/` — one example per surveyed consumer pattern (`nilctx`,
  `envfunc`, `varsfile`).
- Repo groundwork the scaffold still needs: HCL/cty dependencies,
  justfile fixes, coverage-gate scope, `main.date` injection,
  CLAUDE.md layout correction.
- Tagging each merged PR (per OQ-5) so consumer migrations have
  releases to pin.

### Out of Scope

- The DSL layer (operators, rule grammar) — gated behind RFC-0001's
  re-trigger condition, evaluated at the end of Phase 4.
- Refined CIDR, URL, port, or regex types (zero current consumers).
- Centralized `Config` schemas, plugin/registry systems, YAML support.
- Schema generation/inference, REPL/eval, LSP, watch mode, color
  theming (each is its own future design discussion).
- The consumer-repo migrations (`claudelint`, `mcp-go-gen`, `spt`,
  `forge`, `fwsync`, `repo-guardian`, `docz`) — moved to
  [IMPL-0002](0002-hclkit-fleet-adoption.md) so this doc contains
  only in-repo work. RFC-0001's per-phase adopter validation still
  applies; it gates IMPL-0002's waves, not the phases here.
- A container image for the binary (deferred per DESIGN-0001 open
  question 4 until a CI integration asks).
- The full `hclkit lint` schema specification — Phase 4 ships the
  minimal grammar; the full spec is a follow-on DESIGN per
  DESIGN-0001 open question 5.

## Implementation Phases

Each phase builds on the previous one. A phase is complete when all
its tasks are checked off and its success criteria are met. All
tasks below are in-repo; the consumer migrations that validate each
phase are tracked as waves in
[IMPL-0002](0002-hclkit-fleet-adoption.md), gated on this repo's
per-PR tags (OQ-5). Phase ordering is ergonomic, not a hard
dependency chain.

---

### Phase 1: Library skeleton and first consumer

Establishes the package layout, the `Loader` API, the `Diagnostics`
wrapper, and the parse-only CLI surface (`fmt`, `validate`,
`version`). The first-adopter validation is IMPL-0002 wave 1.

#### Tasks

Repo groundwork:

- [x] Add `github.com/hashicorp/hcl/v2`, `github.com/zclconf/go-cty`,
      and `github.com/spf13/cobra` to `go.mod`; run `go mod tidy` and
      `just license-check`.
- [x] Create the `pkg/hclkit/` tree per DESIGN-0001's layout; update
      CLAUDE.md's layout/conventions sections (they currently say all
      library code lives under `internal/`, which predates the
      public-API decision in RFC-0001).
- [x] Fix claudelint leftovers in the justfile: `self-check` invokes a
      nonexistent `run` subcommand, `profile` assumes `run --profile`,
      and `bench` targets `./internal/engine/...` which will not exist.
- [x] Extend `just coverage-gate` to cover `pkg/...` (per OQ-2) and add
      `examples/` to `.codecov.yml` ignores.
- [x] Wire `-X main.date=...` into the justfile and `.goreleaser.yml`
      ldflags (closes the CLAUDE.md gotcha; `hclkit version` prints
      version + commit + date per DESIGN-0001).

Library:

- [x] Implement `Diagnostics` (`pkg/hclkit/diag.go`): wrap
      `hcl.Diagnostics` with `WriteTo`, `HasErrors`, `Error`, and the
      machine-parseable line prefix (format per OQ-3).
      *Deviation from DESIGN-0001:* `WriteTo` returns
      `(int64, error)` — govet's stdmethods check requires the
      `io.WriterTo` shape for that name; the design sketched a bare
      `error` return.
- [x] Implement `Loader` (`pkg/hclkit/loader.go`): `New(opts...)`,
      `LoadFile`, `LoadBytes`, and `LoadDir` with lexical-order,
      per-file-override merge (DESIGN-0001 open question 3 decision)
      and a `WithMergeMode(append)` opt-in.
- [x] Implement the Phase-1 functional options: `WithEvalContext`,
      `WithFunctions`, `WithVariables`, `WithDiagnosticWriter`
      (`WithVarsFile` lands in Phase 3, `WithValidators` in Phase 4).
- [x] Implement `internal/parser` wrappers around `hclparse` /
      `hclsyntax`.
- [x] Implement `internal/testutil`: golden-file helpers with a
      `-update` regeneration flag, fixture loading.
- [x] Write unit tests for loader (file/bytes/dir, merge modes, error
      paths) and golden tests for the diagnostic renderer.

CLI:

- [x] Restructure `cmd/hclkit/main.go` into `spf13/cobra` subcommand
      dispatch (OQ-1); keep it thin — parse flags, call into the
      library.
- [x] Implement `hclkit fmt [files...]` via `hclwrite`, with `--check`
      for CI (non-zero exit on unformatted files).
- [x] Implement `hclkit validate [files...]` (parse-only; emits hclkit
      diagnostics; non-zero exit on errors).
- [x] Implement `hclkit version` (version, commit, date).
- [x] Document the reserved flags (`--profile`, `--format`,
      `--no-color`, `--schema-stdin`) as reserved/not implemented.
- [x] Write golden tests for `fmt --check` and `validate` output and
      exit codes.
- [x] Add `examples/nilctx` (the gohcl-with-nil-ctx / claudelint
      shape) and an integration test exercising it behind
      `//go:build integration` (`just test-integration`).

Release:

- [x] Tag each merged PR (per-PR tagging, OQ-5); the phase's latest
      tag is what IMPL-0002 wave 1 (`claudelint`/`mcp-go-gen`) pins.
      *(v0.1.0 auto-tagged on the Phase 1 PR merge, 2026-08-01.)*

#### Success Criteria

- `just ci` (lint + test + build + license-check) passes with the new
  packages in place. *(Verified 2026-07-03 — coverage: pkg/hclkit
  98.2%, internal/parser 100%, internal/format 91.7%,
  internal/testutil 55.6%.)*
- `just coverage-gate` passes at ≥55% for every library package in its
  (possibly extended, per OQ-2) scope. *(Verified 2026-07-03.)*
- `hclkit fmt --check`, `hclkit validate`, and `hclkit version` behave
  per DESIGN-0001 against the `examples/nilctx` fixtures, with correct
  exit codes. *(Verified 2026-07-03 — golden-tested + binary smoke
  runs.)*
- A tagged release exists for the merged Phase 1 PR (OQ-5). The
  adopter validation (RFC-0001 Phase 1 criterion) is IMPL-0002
  wave 1 and does not gate the next phase here.

---

### Phase 2: Low-friction adopters

**Moved to [IMPL-0002](0002-hclkit-fleet-adoption.md) wave 1.** This
phase had no in-repo implementation tasks — it was the
`claudelint`/`mcp-go-gen` migrations plus the feedback
loop they trigger (API-friction fixes, new `examples/` shapes). Any
hclkit changes that feedback produces land as normal PRs here; the
tracking lives in IMPL-0002. The phase number is retained so
cross-references (OQ-7, CLAUDE.md, commit history) stay valid; for
in-repo work, Phase 1 proceeds directly to Phase 3.

---

### Phase 3: EvalContext, vars-file, refined types, and partial-decode

The widest phase: EvalContext assembly, the standard function bundle,
the vars-file decode path, refined `ctytypes`, and the
`pkg/hclkit/partial` surface (the hairiest code in v0). The consumers
this phase serves (**`spt`**, **`forge`**, **`fwsync`**, and the
`repo-guardian` `env()` decision point) are IMPL-0002 wave 2.

#### Tasks

EvalContext + functions:

- [ ] Implement `EvalCtxBuilder` (`pkg/hclkit/evalctx.go`):
      `NewEvalCtx`, `WithStdFuncs`, `WithFunc`, `WithVar`,
      `WithLocals(body)`, `Build` — `WithLocals` mirrors
      repo-guardian's `decodeLocals` so that migration is near-1:1.
- [ ] Implement `pkg/hclkit/funcs`: `env(name)` (configurable env map,
      empty string for missing keys — the canonical `env()`),
      `snakeCase`/`camelCase`/`pascalCase`/`kebabCase` (lifted from
      forge), `now(layout)` (UTC, not memoized across loads).
- [ ] Unit tests for builder composition and every bundled function.

Vars-file:

- [ ] Implement `pkg/hclkit/varsfile`: `variable` block decode
      (`type`, `default`, `validate`, `choices`), assignment-file
      parsing, binding as `var.<name>`.
- [ ] Implement `Loader.LoadVarsFile` → `(*VarsResult, Diagnostics)`
      and the `WithVarsFile(path)` one-shot option.
- [ ] Add `examples/envfunc` (spt shape) and `examples/varsfile`
      (forge shape) with integration tests.

Refined types:

- [ ] Implement `ctytypes.Duration` and `ctytypes.Enum` with
      HCL-position diagnostics.
- [ ] Spike gohcl struct-tag compatibility against `spt`
      (DESIGN-0001 open question 2 decision); if lossy, fall back to
      `Validate(target)` post-decode helpers without changing the
      consumer API shape. Record the outcome in this doc.
- [ ] Property-based tests: cty round-trip preservation, decode-error
      positions match source ranges.

Partial-decode:

- [ ] Implement `partial.DecodeSpec(body, spec, ctx)` returning
      `(cty.Value, ExprMap, Diagnostics)` with retained
      `hcl.Expression` handles for late-bound attributes.
- [ ] Implement `partial.Walk(body, schema, fn)` for ordered
      block-kind iteration (locals-first shape).
- [ ] Add `hcldec.Spec` decoding to the Loader — entry-point shape is
      OQ-7 (the design's type-switch-on-`target` has no return path
      for the decoded `cty.Value`); resolve before implementing.
- [ ] Unit tests for `DecodeSpec` retained-expression flows and `Walk`
      ordering; fixtures per OQ-6.

Benchmarks + release:

- [ ] Add load+decode benchmarks for representative consumer configs
      (a forge blueprint, a repo-guardian policy file); point
      `just bench` at them.
- [ ] Tag each merged PR (per OQ-5); the phase's latest tag is what
      IMPL-0002 wave 2 (`spt`/`forge`/`fwsync`) pins.

#### Success Criteria

- The gohcl × refined-types spike outcome (refined path or
  validating-helper fallback) is recorded in this doc.
- Property tests and integration tests pass; `just bench` runs the
  new benchmarks; coverage gates still hold.
- A tagged release exists for the merged Phase 3 PR(s) (OQ-5). The
  adopter validation (`spt`/`forge`/`fwsync`, RFC-0001 Phase 3
  criterion) is IMPL-0002 wave 2 and does not gate the next phase
  here.

---

### Phase 4: Cross-block validation, the lint binary, and v1.0

Ships the two decode-time validators and the schema-driven
`hclkit lint` subcommand. Ends with the partial-decode test gate,
the DSL re-trigger evaluation, and the v1.0.0 tag — after which
SemVer applies. The final two migrations (**`repo-guardian`**,
**`docz`**) are IMPL-0002 wave 3, which must be green before the
v1.0.0 tag.

#### Tasks

Validators:

- [ ] Define the `Validator` interface and wire `WithValidators(...)`
      into the Loader decode path.
- [ ] Implement `validate.NewRefValidator(verb, targetKind)` —
      collects declared block labels by kind, verifies every
      referenced name resolves, diagnostics anchored at the
      *reference* site.
- [ ] Implement `validate.NewUniqueValidator(blockKind, attribute)`.
- [ ] Unit tests: resolution across files (`LoadDir`), missing
      targets, duplicate detection, diagnostic positions.

Lint binary:

- [ ] Implement the minimal lint-schema grammar: `block`, `attribute`,
      `reference`, `unique` top-level kinds with required/optional
      attributes and refined-type references (attribute names may
      still evolve; full spec deferred per DESIGN-0001 open
      question 5).
- [ ] Implement `hclkit lint --schema=schema.hcl [files...]` mapping
      schema declarations onto the library validators.
- [ ] Golden tests for lint output and exit codes; document the
      schema grammar in the README or docs.

Release gates:

- [ ] Run the partial-decode test pass against real `forge` and
      `repo-guardian` fixtures end-to-end, including EvalContext +
      late-bound expression flows (v1.0 gate; vendored fixtures per
      OQ-6).
- [ ] Evaluate the DSL re-trigger: re-run INV-0001 section C against
      the then-current `claudelint`, `fwsync`, and `repo-guardian`
      rule grammars; record the outcome in RFC-0001's references.
- [ ] Sweep the public API for pre-1.0 regrets (naming, option shapes,
      error contracts) — last chance for breaking changes.
- [ ] Tag each merged PR (per OQ-5); the phase's latest tag is what
      IMPL-0002 wave 3 (`repo-guardian`/`docz`) pins.
- [ ] Tag v1.0.0 (`just release v1.0.0`; requires `GPG_FINGERPRINT`
      in repo Secrets). Waits on IMPL-0002 wave 3 — RFC-0001's
      Phase 4 criterion requires the final adopters green. SemVer
      applies from here.

#### Success Criteria

- The partial-decode gate passed against the vendored forge and
  repo-guardian fixtures (OQ-6).
- The DSL re-trigger evaluation is documented (triggered or not).
- IMPL-0002 wave 3 is green (`repo-guardian` + `docz` against a
  tagged hclkit; `hclkit lint` in CI for two consumers — RFC-0001
  Phase 4 criterion).
- v1.0.0 is tagged with signed, SBOM-carrying release artifacts, and
  the public API is under SemVer.

---

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `go.mod` / `go.sum` | Modify | Add `hashicorp/hcl/v2`, `zclconf/go-cty` |
| `pkg/hclkit/loader.go` | Create | `Loader`, `New`, `LoadFile`/`LoadDir`/`LoadBytes`, options |
| `pkg/hclkit/diag.go` | Create | `Diagnostics` wrapper + renderer |
| `pkg/hclkit/evalctx.go` | Create | `EvalCtxBuilder` (Phase 3) |
| `pkg/hclkit/funcs/` | Create | `env`, case helpers, `now` (Phase 3) |
| `pkg/hclkit/varsfile/` | Create | Vars-file decode + `variable` blocks (Phase 3) |
| `pkg/hclkit/ctytypes/` | Create | `Duration`, `Enum` (Phase 3) |
| `pkg/hclkit/partial/` | Create | `DecodeSpec`, `ExprMap`, `Walk` (Phase 3) |
| `pkg/hclkit/validate/` | Create | `RefValidator`, `UniqueValidator` (Phase 4) |
| `internal/parser/` | Create | `hashicorp/hcl` wrappers |
| `internal/testutil/` | Create | Golden-file + fixture helpers |
| `cmd/hclkit/main.go` | Modify | Subcommand dispatch; stays thin |
| `cmd/hclkit/` (subcommand files) | Create | `fmt`, `validate`, `lint`, `version` |
| `examples/{nilctx,envfunc,varsfile}/` | Create | One example per surveyed consumer pattern |
| `justfile` | Modify | Coverage-gate scope, bench target, `main.date`, claudelint leftovers |
| `.goreleaser.yml` | Modify | Add `-X main.date` ldflag |
| `.codecov.yml` | Modify | Ignore `examples/` |
| `CLAUDE.md` | Modify | Layout: `pkg/hclkit` is the public API surface |

## Testing Plan

- [x] Unit tests colocated with packages; `just coverage-gate` floor
      of 55% per library package (scope per OQ-2); Codecov project
      gate 60%/40% unchanged.
- [x] Golden tests for the diagnostic renderer and all CLI output,
      regenerated via the `-update` flag in `internal/testutil`.
- [ ] Integration tests behind `//go:build integration` — at minimum
      one end-to-end test per `examples/` pattern (`nilctx`,
      `envfunc`, `varsfile`).
- [ ] Table-driven tests for merge modes, option combinations, and
      every bundled function.
- [ ] Property-based tests for `Duration`/`Enum` round-trips and
      diagnostic positions.
- [ ] Benchmarks for load+decode of representative consumer configs,
      wired to `just bench`.
- [ ] Partial-decode test pass on real forge + repo-guardian fixtures
      before tagging v1.0 (Phase 4 gate).

## Dependencies

- `github.com/hashicorp/hcl/v2` (MPL-2.0 — on the license allow list),
  `github.com/zclconf/go-cty` (MIT), and `github.com/spf13/cobra`
  (Apache-2.0) per OQ-1.
- Consumer migrations are tracked in
  [IMPL-0002](0002-hclkit-fleet-adoption.md); only the v1.0.0 tag
  (Phase 4) waits on them. Gate mechanics per OQ-4.
- `GPG_FINGERPRINT` in repo Secrets for signed release tags
  (`just release-local` for signing-free snapshots).
- DESIGN-0001's six open questions are all decided; this doc inherits
  those decisions and adds only implementation-level questions below.

## Open Questions

Each question lists `a` as the recommended option, `b`+ as
alternatives, and `other` as a free-form slot for the decision-maker
to fill in.

### 1. CLI command framework

DESIGN-0001 hard-caps the binary at four subcommands but doesn't say
how they're dispatched.

- **a.** _Recommended._ **Stdlib `flag` + hand-rolled subcommand
  dispatch.** Four subcommands, a handful of flags, zero new
  dependencies to license-check, and no framework gravity pulling the
  "deliberately small" binary toward kitchen-sink territory
  (RFC-0001's scope-creep risk).
- **b.** **`spf13/cobra`.** Free help/completion/usage text and a
  familiar shape if other homelab CLIs use it; cost is a dependency
  tree and boilerplate disproportionate to a hard-capped 4-command
  binary.
- **c.** **`urfave/cli`.** Lighter than cobra with structured
  help; still a third-party dependency for dispatch the stdlib can do.
- **other:**

> **Decision: b.** Use `spf13/cobra`. The extra dependency weight is
> not a real issue and cobra is the standard across our CLI tools.

### 2. Coverage-gate scope for `pkg/hclkit`

`just coverage-gate` only measures `./internal/...`, but DESIGN-0001
puts nearly all v0 logic in `pkg/hclkit/...` — as written, the gate
would measure almost nothing.

- **a.** _Recommended._ **Extend the gate to `./internal/...
  ./pkg/...` at the same 55% floor.** One-line justfile change; the
  gate then covers where the logic actually lives. Add `examples/` to
  `.codecov.yml` ignores at the same time.
- **b.** **Keep the gate on `internal/` only.** Rely on the Codecov
  project gate (60% target / 40% threshold) for `pkg/`; weaker
  per-package guarantee, no justfile churn.
- **c.** **Move all logic into `internal/` and make `pkg/hclkit` thin
  re-export wrappers.** Keeps the gate as-is and honors CLAUDE.md's
  current "internal/ is a hard wall" framing, at the cost of a
  pass-through layer on every exported symbol.
- **other:**

> **Decision: a.** Extend `just coverage-gate` to `./internal/...
> ./pkg/...` at the same 55% floor; add `examples/` to `.codecov.yml`
> ignores.

### 3. Machine-parseable diagnostic prefix

DESIGN-0001 requires "a stable machine-parseable line prefix for CI
integration" on the default renderer but doesn't define the format.

- **a.** _Recommended._ **GCC-style `file:line:col: severity:
  summary` prefix line,** followed by the standard
  `hcl.NewDiagnosticTextWriter` body. Editors, CI log annotators, and
  plain grep all already parse this shape; it needs no environment
  detection.
- **b.** **GitHub Actions workflow commands** (`::error
  file=...,line=...,title=...`) emitted when `GITHUB_ACTIONS` is set,
  GCC-style otherwise. Nicer PR annotations, but two output modes to
  keep stable.
- **c.** **No prefix in v0.** Keep the default renderer identical to
  `hcl.NewDiagnosticTextWriter` and defer machine-parseability to the
  post-v0 `--format=json`. Simplest, but leaves the design requirement
  unmet.
- **other:**

> **Decision: a.** GCC-style `file:line:col: severity: summary` prefix
> line, followed by the standard renderer body.

### 4. Adopter-gate mechanics

Every phase's completion criteria depend on migrations in other
repos. What exactly does "consumer X ships on hclkit" mean for
checking off a phase here?

- **a.** _Recommended._ **Tagged hclkit release + the consumer's
  migration branch builds and passes its own tests against that tag;
  merge in the consumer repo not required.** Keeps hclkit's phase
  progress decoupled from consumer-repo review latency while still
  proving the API end-to-end.
- **b.** **Strict reading: the consumer PR must merge.** Highest
  confidence the migration is real and stays; hclkit phases can stall
  indefinitely on unrelated consumer-repo review queues.
- **c.** **Gate phases only on in-repo criteria** (tests, examples,
  coverage) and track adopter migrations entirely in the consumer
  repos. Fastest here, but loses RFC-0001's per-phase adopter
  evidence.
- **other:**

> **Decision: a.** A phase's adopter criterion is met when the
> consumer's migration branch builds and passes its tests against a
> tagged hclkit release; merging in the consumer repo is not required.

### 5. Release tagging cadence

DESIGN-0001 fixes v1.0.0 at Phase 4 but says nothing about tags in
between, and adopters need pinnable versions.

- **a.** _Recommended._ **One minor tag per phase:** v0.1.0 (Phase 1),
  v0.2.0 (Phase 2), v0.3.0 (Phase 3), v1.0.0 (Phase 4). Adopters pin
  a phase-shaped release; breaking changes land at minor bumps,
  matching the pre-1.0 discipline.
- **b.** **Tag only v0.1.0 and v1.0.0.** Mid-phase adopters pin
  commits via Go pseudo-versions; fewer release-pipeline runs, uglier
  go.mod lines and no SBOM/signed artifacts for the middle phases.
- **c.** **Tag ad hoc whenever a consumer needs a pin.** Maximum
  flexibility, no legible mapping between tags and phases.
- **other:**

> **Decision: other.** Every merged PR gets a tag — if this IMPL lands
> in a single PR, that's a single tag. The tag that closes Phase 4 is
> v1.0.0 per DESIGN-0001.

### 6. Fixture sourcing for the partial-decode gate

The Phase 4 v1.0 gate (and Phase 3 benchmarks) needs real `forge`
blueprints and `repo-guardian` policy files. Where do those fixtures
live?

- **a.** _Recommended._ **Vendor sanitized snapshots into
  `internal/testutil/fixtures/`,** refreshed manually when a consumer
  changes shape (with a note recording the source repo + commit). CI
  stays hermetic; no cross-repo coupling or network dependency in
  tests.
- **b.** **Fetch fixtures from the consumer repos at test time**
  (submodule or `go:generate` sync). Always current, but couples
  hclkit CI to other repos' availability, layout, and auth.
- **c.** **No cross-repo fixtures in hclkit.** Rely on consumer repos
  running their own suites against hclkit tags as the gate. Zero
  duplication, but the v1.0 gate then lives outside this repo's CI
  entirely.
- **other:**

> **Decision: a.** Vendor sanitized snapshots into
> `internal/testutil/fixtures/`, recording the source repo + commit
> with each refresh.

### 7. `hcldec.Spec` loader entry point (decide before Phase 3)

The Phase 1 architecture review (go-architect, 2026-07-03) found that
DESIGN-0001's "pass `*hcldec.Spec` as the `target`" dispatch has no
return path for the decoded value: `Load*` returns only
`Diagnostics`, and a spec is a schema, not a decode destination —
`gohcl` targets receive the result in place; a spec cannot. Phase 1
therefore shipped `decode` with a single `gohcl` arm and an explicit
invalid-target diagnostic for everything else.

- **a.** _Recommended._ **Dedicated method mirroring
  `partial.DecodeSpec`:** `LoadSpec(path string, spec hcldec.Spec)
  (cty.Value, partial.ExprMap, Diagnostics)`. Keeps `target` meaning
  "pointer that receives the decode" everywhere, and the return shape
  matches the lower-level surface it wraps.
- **b.** **Option + value destination:** keep the `Load*` entry
  points and add `WithSpec(spec)`, with the caller passing a
  `*cty.Value` target that receives the decoded value in place. One
  entry point, but `ExprMap` still needs a home.
- **c.** **Design as written:** type-switch on `target` accepting
  `*hcldec.Spec` directly, inventing a result-carrying field on
  `Diagnostics` or a separate results accessor. Preserves the
  design text at the cost of a muddier calling convention.
- **other:**

> **Decision: a.** Dedicated `LoadSpec(path, spec) (cty.Value,
> partial.ExprMap, Diagnostics)` mirroring `partial.DecodeSpec`.
> Keeps `target` meaning "pointer that receives the decode"
> everywhere, gives `ExprMap` an explicit home, and avoids growing
> `Diagnostics` into a result carrier (its error embed is
> load-bearing for consumer errcheck coverage).

## References

- [DESIGN-0001 — hclkit v0 library and validator binary](../design/0001-hclkit-v0-library-and-validator-binary.md)
- [RFC-0001 — Build hclkit as a mechanism-only HCL library](../rfc/0001-build-hclkit-as-a-mechanism-only-hcl-library.md)
- [INV-0001 — HCL Config Reuse: Shared Library vs Full DSL](../investigation/0001-hcl-config-reuse-shared-library-vs-full-dsl.md)
- `hashicorp/hcl/v2` — `gohcl`, `hclsimple`, `hclsyntax`, `hcldec`,
  `hclwrite`.
- `zclconf/go-cty` — `cty.Value`, `cty.Type`, `function.Function`.
