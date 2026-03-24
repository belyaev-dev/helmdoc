package fix

import (
	"strings"
	"testing"

	"github.com/belyaev-dev/helmdoc/pkg/models"
	"sigs.k8s.io/yaml"
)

func TestMergeValuesOverrides(t *testing.T) {
	fixes := []AppliedValuesFix{
		{
			Finding:    models.Finding{RuleID: "SCL001", Path: "templates/controller-deployment.yaml", Resource: "Deployment/helmdoc-ingress-nginx-controller"},
			ValuesPath: "controller.autoscaling",
			Value: map[string]any{
				"enabled":                           true,
				"maxReplicas":                       3,
				"minReplicas":                       2,
				"targetCPUUtilizationPercentage":    50,
				"targetMemoryUtilizationPercentage": 50,
			},
		},
		{
			Finding:    models.Finding{RuleID: "AVL001", Path: "templates/controller-deployment.yaml", Resource: "Deployment/helmdoc-ingress-nginx-controller"},
			ValuesPath: "controller.autoscaling.minReplicas",
			Value:      2,
		},
		{
			Finding:    models.Finding{RuleID: "RES001", Path: "templates/controller-deployment.yaml", Resource: "Deployment/helmdoc-ingress-nginx-controller"},
			ValuesPath: "controller.resources.limits",
			Value:      map[string]any{"cpu": "100m", "memory": "90Mi"},
		},
		{
			Finding:    models.Finding{RuleID: "RES002", Path: "templates/controller-deployment.yaml", Resource: "Deployment/helmdoc-ingress-nginx-controller"},
			ValuesPath: "controller.resources.requests",
			Value:      map[string]any{"cpu": "100m", "memory": "90Mi"},
		},
	}

	merged, err := MergeValuesOverrides(fixes)
	if err != nil {
		t.Fatalf("MergeValuesOverrides() error = %v", err)
	}

	controller := requireNestedMap(t, merged, "controller")
	autoscaling := requireNestedMap(t, controller, "autoscaling")
	if autoscaling["enabled"] != true || autoscaling["minReplicas"] != 2 || autoscaling["maxReplicas"] != 3 {
		t.Fatalf("controller.autoscaling = %#v", autoscaling)
	}

	valuesYAML, err := RenderValuesOverridesYAML(BundlePlan{AppliedValuesFixes: fixes})
	if err != nil {
		t.Fatalf("RenderValuesOverridesYAML() error = %v", err)
	}

	var roundTrip map[string]any
	if err := yaml.Unmarshal(valuesYAML, &roundTrip); err != nil {
		t.Fatalf("yaml.Unmarshal(valuesYAML) error = %v\n%s", err, valuesYAML)
	}
	if got := requireNestedMap(t, requireNestedMap(t, roundTrip, "controller"), "autoscaling")["minReplicas"]; got != float64(2) && got != 2 {
		t.Fatalf("roundTrip controller.autoscaling.minReplicas = %#v, want 2", got)
	}
	for _, snippet := range []string{"autoscaling:", "resources:", "limits:", "requests:"} {
		if !strings.Contains(string(valuesYAML), snippet) {
			t.Fatalf("valuesYAML missing %q\n%s", snippet, valuesYAML)
		}
	}
}

func TestRenderREADMEIncludesUsageAndAdvisorySections(t *testing.T) {
	plan := BundlePlan{
		AppliedValuesFixes: []AppliedValuesFix{
			{
				Finding: models.Finding{
					RuleID:      "SEC003",
					Path:        "templates/controller-deployment.yaml",
					Resource:    "Deployment/helmdoc-ingress-nginx-controller",
					Description: "The controller container can write to its root filesystem.",
					Remediation: "Prefer a chart value that enables a read-only root filesystem when the chart exposes one.",
				},
				ValuesPath: "controller.image.readOnlyRootFilesystem",
				Value:      true,
				Summary:    "Enable the controller image read-only root filesystem knob.",
			},
		},
		KustomizePatches: []KustomizePatch{
			sampleReadmePatch(t),
		},
		AdvisoryFindings: []AdvisoryFinding{
			{
				Finding: models.Finding{
					RuleID:      "IMG002",
					Path:        "templates/controller-deployment.yaml",
					Resource:    "Deployment/helmdoc-ingress-nginx-controller",
					Description: "The controller image tag is not pinned by digest.",
					Remediation: "Look up the correct image digest in the registry used by this deployment.",
				},
				Explanation: "Pinning by digest requires looking up the correct digest in your container registry, so helmdoc cannot determine it automatically.",
			},
		},
		PendingFindings: []PendingFinding{
			{
				Finding: models.Finding{
					RuleID:      "UNK001",
					Path:        "templates/sidecar.yaml",
					Resource:    "Deployment/example",
					Description: "The sidecar uses an unsupported configuration pattern.",
					Remediation: "Refactor the template or chart values so the sidecar can be configured safely.",
				},
				Reason: "no safe automated remediation is available yet",
			},
		},
	}

	readme, err := RenderREADME(plan)
	if err != nil {
		t.Fatalf("RenderREADME() error = %v", err)
	}

	checks := []string{
		"# Helmdoc fix bundle",
		"## How to apply this bundle",
		"values-overrides.yaml",
		"kustomize/kustomization.yaml",
		"## Applied values fixes (1)",
		"Values path: `controller.image.readOnlyRootFilesystem`",
		"Helmdoc change: Enable the controller image read-only root filesystem knob.",
		"Rule detail: The controller container can write to its root filesystem.",
		"## Kustomize patches (1)",
		"Patch file: `kustomize/hlt001-job-helmdoc-ingress-nginx-admission-create.yaml`",
		"Target container: `create`",
		"## Advisory-only findings (1)",
		"Why helmdoc left this advisory-only: Pinning by digest requires looking up the correct digest in your container registry, so helmdoc cannot determine it automatically.",
		"Manual follow-up: Look up the correct image digest in the registry used by this deployment.",
		"## Findings pending another fix path (1)",
		"Still pending because: no safe automated remediation is available yet",
	}
	for _, check := range checks {
		if !strings.Contains(string(readme), check) {
			t.Fatalf("README missing %q\n%s", check, readme)
		}
	}
}

