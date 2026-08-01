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
// functions and variables, multi-file merge behavior, and a
// diagnostic tee writer. Later phases add subpackages for the
// standard function bundle, vars-file decoding, refined cty types,
// partial decoding, and decode-time validators; see DESIGN-0001 in
// the repository's docs tree.
package hclkit
