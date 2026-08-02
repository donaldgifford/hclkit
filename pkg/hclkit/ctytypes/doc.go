// Package ctytypes provides refined decode helpers for HCL values
// that need validation beyond their cty type: Go-syntax durations and
// closed string sets, both with diagnostics anchored at real HCL
// source positions.
//
// gohcl struct-tag decode has no custom-type hook — gocty.ImpliedType
// maps a time.Duration field to cty.Number, so a bare HCL number
// decodes as nanoseconds without ever parsing. Consumers therefore
// declare such fields as hcl.Expression and hand them to
// DecodeDuration or EnumType.DecodeExpr, or validate an
// already-decoded string via ValidateDuration or EnumType.Validate
// with a range they hold (recorded as a DESIGN-0001 deviation in
// IMPL-0001).
//
// This package must import only hcl/cty and the standard library —
// never hclkit itself — so pkg/hclkit can depend on it without a
// cycle.
package ctytypes
