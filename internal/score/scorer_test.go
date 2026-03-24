package score

import (
	"math"
	"reflect"
	"testing"

	"github.com/belyaev-dev/helmdoc/pkg/models"
)

func TestComputeReport(t *testing.T) {
	t.Run("emits_all_categories_for_empty_scan", func(t *testing.T) {
		report := ComputeReport(nil)

		if report.TotalFindings != 0 {
			t.Fatalf("report.TotalFindings = %d, want 0", report.TotalFindings)
		}
		if report.OverallScore != 100 {
			t.Fatalf("report.OverallScore = %v, want 100", report.OverallScore)
		}
		if report.OverallGrade != models.GradeA {
			t.Fatalf("report.OverallGrade = %q, want %q", report.OverallGrade, models.GradeA)
		}
		if len(report.Categories) != len(models.AllCategories()) {
			t.Fatalf("len(report.Categories) = %d, want %d", len(report.Categories), len(models.AllCategories()))
		}

		gotCategories := make([]models.Category, 0, len(report.Categories))
		for _, category := range report.Categories {
			gotCategories = append(gotCategories, category.Category)
			if category.Score != 100 {
				t.Fatalf("category %q score = %v, want 100", category.Category, category.Score)
			}
			if category.Grade != models.GradeA {
				t.Fatalf("category %q grade = %q, want %q", category.Category, category.Grade, models.GradeA)
			}
			if len(category.Findings) != 0 {
				t.Fatalf("category %q findings = %d, want 0", category.Category, len(category.Findings))
			}
		}
		if !reflect.DeepEqual(gotCategories, models.AllCategories()) {
			t.Fatalf("category order = %#v, want %#v", gotCategories, models.AllCategories())
		}
	})

	t.Run("computes_weighted_scores_from_findings", func(t *testing.T) {
		report := ComputeReport([]models.Finding{
			{RuleID: "SEC001", Category: models.CategorySecurity, Severity: models.SeverityError},
			{RuleID: "RES001", Category: models.CategoryResources, Severity: models.SeverityWarning},
			{RuleID: "RES002", Category: models.CategoryResources, Severity: models.SeverityWarning},
			{RuleID: "HLT001", Category: models.CategoryHealth, Severity: models.SeverityInfo},
			{RuleID: "CFG001", Category: models.CategoryConfig, Severity: models.SeverityCritical},
		})

		assertCategory(t, report, models.CategorySecurity, 85, models.GradeB, 3.0, 1)
		assertCategory(t, report, models.CategoryResources, 84, models.GradeB, 2.5, 2)
		assertCategory(t, report, models.CategoryHealth, 97, models.GradeA, 2.0, 1)
		assertCategory(t, report, models.CategoryConfig, 75, models.GradeC, 1.0, 1)
		assertCategory(t, report, models.CategoryStorage, 100, models.GradeA, 1.5, 0)

		const wantOverall = 1384.0 / 15.0
		if math.Abs(report.OverallScore-wantOverall) > 0.000001 {
			t.Fatalf("report.OverallScore = %v, want %v", report.OverallScore, wantOverall)
		}
		if report.OverallGrade != models.GradeA {
			t.Fatalf("report.OverallGrade = %q, want %q", report.OverallGrade, models.GradeA)
		}
		if report.TotalFindings != 5 {
			t.Fatalf("report.TotalFindings = %d, want 5", report.TotalFindings)
		}
	})

	t.Run("floors_category_scores_at_zero", func(t *testing.T) {
		findings := make([]models.Finding, 0, 5)
		for range 5 {
			findings = append(findings, models.Finding{RuleID: "SEC999", Category: models.CategorySecurity, Severity: models.SeverityCritical})
		}

		report := ComputeReport(findings)
		assertCategory(t, report, models.CategorySecurity, 0, models.GradeF, 3.0, 5)
		if report.OverallGrade != models.GradeB {
			t.Fatalf("report.OverallGrade = %q, want %q", report.OverallGrade, models.GradeB)
		}
		if math.Abs(report.OverallScore-80) > 0.000001 {
			t.Fatalf("report.OverallScore = %v, want 80", report.OverallScore)
		}
	})

	t.Run("trusts_finding_severity_after_config_overrides", func(t *testing.T) {
		report := ComputeReport([]models.Finding{{
			RuleID:   "AVL001",
			Category: models.CategoryAvailability,
			Severity: models.SeverityCritical,
		}})

		assertCategory(t, report, models.CategoryAvailability, 75, models.GradeC, 1.0, 1)
		if got := report.Categories[4].Findings[0].Severity; got != models.SeverityCritical {
			t.Fatalf("availability finding severity = %v, want %v", got, models.SeverityCritical)
		}
	})
}

func assertCategory(t *testing.T, report models.Report, wantCategory models.Category, wantScore float64, wantGrade models.Grade, wantWeight float64, wantFindings int) {
	t.Helper()

	for _, category := range report.Categories {
		if category.Category != wantCategory {
			continue
		}
		if math.Abs(category.Score-wantScore) > 0.000001 {
			t.Fatalf("category %q score = %v, want %v", wantCategory, category.Score, wantScore)
		}
		if category.Grade != wantGrade {
			t.Fatalf("category %q grade = %q, want %q", wantCategory, category.Grade, wantGrade)
		}
		if math.Abs(category.Weight-wantWeight) > 0.000001 {
			t.Fatalf("category %q weight = %v, want %v", wantCategory, category.Weight, wantWeight)
		}
		if len(category.Findings) != wantFindings {
			t.Fatalf("category %q findings = %d, want %d", wantCategory, len(category.Findings), wantFindings)
		}
		return
	}

	t.Fatalf("category %q missing from report", wantCategory)
}