func TestRenderValuesOverridesYAMLIncludesProvenanceComments(t *testing.T) {
	plan := BundlePlan{
		AppliedValuesFixes: []AppliedValuesFix{
			{
				Finding: models.Finding{
					RuleID:      "SEC003",
					Path:        "templates/controller-deployment.yaml",
					Resource:    "Deployment/helmdoc-ingress-nginx-controller",
					Description: "The controller container can write to its root filesystem.",
				},
				ValuesPath: "controller.image.readOnlyRootFilesystem",
				Value:      true,
				Summary:    "Enable the controller image read-only root filesystem knob.",
			},
			{
				Finding: models.Finding{
					RuleID:      "RES001",
					Path:        "templates/controller-deployment.yaml",
					Resource:    "Deployment/helmdoc-ingress-nginx-controller",
					Description: "The controller container has no CPU or memory limits.",
				},
				ValuesPath: "controller.resources.limits",
				Value:      map[string]any{"cpu": "100m", "memory": "90Mi"},
				Summary:    "Populate baseline controller limits.",
			},
		},
	}

	valuesYAML, err := RenderValuesOverridesYAML(plan)
	if err != nil {
		t.Fatalf("RenderValuesOverridesYAML() error = %v", err)
	}

	text := string(valuesYAML)
	for _, check := range []string{
		"# Generated by helmdoc fix.",
		"# File: values-overrides.yaml",
		"# Apply this file with your Helm values workflow",
		"# Provenance:",
		"# - SEC003 Deployment/helmdoc-ingress-nginx-controller @ templates/controller-deployment.yaml -> controller.image.readOnlyRootFilesystem",
		"# planned fix: Enable the controller image read-only root filesystem knob.",
		"# rule detail: The controller container can write to its root filesystem.",
		"# - RES001 Deployment/helmdoc-ingress-nginx-controller @ templates/controller-deployment.yaml -> controller.resources.limits",
		"controller:",
		"readOnlyRootFilesystem: true",
		"limits:",
	} {
		if !strings.Contains(text, check) {
			t.Fatalf("values-overrides.yaml missing %q\n%s", check, text)
		}
	}

	var roundTrip map[string]any
	if err := yaml.Unmarshal(valuesYAML, &roundTrip); err != nil {
		t.Fatalf("yaml.Unmarshal(valuesYAML) error = %v\n%s", err, valuesYAML)
	}
	controller := requireNestedMap(t, roundTrip, "controller")
	image := requireNestedMap(t, controller, "image")
	if image["readOnlyRootFilesystem"] != true {
		t.Fatalf("controller.image.readOnlyRootFilesystem = %#v, want true", image["readOnlyRootFilesystem"])
	}
}

func TestRenderValuesOverridesYAMLUnaffectedByKustomizePatches(t *testing.T) {
	plan := BundlePlan{
		AppliedValuesFixes: []AppliedValuesFix{
			{
				Finding:    models.Finding{RuleID: "SEC003", Path: "templates/controller-deployment.yaml", Resource: "Deployment/helmdoc-ingress-nginx-controller"},
				ValuesPath: "controller.image.readOnlyRootFilesystem",
				Value:      true,
				Summary:    "Enable the controller image read-only root filesystem knob.",
			},
			{
				Finding:    models.Finding{RuleID: "RES001", Path: "templates/controller-deployment.yaml", Resource: "Deployment/helmdoc-ingress-nginx-controller"},
				ValuesPath: "controller.resources.limits",
				Value:      map[string]any{"cpu": "100m", "memory": "90Mi"},
				Summary:    "Populate baseline controller limits.",
			},
		},
	}

	baseline, err := RenderValuesOverridesYAML(plan)
	if err != nil {
		t.Fatalf("RenderValuesOverridesYAML(baseline) error = %v", err)
	}

	plan.KustomizePatches = []KustomizePatch{sampleReadmePatch(t)}
	withKustomize, err := RenderValuesOverridesYAML(plan)
	if err != nil {
		t.Fatalf("RenderValuesOverridesYAML(with Kustomize) error = %v", err)
	}

	if string(withKustomize) != string(baseline) {
		t.Fatalf("RenderValuesOverridesYAML changed when Kustomize patches were present\nbaseline:\n%s\nwith Kustomize:\n%s", baseline, withKustomize)
	}
}

func sampleReadmePatch(t *testing.T) KustomizePatch {
	t.Helper()

	finding := models.Finding{
		RuleID:      "HLT001",
		Path:        "templates/admission-webhooks/job-patch/job-createSecret.yaml",
		Resource:    "Job/helmdoc-ingress-nginx-admission-create",
		Description: `container "create" in Job/helmdoc-ingress-nginx-admission-create has no livenessProbe.`,
	}
	target := ResourceRef{
		APIVersion: "batch/v1",
		Kind:       "Job",
		Name:       "helmdoc-ingress-nginx-admission-create",
		Namespace:  "default",
	}
	patchBody, summary, err := defaultKustomizePatchForFinding(finding, target, "create")
	if err != nil {
		t.Fatalf("defaultKustomizePatchForFinding() error = %v", err)
	}

	return KustomizePatch{
		Finding:       finding,
		Target:        target,
		ContainerName: "create",
		Patch:         patchBody,
		Summary:       summary,
	}
}
