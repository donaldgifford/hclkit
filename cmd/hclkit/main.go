// Command hclkit is a validator binary for HCL configuration files.
// It fronts the hclkit library with fmt, validate, and lint
// subcommands so developers and CI get a consistent surface.
package main

import (
	"log/slog"
	"os"
)

// Injected at build time via -ldflags (see justfile / .goreleaser.yml).
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, nil)))

	info := buildInfo{version: version, commit: commit, date: date}
	if err := newRootCmd(info).Execute(); err != nil {
		os.Exit(1)
	}
}
