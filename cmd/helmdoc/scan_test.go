package helmdoc

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/belyaev-dev/helmdoc/internal/testutil/realcharts"
	"github.com/belyaev-dev/helmdoc/pkg/models"
)

func TestScanCommandEmitsRealTextReport(t *testing.T) {
	fixture := realcharts.FixtureByID(t, "nginx-ingress")
	chartPath := realcharts.FixturePath(t, fixture)

	t.Run("renders_real_text_report_markers", func(t *testing.T) {
		stdout, stderr, err := executeScanCommand(t, chartPath)
		if err != nil {
			t.Fatalf("executeScanCommand(%q) error = %v\nstderr:\n%s\nstdout:\n%s", chartPath, err, stderr, stdout)
		}

		for _, want := range []string{
			"HelmDoc scan report",
			"Chart: ingress-nginx@4.15.1",
			"Overall: B",
			"Score: 84.5/100",
			"Total findings: 13",
			"Security findings (1):",
		} {
			if !strings.Contains(stdout, want) {
				t.Fatalf("scan output missing %q\n\nstdout:\n%s", want, stdout)
			}
		}
		for _, unwanted := range []string{"(not yet implemented)", "Total findings: 0"} {
			if strings.Contains(stdout, unwanted) {
				t.Fatalf("scan output unexpectedly contained %q\n\nstdout:\n%s", unwanted, stdout)
			}
		}
		if stderr != "" {
			t.Fatalf("stderr = %q, want empty", stderr)
		}
	})

	t.Run("missing_chart_reports_load_phase", func(t *testing.T) {
		missingPath := filepath.Join(t.TempDir(), "missing-chart")

		stdout, stderr, err := executeScanCommand(t, missingPath)
		if err == nil {
			t.Fatalf("executeScanCommand(%q) error = nil, want non-nil", missingPath)
		}
		if !strings.Contains(err.Error(), "load:") {
			t.Fatalf("error %q does not identify load phase", err)
		}
		if stdout != "" {
			t.Fatalf("stdout = %q, want empty", stdout)
		}
		if !strings.Contains(stderr, "load:") {
			t.Fatalf("stderr %q does not include load phase error", stderr)
		}
	})
}

func TestScanCommandEmitsJSONReportAndHonorsMinScore(t *testing.T) {
	fixture := realcharts.FixtureByID(t, "nginx-ingress")
	chartPath := realcharts.FixturePath(t, fixture)

	t.Run("renders_machine_parseable_json_and_accepts_matching_threshold", func(t *testing.T) {
		stdout, stderr, err := executeScanCommand(t, "--output", "json", "--min-score", "B", chartPath)
		if err != nil {
			t.Fatalf("executeScanCommand(json) error = %v\nstderr:\n%s\nstdout:\n%s", err, stderr, stdout)
		}

		report := decodeReport(t, stdout)
		if report.ChartName != fixture.ChartName || report.ChartVersion != fixture.ChartVersion {
			t.Fatalf("report metadata = (%q, %q), want (%q, %q)", report.ChartName, report.ChartVersion, fixture.ChartName, fixture.ChartVersion)
		}
		if report.OverallGrade != fixture.Expected.OverallGrade {
			t.Fatalf("report.OverallGrade = %q, want %q", report.OverallGrade, fixture.Expected.OverallGrade)
		}
		if report.TotalFindings != fixture.Expected.TotalFindings {
			t.Fatalf("report.TotalFindings = %d, want %d", report.TotalFindings, fixture.Expected.TotalFindings)
		}
		if len(report.Categories) != len(models.AllCategories()) {
			t.Fatalf("len(report.Categories) = %d, want %d", len(report.Categories), len(models.AllCategories()))
		}
		if stderr != "" {
			t.Fatalf("stderr = %q, want empty", stderr)
		}
	})

	t.Run("unmet_threshold_returns_min_score_failure_after_emitting_json", func(t *testing.T) {
		stdout, stderr, err := executeScanCommand(t, "--output", "json", "--min-score", "A", chartPath)
		if err == nil {
			t.Fatal("executeScanCommand(min-score A) error = nil, want non-nil")
		}
		if !strings.Contains(err.Error(), "min-score:") || !strings.Contains(err.Error(), "below required A") {
			t.Fatalf("error %q does not describe min-score failure", err)
		}

		report := decodeReport(t, stdout)
		if report.OverallGrade != fixture.Expected.OverallGrade {
			t.Fatalf("report.OverallGrade = %q, want %q", report.OverallGrade, fixture.Expected.OverallGrade)
		}
		if !strings.Contains(stderr, "min-score:") {
			t.Fatalf("stderr %q does not include min-score failure", stderr)
		}
	})

	t.Run("invalid_threshold_is_rejected_before_scan_runs", func(t *testing.T) {
		stdout, stderr, err := executeScanCommand(t, "--min-score", "Z", chartPath)
		if err == nil {
			t.Fatal("executeScanCommand(min-score Z) error = nil, want non-nil")
		}
		if !strings.Contains(err.Error(), "min-score: invalid grade") {
			t.Fatalf("error %q does not describe invalid grade", err)
		}
		if stdout != "" {
			t.Fatalf("stdout = %q, want empty", stdout)
		}
		if !strings.Contains(stderr, "min-score: invalid grade") {
			t.Fatalf("stderr %q does not include invalid-grade message", stderr)
		}
	})

	t.Run("unsupported_output_reports_report_phase", func(t *testing.T) {
		stdout, stderr, err := executeScanCommand(t, "--output", "sarif", chartPath)
		if err == nil {
			t.Fatal("executeScanCommand(output sarif) error = nil, want non-nil")
		}
		if !strings.Contains(err.Error(), "report: unsupported output format") {
			t.Fatalf("error %q does not identify report phase", err)
		}
		if stdout != "" {
			t.Fatalf("stdout = %q, want empty", stdout)
		}
		if !strings.Contains(stderr, "report: unsupported output format") {
			t.Fatalf("stderr %q does not include report phase error", stderr)
		}
	})
}

