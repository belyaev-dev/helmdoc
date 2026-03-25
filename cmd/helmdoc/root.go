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

var rootCmd = &cobra.Command{
	Use:     "helmdoc",
	Short:   "Helm Chart Doctor — production-readiness linter for Helm charts",
	Version: version,
	Long: `helmdoc scans Helm charts for production-readiness issues, scores them A-F,
and generates auto-fix patches.

The linter Helm should have shipped.`,
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("helmdoc %s (commit: %s, built: %s)\n", version, commit, date)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(scanCmd)
}

func Execute() error {
	return rootCmd.Execute()
}
