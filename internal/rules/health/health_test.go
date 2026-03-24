package health

import (
	"path/filepath"
	"testing"

	internalchart "github.com/belyaev-dev/helmdoc/internal/chart"
	"github.com/belyaev-dev/helmdoc/internal/rules"
	"github.com/belyaev-dev/helmdoc/pkg/models"
)

func TestHealthRules(t *testing.T) {
	livenessRule := findHealthRule(t, "HLT001")
	readinessRule := findHealthRule(t, "HLT002")

	if livenessRule.Title() != "Container liveness probe is missing" {
		t.Fatalf("livenessRule.Title() = %q", livenessRule.Title())
	}
	if readinessRule.Title() != "Container readiness probe is missing" {
		t.Fatalf("readinessRule.Title() = %q", readinessRule.Title())
	}
	for _, rule := range []rules.Rule{livenessRule, readinessRule} {
		if rule.Category() != models.CategoryHealth {
			t.Fatalf("rule %s Category() = %q, want %q", rule.ID(), rule.Category(), models.CategoryHealth)
		}
		if rule.Severity() != models.SeverityWarning {
			t.Fatalf("rule %s Severity() = %v, want %v", rule.ID(), rule.Severity(), models.SeverityWarning)
		}
	}

	t.Run("healthy_workloads_pass_and_init_containers_are_skipped", func(t *testing.T) {
		ctx := rules.AnalysisContext{
			RenderedResources: map[string][]models.K8sResource{
				"templates/controller-deployment.yaml": {
					makeDeployment("healthy", []map[string]any{{
						"name": "bootstrap",
					}}, []map[string]any{{
						"name": "controller",
						"livenessProbe": map[string]any{
							"httpGet": map[string]any{"path": "/healthz"},
						},
						"readinessProbe": map[string]any{
							"httpGet": map[string]any{"path": "/ready"},
						},
					}}),
				},
			},
			ValuesSurface: models.NewValuesSurface(map[string]models.ValuePath{
				"controller.livenessProbe":  {Default: map[string]any{"httpGet": map[string]any{"path": "/healthz"}}, Type: "object"},
				"controller.readinessProbe": {Default: map[string]any{"httpGet": map[string]any{"path": "/ready"}}, Type: "object"},
			}),
		}

		if findings := healthFindingsForRule(ctx, livenessRule.ID()); len(findings) != 0 {
			t.Fatalf("HLT001 findings = %#v, want none", findings)
		}
		if findings := healthFindingsForRule(ctx, readinessRule.ID()); len(findings) != 0 {
			t.Fatalf("HLT002 findings = %#v, want none", findings)
		}
	})

	t.Run("one_shot_jobs_without_probes_are_flagged", func(t *testing.T) {
		ctx := rules.AnalysisContext{
			RenderedResources: map[string][]models.K8sResource{
				"templates/admission-webhooks/job-patch/job-createSecret.yaml": {
					makeJob("create-secret", []map[string]any{{
						"name": "create",
					}}),
				},
			},
			ValuesSurface: models.NewValuesSurface(map[string]models.ValuePath{
				"controller.admissionWebhooks.createSecretJob.resources": {Default: map[string]any{}, Type: "object"},
			}),
		}

		livenessFindings := healthFindingsForRule(ctx, livenessRule.ID())
		if len(livenessFindings) != 1 {
			t.Fatalf("len(HLT001 findings) = %d, want 1 (%#v)", len(livenessFindings), livenessFindings)
		}
		assertHealthFindingCoreMetadata(t, livenessFindings[0], livenessRule)
		if livenessFindings[0].Description != "container \"create\" in Job/create-secret has no livenessProbe." {
			t.Fatalf("HLT001 description = %q", livenessFindings[0].Description)
		}
		if livenessFindings[0].Remediation != "Add livenessProbe directly to the template for container \"create\" or expose controller.admissionWebhooks.createSecretJob.livenessProbe in values.yaml." {
			t.Fatalf("HLT001 remediation = %q", livenessFindings[0].Remediation)
		}

		readinessFindings := healthFindingsForRule(ctx, readinessRule.ID())
		if len(readinessFindings) != 1 {
			t.Fatalf("len(HLT002 findings) = %d, want 1 (%#v)", len(readinessFindings), readinessFindings)
		}
		assertHealthFindingCoreMetadata(t, readinessFindings[0], readinessRule)
		if readinessFindings[0].Description != "container \"create\" in Job/create-secret has no readinessProbe." {
			t.Fatalf("HLT002 description = %q", readinessFindings[0].Description)
		}
		if readinessFindings[0].Remediation != "Add readinessProbe directly to the template for container \"create\" or expose controller.admissionWebhooks.createSecretJob.readinessProbe in values.yaml." {
			t.Fatalf("HLT002 remediation = %q", readinessFindings[0].Remediation)
		}
	})

	t.Run("nginx_ingress_controller_stays_clean_while_admission_jobs_fail", func(t *testing.T) {
		ctx := renderedAnalysisContext(t, nginxIngressChartPath())

		livenessFindings := healthFindingsForRule(ctx, livenessRule.ID())
		if findHealthFindingByPath(livenessFindings, "templates/controller-deployment.yaml") != nil {
			t.Fatalf("HLT001 unexpectedly flagged controller deployment: %#v", livenessFindings)
		}
		if findHealthFindingByPath(livenessFindings, "templates/admission-webhooks/job-patch/job-createSecret.yaml") == nil {
			t.Fatalf("HLT001 did not flag create-secret job: %#v", livenessFindings)
		}
		if findHealthFindingByPath(livenessFindings, "templates/admission-webhooks/job-patch/job-patchWebhook.yaml") == nil {
			t.Fatalf("HLT001 did not flag patch-webhook job: %#v", livenessFindings)
		}

		readinessFindings := healthFindingsForRule(ctx, readinessRule.ID())
		if findHealthFindingByPath(readinessFindings, "templates/controller-deployment.yaml") != nil {
			t.Fatalf("HLT002 unexpectedly flagged controller deployment: %#v", readinessFindings)
		}
		if findHealthFindingByPath(readinessFindings, "templates/admission-webhooks/job-patch/job-createSecret.yaml") == nil {
			t.Fatalf("HLT002 did not flag create-secret job: %#v", readinessFindings)
		}
		if findHealthFindingByPath(readinessFindings, "templates/admission-webhooks/job-patch/job-patchWebhook.yaml") == nil {
			t.Fatalf("HLT002 did not flag patch-webhook job: %#v", readinessFindings)
		}
	})
}

