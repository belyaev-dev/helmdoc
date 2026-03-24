package resources

import (
	"path/filepath"
	"testing"

	internalchart "github.com/belyaev-dev/helmdoc/internal/chart"
	"github.com/belyaev-dev/helmdoc/internal/rules"
	"github.com/belyaev-dev/helmdoc/pkg/models"
)

func TestResourceRulesConsultValuesSurface(t *testing.T) {
	limitsRule := findResourceRule(t, "RES001")
	requestsRule := findResourceRule(t, "RES002")

	if limitsRule.Title() != "Container resource limits are missing" {
		t.Fatalf("limitsRule.Title() = %q", limitsRule.Title())
	}
	if requestsRule.Title() != "Container resource requests are missing" {
		t.Fatalf("requestsRule.Title() = %q", requestsRule.Title())
	}
	for _, rule := range []rules.Rule{limitsRule, requestsRule} {
		if rule.Category() != models.CategoryResources {
			t.Fatalf("rule %s Category() = %q, want %q", rule.ID(), rule.Category(), models.CategoryResources)
		}
		if rule.Severity() != models.SeverityWarning {
			t.Fatalf("rule %s Severity() = %v, want %v", rule.ID(), rule.Severity(), models.SeverityWarning)
		}
	}

	t.Run("exposed_empty_defaults_are_treated_as_configurable", func(t *testing.T) {
		ctx := rules.AnalysisContext{
			RenderedResources: map[string][]models.K8sResource{
				"templates/controller-deployment.yaml": {
					makeDeployment("controller", nil, []map[string]any{{
						"name": "controller",
						"resources": map[string]any{
							"requests": map[string]any{"cpu": "100m"},
						},
					}}),
				},
				"templates/admission-webhooks/job-patch/job-createSecret.yaml": {
					makeJob("create-secret", []map[string]any{{
						"name": "create",
					}}),
				},
			},
			ValuesSurface: models.NewValuesSurface(map[string]models.ValuePath{
				"controller.resources":                                   {Default: map[string]any{"requests": map[string]any{"cpu": "100m"}}, Type: "object"},
				"controller.resources.requests.cpu":                      {Default: "100m", Type: "string"},
				"controller.admissionWebhooks.createSecretJob.resources": {Default: map[string]any{}, Type: "object"},
			}),
		}

		limitsFindings := findingsForRule(ctx, limitsRule.ID())
		if len(limitsFindings) != 2 {
			t.Fatalf("len(RES001 findings) = %d, want 2 (%#v)", len(limitsFindings), limitsFindings)
		}
		assertFindingCoreMetadata(t, limitsFindings[0], limitsRule)
		if limitsFindings[0].Path != "templates/admission-webhooks/job-patch/job-createSecret.yaml" || limitsFindings[0].Resource != "Job/create-secret" {
			t.Fatalf("job RES001 finding = (%q, %q)", limitsFindings[0].Path, limitsFindings[0].Resource)
		}
		if limitsFindings[0].Remediation != "Set controller.admissionWebhooks.createSecretJob.resources.limits in values.yaml. The chart already exposes controller.admissionWebhooks.createSecretJob.resources, but its default leaves limits empty." {
			t.Fatalf("job RES001 remediation = %q", limitsFindings[0].Remediation)
		}
		assertFindingCoreMetadata(t, limitsFindings[1], limitsRule)
		if limitsFindings[1].Path != "templates/controller-deployment.yaml" || limitsFindings[1].Resource != "Deployment/controller" {
			t.Fatalf("controller RES001 finding = (%q, %q)", limitsFindings[1].Path, limitsFindings[1].Resource)
		}
		if limitsFindings[1].Description != "container \"controller\" in Deployment/controller does not define resource limits." {
			t.Fatalf("controller RES001 description = %q", limitsFindings[1].Description)
		}
		if limitsFindings[1].Remediation != "Set controller.resources.limits in values.yaml. The chart already exposes controller.resources, but its default leaves limits empty." {
			t.Fatalf("controller RES001 remediation = %q", limitsFindings[1].Remediation)
		}

		requestsFindings := findingsForRule(ctx, requestsRule.ID())
		if len(requestsFindings) != 1 {
			t.Fatalf("len(RES002 findings) = %d, want 1 (%#v)", len(requestsFindings), requestsFindings)
		}
		assertFindingCoreMetadata(t, requestsFindings[0], requestsRule)
		if requestsFindings[0].Path != "templates/admission-webhooks/job-patch/job-createSecret.yaml" || requestsFindings[0].Resource != "Job/create-secret" {
			t.Fatalf("job RES002 finding = (%q, %q)", requestsFindings[0].Path, requestsFindings[0].Resource)
		}
		if requestsFindings[0].Remediation != "Set controller.admissionWebhooks.createSecretJob.resources.requests in values.yaml. The chart already exposes controller.admissionWebhooks.createSecretJob.resources, but its default leaves requests empty." {
			t.Fatalf("job RES002 remediation = %q", requestsFindings[0].Remediation)
		}
	})

	t.Run("missing_values_path_is_reported_as_unexposed", func(t *testing.T) {
		ctx := rules.AnalysisContext{
			RenderedResources: map[string][]models.K8sResource{
				"templates/controller-deployment.yaml": {
					makeDeployment("unexposed", nil, []map[string]any{{
						"name": "controller",
					}}),
				},
			},
			ValuesSurface: models.NewValuesSurface(nil),
		}

		limitsFindings := findingsForRule(ctx, limitsRule.ID())
		if len(limitsFindings) != 1 {
			t.Fatalf("len(RES001 findings) = %d, want 1 (%#v)", len(limitsFindings), limitsFindings)
		}
		if limitsFindings[0].Remediation != "Expose controller.resources in values.yaml or add resources.limits directly to the template, because this chart does not currently expose that knob." {
			t.Fatalf("unexposed RES001 remediation = %q", limitsFindings[0].Remediation)
		}

		requestsFindings := findingsForRule(ctx, requestsRule.ID())
		if len(requestsFindings) != 1 {
			t.Fatalf("len(RES002 findings) = %d, want 1 (%#v)", len(requestsFindings), requestsFindings)
		}
		if requestsFindings[0].Remediation != "Expose controller.resources in values.yaml or add resources.requests directly to the template, because this chart does not currently expose that knob." {
			t.Fatalf("unexposed RES002 remediation = %q", requestsFindings[0].Remediation)
		}
	})

	t.Run("rendered_fixtures_keep_expected_findings", func(t *testing.T) {
		secureCtx := renderedAnalysisContext(t, fixtureChartPath())
		if findings := findingsForRule(secureCtx, limitsRule.ID()); len(findings) != 0 {
			t.Fatalf("RES001 findings for secure fixture = %#v, want none", findings)
		}
		if findings := findingsForRule(secureCtx, requestsRule.ID()); len(findings) != 0 {
			t.Fatalf("RES002 findings for secure fixture = %#v, want none", findings)
		}

		nginxCtx := renderedAnalysisContext(t, nginxIngressChartPath())

		limitsFindings := findingsForRule(nginxCtx, limitsRule.ID())
		controllerLimits := findFindingByPath(limitsFindings, "templates/controller-deployment.yaml")
		if controllerLimits == nil {
			t.Fatalf("RES001 did not flag controller deployment: %#v", limitsFindings)
		}
		assertFindingCoreMetadata(t, *controllerLimits, limitsRule)
		if controllerLimits.Resource != "Deployment/helmdoc-ingress-nginx-controller" {
			t.Fatalf("controller RES001 resource = %q", controllerLimits.Resource)
		}
		if controllerLimits.Remediation != "Set controller.resources.limits in values.yaml. The chart already exposes controller.resources, but its default leaves limits empty." {
			t.Fatalf("controller RES001 remediation = %q", controllerLimits.Remediation)
		}

		createSecretLimits := findFindingByPath(limitsFindings, "templates/admission-webhooks/job-patch/job-createSecret.yaml")
		if createSecretLimits == nil {
			t.Fatalf("RES001 did not flag create-secret job: %#v", limitsFindings)
		}
		if createSecretLimits.Remediation != "Set controller.admissionWebhooks.createSecretJob.resources.limits in values.yaml. The chart already exposes controller.admissionWebhooks.createSecretJob.resources, but its default leaves limits empty." {
			t.Fatalf("createSecret RES001 remediation = %q", createSecretLimits.Remediation)
		}

		requestsFindings := findingsForRule(nginxCtx, requestsRule.ID())
		if findFindingByPath(requestsFindings, "templates/controller-deployment.yaml") != nil {
			t.Fatalf("RES002 unexpectedly flagged controller deployment: %#v", requestsFindings)
		}
		createSecretRequests := findFindingByPath(requestsFindings, "templates/admission-webhooks/job-patch/job-createSecret.yaml")
		if createSecretRequests == nil {
			t.Fatalf("RES002 did not flag create-secret job: %#v", requestsFindings)
		}
		if createSecretRequests.Remediation != "Set controller.admissionWebhooks.createSecretJob.resources.requests in values.yaml. The chart already exposes controller.admissionWebhooks.createSecretJob.resources, but its default leaves requests empty." {
			t.Fatalf("createSecret RES002 remediation = %q", createSecretRequests.Remediation)
		}
	})
}

func findResourceRule(t *testing.T, ruleID string) rules.Rule {
	t.Helper()
	for _, rule := range rules.ByCategory(models.CategoryResources) {
		if rule.ID() == ruleID {
			return rule
		}
	}
	t.Fatalf("resource rule %q not registered", ruleID)
	return nil
}

func findingsForRule(ctx rules.AnalysisContext, ruleID string) []models.Finding {
	allFindings := rules.RunAll(ctx)
	findings := make([]models.Finding, 0)
	for _, finding := range allFindings {
		if finding.RuleID == ruleID {
			findings = append(findings, finding)
		}
	}
	return findings
}

func assertFindingCoreMetadata(t *testing.T, finding models.Finding, rule rules.Rule) {
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

func fixtureChartPath() string {
	return filepath.Join("..", "..", "..", "testdata", "testchart")
}

func nginxIngressChartPath() string {
	return filepath.Join("..", "..", "..", "testdata", "nginx-ingress")
}

func findFindingByPath(findings []models.Finding, path string) *models.Finding {
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
