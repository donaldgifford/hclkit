# Changelog

All notable changes to this project are documented here. The format is
based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and
this project adheres to [Semantic Versioning](https://semver.org/).
## [unreleased]

### Features

- *(funcs)* Add standard function bundle (env, case helpers, now)
- *(hclkit)* Add EvalCtxBuilder with deferred locals evaluation
- *(varsfile)* Add variable declaration decode and resolution
- *(hclkit)* Wire vars-file mode into the Loader
- *(examples)* Add envfunc (spt shape) and varsfile (forge shape) examples
- *(ctytypes)* Add Duration and Enum decode helpers with HCL-position diagnostics
- *(ctytypes)* Record gohcl spt compat spike; treat nil/null expressions as absent
- *(partial)* Add DecodeSpec/Walk and Loader.LoadSpec per OQ-7
- *(validate)* Add RefValidator/UniqueValidator and WithValidators wiring
- *(cli)* Add hclkit lint --schema with the minimal v0 schema grammar

### Documentation

- *(impl)* Check off Phase 1 per-PR tag task (v0.1.0)
- *(claude)* Document label-driven release flow
- *(impl)* Amend vars-file spec from Phase 3 architecture review
- Phase-end pass for IMPL-0001 Phase 3
- *(rfc)* Record the DSL re-trigger evaluation — not triggered
- Record the pre-1.0 API sweep — no breaking changes needed

### Testing

- *(ctytypes)* Add property-based fuzz targets for Duration and Enum
- *(bench)* Add load+decode benchmarks for forge and repo-guardian shapes
- *(partial)* Add the v1.0 partial-decode gate over vendored consumer fixtures

## [0.1.0] - 2026-08-01

### Features

- *(build)* Inject main.date via ldflags in justfile, Makefile, goreleaser
- *(cli)* Restructure cmd/hclkit into cobra dispatch with version subcommand
- *(testutil)* Add golden-file helpers with -update regeneration
- *(parser)* Add internal hclparse wrapper with extension dispatch
- *(hclkit)* Add Diagnostics with GCC-prefix renderer
- *(hclkit)* Add Loader with functional options and merge modes
- *(cli)* Add fmt and validate subcommands
- *(examples)* Add nilctx consumer example with integration test

### Documentation

- Scaffold hclkit foundations and refresh CLAUDE.md
- *(impl)* Add IMPL-0001 phased implementation plan for hclkit v0
- *(readme)* Document reserved CLI flags; refresh stale sections
- *(impl)* Record Phase 1 verified success criteria
- *(impl)* Mark IMPL-0001 In Progress
- *(impl)* Record OQ-7 decision — dedicated LoadSpec entry point
- *(impl)* Check off Phase 1-scope testing-plan items
- *(impl)* Split fleet adoption out of IMPL-0001 into IMPL-0002
- Drop two consumer repos from fleet docs and survey

### Styling

- *(hclkit)* Apply go-style review findings

### Miscellaneous Tasks

- *(markdownlint)* Allow repeated sibling headings for docz IMPL docs
- *(justfile)* Remove claudelint leftovers
- *(make)* Add Makefile mirroring the justfile target set
- *(coverage)* Gate pkg/... alongside internal/, ignore examples/ in codecov
- *(trufflehog)* Pin action to v3.96.0