func findHealthRule(t *testing.T, ruleID string) rules.Rule {
	t.Helper()
	for _, rule := range rules.ByCategory(models.CategoryHealth) {
		if rule.ID() == ruleID {
			return rule
		}
	}
	t.Fatalf("health rule %q not registered", ruleID)
	return nil
}

func healthFindingsForRule(ctx rules.AnalysisContext, ruleID string) []models.Finding {
	allFindings := rules.RunAll(ctx)
	findings := make([]models.Finding, 0)
	for _, finding := range allFindings {
		if finding.RuleID == ruleID {
			findings = append(findings, finding)
		}
	}
	return findings
}

func assertHealthFindingCoreMetadata(t *testing.T, finding models.Finding, rule rules.Rule) {
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

func findHealthFindingByPath(findings []models.Finding, path string) *models.Finding {
	for index := range findings {
		if findings[index].Path == path {
			return &findings[index]
		}
	}
	return nil
}

func makeDeployment(name string, initContainers, containers []map[string]any) models.K8sResource {
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

func makeJob(name string, containers []map[string]any) models.K8sResource {
	return models.K8sResource{
		APIVersion: "batch/v1",
		Kind:       "Job",
		Name:       name,
		Raw: map[string]any{
			"apiVersion": "batch/v1",
			"kind":       "Job",
			"metadata": map[string]any{
				"name": name,
			},
			"spec": map[string]any{
				"template": map[string]any{
					"spec": map[string]any{
						"containers": toAnySlice(containers),
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
