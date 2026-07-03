package main

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/donaldgifford/hclkit/pkg/hclkit"
)

// buildInfo carries the link-time version identity from main's
// ldflags-injected vars into the command constructors.
type buildInfo struct {
	version string
	commit  string
	date    string
}

func newRootCmd(info buildInfo) *cobra.Command {
	root := &cobra.Command{
		Use:   "hclkit",
		Short: "Format, validate, and lint HCL configuration files",
		Long: `hclkit gives developers and CI a consistent surface for the
HCL plumbing shared across tools: formatting via hclwrite, parse-only
validation, and schema-driven linting.`,
		SilenceUsage: true,
	}

	root.AddCommand(
		newFmtCmd(),
		newValidateCmd(),
		newVersionCmd(info),
	)

	return root
}

// fprintln writes one line best-effort. CLI output failures don't
// change command outcomes beyond the exit code RunE errors already
// carry.
func fprintln(w io.Writer, a ...any) {
	_, _ = fmt.Fprintln(w, a...) //nolint:errcheck // best-effort CLI output
}

// renderDiags writes diagnostics best-effort; a rendering failure
// must not mask the diagnostics-driven exit code.
func renderDiags(w io.Writer, diags hclkit.Diagnostics) {
	if len(diags.Diagnostics) == 0 {
		return
	}
	_, _ = diags.WriteTo(w) //nolint:errcheck // best-effort CLI output
}
