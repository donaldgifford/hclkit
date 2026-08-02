package partial

import (
	"fmt"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hcldec"
	"github.com/zclconf/go-cty/cty"
)

// ExprMap is a name → hcl.Expression lookup for attributes retained
// past the initial decode, to be evaluated later against a context
// assembled after decoding (e.g. once variables are bound). Keys are
// flat attribute names scoped to the decoded body.
type ExprMap map[string]hcl.Expression

// DecodeSpec decodes body against spec, first peeling off the
// attributes named in retain as unevaluated expressions. Retained
// attributes are removed from spec decoding, so a retain name that
// also appears in the spec's implied schema is an error. An absent
// retained attribute is simply missing from the returned ExprMap —
// not a diagnostic; the caller decides whether absence matters.
//
// ctx may be nil for a literals-only eager decode. On error the
// decoded value and ExprMap are still returned as far as they got —
// hcldec produces partial values — so callers gate on
// diags.HasErrors(), same as every Load* path.
func DecodeSpec(body hcl.Body, spec hcldec.Spec, ctx *hcl.EvalContext, retain ...string) (cty.Value, ExprMap, hcl.Diagnostics) {
	if body == nil || spec == nil {
		return cty.NilVal, nil, hcl.Diagnostics{{
			Severity: hcl.DiagError,
			Summary:  "Invalid decode spec",
			Detail:   "DecodeSpec requires a non-nil body and spec.",
		}}
	}

	if len(retain) == 0 {
		val, diags := hcldec.Decode(body, spec, ctx)
		return val, ExprMap{}, diags
	}

	var diags hcl.Diagnostics

	implied := hcldec.ImpliedSchema(spec)
	specAttrs := make(map[string]struct{}, len(implied.Attributes))
	for _, attr := range implied.Attributes {
		specAttrs[attr.Name] = struct{}{}
	}

	names := make([]string, 0, len(retain))
	seen := make(map[string]struct{}, len(retain))
	for _, name := range retain {
		if _, dup := seen[name]; dup {
			continue
		}
		seen[name] = struct{}{}
		if _, conflict := specAttrs[name]; conflict {
			diags = diags.Append(&hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  "Conflicting retained attribute",
				Detail: fmt.Sprintf(
					"Attribute %q is both retained and declared in the spec; retained attributes are removed from spec decoding.",
					name),
			})
			continue
		}
		names = append(names, name)
	}
	if diags.HasErrors() {
		return cty.NilVal, nil, diags
	}

	exprs := make(ExprMap, len(names))
	attrSchema := make([]hcl.AttributeSchema, len(names))
	for i, name := range names {
		attrSchema[i] = hcl.AttributeSchema{Name: name}
	}
	content, remain, pcDiags := body.PartialContent(&hcl.BodySchema{Attributes: attrSchema})
	diags = diags.Extend(pcDiags)
	for name, attr := range content.Attributes {
		exprs[name] = attr.Expr
	}

	val, decDiags := hcldec.Decode(remain, spec, ctx)
	return val, exprs, diags.Extend(decDiags)
}
