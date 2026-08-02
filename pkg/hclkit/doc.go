// Package hclkit is the public API root for the hclkit library: a
// composable Loader over hashicorp/hcl's gohcl decode path with a
// consistent, position-aware Diagnostics surface.
//
// The common case is a single import and a single call:
//
//	var cfg Config
//	diags := hclkit.New().LoadFile("app.hcl", &cfg)
//	if diags.HasErrors() {
//		_, _ = diags.WriteTo(os.Stderr)
//		os.Exit(1)
//	}
//
// Loaders compose through functional options: eval contexts, extra
// functions and variables, multi-file merge behavior, vars files
// (WithVarsFile), and a diagnostic tee writer. EvalCtxBuilder
// assembles eval contexts from functions, variables, and locals
// blocks.
//
// Subpackages cover the shapes gohcl alone doesn't:
//
//   - funcs: the standard HCL function bundle (case helpers, now,
//     env) for Loader and EvalCtxBuilder registration.
//   - varsfile: Terraform-style variable declarations with
//     validation blocks, resolved from literal assignment files.
//   - ctytypes: refined decode helpers (durations, closed string
//     sets) with HCL-position diagnostics.
//   - partial: hcldec spec decoding with retained expressions and
//     ordered block-kind walks; Loader.LoadSpec is the entry point.
//   - validate: decode-time cross-block validators (reference
//     resolution, uniqueness), registered via WithValidators.
//
// See DESIGN-0001 in the repository's docs tree for the rationale.
package hclkit
