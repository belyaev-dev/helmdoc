package report

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/belyaev-dev/helmdoc/internal/score"
	"github.com/belyaev-dev/helmdoc/pkg/models"
)

func TestRenderJSON(t *testing.T) {
	report := score.ComputeReport([]models.Finding{
		{
			RuleID:      "SEC003",
			Category:    models.CategorySecurity,
			Severity:    models.SeverityError,
			Title:       "Container root filesystem is writable",
			Description: "readOnlyRootFilesystem is not true",
			Remediation: "Set readOnlyRootFilesystem: true",
			Path:        "templates/controller-deployment.yaml",
			Resource:    "Deployment/demo",
		},
	})
	report.ChartName = "demo"
	report.ChartVersion = "1.2.3"

	encoded, err := RenderJSON(report)
	if err != nil {
		t.Fatalf("RenderJSON() error = %v", err)
	}
	if !strings.Contains(string(encoded), "\n  \"chart_name\": \"demo\"") {
		t.Fatalf("RenderJSON() did not produce indented output:\n%s", string(encoded))
	}

	var decoded models.Report
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("json.Unmarshal(RenderJSON()) error = %v", err)
	}

	if decoded.ChartName != report.ChartName || decoded.ChartVersion != report.ChartVersion {
		t.Fatalf("decoded chart metadata = (%q, %q), want (%q, %q)", decoded.ChartName, decoded.ChartVersion, report.ChartName, report.ChartVersion)
	}
	if len(decoded.Categories) != len(models.AllCategories()) {
		t.Fatalf("len(decoded.Categories) = %d, want %d", len(decoded.Categories), len(models.AllCategories()))
	}

	gotOrder := make([]models.Category, 0, len(decoded.Categories))
	for _, category := range decoded.Categories {
		gotOrder = append(gotOrder, category.Category)
	}
	if !reflect.DeepEqual(gotOrder, models.AllCategories()) {
		t.Fatalf("decoded category order = %#v, want %#v", gotOrder, models.AllCategories())
	}

	security := decoded.Categories[0]
	if security.Weight != 3.0 {
		t.Fatalf("security weight = %v, want 3.0", security.Weight)
	}
	if len(security.Findings) != 1 {
		t.Fatalf("len(security.Findings) = %d, want 1", len(security.Findings))
	}
	if got := security.Findings[0].Severity; got != models.SeverityError {
		t.Fatalf("decoded severity = %v, want %v", got, models.SeverityError)
	}
}
