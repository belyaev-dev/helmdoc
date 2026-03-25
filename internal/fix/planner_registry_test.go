package fix

import (
	"testing"

	"github.com/belyaev-dev/helmdoc/internal/rules"
	"github.com/belyaev-dev/helmdoc/pkg/models"
	helmchart "helm.sh/helm/v3/pkg/chart"
)

func TestPlanBundleUsesRegistryBeforeSuffixFallback(t *testing.T) {
	t.Run("registry candidate wins when suffix fallback could also match", func(t *testing.T) {
		ctx := analysisContextWithChartName("ingress-nginx", map[string]models.ValuePath{
			"controller.autoscaling.minReplicas": {Default: 1, Type: "number"},
			"controller.autoscaling.maxReplicas": {Default: 3, Type: "number"},
			"shared.autoscaling":                 {Default: map[string]any{"enabled": false}, Type: "object"},
			"shared.autoscaling.minReplicas":     {Default: 1, Type: "number"},
			"shared.autoscaling.maxReplicas":     {Default: 4, Type: "number"},
		})
		finding := models.Finding{
			RuleID:   "SCL001",
			Path:     "templates/controller-deployment.yaml",
			Resource: "Deployment/helmdoc-ingress-nginx-controller",
		}

		plan := PlanBundle(ctx, []models.Finding{finding})
		if got := len(plan.AppliedValuesFixes); got != 1 {
			t.Fatalf("chart=%q rule=%s template=%s applied fixes = %d, want 1", analysisChartName(ctx), finding.RuleID, finding.Path, got)
		}
		if got := len(plan.PendingFindings); got != 0 {
			t.Fatalf("chart=%q rule=%s template=%s pending findings = %d, want 0", analysisChartName(ctx), finding.RuleID, finding.Path, got)
		}

		fix := plan.AppliedValuesFixes[0]
		if got, want := fix.ValuesPath, "controller.autoscaling"; got != want {
			t.Fatalf("chart=%q rule=%s template=%s selected values path = %q, want %q before suffix fallback", analysisChartName(ctx), finding.RuleID, finding.Path, got, want)
		}
		value, ok := fix.Value.(map[string]any)
		if !ok {
			t.Fatalf("chart=%q rule=%s template=%s payload = %#v, want autoscaling map", analysisChartName(ctx), finding.RuleID, finding.Path, fix.Value)
		}
		if got, _ := value["enabled"].(bool); !got {
			t.Fatalf("chart=%q rule=%s template=%s payload.enabled = %#v, want true", analysisChartName(ctx), finding.RuleID, finding.Path, value["enabled"])
		}
	})

	t.Run("missing registry mapping falls back to suffix routing", func(t *testing.T) {
		ctx := analysisContextWithChartName("not-curated", map[string]models.ValuePath{
			"shared.autoscaling":             {Default: map[string]any{"enabled": false}, Type: "object"},
			"shared.autoscaling.minReplicas": {Default: 1, Type: "number"},
			"shared.autoscaling.maxReplicas": {Default: 2, Type: "number"},
		})
		finding := models.Finding{
			RuleID:   "SCL001",
			Path:     "templates/deployment.yaml",
			Resource: "Deployment/example",
		}

		plan := PlanBundle(ctx, []models.Finding{finding})
		if got := len(plan.AppliedValuesFixes); got != 1 {
			t.Fatalf("chart=%q rule=%s template=%s applied fixes = %d, want 1 via suffix fallback", analysisChartName(ctx), finding.RuleID, finding.Path, got)
		}
		if got := len(plan.PendingFindings); got != 0 {
			t.Fatalf("chart=%q rule=%s template=%s pending findings = %d, want 0 after suffix fallback", analysisChartName(ctx), finding.RuleID, finding.Path, got)
		}
		if got, want := plan.AppliedValuesFixes[0].ValuesPath, "shared.autoscaling"; got != want {
			t.Fatalf("chart=%q rule=%s template=%s selected values path = %q, want suffix fallback path %q", analysisChartName(ctx), finding.RuleID, finding.Path, got, want)
		}
	})
}

func analysisContextWithChartName(chartName string, values map[string]models.ValuePath) rules.AnalysisContext {
	return rules.AnalysisContext{
		Chart:         &helmchart.Chart{Metadata: &helmchart.Metadata{Name: chartName}},
		ValuesSurface: models.NewValuesSurface(values),
	}
}
