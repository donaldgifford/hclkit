package hclkit_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/hashicorp/hcl/v2"

	"github.com/donaldgifford/hclkit/pkg/hclkit"
)

func TestDiagnosticsZeroValue(t *testing.T) {
	var d hclkit.Diagnostics

	if d.HasErrors() {
		t.Error("zero Diagnostics HasErrors() = true, want false")
	}
	if got, want := d.Error(), "no diagnostics"; got != want {
		t.Errorf("zero Diagnostics Error() = %q, want %q", got, want)
	}

	var buf bytes.Buffer
	n, err := d.WriteTo(&buf)
	if err != nil {
		t.Fatalf("zero Diagnostics WriteTo() error: %v", err)
	}
	if n != 0 || buf.Len() != 0 {
		t.Errorf("zero Diagnostics WriteTo() = %d bytes, wrote %q; want 0 and empty", n, buf.String())
	}
}

func TestWriteToPrefixLine(t *testing.T) {
	tests := []struct {
		name string
		diag *hcl.Diagnostic
		want string
	}{
		{
			name: "error with subject",
			diag: &hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  "Unsupported argument",
				Subject:  &hcl.Range{Filename: "app.hcl", Start: hcl.Pos{Line: 3, Column: 5}},
			},
			want: "app.hcl:3:5: error: Unsupported argument",
		},
		{
			name: "warning with subject",
			diag: &hcl.Diagnostic{
				Severity: hcl.DiagWarning,
				Summary:  "Deprecated attribute",
				Subject:  &hcl.Range{Filename: "app.hcl", Start: hcl.Pos{Line: 9, Column: 1}},
			},
			want: "app.hcl:9:1: warning: Deprecated attribute",
		},
		{
			name: "nil subject omits position",
			diag: &hcl.Diagnostic{Severity: hcl.DiagError, Summary: "Synthesized failure"},
			want: "error: Synthesized failure",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := hclkit.Diagnostics{Diagnostics: hcl.Diagnostics{tt.diag}}

			var buf bytes.Buffer
			n, err := d.WriteTo(&buf)
			if err != nil {
				t.Fatalf("WriteTo() error: %v", err)
			}
			if n != int64(buf.Len()) {
				t.Errorf("WriteTo() = %d bytes, buffer holds %d", n, buf.Len())
			}

			first, _, _ := strings.Cut(buf.String(), "\n")
			if first != tt.want {
				t.Errorf("WriteTo() first line = %q, want %q", first, tt.want)
			}
		})
	}
}

type errWriter struct{}

func (errWriter) Write([]byte) (int, error) { return 0, errors.New("sink failed") }

func TestWriteToPropagatesWriterError(t *testing.T) {
	d := hclkit.Diagnostics{
		Diagnostics: hcl.Diagnostics{{Severity: hcl.DiagError, Summary: "boom"}},
	}
	if _, err := d.WriteTo(errWriter{}); err == nil {
		t.Error("WriteTo(errWriter) = nil error, want non-nil")
	}
}
