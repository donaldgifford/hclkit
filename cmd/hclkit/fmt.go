package main

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/donaldgifford/hclkit/internal/format"
)

// errFormattingFailed signals unreadable or unparseable input after
// the diagnostics have already been rendered to stderr.
var errFormattingFailed = errors.New("formatting failed")

func newFmtCmd() *cobra.Command {
	var check bool

	cmd := &cobra.Command{
		Use:   "fmt [files...]",
		Short: "Format HCL files to canonical style",
		Long: `Rewrites the given HCL files to hclwrite's canonical style in
place, printing the path of each file it changes. With --check no
file is modified: paths needing formatting are printed and the exit
code is non-zero, for CI gates.`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			changed, diags := format.Files(args, check)
			renderDiags(cmd.ErrOrStderr(), diags)
			for _, path := range changed {
				fprintln(cmd.OutOrStdout(), path)
			}

			if diags.HasErrors() {
				return errFormattingFailed
			}
			if check && len(changed) > 0 {
				return fmt.Errorf("%d file(s) not formatted", len(changed))
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&check, "check", false,
		"report files needing formatting and exit non-zero instead of rewriting")
	return cmd
}
