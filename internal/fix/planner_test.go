package fix

import (
	"reflect"
	"strings"
	"testing"

	"github.com/belyaev-dev/helmdoc/internal/rules"
	_ "github.com/belyaev-dev/helmdoc/internal/rules/all"
	"github.com/belyaev-dev/helmdoc/internal/testutil/realcharts"
	"github.com/belyaev-dev/helmdoc/pkg/models"
)

func TestPlanValuesBundleForNginxIngress(t *testing.T) {
	fixture := realcharts.FixtureByID(t, "nginx-ingress")
	ctx := realcharts.AnalysisContext(t, fixture)
	findings := rules.RunAll(ctx)

	plan := PlanValuesBundle(ctx, findings)
	if got := len(plan.AppliedValuesFixes); got != 9 {
		t.Fatalf("len(plan.AppliedValuesFixes) = %d, want 9", got)
	}
	if got := len(plan.KustomizePatches); got != 0 {
		t.Fatalf("len(plan.KustomizePatches) = %d, want 0 for the S01 values-only planner", got)
	}
	if got := len(plan.PendingFindings); got != 4 {
		t.Fatalf("len(plan.PendingFindings) = %d, want 4", got)
	}

	appliedByRuleAndPath := map[string]string{}
	for _, fix := range plan.AppliedValuesFixes {
		appliedByRuleAndPath[fix.Finding.RuleID+"|"+fix.Finding.Path] = fix.ValuesPath
	}

	checks := map[string]string{
		"SEC003|templates/controller-deployment.yaml":                         "controller.image.readOnlyRootFilesystem",
		"RES001|templates/controller-deployment.yaml":                         "controller.resources.limits",
		"RES001|templates/admission-webhooks/job-patch/job-createSecret.yaml": "controller.admissionWebhooks.createSecretJob.resources.limits",
		"RES001|templates/admission-webhooks/job-patch/job-patchWebhook.yaml": "controller.admissionWebhooks.patchWebhookJob.resources.limits",
		"RES002|templates/admission-webhooks/job-patch/job-createSecret.yaml": "controller.admissionWebhooks.createSecretJob.resources.requests",
		"RES002|templates/admission-webhooks/job-patch/job-patchWebhook.yaml": "controller.admissionWebhooks.patchWebhookJob.resources.requests",
		"AVL001|templates/controller-deployment.yaml":                         "controller.autoscaling.minReplicas",
		"NET001|templates/controller-deployment.yaml":                         "controller.networkPolicy.enabled",
		"SCL001|templates/controller-deployment.yaml":                         "controller.autoscaling",
	}
	for key, want := range checks {
		if got := appliedByRuleAndPath[key]; got != want {
			t.Fatalf("appliedByRuleAndPath[%q] = %q, want %q", key, got, want)
		}
	}

	merged, err := MergeValuesOverrides(plan.AppliedValuesFixes)
	if err != nil {
		t.Fatalf("MergeValuesOverrides(plan.AppliedValuesFixes) error = %v", err)
	}

	controller := requireNestedMap(t, merged, "controller")
	image := requireNestedMap(t, controller, "image")
	if got, _ := image["readOnlyRootFilesystem"].(bool); !got {
		t.Fatalf("controller.image.readOnlyRootFilesystem = %#v, want true", image["readOnlyRootFilesystem"])
	}

	resources := requireNestedMap(t, controller, "resources")
	limits := requireNestedMap(t, resources, "limits")
	if limits["cpu"] != "100m" || limits["memory"] != "90Mi" {
		t.Fatalf("controller.resources.limits = %#v, want cpu=100m memory=90Mi", limits)
	}

	autoscaling := requireNestedMap(t, controller, "autoscaling")
	if got, _ := autoscaling["enabled"].(bool); !got {
		t.Fatalf("controller.autoscaling.enabled = %#v, want true", autoscaling["enabled"])
	}
	if got, _ := autoscaling["minReplicas"].(int); got != 2 {
		t.Fatalf("controller.autoscaling.minReplicas = %#v, want 2", autoscaling["minReplicas"])
	}

	networkPolicy := requireNestedMap(t, controller, "networkPolicy")
	if got, _ := networkPolicy["enabled"].(bool); !got {
		t.Fatalf("controller.networkPolicy.enabled = %#v, want true", networkPolicy["enabled"])
	}

	createSecretJob := requireNestedMap(t, requireNestedMap(t, requireNestedMap(t, controller, "admissionWebhooks"), "createSecretJob"), "resources")
	if requireNestedMap(t, createSecretJob, "limits")["memory"] != "20Mi" {
		t.Fatalf("createSecretJob.resources.limits = %#v, want memory=20Mi", requireNestedMap(t, createSecretJob, "limits"))
	}
	if requireNestedMap(t, createSecretJob, "requests")["cpu"] != "10m" {
		t.Fatalf("createSecretJob.resources.requests = %#v, want cpu=10m", requireNestedMap(t, createSecretJob, "requests"))
	}
}

