package fix

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/belyaev-dev/helmdoc/internal/rules"
	_ "github.com/belyaev-dev/helmdoc/internal/rules/all"
	"github.com/belyaev-dev/helmdoc/internal/testutil/realcharts"
	"github.com/belyaev-dev/helmdoc/pkg/models"
)

func TestRealChartValidationNginxIngress(t *testing.T) {
	fixture := realcharts.FixtureByID(t, "nginx-ingress")
	ctx := realcharts.AnalysisContext(t, fixture)
	findings := rules.RunAll(ctx)
	plan := PlanBundle(ctx, findings)

	if got := len(plan.AppliedValuesFixes); got != 9 {
		t.Fatalf("fixture %s values fixes = %d, want 9", fixture.ID, got)
	}
	if got := len(plan.KustomizePatches); got != 4 {
		t.Fatalf("fixture %s kustomize patches = %d, want 4", fixture.ID, got)
	}
	if got := len(plan.AdvisoryFindings); got != 0 {
		t.Fatalf("fixture %s advisory findings = %d, want 0", fixture.ID, got)
	}
	if got := len(plan.PendingFindings); got != 0 {
		t.Fatalf("fixture %s pending findings = %d, want 0", fixture.ID, got)
	}

	overrides, err := MergeValuesOverrides(plan.AppliedValuesFixes)
	if err != nil {
		t.Fatalf("fixture %s MergeValuesOverrides() error = %v", fixture.ID, err)
	}

	_, rerendered := realcharts.RenderChartWithValues(t, fixture, overrides)

	requireRenderedResourceForChart(t, fixture.ID, rerendered, "templates/controller-networkpolicy.yaml", "NetworkPolicy", "helmdoc-ingress-nginx-controller")
	pdb := requireRenderedResourceForChart(t, fixture.ID, rerendered, "templates/controller-poddisruptionbudget.yaml", "PodDisruptionBudget", "helmdoc-ingress-nginx-controller")
	hpa := requireRenderedResourceForChart(t, fixture.ID, rerendered, "templates/controller-hpa.yaml", "HorizontalPodAutoscaler", "helmdoc-ingress-nginx-controller")

	if got, ok := pdb.GetNestedMap("spec", "selector", "matchLabels"); !ok || len(got) == 0 {
		t.Fatalf("fixture %s rendered PodDisruptionBudget/%s is missing spec.selector.matchLabels", fixture.ID, pdb.Name)
	}
	if got, ok := hpa.GetNestedMap("spec", "scaleTargetRef"); !ok {
		t.Fatalf("fixture %s rendered HorizontalPodAutoscaler/%s is missing spec.scaleTargetRef", fixture.ID, hpa.Name)
	} else {
		if gotKind, _ := got["kind"].(string); gotKind != "Deployment" {
			t.Fatalf("fixture %s rendered HorizontalPodAutoscaler/%s scaleTargetRef.kind = %#v, want %q", fixture.ID, hpa.Name, got["kind"], "Deployment")
		}
		if gotName, _ := got["name"].(string); gotName != "helmdoc-ingress-nginx-controller" {
			t.Fatalf("fixture %s rendered HorizontalPodAutoscaler/%s scaleTargetRef.name = %#v, want %q", fixture.ID, hpa.Name, got["name"], "helmdoc-ingress-nginx-controller")
		}
	}
	if got, ok := hpa.GetNested("spec", "minReplicas"); !ok || fmt.Sprint(got) != "2" {
		t.Fatalf("fixture %s rendered HorizontalPodAutoscaler/%s spec.minReplicas = %#v, want 2", fixture.ID, hpa.Name, got)
	}

	controller := requireRenderedWorkloadContainerForChart(t, fixture.ID, rerendered, "templates/controller-deployment.yaml", "Deployment", "helmdoc-ingress-nginx-controller", "controller")
	requireContainerBoolField(t, fixture.ID, controller.Container, []string{"securityContext", "readOnlyRootFilesystem"}, true, "Deployment/helmdoc-ingress-nginx-controller container \"controller\"")
	requireContainerStringField(t, fixture.ID, controller.Container, []string{"resources", "limits", "cpu"}, "100m", "Deployment/helmdoc-ingress-nginx-controller container \"controller\"")
	requireContainerStringField(t, fixture.ID, controller.Container, []string{"resources", "limits", "memory"}, "90Mi", "Deployment/helmdoc-ingress-nginx-controller container \"controller\"")

	createJob := requireRenderedWorkloadContainerForChart(t, fixture.ID, rerendered, "templates/admission-webhooks/job-patch/job-createSecret.yaml", "Job", "helmdoc-ingress-nginx-admission-create", "create")
	requireContainerStringField(t, fixture.ID, createJob.Container, []string{"resources", "limits", "cpu"}, "10m", "Job/helmdoc-ingress-nginx-admission-create container \"create\"")
	requireContainerStringField(t, fixture.ID, createJob.Container, []string{"resources", "limits", "memory"}, "20Mi", "Job/helmdoc-ingress-nginx-admission-create container \"create\"")
	requireContainerStringField(t, fixture.ID, createJob.Container, []string{"resources", "requests", "cpu"}, "10m", "Job/helmdoc-ingress-nginx-admission-create container \"create\"")
	requireContainerStringField(t, fixture.ID, createJob.Container, []string{"resources", "requests", "memory"}, "20Mi", "Job/helmdoc-ingress-nginx-admission-create container \"create\"")

	patchJob := requireRenderedWorkloadContainerForChart(t, fixture.ID, rerendered, "templates/admission-webhooks/job-patch/job-patchWebhook.yaml", "Job", "helmdoc-ingress-nginx-admission-patch", "patch")
	requireContainerStringField(t, fixture.ID, patchJob.Container, []string{"resources", "limits", "cpu"}, "10m", "Job/helmdoc-ingress-nginx-admission-patch container \"patch\"")
	requireContainerStringField(t, fixture.ID, patchJob.Container, []string{"resources", "limits", "memory"}, "20Mi", "Job/helmdoc-ingress-nginx-admission-patch container \"patch\"")
	requireContainerStringField(t, fixture.ID, patchJob.Container, []string{"resources", "requests", "cpu"}, "10m", "Job/helmdoc-ingress-nginx-admission-patch container \"patch\"")
	requireContainerStringField(t, fixture.ID, patchJob.Container, []string{"resources", "requests", "memory"}, "20Mi", "Job/helmdoc-ingress-nginx-admission-patch container \"patch\"")

	probeFieldByRule := map[string]string{
		"HLT001": "livenessProbe",
		"HLT002": "readinessProbe",
	}
	for _, patch := range plan.KustomizePatches {
		if _, ok := realcharts.FindRenderedResource(ctx.RenderedResources, patch.Finding.Path, patch.Target.Kind, patch.Target.Name); !ok {
			t.Fatalf("fixture %s patch %s targets missing rendered resource %s in %s", fixture.ID, patch.Finding.RuleID, patch.Target.Kind+"/"+patch.Target.Name, patch.Finding.Path)
		}
		if _, ok := realcharts.FindWorkloadContainer(ctx.RenderedResources, patch.Finding.Path, patch.Target.Kind, patch.Target.Name, patch.ContainerName); !ok {
			t.Fatalf("fixture %s patch %s targets missing container %q in %s @ %s", fixture.ID, patch.Finding.RuleID, patch.ContainerName, patch.Target.Kind+"/"+patch.Target.Name, patch.Finding.Path)
		}

		probeField := probeFieldByRule[patch.Finding.RuleID]
		if probeField == "" {
			t.Fatalf("fixture %s patch %s has no expected probe-field assertion", fixture.ID, patch.Finding.RuleID)
		}
		requireProbeFieldOnPatch(t, patch, probeField)
		requirePatchProbeExecCommand(t, fixture.ID, patch, probeField)
	}
}

