package helmdoc

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func newRootCommand() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:     "helmdoc",
		Short:   "Helm Chart Doctor — production-readiness linter for Helm charts",
		Version: version,
		Long: `helmdoc scans Helm charts for production-readiness issues, scores them A-F,
and generates auto-fix patches.

The linter Helm should have shipped.`,
	}

	rootCmd.AddCommand(newVersionCommand())
	rootCmd.AddCommand(newScanCommand())
	rootCmd.AddCommand(newFixCommand())

	return rootCmd
}

func newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("helmdoc %s (commit: %s, built: %s)\n", version, commit, date)
		},
	}
}

func Execute() error {
	return newRootCommand().Execute()
}
