package ctytypes

import (
	"slices"
	"strings"
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
)

func TestEnumConstructor(t *testing.T) {
	values := []string{"low", "medium", "high", "low"}
	e := Enum("severity", values)

	if e.Name() != "severity" {
		t.Errorf("Name() = %q, want %q", e.Name(), "severity")
	}
	if got, want := e.Values(), []string{"low", "medium", "high"}; !slices.Equal(got, want) {
		t.Errorf("Values() = %v, want deduped first-occurrence order %v", got, want)
	}

	// Caller mutation after construction must not reach the validator.
	values[0] = "mutated"
	if diags := e.Validate("low", hcl.Range{}); diags.HasErrors() {
		t.Error(`Validate("low") errored after caller mutated the input slice`)
	}
	e.Values()[0] = "mutated"
	if got := e.Values()[0]; got != "low" {
		t.Errorf("Values()[0] = %q after mutating a returned slice, want %q", got, "low")
	}
}

func TestEnumValidate(t *testing.T) {
	e := Enum("severity", []string{"low", "medium", "high"})
	subject := hcl.Range{Filename: "rules.hcl", Start: hcl.Pos{Line: 4, Column: 12}}

	tests := []struct {
		name       string
		input      string
		wantErr    bool
		wantDetail string
	}{
		{name: "member", input: "medium"},
		{name: "miss", input: "urgent", wantErr: true, wantDetail: `Expected one of: low, medium, high; got "urgent".`},
		{name: "case-sensitive miss suggests", input: "High", wantErr: true, wantDetail: `Did you mean "high"?`},
		{name: "empty string miss", input: "", wantErr: true, wantDetail: `got ""`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diags := e.Validate(tt.input, subject)
			if diags.HasErrors() != tt.wantErr {
				t.Fatalf("Validate(%q) hasErrors = %v, want %v", tt.input, diags.HasErrors(), tt.wantErr)
			}
			if !tt.wantErr {
				return
			}
			if diags[0].Summary != "Invalid value for severity" {
				t.Errorf("Summary = %q, want %q", diags[0].Summary, "Invalid value for severity")
			}
			if !strings.Contains(diags[0].Detail, tt.wantDetail) {
				t.Errorf("Detail = %q, want it to contain %q", diags[0].Detail, tt.wantDetail)
			}
			if subj := diags[0].Subject; subj == nil || subj.Filename != "rules.hcl" || subj.Start.Line != 4 {
				t.Errorf("Subject = %v, want the caller-supplied range", diags[0].Subject)
			}
		})
	}
}

func TestEnumZeroValueAndEmptySet(t *testing.T) {
	var zero EnumType
	if diags := zero.Validate("anything", hcl.Range{}); !diags.HasErrors() {
		t.Error("zero EnumType accepted a value; the zero value must reject every input")
	}

	empty := Enum("mode", nil)
	if diags := empty.Validate("anything", hcl.Range{}); !diags.HasErrors() {
		t.Error("empty Enum accepted a value; an empty set must reject every input")
	}
}

func TestEnumDecodeExpr(t *testing.T) {
	e := Enum("severity", []string{"low", "high"})

	got, diags := e.DecodeExpr(exprFor(t, `v = "high"`), nil)
	if diags.HasErrors() {
		t.Fatalf("DecodeExpr diags = %s, want none", diags)
	}
	if got != "high" {
		t.Errorf("DecodeExpr = %q, want %q", got, "high")
	}

	got, diags = e.DecodeExpr(exprFor(t, `v = "urgent"`), nil)
	if !diags.HasErrors() {
		t.Fatal("DecodeExpr(urgent) diags = none, want membership error")
	}
	if got != "" {
		t.Errorf("DecodeExpr(urgent) = %q, want empty string on failure", got)
	}
	if subj := diags[0].Subject; subj == nil || subj.Filename != "test.hcl" {
		t.Errorf("Subject = %v, want anchored in test.hcl", diags[0].Subject)
	}
}

func TestEnumDecodeExprTypeAndNull(t *testing.T) {
	e := Enum("severity", []string{"low"})

	_, diags := e.DecodeExpr(exprFor(t, "v = [1]"), nil)
	if !diags.HasErrors() || !strings.Contains(diags.Error(), "Value must be a string") {
		t.Errorf("DecodeExpr(list) diags = %q, want type error", diags.Error())
	}

	_, diags = e.DecodeExpr(exprFor(t, "v = null"), nil)
	if !diags.HasErrors() || !strings.Contains(diags.Error(), "Value must not be null") {
		t.Errorf("DecodeExpr(null) diags = %q, want null error", diags.Error())
	}

	ctx := &hcl.EvalContext{Variables: map[string]cty.Value{
		"pending": cty.UnknownVal(cty.String),
	}}
	_, diags = e.DecodeExpr(exprFor(t, "v = pending"), ctx)
	if !diags.HasErrors() || !strings.Contains(diags.Error(), "Value must be known at decode time") {
		t.Errorf("DecodeExpr(unknown) diags = %q, want unknown error", diags.Error())
	}
}

// TestEnumCollectAll pins collect-all across several bad attributes:
// each failed decode contributes its own anchored diagnostic.
func TestEnumCollectAll(t *testing.T) {
	e := Enum("mode", []string{"fast", "safe"})
	exprs := []hcl.Expression{
		exprFor(t, `v = "slow"`),
		exprFor(t, `v = "Safe"`),
	}

	var diags hcl.Diagnostics
	for _, expr := range exprs {
		_, exprDiags := e.DecodeExpr(expr, nil)
		diags = diags.Extend(exprDiags)
	}
	if len(diags) != 2 {
		t.Fatalf("collected diags = %d, want 2", len(diags))
	}
	if !strings.Contains(diags[1].Detail, `Did you mean "safe"?`) {
		t.Errorf("second diag Detail = %q, want case suggestion", diags[1].Detail)
	}
}