func TestScanCommandAcceptsConfigFlag(t *testing.T) {
	fixture := realcharts.FixtureByID(t, "nginx-ingress")
	chartPath := realcharts.FixturePath(t, fixture)

	t.Run("config_file_changes_rule_execution", func(t *testing.T) {
		configPath := filepath.Join(t.TempDir(), ".helmdoc.yaml")
		if err := os.WriteFile(configPath, []byte(strings.TrimSpace(`
categories:
  security:
    enabled: false
`)+"\n"), 0o644); err != nil {
			t.Fatalf("os.WriteFile(%q) error = %v", configPath, err)
		}

		stdout, stderr, err := executeScanCommand(t, "--output", "json", "--config", configPath, chartPath)
		if err != nil {
			t.Fatalf("executeScanCommand(config) error = %v\nstderr:\n%s\nstdout:\n%s", err, stderr, stdout)
		}

		report := decodeReport(t, stdout)
		if report.TotalFindings != fixture.Expected.TotalFindings-1 {
			t.Fatalf("report.TotalFindings = %d, want %d after disabling security", report.TotalFindings, fixture.Expected.TotalFindings-1)
		}

		security := findCategory(t, report, models.CategorySecurity)
		if security.Score != 100 {
			t.Fatalf("security.Score = %v, want 100", security.Score)
		}
		if security.Grade != models.GradeA {
			t.Fatalf("security.Grade = %q, want %q", security.Grade, models.GradeA)
		}
		if len(security.Findings) != 0 {
			t.Fatalf("len(security.Findings) = %d, want 0", len(security.Findings))
		}
		if stderr != "" {
			t.Fatalf("stderr = %q, want empty", stderr)
		}
	})

	t.Run("invalid_config_reports_config_phase", func(t *testing.T) {
		configPath := filepath.Join(t.TempDir(), ".helmdoc.yaml")
		if err := os.WriteFile(configPath, []byte("rules:\n  SEC001:\n    severity: urgent\n"), 0o644); err != nil {
			t.Fatalf("os.WriteFile(%q) error = %v", configPath, err)
		}

		stdout, stderr, err := executeScanCommand(t, "--config", configPath, chartPath)
		if err == nil {
			t.Fatal("executeScanCommand(invalid config) error = nil, want non-nil")
		}
		if !strings.Contains(err.Error(), "config:") || !strings.Contains(err.Error(), configPath) {
			t.Fatalf("error %q does not identify config phase and file path", err)
		}
		if stdout != "" {
			t.Fatalf("stdout = %q, want empty", stdout)
		}
		if !strings.Contains(stderr, "config:") {
			t.Fatalf("stderr %q does not include config phase error", stderr)
		}
	})
}

func executeScanCommand(t *testing.T, args ...string) (stdout string, stderr string, err error) {
	t.Helper()

	cmd := newScanCommand()
	var stdoutBuf bytes.Buffer
	var stderrBuf bytes.Buffer
	cmd.SetOut(&stdoutBuf)
	cmd.SetErr(&stderrBuf)
	cmd.SetArgs(args)

	err = cmd.Execute()
	return stdoutBuf.String(), stderrBuf.String(), err
}

func decodeReport(t *testing.T, raw string) models.Report {
	t.Helper()

	var report models.Report
	if err := json.Unmarshal([]byte(raw), &report); err != nil {
		t.Fatalf("json.Unmarshal(%q) error = %v", raw, err)
	}
	return report
}

func findCategory(t *testing.T, report models.Report, category models.Category) models.CategoryScore {
	t.Helper()

	for _, candidate := range report.Categories {
		if candidate.Category == category {
			return candidate
		}
	}

	t.Fatalf("category %q missing from report", category)
	return models.CategoryScore{}
}
