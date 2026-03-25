package helmdoc

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	outputFormat string
	minScore     string
)

var scanCmd = &cobra.Command{
	Use:   "scan [CHART_PATH]",
	Short: "Scan a Helm chart for production-readiness issues",
	Long: `Analyzes a Helm chart directory or packaged .tgz against production-readiness
rules across 10 categories: storage, resources, security, health, images,
availability, network, ingress, scaling, and config.

Produces a scored report (A-F) with per-category breakdown.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		chartPath := args[0]
		fmt.Printf("Scanning chart: %s\n", chartPath)
		fmt.Println("(not yet implemented)")
		return nil
	},
}

func init() {
	scanCmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format: text, json, sarif")
	scanCmd.Flags().StringVar(&minScore, "min-score", "", "Minimum passing grade (A-F). Exit 1 if below.")
}
