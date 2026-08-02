package validate

import (
	"fmt"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"
)

// RefValidator checks that every value of the Verb attribute —
// wherever it appears, in blocks of any kind or at the file root —
// names a declared block of kind TargetKind (identified by its first
// label). Diagnostics anchor at the reference site, per element for
// list expressions.
type RefValidator struct {
	Verb       string
	TargetKind string
}

// NewRefValidator returns a validator resolving Verb attribute values
// against TargetKind block labels.
func NewRefValidator(verb, targetKind string) *RefValidator {
	return &RefValidator{Verb: verb, TargetKind: targetKind}
}

// Validate implements the hclkit.Validator contract. Declarations are
// collected across all bodies before any reference is checked, so
// cross-file references resolve regardless of file order. Expression
// evaluation errors are skipped — the decode pass reports those —
// and unknown or null values are ignored.
func (v *RefValidator) Validate(bodies []hcl.Body, ctx *hcl.EvalContext) hcl.Diagnostics {
	declared := make(map[string]struct{})
	walkBodies(bodies, nil, func(block *hclsyntax.Block) {
		if block.Type == v.TargetKind && len(block.Labels) > 0 {
			declared[block.Labels[0]] = struct{}{}
		}
	})

	var diags hcl.Diagnostics
	walkBodies(bodies, func(attr *hclsyntax.Attribute) {
		if attr.Name == v.Verb {
			diags = diags.Extend(v.checkAttr(attr, declared, ctx))
		}
	}, nil)
	return diags
}

func (v *RefValidator) checkAttr(attr *hclsyntax.Attribute, declared map[string]struct{}, ctx *hcl.EvalContext) hcl.Diagnostics {
	// A syntactic list gives per-element ranges — the best anchors.
	if elems, listDiags := hcl.ExprList(attr.Expr); !listDiags.HasErrors() {
		var diags hcl.Diagnostics
		for _, elem := range elems {
			val, valDiags := elem.Value(ctx)
			if valDiags.HasErrors() {
				continue // decode reports evaluation failures; don't double up
			}
			diags = diags.Extend(v.checkValue(val, declared, elem.Range()))
		}
		return diags
	}

	val, valDiags := attr.Expr.Value(ctx)
	if valDiags.HasErrors() {
		return nil // decode reports evaluation failures; don't double up
	}
	exprRange := attr.Expr.Range()
	if val.IsNull() || !val.IsKnown() {
		return nil
	}
	if ty := val.Type(); ty.IsTupleType() || ty.IsListType() || ty.IsSetType() {
		var diags hcl.Diagnostics
		for it := val.ElementIterator(); it.Next(); {
			_, elem := it.Element()
			diags = diags.Extend(v.checkValue(elem, declared, exprRange))
		}
		return diags
	}
	return v.checkValue(val, declared, exprRange)
}

// checkValue validates one evaluated reference value against the
// declaration set, anchoring any diagnostic at subject.
func (v *RefValidator) checkValue(val cty.Value, declared map[string]struct{}, subject hcl.Range) hcl.Diagnostics {
	if val.IsNull() || !val.IsKnown() {
		return nil
	}
	if val.Type() != cty.String {
		return hcl.Diagnostics{{
			Severity: hcl.DiagError,
			Summary:  "Invalid reference value",
			Detail:   fmt.Sprintf("Attribute %q must be a string or list of strings.", v.Verb),
			Subject:  &subject,
		}}
	}
	if _, ok := declared[val.AsString()]; !ok {
		return hcl.Diagnostics{{
			Severity: hcl.DiagError,
			Summary:  fmt.Sprintf("Reference to undeclared %s", v.TargetKind),
			Detail: fmt.Sprintf("No %s block named %q is declared in the loaded configuration.",
				v.TargetKind, val.AsString()),
			Subject: &subject,
		}}
	}
	return nil
}
