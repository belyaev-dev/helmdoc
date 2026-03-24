package network

import (
	"testing"

	"github.com/belyaev-dev/helmdoc/internal/rules"
	"github.com/belyaev-dev/helmdoc/pkg/models"
)

func TestNetworkPolicyRule(t *testing.T) {
	rule := findNetworkRule(t, "NET001")

	if rule.Title() != "Workload namespace has no NetworkPolicy" {
		t.Fatalf("rule.Title() = %q", rule.Title())
	}
	if rule.Category() != models.CategoryNetwork {
		t.Fatalf("rule.Category() = %q, want %q", rule.Category(), models.CategoryNetwork)
	}
	if rule.Severity() != models.SeverityWarning {
		t.Fatalf("rule.Severity() = %v, want %v", rule.Severity(), models.SeverityWarning)
	}

	t.Run("deployment_without_namespace_networkpolicy_is_flagged", func(t *testing.T) {
		ctx := rules.AnalysisContext{RenderedResources: map[string][]models.K8sResource{
			"templates/deployment.yaml": {makeNetworkDeployment("api", "prod")},
		}}

		findings := networkFindingsForRule(ctx, rule.ID())
		if len(findings) != 1 {
			t.Fatalf("len(findings) = %d, want 1 (%#v)", len(findings), findings)
		}
		assertNetworkFindingCoreMetadata(t, findings[0], rule)
		if findings[0].Description != "Deployment/api is rendered in namespace \"prod\", but no NetworkPolicy is rendered for that namespace." {
			t.Fatalf("finding.Description = %q", findings[0].Description)
		}
		if findings[0].Remediation != "Add a NetworkPolicy manifest for namespace \"prod\" so Deployment/api is covered." {
			t.Fatalf("finding.Remediation = %q", findings[0].Remediation)
		}
	})

	t.Run("networkpolicy_in_same_namespace_suppresses_findings", func(t *testing.T) {
		ctx := rules.AnalysisContext{RenderedResources: map[string][]models.K8sResource{
			"templates/deployment.yaml":    {makeNetworkDeployment("api", "prod")},
			"templates/networkpolicy.yaml": {makeNetworkPolicy("allow-api", "prod")},
		}}

		if findings := networkFindingsForRule(ctx, rule.ID()); len(findings) != 0 {
			t.Fatalf("findings = %#v, want none", findings)
		}
	})

	t.Run("values_surface_changes_remediation_and_ephemeral_workloads_are_ignored", func(t *testing.T) {
		ctx := rules.AnalysisContext{
			RenderedResources: map[string][]models.K8sResource{
				"templates/statefulset.yaml": {makeNetworkStatefulSet("db", "prod")},
				"templates/job.yaml":         {makeNetworkJob("migrate", "prod")},
			},
			ValuesSurface: models.NewValuesSurface(map[string]models.ValuePath{
				"controller.networkPolicy.enabled": {Default: false, Type: "bool"},
			}),
		}

		findings := networkFindingsForRule(ctx, rule.ID())
		if len(findings) != 1 {
			t.Fatalf("len(findings) = %d, want 1 (%#v)", len(findings), findings)
		}
		if findings[0].Resource != "StatefulSet/db" {
			t.Fatalf("finding.Resource = %q, want StatefulSet/db", findings[0].Resource)
		}
		if findings[0].Remediation != "Enable a chart networkPolicy setting in values.yaml so namespace \"prod\" renders at least one NetworkPolicy for StatefulSet/db." {
			t.Fatalf("finding.Remediation = %q", findings[0].Remediation)
		}
	})
}

func findNetworkRule(t *testing.T, ruleID string) rules.Rule {
	t.Helper()
	for _, rule := range rules.ByCategory(models.CategoryNetwork) {
		if rule.ID() == ruleID {
			return rule
		}
	}
	t.Fatalf("network rule %q not registered", ruleID)
	return nil
}

func networkFindingsForRule(ctx rules.AnalysisContext, ruleID string) []models.Finding {
	allFindings := rules.RunAll(ctx)
	findings := make([]models.Finding, 0)
	for _, finding := range allFindings {
		if finding.RuleID == ruleID {
			findings = append(findings, finding)
		}
	}
	return findings
}

func assertNetworkFindingCoreMetadata(t *testing.T, finding models.Finding, rule rules.Rule) {
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

func makeNetworkDeployment(name, namespace string) models.K8sResource {
	return models.K8sResource{
		APIVersion: "apps/v1",
		Kind:       "Deployment",
		Name:       name,
		Namespace:  namespace,
		Raw: map[string]any{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata":   map[string]any{"name": name, "namespace": namespace},
		},
	}
}

func makeNetworkStatefulSet(name, namespace string) models.K8sResource {
	return models.K8sResource{
		APIVersion: "apps/v1",
		Kind:       "StatefulSet",
		Name:       name,
		Namespace:  namespace,
		Raw: map[string]any{
			"apiVersion": "apps/v1",
			"kind":       "StatefulSet",
			"metadata":   map[string]any{"name": name, "namespace": namespace},
		},
	}
}

func makeNetworkJob(name, namespace string) models.K8sResource {
	return models.K8sResource{
		APIVersion: "batch/v1",
		Kind:       "Job",
		Name:       name,
		Namespace:  namespace,
		Raw: map[string]any{
			"apiVersion": "batch/v1",
			"kind":       "Job",
			"metadata":   map[string]any{"name": name, "namespace": namespace},
		},
	}
}

func makeNetworkPolicy(name, namespace string) models.K8sResource {
	return models.K8sResource{
		APIVersion: "networking.k8s.io/v1",
		Kind:       "NetworkPolicy",
		Name:       name,
		Namespace:  namespace,
		Raw: map[string]any{
			"apiVersion": "networking.k8s.io/v1",
			"kind":       "NetworkPolicy",
			"metadata":   map[string]any{"name": name, "namespace": namespace},
		},
	}
}
