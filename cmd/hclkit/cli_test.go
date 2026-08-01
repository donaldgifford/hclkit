package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/donaldgifford/hclkit/internal/testutil"
)

func runCLI(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()

	root := newRootCmd(buildInfo{version: "test", commit: "test", date: "test"})
	var out, errBuf bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errBuf)
	root.SetArgs(args)

	err = root.Execute()
	return out.String(), errBuf.String(), err
}

func TestValidateValid(t *testing.T) {
	stdout, stderr, err := runCLI(t, "validate", filepath.Join("testdata", "valid.hcl"))
	if err != nil {
		t.Fatalf("validate(valid.hcl) error: %v (stderr: %s)", err, stderr)
	}
	if stdout != "" || stderr != "" {
		t.Errorf("validate(valid.hcl) output = %q / %q, want silence", stdout, stderr)
	}
}

func TestValidateInvalidGolden(t *testing.T) {
	stdout, stderr, err := runCLI(t, "validate", filepath.Join("testdata", "invalid.hcl"))
	if err == nil {
		t.Fatal("validate(invalid.hcl) error = nil, want non-nil for exit code")
	}
	if stdout != "" {
		t.Errorf("validate(invalid.hcl) stdout = %q, want empty (diagnostics go to stderr)", stdout)
	}
	testutil.Golden(t, "validate_invalid_stderr", []byte(stderr))
}

func TestFmtCheckGolden(t *testing.T) {
	stdout, stderr, err := runCLI(t, "fmt", "--check",
		filepath.Join("testdata", "valid.hcl"),
		filepath.Join("testdata", "unformatted.hcl"),
	)
	if err == nil {
		t.Fatal("fmt --check (unformatted present) error = nil, want non-nil for exit code")
	}
	testutil.Golden(t, "fmt_check_stdout", []byte(stdout))
	testutil.Golden(t, "fmt_check_stderr", []byte(stderr))
}

func TestFmtCheckCleanIsSilent(t *testing.T) {
	stdout, stderr, err := runCLI(t, "fmt", "--check", filepath.Join("testdata", "valid.hcl"))
	if err != nil {
		t.Fatalf("fmt --check (canonical) error: %v (stderr: %s)", err, stderr)
	}
	if stdout != "" || stderr != "" {
		t.Errorf("fmt --check (canonical) output = %q / %q, want silence", stdout, stderr)
	}
}

func TestFmtRewritesInPlace(t *testing.T) {
	src := testutil.ReadFixture(t, "unformatted.hcl")
	path := filepath.Join(t.TempDir(), "a.hcl")
	if err := os.WriteFile(path, src, 0o600); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, err := runCLI(t, "fmt", path)
	if err != nil {
		t.Fatalf("fmt error: %v (stderr: %s)", err, stderr)
	}
	if stdout != path+"\n" {
		t.Errorf("fmt stdout = %q, want %q", stdout, path+"\n")
	}

	rewritten, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(rewritten, src) {
		t.Error("fmt left the file unformatted")
	}

	// A second run must be a no-op: canonical output is stable.
	stdout, _, err = runCLI(t, "fmt", path)
	if err != nil || stdout != "" {
		t.Errorf("second fmt run = (%q, %v), want no-op", stdout, err)
	}
}

func TestFmtInvalidSource(t *testing.T) {
	_, stderr, err := runCLI(t, "fmt", "--check", filepath.Join("testdata", "invalid.hcl"))
	if err == nil {
		t.Fatal("fmt --check (invalid.hcl) error = nil, want non-nil")
	}
	if stderr == "" {
		t.Error("fmt --check (invalid.hcl) stderr empty, want diagnostics")
	}
}
