package ingress

import (
	"testing"

	"github.com/belyaev-dev/helmdoc/internal/rules"
	"github.com/belyaev-dev/helmdoc/pkg/models"
)

func TestIngressTLSRule(t *testing.T) {
	rule := findIngressRule(t, "ING001")

	if rule.Title() != "Ingress TLS is not configured" {
		t.Fatalf("rule.Title() = %q", rule.Title())
	}
	if rule.Category() != models.CategoryIngress {
		t.Fatalf("rule.Category() = %q, want %q", rule.Category(), models.CategoryIngress)
	}
	if rule.Severity() != models.SeverityWarning {
		t.Fatalf("rule.Severity() = %v, want %v", rule.Severity(), models.SeverityWarning)
	}

	t.Run("rendered_ingress_without_tls_is_flagged", func(t *testing.T) {
		ctx := rules.AnalysisContext{RenderedResources: map[string][]models.K8sResource{
			"templates/ingress.yaml": {makeIngress("web", false)},
		}}

		findings := ingressFindingsForRule(ctx, rule.ID())
		if len(findings) != 1 {
			t.Fatalf("len(findings) = %d, want 1 (%#v)", len(findings), findings)
		}
		assertIngressFindingCoreMetadata(t, findings[0], rule)
		if findings[0].Description != "Ingress/web does not define spec.tls." {
			t.Fatalf("finding.Description = %q", findings[0].Description)
		}
	})

	t.Run("ingress_with_tls_passes", func(t *testing.T) {
		ctx := rules.AnalysisContext{RenderedResources: map[string][]models.K8sResource{
			"templates/ingress.yaml": {makeIngress("web", true)},
		}}
		if findings := ingressFindingsForRule(ctx, rule.ID()); len(findings) != 0 {
			t.Fatalf("findings = %#v, want none", findings)
		}
	})

	t.Run("charts_without_ingress_resources_are_ignored", func(t *testing.T) {
		ctx := rules.AnalysisContext{RenderedResources: map[string][]models.K8sResource{
			"templates/service.yaml": {makeService("web")},
		}}
		if findings := ingressFindingsForRule(ctx, rule.ID()); len(findings) != 0 {
			t.Fatalf("findings = %#v, want none", findings)
		}
	})
}

func findIngressRule(t *testing.T, ruleID string) rules.Rule {
	t.Helper()
	for _, rule := range rules.ByCategory(models.CategoryIngress) {
		if rule.ID() == ruleID {
			return rule
		}
	}
	t.Fatalf("ingress rule %q not registered", ruleID)
	return nil
}

func ingressFindingsForRule(ctx rules.AnalysisContext, ruleID string) []models.Finding {
	allFindings := rules.RunAll(ctx)
	findings := make([]models.Finding, 0)
	for _, finding := range allFindings {
		if finding.RuleID == ruleID {
			findings = append(findings, finding)
		}
	}
	return findings
}

func assertIngressFindingCoreMetadata(t *testing.T, finding models.Finding, rule rules.Rule) {
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

func makeIngress(name string, withTLS bool) models.K8sResource {
	spec := map[string]any{
		"rules": []any{map[string]any{"host": "example.com"}},
	}
	if withTLS {
		spec["tls"] = []any{map[string]any{"secretName": "example-tls"}}
	}
	return models.K8sResource{
		APIVersion: "networking.k8s.io/v1",
		Kind:       "Ingress",
		Name:       name,
		Raw: map[string]any{
			"apiVersion": "networking.k8s.io/v1",
			"kind":       "Ingress",
			"metadata":   map[string]any{"name": name},
			"spec":       spec,
		},
	}
}

func makeService(name string) models.K8sResource {
	return models.K8sResource{
		APIVersion: "v1",
		Kind:       "Service",
		Name:       name,
		Raw: map[string]any{
			"apiVersion": "v1",
			"kind":       "Service",
			"metadata":   map[string]any{"name": name},
		},
	}
}