func TestPlanKustomizePatchesForNginxIngress(t *testing.T) {
	fixture := realcharts.FixtureByID(t, "nginx-ingress")
	ctx := realcharts.AnalysisContext(t, fixture)
	findings := rules.RunAll(ctx)

	plan := PlanBundle(ctx, findings)
	if got := len(plan.AppliedValuesFixes); got != 9 {
		t.Fatalf("len(plan.AppliedValuesFixes) = %d, want 9", got)
	}
	if got := len(plan.KustomizePatches); got != 4 {
		t.Fatalf("len(plan.KustomizePatches) = %d, want 4", got)
	}
	if got := len(plan.PendingFindings); got != 0 {
		t.Fatalf("len(plan.PendingFindings) = %d, want 0", got)
	}

	wantContainers := map[string]string{
		"HLT001|templates/admission-webhooks/job-patch/job-createSecret.yaml": "create",
		"HLT001|templates/admission-webhooks/job-patch/job-patchWebhook.yaml": "patch",
		"HLT002|templates/admission-webhooks/job-patch/job-createSecret.yaml": "create",
		"HLT002|templates/admission-webhooks/job-patch/job-patchWebhook.yaml": "patch",
	}
	wantProbeField := map[string]string{
		"HLT001": "livenessProbe",
		"HLT002": "readinessProbe",
	}

	for _, patch := range plan.KustomizePatches {
		key := patch.Finding.RuleID + "|" + patch.Finding.Path
		wantContainer := wantContainers[key]
		if wantContainer == "" {
			t.Fatalf("unexpected patch target %q", key)
		}
		if patch.ContainerName != wantContainer {
			t.Fatalf("patch %q container = %q, want %q", key, patch.ContainerName, wantContainer)
		}
		if patch.Summary == "" {
			t.Fatalf("patch %q summary = empty, want explanation", key)
		}

		rendered, err := lookupRenderedResource(ctx.RenderedResources, patch.Finding)
		if err != nil {
			t.Fatalf("lookupRenderedResource(%q) error = %v", key, err)
		}
		if patch.Target.APIVersion != rendered.APIVersion || patch.Target.Kind != rendered.Kind || patch.Target.Name != rendered.Name || patch.Target.Namespace != rendered.Namespace {
			t.Fatalf("patch %q target = %#v, want %#v", key, patch.Target, ResourceRef{
				APIVersion: rendered.APIVersion,
				Kind:       rendered.Kind,
				Name:       rendered.Name,
				Namespace:  rendered.Namespace,
			})
		}

		requireProbeFieldOnPatch(t, patch, wantProbeField[patch.Finding.RuleID])
	}
}

func TestPlanKustomizePatchesPreservesAppliedValuesFixes(t *testing.T) {
	fixture := realcharts.FixtureByID(t, "nginx-ingress")
	ctx := realcharts.AnalysisContext(t, fixture)
	findings := rules.RunAll(ctx)

	valuesPlan := PlanValuesBundle(ctx, findings)
	sortBundlePlan(&valuesPlan)
	bundlePlan := PlanBundle(ctx, findings)

	if !reflect.DeepEqual(bundlePlan.AppliedValuesFixes, valuesPlan.AppliedValuesFixes) {
		t.Fatalf("PlanBundle() changed AppliedValuesFixes\nvalues-only: %#v\nfull bundle: %#v", valuesPlan.AppliedValuesFixes, bundlePlan.AppliedValuesFixes)
	}
	if got := len(bundlePlan.KustomizePatches); got != 4 {
		t.Fatalf("len(bundlePlan.KustomizePatches) = %d, want 4", got)
	}
}

