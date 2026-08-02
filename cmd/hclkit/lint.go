package main

import (
	"errors"

	"github.com/hashicorp/hcl/v2"
	"github.com/spf13/cobra"

	"github.com/donaldgifford/hclkit/internal/lintschema"
	"github.com/donaldgifford/hclkit/internal/parser"
	"github.com/donaldgifford/hclkit/pkg/hclkit"
)

// errLintFailed signals lint findings after the diagnostics have
// already been rendered to stderr.
var errLintFailed = errors.New("lint failed")

func newLintCmd() *cobra.Command {
	var schemaPath string

	cmd := &cobra.Command{
		Use:   "lint --schema=schema.hcl [files...]",
		Short: "Lint HCL files against a schema",
		Long: `Checks each file against the schema's declared rules and renders
any findings in hclkit's standard diagnostic format. Exits non-zero
on findings; produces no output when everything passes.

The schema is itself HCL, with four top-level kinds (the attribute
names may still evolve before v1.0):

  block "doctype" {        # permitted top-level block kind
    labels = 1             # exact label count (default 0)
  }

  attribute "id_prefix" {  # attribute rule for one block kind
    block    = "doctype"   # kind the rule applies to
    required = true        # default false
    type     = string      # typeexpr; literal values must convert
  }

  reference {              # cross-block reference resolution
    verb        = "decides" # attribute holding the reference(s)
    target_kind = "doctype" # kind whose labels are referenced
  }

  unique {                 # per-kind attribute uniqueness
    block_kind = "doctype"
    attribute  = "id_prefix"
  }

Block kinds are only enforced when the schema declares at least one
block rule. Lint evaluates without variables or functions in scope,
so only literal attribute values are type-checked.`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			schema, schemaDiags := lintschema.Load(schemaPath)
			if schemaDiags.HasErrors() {
				renderDiags(cmd.ErrOrStderr(), schemaDiags)
				return errLintFailed
			}

			p := parser.New()
			var diags hcl.Diagnostics
			bodies := make([]hcl.Body, 0, len(args))
			for _, path := range args {
				file, fileDiags := p.ParseFile(path)
				diags = diags.Extend(fileDiags)
				if file != nil && !fileDiags.HasErrors() {
					bodies = append(bodies, file.Body)
				}
			}

			if !diags.HasErrors() {
				for _, v := range schema.Validators() {
					diags = diags.Extend(v.Validate(bodies, nil))
				}
			}

			all := hclkit.NewDiagnostics(diags, p.Files())
			renderDiags(cmd.ErrOrStderr(), all)
			if all.HasErrors() {
				return errLintFailed
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&schemaPath, "schema", "", "path to the lint schema file (required)")
	_ = cmd.MarkFlagRequired("schema")
	return cmd
}
