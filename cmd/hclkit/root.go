package main

import (
	"github.com/spf13/cobra"
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

	root.AddCommand(newVersionCmd(info))

	return root
}
