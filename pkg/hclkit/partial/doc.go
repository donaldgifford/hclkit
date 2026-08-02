// Package partial provides decode surfaces gohcl's struct-tag pass
// doesn't cover: hcldec spec decoding with retained hcl.Expression
// handles for late-bound attributes (forge's condition.when shape),
// and ordered block-kind iteration (repo-guardian's locals-first
// shape).
//
// Nested retention composes: Walk over the enclosing blocks, then
// DecodeSpec each block.Body with its own retain list. ExprMap keys
// are always flat attribute names scoped to the body passed in.
//
// This package must import only hcl/cty and the standard library —
// never hclkit itself — so pkg/hclkit can depend on it without a
// cycle. Functions return hcl.Diagnostics; Loader.LoadSpec wraps
// them into hclkit.Diagnostics at the boundary.
package partial
