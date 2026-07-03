package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newVersionCmd(info buildInfo) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version, commit, and build date",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := fmt.Fprintf(cmd.OutOrStdout(), "hclkit %s (%s, %s)\n",
				info.version, info.commit, info.date)
			return err
		},
	}
}
