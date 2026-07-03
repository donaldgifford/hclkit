package hclkit

import (
	"fmt"
	"io"

	"github.com/hashicorp/hcl/v2"
)

// Diagnostics wraps hcl.Diagnostics with the parsed-file map needed
// to render source snippets. The zero value is empty and error-free.
//
// Diagnostics satisfies the error interface via the embedded
// hcl.Diagnostics, so discarding one from a Load* call trips errcheck
// like any other unchecked error.
//
//nolint:errname // mirrors upstream hcl.Diagnostics; error is a secondary role
type Diagnostics struct {
	hcl.Diagnostics

	files map[string]*hcl.File
}

// Compliance pins: keep the error contract even if the embed is ever
// refactored (losing it silently disables errcheck coverage for
// callers), and keep WriteTo conforming to io.WriterTo.
var (
	_ error       = Diagnostics{}
	_ io.WriterTo = Diagnostics{}
)

func newDiagnostics(diags hcl.Diagnostics, files map[string]*hcl.File) Diagnostics {
	return Diagnostics{Diagnostics: diags, files: files}
}

// WriteTo renders every diagnostic to w: a stable machine-parseable
// GCC-style prefix line (file:line:col: severity: summary) followed
// by hcl's standard human-readable rendering with source snippets.
// Diagnostics synthesized without a source range omit the position
// from the prefix.
//
// The signature implements io.WriterTo (DESIGN-0001 sketched a plain
// error return; govet's stdmethods check rightly demands the stdlib
// shape for this name).
func (d Diagnostics) WriteTo(w io.Writer) (int64, error) {
	if len(d.Diagnostics) == 0 {
		return 0, nil
	}

	cw := &countingWriter{w: w}
	tw := hcl.NewDiagnosticTextWriter(cw, d.files, 0, false)
	for _, diag := range d.Diagnostics {
		if _, err := io.WriteString(cw, prefixLine(diag)); err != nil {
			return cw.n, fmt.Errorf("writing diagnostic prefix: %w", err)
		}
		if err := tw.WriteDiagnostic(diag); err != nil {
			return cw.n, fmt.Errorf("writing diagnostic body: %w", err)
		}
	}
	return cw.n, nil
}

// countingWriter tracks bytes written so WriteTo can honor the
// io.WriterTo contract.
type countingWriter struct {
	w io.Writer
	n int64
}

func (c *countingWriter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	c.n += int64(n)
	return n, err
}

// prefixLine formats the machine-parseable line for one diagnostic.
func prefixLine(diag *hcl.Diagnostic) string {
	sev := "error"
	if diag.Severity == hcl.DiagWarning {
		sev = "warning"
	}
	if diag.Subject == nil {
		return fmt.Sprintf("%s: %s\n", sev, diag.Summary)
	}
	s := diag.Subject
	return fmt.Sprintf("%s:%d:%d: %s: %s\n",
		s.Filename, s.Start.Line, s.Start.Column, sev, diag.Summary)
}
