package availability

import (
	"testing"

	"github.com/belyaev-dev/helmdoc/internal/rules"
	"github.com/belyaev-dev/helmdoc/pkg/models"
)

func TestPodDisruptionBudgetRule(t *testing.T) {
	rule := findAvailabilityRule(t, "AVL001")

	if rule.Title() != "Workload has no matching PodDisruptionBudget" {
		t.Fatalf("rule.Title() = %q", rule.Title())
	}
	if rule.Category() != models.CategoryAvailability {
		t.Fatalf("rule.Category() = %q, want %q", rule.Category(), models.CategoryAvailability)
	}
	if rule.Severity() != models.SeverityWarning {
		t.Fatalf("rule.Severity() = %v, want %v", rule.Severity(), models.SeverityWarning)
	}

	t.Run("deployment_without_matching_pdb_is_flagged", func(t *testing.T) {
		ctx := rules.AnalysisContext{RenderedResources: map[string][]models.K8sResource{
			"templates/deployment.yaml": {makeAvailabilityDeployment("api", "prod", map[string]string{"app": "api"})},
		}}

		findings := availabilityFindingsForRule(ctx, rule.ID())
		if len(findings) != 1 {
			t.Fatalf("len(findings) = %d, want 1 (%#v)", len(findings), findings)
		}
		assertAvailabilityFindingCoreMetadata(t, findings[0], rule)
		if findings[0].Description != "Deployment/api has no matching PodDisruptionBudget in namespace \"prod\"." {
			t.Fatalf("finding.Description = %q", findings[0].Description)
		}
	})

	t.Run("matching_pdb_suppresses_findings", func(t *testing.T) {
		ctx := rules.AnalysisContext{RenderedResources: map[string][]models.K8sResource{
			"templates/deployment.yaml": {makeAvailabilityDeployment("api", "prod", map[string]string{"app": "api", "tier": "web"})},
			"templates/pdb.yaml":        {makePodDisruptionBudget("api", "prod", map[string]string{"app": "api"})},
		}}

		if findings := availabilityFindingsForRule(ctx, rule.ID()); len(findings) != 0 {
			t.Fatalf("findings = %#v, want none", findings)
		}
	})

	t.Run("unsupported_workloads_are_ignored", func(t *testing.T) {
		ctx := rules.AnalysisContext{RenderedResources: map[string][]models.K8sResource{
			"templates/job.yaml":       {makeAvailabilityJob("migrate", "prod")},
			"templates/daemonset.yaml": {makeAvailabilityDaemonSet("agent", "prod", map[string]string{"app": "agent"})},
		}}

		if findings := availabilityFindingsForRule(ctx, rule.ID()); len(findings) != 0 {
			t.Fatalf("findings = %#v, want none", findings)
		}
	})
}

func findAvailabilityRule(t *testing.T, ruleID string) rules.Rule {
	t.Helper()
	for _, rule := range rules.ByCategory(models.CategoryAvailability) {
		if rule.ID() == ruleID {
			return rule
		}
	}
	t.Fatalf("availability rule %q not registered", ruleID)
	return nil
}

func availabilityFindingsForRule(ctx rules.AnalysisContext, ruleID string) []models.Finding {
	allFindings := rules.RunAll(ctx)
	findings := make([]models.Finding, 0)
	for _, finding := range allFindings {
		if finding.RuleID == ruleID {
			findings = append(findings, finding)
		}
	}
	return findings
}

func assertAvailabilityFindingCoreMetadata(t *testing.T, finding models.Finding, rule rules.Rule) {
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

func makeAvailabilityDeployment(name, namespace string, labels map[string]string) models.K8sResource {
	return models.K8sResource{
		APIVersion: "apps/v1",
		Kind:       "Deployment",
		Name:       name,
		Namespace:  namespace,
		Raw: map[string]any{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata":   map[string]any{"name": name, "namespace": namespace},
			"spec": map[string]any{
				"template": map[string]any{
					"metadata": map[string]any{"labels": stringMapToAny(labels)},
				},
			},
		},
	}
}

func makeAvailabilityDaemonSet(name, namespace string, labels map[string]string) models.K8sResource {
	return models.K8sResource{
		APIVersion: "apps/v1",
		Kind:       "DaemonSet",
		Name:       name,
		Namespace:  namespace,
		Raw: map[string]any{
			"apiVersion": "apps/v1",
			"kind":       "DaemonSet",
			"metadata":   map[string]any{"name": name, "namespace": namespace},
			"spec": map[string]any{
				"template": map[string]any{
					"metadata": map[string]any{"labels": stringMapToAny(labels)},
				},
			},
		},
	}
}

func makeAvailabilityJob(name, namespace string) models.K8sResource {
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

func makePodDisruptionBudget(name, namespace string, selector map[string]string) models.K8sResource {
	return models.K8sResource{
		APIVersion: "policy/v1",
		Kind:       "PodDisruptionBudget",
		Name:       name,
		Namespace:  namespace,
		Raw: map[string]any{
			"apiVersion": "policy/v1",
			"kind":       "PodDisruptionBudget",
			"metadata":   map[string]any{"name": name, "namespace": namespace},
			"spec": map[string]any{
				"selector": map[string]any{"matchLabels": stringMapToAny(selector)},
			},
		},
	}
}

func stringMapToAny(values map[string]string) map[string]any {
	result := make(map[string]any, len(values))
	for key, value := range values {
		result[key] = value
	}
	return result
}
