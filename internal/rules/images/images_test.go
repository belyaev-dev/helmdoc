package images

import (
	"path/filepath"
	"testing"

	internalchart "github.com/belyaev-dev/helmdoc/internal/chart"
	"github.com/belyaev-dev/helmdoc/internal/rules"
	"github.com/belyaev-dev/helmdoc/pkg/models"
)

func TestImageRules(t *testing.T) {
	t.Run("tag_rule_flags_missing_and_latest_tags", func(t *testing.T) {
		rule := findImageRule(t, "IMG001")
		if rule.Title() != "Container image tag is missing or mutable" {
			t.Fatalf("rule.Title() = %q", rule.Title())
		}
		if rule.Category() != models.CategoryImages {
			t.Fatalf("rule.Category() = %q, want %q", rule.Category(), models.CategoryImages)
		}
		if rule.Severity() != models.SeverityWarning {
			t.Fatalf("rule.Severity() = %v, want %v", rule.Severity(), models.SeverityWarning)
		}

		ctx := rules.AnalysisContext{RenderedResources: map[string][]models.K8sResource{
			"templates/workload.yaml": {
				makeImageDeployment(
					"demo",
					[]map[string]any{{"name": "bootstrap", "image": "busybox:latest"}},
					[]map[string]any{{"name": "app", "image": "ghcr.io/example/demo"}},
				),
			},
		}}

		findings := imageFindingsForRule(ctx, rule.ID())
		if len(findings) != 2 {
			t.Fatalf("len(findings) = %d, want 2 (%#v)", len(findings), findings)
		}

		assertImageFindingMetadata(t, findings[0], rule, "templates/workload.yaml", "Deployment/demo")
		if findings[0].Description != "container \"app\" in Deployment/demo uses image \"ghcr.io/example/demo\" without an explicit tag." {
			t.Fatalf("findings[0].Description = %q", findings[0].Description)
		}

		assertImageFindingMetadata(t, findings[1], rule, "templates/workload.yaml", "Deployment/demo")
		if findings[1].Description != "init container \"bootstrap\" in Deployment/demo uses image \"busybox:latest\" with the mutable \"latest\" tag." {
			t.Fatalf("findings[1].Description = %q", findings[1].Description)
		}
	})

	t.Run("digest_rule_flags_unpinned_images_and_keeps_pinned_images_clean", func(t *testing.T) {
		rule := findImageRule(t, "IMG002")
		if rule.Title() != "Container image is not pinned by digest" {
			t.Fatalf("rule.Title() = %q", rule.Title())
		}
		if rule.Category() != models.CategoryImages {
			t.Fatalf("rule.Category() = %q, want %q", rule.Category(), models.CategoryImages)
		}
		if rule.Severity() != models.SeverityWarning {
			t.Fatalf("rule.Severity() = %v, want %v", rule.Severity(), models.SeverityWarning)
		}

		ctx := rules.AnalysisContext{RenderedResources: map[string][]models.K8sResource{
			"templates/workload.yaml": {
				makeImageDeployment(
					"demo",
					nil,
					[]map[string]any{{"name": "app", "image": "ghcr.io/example/demo:1.2.3"}},
				),
				makeImageCronJob(
					"cleanup",
					[]map[string]any{{"name": "cleanup", "image": "registry.k8s.io/ingress-nginx/controller:v1.11.1@sha256:deadbeef"}},
				),
			},
		}}

		findings := imageFindingsForRule(ctx, rule.ID())
		if len(findings) != 1 {
			t.Fatalf("len(findings) = %d, want 1 (%#v)", len(findings), findings)
		}

		assertImageFindingMetadata(t, findings[0], rule, "templates/workload.yaml", "Deployment/demo")
		if findings[0].Description != "container \"app\" in Deployment/demo uses image \"ghcr.io/example/demo:1.2.3\" without a pinned digest." {
			t.Fatalf("findings[0].Description = %q", findings[0].Description)
		}

		nginxCtx := renderedAnalysisContext(t, nginxIngressChartPath())
		if findings := imageFindingsForRule(nginxCtx, rule.ID()); len(findings) != 0 {
			t.Fatalf("IMG002 findings for nginx-ingress = %#v, want none", findings)
		}
		if findings := imageFindingsForRule(nginxCtx, "IMG001"); len(findings) != 0 {
			t.Fatalf("IMG001 findings for nginx-ingress = %#v, want none", findings)
		}
	})
}