func TestFindingsWithoutCredibleValuesPathStayPending(t *testing.T) {
	ctx := rules.AnalysisContext{
		ValuesSurface: models.NewValuesSurface(map[string]models.ValuePath{
			"controller.admissionWebhooks.createSecretJob.resources": {Default: map[string]any{}, Type: "object"},
		}),
	}

	findings := []models.Finding{
		{RuleID: "HLT001", Path: "templates/admission-webhooks/job-patch/job-createSecret.yaml", Resource: "Job/helmdoc-ingress-nginx-admission-create"},
		{RuleID: "HLT002", Path: "templates/admission-webhooks/job-patch/job-patchWebhook.yaml", Resource: "Job/helmdoc-ingress-nginx-admission-patch"},
	}

	plan := PlanValuesBundle(ctx, findings)
	if len(plan.AppliedValuesFixes) != 0 {
		t.Fatalf("len(plan.AppliedValuesFixes) = %d, want 0", len(plan.AppliedValuesFixes))
	}
	if len(plan.PendingFindings) != 2 {
		t.Fatalf("len(plan.PendingFindings) = %d, want 2", len(plan.PendingFindings))
	}

	for _, pending := range plan.PendingFindings {
		if pending.Finding.RuleID != "HLT001" && pending.Finding.RuleID != "HLT002" {
			t.Fatalf("pending rule = %q, want HLT001/HLT002", pending.Finding.RuleID)
		}
		if pending.Reason == "" || pending.Reason == "rule is not supported by the S01 values-first planner" {
			t.Fatalf("pending reason = %q, want specific missing-values-path guidance", pending.Reason)
		}
	}
}

