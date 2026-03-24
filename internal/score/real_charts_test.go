package score

import (
	"math"
	"reflect"
	"testing"

	"github.com/belyaev-dev/helmdoc/internal/testutil/realcharts"
	"github.com/belyaev-dev/helmdoc/pkg/models"
)

func TestComputeReportAgainstRealCharts(t *testing.T) {
	manifest := realcharts.LoadManifest(t)
	if len(manifest.Fixtures) != 3 {
		t.Fatalf("len(manifest.Fixtures) = %d, want 3", len(manifest.Fixtures))
	}

	for _, fixture := range manifest.Fixtures {
		fixture := fixture
		t.Run(fixture.ID, func(t *testing.T) {
			ctx := realcharts.AnalysisContext(t, fixture)
			findings := realcharts.RunRules(t, fixture)
			report := ComputeReport(findings)
			if ctx.Chart != nil && ctx.Chart.Metadata != nil {
				report.ChartName = ctx.Chart.Metadata.Name
				report.ChartVersion = ctx.Chart.Metadata.Version
			}

			if report.ChartName != fixture.ChartName || report.ChartVersion != fixture.ChartVersion {
				t.Fatalf("report chart metadata = (%q, %q), want (%q, %q)", report.ChartName, report.ChartVersion, fixture.ChartName, fixture.ChartVersion)
			}
			if report.OverallGrade != fixture.Expected.OverallGrade {
				t.Fatalf("report.OverallGrade = %q, want %q", report.OverallGrade, fixture.Expected.OverallGrade)
			}
			if math.Abs(report.OverallScore-fixture.Expected.OverallScore) > 0.05 {
				t.Fatalf("report.OverallScore = %v, want about %v", report.OverallScore, fixture.Expected.OverallScore)
			}
			if report.TotalFindings != fixture.Expected.TotalFindings {
				t.Fatalf("report.TotalFindings = %d, want %d", report.TotalFindings, fixture.Expected.TotalFindings)
			}
			if len(report.Categories) != len(models.AllCategories()) {
				t.Fatalf("len(report.Categories) = %d, want %d", len(report.Categories), len(models.AllCategories()))
			}
			if len(fixture.Expected.CategorySummaries) != len(models.AllCategories()) {
				t.Fatalf("len(fixture.Expected.CategorySummaries) = %d, want %d", len(fixture.Expected.CategorySummaries), len(models.AllCategories()))
			}

			gotOrder := make([]models.Category, 0, len(report.Categories))
			for _, category := range report.Categories {
				gotOrder = append(gotOrder, category.Category)
			}
			if !reflect.DeepEqual(gotOrder, models.AllCategories()) {
				t.Fatalf("report category order = %#v, want %#v", gotOrder, models.AllCategories())
			}

			for _, summary := range fixture.Expected.CategorySummaries {
				assertCategory(t, report, summary.Category, summary.Score, summary.Grade, categoryWeights[summary.Category], summary.Findings)
			}

			t.Logf("fixture=%s overall_score=%.6f overall_grade=%s total_findings=%d", fixture.ID, report.OverallScore, report.OverallGrade, report.TotalFindings)
		})
	}
}
