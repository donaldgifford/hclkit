package ctytypes

import (
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
	"github.com/zclconf/go-cty/cty/function/stdlib"
)

// exprFor parses src as a one-attribute body ("v = ...") and returns
// the attribute's expression, so Subject ranges in tests come from
// real parsed positions.
func exprFor(t *testing.T, src string) hcl.Expression {
	t.Helper()
	file, diags := hclsyntax.ParseConfig([]byte(src), "test.hcl", hcl.InitialPos)
	if diags.HasErrors() {
		t.Fatalf("parsing %q: %s", src, diags)
	}
	attrs, attrDiags := file.Body.JustAttributes()
	if attrDiags.HasErrors() {
		t.Fatalf("attributes of %q: %s", src, attrDiags)
	}
	attr, ok := attrs["v"]
	if !ok {
		t.Fatalf("source %q has no attribute v", src)
	}
	return attr.Expr
}

func TestDecodeDuration(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want time.Duration
	}{
		{name: "seconds", src: `v = "30s"`, want: 30 * time.Second},
		{name: "compound", src: `v = "1h30m"`, want: 90 * time.Minute},
		{name: "negative allowed", src: `v = "-5m"`, want: -5 * time.Minute},
		{name: "zero", src: `v = "0s"`, want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, diags := DecodeDuration(exprFor(t, tt.src), nil)
			if diags.HasErrors() {
				t.Fatalf("DecodeDuration(%s) diags = %s, want none", tt.src, diags)
			}
			if got != tt.want {
				t.Errorf("DecodeDuration(%s) = %v, want %v", tt.src, got, tt.want)
			}
		})
	}
}

func TestDecodeDurationDiagnostics(t *testing.T) {
	tests := []struct {
		name       string
		src        string
		wantDetail string
	}{
		{
			name:       "unparseable string",
			src:        `v = "not-a-duration"`,
			wantDetail: `Cannot parse "not-a-duration" as a duration`,
		},
		{
			name: "bare number is the nanoseconds trap",
			src:  `v = 30`,
			// convert.Convert stringifies the number, then parse
			// fails — a bare 30 never silently becomes 30ns.
			wantDetail: `Cannot parse "30" as a duration`,
		},
		{
			name:       "non-string type",
			src:        `v = ["30s"]`,
			wantDetail: "Duration must be a string",
		},
		{
			name:       "null",
			src:        `v = null`,
			wantDetail: "Duration must not be null",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, diags := DecodeDuration(exprFor(t, tt.src), nil)
			if !diags.HasErrors() {
				t.Fatalf("DecodeDuration(%s) diags = none, want error", tt.src)
			}
			if !strings.Contains(diags.Error(), tt.wantDetail) {
				t.Errorf("DecodeDuration(%s) diags = %q, want detail containing %q",
					tt.src, diags.Error(), tt.wantDetail)
			}
			if subj := diags[0].Subject; subj == nil || subj.Filename != "test.hcl" || subj.Start.Line != 1 {
				t.Errorf("DecodeDuration(%s) Subject = %v, want anchored in test.hcl line 1", tt.src, subj)
			}
		})
	}
}

func TestDecodeDurationUnknownValue(t *testing.T) {
	ctx := &hcl.EvalContext{Variables: map[string]cty.Value{
		"pending": cty.UnknownVal(cty.String),
	}}
	_, diags := DecodeDuration(exprFor(t, "v = pending"), ctx)
	if !diags.HasErrors() {
		t.Fatal("DecodeDuration(unknown) diags = none, want error")
	}
	if !strings.Contains(diags.Error(), "Duration must be known at decode time") {
		t.Errorf("diags = %q, want known-at-decode-time detail", diags.Error())
	}
}

func TestDecodeDurationEvalError(t *testing.T) {
	_, diags := DecodeDuration(exprFor(t, "v = missing.attr"), &hcl.EvalContext{})
	if !diags.HasErrors() {
		t.Fatal("DecodeDuration(bad reference) diags = none, want evaluation error")
	}
}

func TestDecodeDurationUsesContext(t *testing.T) {
	ctx := &hcl.EvalContext{
		Variables: map[string]cty.Value{"timeout": cty.StringVal("45")},
		Functions: map[string]function.Function{"format": stdlib.FormatFunc},
	}
	got, diags := DecodeDuration(exprFor(t, `v = format("%ss", timeout)`), ctx)
	if diags.HasErrors() {
		t.Fatalf("DecodeDuration diags = %s, want none", diags)
	}
	if got != 45*time.Second {
		t.Errorf("DecodeDuration = %v, want 45s", got)
	}
}

func TestValidateDuration(t *testing.T) {
	subject := hcl.Range{
		Filename: "held.hcl",
		Start:    hcl.Pos{Line: 7, Column: 3, Byte: 40},
		End:      hcl.Pos{Line: 7, Column: 9, Byte: 46},
	}

	got, diags := ValidateDuration("90s", subject)
	if diags.HasErrors() {
		t.Fatalf("ValidateDuration(90s) diags = %s, want none", diags)
	}
	if got != 90*time.Second {
		t.Errorf("ValidateDuration(90s) = %v, want 90s", got)
	}

	_, diags = ValidateDuration("bogus", subject)
	if !diags.HasErrors() {
		t.Fatal("ValidateDuration(bogus) diags = none, want error")
	}
	if subj := diags[0].Subject; subj == nil || subj.Filename != "held.hcl" || subj.Start.Line != 7 {
		t.Errorf("Subject = %v, want the caller-supplied range", diags[0].Subject)
	}
}

// TestDurationConsumerPattern locks the documented gohcl shape: the
// field is an hcl.Expression, decoded lazily via DecodeDuration.
func TestDurationConsumerPattern(t *testing.T) {
	src := `
timeout = "2m"
retry   = "oops"
`
	file, parseDiags := hclsyntax.ParseConfig([]byte(src), "app.hcl", hcl.InitialPos)
	if parseDiags.HasErrors() {
		t.Fatalf("parse: %s", parseDiags)
	}

	var cfg struct {
		Timeout hcl.Expression `hcl:"timeout"`
		Retry   hcl.Expression `hcl:"retry"`
	}
	if diags := gohcl.DecodeBody(file.Body, nil, &cfg); diags.HasErrors() {
		t.Fatalf("DecodeBody: %s", diags)
	}

	timeout, diags := DecodeDuration(cfg.Timeout, nil)
	if diags.HasErrors() {
		t.Fatalf("DecodeDuration(timeout) diags = %s, want none", diags)
	}
	if timeout != 2*time.Minute {
		t.Errorf("timeout = %v, want 2m", timeout)
	}

	_, diags = DecodeDuration(cfg.Retry, nil)
	if !diags.HasErrors() {
		t.Fatal("DecodeDuration(retry) diags = none, want parse error")
	}
	if subj := diags[0].Subject; subj == nil || subj.Filename != "app.hcl" || subj.Start.Line != 3 {
		t.Errorf("Subject = %v, want app.hcl line 3 (range survives gohcl decode)", diags[0].Subject)
	}
}
