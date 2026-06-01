---
id: INV-0001
title: "HCL Config Reuse: Shared Library vs Full DSL"
status: Concluded
author: Donald Gifford
created: 2026-05-31
---

<!-- markdownlint-disable-file MD025 MD041 -->

# INV 0001: HCL Config Reuse: Shared Library vs Full DSL

**Status:** Concluded **Author:** Donald Gifford **Date:** 2026-06-01

<!--toc:start-->
- [Question](#question)
- [Hypothesis](#hypothesis)
- [Context](#context)
  - [Conceptual model (read before filling the rubric)](#conceptual-model-read-before-filling-the-rubric)
- [Approach](#approach)
  - [A. Do we need to centralize at all? (per-app vs anything shared)](#a-do-we-need-to-centralize-at-all-per-app-vs-anything-shared)
  - [B. Library scope — mechanism only (the shared toolkit)](#b-library-scope--mechanism-only-the-shared-toolkit)
  - [C. DSL signals — shared semantics / vocabulary](#c-dsl-signals--shared-semantics--vocabulary)
  - [X. Antipattern checks (apply regardless of outcome)](#x-antipattern-checks-apply-regardless-of-outcome)
  - [Decision matrix](#decision-matrix)
- [Environment](#environment)
  - [Repos in Scope](#repos-in-scope)
- [Findings](#findings)
  - [Per-repo inventory](#per-repo-inventory)
  - [Cross-repo overlap synthesis](#cross-repo-overlap-synthesis)
  - [Observation notes](#observation-notes)
- [Conclusion](#conclusion)
- [Recommendation](#recommendation)
  - [Build hclkit as a mechanism-only shared library now](#build-hclkit-as-a-mechanism-only-shared-library-now)
  - [Sequence the adopters](#sequence-the-adopters)
  - [Defer the DSL layer with a defined re-trigger](#defer-the-dsl-layer-with-a-defined-re-trigger)
  - [Housekeeping discovered in passing](#housekeeping-discovered-in-passing)
- [References](#references)
<!--toc:end-->

## Question

Across the repos that use HCL for configuration, should we extract a **shared
library** (centralized _mechanism_ — parser, eval context, functions,
diagnostics, primitive types), a **full DSL** (a shared _vocabulary_ of blocks,
operators, and semantics), **both**, or **neither** (keep it per-app)?

The decision is not binary. This investigation surveys the repos against a fixed
rubric and reads the answer off the evidence rather than off intuition.

## Hypothesis

Current lean, to be confirmed or refuted by the survey:

- A **mechanism-only shared library is already justified** — parser setup, eval
  context, custom `cty` functions, file merge/include semantics, and diagnostic
  rendering are duplicated across repos, and we want consistent config UX.
- A **full DSL is justified only for the subset of repos that share the
  rule/operator grammar** (the `repo-guardian` ↔ `fwsync` pattern: identical
  assertion structure, different implementation underneath).
- Correct sequence: **build the library first; extract the rule-grammar DSL as a
  second layer on top of it once ≥2 repos confirm a genuinely shared
  vocabulary.**
- We will _not_ centralize whole config schemas (the god-package antipattern).

## Context

We use HCL as config across many projects. The recurring question is whether to
centralize, and if so, whether that centralized thing becomes a DSL or stays a
library. This doc exists to make that call on evidence after gathering every
repo that decodes HCL.

**Triggered by:**
<!-- RFC-XXXX / DESIGN-XXXX / issue #XXX — link the parent if any -->

### Conceptual model (read before filling the rubric)

**Three layers, with opposite reuse profiles:**

| Layer                      | What it is                                                                                     | Reuse verdict                                                                                                          |
| -------------------------- | ---------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------- |
| **Plumbing**               | Parser setup, eval context, custom `cty` functions, file discovery/merge, diagnostic rendering | **Centralize** — duplicated boilerplate, low churn, clear interface. This is a _library_.                              |
| **Schema**                 | Each app's `Config` struct + `hcl:"..."` tags                                                  | **Keep per-app** — intrinsically app-specific. Centralizing whole schemas = god package (coupling + churn, no payoff). |
| **Semantics / vocabulary** | Shared block types _with meaning_, operators, cross-block refs, composition                    | **Centralize only if shared across ≥2 apps.** This — and only this — is the _DSL_.                                     |

**Library and DSL are orthogonal axes, not a spectrum:**

- Centralizing mechanism = a shared toolkit. It is **not** a DSL.
- You cross into DSL territory only when you centralize **semantics** (operators
  with behavior, a vocabulary of blocks, evaluation rules).
- A DSL always sits **on top of** the library. You never build a DSL without the
  library, and you never centralize schemas.

**The rule-grammar seam (where the DSL, if any, lives):**

A rule is `subject <operator> expected`.

- **Shared (DSL core):** operator vocabulary (`equals`, `matches`, `in`,
  `exists`, comparisons), boolean composition (`and`/`or`/`not`, `all`/`any`),
  primitive/domain types, diagnostics. Operators are **pure predicates over
  `cty.Value`** — they never learn what kind of subject they act on.
- **App-specific ("the underneath"):** how a subject _resolves_ to a value (via
  the app-provided `EvalContext` functions/variables) and what _effect_ a
  pass/fail triggers.

"Same structure, different underneath" is the tell that you've found an
interface, and that the shared structure is worth a DSL.

## Approach

Survey every in-scope repo (see Environment), fill the inventory tables under
Findings, then answer the rubric below. Each block ends with a verdict rule.

### A. Do we need to centralize at all? (per-app vs anything shared)

- **A1.** Is the same HCL plumbing (parser setup, eval-context construction,
  diagnostic rendering, file discovery/merge) implemented in more than one repo?
- **A2.** Do ≥2 repos define the same or near-identical custom `cty` functions?
- **A3.** Do ≥2 repos share validated/domain primitive types (CIDR, Duration,
  Regex, constrained enums)?
- **A4.** Is config-file UX inconsistent across apps in ways that bother
  authors/operators (different error formats, function sets, include/merge
  rules)?
- **A5.** Is the per-app duplication actually costing time (drift, bugs,
  onboarding friction)?

> **Verdict:** Mostly **No** → stay per-app; stop here. Several **Yes** → a
> shared library is warranted; continue to B.

### B. Library scope — mechanism only (the shared toolkit)

- **B1.** Are the shared pieces purely _mechanism_ (loader, functions,
  diagnostics, merge), with each app keeping its own `Config` schema?
- **B2.** Do apps decode mostly via `gohcl` struct tags (config-format
  territory), i.e. no significant expression/eval needs?
- **B3.** For primitives: is the recurring thing the _machinery_ for declaring
  types (a generic enum validator, a `cty` refinement) versus the _specific
  values_ (identical enum members)? Share machinery freely; share specific
  values only where genuinely identical across apps.

> **Verdict:** Yes across B → a mechanism-only shared library is the target.
> Stop here unless C trips.

### C. DSL signals — shared semantics / vocabulary

- **C1.** Do ≥2 repos contain a structurally identical abstraction (a
  "rule"/"assertion") that differs only in what it resolves against and what it
  does — same grammar, different implementation? (e.g. `repo-guardian` ↔
  `fwsync`)
- **C2.** Is there a recurring operator vocabulary (`equals`/`matches`/`in`/
  `exists`/comparison) used across apps?
- **C3.** Do config files contain expressions, variables/`locals`, or references
  between blocks — not just static key/value pairs?
- **C4.** Would authors benefit from boolean composition (`and`/`or`/`not`,
  `all`/`any`) in config?
- **C5.** Is config authored/edited by humans (not generated), so a stable,
  documented, diagnosable language surface has real value?
- **C6.** Are we willing to pay the **DSL tax**: a versioned language surface,
  backward-compat obligations on operator names/semantics, a written spec, a
  diagnostics quality bar, and ideally editor/schema support?

> **Verdict:** C1–C5 mostly **Yes** **and** C6 **Yes** → build the
> DSL/vocabulary layer, on top of the library, scoped to the grammar-sharing
> repos. C6 **No** → not ready to own a DSL; stay library-only and re-run this
> section later.

### X. Antipattern checks (apply regardless of outcome)

- **X1.** Are we about to centralize whole `Config` schemas? → **Don't.** God
  package: coupling + churn, no payoff.
- **X2.** Is any operator/primitive being designed to know about a specific
  app's subject or domain? → **Don't.** Operators stay pure over `cty.Value`;
  subject resolution + effects stay app-side.
- **X3.** Are we generalizing the operator set from a single app, or
  anticipating apps that don't exist yet? → **Resist.** Design a minimal,
  orthogonal core and lean on composition instead of bespoke operators.

### Decision matrix

| Answer pattern                      | Outcome                                                              |
| ----------------------------------- | -------------------------------------------------------------------- |
| A mostly No                         | Stay per-app; no shared code                                         |
| A several Yes · B Yes · C mostly No | **Shared library** (mechanism only)                                  |
| A Yes · B Yes · C1–C5 Yes · C6 Yes  | **Shared library + DSL layer** (scoped to grammar-sharing repos)     |
| C signals present · C6 No           | Library now; **defer DSL**; re-run C when willing to own the surface |

## Environment

### Repos in Scope

List every repo that decodes HCL. Copy them locally or link them so the survey
is mechanical. Mark whether each is included and why.

| Repo           | Link / local path                                 | HCL config?      | Included? | Notes                                                                                                                                 |
| -------------- | ------------------------------------------------- | ---------------- | --------- | ------------------------------------------------------------------------------------------------------------------------------------- |
| repo-guardian  | <https://github.com/donaldgifford/repo-guardian>  | yes              | yes       | Only repo with a real `subject op expected` rule grammar today; partial `hclsyntax` decode + `locals`. Heaviest plumbing (~1087 LOC). **Trajectory:** likely to add a Terraform-style vars-file path next. |
| fwsync         | <https://github.com/donaldgifford/fwsync>         | yes              | yes       | Today: structured-content store for compliance docs (`gohcl`, no ctx). **Trajectory:** moves toward forge-shape — variables/vars files, HCL-wrapped wiz/OpenSemgrep rule blocks (open question: rule body inlined HCL ops vs transport for existing semgrep syntax). Wiz SDK underneath creates frameworks/automations/policies in Wiz + renders Markdown for rfc-site / rfc-api. |
| forge          | <https://github.com/donaldgifford/forge>          | yes              | yes       | Scaffolding tool. Heavy expression use (templates, `condition.when`), Terraform-style `variable` blocks, 6 custom `cty` funcs. Today's reference exemplar for the variables-and-EvalContext pattern. |
| wiz-go-gen     | <https://github.com/donaldgifford/wiz-go-gen>     | yes              | yes       | Codegen config (`wiz-sdk-gen.hcl`). `hclsimple`/`gohcl`, nil ctx. Trivial. Style template cited by docz meta-model plans.              |
| claudelint     | <https://github.com/donaldgifford/claudelint>     | yes              | yes       | Today: linter config (`gohcl` + `cty.Value` as opaque `options` bag). **Trajectory:** config-driven custom rules — a real rule-grammar consumer in the medium term. |
| mcp-go-gen     | <https://github.com/donaldgifford/mcp-go-gen>     | yes              | yes       | MCP server scaffolding config. `gohcl`, nil ctx. Trivial. Style template cited by docz meta-model plans.                              |
| spt            | <https://github.com/donaldgifford/spt>            | yes              | yes       | Runtime service config; multi-file merge. `gohcl` **with EvalContext** — registers an `env()` function.                               |
| webhookd       | <https://github.com/donaldgifford/webhookd>       | planned (RFC)    | no        | Current `internal/` has zero `hcl` imports. RFC-0001/ADR-0009 commit to `gohcl` + partial decode but IMPL-0004 has not started.       |
| wiz-access-cli | <https://github.com/donaldgifford/wiz-access-cli> | PR-only (PR #7)  | yes       | `feature/cli-mvp` (open). Declarative resource model — `project`/`team_mapping`/`custom_role`. `hclsimple`, nil ctx. Likely first hclkit consumer if hclkit lands first. |
| docz           | <https://github.com/donaldgifford/docz>           | planned          | yes       | Planned HCL support alongside legacy YAML. Will host the PST doc-model feature: a higher-order grammar where users declare custom doctypes that compile down to docz's existing doctype config. (Subsumes the prior "pst-doc-model" candidate.) |

## Findings

Fill in as the survey proceeds. The overlap _is_ the evidence.

### Per-repo inventory

| Repo           | Decode method                            | Expressions? | Vars/`locals`?            | Cross-block refs? | Custom `cty` funcs                                                                            | Primitive/domain types                                          | "Rule"/assertion concept? (shape)                                                                            | Operator vocab                                                          | Subject resolution                                              | Effects on match/fail                                            | Config author              | Plumbing LOC |
| -------------- | ---------------------------------------- | ------------ | ------------------------- | ----------------- | --------------------------------------------------------------------------------------------- | --------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------ | ----------------------------------------------------------------------- | --------------------------------------------------------------- | ---------------------------------------------------------------- | -------------------------- | ------------ |
| repo-guardian  | `hclsyntax` partial (manual `decodeBody`) | yes          | yes — `locals` block      | yes — `local.*`   | none (vars only)                                                                              | none custom; durations as `string` + `time.ParseDuration`; RE2 regex compiled at load | yes — `rule "file" { assertion { ... } }`, `rule "setting" { property/expected }`, `rule "branch_protection"` | `pattern`, `not_pattern`, `equals`, `contains`, `non_empty`; `exists`/`contains`/`exact` for file checks | implicit by rule type (filesystem path / GraphQL property / branch ruleset) | emit issue + optionally open/update remediation PR; reconcile repo state | engineer in-repo (platform) | ~1087        |
| fwsync         | `gohcl`, nil ctx                         | no           | no                        | no (Go-side only) | none                                                                                          | none (Severity/SASTEngine as plain string)                      | no — structured content store (controls + `wiz_policy` refs by string ID)                                    | n/a                                                                     | n/a                                                             | n/a (renders to Markdown/Wiz/SAST downstream)                    | engineer in-repo (security) | ~551         |
| forge          | mixed: `hcldec` + manual `PartialContent` + `hclsyntax.ParseTemplate` | yes (heavy) | yes — `variable` blocks + `.forge-vars.hcl` | yes — `var.*` interpolation | `snakeCase`, `camelCase`, `pascalCase`, `kebabCase`, `now`, `env` + 7 stdlib (`upper`, `lower`, `title`, `replace`, `trimPrefix`, `trimSuffix`, `coalesce`) | none refined (variable types: string/bool/number → `cty.Type`)  | no — `condition { when, exclude }` gates file inclusion (boolean expr, not `subj op exp`)                    | full HCL expression language                                            | variable interpolation via EvalContext                          | file-system mutation (write/rename/exclude)                      | engineer (blueprint) / end user (vars) | ~1886        |
| wiz-go-gen     | `hclsimple` (`gohcl`), nil ctx           | no           | no                        | no                | none                                                                                          | none                                                            | no                                                                                                           | n/a                                                                     | n/a                                                             | drives Go code generation                                        | engineer (SDK author)       | ~44          |
| claudelint     | `gohcl`, nil ctx (+ `cty.Value` for `options`) | no       | no                        | no                | none registered (cty used as type carrier, not eval)                                          | none (severity = plain string, validated semantically)          | no — `rule "<id>" {...}` only overrides settings; lint rules live in Go                                      | n/a                                                                     | rule IDs resolved to Go-registered rules                        | enable/disable rule, change severity, narrow path globs          | end user (linter consumer)  | ~269         |
| mcp-go-gen     | `gohcl`, nil ctx                         | no           | no                        | no                | none                                                                                          | none                                                            | no                                                                                                           | n/a                                                                     | n/a                                                             | codegen output (MCP server source + manifests)                   | engineer (server author)    | ~79          |
| spt            | `gohcl` **with EvalContext**             | yes          | no                        | no                | `env(name) → string`                                                                          | none (durations as `string`, parsed downstream)                 | no                                                                                                           | n/a                                                                     | n/a                                                             | process runtime config                                           | engineer / operator         | ~293         |
| webhookd       | planned: `gohcl` + `,remain` partial decode | no (planned) | no (planned)            | no (planned)      | none (planned)                                                                                | none (planned; durations as string)                             | no                                                                                                           | n/a                                                                     | n/a                                                             | provider × backend wiring at boot (planned)                      | engineer / operator (planned) | n/a (not impl) |
| wiz-access-cli | `hclsimple` (`gohcl`), nil ctx           | no           | no                        | no in-HCL (Go-side string refs) | none                                                                                | none (all scalars plain string)                                 | no — declarative resource model (`project`, `team_mapping`, `custom_role`)                                   | n/a                                                                     | n/a                                                             | create/update/delete Wiz API resources; ID writeback to HCL      | end user / security operator | ~73 (parser) + ~149 (writer) + ~92 (generate) |
| docz (planned) | planned: `gohcl` + custom validation in Go decoder; legacy YAML path retained during transition | no (planned) | no (planned)        | **yes (planned, decode-time)** — meta-model relationship verbs whose target must resolve to a declared block of a stated kind | none planned                                                                  | **closed-set enum refinement** + **per-type uniqueness constraints** (e.g. unique `id_prefix`) | no — meta-model (declares doctypes; instances live in docz itself)                                           | n/a                                                                     | n/a                                                             | compiles to docz's existing doctype config + generates API registry, site nav, glossary, diagram, lint rules | engineer (platform docs maintainer) | n/a (not impl) |

### Cross-repo overlap synthesis

What actually recurs across repos (this drives the verdict):

| Candidate shared element                            | Repos that use it                                                                              | Same machinery?                                            | Same values/semantics?                                                | Centralize? (library / DSL / no) |
| --------------------------------------------------- | ---------------------------------------------------------------------------------------------- | ---------------------------------------------------------- | --------------------------------------------------------------------- | -------------------------------- |
| Parser + diagnostics plumbing (load file → decode → format diags) | fwsync, wiz-go-gen, claudelint, mcp-go-gen, spt, wiz-access-cli (+ webhookd planned, docz planned) | nearly — 5+ repos wrap `hclsimple`/`gohcl` near-identically | n/a                                                                   | **library** (high payoff)        |
| EvalContext construction (functions + variables)    | spt (funcs), repo-guardian (vars + locals), forge (funcs + vars + locals) (+ fwsync planned, repo-guardian future vars files) | no — multiple different shapes today                       | n/a                                                                   | **library** (factor out the assembly) |
| Vars-file decode path (Terraform-style `.tfvars` shape, separate user-input file) | forge (`.forge-vars.hcl`) today; fwsync planned; repo-guardian likely next               | no — single consumer today, but a 3-consumer shape is forming | similar intent (typed user input bound into EvalContext at load time) | **library** (lift forge's pattern to a first-class primitive) |
| Custom func `env(name)`                             | spt, forge                                                                                     | similar (both read `os.Getenv`-style with empty fallback)  | same Unix-shell-default semantics                                     | **library**                      |
| String-case funcs (`snakeCase`/`camelCase`/...)     | forge only                                                                                     | n/a                                                        | n/a                                                                   | library (ship in stdlib bundle, no current second consumer) |
| `cty.Value` → `map[string]any` (opaque options bag) | claudelint                                                                                     | n/a (single consumer)                                      | n/a                                                                   | library (utility worth sharing if another linter/plugin pattern appears) |
| Primitive: CIDR                                     | none                                                                                           | n/a                                                        | n/a                                                                   | no                               |
| Primitive: Duration                                 | repo-guardian, spt, webhookd (planned) — all as `string` + `time.ParseDuration` downstream     | no — none validate at decode time                          | semantically same, parsed at different layers                         | **library** (refined `cty` type w/ load-time validation) |
| Primitive: Regex                                    | repo-guardian only (RE2 via Go `regexp`)                                                        | n/a                                                        | n/a                                                                   | library (helper, low cost)       |
| Primitive: validated enum machinery                 | fwsync (Severity), claudelint (severity), repo-guardian (rule kinds), pst-doc-model planned (`class`, `permanence`) | no — each validates ad hoc post-decode (pst-doc-model proposes decode-time refinement) | similar intent                                                        | **library** (generic enum refinement) |
| Cross-block reference validation (target must resolve to declared block of a stated kind) | fwsync (string-keyed Go-side post-decode), wiz-access-cli (string-keyed Go-side post-decode), docz meta-model planned (decode-time, with HCL position diagnostics) | no — current consumers resolve in Go without HCL-position diagnostics | similar intent (declared-name lookup) | **library** (declared verbs + target-kind constraint + diag surface) |
| Uniqueness constraints (e.g. unique `id_prefix`, unique rule IDs, unique labels) | docz meta-model planned; repo-guardian (rule IDs by convention), wiz-access-cli (resource labels by convention) | no — currently convention-only or ad hoc                   | similar intent                                                        | **library** (generic uniqueness validator) |
| Operator: `equals`                                  | repo-guardian today; claudelint custom rules planned; fwsync wiz/OpenSemgrep rules planned (TBC) | n/a today                                                  | literal equality                                                      | **defer DSL** today; reopen when ≥2 ship with aligned vocab |
| Operator: `matches` / `pattern`                     | repo-guardian today (RE2); claudelint planned; fwsync planned (TBC)                            | n/a today                                                  | RE2 today; semgrep flavour TBC if fwsync inlines pattern syntax       | defer DSL today; reopen later     |
| Operator: `contains`                                | repo-guardian today; claudelint planned; fwsync planned (TBC)                                  | n/a today                                                  | substring today                                                       | defer DSL today; reopen later     |
| Operator: `in`                                      | none today; plausible in claudelint custom rules                                               | n/a                                                        | n/a                                                                   | defer DSL                        |
| Operator: `exists`                                  | repo-guardian today (`exists`/`contains`/`exact` file check modes)                             | n/a                                                        | filesystem presence today                                             | defer DSL today; reopen later     |
| Boolean composition (`and`/`or`/`not`, `all`/`any`) | none today (only HCL expression `&&`/`||` in forge)                                            | n/a                                                        | n/a                                                                   | defer DSL                        |
| Rule block grammar (`subject op expected`)          | repo-guardian today; claudelint planned (config-driven custom rules); fwsync planned (wiz/OpenSemgrep, TBC) | n/a today                                                  | n/a today                                                             | **defer DSL today; trigger condition: ≥2 of {claudelint custom rules, fwsync wiz rules, evolved repo-guardian} ship with aligned operator vocabulary** |

### Observation notes

- **Hypothesis partially refuted.** The pre-survey lean was that `repo-guardian`
  and `fwsync` shared a rule grammar. They don't — `fwsync` is a
  structured-content store for compliance docs, not a rule engine. **Only
  `repo-guardian` has a `subject op expected` grammar today.** A "shared
  rule-grammar DSL" would currently have exactly one consumer. That kills the
  C1 signal until a second grammar-sharing repo materializes.
- **Decode-method split sets the library shape.** Five repos use plain
  `gohcl`/`hclsimple` with nil ctx (fwsync, wiz-go-gen, claudelint, mcp-go-gen,
  wiz-access-cli) + planned webhookd. Two register an EvalContext (spt funcs,
  repo-guardian vars+locals). One uses `hcldec` + manual partial decode (forge).
  A "make `gohcl` ergonomic" library helps the majority; `forge` and
  `repo-guardian` need richer building blocks (EvalContext assembly, expression
  retention).
- **`env()` semantic divergence is a real bug surface.** `spt` exposes `env`
  as an HCL function evaluated at decode time. `repo-guardian` applies env
  overrides *after* HCL load via a fixed allowlist (`applyEnvOverrides`,
  loader.go:993). Same user need, two incompatible mechanisms. A shared
  library should pick one — probably the `spt` model (HCL function), since
  it composes with the rest of the expression language.
- **Duration handling is fragmented across three repos.** repo-guardian
  (`schedule_interval`), spt (`ReadTimeout`/`TickInterval`/etc.), and
  webhookd (planned `idempotency_ttl`/`skew`/`shutdown_timeout`) all keep
  durations as `string` and call `time.ParseDuration` downstream. None
  validates at decode time. A refined `cty` Duration type is the
  highest-leverage primitive in this survey.
- **No CIDR / URL / port / regex types anywhere.** Refined `cty` primitives
  have zero current consumers. Ship machinery (generic refinement helpers),
  not specific values.
- **Regex flavor uniform (Go RE2).** Only `repo-guardian` compiles regexes
  from HCL config today, and it uses Go's `regexp`. No PCRE/RE2 split exists
  — no compat trap waiting.
- **`webhookd` is RFC-only.** Current `internal/` has no HCL imports; ADR-0009
  commits to `gohcl` + partial decode but IMPL-0004 has not started. Building
  the hclkit consumer story around webhookd is premature.
- **`wiz-access-cli` HCL lives in PR #7 (open, `feature/cli-mvp`).** Mainline
  has no HCL. If hclkit lands first, that PR is a candidate first-consumer
  for the library (it's a thin `hclsimple` wrapper that would benefit
  directly).
- **`forge` is the outlier in scope.** It's the only repo with end-user
  `variable` prompting + a separate `.forge-vars.hcl` input file. That
  Terraform-style variable-input pattern is unique and probably too
  app-specific to centralize.
- **Dead code to clean up (not blocking):**
  `fwsync/docs/fwsync3/fwsync/pkg/framework/load.go` is a duplicate of the
  production loader nested inside the docs tree. Likely stale doc artifact.
- **Forward trajectory shifts the picture meaningfully** (per direction from the
  owner; not derivable from the current code):
  - **`fwsync` moves toward `forge`-shape.** Adds variables / vars-file
    decode and HCL-wrapped wiz rules (or the OpenSemgrep rule format).
    The Wiz SDK underneath continues to produce Wiz frameworks /
    automations / policies _and_ render Markdown for `rfc-site` /
    `rfc-api`. **Open question:** are the rule bodies inlined HCL ops
    (real grammar consumer) or HCL transport for an existing semgrep
    YAML/JSON pattern syntax (config wrapper, not a grammar consumer)?
    The answer materially changes the DSL trigger calculus — flag this
    for resolution before C is re-run.
  - **`claudelint` evolves to config-driven custom rules.** Today it only
    toggles Go-registered rules; the planned evolution lets users
    express rule logic in the HCL config. That is a real `subject op
    expected` consumer in waiting.
  - **`repo-guardian` likely adds a vars-file path** as the next config
    iteration.
  - **`docz` gains HCL support alongside legacy YAML.** The PST
    doc-model lives inside docz as a higher-order grammar where the
    config declares custom doctypes that compile down to docz's existing
    doctype config — so the prior "pst-doc-model" candidate collapses
    into a docz feature, not a separate consumer.
- **Variables / vars-file decode path becomes a 3-consumer shape.** `forge`
  today, `fwsync` planned, `repo-guardian` likely. That elevates it from
  "forge outlier" to a first-class library primitive: a Terraform-style
  `.tfvars`-equivalent file decoded into typed values and bound into
  the EvalContext at load time, with validation/choices on the variable
  declarations.
- **Two new shared needs surfaced by the docz meta-model** (and not
  previously flagged by current consumers):
  1. **Cross-block reference validation at decode time** — when a block
     declares a relationship to another named block, the target must
     resolve to a block of a stated kind, and a mismatch must surface
     as a diagnostic with HCL position info. Current consumers
     (`fwsync`, `wiz-access-cli`) do string-keyed lookups in Go
     post-decode, without position-aware diagnostics. A shared
     verb-and-target-kind primitive would help both retroactively.
  2. **Uniqueness constraints** — per-type uniqueness (unique
     `id_prefix`, unique rule IDs, unique resource labels) is currently
     convention-only or ad hoc. A generic uniqueness validator is a
     small, broadly useful library primitive.
- **Meta-models are a recurring shape worth supporting.** The docz
  meta-model pattern (a small declarative grammar on top of `gohcl`,
  validated in Go, used to generate multiple downstream artifacts) is
  likely to recur — doctype registries, plugin registries, tool
  catalogs, codegen manifests. Treat it as a first-class hclkit use case
  rather than a one-off: ship the building blocks (decode + validate +
  diag-surface + refined types + cross-block refs) and let consumers
  compose their own small grammars.
- **C1/DSL signal: false today, plausibly true in 12–18 months.** Today
  only `repo-guardian` exposes a `subject op expected` grammar.
  Forward-looking, `claudelint` (custom rules) and `fwsync` (wiz/OpenSemgrep
  rules, if HCL-native) become grammar consumers. Two of three would
  trip C1, but only if they align on an operator vocabulary. The
  recommendation should therefore be **library now with a defined DSL
  re-trigger condition**, not "defer the DSL forever."
- **Decision pattern today: "A several Yes · B Yes · C mostly No" →
  shared library (mechanism only).** A, B, X all pass; C1–C5 do not.
  Re-trigger condition: when ≥2 of {claudelint custom rules, fwsync
  wiz/OpenSemgrep rules, evolved repo-guardian} ship with an aligned
  operator vocabulary, re-run the C-section.

## Conclusion

**Answer: Library now, DSL deferred — with a defined re-trigger condition.**

The survey of 9 active repos plus 1 planned consumer (`docz` w/ HCL support)
maps onto the decision matrix as **"A several Yes · B Yes · C mostly No"**:

- **A (centralization warranted).** Five or more repos wrap
  `hclsimple`/`gohcl` with near-identical boilerplate; three repos build
  ad-hoc EvalContexts; durations and enums are validated post-decode in
  several places with no uniform diagnostics; the `env()` lookup is
  implemented two incompatible ways. There is real, recurring,
  per-app cost.
- **B (mechanism is what to share).** The shared pieces are loader,
  function library, refined primitives, validators, and diagnostics —
  not schemas. Every consumer keeps its own `Config` struct + tags.
  This is library territory, cleanly.
- **C (DSL not warranted today).** Only `repo-guardian` exposes a
  `subject op expected` rule grammar today. `fwsync` is a
  structured-content store, not an assertion engine. The `docz`
  meta-model is a meta-model, not a rule grammar. Forward-looking,
  `claudelint` custom rules and `fwsync` wiz/OpenSemgrep rules are
  plausible grammar consumers in 12–18 months, but neither exists yet
  and their operator vocabularies have not been designed. C6 (willing
  to own the DSL tax) is unanswered.
- **X (antipatterns avoided).** Nothing in the proposed library
  centralizes whole `Config` schemas, ties operators to specific
  domains, or generalizes from a single app's operator set.

The hypothesis was right on the library and partially wrong on the DSL:
`fwsync` and `repo-guardian` do **not** share a rule grammar today
(`fwsync` is a content store), so the "two repos already drive a DSL"
premise is refuted. The DSL question becomes a forward-looking one
gated on the trajectory above.

## Recommendation

### Build hclkit as a mechanism-only shared library now

Scope the v0 library around the recurring needs the survey actually
surfaced — no speculative operators, no rule grammar, no centralized
schemas.

**First-wave primitives** (each maps to evidence from ≥2 surveyed
consumers, present or planned):

1. **Loader + diagnostic wrapper around `gohcl`/`hclsimple`.** Replaces
   the near-duplicate boilerplate in fwsync, wiz-go-gen, claudelint,
   mcp-go-gen, spt, wiz-access-cli (and webhookd / docz when they land).
2. **EvalContext assembly.** Declarative registration of functions and
   variables/locals. Subsumes the three different shapes in spt,
   repo-guardian, forge — and the planned shapes in fwsync,
   repo-guardian (vars-file), and the docz meta-model.
3. **Standard function bundle, starting with `env()`.** Adopt the spt
   model (HCL function, evaluated at decode time) and retire
   repo-guardian's post-decode allowlist (`applyEnvOverrides`).
   Resolving this divergence early prevents the inconsistency from
   becoming permanent. Ship `forge`'s case helpers (`snakeCase`,
   `camelCase`, etc.) and `now()` in the same bundle.
4. **Vars-file decode path.** Lift forge's `.forge-vars.hcl` pattern to
   a first-class primitive: a separate, typed user-input file decoded
   into the EvalContext at load time, with `variable` declarations
   that carry `default`/`validate`/`choices`. Targets forge (existing),
   fwsync (planned), repo-guardian (likely).
5. **Refined `cty` Duration type.** Decode-time validation, replacing
   the `string` + `time.ParseDuration`-downstream pattern in
   repo-guardian, spt, and planned webhookd.
6. **Generic enum-refinement machinery.** Closed-set validation with
   HCL-position diagnostics. Replaces ad-hoc post-decode string checks
   in fwsync (Severity), claudelint (severity), repo-guardian (rule
   kinds), and planned docz meta-model (`class`, `permanence`).
7. **Cross-block reference validation.** Declared verbs with
   target-kind constraints, resolved at decode time with
   position-aware diagnostics. Targets the docz meta-model directly
   and helps fwsync / wiz-access-cli retrofit their Go-side string
   lookups.
8. **Generic uniqueness validator.** Per-type uniqueness on a named
   attribute (e.g. `id_prefix`, label, rule ID). Small primitive,
   broadly applicable.

Out of scope for v0:

- Refined CIDR / URL / port types — zero current consumers.
- Boolean composition / operator vocabularies / rule blocks — the DSL
  layer, deferred.
- Centralized `Config` schemas of any kind.
- Anything that ties operators to specific subjects or domains.

### Sequence the adopters

1. **First consumer: `wiz-access-cli` (PR #7).** It's an `hclsimple`
   thin wrapper with no idiosyncrasies — the path of least resistance
   to validate the loader/diagnostics surface end-to-end before
   anything depends on it.
2. **Second wave (low friction): `claudelint`, `mcp-go-gen`,
   `wiz-go-gen`.** All `gohcl`-with-nil-ctx, all trivial. Retrofit
   them once the loader API is stable.
3. **Third wave (drives the EvalContext surface): `spt`, then `forge`,
   then `fwsync` planned, then `repo-guardian`** (which is the most
   work, given its `hclsyntax` partial decode and `locals` handling).
4. **Webhookd and docz** consume the library natively when they
   actually start building.

### Defer the DSL layer with a defined re-trigger

**Trigger condition:** when ≥2 of {`claudelint` config-driven custom
rules, `fwsync` wiz/OpenSemgrep rule blocks, evolved `repo-guardian`
rule grammar} have shipped or have stable specs, and their operator
vocabularies overlap meaningfully, re-open the C-section of this
investigation. At that point, the question becomes a design question
about the operator core, not a discovery question about whether
sharing is warranted.

**Open questions to resolve before the DSL trigger fires:**

- Are `fwsync`'s wiz rule bodies inlined HCL operators (real grammar
  consumer) or HCL transport for an existing semgrep YAML/JSON pattern
  syntax (config wrapper)? The answer changes whether `fwsync` counts
  toward the trigger.
- Is the team willing to pay the DSL tax (C6) — versioned operator
  surface, backward-compat policy on operator names/semantics, written
  spec, diagnostics quality bar, editor support? Unanswered today.

### Housekeeping discovered in passing

- `fwsync/docs/fwsync3/fwsync/pkg/framework/load.go` is a duplicate of
  the production loader nested inside the docs tree. Likely stale
  artifact; confirm and prune.
- `webhookd` RFC/ADR documentation should be revisited once hclkit
  lands so IMPL-0004 can be planned against the library rather than
  re-implementing the loader/diagnostics pattern in-tree.

## References

- Conceptual model and rubric: this investigation (self-contained).
- Existence proof — HashiCorp shares the `hashicorp/hcl/v2` _toolkit_ across
  Terraform, Nomad, Packer, and Consul, but each defines its own language on top
  and none of them share schemas. Strong support for "centralize mechanism, not
  schema."
- `hashicorp/hcl/v2` — `gohcl` (struct-tag decode, config-format territory) vs
  `hcldec` / `hclsyntax` partial decode + custom eval (language territory).
- `zclconf/go-cty` — value model and custom/refined types for the primitive
  layer.
<!-- Parent RFC / ADR once this concludes -->
<!-- In-scope repo links (mirror the "Repos in Scope" table) -->
