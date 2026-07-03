package main

import (
	"bytes"
	"testing"
)

func TestVersionCmd(t *testing.T) {
	info := buildInfo{version: "1.2.3", commit: "abc1234", date: "2026-07-02T00:00:00Z"}

	var buf bytes.Buffer
	root := newRootCmd(info)
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"version"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute(version) returned error: %v", err)
	}

	want := "hclkit 1.2.3 (abc1234, 2026-07-02T00:00:00Z)\n"
	if got := buf.String(); got != want {
		t.Errorf("Execute(version) output = %q, want %q", got, want)
	}
}

func TestVersionCmdRejectsArgs(t *testing.T) {
	root := newRootCmd(buildInfo{})
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"version", "extra"})

	if err := root.Execute(); err == nil {
		t.Error("Execute(version extra) = nil error, want non-nil")
	}
}
