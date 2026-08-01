# Changelog

All notable changes to this project are documented here. The format is
based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and
this project adheres to [Semantic Versioning](https://semver.org/).
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

