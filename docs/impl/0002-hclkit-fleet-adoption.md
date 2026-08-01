---
id: IMPL-0002
title: "hclkit fleet adoption"
status: Draft
author: Donald Gifford
created: 2026-07-06
---
<!-- markdownlint-disable-file MD025 MD041 -->

# IMPL 0002: hclkit fleet adoption

**Status:** Draft
**Author:** Donald Gifford
**Date:** 2026-07-06

<!--toc:start-->
- [Objective](#objective)
- [Scope](#scope)
  - [In Scope](#in-scope)
  - [Out of Scope](#out-of-scope)
- [Adoption Waves](#adoption-waves)
  - [Wave 1: Low-friction adopters](#wave-1-low-friction-adopters)
    - [Tasks](#tasks)
    - [Success Criteria](#success-criteria)
  - [Wave 2: EvalContext and partial-decode consumers](#wave-2-evalcontext-and-partial-decode-consumers)
    - [Tasks](#tasks-1)
    - [Success Criteria](#success-criteria-1)
  - [Wave 3: Final adopters and the v1.0 gate](#wave-3-final-adopters-and-the-v10-gate)
    - [Tasks](#tasks-2)
    - [Success Criteria](#success-criteria-2)
- [Dependencies](#dependencies)
- [References](#references)
<!--toc:end-->

## Objective

Track the seven consumer-repo migrations onto `hclkit` that validate
each phase of [IMPL-0001](0001-hclkit-v0-library-and-validator-binary.md).
This work was originally embedded in IMPL-0001's phases, but every
task here lands in a *consumer* repo (or is triggered by consumer
feedback), so it was split out: IMPL-0001 stays the in-repo
implementation driver — suitable for an autonomous loop over this
repo — and this doc is the cross-repo adoption tracker driven by
user action.

Each wave is gated on a tagged hclkit release from the corresponding
IMPL-0001 phase (per-PR tagging, IMPL-0001 OQ-5). Gate mechanics per
IMPL-0001 OQ-4: a wave's criterion is met when the consumer's
migration branch builds and passes its tests against the tag —
merging in the consumer repo is not required.

**Implements:** the adopter rollout of
[RFC-0001](../rfc/0001-build-hclkit-as-a-mechanism-only-hcl-library.md)
(detailed in [DESIGN-0001](../design/0001-hclkit-v0-library-and-validator-binary.md))

## Scope

### In Scope

- The seven consumer migrations: `claudelint`, `mcp-go-gen`, `spt`,
  `forge`, `fwsync`, `repo-guardian`, `docz`.
- Feedback loops back into hclkit: API-friction triage, pre-1.0
  breaking fixes, and new `examples/` shapes surfaced by migrations
  (the resulting hclkit changes land as normal PRs against this
  repo).
- Per-consumer LOC-delta recording (RFC-0001 targets ≥50% reduction
  of in-tree HCL plumbing per adopter).

### Out of Scope

- hclkit library/binary implementation — IMPL-0001.
- The migration PRs' own review cycles inside consumer repos; this
  doc tracks whether each gate is met, not consumer-repo process.

## Adoption Waves

Waves gate on hclkit tags, not on each other: wave 1 needs only the
Phase 1 surface; wave 2 needs the Phase 3 surface; wave 3 needs the
Phase 4 surface. Within a wave, migrations are independent.

---

### Wave 1: Low-friction adopters

**Requires:** a tagged hclkit release with the Phase 1 surface
(`Loader`, `Diagnostics`, `fmt`/`validate`/`version`).

Proves the loader API holds up across consumers without churn. Both
adopters are `gohcl`-with-nil-ctx shapes; `claudelint` goes first as
the lowest-friction end-to-end validation of the API before anything
depends on it, and the migrations are otherwise independent.

#### Tasks

- [ ] Migrate `claudelint` (first adopter): swap the in-tree loader
      for `hclkit.New().LoadFile`, delete its diagnostic helpers;
      capture any API friction as hclkit issues.
- [ ] Migrate `mcp-go-gen`: same sequence.
- [ ] Record per-consumer LOC delta for in-tree HCL plumbing (RFC-0001
      targets ≥50% reduction per adopter).
- [ ] Triage API friction found during the migrations; land any
      breaking fixes in hclkit now (pre-1.0 breaks are cheapest here,
      before the Phase 3 surface multiplies consumers).
- [ ] Extend hclkit's `nilctx` integration tests with any consumer
      shapes the migrations surfaced.

#### Success Criteria

- `claudelint` and `mcp-go-gen` both build and pass their tests
  against a tagged hclkit with no in-tree HCL loader or
  diagnostic-formatting code remaining (RFC-0001 Phase 1 + Phase 2
  criteria).
- Each migrated consumer's diagnostic output matches hclkit's default
  renderer (no in-tree overrides).
- In-tree HCL plumbing LOC is down ≥50% in each adopter.
- The `Loader` API survived the migrations with zero breaking
  changes, or every break is recorded with its migration note.

---

### Wave 2: EvalContext and partial-decode consumers

**Requires:** a tagged hclkit release with the Phase 3 surface
(`EvalCtxBuilder`, `funcs`, `varsfile`, `ctytypes`, `partial`,
`LoadSpec`).

The consumers that exercise the widest API surface. Also the
decision point that retires `repo-guardian`'s `applyEnvOverrides`
divergence.

#### Tasks

- [ ] `spt` migrates (EvalContext + `env()`; hosts the refined-types
      spike — outcome recorded in IMPL-0001).
- [ ] `forge` migrates fully (vars-file + `LoadSpec` + late-bound
      expressions via `partial`); its in-tree partial-decode helpers
      are deleted.
- [ ] `fwsync` vars-file work builds on hclkit.
- [ ] Decision point: land the library `env()` in `repo-guardian`
      behind a feature flag; deprecate `applyEnvOverrides` over one
      release cycle (RFC-0001 risk mitigation).

#### Success Criteria

- `spt`, `forge`, and the `fwsync` vars-file work-in-progress all
  build and pass tests against a tagged hclkit (RFC-0001 Phase 3
  criterion).
- `forge` has no in-tree partial-decode code left; its late-bound
  expression flow runs through `partial.DecodeSpec`/`ExprMap`.
- `repo-guardian`'s `env()` semantics match the library's (flag may
  still be in its deprecation cycle).

---

### Wave 3: Final adopters and the v1.0 gate

**Requires:** a tagged hclkit release with the Phase 4 surface
(validators, `lint --schema`).

The final two migrations plus CI wiring. Completing this wave is a
prerequisite for tagging hclkit v1.0.0 (IMPL-0001 Phase 4's last
task — RFC-0001's Phase 4 criterion requires these adopters green).

#### Tasks

- [ ] `repo-guardian` migrates fully: `locals`-first flow on
      `partial.Walk` + `EvalCtxBuilder.WithLocals`; manual
      `decodeBody` deleted; `applyEnvOverrides` removed at the end of
      its deprecation cycle.
- [ ] `docz` gains HCL support on hclkit, using `RefValidator` /
      `UniqueValidator` for its meta-model (`decides` refs,
      `id_prefix` uniqueness).
- [ ] Wire `hclkit lint` (or `validate`) into CI for at least two
      consumers.

#### Success Criteria

- `repo-guardian` and `docz` both build and pass tests against a
  tagged hclkit (RFC-0001 Phase 4 criterion).
- `hclkit lint` runs in CI for at least two consumers.
- hclkit v1.0.0 is unblocked (tagging itself is IMPL-0001 Phase 4).

---

## Dependencies

- Tagged hclkit releases per IMPL-0001 phase (per-PR tagging,
  IMPL-0001 OQ-5).
- Consumer-repo availability: `fwsync`'s vars-file work is planned,
  not started. Waves can stall on consumer schedules without
  blocking IMPL-0001's in-repo phases.
- Gate mechanics per IMPL-0001 OQ-4 (migration branch builds against
  the tag; consumer-repo merge not required).

## References

- [IMPL-0001 — hclkit v0 library and validator binary](0001-hclkit-v0-library-and-validator-binary.md)
- [DESIGN-0001 — hclkit v0 library and validator binary](../design/0001-hclkit-v0-library-and-validator-binary.md)
- [RFC-0001 — Build hclkit as a mechanism-only HCL library](../rfc/0001-build-hclkit-as-a-mechanism-only-hcl-library.md)
