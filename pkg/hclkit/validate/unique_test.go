package validate

import (
	"strings"
	"testing"

	"github.com/hashicorp/hcl/v2"
)

func TestUniqueValidatorAllUnique(t *testing.T) {
	body := parseBody(t, "a.hcl", `
doctype "rfc" {
  id_prefix = "RFC"
}

doctype "adr" {
  id_prefix = "ADR"
}
`)
	diags := NewUniqueValidator("doctype", "id_prefix").Validate([]hcl.Body{body}, nil)
	if diags.HasErrors() {
		t.Fatalf("Validate diags = %s, want none", diags)
	}
}

func TestUniqueValidatorDuplicateAcrossBodies(t *testing.T) {
	first := parseBody(t, "01-a.hcl", `
doctype "rfc" {
  id_prefix = "RFC"
}
`)
	second := parseBody(t, "02-b.hcl", `
doctype "memo" {
  id_prefix = "RFC"
}
`)

	diags := NewUniqueValidator("doctype", "id_prefix").Validate([]hcl.Body{first, second}, nil)
	if len(diags) != 1 {
		t.Fatalf("diags = %d, want 1: %s", len(diags), diags)
	}
	d := diags[0]
	if d.Summary != "Duplicate doctype id_prefix" {
		t.Errorf("Summary = %q", d.Summary)
	}
	if d.Subject == nil || d.Subject.Filename != "02-b.hcl" || d.Subject.Start.Line != 3 {
		t.Errorf("Subject = %v, want the LATER occurrence in 02-b.hcl line 3", d.Subject)
	}
	if !strings.Contains(d.Detail, "01-a.hcl") {
		t.Errorf("Detail = %q, want the first occurrence's range named", d.Detail)
	}
}

func TestUniqueValidatorEdgeCases(t *testing.T) {
	tests := []struct {
		name      string
		src       string
		wantDiags int
		wantIn    string
	}{
		{name: "missing attribute skipped", src: "rule \"a\" {}\nrule \"b\" {}\n"},
		{name: "numeric ids convert", src: "rule \"a\" {\n  id = 1\n}\nrule \"b\" {\n  id = 1\n}\n", wantDiags: 1, wantIn: "Duplicate rule id"},
		{name: "unconvertible value", src: "rule \"a\" {\n  id = [1]\n}\n", wantDiags: 1, wantIn: "Invalid attribute value"},
		{name: "eval error skipped", src: "rule \"a\" {\n  id = nosuch.attr\n}\n"},
		{name: "null skipped", src: "rule \"a\" {\n  id = null\n}\nrule \"b\" {\n  id = null\n}\n"},
		{name: "other kinds ignored", src: "check \"a\" {\n  id = \"x\"\n}\ncheck \"b\" {\n  id = \"x\"\n}\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := parseBody(t, "t.hcl", tt.src)
			diags := NewUniqueValidator("rule", "id").Validate([]hcl.Body{body}, nil)
			if len(diags) != tt.wantDiags {
				t.Fatalf("diags = %d (%s), want %d", len(diags), diags, tt.wantDiags)
			}
			if tt.wantDiags > 0 && !strings.Contains(diags.Error(), tt.wantIn) {
				t.Errorf("diags = %q, want %q", diags.Error(), tt.wantIn)
			}
		})
	}
}

func TestUniqueValidatorNestedBlocks(t *testing.T) {
	body := parseBody(t, "t.hcl", `
group {
  rule "a" {
    id = "x"
  }
}

rule "b" {
  id = "x"
}
`)
	diags := NewUniqueValidator("rule", "id").Validate([]hcl.Body{body}, nil)
	if len(diags) != 1 {
		t.Fatalf("diags = %d, want 1 (nested blocks participate)", len(diags))
	}
}
