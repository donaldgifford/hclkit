package ctytypes

import (
	"fmt"
	"time"

	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/convert"
)

// DecodeDuration evaluates expr against ctx and parses the result as
// a Go duration ("30s", "1h30m"). Failures — evaluation errors, a
// non-string value, or an unparseable duration — return a zero
// duration and diagnostics anchored at the expression. Negative
// durations are allowed; policy belongs to the consumer.
func DecodeDuration(expr hcl.Expression, ctx *hcl.EvalContext) (time.Duration, hcl.Diagnostics) {
	val, diags := expr.Value(ctx)
	if diags.HasErrors() {
		return 0, diags
	}

	s, sDiags := stringValue(val, expr.Range(), "Invalid duration", "Duration")
	diags = diags.Extend(sDiags)
	if sDiags.HasErrors() {
		return 0, diags
	}

	d, parseDiags := ValidateDuration(s, expr.Range())
	return d, diags.Extend(parseDiags)
}

// ValidateDuration parses s as a Go duration, anchoring any failure
// at subject — the post-decode path for consumers that already hold a
// plain string and its source range.
func ValidateDuration(s string, subject hcl.Range) (time.Duration, hcl.Diagnostics) {
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, hcl.Diagnostics{{
			Severity: hcl.DiagError,
			Summary:  "Invalid duration",
			Detail: fmt.Sprintf(
				"Cannot parse %q as a duration; use Go duration syntax such as %q or %q.",
				s, "30s", "1h30m"),
			Subject: &subject,
		}}
	}
	return d, nil
}

// stringValue reduces val to a known, non-null string, or explains
// why it can't with a diagnostic anchored at subject. noun names the
// value in Detail text ("Duration", "Value").
func stringValue(val cty.Value, subject hcl.Range, summary, noun string) (string, hcl.Diagnostics) {
	fail := func(detail string) hcl.Diagnostics {
		return hcl.Diagnostics{{
			Severity: hcl.DiagError,
			Summary:  summary,
			Detail:   detail,
			Subject:  &subject,
		}}
	}

	converted, err := convert.Convert(val, cty.String)
	if err != nil {
		return "", fail(fmt.Sprintf("%s must be a string; got %s.", noun, val.Type().FriendlyName()))
	}
	if converted.IsNull() {
		return "", fail(fmt.Sprintf("%s must not be null.", noun))
	}
	if !converted.IsKnown() {
		return "", fail(fmt.Sprintf("%s must be known at decode time.", noun))
	}
	return converted.AsString(), nil
}
