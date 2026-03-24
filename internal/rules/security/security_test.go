package security

import (
	"path/filepath"
	"testing"

	internalchart "github.com/belyaev-dev/helmdoc/internal/chart"
	"github.com/belyaev-dev/helmdoc/internal/rules"
	"github.com/belyaev-dev/helmdoc/pkg/models"
)

func TestSecurityContextRule(t *testing.T) {
	rule := findSecurityRule(t, "SEC001")
	if rule.Title() != "Container security context is missing or unsafe" {
		t.Fatalf("rule.Title() = %q", rule.Title())
	}
	if rule.Category() != models.CategorySecurity {
		t.Fatalf("rule.Category() = %q, want %q", rule.Category(), models.CategorySecurity)
	}
	if rule.Severity() != models.SeverityError {
		t.Fatalf("rule.Severity() = %v, want %v", rule.Severity(), models.SeverityError)
	}

	ctx := rules.AnalysisContext{
		RenderedResources: map[string][]models.K8sResource{
			"templates/security.yaml": {
				makeDeployment(
					"insecure",
					[]map[string]any{{
						"name": "bootstrap",
						"securityContext": map[string]any{
							"allowPrivilegeEscalation": true,
						},
					}},
					[]map[string]any{{
						"name": "app",
					}},
				),
			},
		},
	}

	got := findingsForRule(ctx, rule.ID())
	if len(got) != 2 {
		t.Fatalf("len(findings) = %d, want 2 (%#v)", len(got), got)
	}

	assertFindingMetadata(t, got[0], rule, "templates/security.yaml", "Deployment/insecure")
	if got[0].Description != "container \"app\" in Deployment/insecure has no effective securityContext." {
		t.Fatalf("got[0].Description = %q", got[0].Description)
	}
	if got[0].Remediation != "Set container \"app\" securityContext explicitly and ensure allowPrivilegeEscalation is false." {
		t.Fatalf("got[0].Remediation = %q", got[0].Remediation)
	}

	assertFindingMetadata(t, got[1], rule, "templates/security.yaml", "Deployment/insecure")
	if got[1].Description != "init container \"bootstrap\" in Deployment/insecure explicitly sets allowPrivilegeEscalation: true." {
		t.Fatalf("got[1].Description = %q", got[1].Description)
	}
	if got[1].Remediation != "Set init container \"bootstrap\" securityContext.allowPrivilegeEscalation to false." {
		t.Fatalf("got[1].Remediation = %q", got[1].Remediation)
	}

	for _, chartPath := range []string{fixtureChartPath(), nginxIngressChartPath()} {
		ctx := renderedAnalysisContext(t, chartPath)
		if findings := findingsForRule(ctx, rule.ID()); len(findings) != 0 {
			t.Fatalf("SEC001 findings for %s = %#v, want none", chartPath, findings)
		}
	}
}

func TestCapabilitiesRule(t *testing.T) {
	rule := findSecurityRule(t, "SEC002")
	if rule.Title() != "Container does not drop ALL capabilities" {
		t.Fatalf("rule.Title() = %q", rule.Title())
	}
	if rule.Category() != models.CategorySecurity {
		t.Fatalf("rule.Category() = %q, want %q", rule.Category(), models.CategorySecurity)
	}
	if rule.Severity() != models.SeverityError {
		t.Fatalf("rule.Severity() = %v, want %v", rule.Severity(), models.SeverityError)
	}

	ctx := rules.AnalysisContext{
		RenderedResources: map[string][]models.K8sResource{
			"templates/security.yaml": {
				makeDeployment(
					"cap-gap",
					[]map[string]any{{
						"name": "bootstrap",
						"securityContext": map[string]any{
							"allowPrivilegeEscalation": false,
							"readOnlyRootFilesystem":   true,
							"capabilities": map[string]any{
								"drop": []any{"NET_BIND_SERVICE"},
							},
						},
					}},
					[]map[string]any{{
						"name": "app",
						"securityContext": map[string]any{
							"allowPrivilegeEscalation": false,
							"readOnlyRootFilesystem":   true,
							"capabilities": map[string]any{
								"drop": []any{"ALL"},
							},
						},
					}},
				),
			},
		},
	}

	got := findingsForRule(ctx, rule.ID())
	if len(got) != 1 {
		t.Fatalf("len(findings) = %d, want 1 (%#v)", len(got), got)
	}

	assertFindingMetadata(t, got[0], rule, "templates/security.yaml", "Deployment/cap-gap")
	if got[0].Description != "init container \"bootstrap\" in Deployment/cap-gap does not drop all Linux capabilities." {
		t.Fatalf("got[0].Description = %q", got[0].Description)
	}
	if got[0].Remediation != "Set init container \"bootstrap\" securityContext.capabilities.drop to include \"ALL\"." {
		t.Fatalf("got[0].Remediation = %q", got[0].Remediation)
	}

	for _, chartPath := range []string{fixtureChartPath(), nginxIngressChartPath()} {
		ctx := renderedAnalysisContext(t, chartPath)
		if findings := findingsForRule(ctx, rule.ID()); len(findings) != 0 {
			t.Fatalf("SEC002 findings for %s = %#v, want none", chartPath, findings)
		}
	}
}