func TestRealChartValidationGrafana(t *testing.T) {
	fixture := realcharts.FixtureByID(t, "grafana")
	ctx := realcharts.AnalysisContext(t, fixture)
	plan := PlanBundle(ctx, rules.RunAll(ctx))

	if got := len(plan.AppliedValuesFixes); got != 1 {
		t.Fatalf("fixture %s values fixes = %d, want 1", fixture.ID, got)
	}
	if got := len(plan.KustomizePatches); got != 0 {
		t.Fatalf("fixture %s kustomize patches = %d, want 0", fixture.ID, got)
	}
	if got := len(plan.AdvisoryFindings); got != 1 {
		t.Fatalf("fixture %s advisory findings = %d, want 1", fixture.ID, got)
	}
	if got := len(plan.PendingFindings); got != 5 {
		t.Fatalf("fixture %s pending findings = %d, want 5", fixture.ID, got)
	}

	advisory := requireAdvisoryFinding(t, plan, "IMG002")
	if want, _ := advisoryExplanationForFinding(advisory.Finding); advisory.Explanation != want {
		t.Fatalf("fixture %s advisory explanation = %q, want %q", fixture.ID, advisory.Explanation, want)
	}
	requirePendingFindingRuleIDs(t, fixture.ID, plan.PendingFindings, []string{"AVL001", "RES001", "RES002", "SCL001", "SEC003"})
	requireAppliedValuesPathForChart(t, fixture.ID, plan.AppliedValuesFixes, "NET001", "templates/deployment.yaml", "imageRenderer.networkPolicy.enabled")

	overrides, err := MergeValuesOverrides(plan.AppliedValuesFixes)
	if err != nil {
		t.Fatalf("fixture %s MergeValuesOverrides() error = %v", fixture.ID, err)
	}

	_, rerendered := realcharts.RenderChartWithValues(t, fixture, overrides)
	deployment := requireRenderedResourceForChart(t, fixture.ID, rerendered, "templates/deployment.yaml", "Deployment", "helmdoc-grafana")
	if got, ok := deployment.GetNestedMap("spec", "selector", "matchLabels"); !ok || len(got) == 0 {
		t.Fatalf("fixture %s rendered Deployment/%s is missing spec.selector.matchLabels", fixture.ID, deployment.Name)
	}
	requireRenderedWorkloadContainerForChart(t, fixture.ID, rerendered, "templates/deployment.yaml", "Deployment", "helmdoc-grafana", "grafana")
	requireNoRenderedResourcesForChart(t, fixture.ID, rerendered, "templates/networkpolicy.yaml")
	requireNoRenderedResourcesForChart(t, fixture.ID, rerendered, "templates/poddisruptionbudget.yaml")
	requireNoRenderedResourcesForChart(t, fixture.ID, rerendered, "templates/hpa.yaml")
	requireNoRenderedResourcesForChart(t, fixture.ID, rerendered, "templates/image-renderer-network-policy.yaml")
	requireNoRenderedResourcesForChart(t, fixture.ID, rerendered, "templates/image-renderer-hpa.yaml")
}