func TestPlanBundleRoutesAdvisoryFindings(t *testing.T) {
	const wantDigestExplanation = "Pinning by digest requires looking up the correct digest in your container registry, so helmdoc cannot determine it automatically."

	t.Run("sorts advisory findings deterministically", func(t *testing.T) {
		plan := PlanBundle(rules.AnalysisContext{}, []models.Finding{
			{RuleID: "ING001", Path: "templates/z.yaml", Resource: "Ingress/z"},
			{RuleID: "CFG001", Path: "templates/a.yaml", Resource: "Deployment/a"},
			{RuleID: "IMG001", Path: "templates/a.yaml", Resource: "Deployment/a"},
		})

		if len(plan.AppliedValuesFixes) != 0 {
			t.Fatalf("len(plan.AppliedValuesFixes) = %d, want 0", len(plan.AppliedValuesFixes))
		}
		if len(plan.KustomizePatches) != 0 {
			t.Fatalf("len(plan.KustomizePatches) = %d, want 0", len(plan.KustomizePatches))
		}
		if len(plan.PendingFindings) != 0 {
			t.Fatalf("len(plan.PendingFindings) = %d, want 0", len(plan.PendingFindings))
		}

		gotOrder := make([]string, 0, len(plan.AdvisoryFindings))
		for _, advisory := range plan.AdvisoryFindings {
			gotOrder = append(gotOrder, advisory.Finding.RuleID+"|"+advisory.Finding.Path+"|"+advisory.Finding.Resource)
		}
		wantOrder := []string{
			"CFG001|templates/a.yaml|Deployment/a",
			"IMG001|templates/a.yaml|Deployment/a",
			"ING001|templates/z.yaml|Ingress/z",
		}
		if !reflect.DeepEqual(gotOrder, wantOrder) {
			t.Fatalf("advisory order = %#v, want %#v", gotOrder, wantOrder)
		}
	})

	tests := []struct {
		name             string
		fixtureID        string
		wantValues       int
		wantPatches      int
		wantAdvisories   int
		wantPending      int
		wantAppliedKey   string
		wantAppliedValue string
	}{
		{
			name:             "grafana",
			fixtureID:        "grafana",
			wantValues:       5,
			wantPatches:      0,
			wantAdvisories:   1,
			wantPending:      1,
			wantAppliedKey:   "NET001|templates/deployment.yaml",
			wantAppliedValue: "networkPolicy.enabled",
		},
		{
			name:           "postgresql",
			fixtureID:      "postgresql",
			wantValues:     0,
			wantPatches:    0,
			wantAdvisories: 1,
			wantPending:    1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixture := realcharts.FixtureByID(t, tt.fixtureID)
			ctx := realcharts.AnalysisContext(t, fixture)
			plan := PlanBundle(ctx, rules.RunAll(ctx))

			if got := len(plan.AppliedValuesFixes); got != tt.wantValues {
				t.Fatalf("len(plan.AppliedValuesFixes) = %d, want %d", got, tt.wantValues)
			}
			if got := len(plan.KustomizePatches); got != tt.wantPatches {
				t.Fatalf("len(plan.KustomizePatches) = %d, want %d", got, tt.wantPatches)
			}
			if got := len(plan.AdvisoryFindings); got != tt.wantAdvisories {
				t.Fatalf("len(plan.AdvisoryFindings) = %d, want %d", got, tt.wantAdvisories)
			}
			if got := len(plan.PendingFindings); got != tt.wantPending {
				t.Fatalf("len(plan.PendingFindings) = %d, want %d", got, tt.wantPending)
			}

			advisory := requireAdvisoryFinding(t, plan, "IMG002")
			if advisory.Explanation != wantDigestExplanation {
				t.Fatalf("advisory explanation = %q, want %q", advisory.Explanation, wantDigestExplanation)
			}
			if strings.Contains(advisory.Explanation, "S01 values-first planner") || strings.Contains(advisory.Explanation, "supported Kustomize default") {
				t.Fatalf("advisory explanation = %q, want user-facing guidance without planner internals", advisory.Explanation)
			}

			for _, pending := range plan.PendingFindings {
				if _, ok := advisoryExplanationForFinding(pending.Finding); ok {
					t.Fatalf("pending finding unexpectedly includes advisory-only rule %s", pending.Finding.RuleID)
				}
			}
			for _, patch := range plan.KustomizePatches {
				if _, ok := advisoryExplanationForFinding(patch.Finding); ok {
					t.Fatalf("kustomize patch unexpectedly includes advisory-only rule %s", patch.Finding.RuleID)
				}
			}

			if tt.wantAppliedKey != "" {
				appliedByRuleAndPath := map[string]string{}
				for _, fix := range plan.AppliedValuesFixes {
					appliedByRuleAndPath[fix.Finding.RuleID+"|"+fix.Finding.Path] = fix.ValuesPath
				}
				if got := appliedByRuleAndPath[tt.wantAppliedKey]; got != tt.wantAppliedValue {
					t.Fatalf("appliedByRuleAndPath[%q] = %q, want %q", tt.wantAppliedKey, got, tt.wantAppliedValue)
				}
			}
		})
	}
}

