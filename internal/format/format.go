// Package format implements the engine behind `hclkit fmt`:
// canonical hclwrite formatting with an optional check-only mode.
package format

import (
	"bytes"
	"fmt"
	"os"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclwrite"

	"github.com/donaldgifford/hclkit/internal/parser"
	"github.com/donaldgifford/hclkit/pkg/hclkit"
)

// Files formats every path to canonical hclwrite style, rewriting
// files in place. With check set it only reports. It returns the
// paths that were (or would be) changed, in input order, plus any
// diagnostics; files that fail to read or parse are reported and
// skipped, never rewritten.
func Files(paths []string, check bool) ([]string, hclkit.Diagnostics) {
	p := parser.New()

	var changed []string
	var diags hcl.Diagnostics
	for _, path := range paths {
		src, err := os.ReadFile(path)
		if err != nil {
			diags = diags.Append(&hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  "Failed to read file",
				Detail:   fmt.Sprintf("Cannot read %s: %s.", path, err),
			})
			continue
		}

		// Validate before formatting: hclwrite.Format is best-effort
		// on broken input, and rewriting a file that doesn't parse
		// would launder the syntax error into a formatted-looking one.
		if _, parseDiags := p.ParseBytes(path, src); parseDiags.HasErrors() {
			diags = diags.Extend(parseDiags)
			continue
		}

		formatted := hclwrite.Format(src)
		if bytes.Equal(src, formatted) {
			continue
		}
		changed = append(changed, path)

		if check {
			continue
		}
		if err := writeBack(path, formatted); err != nil {
			diags = diags.Append(&hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  "Failed to write file",
				Detail:   fmt.Sprintf("Cannot rewrite %s: %s.", path, err),
			})
		}
	}
	return changed, hclkit.NewDiagnostics(diags, p.Files())
}

// writeBack rewrites path preserving its existing permissions.
func writeBack(path string, content []byte) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	//nolint:gosec // G703: rewriting the user-named file is fmt's purpose
	return os.WriteFile(path, content, info.Mode().Perm())
}
