package registry

import (
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/belyaev-dev/helmdoc/pkg/models"
)

func TestDefaultRegistryLoadsEmbeddedCharts(t *testing.T) {
	registry, err := Default()
	if err != nil {
		t.Fatalf("Default() error = %v", err)
	}

	chart, ok := registry.LookupChart("ingress-nginx")
	if !ok {
		t.Fatalf("LookupChart(%q) found = false, want true; loaded charts = %#v", "ingress-nginx", registry.ChartNames())
	}
	if chart.SourceFile != "charts/ingress-nginx.yaml" {
		t.Fatalf("chart.SourceFile = %q, want %q", chart.SourceFile, "charts/ingress-nginx.yaml")
	}
	if len(chart.Mappings) == 0 {
		t.Fatal("len(chart.Mappings) = 0, want extracted ingress-nginx mappings")
	}

	candidates := registry.LookupCandidatesForFinding("ingress-nginx", nil, models.Finding{
		RuleID: "RES001",
		Path:   "templates/admission-webhooks/job-patch/job-createSecret.yaml",
	})
	if len(candidates) != 1 {
		t.Fatalf("len(candidates) = %d, want 1", len(candidates))
	}
	if got := candidates[0].ValuesBase; got != "controller.admissionWebhooks.createSecretJob.resources" {
		t.Fatalf("candidates[0].ValuesBase = %q, want %q", got, "controller.admissionWebhooks.createSecretJob.resources")
	}
	if candidates[0].ValuesPath != "" {
		t.Fatalf("candidates[0].ValuesPath = %q, want empty because resource rules are stored as values_base", candidates[0].ValuesPath)
	}
}

func TestRegistryLookupCandidatesForFinding(t *testing.T) {
	registry, err := Default()
	if err != nil {
		t.Fatalf("Default() error = %v", err)
	}

	finding := models.Finding{
		RuleID:   "AVL001",
		Path:     "templates/controller-deployment.yaml",
		Resource: "Deployment/ingress-nginx-controller",
	}
	withScaling := []models.Finding{{
		RuleID:   "SCL001",
		Path:     finding.Path,
		Resource: finding.Resource,
	}}

	candidates := registry.LookupCandidatesForFinding("ingress-nginx", withScaling, finding)
	if len(candidates) != 2 {
		t.Fatalf("len(candidates) = %d, want 2", len(candidates))
	}
	if got := candidates[0].ValuesPath; got != "controller.autoscaling.minReplicas" {
		t.Fatalf("candidates[0].ValuesPath = %q, want %q", got, "controller.autoscaling.minReplicas")
	}
	if got := candidates[0].RequiresRelatedRule; got != "SCL001" {
		t.Fatalf("candidates[0].RequiresRelatedRule = %q, want %q", got, "SCL001")
	}
	if got := candidates[1].ValuesPath; got != "controller.replicaCount" {
		t.Fatalf("candidates[1].ValuesPath = %q, want %q", got, "controller.replicaCount")
	}

	withoutScaling := registry.LookupCandidatesForFinding("ingress-nginx", nil, finding)
	if len(withoutScaling) != 1 {
		t.Fatalf("len(withoutScaling) = %d, want 1", len(withoutScaling))
	}
	if got := withoutScaling[0].ValuesPath; got != "controller.replicaCount" {
		t.Fatalf("withoutScaling[0].ValuesPath = %q, want %q", got, "controller.replicaCount")
	}

	network := registry.LookupCandidatesForFinding("ingress-nginx", nil, models.Finding{
		RuleID: "NET001",
		Path:   "templates/default-backend-deployment.yaml",
	})
	if len(network) != 1 {
		t.Fatalf("len(network) = %d, want 1", len(network))
	}
	if got := network[0].ValuesPath; got != "defaultBackend.networkPolicy.enabled" {
		t.Fatalf("network[0].ValuesPath = %q, want %q", got, "defaultBackend.networkPolicy.enabled")
	}
}

