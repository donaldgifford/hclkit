---
id: DESIGN-0001
title: "hclkit v0 library and validator binary"
status: Draft
author: Donald Gifford
created: 2026-06-01
---
<!-- markdownlint-disable-file MD025 MD041 -->

# DESIGN 0001: hclkit v0 library and validator binary

**Status:** Draft
**Author:** Donald Gifford
**Date:** 2026-06-01

<!--toc:start-->
- [Overview](#overview)
- [Goals and Non-Goals](#goals-and-non-goals)
  - [Goals](#goals)
  - [Non-Goals](#non-goals)
- [Background](#background)
- [Detailed Design](#detailed-design)
  - [Package layout](#package-layout)
  - [Loader and diagnostics](#loader-and-diagnostics)
  - [EvalContext assembly](#evalcontext-assembly)
  - [Standard function bundle](#standard-function-bundle)
  - [Vars-file decode path](#vars-file-decode-path)
  - [Refined cty primitives](#refined-cty-primitives)
  - [Cross-block reference validation](#cross-block-reference-validation)
  - [Uniqueness validator](#uniqueness-validator)
  - [Partial-decode helpers (`pkg/hclkit/partial`)](#partial-decode-helpers-pkghclkitpartial)
  - [Validator binary (hclkit)](#validator-binary-hclkit)
- [API / Interface Changes](#api--interface-changes)
- [Data Model](#data-model)
- [Testing Strategy](#testing-strategy)
- [Migration / Rollout Plan](#migration--rollout-plan)
- [Open Questions](#open-questions)
  - [1. `fwsync` rule-body grammar](#1-fwsync-rule-body-grammar)
  - [2. `gohcl` × refined `cty` type compatibility](#2-gohcl--refined-cty-type-compatibility)
  - [3. `LoadDir` multi-file merge semantics](#3-loaddir-multi-file-merge-semantics)
  - [4. Validator binary distribution](#4-validator-binary-distribution)
  - [5. Schema format for `hclkit lint`](#5-schema-format-for-hclkit-lint)
  - [6. `hcldec` / `hclsyntax` partial-decode support](#6-hcldec--hclsyntax-partial-decode-support)
- [DSL Conversion Triggers](#dsl-conversion-triggers)
- [References](#references)
<!--toc:end-->

## Overview

`hclkit` v0 is a Go library plus a small CLI that consolidates the
recurring HCL plumbing across nine homelab repos. The library exposes
a composable `Loader`, an EvalContext builder, a standard function
bundle, a Terraform-style vars-file decode path, refined `cty`
primitives, and decode-time validators for cross-block references and
uniqueness. The CLI (`hclkit`) gives developers and CI a consistent
`fmt`/`validate`/`lint` surface without forcing every consumer to
expose its own debug subcommands.

## Goals and Non-Goals

### Goals

- One **Loader API** that subsumes the near-duplicate
  `hclsimple.Decode` / `gohcl.DecodeBody(file.Body, nil, &out)`
  boilerplate found in 5+ surveyed repos.
- One **diagnostic format** so consumers don't ship divergent error
  messages.
- **EvalContext assembly** that composes from the library's standard
  function bundle + consumer-supplied functions + variables + locals.
- A **vars-file decode path** that lifts forge's `.forge-vars.hcl`
  pattern to a first-class primitive.
- **Decode-time validation** for refined primitives (Duration, enum),
  cross-block references, and uniqueness — all with HCL-position
  diagnostics.
- A **CLI binary (`hclkit`)** with `fmt`, `validate`, and `lint`
  subcommands, scoped tightly enough not to compete with consumer CLIs.
- **`hcldec` target support + `hclsyntax` partial-decode helpers**
  in v0 — so `forge` (templates, `condition.when`, late-bound
  expressions) and `repo-guardian` (manual `decodeBody` + `locals`)
  can fully migrate without keeping in-tree partial-decode paths.
- **Pre-1.0 API stability discipline:** breaking changes are allowed
  until Phase 4 lands; after v1.0, SemVer applies.

### Non-Goals

- **Operators, rule grammars, or `subject op expected` semantics.**
  Deferred per RFC-0001's DSL re-trigger.
- **Centralized `Config` schemas.** Each consumer keeps its own struct.
- **Refined CIDR, URL, port, or regex types.** Zero current consumers.
  Ship if/when a second consumer asks; not before.
- **A plugin/registry system.** Composition by import, not by global
  registration.
- **YAML support.** docz keeps its own YAML loader; hclkit is
  HCL-only.

## Background

This design implements RFC-0001's eight first-wave primitives. The
evidence base is INV-0001's survey of 9 active repos + 1 planned
consumer. Two findings drive the v0 shape:

- The **majority cluster** (fwsync, wiz-go-gen, claudelint,
  mcp-go-gen, wiz-access-cli, webhookd-planned) uses
  `gohcl`/`hclsimple` with nil ctx. The Loader API targets this
  cluster first.
- The **EvalContext cluster** (spt, repo-guardian, forge) plus the
  planned vars-file shape (forge today, fwsync + repo-guardian
  upcoming) drives Phase 3.

## Detailed Design

### Package layout

```
hclkit/
├── cmd/hclkit/             # CLI binary entry point
│   └── main.go
├── pkg/hclkit/             # public API root
│   ├── loader.go           # Loader, LoadFile, LoadDir
│   ├── diag.go             # Diagnostic format, position helpers
│   ├── evalctx.go          # EvalContext builder
│   ├── funcs/              # standard function bundle
│   │   ├── env.go
│   │   ├── strcase.go
│   │   └── time.go
│   ├── varsfile/           # Terraform-style vars-file decode
│   │   ├── decode.go
│   │   └── variable.go
│   ├── ctytypes/           # refined cty primitives
│   │   ├── duration.go
│   │   └── enum.go
│   ├── partial/            # hcldec target + hclsyntax partial-decode helpers
│   │   ├── hcldec.go       # decode against an hcldec.Spec, retain hcl.Expression
│   │   └── walk.go         # iterate body.PartialContent for hclsyntax consumers
│   └── validate/           # cross-block refs, uniqueness
│       ├── refs.go
│       └── unique.go
├── internal/               # implementation details, not importable externally
│   ├── parser/             # hashicorp/hcl wrappers
│   └── testutil/           # internal-only test helpers (golden files, fixtures)
└── examples/               # one example per surveyed consumer pattern
    ├── nilctx/             # gohcl-with-nil-ctx (claudelint shape)
    ├── envfunc/            # gohcl + env() (spt shape)
    └── varsfile/           # vars-file decode (forge shape)
```

The `pkg/hclkit` root exposes top-level constructors (`hclkit.New`,
`hclkit.LoadFile`) that delegate to subpackages, so the common case is
a single import. Subpackage imports are available for narrower
consumers.

### Loader and diagnostics

```go
// Loader is the composable decode surface. Zero value is usable.
type Loader struct {
    // configured via functional options
}

type Option func(*Loader)

func New(opts ...Option) *Loader

// LoadFile decodes a single .hcl file into target (a struct with hcl: tags
// or an hcldec.Spec).
func (l *Loader) LoadFile(path string, target any) Diagnostics

// LoadDir merges all *.hcl files in dir (lexical order) before decoding.
// Later files override earlier; matches spt's multi-file merge.
func (l *Loader) LoadDir(dir string, target any) Diagnostics

// LoadBytes decodes raw HCL source. Filename is used in diagnostics.
func (l *Loader) LoadBytes(filename string, src []byte, target any) Diagnostics

// Options
func WithEvalContext(ctx *hcl.EvalContext) Option
func WithFunctions(fns map[string]function.Function) Option
func WithVariables(vars map[string]cty.Value) Option
func WithVarsFile(path string) Option       // forge-style .tfvars equivalent
func WithDiagnosticWriter(w io.Writer) Option
func WithValidators(v ...Validator) Option  // cross-block refs, uniqueness, etc.
```

`Diagnostics` is a thin wrapper around `hcl.Diagnostics` with a
consistent rendering API (`d.WriteTo(w io.Writer) error`,
`d.HasErrors() bool`, `d.Error() string`). Default renderer emits the
same shape as `hcl.NewDiagnosticTextWriter` with one tweak: a stable
machine-parseable line prefix for CI integration.

### EvalContext assembly

```go
// EvalCtxBuilder accumulates functions, variables, and locals into a
// single *hcl.EvalContext. The standard function bundle is included by
// default; consumers can opt out.
type EvalCtxBuilder struct { ... }

func NewEvalCtx() *EvalCtxBuilder
func (b *EvalCtxBuilder) WithStdFuncs() *EvalCtxBuilder
func (b *EvalCtxBuilder) WithFunc(name string, fn function.Function) *EvalCtxBuilder
func (b *EvalCtxBuilder) WithVar(name string, val cty.Value) *EvalCtxBuilder
func (b *EvalCtxBuilder) WithLocals(body hcl.Body) *EvalCtxBuilder  // matches repo-guardian's locals block decode
func (b *EvalCtxBuilder) Build() *hcl.EvalContext
```

Composes cleanly with `hclkit.WithEvalContext(b.Build())`. The
`WithLocals` shape mirrors `repo-guardian`'s manual `decodeLocals`
pattern so the migration is a near-1:1 swap.

### Standard function bundle

Shipped under `pkg/hclkit/funcs`, opt-in via `WithStdFuncs()`:

- **`env(name)` → string** — reads from a configurable env map
  (default `os.Environ`), returns empty string for missing keys (Unix
  shell semantics). **This is the canonical `env()`.** `spt`'s
  implementation is identical; `repo-guardian`'s `applyEnvOverrides`
  is replaced by this function during Phase 3.
- **`snakeCase`, `camelCase`, `pascalCase`, `kebabCase`** — lifted
  from `forge`'s `internal/template/funcs.go`.
- **`now(layout)` → string** — `time.Now().UTC().Format(layout)`.
  Pure for the duration of a single load; not memoized across loads.

Consumers register additional functions via `WithFunc`. The bundle is
extensible but not pluggable — no global registry.

### Vars-file decode path

Targets the forge `.forge-vars.hcl` pattern. A vars-file declares
attribute assignments:

```hcl
project_name = "demo"
features     = ["auth", "billing"]
```

Bound into the EvalContext as `var.<name>` references. Declaration
of allowed variables happens in the main config via `variable` blocks:

```hcl
variable "project_name" {
  type    = string
  default = "untitled"
}

variable "features" {
  type     = list(string)
  validate = "length(features) > 0"
}
```

API:

```go
func (l *Loader) LoadVarsFile(path string) (*VarsResult, Diagnostics)

type VarsResult struct {
    Values cty.Value             // for binding as `var` in EvalContext
    Declared map[string]Variable // for downstream prompting / validation
}

type Variable struct {
    Type     cty.Type
    Default  cty.Value
    Validate hcl.Expression       // optional; evaluated after binding
    Choices  []cty.Value          // optional
}
```

Cross-cutting with the Loader: `WithVarsFile(path)` is the one-shot
convenience; `LoadVarsFile` is the lower-level entry point for forge's
interactive-prompt flow.

### Refined `cty` primitives

Two primitives in v0:

- **`ctytypes.Duration`** — wraps a `time.Duration` as a refined
  `cty.String` type. Validates at decode time via
  `time.ParseDuration`. Decodes into `time.Duration` via a custom
  `cty.Type` refinement. Replaces the
  `string` + `time.ParseDuration`-downstream pattern in repo-guardian,
  spt, webhookd-planned.
- **`ctytypes.Enum`** — generic closed-set validator. Constructor:
  `ctytypes.Enum("severity", []string{"low", "medium", "high"})`.
  Returns a `cty.Type` that fails decode if the value isn't in the
  set, with HCL-position diagnostics.

Both are designed to compose with `gohcl` struct-tag decode via custom
type conversion hooks. The composition is **validated in Phase 3 with
`spt`** before either type is committed to v0; if the gohcl
integration is too lossy, both fall back to validating helpers
(`Validate(target)` after `Decode`) without changing the consumer API
shape.

### Cross-block reference validation

A consumer declares "this attribute is a reference to a block of kind
X" and the library resolves the reference at decode time. Mismatches
emit HCL-position diagnostics.

```go
// Validator that resolves named references against declared blocks.
//
// Example: in docz meta-model, doctype "rfc" { decides = ["policy", "guide"] }
// — the validator checks that "policy" and "guide" are both declared
// doctypes.
type RefValidator struct {
    Verb       string  // attribute name, e.g. "decides"
    TargetKind string  // expected block label, e.g. "doctype"
}

func NewRefValidator(verb, targetKind string) Validator
```

Used via `WithValidators(...)`. The validator walks the decoded body,
collects declared block labels by kind, then verifies every referenced
name resolves. Diagnostics carry the source range of the *reference*
site, not the target's missing declaration.

Helps `fwsync` and `wiz-access-cli` retrofit their Go-side string
lookups with position-aware errors.

### Uniqueness validator

```go
// Asserts that a named attribute is unique across all blocks of a kind.
//
// Example: docz meta-model — doctype.id_prefix must be unique across
// all doctype blocks.
type UniqueValidator struct {
    BlockKind string
    Attribute string
}

func NewUniqueValidator(blockKind, attribute string) Validator
```

Generic, applies anywhere per-type uniqueness matters (rule IDs in
repo-guardian, resource labels in wiz-access-cli, `id_prefix` in
docz).

### Partial-decode helpers (`pkg/hclkit/partial`)

`forge` and `repo-guardian` need decode shapes that `gohcl`'s
struct-tag pass doesn't cover:

- `forge` retains attribute expressions (`condition.when`,
  template strings) past the initial decode so they can be evaluated
  later against an EvalContext that's only assembled after variables
  are bound. It uses `hcldec.Decode` for the eager parts of the body
  and `body.PartialContent` for the lazy parts.
- `repo-guardian` walks each top-level block kind manually via
  `body.PartialContent` and `attr.Expr.Value(ctx)`, decoding
  `locals` first to populate the EvalContext before processing the
  remaining rule blocks.

`pkg/hclkit/partial` ships two surfaces for this:

```go
// DecodeSpec decodes body against an hcldec.Spec, returning the
// decoded cty.Value plus any retained hcl.Expression handles for
// late-bound attributes. Diagnostics flow through the Loader's
// standard renderer.
func DecodeSpec(body hcl.Body, spec hcldec.Spec, ctx *hcl.EvalContext) (cty.Value, ExprMap, Diagnostics)

// ExprMap is a name → hcl.Expression lookup for attributes that
// should be evaluated later (e.g. forge's `condition.when`).
type ExprMap map[string]hcl.Expression

// Walk iterates a body's blocks one kind at a time, calling fn for
// each block. Use it when a consumer needs to decode block kinds in
// a specific order (e.g. repo-guardian decodes `locals` first so its
// values populate the EvalContext before later blocks are evaluated).
func Walk(body hcl.Body, schema *hcl.BodySchema, fn WalkFunc) Diagnostics

type WalkFunc func(block *hcl.Block) Diagnostics
```

`Loader.LoadFile`/`LoadBytes`/`LoadDir` accept `*hcldec.Spec` as the
`target` argument directly — same entry point as `gohcl`-shaped
targets, dispatched via a type switch. Consumers that need the
lower-level `DecodeSpec` or `Walk` surfaces import the subpackage.

This subpackage is the **hairiest surface in v0**; the testing
strategy below calls out a partial-decode-specific test pass against
`forge` and `repo-guardian` fixtures before tagging v1.0.

### Validator binary (`hclkit`)

The binary is **deliberately small**. Its job is to give CI/CD and
developers a consistent surface for the things the library can
already do — not to compete with consumer CLIs.

```
hclkit fmt [files...]            # format HCL via hclwrite; --check for CI
hclkit validate [files...]       # parse-only validation; emits hclkit diagnostics
hclkit lint --schema=schema.hcl [files...]
                                 # schema-driven lint: requires a schema file
                                 # describing block kinds, required attributes,
                                 # refined types, references, and uniqueness
hclkit version                   # version + commit + (eventually) date
```

`hclkit lint` is the interesting subcommand. It accepts a schema
declared in HCL — a meta-model of the consumer's config shape — and
runs the library's validators against the target files. Consumers
that want CI-level enforcement without writing Go validators ship a
schema and invoke `hclkit lint` from CI.

**Out of scope for v0:** schema generation from Go structs, schema
inference, REPL/eval, language server protocol, watching/incremental
mode, plugin loading, color theming. Each of these is its own design
discussion.

**Build/release:** the binary already ships via `.goreleaser.yml`
(multi-arch archives + SBOMs + signed checksums). No changes needed
beyond expanding `main.go` past the current placeholder.

## API / Interface Changes

This is a v0 spec; there is no prior API to change. The public
surface above is the initial commitment. Backwards-incompatible
changes are permitted (per RFC-0001 risk mitigation) until Phase 4
lands and the library reaches v1.0.0.

CLI flags reserved (not implemented in v0): `--profile`, `--format`,
`--no-color`, `--schema-stdin`. Documenting reservation now to avoid
conflicting user shortcuts later.

## Data Model

Two persistent shapes:

1. **Diagnostics** — wire format for `hclkit validate`/`lint` output.
   Default is human-readable text; `--format=json` (post-v0) emits
   one JSON object per diagnostic with file, range, severity, summary,
   detail.
2. **Schema** — the input to `hclkit lint`. A v0 schema declares:
   - Block kinds (top-level labels and their nesting).
   - Required and optional attributes per kind, with `cty.Type`
     (including refined Duration / Enum).
   - Reference relationships (`verb` + `target_kind`).
   - Uniqueness constraints (`block_kind` + `attribute`).

The schema format is itself HCL. Concrete grammar lands as part of
the Phase 4 implementation; this design reserves the namespace and
top-level block kinds (`block`, `attribute`, `reference`, `unique`)
but does not lock the attribute names.

## Testing Strategy

- **Unit tests** colocated with packages (`foo_test.go` next to
  `foo.go`). Coverage gate per `internal/...` package stays at 55%
  (per `justfile` `coverage-gate`); the project-level Codecov gate
  stays at 60% target with 40% threshold.
- **Golden tests** for the diagnostic renderer and the validator
  binary. Inputs in `testdata/`, expected outputs in
  `testdata/*.golden`. `go test -update` regenerates goldens.
- **Integration tests** behind `//go:build integration` — at minimum,
  one end-to-end test per surveyed consumer pattern using fixtures
  from the `examples/` directory (`nilctx`, `envfunc`, `varsfile`).
- **Benchmarks** under `internal/...` for load + decode of
  representative consumer configs (a `forge` blueprint, a
  `repo-guardian` policy file). Wired into `just bench`.
- **Property-based** for the refined Duration / Enum types — `cty`
  round-trip preservation, decode-time error positions match source
  ranges.
- **Adopter validation:** Phase 1 success criterion is `wiz-access-cli`
  PR #7 building and passing its own tests against hclkit. Subsequent
  phases gate on the same criterion per adopter.
- **Partial-decode test pass:** before tagging v1.0, run
  `pkg/hclkit/partial` against real `forge` and `repo-guardian`
  fixtures (blueprints + policy files) end-to-end, including the
  EvalContext + late-bound expression flows. This subpackage is the
  hairiest surface in v0 and warrants its own gate.

## Migration / Rollout Plan

Mirrors RFC-0001's four phases. Per-adopter migration sequence:

1. Replace the consumer's in-tree `hclsimple.Decode` /
   `gohcl.DecodeBody` call with `hclkit.New().LoadFile`.
2. Delete the consumer's in-tree diagnostic-formatting helpers; use
   `Diagnostics.WriteTo(os.Stderr)`.
3. If the consumer registers any cty funcs or builds an EvalContext,
   replace with `NewEvalCtx().WithStdFuncs().WithFunc(...)`.
4. If the consumer has refined-primitive needs (Duration, Enum),
   swap in `ctytypes.Duration` / `ctytypes.Enum`.
5. If the consumer does Go-side cross-block lookups
   (`fwsync.resolvePolicyRefs`, `wiz-access-cli` label resolution),
   add `WithValidators(NewRefValidator(...))` and delete the manual
   lookup.
6. If the consumer uses `hcldec` or manual `hclsyntax`
   partial-decode (`forge`, `repo-guardian`), pass the `hcldec.Spec`
   to `LoadFile` directly, or use `pkg/hclkit/partial.DecodeSpec` /
   `partial.Walk` for the lower-level surfaces. Delete the in-tree
   partial-decode helpers.

Each migration is its own PR in the consumer repo. Phase ordering is
about ergonomics, not blocking dependencies — Phase 2's three
adopters are independent and can happen in parallel.

**No backwards-compatibility shims in v0.** Adopters bump their
`go.mod` to a tagged hclkit release; we don't run an `internal/hclkit`
fork in any consumer. If hclkit's API breaks pre-1.0, consumers pin
the prior version until they migrate.

## Open Questions

Each question lists `a` as the recommended option, `b`+ as alternatives,
and `other` as a free-form slot for the decision-maker to fill in.

### 1. `fwsync` rule-body grammar

Are the planned wiz / OpenSemgrep rule bodies inlined HCL operators
(real DSL-trigger consumer) or HCL transport for an existing semgrep
YAML/JSON pattern syntax (config wrapper, no trigger contribution)?
Determines whether `fwsync` counts toward the DSL re-trigger condition.

- **a.** _Recommended._ **Inlined HCL operators.** Express rule bodies
  natively in HCL so fwsync composes with hclkit's EvalContext,
  refined primitives, and (eventually) DSL operator set. Makes fwsync a
  real trigger contributor.
- **b.** **HCL transport wrapping raw semgrep YAML/JSON.** A single
  `pattern = <<EOT ... EOT` heredoc per rule, lowest upfront cost,
  but fwsync stays out of the DSL trigger calculus permanently.
- **c.** **Hybrid:** HCL block envelope (`rule "<id>" { ... }`) with
  metadata as HCL attributes and the match expression as a heredoc
  carrying raw semgrep. Ergonomic; still does not count toward the
  trigger because the operator vocab lives outside HCL.
- **other:**

> **Decision: a.** fwsync rule bodies will be inlined HCL operators —
> fwsync is in the DSL-trigger consumer set.

### 2. `gohcl` × refined `cty` type compatibility

Whether `ctytypes.Duration` and `ctytypes.Enum` decode cleanly through
`gohcl` struct-tag decode or require a separate post-decode validation
pass.

- **a.** _Recommended._ **Spike with `spt` in Phase 3, fall back if
  needed.** Try the refined-type path first; if `gohcl` integration is
  lossy or requires invasive custom hooks, fall back to validating
  helpers (`Validate(target)` called after decode) without changing
  the consumer API shape. Keeps v0 honest about what actually works.
- **b.** **Commit to refined-type compatibility upfront.** Invest the
  engineering to make `gohcl` decode through `cty` refinement even if
  it needs custom hooks. Cleaner final API, higher risk and cost.
- **c.** **Skip refined types in v0; ship only validating helpers.**
  Simpler, less ambitious; accepts the `string` + post-decode parse
  pattern as the v0 norm and revisits when a consumer hits its
  limits.
- **other:**

> **Decision: a.** Spike refined types against `gohcl` with `spt` in
> Phase 3. Validating-helper fallback remains available without
> changing the consumer API shape.

### 3. `LoadDir` multi-file merge semantics

`spt` does per-file `gohcl.DecodeBody` with later files overriding
earlier; HashiCorp's `MergeFiles` does block-level append. What does
`LoadDir` use by default?

- **a.** _Recommended._ **Per-file override default, with
  `WithMergeMode(append)` as opt-in.** Matches the only existing
  multi-file consumer (`spt`); append is available when a future
  consumer wants HashiCorp-shaped behavior.
- **b.** **Block-level append default (HashiCorp `MergeFiles`
  semantics), with `WithMergeMode(override)` as opt-in.** Aligns the
  library with upstream defaults; requires migrating `spt`.
- **c.** **No default — require explicit `WithMergeMode(...)`,
  `LoadDir` errors without it.** Forces every consumer to choose
  consciously; verbose but unambiguous.
- **other:**

> **Decision: a.** `LoadDir` defaults to per-file override (matches
> `spt`); `WithMergeMode(append)` is the opt-in for HashiCorp-shaped
> behavior.

### 4. Validator binary distribution

Library users get a `go install` path; container users get the
existing multi-arch tarballs via `goreleaser`. Do we also publish a
distroless container image for CI consumers?

- **a.** _Recommended._ **Defer until at least one CI integration
  asks for it.** Avoid YAGNI; tarballs cover the immediate need.
- **b.** **Ship distroless container in v0.** CI is a common adopter
  and waiting creates friction. Cost is one stage in `goreleaser` + a
  Dockerfile.
- **c.** **Ship a multi-stage `Dockerfile` in-repo but don't publish a
  registry image.** Consumers build their own; the repo provides a
  reference build.
- **other:**

> **Decision: a.** No container image in v0; revisit when a CI
> integration requests one. Tarballs (already shipped via
> `.goreleaser.yml`) cover the immediate path.

### 5. Schema format for `hclkit lint`

v0 reserves the top-level block kinds (`block`, `attribute`,
`reference`, `unique`) but defers the attribute-naming details.

- **a.** _Recommended._ **Defer the full schema spec to a follow-on
  DESIGN once the cross-block validator API is stable.** Ship v0
  `hclkit lint` with a minimal schema that grows with adopter needs;
  avoids designing in a vacuum.
- **b.** **Lock the full schema now via a DESIGN spike before Phase
  4.** Gives consumers a stable target sooner; risks over-designing
  before adopter feedback.
- **c.** **Adopt an existing schema language (CUE, JSON Schema)
  instead of inventing one in HCL.** Outsources the bikeshed; loses
  the "HCL all the way down" symmetry and adds a non-Go dependency.
- **other:**

> **Decision: a.** Defer the full lint-schema spec to a follow-on
> DESIGN, once the cross-block validator API has been exercised by
> Phase 3/4 adopters. v0 `hclkit lint` ships with the minimal schema.

### 6. `hcldec` / `hclsyntax` partial-decode support

Required for `forge` (templates + late-bound expressions) and
`repo-guardian` (manual `decodeBody`). Currently a v0 stretch goal.

- **a.** _Recommended._ **Stretch goal; promote to v0 only if Phase 3
  adopters can't migrate without it.** Lets `forge` and
  `repo-guardian` keep their in-tree partial-decode while still
  adopting hclkit's loader/diagnostics. Library investment follows
  demonstrated need.
- **b.** **Promote to v0 now.** `forge` and `repo-guardian` are two of
  the most important adopters and both need it; deferring blocks the
  two heaviest in-tree HCL implementations from full migration.
- **c.** **Drop entirely from v0 scope.** `forge` and `repo-guardian`
  keep their in-tree partial-decode paths permanently; they consume
  only the loader/diagnostics/EvalContext/funcs surface.
- **other:**

> **Decision: b.** Promote `hcldec` target support and `hclsyntax`
> partial-decode helpers into v0. Cascades through the design:
> a new `pkg/hclkit/partial/` subpackage joins the layout; the
> `hcldec`-as-non-goal line is removed; Phase 3 grows to include the
> partial-decode work so `forge` migrates fully in Phase 3 and
> `repo-guardian` migrates fully in Phase 4; the primitives list grows
> from 8 to 9. The cost is real — partial-decode is the hairiest
> surface in the library — but it unblocks the two heaviest in-tree
> HCL implementations and aligns v0 scope with project priority.

## DSL Conversion Triggers

This design ships v0 as mechanism-only. The library converts to a
**library + DSL layer** when **both** RFC-0001 conditions hold:

1. **Two or more of** {`claudelint` config-driven custom rules,
   `fwsync` wiz/OpenSemgrep rule blocks as inlined HCL ops, evolved
   `repo-guardian` rule grammar} have shipped or have stable specs.
2. Their operator vocabularies overlap meaningfully — i.e. not
   trivially absorbable by HCL expression primitives (`==`, `&&`,
   `||`).

When triggered, a **new DESIGN** (DESIGN-NNNN) extends the v0 surface
with a `pkg/hclkit/dsl/` subpackage containing the operator core
(pure predicates over `cty.Value`), boolean composition (`all`/`any`),
and a rule-block schema. The DSL layer sits **on top of** the v0
library — the library never depends on it, and consumers that don't
need a DSL never import it. The trigger evaluation happens at the end
of Phase 4 and re-runs annually thereafter if not triggered.

**What does not trigger DSL work:** meta-models (the docz pattern),
declarative resource models (wiz-access-cli), structured-content
stores (fwsync content side), or codegen configs (wiz-go-gen,
mcp-go-gen). These are mechanism-library use cases regardless of how
they evolve.

## References

- [RFC-0001 — Build hclkit as a mechanism-only HCL library](../rfc/0001-build-hclkit-as-a-mechanism-only-hcl-library.md)
- [INV-0001 — HCL Config Reuse: Shared Library vs Full DSL](../investigation/0001-hcl-config-reuse-shared-library-vs-full-dsl.md)
- `hashicorp/hcl/v2` — `gohcl`, `hclsimple`, `hclsyntax`, `hcldec`,
  `hclwrite`.
- `zclconf/go-cty` — `cty.Value`, `cty.Type`, `function.Function`,
  refined types.
- Per-consumer reference files cited in INV-0001's per-repo
  inventory.