func TestPlanBundleKeepsAdvisoryFindingsOutOfKustomizeRouting(t *testing.T) {
	const wantDigestExplanation = "Pinning by digest requires looking up the correct digest in your container registry, so helmdoc cannot determine it automatically."

	fixture := realcharts.FixtureByID(t, "nginx-ingress")
	ctx := realcharts.AnalysisContext(t, fixture)
	findings := rules.RunAll(ctx)
	probeFinding := requireFindingByKey(t, findings, "HLT001", "templates/admission-webhooks/job-patch/job-createSecret.yaml", "Job/helmdoc-ingress-nginx-admission-create")
	advisoryFinding := models.Finding{
		RuleID:      "IMG002",
		Path:        "templates/controller-deployment.yaml",
		Resource:    "Deployment/helmdoc-ingress-nginx-controller",
		Title:       "Container image is not pinned by digest",
		Description: "The controller image is referenced by tag only.",
	}

	plan := PlanBundle(ctx, []models.Finding{advisoryFinding, probeFinding})
	if len(plan.AppliedValuesFixes) != 0 {
		t.Fatalf("len(plan.AppliedValuesFixes) = %d, want 0", len(plan.AppliedValuesFixes))
	}
	if len(plan.AdvisoryFindings) != 1 {
		t.Fatalf("len(plan.AdvisoryFindings) = %d, want 1", len(plan.AdvisoryFindings))
	}
	if len(plan.KustomizePatches) != 1 {
		t.Fatalf("len(plan.KustomizePatches) = %d, want 1", len(plan.KustomizePatches))
	}
	if len(plan.PendingFindings) != 0 {
		t.Fatalf("len(plan.PendingFindings) = %d, want 0", len(plan.PendingFindings))
	}

	advisory := requireAdvisoryFinding(t, plan, "IMG002")
	if advisory.Explanation != wantDigestExplanation {
		t.Fatalf("advisory explanation = %q, want %q", advisory.Explanation, wantDigestExplanation)
	}

	patch := plan.KustomizePatches[0]
	if patch.Finding.RuleID != "HLT001" {
		t.Fatalf("patch rule = %q, want %q", patch.Finding.RuleID, "HLT001")
	}
	if patch.Summary == "" {
		t.Fatal("patch summary = empty, want explanation")
	}
	if _, ok := advisoryExplanationForFinding(patch.Finding); ok {
		t.Fatalf("patch rule = %q, want supported Kustomize rule", patch.Finding.RuleID)
	}
	if patch.Finding.Resource != probeFinding.Resource || patch.Finding.Path != probeFinding.Path {
		t.Fatalf("patch finding = %#v, want %#v", patch.Finding, probeFinding)
	}
}

func requireAdvisoryFinding(t *testing.T, plan BundlePlan, ruleID string) AdvisoryFinding {
	t.Helper()

	for _, advisory := range plan.AdvisoryFindings {
		if advisory.Finding.RuleID == ruleID {
			return advisory
		}
	}

	t.Fatalf("advisory finding for %q missing from %#v", ruleID, plan.AdvisoryFindings)
	return AdvisoryFinding{}
}

func requireFindingByKey(t *testing.T, findings []models.Finding, ruleID, path, resource string) models.Finding {
	t.Helper()

	for _, finding := range findings {
		if finding.RuleID == ruleID && finding.Path == path && finding.Resource == resource {
			return finding
		}
	}

	t.Fatalf("finding %s %s @ %s missing from %#v", ruleID, resource, path, findings)
	return models.Finding{}
}

func requireProbeFieldOnPatch(t *testing.T, patch KustomizePatch, field string) {
	t.Helper()

	spec := requireNestedMap(t, patch.Patch, "spec")
	template := requireNestedMap(t, spec, "template")
	podSpec := requireNestedMap(t, template, "spec")
	containers, ok := podSpec["containers"].([]any)
	if !ok || len(containers) != 1 {
		t.Fatalf("patch containers = %#v, want one strategic-merge container entry", podSpec["containers"])
	}
	container, ok := containers[0].(map[string]any)
	if !ok {
		t.Fatalf("patch container = %#v, want map[string]any", containers[0])
	}
	if container["name"] != patch.ContainerName {
		t.Fatalf("patch container name = %#v, want %q", container["name"], patch.ContainerName)
	}
	probe, ok := container[field].(map[string]any)
	if !ok || len(probe) == 0 {
		t.Fatalf("patch %s = %#v, want populated probe payload", field, container[field])
	}
	if _, ok := probe["exec"].(map[string]any); !ok {
		t.Fatalf("patch %s.exec = %#v, want exec payload", field, probe["exec"])
	}
}

func requireNestedMap(t *testing.T, value map[string]any, key string) map[string]any {
	t.Helper()
	nested, ok := value[key].(map[string]any)
	if !ok {
		t.Fatalf("value[%q] = %#v, want map[string]any", key, value[key])
	}
	return nested
}
