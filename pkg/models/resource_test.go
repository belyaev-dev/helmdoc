package models

import "testing"

func TestK8sResourceHelpers(t *testing.T) {
	resource := K8sResource{
		APIVersion: "apps/v1",
		Kind:       "Deployment",
		Name:       "testchart-controller",
		Namespace:  "default",
		Raw: map[string]any{
			"metadata": map[string]any{
				"name": "testchart-controller",
				"labels": map[string]any{
					"app.kubernetes.io/name": "testchart",
				},
			},
			"spec": map[string]any{
				"template": map[string]any{
					"spec": map[string]any{
						"serviceAccountName": "testchart-sa",
						"containers": []any{
							map[string]any{
								"name":  "controller",
								"image": "registry.k8s.io/ingress-nginx/controller:1.11.1",
								"securityContext": map[string]any{
									"allowPrivilegeEscalation": false,
									"readOnlyRootFilesystem":   "false",
								},
							},
						},
					},
				},
			},
		},
	}

	if got, ok := resource.GetNestedString("metadata", "name"); !ok || got != "testchart-controller" {
		t.Fatalf("GetNestedString(metadata.name) = (%q, %t), want (%q, true)", got, ok, "testchart-controller")
	}

	if got, ok := resource.GetNestedString("spec", "template", "spec", "containers", "0", "image"); !ok || got != "registry.k8s.io/ingress-nginx/controller:1.11.1" {
		t.Fatalf("GetNestedString(spec.template.spec.containers.0.image) = (%q, %t)", got, ok)
	}

	if got, ok := resource.GetNestedBool("spec", "template", "spec", "containers", "0", "securityContext", "allowPrivilegeEscalation"); !ok || got {
		t.Fatalf("GetNestedBool(...allowPrivilegeEscalation) = (%t, %t), want (false, true)", got, ok)
	}

	labels, ok := resource.GetNestedMap("metadata", "labels")
	if !ok {
		t.Fatal("GetNestedMap(metadata.labels) returned !ok")
	}
	if labels["app.kubernetes.io/name"] != "testchart" {
		t.Fatalf("labels[app.kubernetes.io/name] = %v, want testchart", labels["app.kubernetes.io/name"])
	}

	containers, ok := resource.GetNestedSlice("spec", "template", "spec", "containers")
	if !ok {
		t.Fatal("GetNestedSlice(spec.template.spec.containers) returned !ok")
	}
	if len(containers) != 1 {
		t.Fatalf("len(containers) = %d, want 1", len(containers))
	}

	if _, ok := resource.GetNestedString("spec", "template", "spec", "containers"); ok {
		t.Fatal("GetNestedString on slice path returned ok, want false")
	}

	if _, ok := resource.GetNestedBool("spec", "template", "spec", "containers", "0", "securityContext", "readOnlyRootFilesystem"); ok {
		t.Fatal("GetNestedBool on string value returned ok, want false")
	}

	if _, ok := resource.GetNestedBool("spec", "template", "spec", "missing"); ok {
		t.Fatal("GetNestedBool on missing path returned ok, want false")
	}

	if _, ok := resource.GetNestedString("spec", "template", "spec", "missing"); ok {
		t.Fatal("GetNestedString on missing path returned ok, want false")
	}
}
