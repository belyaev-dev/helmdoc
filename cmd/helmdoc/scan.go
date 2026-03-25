package helmdoc

import (
	"fmt"
	"io"
	"strings"

	"github.com/belyaev-dev/helmdoc/internal/report"
	_ "github.com/belyaev-dev/helmdoc/internal/rules/all"
	"github.com/belyaev-dev/helmdoc/pkg/models"
	"github.com/spf13/cobra"
)

func newScanCommand() *cobra.Command {
	var outputFormat string
	var minScore string
	var configPath string

	cmd := &cobra.Command{
		Use:   "scan [CHART_PATH]",
		Short: "Scan a Helm chart for production-readiness issues",
		Long: `Analyzes a Helm chart directory or packaged .tgz against production-readiness
rules across 10 categories: storage, resources, security, health, images,
availability, network, ingress, scaling, and config.

Produces a scored report (A-F) with per-category breakdown.`,
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runScan(cmd.OutOrStdout(), args[0], outputFormat, minScore, configPath)
		},
	}

	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format: text or json")
	cmd.Flags().StringVar(&minScore, "min-score", "", "Minimum passing grade (A-F). Exit 1 if below.")
	cmd.Flags().StringVar(&configPath, "config", "", "Optional path to a .helmdoc.yaml policy file")

	return cmd
}

func runScan(w io.Writer, chartPath, outputFormat, minScore, configPath string) error {
	minimumGrade, err := parseMinimumGrade(minScore)
	if err != nil {
		return err
	}

	analysis, err := analyzeChart(chartPath, configPath)
	if err != nil {
		return err
	}

	if err := emitScanReport(w, analysis.Report, outputFormat); err != nil {
		return err
	}

	if minimumGrade == "" {
		return nil
	}
	if analysis.Report.OverallGrade.Rank() < minimumGrade.Rank() {
		return fmt.Errorf("min-score: overall grade %s is below required %s", analysis.Report.OverallGrade, minimumGrade)
	}

	return nil
}

func parseMinimumGrade(raw string) (models.Grade, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", nil
	}

	grade := models.Grade(strings.ToUpper(trimmed))
	switch grade {
	case models.GradeA, models.GradeB, models.GradeC, models.GradeD, models.GradeF:
		return grade, nil
	default:
		return "", fmt.Errorf("min-score: invalid grade %q (want one of A, B, C, D, F)", raw)
	}
}

func emitScanReport(w io.Writer, scanReport models.Report, outputFormat string) error {
	switch strings.ToLower(strings.TrimSpace(outputFormat)) {
	case "", "text":
		if err := report.RenderText(w, &scanReport); err != nil {
			return fmt.Errorf("report: %w", err)
		}
		return nil
	case "json":
		payload, err := report.RenderJSON(scanReport)
		if err != nil {
			return fmt.Errorf("report: %w", err)
		}
		if _, err := fmt.Fprintln(w, string(payload)); err != nil {
			return fmt.Errorf("report: %w", err)
		}
		return nil
	default:
		return fmt.Errorf("report: unsupported output format %q (want text or json)", outputFormat)
	}
}
