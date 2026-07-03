// Command hclkit is a validator binary for HCL configuration files.
// It fronts the hclkit library with fmt, validate, and lint
// subcommands so developers and CI get a consistent surface.
package main

import (
	"fmt"
	"log/slog"
	"os"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, nil)))
	if err := run(); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run() error {
	fmt.Printf("hclkit %s (%s, %s)\n", version, commit, date)
	return nil
}
