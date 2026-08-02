package main

import (
	"path/filepath"
	"testing"

	"github.com/donaldgifford/hclkit/internal/testutil"
)

func TestLintClean(t *testing.T) {
	stdout, stderr, err := runCLI(t, "lint",
		"--schema", filepath.Join("testdata", "lint_schema.hcl"),
		filepath.Join("testdata", "lint_clean.hcl"),
	)
	if err != nil {
		t.Fatalf("lint(clean) error: %v (stderr: %s)", err, stderr)
	}
	if stdout != "" || stderr != "" {
		t.Errorf("lint(clean) output = %q / %q, want silence", stdout, stderr)
	}
}

func TestLintFindingsGolden(t *testing.T) {
	stdout, stderr, err := runCLI(t, "lint",
		"--schema", filepath.Join("testdata", "lint_schema.hcl"),
		filepath.Join("testdata", "lint_bad.hcl"),
	)
	if err == nil {
		t.Fatal("lint(bad) error = nil, want non-nil for exit code")
	}
	if stdout != "" {
		t.Errorf("lint(bad) stdout = %q, want empty (diagnostics go to stderr)", stdout)
	}
	testutil.Golden(t, "lint_bad_stderr", []byte(stderr))
}

func TestLintMissingSchemaFlag(t *testing.T) {
	_, _, err := runCLI(t, "lint", filepath.Join("testdata", "lint_clean.hcl"))
	if err == nil {
		t.Fatal("lint without --schema error = nil, want required-flag error")
	}
}

func TestLintBrokenSchema(t *testing.T) {
	_, stderr, err := runCLI(t, "lint",
		"--schema", filepath.Join("testdata", "invalid.hcl"),
		filepath.Join("testdata", "lint_clean.hcl"),
	)
	if err == nil {
		t.Fatal("lint(broken schema) error = nil, want non-nil")
	}
	if stderr == "" {
		t.Error("lint(broken schema) stderr empty, want schema diagnostics")
	}
}

func TestLintUnparseableTarget(t *testing.T) {
	_, stderr, err := runCLI(t, "lint",
		"--schema", filepath.Join("testdata", "lint_schema.hcl"),
		filepath.Join("testdata", "invalid.hcl"),
	)
	if err == nil {
		t.Fatal("lint(invalid target) error = nil, want non-nil")
	}
	if stderr == "" {
		t.Error("lint(invalid target) stderr empty, want parse diagnostics")
	}
}
