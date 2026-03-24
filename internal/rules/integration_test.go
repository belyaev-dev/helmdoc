package rules_test

import (
	"path/filepath"
	"reflect"
	"testing"

	chartanalysis "github.com/belyaev-dev/helmdoc/internal/chart"
	"github.com/belyaev-dev/helmdoc/internal/rules"
	_ "github.com/belyaev-dev/helmdoc/internal/rules/all"
	"github.com/belyaev-dev/helmdoc/pkg/models"
)

func TestRunAllAgainstNginxIngress(t *testing.T) {
	ctx := renderedAnalysisContext(t, filepath.Join("..", "..", "testdata", "nginx-ingress"))

	findings := rules.RunAll(ctx)
	if len(findings) != 13 {
		t.Fatalf("len(findings) = %d, want 13 (%#v)", len(findings), findings)
	}

	wantFindings := []findingKey{
		{RuleID: "SEC003", Category: models.CategorySecurity, Severity: models.SeverityError, Path: "templates/controller-deployment.yaml", Resource: "Deployment/helmdoc-ingress-nginx-controller"},
		{RuleID: "RES001", Category: models.CategoryResources, Severity: models.SeverityWarning, Path: "templates/admission-webhooks/job-patch/job-createSecret.yaml", Resource: "Job/helmdoc-ingress-nginx-admission-create"},
		{RuleID: "RES001", Category: models.CategoryResources, Severity: models.SeverityWarning, Path: "templates/admission-webhooks/job-patch/job-patchWebhook.yaml", Resource: "Job/helmdoc-ingress-nginx-admission-patch"},
		{RuleID: "RES001", Category: models.CategoryResources, Severity: models.SeverityWarning, Path: "templates/controller-deployment.yaml", Resource: "Deployment/helmdoc-ingress-nginx-controller"},
		{RuleID: "RES002", Category: models.CategoryResources, Severity: models.SeverityWarning, Path: "templates/admission-webhooks/job-patch/job-createSecret.yaml", Resource: "Job/helmdoc-ingress-nginx-admission-create"},
		{RuleID: "RES002", Category: models.CategoryResources, Severity: models.SeverityWarning, Path: "templates/admission-webhooks/job-patch/job-patchWebhook.yaml", Resource: "Job/helmdoc-ingress-nginx-admission-patch"},
		{RuleID: "HLT001", Category: models.CategoryHealth, Severity: models.SeverityWarning, Path: "templates/admission-webhooks/job-patch/job-createSecret.yaml", Resource: "Job/helmdoc-ingress-nginx-admission-create"},
		{RuleID: "HLT001", Category: models.CategoryHealth, Severity: models.SeverityWarning, Path: "templates/admission-webhooks/job-patch/job-patchWebhook.yaml", Resource: "Job/helmdoc-ingress-nginx-admission-patch"},
		{RuleID: "HLT002", Category: models.CategoryHealth, Severity: models.SeverityWarning, Path: "templates/admission-webhooks/job-patch/job-createSecret.yaml", Resource: "Job/helmdoc-ingress-nginx-admission-create"},
		{RuleID: "HLT002", Category: models.CategoryHealth, Severity: models.SeverityWarning, Path: "templates/admission-webhooks/job-patch/job-patchWebhook.yaml", Resource: "Job/helmdoc-ingress-nginx-admission-patch"},
		{RuleID: "AVL001", Category: models.CategoryAvailability, Severity: models.SeverityWarning, Path: "templates/controller-deployment.yaml", Resource: "Deployment/helmdoc-ingress-nginx-controller"},
		{RuleID: "NET001", Category: models.CategoryNetwork, Severity: models.SeverityWarning, Path: "templates/controller-deployment.yaml", Resource: "Deployment/helmdoc-ingress-nginx-controller"},
		{RuleID: "SCL001", Category: models.CategoryScaling, Severity: models.SeverityWarning, Path: "templates/controller-deployment.yaml", Resource: "Deployment/helmdoc-ingress-nginx-controller"},
	}

	gotFindings := make([]findingKey, 0, len(findings))
	categoryCounts := map[models.Category]int{}
	for _, finding := range findings {
		gotFindings = append(gotFindings, findingKey{
			RuleID:   finding.RuleID,
			Category: finding.Category,
			Severity: finding.Severity,
			Path:     finding.Path,
			Resource: finding.Resource,
		})
		categoryCounts[finding.Category]++
	}

	if !reflect.DeepEqual(gotFindings, wantFindings) {
		t.Fatalf("finding tuples = %#v, want %#v", gotFindings, wantFindings)
	}

	wantCategoryCounts := map[models.Category]int{
		models.CategorySecurity:     1,
		models.CategoryResources:    5,
		models.CategoryStorage:      0,
		models.CategoryHealth:       4,
		models.CategoryAvailability: 1,
		models.CategoryNetwork:      1,
		models.CategoryImages:       0,
		models.CategoryIngress:      0,
		models.CategoryScaling:      1,
		models.CategoryConfig:       0,
	}
	for _, category := range models.AllCategories() {
		if got, want := categoryCounts[category], wantCategoryCounts[category]; got != want {
			t.Fatalf("categoryCounts[%q] = %d, want %d (all=%#v)", category, got, want, categoryCounts)
		}
	}

	controllerSecurity := requireFinding(t, findings, findingKey{RuleID: "SEC003", Path: "templates/controller-deployment.yaml", Resource: "Deployment/helmdoc-ingress-nginx-controller"})
	if controllerSecurity.Category != models.CategorySecurity {
		t.Fatalf("controllerSecurity.Category = %q, want %q", controllerSecurity.Category, models.CategorySecurity)
	}
	if controllerSecurity.Severity != models.SeverityError {
		t.Fatalf("controllerSecurity.Severity = %v, want %v", controllerSecurity.Severity, models.SeverityError)
	}
	if controllerSecurity.Title != "Container root filesystem is writable" {
		t.Fatalf("controllerSecurity.Title = %q", controllerSecurity.Title)
	}

	controllerLimits := requireFinding(t, findings, findingKey{RuleID: "RES001", Path: "templates/controller-deployment.yaml", Resource: "Deployment/helmdoc-ingress-nginx-controller"})
	if controllerLimits.Remediation != "Set controller.resources.limits in values.yaml. The chart already exposes controller.resources, but its default leaves limits empty." {
		t.Fatalf("controllerLimits.Remediation = %q", controllerLimits.Remediation)
	}

	createSecretRequests := requireFinding(t, findings, findingKey{RuleID: "RES002", Path: "templates/admission-webhooks/job-patch/job-createSecret.yaml", Resource: "Job/helmdoc-ingress-nginx-admission-create"})
	if createSecretRequests.Remediation != "Set controller.admissionWebhooks.createSecretJob.resources.requests in values.yaml. The chart already exposes controller.admissionWebhooks.createSecretJob.resources, but its default leaves requests empty." {
		t.Fatalf("createSecretRequests.Remediation = %q", createSecretRequests.Remediation)
	}

	patchWebhookReadiness := requireFinding(t, findings, findingKey{RuleID: "HLT002", Path: "templates/admission-webhooks/job-patch/job-patchWebhook.yaml", Resource: "Job/helmdoc-ingress-nginx-admission-patch"})
	if patchWebhookReadiness.Remediation != "Add readinessProbe directly to the template for container \"patch\" or expose controller.admissionWebhooks.patchWebhookJob.readinessProbe in values.yaml." {
		t.Fatalf("patchWebhookReadiness.Remediation = %q", patchWebhookReadiness.Remediation)
	}
}

type findingKey struct {
	RuleID   string
	Category models.Category
	Severity models.Severity
	Path     string
	Resource string
}

func requireFinding(t *testing.T, findings []models.Finding, want findingKey) models.Finding {
	t.Helper()
	for _, finding := range findings {
		if finding.RuleID == want.RuleID && finding.Path == want.Path && finding.Resource == want.Resource {
			return finding
		}
	}
	t.Fatalf("finding %#v missing from %#v", want, findings)
	return models.Finding{}
}

func renderedAnalysisContext(t *testing.T, chartPath string) rules.AnalysisContext {
	t.Helper()

	loadedChart, err := chartanalysis.LoadChart(chartPath)
	if err != nil {
		t.Fatalf("LoadChart(%q) error = %v", chartPath, err)
	}

	rendered, err := chartanalysis.RenderChart(loadedChart)
	if err != nil {
		t.Fatalf("RenderChart(%q) error = %v", chartPath, err)
	}

	return rules.AnalysisContext{
		Chart:             loadedChart,
		RenderedResources: rendered,
		ValuesSurface:     chartanalysis.AnalyzeValues(loadedChart),
	}
}
