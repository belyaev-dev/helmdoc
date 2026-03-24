package scaling

import (
	"testing"

	"github.com/belyaev-dev/helmdoc/internal/rules"
	"github.com/belyaev-dev/helmdoc/pkg/models"
)

func TestHPARule(t *testing.T) {
	rule := findScalingRule(t, "SCL001")

	if rule.Title() != "Workload has no matching HorizontalPodAutoscaler" {
		t.Fatalf("rule.Title() = %q", rule.Title())
	}
	if rule.Category() != models.CategoryScaling {
		t.Fatalf("rule.Category() = %q, want %q", rule.Category(), models.CategoryScaling)
	}
	if rule.Severity() != models.SeverityWarning {
		t.Fatalf("rule.Severity() = %v, want %v", rule.Severity(), models.SeverityWarning)
	}

	t.Run("deployment_without_matching_hpa_is_flagged", func(t *testing.T) {
		ctx := rules.AnalysisContext{RenderedResources: map[string][]models.K8sResource{
			"templates/deployment.yaml": {makeScalingDeployment("api", "prod")},
		}}

		findings := scalingFindingsForRule(ctx, rule.ID())
		if len(findings) != 1 {
			t.Fatalf("len(findings) = %d, want 1 (%#v)", len(findings), findings)
		}
		assertScalingFindingCoreMetadata(t, findings[0], rule)
		if findings[0].Description != "Deployment/api has no matching HorizontalPodAutoscaler in namespace \"prod\"." {
			t.Fatalf("finding.Description = %q", findings[0].Description)
		}
	})

	t.Run("matching_hpa_suppresses_findings", func(t *testing.T) {
		ctx := rules.AnalysisContext{RenderedResources: map[string][]models.K8sResource{
			"templates/statefulset.yaml": {makeScalingStatefulSet("db", "prod")},
			"templates/hpa.yaml":         {makeHorizontalPodAutoscaler("db", "prod", "StatefulSet", "db")},
		}}

		if findings := scalingFindingsForRule(ctx, rule.ID()); len(findings) != 0 {
			t.Fatalf("findings = %#v, want none", findings)
		}
	})

	t.Run("unsupported_workloads_are_ignored", func(t *testing.T) {
		ctx := rules.AnalysisContext{RenderedResources: map[string][]models.K8sResource{
			"templates/job.yaml":       {makeScalingJob("migrate", "prod")},
			"templates/daemonset.yaml": {makeScalingDaemonSet("agent", "prod")},
		}}

		if findings := scalingFindingsForRule(ctx, rule.ID()); len(findings) != 0 {
			t.Fatalf("findings = %#v, want none", findings)
		}
	})
}

func findScalingRule(t *testing.T, ruleID string) rules.Rule {
	t.Helper()
	for _, rule := range rules.ByCategory(models.CategoryScaling) {
		if rule.ID() == ruleID {
			return rule
		}
	}
	t.Fatalf("scaling rule %q not registered", ruleID)
	return nil
}

func scalingFindingsForRule(ctx rules.AnalysisContext, ruleID string) []models.Finding {
	allFindings := rules.RunAll(ctx)
	findings := make([]models.Finding, 0)
	for _, finding := range allFindings {
		if finding.RuleID == ruleID {
			findings = append(findings, finding)
		}
	}
	return findings
}

func assertScalingFindingCoreMetadata(t *testing.T, finding models.Finding, rule rules.Rule) {
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

func makeScalingDeployment(name, namespace string) models.K8sResource {
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

func makeScalingStatefulSet(name, namespace string) models.K8sResource {
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

func makeScalingDaemonSet(name, namespace string) models.K8sResource {
	return models.K8sResource{
		APIVersion: "apps/v1",
		Kind:       "DaemonSet",
		Name:       name,
		Namespace:  namespace,
		Raw: map[string]any{
			"apiVersion": "apps/v1",
			"kind":       "DaemonSet",
			"metadata":   map[string]any{"name": name, "namespace": namespace},
		},
	}
}

func makeScalingJob(name, namespace string) models.K8sResource {
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

func makeHorizontalPodAutoscaler(name, namespace, targetKind, targetName string) models.K8sResource {
	return models.K8sResource{
		APIVersion: "autoscaling/v2",
		Kind:       "HorizontalPodAutoscaler",
		Name:       name,
		Namespace:  namespace,
		Raw: map[string]any{
			"apiVersion": "autoscaling/v2",
			"kind":       "HorizontalPodAutoscaler",
			"metadata":   map[string]any{"name": name, "namespace": namespace},
			"spec": map[string]any{
				"scaleTargetRef": map[string]any{"kind": targetKind, "name": targetName},
			},
		},
	}
}
