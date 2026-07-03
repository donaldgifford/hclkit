package main

import (
	"errors"

	"github.com/hashicorp/hcl/v2"
	"github.com/spf13/cobra"

	"github.com/donaldgifford/hclkit/internal/parser"
	"github.com/donaldgifford/hclkit/pkg/hclkit"
)

// errValidationFailed signals parse errors after the diagnostics have
// already been rendered to stderr.
var errValidationFailed = errors.New("validation failed")

func newValidateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "validate [files...]",
		Short: "Parse HCL files and report diagnostics",
		Long: `Parses each file (native or JSON syntax, by extension) and
renders any diagnostics in hclkit's standard format. Exits non-zero
if any file fails to parse; produces no output when everything is
valid.`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p := parser.New()

			var diags hcl.Diagnostics
			for _, path := range args {
				_, fileDiags := p.ParseFile(path)
				diags = diags.Extend(fileDiags)
			}

			all := hclkit.NewDiagnostics(diags, p.Files())
			renderDiags(cmd.ErrOrStderr(), all)
			if all.HasErrors() {
				return errValidationFailed
			}
			return nil
		},
	}
}