func findImageRule(t *testing.T, ruleID string) rules.Rule {
	t.Helper()
	for _, rule := range rules.ByCategory(models.CategoryImages) {
		if rule.ID() == ruleID {
			return rule
		}
	}
	t.Fatalf("image rule %q not registered", ruleID)
	return nil
}

func imageFindingsForRule(ctx rules.AnalysisContext, ruleID string) []models.Finding {
	allFindings := rules.RunAll(ctx)
	findings := make([]models.Finding, 0)
	for _, finding := range allFindings {
		if finding.RuleID == ruleID {
			findings = append(findings, finding)
		}
	}
	return findings
}

func assertImageFindingMetadata(t *testing.T, finding models.Finding, rule rules.Rule, wantPath, wantResource string) {
	t.Helper()
	if finding.RuleID != rule.ID() {
		t.Fatalf("finding.RuleID = %q, want %q", finding.RuleID, rule.ID())
	}
	if finding.Category != rule.Category() {
		t.Fatalf("finding.Category = %q, want %q", finding.Category, rule.Category())
	}
	if finding.Severity != rule.Severity() {
		t.Fatalf("finding.Severity = %v, want %v", finding.Severity, rule.Severity())
	}
	if finding.Title != rule.Title() {
		t.Fatalf("finding.Title = %q, want %q", finding.Title, rule.Title())
	}
	if finding.Path != wantPath {
		t.Fatalf("finding.Path = %q, want %q", finding.Path, wantPath)
	}
	if finding.Resource != wantResource {
		t.Fatalf("finding.Resource = %q, want %q", finding.Resource, wantResource)
	}
}

func renderedAnalysisContext(t *testing.T, chartPath string) rules.AnalysisContext {
	t.Helper()

	loadedChart, err := internalchart.LoadChart(chartPath)
	if err != nil {
		t.Fatalf("LoadChart(%q) error = %v", chartPath, err)
	}

	rendered, err := internalchart.RenderChart(loadedChart)
	if err != nil {
		t.Fatalf("RenderChart(%q) error = %v", chartPath, err)
	}

	return rules.AnalysisContext{
		Chart:             loadedChart,
		RenderedResources: rendered,
		ValuesSurface:     internalchart.AnalyzeValues(loadedChart),
	}
}

func nginxIngressChartPath() string {
	return filepath.Join("..", "..", "..", "testdata", "nginx-ingress")
}

func makeImageDeployment(name string, initContainers, containers []map[string]any) models.K8sResource {
	podSpec := map[string]any{}
	if len(initContainers) > 0 {
		podSpec["initContainers"] = toAnySlice(initContainers)
	}
	if len(containers) > 0 {
		podSpec["containers"] = toAnySlice(containers)
	}

	return models.K8sResource{
		APIVersion: "apps/v1",
		Kind:       "Deployment",
		Name:       name,
		Raw: map[string]any{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]any{
				"name": name,
			},
			"spec": map[string]any{
				"template": map[string]any{
					"spec": podSpec,
				},
			},
		},
	}
}

func makeImageCronJob(name string, containers []map[string]any) models.K8sResource {
	podSpec := map[string]any{"containers": toAnySlice(containers)}
	return models.K8sResource{
		APIVersion: "batch/v1",
		Kind:       "CronJob",
		Name:       name,
		Raw: map[string]any{
			"apiVersion": "batch/v1",
			"kind":       "CronJob",
			"metadata": map[string]any{
				"name": name,
			},
			"spec": map[string]any{
				"jobTemplate": map[string]any{
					"spec": map[string]any{
						"template": map[string]any{
							"spec": podSpec,
						},
					},
				},
			},
		},
	}
}

func toAnySlice(items []map[string]any) []any {
	result := make([]any, 0, len(items))
	for _, item := range items {
		result = append(result, item)
	}
	return result
}
