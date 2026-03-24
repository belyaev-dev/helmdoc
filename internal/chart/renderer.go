// Package chart handles Helm chart loading and parsing.
package chart

import (
	"fmt"
	pathpkg "path"
	"sort"
	"strings"

	"github.com/belyaev-dev/helmdoc/pkg/models"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/engine"
	"helm.sh/helm/v3/pkg/releaseutil"
	"sigs.k8s.io/yaml"
)

const schemaValidationFailurePrefix = "values don't meet the specifications of the schema"

// RenderChart renders a chart fully offline and returns parsed Kubernetes resources grouped by template path.
func RenderChart(c *chart.Chart) (map[string][]models.K8sResource, error) {
	if c == nil {
		return nil, fmt.Errorf("render chart: nil chart")
	}

	valuesToRender, err := buildRenderValues(c)
	if err != nil {
		return nil, err
	}

	renderedFiles, err := (engine.Engine{LintMode: true}).Render(c, valuesToRender)
	if err != nil {
		return nil, fmt.Errorf("render chart %q: %w", chartIdentifier(c), err)
	}

	result := make(map[string][]models.K8sResource)
	chartRoot := c.ChartFullPath()

	paths := make([]string, 0, len(renderedFiles))
	for renderedPath := range renderedFiles {
		paths = append(paths, renderedPath)
	}
	sort.Strings(paths)

	for _, renderedPath := range paths {
		templatePath := normalizeTemplatePath(chartRoot, renderedPath)
		if shouldSkipRenderedTemplate(templatePath) {
			continue
		}

		documents := releaseutil.SplitManifests(renderedFiles[renderedPath])
		if len(documents) == 0 {
			continue
		}

		documentKeys := make([]string, 0, len(documents))
		for key := range documents {
			documentKeys = append(documentKeys, key)
		}
		sort.Sort(releaseutil.BySplitManifestsOrder(documentKeys))

		for index, documentKey := range documentKeys {
			resource, ok, err := parseRenderedResource(documents[documentKey])
			if err != nil {
				return nil, fmt.Errorf("parse rendered manifest for chart %q template %q document %d: %w", chartIdentifier(c), templatePath, index+1, err)
			}
			if !ok {
				continue
			}

			result[templatePath] = append(result[templatePath], resource)
		}
	}

	return result, nil
}

func buildRenderValues(c *chart.Chart) (chartutil.Values, error) {
	baseValues := c.Values
	if baseValues == nil {
		baseValues = map[string]any{}
	}

	releaseOptions := chartutil.ReleaseOptions{
		Name:      "helmdoc",
		Namespace: "default",
		IsInstall: true,
		Revision:  1,
	}

	valuesToRender, err := chartutil.ToRenderValuesWithSchemaValidation(c, baseValues, releaseOptions, chartutil.DefaultCapabilities, false)
	if err == nil {
		return valuesToRender, nil
	}
	if !strings.Contains(err.Error(), schemaValidationFailurePrefix) {
		return nil, fmt.Errorf("build render values for chart %q: %w", chartIdentifier(c), err)
	}

	valuesToRender, fallbackErr := chartutil.ToRenderValuesWithSchemaValidation(c, baseValues, releaseOptions, chartutil.DefaultCapabilities, true)
	if fallbackErr != nil {
		return nil, fmt.Errorf("build render values for chart %q after schema-validation fallback (initial error: %v): %w", chartIdentifier(c), err, fallbackErr)
	}

	return valuesToRender, nil
}

func parseRenderedResource(document string) (models.K8sResource, bool, error) {
	trimmed := strings.TrimSpace(document)
	if trimmed == "" {
		return models.K8sResource{}, false, nil
	}

	var decoded any
	if err := yaml.Unmarshal([]byte(trimmed), &decoded); err != nil {
		return models.K8sResource{}, false, err
	}
	if decoded == nil {
		return models.K8sResource{}, false, nil
	}

	raw, ok := decoded.(map[string]any)
	if !ok || len(raw) == 0 {
		return models.K8sResource{}, false, nil
	}

	apiVersion, _ := raw["apiVersion"].(string)
	kind, _ := raw["kind"].(string)
	if apiVersion == "" && kind == "" {
		return models.K8sResource{}, false, nil
	}
	if apiVersion == "" || kind == "" {
		return models.K8sResource{}, false, fmt.Errorf("manifest is missing apiVersion or kind")
	}

	resource := models.K8sResource{
		APIVersion: apiVersion,
		Kind:       kind,
		Raw:        raw,
	}

	if metadata, ok := raw["metadata"].(map[string]any); ok {
		if name, ok := metadata["name"].(string); ok {
			resource.Name = name
		}
		if namespace, ok := metadata["namespace"].(string); ok {
			resource.Namespace = namespace
		}
	}

	return resource, true, nil
}

func normalizeTemplatePath(chartRoot, renderedPath string) string {
	prefix := chartRoot + "/"
	if strings.HasPrefix(renderedPath, prefix) {
		return strings.TrimPrefix(renderedPath, prefix)
	}
	return renderedPath
}

func shouldSkipRenderedTemplate(templatePath string) bool {
	cleaned := strings.TrimPrefix(templatePath, "./")
	if strings.EqualFold(pathpkg.Base(cleaned), "NOTES.txt") {
		return true
	}
	return strings.Contains("/"+cleaned, "/templates/tests/")
}

func chartIdentifier(c *chart.Chart) string {
	if c == nil {
		return "<nil>"
	}
	if fullPath := c.ChartFullPath(); fullPath != "" {
		return fullPath
	}
	if name := c.Name(); name != "" {
		return name
	}
	return "<unnamed-chart>"
}
