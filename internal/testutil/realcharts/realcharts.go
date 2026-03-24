package realcharts

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	chartanalysis "github.com/belyaev-dev/helmdoc/internal/chart"
	"github.com/belyaev-dev/helmdoc/internal/rules"
	_ "github.com/belyaev-dev/helmdoc/internal/rules/all"
	"github.com/belyaev-dev/helmdoc/pkg/models"
	helmchart "helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chartutil"
)

type Manifest struct {
	Fixtures []Fixture `json:"fixtures"`
}

type Fixture struct {
	ID           string   `json:"id"`
	Path         string   `json:"path"`
	SourceURL    string   `json:"source_url"`
	ChartName    string   `json:"chart_name"`
	ChartVersion string   `json:"chart_version"`
	Expected     Expected `json:"expected"`
}

type Expected struct {
	OverallGrade      models.Grade      `json:"overall_grade"`
	OverallScore      float64           `json:"overall_score"`
	TotalFindings     int               `json:"total_findings"`
	CategorySummaries []CategorySummary `json:"category_summaries"`
	FindingTuples     []ExpectedFinding `json:"finding_tuples"`
	AnchorFindings    []AnchorFinding   `json:"anchor_findings"`
}

type CategorySummary struct {
	Category models.Category `json:"category"`
	Score    float64         `json:"score"`
	Grade    models.Grade    `json:"grade"`
	Findings int             `json:"findings"`
}

type ExpectedFinding struct {
	RuleID   string          `json:"rule_id"`
	Category models.Category `json:"category"`
	Severity models.Severity `json:"severity"`
	Path     string          `json:"path"`
	Resource string          `json:"resource"`
}

type AnchorFinding struct {
	RuleID      string          `json:"rule_id"`
	Category    models.Category `json:"category"`
	Severity    models.Severity `json:"severity"`
	Path        string          `json:"path"`
	Resource    string          `json:"resource"`
	Title       string          `json:"title"`
	Description string          `json:"description"`
	Remediation string          `json:"remediation"`
}

func LoadManifest(t testing.TB) Manifest {
	t.Helper()

	manifestPath := filepath.Join(findRepoRoot(t), "testdata", "real-chart-fixtures.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v", manifestPath, err)
	}

	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("json.Unmarshal(%q) error = %v", manifestPath, err)
	}

	return manifest
}

func FixtureByID(t testing.TB, id string) Fixture {
	t.Helper()

	for _, fixture := range LoadManifest(t).Fixtures {
		if fixture.ID == id {
			return fixture
		}
	}

	t.Fatalf("fixture %q missing from manifest", id)
	return Fixture{}
}

func LoadChart(t testing.TB, fixture Fixture) *helmchart.Chart {
	t.Helper()

	chartPath := FixturePath(t, fixture)
	loadedChart, err := chartanalysis.LoadChart(chartPath)
	if err != nil {
		t.Fatalf("LoadChart(%q) error = %v", chartPath, err)
	}

	return loadedChart
}

func AnalysisContext(t testing.TB, fixture Fixture) rules.AnalysisContext {
	t.Helper()

	loadedChart := LoadChart(t, fixture)
	rendered, err := chartanalysis.RenderChart(loadedChart)
	if err != nil {
		t.Fatalf("RenderChart(%q) error = %v", FixturePath(t, fixture), err)
	}

	return rules.AnalysisContext{
		Chart:             loadedChart,
		RenderedResources: rendered,
		ValuesSurface:     chartanalysis.AnalyzeValues(loadedChart),
	}
}

func RunRules(t testing.TB, fixture Fixture) []models.Finding {
	t.Helper()

	ctx := AnalysisContext(t, fixture)
	return rules.RunAll(ctx)
}

func RenderChartWithValues(t testing.TB, fixture Fixture, overrideValues map[string]any) (*helmchart.Chart, map[string][]models.K8sResource) {
	t.Helper()

	loadedChart := LoadChart(t, fixture)
	loadedChart.Values = MergeChartValues(loadedChart.Values, overrideValues)

	rendered, err := chartanalysis.RenderChart(loadedChart)
	if err != nil {
		t.Fatalf("RenderChart(%q) with overrides error = %v", FixturePath(t, fixture), err)
	}

	return loadedChart, rendered
}

func MergeChartValues(baseValues, overrideValues map[string]any) map[string]any {
	mergedOverrides := cloneMap(overrideValues)
	if mergedOverrides == nil {
		mergedOverrides = map[string]any{}
	}

	return chartutil.CoalesceTables(mergedOverrides, cloneMap(baseValues))
}

func FindRenderedResource(rendered map[string][]models.K8sResource, templatePath, kind, name string) (models.K8sResource, bool) {
	for _, resource := range rendered[templatePath] {
		if resource.Kind != kind {
			continue
		}
		if resource.Name != name {
			continue
		}
		return resource, true
	}

	return models.K8sResource{}, false
}

func FindWorkloadContainer(rendered map[string][]models.K8sResource, templatePath, kind, name, containerName string) (rules.WorkloadContainer, bool) {
	var matched rules.WorkloadContainer
	found := false

	rules.IterateWorkloadContainers(rendered, func(container rules.WorkloadContainer) bool {
		if container.TemplatePath != templatePath {
			return true
		}
		if container.Resource.Kind != kind || container.Resource.Name != name {
			return true
		}
		if container.Name != containerName {
			return true
		}
		matched = container
		found = true
		return false
	})

	return matched, found
}

func RepoPath(t testing.TB, rel string) string {
	t.Helper()

	return filepath.Join(findRepoRoot(t), filepath.FromSlash(rel))
}

func FixturePath(t testing.TB, fixture Fixture) string {
	t.Helper()

	return RepoPath(t, fixture.Path)
}

func findRepoRoot(t testing.TB) string {
	t.Helper()

	for _, candidate := range []string{".", "..", filepath.Join("..", ".."), filepath.Join("..", "..", ".."), filepath.Join("..", "..", "..", "..")} {
		manifestPath := filepath.Join(candidate, "testdata", "real-chart-fixtures.json")
		if _, err := os.Stat(manifestPath); err == nil {
			return candidate
		}
	}

	t.Fatal("unable to locate repo root from test working directory")
	return ""
}

func cloneMap(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}

	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = cloneValue(value)
	}
	return out
}

func cloneValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return cloneMap(typed)
	case []any:
		cloned := make([]any, len(typed))
		for i, nested := range typed {
			cloned[i] = cloneValue(nested)
		}
		return cloned
	default:
		return typed
	}
}
