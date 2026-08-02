# Vendored consumer fixtures (OQ-6)

Sanitized snapshots of real consumer configs for the partial-decode
v1.0 gate (`pkg/hclkit/partialgate_test.go`). Refresh manually when a
consumer changes shape and update the provenance below.

| Fixture | Source repo | Path | Commit |
|---|---|---|---|
| `forge-blueprint.hcl` | `donaldgifford/forge` | `testdata/v2-registry/go/api/blueprint.hcl` | `83e3789` (2026-08-02) |
| `repo-guardian-policy.hcl` | `donaldgifford/repo-guardian` | `examples/guardian-full.hcl` | `eb451e1` (2026-08-02) |

Note: the forge blueprint uses forge's *legacy* variable grammar
(`required` attribute, quoted type names) — it deliberately does NOT
decode through `pkg/hclkit/varsfile`, which implements the
Terraform-style grammar forge is migrating to (RFC-0003). The gate
exercises the `partial` surfaces against it, not varsfile.
