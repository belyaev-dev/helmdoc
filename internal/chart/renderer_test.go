package chart

import (
	"path/filepath"
	"strings"
	"testing"

	helmchart "helm.sh/helm/v3/pkg/chart"
)

func TestRenderChartParsesMultiDocumentYAML(t *testing.T) {
	loadedChart, err := LoadChart(fixtureChartPath(t))
	if err != nil {
		t.Fatalf("LoadChart() error = %v", err)
	}

	loadedChart.Templates = append(loadedChart.Templates,
		&helmchart.File{Name: "templates/NOTES.txt", Data: []byte("This should never be parsed as a manifest")},
		&helmchart.File{Name: "templates/tests/test-connection.yaml", Data: []byte(`apiVersion: v1
kind: Pod
metadata:
  name: skipped-test
  annotations:
    helm.sh/hook: test
spec:
  containers:
    - name: smoke
      image: busybox
      command: ["true"]
`)},
	)

	rendered, err := RenderChart(loadedChart)
	if err != nil {
		t.Fatalf("RenderChart() error = %v", err)
	}

	resources := rendered["templates/resources.yaml"]
	if len(resources) != 2 {
		t.Fatalf("len(rendered[templates/resources.yaml]) = %d, want 2", len(resources))
	}

	deployment := resources[0]
	if deployment.Kind != "Deployment" {
		t.Fatalf("resources[0].Kind = %q, want Deployment", deployment.Kind)
	}
	if deployment.APIVersion != "apps/v1" {
		t.Fatalf("resources[0].APIVersion = %q, want apps/v1", deployment.APIVersion)
	}
	if deployment.Name != "testchart-controller" {
		t.Fatalf("resources[0].Name = %q, want testchart-controller", deployment.Name)
	}
	if image, ok := deployment.GetNestedString("spec", "template", "spec", "containers", "0", "image"); !ok || image != "registry.k8s.io/ingress-nginx/controller:1.11.1" {
		t.Fatalf("deployment image = (%q, %t), want (%q, true)", image, ok, "registry.k8s.io/ingress-nginx/controller:1.11.1")
	}

	service := resources[1]
	if service.Kind != "Service" {
		t.Fatalf("resources[1].Kind = %q, want Service", service.Kind)
	}
	if service.Name != "testchart-controller" {
		t.Fatalf("resources[1].Name = %q, want testchart-controller", service.Name)
	}
	if port, ok := service.GetNested("spec", "ports", "0", "port"); !ok || port != float64(80) {
		t.Fatalf("service port = (%v, %t), want (80, true)", port, ok)
	}

	if _, ok := rendered["templates/NOTES.txt"]; ok {
		t.Fatal("rendered output unexpectedly includes templates/NOTES.txt")
	}
	if _, ok := rendered["templates/tests/test-connection.yaml"]; ok {
		t.Fatal("rendered output unexpectedly includes templates/tests/test-connection.yaml")
	}
}

func TestRenderChartUsesLintModeForGracefulOfflineTemplates(t *testing.T) {
	loadedChart, err := LoadChart(fixtureChartPath(t))
	if err != nil {
		t.Fatalf("LoadChart() error = %v", err)
	}

	loadedChart.Schema = []byte(`{"type":"object","properties":{"controller":{"type":"string"}}}`)

	rendered, err := RenderChart(loadedChart)
	if err != nil {
		t.Fatalf("RenderChart() error = %v", err)
	}

	resources := rendered["templates/graceful.yaml"]
	if len(resources) != 1 {
		t.Fatalf("len(rendered[templates/graceful.yaml]) = %d, want 1", len(resources))
	}

	resource := resources[0]
	if resource.Kind != "ConfigMap" {
		t.Fatalf("resource.Kind = %q, want ConfigMap", resource.Kind)
	}
	if resource.Name != "testchart-graceful" {
		t.Fatalf("resource.Name = %q, want testchart-graceful", resource.Name)
	}
	if resource.Namespace != "default" {
		t.Fatalf("resource.Namespace = %q, want default", resource.Namespace)
	}
	if got, ok := resource.GetNestedString("data", "lookupResult"); !ok || got != "absent" {
		t.Fatalf("lookupResult = (%q, %t), want (%q, true)", got, ok, "absent")
	}
	if got, ok := resource.GetNestedString("data", "requiredResult"); !ok || got != "lint-mode-fallback" {
		t.Fatalf("requiredResult = (%q, %t), want (%q, true)", got, ok, "lint-mode-fallback")
	}
}

