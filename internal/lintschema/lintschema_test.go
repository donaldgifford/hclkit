package lintschema

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
)

const schemaSrc = `
block "doctype" {
  labels = 1
}

attribute "id_prefix" {
  block    = "doctype"
  required = true
  type     = string
}

reference {
  verb        = "decides"
  target_kind = "doctype"
}

unique {
  block_kind = "doctype"
  attribute  = "id_prefix"
}
`

func loadSchema(t *testing.T, src string) *Schema {
	t.Helper()
	path := filepath.Join(t.TempDir(), "schema.hcl")
	if err := os.WriteFile(path, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	schema, diags := Load(path)
	if diags.HasErrors() {
		t.Fatalf("Load: %s", diags)
	}
	return schema
}

func bodyFor(t *testing.T, src string) hcl.Body {
	t.Helper()
	file, diags := hclparse.NewParser().ParseHCL([]byte(src), "t.hcl")
	if diags.HasErrors() {
		t.Fatalf("parse: %s", diags)
	}
	return file.Body
}

func TestLoadAndValidators(t *testing.T) {
	schema := loadSchema(t, schemaSrc)
	if len(schema.Blocks) != 1 || len(schema.Attributes) != 1 ||
		len(schema.References) != 1 || len(schema.Uniques) != 1 {
		t.Fatalf("schema = %+v, want one rule of each kind", schema)
	}
	if got := len(schema.Validators()); got != 3 {
		t.Errorf("Validators() len = %d, want 3 (structure + ref + unique)", got)
	}
}

func TestStructureValidator(t *testing.T) {
	schema := loadSchema(t, schemaSrc)
	structure := schema.Validators()[0]

	tests := []struct {
		name   string
		src    string
		wantIn string // empty = no diagnostics expected
	}{
		{name: "clean", src: "doctype \"rfc\" {\n  id_prefix = \"RFC\"\n}\n"},
		{name: "unknown kind", src: "mystery {}\n", wantIn: "Unknown block kind"},
		{name: "label count", src: "doctype \"a\" \"b\" {\n  id_prefix = \"X\"\n}\n", wantIn: "Wrong label count"},
		{name: "missing required", src: "doctype \"rfc\" {}\n", wantIn: "Missing required attribute"},
		{name: "bad type", src: "doctype \"rfc\" {\n  id_prefix = [1]\n}\n", wantIn: "Invalid attribute type"},
		{name: "non-literal skipped", src: "doctype \"rfc\" {\n  id_prefix = var.x\n}\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diags := structure.Validate([]hcl.Body{bodyFor(t, tt.src)}, nil)
			if tt.wantIn == "" {
				if diags.HasErrors() {
					t.Fatalf("diags = %s, want none", diags)
				}
				return
			}
			if !diags.HasErrors() || !strings.Contains(diags.Error(), tt.wantIn) {
				t.Errorf("diags = %q, want %q", diags.Error(), tt.wantIn)
			}
		})
	}
}

func TestStructureValidatorNoBlockRules(t *testing.T) {
	schema := loadSchema(t, `
attribute "id" {
  block    = "rule"
  required = true
}
`)
	structure := schema.Validators()[0]

	// With no block rules declared, unknown kinds pass — only the
	// attribute rules apply.
	diags := structure.Validate([]hcl.Body{bodyFor(t, "anything {}\nrule \"a\" {}\n")}, nil)
	if len(diags) != 1 || !strings.Contains(diags.Error(), "Missing required attribute") {
		t.Errorf("diags = %q, want only the missing-attribute finding", diags.Error())
	}
}

func TestLoadRejectsMalformedSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "schema.hcl")
	if err := os.WriteFile(path, []byte("reference {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, diags := Load(path); !diags.HasErrors() {
		t.Fatal("Load(reference without verb) diags = none, want missing-attribute errors")
	}
}
