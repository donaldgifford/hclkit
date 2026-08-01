---
id: RFC-0001
title: "Build hclkit as a mechanism-only HCL library"
status: Draft
author: Donald Gifford
created: 2026-06-01
---
<!-- markdownlint-disable-file MD025 MD041 -->

# RFC 0001: Build hclkit as a mechanism-only HCL library

**Status:** Draft
**Author:** Donald Gifford
**Date:** 2026-06-01

<!--toc:start-->
- [Summary](#summary)
- [Problem Statement](#problem-statement)
- [Proposed Solution](#proposed-solution)
- [Design](#design)
- [Alternatives Considered](#alternatives-considered)
- [Implementation Phases](#implementation-phases)
  - [Phase 1: Library skeleton and first consumer](#phase-1-library-skeleton-and-first-consumer)
  - [Phase 2: Low-friction adopters](#phase-2-low-friction-adopters)
  - [Phase 3: EvalContext, vars-file, refined types, partial-decode](#phase-3-evalcontext-vars-file-refined-types-partial-decode)
  - [Phase 4: Cross-block refs, uniqueness, and the validator binary](#phase-4-cross-block-refs-uniqueness-and-the-validator-binary)
- [Risks and Mitigations](#risks-and-mitigations)
- [Success Criteria](#success-criteria)
- [DSL Re-Trigger](#dsl-re-trigger)
- [References](#references)
<!--toc:end-->

## Summary

Build `hclkit` as a **mechanism-only shared HCL library** that consolidates
the loader, diagnostics, EvalContext assembly, vars-file decode path,
refined `cty` primitives, cross-block reference validation, and
uniqueness constraints that are duplicated across 9+ homelab repos.
Defer the question of a shared DSL (operator vocabulary, rule grammar)
behind a **defined re-trigger condition** rather than building it now,
because today only one repo has a `subject op expected` grammar.

## Problem Statement

INV-0001 surveyed every homelab Go repo that decodes HCL (`forge`,
`fwsync`, `repo-guardian`, `claudelint`, `mcp-go-gen`, `spt`,
`webhookd`) and one planned consumer (`docz`
gaining HCL support, hosting the PST doc-model as a feature). The
findings:

- **Four repos wrap `hclsimple`/`gohcl` with near-identical
  boilerplate** for load → decode → diagnostic formatting. Every new
  repo re-implements the same ~70–300 LOC.
- **Three repos build ad-hoc EvalContexts** with three different
  shapes (`spt` functions only, `repo-guardian` variables + locals,
  `forge` functions + variables + locals).
- **Three repos handle durations as `string` + `time.ParseDuration`
  downstream** with no decode-time validation
  (`repo-guardian`, `spt`, planned `webhookd`).
- **Three repos validate severity / kind enums ad hoc post-decode**
  (`fwsync`, `claudelint`, `repo-guardian`).
- **The `env()` lookup is implemented two incompatible ways:** as an
  HCL function in `spt`, as a post-decode allowlist in `repo-guardian`.
  This semantic divergence will calcify if we don't fix it now.
- **Cross-block references are resolved in Go post-decode** in
  `fwsync` without HCL-position diagnostics.
- **Vars-file pattern (Terraform-style `.tfvars`)** exists in `forge`
  today, is planned for `fwsync`, and is the likely next addition to
  `repo-guardian` — a 3-consumer shape in formation.

The cost today is concrete: drift between consumers' diagnostic
formats, behavioral inconsistencies (the `env()` example), and
re-implementation overhead on every new HCL-using repo (`webhookd`
IMPL-0004 and `docz` HCL support are both queued).

## Proposed Solution

Ship `hclkit` v0 as a mechanism-only Go library plus a small validator
binary, scoped to primitives that are evidence-backed by ≥2 surveyed
consumers (present or planned). **No operators, no rule grammar, no
centralized schemas.** Each consumer keeps its own `Config` struct +
`hcl:"..."` tags; the library provides the shared machinery underneath.

Defer the DSL layer (operator vocabulary, `subject op expected` grammar)
until the **DSL Re-Trigger** condition (below) fires.

## Design

This RFC ratifies the architectural decision; **detailed package
layout, public API surface, and the validator binary spec live in
DESIGN-0001**.

At the architectural level, `hclkit` v0 ships nine primitives:

1. **Loader + diagnostic wrapper around `gohcl`/`hclsimple`** — one
   entry point, consistent diagnostic format, position-preserving
   errors. Subsumes the near-duplicate boilerplate in fwsync,
   claudelint, mcp-go-gen, spt.
2. **EvalContext assembly** — declarative registration of functions and
   variables, with `locals` support. Subsumes the three different
   shapes in spt / repo-guardian / forge.
3. **Standard function bundle** — `env()` (HCL function, evaluated at
   decode time; resolves the spt/repo-guardian divergence), plus
   forge's case helpers (`snakeCase`, `camelCase`, `pascalCase`,
   `kebabCase`) and `now()`.
4. **Vars-file decode path** — first-class support for a separate,
   typed user-input file (Terraform-style `.tfvars` shape) with
   `variable` declarations carrying `default`/`validate`/`choices`.
   Targets forge today + fwsync planned + repo-guardian likely.
5. **Refined `cty` Duration type** — decode-time validation, replacing
   the `string` + `time.ParseDuration`-downstream pattern.
6. **Generic enum-refinement machinery** — closed-set validation with
   HCL-position diagnostics, replacing ad-hoc post-decode checks.
7. **Cross-block reference validation** — declared verbs with
   target-kind constraints, resolved at decode time with
   position-aware diagnostics. Targets the docz meta-model directly
   and helps fwsync retrofit Go-side string lookups.
8. **Generic uniqueness validator** — per-type uniqueness on a named
   attribute (e.g. `id_prefix`, label, rule ID).
9. **`hcldec` target support + `hclsyntax` partial-decode helpers**
   — `Loader.LoadFile` accepts an `hcldec.Spec` as a decode target;
   a `pkg/hclkit/partial` subpackage exposes `DecodeSpec` (decode +
   retain `hcl.Expression` for late-bound attributes) and `Walk`
   (iterate body blocks in a chosen order, e.g. `locals` first).
   Unblocks `forge` (templates + `condition.when`) and
   `repo-guardian` (manual `decodeBody` + `locals`) for full
   migration.

Architectural constraints:

- **Mechanism only.** Operators stay pure over `cty.Value`; subject
  resolution and effects stay app-side. Nothing centralizes whole
  `Config` schemas. Nothing generalizes operators from a single app.
- **Composition over registration.** Each consumer assembles a
  `Loader` from the primitives it needs. No globals, no plugin
  registry, no required initialization order.
- **One module, `internal/` for the implementation, public API in
  `pkg/hclkit`.** Promote to subpackages only when a public consumer
  needs a narrower import. This matches the project's per-repo CLAUDE.md
  "internal/ is a hard wall" rule.
- **A validator binary (`hclkit lint|fmt|validate`)** ships alongside
  the library to give CI/CD and developers a consistent UX without
  forcing every consumer to expose its own debug subcommands. Scope and
  subcommand surface in DESIGN-0001.

## Alternatives Considered

- **Stay per-app.** Rejected. The survey shows real, recurring cost:
  duplicated loaders, divergent `env()` semantics, fragmented duration
  handling, position-blind cross-block ref errors. Five-plus
  near-identical loaders today, three more queued.
- **Library + DSL upfront.** Rejected. Only `repo-guardian` has a
  `subject op expected` grammar today. `fwsync` was thought to share
  it but doesn't (it's a structured-content store). Designing the
  operator vocabulary from one consumer is the explicit antipattern
  (X3 in INV-0001) and locks us into the wrong operator set.
- **Library + DSL deferred without a trigger.** Rejected as
  half-measure. "Defer forever" loses signal. A defined re-trigger
  keeps the decision honest and lets future consumers know exactly
  what evidence would re-open it.
- **Just vendor `hashicorp/hcl/v2` and wrap nothing.** Rejected.
  HashiCorp ships the toolkit; the recurring cost is in the *assembly*
  of the toolkit (loader + diagnostics + EvalContext + refined types).
  That's our value-add.

## Implementation Phases

### Phase 1: Library skeleton and first consumer

- Land package layout, `Loader` API, diagnostic format. v0 of the
  validator binary (`hclkit fmt`, `hclkit validate` parse-only mode).
- First consumer: **`claudelint`**. It's a thin `gohcl`-with-nil-ctx
  wrapper with no idiosyncrasies — lowest-friction path to validate
  the API end-to-end before anything depends on it.

### Phase 2: Low-friction adopters

- Retrofit **`mcp-go-gen`** (and any other `gohcl`-with-nil-ctx
  consumers that land) — all trivial. Confirms the loader API holds
  up across multiple consumers without churn.

### Phase 3: EvalContext, vars-file, refined types, partial-decode

- Ship EvalContext assembly, standard function bundle (`env()`, case
  helpers, `now()`), vars-file decode path, refined Duration, enum
  refinement, **and `hcldec` target + `hclsyntax` partial-decode
  helpers** (`pkg/hclkit/partial`).
- Adopters in this phase: **`spt`** (EvalContext + `env()`),
  **`forge`** (EvalContext + vars-file + `hcldec.Spec` target + late-bound
  expressions — full migration), **`fwsync`** planned variables /
  vars-file work.
- **Decision point:** retire `repo-guardian`'s post-decode
  `applyEnvOverrides` allowlist in favor of the library's `env()` HCL
  function. Prevents the divergence from becoming permanent.

### Phase 4: Cross-block refs, uniqueness, and the validator binary

- Ship cross-block reference validation, uniqueness validator, and the
  full validator binary (`hclkit lint` with schema/grammar input).
- Adopters: **`repo-guardian`** (full migration — its `hclsyntax`
  partial-decode + `locals` flows ride on `pkg/hclkit/partial`
  shipped in Phase 3), **`docz`** HCL support + meta-model.

## Risks and Mitigations

| Risk                                                                                                | Impact                                                       | Likelihood | Mitigation                                                                                                                                                                |
| --------------------------------------------------------------------------------------------------- | ------------------------------------------------------------ | ---------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Library API churns as Phase 3/4 adopters reveal needs Phase 1/2 didn't                              | Breaking changes ripple through 4+ consumers                 | Medium     | Stay pre-1.0 until Phase 4 lands. No SemVer compatibility promise until repo-guardian and docz both build against a stable surface.                                       |
| `env()` semantic change in repo-guardian breaks operator expectations                               | Behavior change for users of existing HCL configs            | Low–Medium | Land the library `env()` behind a feature flag in repo-guardian; deprecate `applyEnvOverrides` over one release cycle; ship a `hclkit migrate` helper.                    |
| Validator binary scope creep — becomes a kitchen-sink CLI duplicating consumer subcommands          | Maintenance burden, confused UX                              | Medium     | Hard-cap v0 binary subcommands at `fmt`, `validate` (parse-only), `lint` (schema-driven). Anything more goes in the consumer CLI.                                         |
| Premature DSL design pressure — a contributor proposes operators because "we'll need them anyway"   | Locks in the wrong vocabulary from one consumer's needs      | Medium     | Hold the DSL re-trigger condition (below) in the README and in DESIGN-0001 as a gate. PRs that add operators get rejected with a pointer.                                 |
| `fwsync` rule body turns out to be HCL transport for existing semgrep YAML, not a grammar consumer  | One less data point for the DSL trigger                      | Medium     | Resolve the open question early (see INV-0001 Observation notes) before Phase 3 starts. Document the answer in this RFC's References.                                    |
| Library's refined Duration type doesn't compose with `gohcl` struct-tag decode cleanly              | Adopters can't use it without `hcldec`                       | Medium     | Validate the gohcl integration in Phase 3 with `spt` before committing the type to v0. Fall back to a validating helper if needed. Less acute now that `hcldec` target support is in v0 — refined types compose cleanly via `hcldec` regardless. |
| `pkg/hclkit/partial` is the largest in-tree API surface and lands in Phase 3                        | API churn risk concentrates in Phase 3                       | Medium     | Test `partial` against real `forge` and `repo-guardian` fixtures end-to-end before tagging v1.0 (Phase 4 gate). Keep `DecodeSpec` / `Walk` as the low-level surfaces; Loader's `hcldec.Spec` target is the ergonomic path.                       |
| Adoption stalls — repos prefer their existing in-tree loader because it works                       | Library exists but nobody uses it                            | Low–Medium | Phase 1 ships with `claudelint` as the first integrated consumer; Phase 2 batches further trivial migrations to demonstrate ergonomics.                           |

## Success Criteria

- **Phase 1 complete** when `claudelint` ships using hclkit
  for its loader and diagnostics.
- **Phase 2 complete** when `mcp-go-gen` (and any remaining nil-ctx
  consumers) build against hclkit with no in-tree HCL loader code
  remaining.
- **Phase 3 complete** when `spt`, `forge`, and at least the `fwsync`
  variables/vars-file work-in-progress build against hclkit, and
  `repo-guardian`'s `env()` semantics match the library's.
- **Phase 4 complete** when `repo-guardian` and `docz` both build
  against hclkit, and the validator binary (`hclkit lint`) runs in CI
  for at least two consumers.
- **Per-consumer evidence of payoff:** the consumer's in-tree HCL
  plumbing LOC drops by at least 50% post-migration; the consumer's
  diagnostic format matches hclkit's default (no in-tree override).
- **DSL trigger evaluated** at the end of Phase 4: re-run the C
  section of INV-0001 against the then-current state of `claudelint`,
  `fwsync`, and `repo-guardian` rule grammars.

## DSL Re-Trigger

A separate RFC (or a follow-on revision of this one) is warranted
when **both** conditions hold:

1. **Two or more of the following have shipped or have stable
   specs:** `claudelint` config-driven custom rules, `fwsync`
   OpenSemgrep rule blocks (as inlined HCL ops, not transport for
   existing semgrep syntax), evolved `repo-guardian` rule grammar.
2. **The shipped/spec'd grammars share operator vocabulary** in a way
   that's not trivially trivially absorbable by HCL expression
   primitives (`==`, `&&`, `||`).

At that point the question is a design question (what operator set,
what type signatures, what versioning policy) rather than a discovery
question (whether sharing is warranted). This RFC explicitly does not
commit to that answer.

## References

- [INV-0001 — HCL Config Reuse: Shared Library vs Full DSL](../investigation/0001-hcl-config-reuse-shared-library-vs-full-dsl.md)
- [DESIGN-0001 — hclkit v0 library and validator binary](../design/0001-hclkit-v0-library-and-validator-binary.md)
- `hashicorp/hcl/v2` toolkit — the layer hclkit composes on top of.
- `zclconf/go-cty` — the value/type substrate for refined primitives.
- HashiCorp prior art: Terraform, Nomad, Packer, Consul all share the
  HCL toolkit but each defines its own language on top; none share
  schemas. Strong existence proof for "centralize mechanism, not
  schema."
