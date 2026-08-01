# hclkit

HCL2 shared library + validator binary for the homelab fleet.

## Quickstart

```sh
mise install                  # toolchain
just                          # task menu
just build                    # binary at build/bin/hclkit
just test                     # race detector
./build/bin/hclkit --help
```

A `Makefile` mirroring the justfile target set is also available
(`make help`).

## CLI

```sh
hclkit version                # version + commit + build date
hclkit fmt [files...]         # format via hclwrite; --check for CI
hclkit validate [files...]    # parse-only validation, GCC-style diagnostics
hclkit lint --schema=x.hcl    # (Phase 4) schema-driven lint
```

Subcommand rollout tracks
[IMPL-0001](docs/impl/0001-hclkit-v0-library-and-validator-binary.md).

### Reserved flags

`--profile`, `--format`, `--no-color`, and `--schema-stdin` are
**reserved but not implemented** in v0 (per
[DESIGN-0001](docs/design/0001-hclkit-v0-library-and-validator-binary.md)).
They are documented now so user aliases and CI wrappers don't claim
them for other meanings before they land.

## Release

```sh
just release v0.1.0           # tags + pushes; CI runs goreleaser
```

Multi-arch archives land on the Forgejo (or GitHub) release page.
Version metadata (`version`, `commit`, `date`) is embedded via
`-ldflags` and surfaced by `hclkit version`. No container image ships
in v0 (deferred per DESIGN-0001 until a CI integration asks).

## Layout

```text
cmd/hclkit/             main package — cobra subcommands
pkg/hclkit/             public library API (Loader, Diagnostics, options)
internal/               private implementation (parser, testutil)
docs/                   docz-managed docs (rfc/adr/design/impl/plan/investigation)
.goreleaser.yml         release config
mise.toml               pinned toolchain
justfile                task runner (Makefile mirror in repo root)
```

## Conventions

See `CLAUDE.md` for the full operating notes (Go-specific +
homelab universals).

## License

Apache-2.0