func TestLaunchRegistryContainsFiveCharts(t *testing.T) {
	registry, err := Default()
	if err != nil {
		t.Fatalf("Default() error = %v", err)
	}

	wantCharts := []string{"cert-manager", "external-secrets", "grafana", "ingress-nginx", "redis"}
	gotCharts := registry.ChartNames()
	if len(gotCharts) != len(wantCharts) {
		t.Fatalf("len(ChartNames()) = %d, want %d (%v)", len(gotCharts), len(wantCharts), wantCharts)
	}
	for i, want := range wantCharts {
		if gotCharts[i] != want {
			t.Fatalf("ChartNames()[%d] = %q, want %q (full set: %#v)", i, gotCharts[i], want, gotCharts)
		}
	}

	for _, chartName := range wantCharts {
		chart, ok := registry.LookupChart(chartName)
		if !ok {
			t.Fatalf("LookupChart(%q) found = false", chartName)
		}
		if chart.SourceFile == "" {
			t.Fatalf("chart %q SourceFile is empty", chartName)
		}
		if len(chart.Mappings) == 0 {
			t.Fatalf("chart %q has no mappings", chartName)
		}
		for _, mapping := range chart.Mappings {
			if mapping.RuleID == "" || mapping.TemplatePath == "" {
				t.Fatalf("chart %q has incomplete mapping %#v", chartName, mapping)
			}
			if len(mapping.Candidates) == 0 {
				t.Fatalf("chart %q mapping %s @ %s has no candidates", chartName, mapping.RuleID, mapping.TemplatePath)
			}
			for idx, candidate := range mapping.Candidates {
				if strings.TrimSpace(candidate.Summary) == "" {
					t.Fatalf("chart %q mapping %s @ %s candidate %d has empty summary", chartName, mapping.RuleID, mapping.TemplatePath, idx+1)
				}
				hasPath := strings.TrimSpace(candidate.ValuesPath) != ""
				hasBase := strings.TrimSpace(candidate.ValuesBase) != ""
				if hasPath == hasBase {
					t.Fatalf("chart %q mapping %s @ %s candidate %d must set exactly one of values_path or values_base", chartName, mapping.RuleID, mapping.TemplatePath, idx+1)
				}
			}
		}
	}
}

func TestRegistryRejectsInvalidMappings(t *testing.T) {
	tests := []struct {
		name        string
		fsys        fs.FS
		wantSubstrs []string
	}{
		{
			name: "duplicate rule template mapping",
			fsys: fstest.MapFS{
				"charts/duplicate.yaml": {Data: []byte(`chart: ingress-nginx
mappings:
  - rule: RES001
    template: templates/controller-deployment.yaml
    candidates:
      - values_base: controller.resources
        summary: Populate resource limits through the chart values surface.
  - rule: RES001
    template: templates/controller-deployment.yaml
    candidates:
      - values_base: controller.resources
        summary: Populate resource limits through the chart values surface.
`)},
			},
			wantSubstrs: []string{"charts/duplicate.yaml", "duplicate mapping RES001 @ templates/controller-deployment.yaml"},
		},
		{
			name: "candidate sets both values path and base",
			fsys: fstest.MapFS{
				"charts/both.yaml": {Data: []byte(`chart: ingress-nginx
mappings:
  - rule: NET001
    template: templates/controller-deployment.yaml
    candidates:
      - values_path: controller.networkPolicy.enabled
        values_base: controller.networkPolicy
        summary: Enable the controller networkPolicy switch.
`)},
			},
			wantSubstrs: []string{"charts/both.yaml", "mapping NET001 @ templates/controller-deployment.yaml candidate 1 must set exactly one of values_path or values_base"},
		},
		{
			name: "candidate sets neither values path nor base",
			fsys: fstest.MapFS{
				"charts/neither.yaml": {Data: []byte(`chart: ingress-nginx
mappings:
  - rule: SCL001
    template: templates/controller-deployment.yaml
    candidates:
      - summary: Enable controller autoscaling with a conservative baseline.
`)},
			},
			wantSubstrs: []string{"charts/neither.yaml", "mapping SCL001 @ templates/controller-deployment.yaml candidate 1 must set exactly one of values_path or values_base"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := LoadFromFS(tt.fsys)
			if err == nil {
				t.Fatal("LoadFromFS() error = nil, want non-nil")
			}
			for _, want := range tt.wantSubstrs {
				if !strings.Contains(err.Error(), want) {
					t.Fatalf("LoadFromFS() error = %q, want substring %q", err.Error(), want)
				}
			}
		})
	}
}
