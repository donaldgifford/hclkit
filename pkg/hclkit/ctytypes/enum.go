package ctytypes

import (
	"fmt"
	"slices"
	"strings"

	"github.com/hashicorp/hcl/v2"
)

// EnumType is a closed set of allowed string values. Construct with
// Enum; the value is immutable afterward and safe to share. The zero
// value rejects every input.
type EnumType struct {
	name   string
	values []string
	index  map[string]struct{}
}

// Enum returns a closed-set validator named name — the name appears
// in diagnostics ("Invalid value for severity"). Matching is
// case-sensitive and exact. Duplicates in values are dropped,
// preserving first-occurrence order; the slice is cloned so later
// caller mutation cannot reach the validator. An empty set is legal
// and rejects every input.
func Enum(name string, values []string) EnumType {
	vals := make([]string, 0, len(values))
	index := make(map[string]struct{}, len(values))
	for _, v := range values {
		if _, ok := index[v]; ok {
			continue
		}
		index[v] = struct{}{}
		vals = append(vals, v)
	}
	return EnumType{name: name, values: vals, index: index}
}

// Name returns the enum's diagnostic name.
func (e EnumType) Name() string { return e.name }

// Values returns the allowed values in constructor order, deduped —
// for documentation and interactive prompting.
func (e EnumType) Values() []string { return slices.Clone(e.values) }

// DecodeExpr evaluates expr against ctx and validates the result is
// in the set. Failures return "" and diagnostics anchored at the
// expression.
func (e EnumType) DecodeExpr(expr hcl.Expression, ctx *hcl.EvalContext) (string, hcl.Diagnostics) {
	val, diags := expr.Value(ctx)
	if diags.HasErrors() {
		return "", diags
	}

	s, sDiags := stringValue(val, expr.Range(), e.summary(), "Value")
	diags = diags.Extend(sDiags)
	if sDiags.HasErrors() {
		return "", diags
	}

	if vDiags := e.Validate(s, expr.Range()); vDiags.HasErrors() {
		return "", diags.Extend(vDiags)
	}
	return s, diags
}

// Validate checks membership of an already-decoded string, anchoring
// any failure at subject. A near-miss differing only in case is
// suggested in the diagnostic.
func (e EnumType) Validate(s string, subject hcl.Range) hcl.Diagnostics {
	if _, ok := e.index[s]; ok {
		return nil
	}

	detail := fmt.Sprintf("Expected one of: %s; got %q.", strings.Join(e.values, ", "), s)
	if i := slices.IndexFunc(e.values, func(v string) bool { return strings.EqualFold(v, s) }); i >= 0 {
		detail = fmt.Sprintf("%s Did you mean %q?", detail, e.values[i])
	}
	return hcl.Diagnostics{{
		Severity: hcl.DiagError,
		Summary:  e.summary(),
		Detail:   detail,
		Subject:  &subject,
	}}
}

func (e EnumType) summary() string {
	return fmt.Sprintf("Invalid value for %s", e.name)
}