func TestRealChartValidationPostgresql(t *testing.T) {
	fixture := realcharts.FixtureByID(t, "postgresql")
	ctx := realcharts.AnalysisContext(t, fixture)
	plan := PlanBundle(ctx, rules.RunAll(ctx))

	if got := len(plan.AppliedValuesFixes); got != 0 {
		t.Fatalf("fixture %s values fixes = %d, want 0", fixture.ID, got)
	}
	if got := len(plan.KustomizePatches); got != 0 {
		t.Fatalf("fixture %s kustomize patches = %d, want 0", fixture.ID, got)
	}
	if got := len(plan.AdvisoryFindings); got != 1 {
		t.Fatalf("fixture %s advisory findings = %d, want 1", fixture.ID, got)
	}
	if got := len(plan.PendingFindings); got != 1 {
		t.Fatalf("fixture %s pending findings = %d, want 1", fixture.ID, got)
	}

	advisory := requireAdvisoryFinding(t, plan, "IMG002")
	if want, _ := advisoryExplanationForFinding(advisory.Finding); advisory.Explanation != want {
		t.Fatalf("fixture %s advisory explanation = %q, want %q", fixture.ID, advisory.Explanation, want)
	}
	requirePendingFindingRuleIDs(t, fixture.ID, plan.PendingFindings, []string{"SCL001"})

	overrides, err := MergeValuesOverrides(plan.AppliedValuesFixes)
	if err != nil {
		t.Fatalf("fixture %s MergeValuesOverrides() error = %v", fixture.ID, err)
	}
	if len(overrides) != 0 {
		t.Fatalf("fixture %s merged overrides = %#v, want empty map", fixture.ID, overrides)
	}

	_, rerendered := realcharts.RenderChartWithValues(t, fixture, overrides)
	requireRenderedResourceForChart(t, fixture.ID, rerendered, "templates/primary/networkpolicy.yaml", "NetworkPolicy", "helmdoc-postgresql")
	pdb := requireRenderedResourceForChart(t, fixture.ID, rerendered, "templates/primary/pdb.yaml", "PodDisruptionBudget", "helmdoc-postgresql")
	statefulSet := requireRenderedResourceForChart(t, fixture.ID, rerendered, "templates/primary/statefulset.yaml", "StatefulSet", "helmdoc-postgresql")
	if got, ok := pdb.GetNestedMap("spec", "selector", "matchLabels"); !ok || len(got) == 0 {
		t.Fatalf("fixture %s rendered PodDisruptionBudget/%s is missing spec.selector.matchLabels", fixture.ID, pdb.Name)
	}
	container := requireRenderedWorkloadContainerForChart(t, fixture.ID, rerendered, "templates/primary/statefulset.yaml", "StatefulSet", "helmdoc-postgresql", "postgresql")
	requireContainerBoolField(t, fixture.ID, container.Container, []string{"securityContext", "readOnlyRootFilesystem"}, true, "StatefulSet/helmdoc-postgresql container \"postgresql\"")
	if got, ok := statefulSet.GetNested("spec", "replicas"); !ok || fmt.Sprint(got) != "1" {
		t.Fatalf("fixture %s rendered StatefulSet/%s spec.replicas = %#v, want 1", fixture.ID, statefulSet.Name, got)
	}
	requireNoRenderedKindForChart(t, fixture.ID, rerendered, "HorizontalPodAutoscaler")
}

