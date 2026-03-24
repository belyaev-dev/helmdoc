package storage

import (
	"testing"

	"github.com/belyaev-dev/helmdoc/internal/rules"
	"github.com/belyaev-dev/helmdoc/pkg/models"
)

func TestStorageClassRule(t *testing.T) {
	rule := findStorageRule(t, "STR001")

	if rule.Title() != "Persistent storage class is not configurable" {
		t.Fatalf("rule.Title() = %q", rule.Title())
	}
	if rule.Category() != models.CategoryStorage {
		t.Fatalf("rule.Category() = %q, want %q", rule.Category(), models.CategoryStorage)
	}
	if rule.Severity() != models.SeverityWarning {
		t.Fatalf("rule.Severity() = %v, want %v", rule.Severity(), models.SeverityWarning)
	}

	t.Run("rendered_pvc_without_storage_class_or_values_path_is_flagged", func(t *testing.T) {
		ctx := rules.AnalysisContext{RenderedResources: map[string][]models.K8sResource{
			"templates/pvc.yaml": {makePersistentVolumeClaim("data", "", false)},
		}}

		findings := storageFindingsForRule(ctx, rule.ID())
		if len(findings) != 1 {
			t.Fatalf("len(findings) = %d, want 1 (%#v)", len(findings), findings)
		}
		assertStorageFindingCoreMetadata(t, findings[0], rule)
		if findings[0].Description != "PersistentVolumeClaim/data does not set spec.storageClassName and the chart does not expose a storageClass values path." {
			t.Fatalf("finding.Description = %q", findings[0].Description)
		}
		if findings[0].Remediation != "Expose a values.yaml path for storageClassName or set spec.storageClassName directly in the template for PersistentVolumeClaim/data." {
			t.Fatalf("finding.Remediation = %q", findings[0].Remediation)
		}
	})

	t.Run("explicit_storage_class_or_values_knob_suppresses_findings", func(t *testing.T) {
		withStorageClass := rules.AnalysisContext{RenderedResources: map[string][]models.K8sResource{
			"templates/pvc.yaml": {makePersistentVolumeClaim("data", "fast-ssd", true)},
		}}
		if findings := storageFindingsForRule(withStorageClass, rule.ID()); len(findings) != 0 {
			t.Fatalf("explicit storageClass findings = %#v, want none", findings)
		}

		withValuesPath := rules.AnalysisContext{
			RenderedResources: map[string][]models.K8sResource{
				"templates/pvc.yaml": {makePersistentVolumeClaim("data", "", false)},
			},
			ValuesSurface: models.NewValuesSurface(map[string]models.ValuePath{
				"persistence.storageClassName": {Default: "", Type: "string"},
			}),
		}
		if findings := storageFindingsForRule(withValuesPath, rule.ID()); len(findings) != 0 {
			t.Fatalf("values-surface findings = %#v, want none", findings)
		}
	})

	t.Run("statefulset_volume_claim_templates_are_checked", func(t *testing.T) {
		ctx := rules.AnalysisContext{RenderedResources: map[string][]models.K8sResource{
			"templates/statefulset.yaml": {makeStatefulSetWithVolumeClaims("db", []map[string]any{{
				"metadata": map[string]any{"name": "data"},
				"spec":     map[string]any{"accessModes": []any{"ReadWriteOnce"}},
			}})},
		}}

		findings := storageFindingsForRule(ctx, rule.ID())
		if len(findings) != 1 {
			t.Fatalf("len(findings) = %d, want 1 (%#v)", len(findings), findings)
		}
		if findings[0].Resource != "StatefulSet/db" {
			t.Fatalf("finding.Resource = %q, want StatefulSet/db", findings[0].Resource)
		}
		if findings[0].Description != "volumeClaimTemplate \"data\" in StatefulSet/db does not set spec.storageClassName and the chart does not expose a storageClass values path." {
			t.Fatalf("finding.Description = %q", findings[0].Description)
		}
	})

	t.Run("charts_without_rendered_pvcs_are_ignored", func(t *testing.T) {
		ctx := rules.AnalysisContext{RenderedResources: map[string][]models.K8sResource{
			"templates/configmap.yaml": {makeConfigMap("settings")},
		}}
		if findings := storageFindingsForRule(ctx, rule.ID()); len(findings) != 0 {
			t.Fatalf("findings = %#v, want none", findings)
		}
	})
}

func findStorageRule(t *testing.T, ruleID string) rules.Rule {
	t.Helper()
	for _, rule := range rules.ByCategory(models.CategoryStorage) {
		if rule.ID() == ruleID {
			return rule
		}
	}
	t.Fatalf("storage rule %q not registered", ruleID)
	return nil
}

func storageFindingsForRule(ctx rules.AnalysisContext, ruleID string) []models.Finding {
	allFindings := rules.RunAll(ctx)
	findings := make([]models.Finding, 0)
	for _, finding := range allFindings {
		if finding.RuleID == ruleID {
			findings = append(findings, finding)
		}
	}
	return findings
}

func assertStorageFindingCoreMetadata(t *testing.T, finding models.Finding, rule rules.Rule) {
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

func makePersistentVolumeClaim(name, storageClassName string, setStorageClass bool) models.K8sResource {
	spec := map[string]any{"accessModes": []any{"ReadWriteOnce"}}
	if setStorageClass {
		spec["storageClassName"] = storageClassName
	}
	return models.K8sResource{
		APIVersion: "v1",
		Kind:       "PersistentVolumeClaim",
		Name:       name,
		Raw: map[string]any{
			"apiVersion": "v1",
			"kind":       "PersistentVolumeClaim",
			"metadata":   map[string]any{"name": name},
			"spec":       spec,
		},
	}
}

func makeStatefulSetWithVolumeClaims(name string, claims []map[string]any) models.K8sResource {
	volumeClaimTemplates := make([]any, 0, len(claims))
	for _, claim := range claims {
		volumeClaimTemplates = append(volumeClaimTemplates, claim)
	}
	return models.K8sResource{
		APIVersion: "apps/v1",
		Kind:       "StatefulSet",
		Name:       name,
		Raw: map[string]any{
			"apiVersion": "apps/v1",
			"kind":       "StatefulSet",
			"metadata":   map[string]any{"name": name},
			"spec":       map[string]any{"volumeClaimTemplates": volumeClaimTemplates},
		},
	}
}

func makeConfigMap(name string) models.K8sResource {
	return models.K8sResource{
		APIVersion: "v1",
		Kind:       "ConfigMap",
		Name:       name,
		Raw: map[string]any{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata":   map[string]any{"name": name},
		},
	}
}