func TestReadOnlyRootFilesystemRule(t *testing.T) {
	rule := findSecurityRule(t, "SEC003")
	if rule.Title() != "Container root filesystem is writable" {
		t.Fatalf("rule.Title() = %q", rule.Title())
	}
	if rule.Category() != models.CategorySecurity {
		t.Fatalf("rule.Category() = %q, want %q", rule.Category(), models.CategorySecurity)
	}
	if rule.Severity() != models.SeverityError {
		t.Fatalf("rule.Severity() = %v, want %v", rule.Severity(), models.SeverityError)
	}

	ctx := rules.AnalysisContext{
		RenderedResources: map[string][]models.K8sResource{
			"templates/security.yaml": {
				makeDeployment(
					"rootfs-gap",
					nil,
					[]map[string]any{{
						"name": "app",
						"securityContext": map[string]any{
							"allowPrivilegeEscalation": false,
							"readOnlyRootFilesystem":   false,
							"capabilities": map[string]any{
								"drop": []any{"ALL"},
							},
						},
					}},
				),
			},
		},
	}

	got := findingsForRule(ctx, rule.ID())
	if len(got) != 1 {
		t.Fatalf("len(findings) = %d, want 1 (%#v)", len(got), got)
	}

	assertFindingMetadata(t, got[0], rule, "templates/security.yaml", "Deployment/rootfs-gap")
	if got[0].Description != "container \"app\" in Deployment/rootfs-gap does not set readOnlyRootFilesystem: true." {
		t.Fatalf("got[0].Description = %q", got[0].Description)
	}
	if got[0].Remediation != "Set container \"app\" securityContext.readOnlyRootFilesystem to true." {
		t.Fatalf("got[0].Remediation = %q", got[0].Remediation)
	}

	secureCtx := renderedAnalysisContext(t, fixtureChartPath())
	if findings := findingsForRule(secureCtx, rule.ID()); len(findings) != 0 {
		t.Fatalf("SEC003 findings for secure fixture = %#v, want none", findings)
	}

	nginxCtx := renderedAnalysisContext(t, nginxIngressChartPath())
	findings := findingsForRule(nginxCtx, rule.ID())
	if len(findings) == 0 {
		t.Fatal("SEC003 findings for nginx-ingress = 0, want at least controller deployment")
	}

	var controllerFinding *models.Finding
	for index := range findings {
		finding := &findings[index]
		if finding.Path == "templates/controller-deployment.yaml" && finding.Resource == "Deployment/helmdoc-ingress-nginx-controller" {
			controllerFinding = finding
			break
		}
	}
	if controllerFinding == nil {
		t.Fatalf("SEC003 did not flag controller deployment: %#v", findings)
	}
	assertFindingMetadata(t, *controllerFinding, rule, "templates/controller-deployment.yaml", "Deployment/helmdoc-ingress-nginx-controller")
	if controllerFinding.Description != "container \"controller\" in Deployment/helmdoc-ingress-nginx-controller does not set readOnlyRootFilesystem: true." {
		t.Fatalf("controllerFinding.Description = %q", controllerFinding.Description)
	}
	if controllerFinding.Remediation != "Set container \"controller\" securityContext.readOnlyRootFilesystem to true." {
		t.Fatalf("controllerFinding.Remediation = %q", controllerFinding.Remediation)
	}
}

func findSecurityRule(t *testing.T, ruleID string) rules.Rule {
	t.Helper()
	for _, rule := range rules.ByCategory(models.CategorySecurity) {
		if rule.ID() == ruleID {
			return rule
		}
	}
	t.Fatalf("security rule %q not registered", ruleID)
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

func assertFindingMetadata(t *testing.T, finding models.Finding, rule rules.Rule, wantPath, wantResource string) {
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

func fixtureChartPath() string {
	return filepath.Join("..", "..", "..", "testdata", "testchart")
}

func nginxIngressChartPath() string {
	return filepath.Join("..", "..", "..", "testdata", "nginx-ingress")
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

func toAnySlice(items []map[string]any) []any {
	result := make([]any, 0, len(items))
	for _, item := range items {
		result = append(result, item)
	}
	return result
}