func requireRenderedResourceForChart(t *testing.T, fixtureID string, rendered map[string][]models.K8sResource, templatePath, kind, name string) models.K8sResource {
	t.Helper()

	resource, ok := realcharts.FindRenderedResource(rendered, templatePath, kind, name)
	if !ok {
		available := make([]string, 0, len(rendered[templatePath]))
		for _, candidate := range rendered[templatePath] {
			available = append(available, candidate.Kind+"/"+candidate.Name)
		}
		t.Fatalf("fixture %s rendered %s missing from %s (available: %s)", fixtureID, kind+"/"+name, templatePath, strings.Join(available, ", "))
	}

	return resource
}

func requireRenderedWorkloadContainerForChart(t *testing.T, fixtureID string, rendered map[string][]models.K8sResource, templatePath, kind, name, containerName string) rules.WorkloadContainer {
	t.Helper()

	container, ok := realcharts.FindWorkloadContainer(rendered, templatePath, kind, name, containerName)
	if !ok {
		resource := requireRenderedResourceForChart(t, fixtureID, rendered, templatePath, kind, name)
		containers, _ := resource.GetNestedSlice("spec", "template", "spec", "containers")
		seen := make([]string, 0, len(containers))
		for _, raw := range containers {
			containerMap, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			if name, _ := containerMap["name"].(string); name != "" {
				seen = append(seen, name)
			}
		}
		t.Fatalf("fixture %s rendered %s in %s is missing container %q (available: %s)", fixtureID, kind+"/"+name, templatePath, containerName, strings.Join(seen, ", "))
	}

	return container
}

func requireContainerBoolField(t *testing.T, fixtureID string, container map[string]any, path []string, want bool, objectLabel string) {
	t.Helper()

	got, ok := nestedValue(container, path...)
	if !ok {
		t.Fatalf("fixture %s rendered %s is missing %s", fixtureID, objectLabel, dottedPath(path))
	}
	gotBool, ok := got.(bool)
	if !ok {
		t.Fatalf("fixture %s rendered %s %s = %#v, want bool(%t)", fixtureID, objectLabel, dottedPath(path), got, want)
	}
	if gotBool != want {
		t.Fatalf("fixture %s rendered %s %s = %t, want %t", fixtureID, objectLabel, dottedPath(path), gotBool, want)
	}
}

func requireContainerStringField(t *testing.T, fixtureID string, container map[string]any, path []string, want, objectLabel string) {
	t.Helper()

	got, ok := nestedValue(container, path...)
	if !ok {
		t.Fatalf("fixture %s rendered %s is missing %s", fixtureID, objectLabel, dottedPath(path))
	}
	gotString, ok := got.(string)
	if !ok {
		t.Fatalf("fixture %s rendered %s %s = %#v, want %q", fixtureID, objectLabel, dottedPath(path), got, want)
	}
	if gotString != want {
		t.Fatalf("fixture %s rendered %s %s = %q, want %q", fixtureID, objectLabel, dottedPath(path), gotString, want)
	}
}

