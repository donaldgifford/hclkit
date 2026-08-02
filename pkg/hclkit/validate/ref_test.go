package validate

import (
	"strings"
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/zclconf/go-cty/cty"
)

func parseBody(t *testing.T, filename, src string) hcl.Body {
	t.Helper()
	file, diags := hclparse.NewParser().ParseHCL([]byte(src), filename)
	if diags.HasErrors() {
		t.Fatalf("parsing %s: %s", filename, diags)
	}
	return file.Body
}

func TestRefValidatorResolves(t *testing.T) {
	body := parseBody(t, "a.hcl", `
doctype "rfc" {
  decides = ["policy", "guide"]
}

doctype "policy" {}

doctype "guide" {}
`)

	v := NewRefValidator("decides", "doctype")
	if diags := v.Validate([]hcl.Body{body}, nil); diags.HasErrors() {
		t.Fatalf("Validate diags = %s, want none", diags)
	}
}

func TestRefValidatorMissingTarget(t *testing.T) {
	body := parseBody(t, "a.hcl", `
doctype "rfc" {
  decides = ["policy", "missing"]
}

doctype "policy" {}
`)

	diags := NewRefValidator("decides", "doctype").Validate([]hcl.Body{body}, nil)
	if len(diags) != 1 {
		t.Fatalf("diags = %d, want exactly 1", len(diags))
	}
	d := diags[0]
	if d.Summary != "Reference to undeclared doctype" {
		t.Errorf("Summary = %q", d.Summary)
	}
	if !strings.Contains(d.Detail, `No doctype block named "missing"`) {
		t.Errorf("Detail = %q", d.Detail)
	}
	// Anchor is the list ELEMENT, not the whole attribute: "missing"
	// starts at column 24 on line 3.
	if d.Subject == nil || d.Subject.Filename != "a.hcl" || d.Subject.Start.Line != 3 || d.Subject.Start.Column != 24 {
		t.Errorf("Subject = %v, want a.hcl:3:24 (the bad element)", d.Subject)
	}
}

func TestRefValidatorCrossBodyAndCrossKind(t *testing.T) {
	// Declaration in body A; reference from a DIFFERENT block kind in
	// body B (fwsync's rule{set=...} shape) — scalar string value.
	declBody := parseBody(t, "01-decl.hcl", `
set "golden" {}
`)
	useBody := parseBody(t, "02-use.hcl", `
rule "check" {
  set = "golden"
}

rule "broken" {
  set = "nope"
}
`)

	diags := NewRefValidator("set", "set").Validate([]hcl.Body{declBody, useBody}, nil)
	if len(diags) != 1 {
		t.Fatalf("diags = %d, want 1: %s", len(diags), diags)
	}
	if diags[0].Subject.Filename != "02-use.hcl" || diags[0].Subject.Start.Line != 7 {
		t.Errorf("Subject = %v, want 02-use.hcl line 7", diags[0].Subject)
	}
}

func TestRefValidatorValueShapes(t *testing.T) {
	ctx := &hcl.EvalContext{Variables: map[string]cty.Value{
		"refs":    cty.ListVal([]cty.Value{cty.StringVal("a"), cty.StringVal("nope")}),
		"pending": cty.UnknownVal(cty.String),
	}}

	tests := []struct {
		name      string
		src       string
		wantDiags int
		wantIn    string
	}{
		{name: "scalar ok", src: "doctype \"a\" {}\nuses = \"a\"\n"},
		{name: "scalar miss", src: "uses = \"zzz\"\n", wantDiags: 1, wantIn: "undeclared"},
		{name: "evaluated list miss", src: "doctype \"a\" {}\nuses = refs\n", wantDiags: 1, wantIn: "undeclared"},
		{name: "non-string value", src: "uses = 42\n", wantDiags: 1, wantIn: "Invalid reference value"},
		{name: "non-string element", src: "uses = [42]\n", wantDiags: 1, wantIn: "Invalid reference value"},
		{name: "null skipped", src: "uses = null\n"},
		{name: "unknown skipped", src: "uses = pending\n"},
		{name: "eval error skipped", src: "uses = nosuch.attr\n"},
		{name: "other attrs ignored", src: "other = \"zzz\"\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := parseBody(t, "t.hcl", tt.src)
			diags := NewRefValidator("uses", "doctype").Validate([]hcl.Body{body}, ctx)
			if len(diags) != tt.wantDiags {
				t.Fatalf("diags = %d (%s), want %d", len(diags), diags, tt.wantDiags)
			}
			if tt.wantDiags > 0 && !strings.Contains(diags.Error(), tt.wantIn) {
				t.Errorf("diags = %q, want %q", diags.Error(), tt.wantIn)
			}
		})
	}
}

func TestRefValidatorZeroLabelTargetSkipped(t *testing.T) {
	body := parseBody(t, "t.hcl", `
doctype {}

uses = "anything"
`)
	diags := NewRefValidator("uses", "doctype").Validate([]hcl.Body{body}, nil)
	if len(diags) != 1 {
		t.Fatalf("diags = %d, want 1 (zero-label block declares nothing)", len(diags))
	}
}

func TestRefValidatorJSONBodySkipped(t *testing.T) {
	file, diags := hclparse.NewParser().ParseJSON([]byte(`{"uses": "zzz"}`), "t.json")
	if diags.HasErrors() {
		t.Fatalf("parse json: %s", diags)
	}
	got := NewRefValidator("uses", "doctype").Validate([]hcl.Body{file.Body}, nil)
	if len(got) != 0 {
		t.Errorf("diags = %s, want none (JSON bodies are skipped)", got)
	}
}
