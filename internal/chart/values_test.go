package chart

import (
	"reflect"
	"testing"
)

func TestAnalyzeValuesBuildsDotPathSurface(t *testing.T) {
	loadedChart, err := LoadChart(fixtureChartPath(t))
	if err != nil {
		t.Fatalf("LoadChart() error = %v", err)
	}

	surface := AnalyzeValues(loadedChart)
	if surface == nil {
		t.Fatal("AnalyzeValues() = nil, want non-nil surface")
	}

	checks := []struct {
		path        string
		wantDefault any
		wantType    string
	}{
		{path: "controller", wantType: "object"},
		{path: "controller.replicaCount", wantDefault: float64(2), wantType: "number"},
		{path: "controller.resources.limits.cpu", wantDefault: "500m", wantType: "string"},
		{path: "controller.service.annotations.service.beta.kubernetes.io/aws-load-balancer-type", wantDefault: "nlb", wantType: "string"},
		{path: "controller.tolerations", wantType: "array"},
		{path: "controller.tolerations.0", wantType: "object"},
		{path: "controller.tolerations.0.key", wantDefault: "workload", wantType: "string"},
		{path: "controller.extraArgs.0", wantDefault: "--enable-ssl-passthrough", wantType: "string"},
		{path: "persistence", wantType: "object"},
		{path: "persistence.accessModes", wantType: "array"},
		{path: "persistence.accessModes.0", wantDefault: "ReadWriteOnce", wantType: "string"},
		{path: "persistence.enabled", wantDefault: true, wantType: "bool"},
		{path: "persistence.storageClass", wantDefault: "gp3", wantType: "string"},
		{path: "serviceAccount.name", wantDefault: "", wantType: "string"},
	}

	for _, check := range checks {
		if !surface.HasPath(check.path) {
			t.Fatalf("HasPath(%q) = false, want true", check.path)
		}
		if got := surface.PathType(check.path); got != check.wantType {
			t.Fatalf("PathType(%q) = %q, want %q", check.path, got, check.wantType)
		}
		if check.wantDefault != nil {
			if got := surface.GetDefault(check.path); !reflect.DeepEqual(got, check.wantDefault) {
				t.Fatalf("GetDefault(%q) = %#v, want %#v", check.path, got, check.wantDefault)
			}
		}
	}

	if surface.HasPath("controller.missing") {
		t.Fatal("HasPath(controller.missing) = true, want false")
	}
	if got := surface.GetDefault("controller.missing"); got != nil {
		t.Fatalf("GetDefault(controller.missing) = %#v, want nil", got)
	}
	if got := surface.PathType("controller.missing"); got != "" {
		t.Fatalf("PathType(controller.missing) = %q, want empty string", got)
	}

	wantPaths := []string{
		"controller",
		"controller.containerSecurityContext",
		"controller.containerSecurityContext.allowPrivilegeEscalation",
		"controller.containerSecurityContext.capabilities",
		"controller.containerSecurityContext.capabilities.drop",
		"controller.containerSecurityContext.capabilities.drop.0",
		"controller.containerSecurityContext.readOnlyRootFilesystem",
		"controller.extraArgs",
		"controller.extraArgs.0",
		"controller.image",
		"controller.image.pullPolicy",
		"controller.image.repository",
		"controller.image.tag",
		"controller.podSecurityContext",
		"controller.podSecurityContext.fsGroup",
		"controller.podSecurityContext.runAsNonRoot",
		"controller.replicaCount",
		"controller.resources",
		"controller.resources.limits",
		"controller.resources.limits.cpu",
		"controller.resources.limits.memory",
		"controller.resources.requests",
		"controller.resources.requests.cpu",
		"controller.resources.requests.memory",
		"controller.service",
		"controller.service.annotations",
		"controller.service.annotations.service.beta.kubernetes.io/aws-load-balancer-type",
		"controller.service.port",
		"controller.service.type",
		"controller.tolerations",
		"controller.tolerations.0",
		"controller.tolerations.0.key",
		"controller.tolerations.0.operator",
		"persistence",
		"persistence.accessModes",
		"persistence.accessModes.0",
		"persistence.enabled",
		"persistence.size",
		"persistence.storageClass",
		"serviceAccount",
		"serviceAccount.create",
		"serviceAccount.name",
	}
	if got := surface.AllPaths(); !reflect.DeepEqual(got, wantPaths) {
		t.Fatalf("AllPaths() = %#v, want %#v", got, wantPaths)
	}

	controllerDefaults, ok := surface.GetDefault("controller").(map[string]any)
	if !ok {
		t.Fatalf("GetDefault(controller) type = %T, want map[string]any", surface.GetDefault("controller"))
	}
	controllerDefaults["replicaCount"] = float64(99)
	if got := surface.GetDefault("controller.replicaCount"); got != float64(2) {
		t.Fatalf("GetDefault(controller.replicaCount) after mutating returned controller map = %#v, want %#v", got, float64(2))
	}

	loadedChart.Values["controller"].(map[string]any)["replicaCount"] = float64(42)
	if got := surface.GetDefault("controller.replicaCount"); got != float64(2) {
		t.Fatalf("GetDefault(controller.replicaCount) after mutating chart values = %#v, want %#v", got, float64(2))
	}
}
