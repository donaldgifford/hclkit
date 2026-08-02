package partial

import (
	"strings"
	"testing"

	"github.com/hashicorp/hcl/v2"
)

// walkSchema lists locals first: kind order in schema.Blocks is
// visitation order regardless of source order.
var walkSchema = &hcl.BodySchema{
	Blocks: []hcl.BlockHeaderSchema{
		{Type: "locals"},
		{Type: "rule", LabelNames: []string{"name"}},
	},
}

func TestWalkLocalsFirstOrdering(t *testing.T) {
	// Source order deliberately interleaves rule and locals.
	body := parseBody(t, `
rule "a" {}

locals {}

rule "b" {}

locals {}
`)

	var visited []string
	diags := Walk(body, walkSchema, func(block *hcl.Block) hcl.Diagnostics {
		name := block.Type
		if len(block.Labels) > 0 {
			name += ":" + block.Labels[0]
		}
		visited = append(visited, name)
		return nil
	})
	if diags.HasErrors() {
		t.Fatalf("Walk diags = %s, want none", diags)
	}

	want := []string{"locals", "locals", "rule:a", "rule:b"}
	if len(visited) != len(want) {
		t.Fatalf("visited = %v, want %v", visited, want)
	}
	for i := range want {
		if visited[i] != want[i] {
			t.Fatalf("visited = %v, want %v (kinds in schema order, blocks in source order)", visited, want)
		}
	}
}

func TestWalkStrictContent(t *testing.T) {
	body := parseBody(t, `
locals {}

unexpected {}
`)

	called := false
	diags := Walk(body, walkSchema, func(*hcl.Block) hcl.Diagnostics {
		called = true
		return nil
	})
	if !diags.HasErrors() {
		t.Fatal("Walk diags = none, want strict-content error for unknown block")
	}
	if called {
		t.Error("fn was called despite schema errors; strict walk must visit nothing")
	}
}

func TestWalkCollectsCallbackDiagnostics(t *testing.T) {
	body := parseBody(t, `
rule "a" {}

rule "b" {}
`)

	var visited int
	diags := Walk(body, walkSchema, func(block *hcl.Block) hcl.Diagnostics {
		visited++
		defRange := block.DefRange
		return hcl.Diagnostics{{
			Severity: hcl.DiagError,
			Summary:  "Callback rejects " + block.Labels[0],
			Subject:  &defRange,
		}}
	})

	if visited != 2 {
		t.Errorf("visited = %d, want 2 (walk continues past callback errors)", visited)
	}
	if len(diags) != 2 {
		t.Errorf("diags len = %d, want 2 collected", len(diags))
	}
}

func TestWalkInvalidArguments(t *testing.T) {
	body := parseBody(t, "locals {}\n")

	for name, call := range map[string]func() hcl.Diagnostics{
		"nil body":   func() hcl.Diagnostics { return Walk(nil, walkSchema, func(*hcl.Block) hcl.Diagnostics { return nil }) },
		"nil schema": func() hcl.Diagnostics { return Walk(body, nil, func(*hcl.Block) hcl.Diagnostics { return nil }) },
		"nil fn":     func() hcl.Diagnostics { return Walk(body, walkSchema, nil) },
	} {
		t.Run(name, func(t *testing.T) {
			diags := call()
			if !diags.HasErrors() || !strings.Contains(diags.Error(), "Invalid walk arguments") {
				t.Errorf("Walk diags = %q, want invalid-arguments error", diags.Error())
			}
		})
	}
}