func requirePatchProbeExecCommand(t *testing.T, fixtureID string, patch KustomizePatch, field string) {
	t.Helper()

	spec := requireNestedMap(t, patch.Patch, "spec")
	template := requireNestedMap(t, spec, "template")
	podSpec := requireNestedMap(t, template, "spec")
	containers, ok := podSpec["containers"].([]any)
	if !ok || len(containers) != 1 {
		t.Fatalf("fixture %s patch %s containers = %#v, want one strategic-merge entry", fixtureID, patch.Finding.RuleID, podSpec["containers"])
	}
	container, ok := containers[0].(map[string]any)
	if !ok {
		t.Fatalf("fixture %s patch %s container = %#v, want map[string]any", fixtureID, patch.Finding.RuleID, containers[0])
	}
	probe, ok := container[field].(map[string]any)
	if !ok {
		t.Fatalf("fixture %s patch %s %s = %#v, want probe map", fixtureID, patch.Finding.RuleID, field, container[field])
	}
	execMap := requireNestedMap(t, probe, "exec")
	command, ok := execMap["command"].([]any)
	if !ok || len(command) != 2 {
		t.Fatalf("fixture %s patch %s %s.exec.command = %#v, want [\"/kube-webhook-certgen\", \"--help\"]", fixtureID, patch.Finding.RuleID, field, execMap["command"])
	}
	if fmt.Sprint(command[0]) != "/kube-webhook-certgen" || fmt.Sprint(command[1]) != "--help" {
		t.Fatalf("fixture %s patch %s %s.exec.command = %#v, want [\"/kube-webhook-certgen\", \"--help\"]", fixtureID, patch.Finding.RuleID, field, command)
	}
}

func requirePendingFindingRuleIDs(t *testing.T, fixtureID string, pending []PendingFinding, want []string) {
	t.Helper()

	got := make([]string, 0, len(pending))
	for _, finding := range pending {
		got = append(got, finding.Finding.RuleID)
	}
	sort.Strings(got)
	want = append([]string(nil), want...)
	sort.Strings(want)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("fixture %s pending rule IDs = %#v, want %#v", fixtureID, got, want)
	}
}

func requireAppliedValuesPathForChart(t *testing.T, fixtureID string, fixes []AppliedValuesFix, ruleID, templatePath, wantValuesPath string) {
	t.Helper()

	for _, fix := range fixes {
		if fix.Finding.RuleID == ruleID && fix.Finding.Path == templatePath {
			if fix.ValuesPath != wantValuesPath {
				t.Fatalf("fixture %s applied values path for %s @ %s = %q, want %q", fixtureID, ruleID, templatePath, fix.ValuesPath, wantValuesPath)
			}
			return
		}
	}

	t.Fatalf("fixture %s missing applied values fix for %s @ %s", fixtureID, ruleID, templatePath)
}

func requireNoRenderedResourcesForChart(t *testing.T, fixtureID string, rendered map[string][]models.K8sResource, templatePath string) {
	t.Helper()

	if got := len(rendered[templatePath]); got != 0 {
		seen := make([]string, 0, got)
		for _, resource := range rendered[templatePath] {
			seen = append(seen, resource.Kind+"/"+resource.Name)
		}
		t.Fatalf("fixture %s rendered %d resources from %s, want none (saw: %s)", fixtureID, got, templatePath, strings.Join(seen, ", "))
	}
}

func requireNoRenderedKindForChart(t *testing.T, fixtureID string, rendered map[string][]models.K8sResource, kind string) {
	t.Helper()

	seen := make([]string, 0)
	for templatePath, resources := range rendered {
		for _, resource := range resources {
			if resource.Kind != kind {
				continue
			}
			seen = append(seen, templatePath+" -> "+resource.Kind+"/"+resource.Name)
		}
	}
	if len(seen) != 0 {
		sort.Strings(seen)
		t.Fatalf("fixture %s rendered unexpected %s resources: %s", fixtureID, kind, strings.Join(seen, ", "))
	}
}

func nestedValue(value map[string]any, path ...string) (any, bool) {
	current := any(value)
	for _, segment := range path {
		mapped, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		next, ok := mapped[segment]
		if !ok {
			return nil, false
		}
		current = next
	}
	return current, true
}

func dottedPath(path []string) string {
	return strings.Join(path, ".")
}
