package report

import (
	"bytes"
	"strings"
	"testing"

	"github.com/belyaev-dev/helmdoc/internal/score"
	"github.com/belyaev-dev/helmdoc/pkg/models"
)

func TestRenderText(t *testing.T) {
	t.Run("renders_stable_no_color_report_with_all_categories", func(t *testing.T) {
		report := score.ComputeReport([]models.Finding{
			{
				RuleID:      "SEC003",
				Category:    models.CategorySecurity,
				Severity:    models.SeverityError,
				Title:       "Container root filesystem is writable",
				Description: "container \"controller\" does not set readOnlyRootFilesystem: true.",
				Remediation: "Set controller.containerSecurityContext.readOnlyRootFilesystem=true.",
				Path:        "templates/controller-deployment.yaml",
				Resource:    "Deployment/demo",
			},
			{
				RuleID:      "RES002",
				Category:    models.CategoryResources,
				Severity:    models.SeverityWarning,
				Title:       "Workload missing resource requests",
				Description: "container \"controller\" does not define cpu or memory requests.",
				Remediation: "Set controller.resources.requests in values.yaml.",
				Path:        "templates/controller-deployment.yaml",
				Resource:    "Deployment/demo",
			},
		})
		report.ChartName = "demo"
		report.ChartVersion = "1.2.3"

		var out bytes.Buffer
		if err := RenderText(&out, &report); err != nil {
			t.Fatalf("RenderText() error = %v", err)
		}

		got := out.String()
		if strings.Contains(got, "\x1b[") {
			t.Fatalf("RenderText() unexpectedly emitted ANSI escapes in no-color mode:\n%s", got)
		}

		for _, want := range []string{
			"HelmDoc scan report",
			"Chart: demo@1.2.3",
			"Overall: A",
			"Score: 95.7/100",
			"Total findings: 2",
			"Findings:",
			"Security findings (1):",
			"- [SEC003][error] Deployment/demo @ templates/controller-deployment.yaml",
			"  Title: Container root filesystem is writable",
			"  Description: container \"controller\" does not set readOnlyRootFilesystem: true.",
			"  Remediation: Set controller.containerSecurityContext.readOnlyRootFilesystem=true.",
			"Resources findings (1):",
			"- [RES002][warning] Deployment/demo @ templates/controller-deployment.yaml",
			"  Title: Workload missing resource requests",
			"  Description: container \"controller\" does not define cpu or memory requests.",
			"  Remediation: Set controller.resources.requests in values.yaml.",
		} {
			if !strings.Contains(got, want) {
				t.Fatalf("RenderText() output missing %q\n\nFull output:\n%s", want, got)
			}
		}

		wantOrder := []string{
			"- Security: B (85.0/100, weight 3.0, findings 1)",
			"- Resources: A (92.0/100, weight 2.5, findings 1)",
			"- Storage: A (100.0/100, weight 1.5, findings 0)",
			"- Health: A (100.0/100, weight 2.0, findings 0)",
			"- Availability: A (100.0/100, weight 1.0, findings 0)",
			"- Network: A (100.0/100, weight 1.0, findings 0)",
			"- Images: A (100.0/100, weight 1.0, findings 0)",
			"- Ingress: A (100.0/100, weight 1.0, findings 0)",
			"- Scaling: A (100.0/100, weight 1.0, findings 0)",
			"- Config: A (100.0/100, weight 1.0, findings 0)",
		}
		lastIndex := -1
		for _, want := range wantOrder {
			idx := strings.Index(got, want)
			if idx == -1 {
				t.Fatalf("RenderText() category summary missing %q\n\nFull output:\n%s", want, got)
			}
			if idx <= lastIndex {
				t.Fatalf("RenderText() category summary order incorrect around %q\n\nFull output:\n%s", want, got)
			}
			lastIndex = idx
		}
	})

	t.Run("renders_empty_report_without_findings_section", func(t *testing.T) {
		report := score.ComputeReport(nil)
		report.ChartName = "clean-chart"

		var out bytes.Buffer
		if err := RenderText(&out, &report); err != nil {
			t.Fatalf("RenderText() error = %v", err)
		}

		got := out.String()
		for _, want := range []string{
			"Chart: clean-chart",
			"Overall: A",
			"Score: 100.0/100",
			"Total findings: 0",
			"- Security: A (100.0/100, weight 3.0, findings 0)",
			"- Config: A (100.0/100, weight 1.0, findings 0)",
			"No findings.",
		} {
			if !strings.Contains(got, want) {
				t.Fatalf("RenderText() output missing %q\n\nFull output:\n%s", want, got)
			}
		}
		if strings.Contains(got, "Findings:") {
			t.Fatalf("RenderText() unexpectedly rendered findings section for empty report:\n%s", got)
		}
	})
}