func TestRenderChartRendersNginxIngressOffline(t *testing.T) {
	loadedChart, err := LoadChart(nginxIngressChartPath(t))
	if err != nil {
		t.Fatalf("LoadChart() error = %v", err)
	}

	rendered, err := RenderChart(loadedChart)
	if err != nil {
		t.Fatalf("RenderChart() error = %v", err)
	}

	if len(rendered) == 0 {
		t.Fatal("RenderChart() returned no rendered templates")
	}

	resources := rendered["templates/controller-deployment.yaml"]
	if len(resources) == 0 {
		t.Fatalf("len(rendered[templates/controller-deployment.yaml]) = %d, want > 0", len(resources))
	}

	deployment := resources[0]
	if deployment.Kind != "Deployment" {
		t.Fatalf("deployment.Kind = %q, want Deployment", deployment.Kind)
	}
	if deployment.Name != "helmdoc-ingress-nginx-controller" {
		t.Fatalf("deployment.Name = %q, want helmdoc-ingress-nginx-controller", deployment.Name)
	}
	if deployment.Namespace != "default" {
		t.Fatalf("deployment.Namespace = %q, want default", deployment.Namespace)
	}
	if image, ok := deployment.GetNestedString("spec", "template", "spec", "containers", "0", "image"); !ok || !strings.Contains(image, "ingress-nginx/controller:v1.15.1") {
		t.Fatalf("deployment image = (%q, %t), want controller:v1.15.1 image", image, ok)
	}

	resourceCount := 0
	for _, templateResources := range rendered {
		resourceCount += len(templateResources)
	}
	if resourceCount < 10 {
		t.Fatalf("rendered resource count = %d, want >= 10", resourceCount)
	}
}

func TestRenderChartRenderErrorIncludesTemplatePath(t *testing.T) {
	loadedChart, err := LoadChart(fixtureChartPath(t))
	if err != nil {
		t.Fatalf("LoadChart() error = %v", err)
	}

	loadedChart.Templates = append(loadedChart.Templates,
		&helmchart.File{Name: "templates/broken-render.yaml", Data: []byte("{{ if .Values.controller.replicaCount }}apiVersion: v1{{ end")},
	)

	_, err = RenderChart(loadedChart)
	if err == nil {
		t.Fatal("RenderChart() error = nil, want non-nil")
	}

	if !strings.Contains(err.Error(), loadedChart.ChartFullPath()) {
		t.Fatalf("error %q does not include chart path %q", err.Error(), loadedChart.ChartFullPath())
	}
	if !strings.Contains(err.Error(), "templates/broken-render.yaml") {
		t.Fatalf("error %q does not include template path", err.Error())
	}
}

func TestRenderChartParseErrorIncludesTemplatePathAndDocumentIndex(t *testing.T) {
	loadedChart, err := LoadChart(fixtureChartPath(t))
	if err != nil {
		t.Fatalf("LoadChart() error = %v", err)
	}

	loadedChart.Templates = append(loadedChart.Templates,
		&helmchart.File{Name: "templates/broken-parse.yaml", Data: []byte(`apiVersion: v1
kind: ConfigMap
metadata:
  name: ok-doc
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: broken-doc
data:
  broken: [unterminated
`)},
	)

	_, err = RenderChart(loadedChart)
	if err == nil {
		t.Fatal("RenderChart() error = nil, want non-nil")
	}

	if !strings.Contains(err.Error(), loadedChart.ChartFullPath()) {
		t.Fatalf("error %q does not include chart path %q", err.Error(), loadedChart.ChartFullPath())
	}
	if !strings.Contains(err.Error(), "templates/broken-parse.yaml") {
		t.Fatalf("error %q does not include template path", err.Error())
	}
	if !strings.Contains(err.Error(), "document 2") {
		t.Fatalf("error %q does not include document index", err.Error())
	}
}

func nginxIngressChartPath(t *testing.T) string {
	t.Helper()
	return filepath.Join("..", "..", "testdata", "nginx-ingress")
}
