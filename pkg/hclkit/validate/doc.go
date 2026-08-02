// Package validate provides decode-time cross-block validators:
// reference resolution (an attribute names blocks of a kind that must
// be declared) and per-kind attribute uniqueness, both with
// diagnostics anchored at the offending source range.
//
// Validators are registered on a Loader via hclkit.WithValidators and
// receive every parsed file body as written, so declarations resolve
// across all files of a LoadDir call. Validation is native-syntax
// only: enumerating arbitrary block types requires *hclsyntax.Body,
// so JSON-syntax bodies contribute nothing and are skipped silently.
//
// This package must import only hcl/cty and the standard library —
// never hclkit itself — so pkg/hclkit can depend on it without a
// cycle. Validators satisfy hclkit.Validator structurally.
package validate
