package config

import (
	"path/filepath"
	"testing"

	internalchart "github.com/belyaev-dev/helmdoc/internal/chart"
	"github.com/belyaev-dev/helmdoc/internal/rules"
	"github.com/belyaev-dev/helmdoc/pkg/models"
)

func TestHardcodedEndpointRule(t *testing.T) {
	rule := findConfigRule(t, "CFG001")
	if rule.Title() != "Container environment variable hardcodes an endpoint" {
		t.Fatalf("rule.Title() = %q", rule.Title())
	}
	if rule.Category() != models.CategoryConfig {
		t.Fatalf("rule.Category() = %q, want %q", rule.Category(), models.CategoryConfig)
	}
	if rule.Severity() != models.SeverityWarning {
		t.Fatalf("rule.Severity() = %v, want %v", rule.Severity(), models.SeverityWarning)
	}

	t.Run("literal_env_endpoints_are_flagged", func(t *testing.T) {
		ctx := rules.AnalysisContext{RenderedResources: map[string][]models.K8sResource{
			"templates/workload.yaml": {
				makeConfigDeployment(
					"demo",
					[]map[string]any{{
						"name": "bootstrap",
						"env":  []any{map[string]any{"name": "BOOTSTRAP_URL", "value": "http://127.0.0.1:8080"}},
					}},
					[]map[string]any{{
						"name": "app",
						"env": []any{
							map[string]any{"name": "API_URL", "value": "https://api.example.com/v1"},
							map[string]any{"name": "CACHE_HOST", "value": "redis.svc.cluster.local"},
							map[string]any{"name": "METRICS_ENDPOINT", "value": "10.0.0.42:9090"},
						},
					}},
				),
			},
		}}

		findings := configFindingsForRule(ctx, rule.ID())
		if len(findings) != 4 {
			t.Fatalf("len(findings) = %d, want 4 (%#v)", len(findings), findings)
		}

		assertConfigFindingMetadata(t, findings[0], rule, "templates/workload.yaml", "Deployment/demo")
		if findings[0].Description != "container \"app\" in Deployment/demo sets env \"API_URL\" to hardcoded URL \"https://api.example.com/v1\"." {
			t.Fatalf("findings[0].Description = %q", findings[0].Description)
		}
		assertConfigFindingMetadata(t, findings[1], rule, "templates/workload.yaml", "Deployment/demo")
		if findings[1].Description != "container \"app\" in Deployment/demo sets env \"CACHE_HOST\" to hardcoded hostname endpoint \"redis.svc.cluster.local\"." {
			t.Fatalf("findings[1].Description = %q", findings[1].Description)
		}
		assertConfigFindingMetadata(t, findings[2], rule, "templates/workload.yaml", "Deployment/demo")
		if findings[2].Description != "container \"app\" in Deployment/demo sets env \"METRICS_ENDPOINT\" to hardcoded endpoint \"10.0.0.42:9090\"." {
			t.Fatalf("findings[2].Description = %q", findings[2].Description)
		}
		assertConfigFindingMetadata(t, findings[3], rule, "templates/workload.yaml", "Deployment/demo")
		if findings[3].Description != "init container \"bootstrap\" in Deployment/demo sets env \"BOOTSTRAP_URL\" to hardcoded URL \"http://127.0.0.1:8080\"." {
			t.Fatalf("findings[3].Description = %q", findings[3].Description)
		}
	})

	t.Run("value_from_and_non_endpoint_literals_are_ignored", func(t *testing.T) {
		ctx := rules.AnalysisContext{RenderedResources: map[string][]models.K8sResource{
			"templates/workload.yaml": {
				makeConfigDeployment(
					"clean",
					nil,
					[]map[string]any{{
						"name": "app",
						"env": []any{
							map[string]any{"name": "POD_NAME", "valueFrom": map[string]any{"fieldRef": map[string]any{"fieldPath": "metadata.name"}}},
							map[string]any{"name": "CONFIG_PATH", "value": "/usr/local/lib/libmimalloc.so"},
							map[string]any{"name": "RELEASE", "value": "v1.2.3"},
							map[string]any{"name": "DYNAMIC_URL", "value": "https://${SERVICE_HOST}"},
						},
					}},
				),
			},
		}}

		if findings := configFindingsForRule(ctx, rule.ID()); len(findings) != 0 {
			t.Fatalf("findings = %#v, want none", findings)
		}
	})

	t.Run("nginx_ingress_fieldref_envs_stay_clean", func(t *testing.T) {
		ctx := renderedAnalysisContext(t, nginxIngressChartPath())
		if findings := configFindingsForRule(ctx, rule.ID()); len(findings) != 0 {
			t.Fatalf("CFG001 findings for nginx-ingress = %#v, want none", findings)
		}
	})
}

func findConfigRule(t *testing.T, ruleID string) rules.Rule {
	t.Helper()
	for _, rule := range rules.ByCategory(models.CategoryConfig) {
		if rule.ID() == ruleID {
			return rule
		}
	}
	t.Fatalf("config rule %q not registered", ruleID)
	return nil
}

func configFindingsForRule(ctx rules.AnalysisContext, ruleID string) []models.Finding {
	allFindings := rules.RunAll(ctx)
	findings := make([]models.Finding, 0)
	for _, finding := range allFindings {
		if finding.RuleID == ruleID {
			findings = append(findings, finding)
		}
	}
	return findings
}

func assertConfigFindingMetadata(t *testing.T, finding models.Finding, rule rules.Rule, wantPath, wantResource string) {
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

func makeConfigDeployment(name string, initContainers, containers []map[string]any) models.K8sResource {
	podSpec := map[string]any{}
	if len(initContainers) > 0 {
		podSpec["initContainers"] = configToAnySlice(initContainers)
	}
	if len(containers) > 0 {
		podSpec["containers"] = configToAnySlice(containers)
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

func configToAnySlice(items []map[string]any) []any {
	result := make([]any, 0, len(items))
	for _, item := range items {
		result = append(result, item)
	}
	return result
}
